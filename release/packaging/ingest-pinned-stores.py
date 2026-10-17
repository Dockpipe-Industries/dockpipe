#!/usr/bin/env python3
"""Verify pinned package-host bundles and compose one complete native store."""
import argparse
import copy
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path, PurePosixPath
import re
import shutil
import stat
import subprocess
import tarfile
import tempfile


MAX_LOCK_BYTES = 64 << 10
MAX_BUNDLE_BYTES = 512 << 20
MAX_EXPANDED_BYTES = 1 << 30
MAX_MEMBERS = 1000
PLATFORMS = {'linux-amd64', 'linux-arm64', 'darwin-amd64', 'darwin-arm64', 'windows-amd64'}
GITHUB_BASELINE_URL = re.compile(
    r'https://github\.com/Dockpipe-Industries/dockpipe/releases/download/'
    r'v(?P<version>[0-9]+\.[0-9]+\.[0-9]+)-staging\.[0-9]+\.[0-9]+\.[0-9a-f]{12}/'
    r'dockpipe-packages_(?P=version)_(?P<platform>linux-amd64|linux-arm64|darwin-amd64|darwin-arm64|windows-amd64)\.tar\.gz'
)
STAGING_CANDIDATE_URL = re.compile(
    r'https://packages\.staging\.dockpipe\.com/packages/standalone/'
    r'(?P<owner>dorkpipe|pipeon)/candidates/'
    r'(?P<version>[0-9]+\.[0-9]+\.[0-9]+)-staging\.[0-9a-f]{12}/'
    r'(?P=owner)-packages-(?P=version)-linux-amd64\.tar\.gz'
)
STABLE_RELEASE_URL = re.compile(
    r'https://packages\.dockpipe\.com/packages/standalone/'
    r'(?P<owner>dorkpipe|pipeon)/releases/'
    r'(?P<version>[0-9]+\.[0-9]+\.[0-9]+)/'
    r'(?P=owner)-packages-(?P=version)-linux-amd64\.tar\.gz'
)
PIN_FIELDS = {'url', 'sha256', 'size_bytes', 'manifest_sha256'}


def composer_module():
    path = Path(__file__).with_name('compose-package-store.py')
    spec = importlib.util.spec_from_file_location('package_store_composer', path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


composer = composer_module()


def regular_file(path, maximum):
    if any(parent.is_symlink() for parent in (path, *path.parents)):
        raise ValueError('Pinned input path contains a link')
    info = path.stat()
    if not stat.S_ISREG(info.st_mode) or info.st_size > maximum:
        raise ValueError('Pinned input is not a bounded regular file')
    return info.st_size


def read_json(path, maximum):
    regular_file(path, maximum)
    with path.open('rb') as stream:
        data = stream.read(maximum + 1)
    if len(data) > maximum:
        raise ValueError('JSON grew beyond capacity')
    return json.loads(data, object_pairs_hook=composer.reject_duplicate_keys)


def validate_pin(pin, *, baseline_platform=None, candidate_name=None, candidate_version=None):
    if not isinstance(pin, dict) or set(pin) != PIN_FIELDS:
        raise ValueError('Published bundle requires complete immutable pins')
    url = pin['url']
    if not isinstance(url, str) or (baseline_platform is None) == (candidate_name is None):
        raise ValueError('Bundle requires an explicit baseline or candidate source')
    if baseline_platform is not None:
        matched = GITHUB_BASELINE_URL.fullmatch(url)
        if matched is None or matched['platform'] != baseline_platform:
            raise ValueError('Baseline must use its immutable GitHub release asset')
    else:
        matched = STAGING_CANDIDATE_URL.fullmatch(url) or STABLE_RELEASE_URL.fullmatch(url)
        if (matched is None or matched['owner'] != candidate_name
                or matched['version'] != candidate_version):
            raise ValueError('Candidate must use its immutable package-host archive')
    for field in ['sha256', 'manifest_sha256']:
        if not isinstance(pin[field], str) or not re.fullmatch(r'[0-9a-f]{64}', pin[field]):
            raise ValueError('Bundle and manifest require exact SHA-256 pins')
    if type(pin['size_bytes']) is not int or not 0 < pin['size_bytes'] <= MAX_BUNDLE_BYTES:
        raise ValueError('Bundle requires a bounded exact byte size')


def validate_hidden_identities(raw_identities, candidate_ownership):
    if not isinstance(raw_identities, list):
        raise ValueError('Hidden identities must be a list')
    identities = composer.validate_ownership(raw_identities) if raw_identities else set()
    if not identities.issubset(candidate_ownership):
        raise ValueError('Hidden identities must belong to a trusted candidate owner')
    return identities


def load_lock(path):
    lock = read_json(path, MAX_LOCK_BYTES)
    if (not isinstance(lock, dict) or set(lock) != {'schema', 'baselines', 'linux_candidates', 'hidden_identities'}
            or type(lock['schema']) is not int or lock['schema'] != 1):
        raise ValueError('Unsupported first-party store lock')
    if not isinstance(lock['baselines'], dict) or set(lock['baselines']) != PLATFORMS:
        raise ValueError('Lock requires all five qualified native platform baselines')
    for platform, pin in lock['baselines'].items():
        validate_pin(pin, baseline_platform=platform)
    candidates = lock['linux_candidates']
    if not isinstance(candidates, dict) or set(candidates) != {'dorkpipe', 'pipeon'}:
        raise ValueError('Lock requires both independently published Linux candidates')
    seen = set()
    for name, candidate in candidates.items():
        if not isinstance(candidate, dict) or set(candidate) != {'version', 'owned', 'artifact'}:
            raise ValueError('Candidate requires version, trusted ownership and immutable artifact')
        if not isinstance(candidate['version'], str) or not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', candidate['version']):
            raise ValueError('Candidate version must be exact')
        identities = composer.validate_ownership(candidate['owned'])
        expected_counts = (8, 18) if name == 'dorkpipe' else (2, 4)
        counts = tuple(sum(kind == expected for kind, _ in identities) for expected in ['resolver', 'workflow'])
        if counts != expected_counts or identities & seen:
            raise ValueError('Candidate ownership is incomplete or overlaps another publisher')
        seen.update(identities)
        validate_pin(candidate['artifact'], candidate_name=name,
                     candidate_version=candidate['version'])
    validate_hidden_identities(lock['hidden_identities'], seen)
    return lock


def apply_hidden_policy(manifest, hidden_identities):
    for kind, entries in (('resolver', manifest['packages'].get('resolvers', [])),
                          ('workflow', manifest['packages'].get('workflows', []))):
        for entry in entries:
            if (kind, entry['name']) in hidden_identities:
                entry['hidden'] = True


def verify_bundle(path, pin):
    size = regular_file(path, MAX_BUNDLE_BYTES)
    if size != pin['size_bytes']:
        raise ValueError('Bundle size differs from its committed pin')
    digest = hashlib.sha256()
    total_bytes = 0
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1 << 20), b''):
            total_bytes += len(block)
            if total_bytes > pin['size_bytes']:
                raise ValueError('Bundle grew beyond its committed pin')
            digest.update(block)
    if total_bytes != pin['size_bytes']:
        raise ValueError('Bundle size changed during verification')
    if digest.hexdigest() != pin['sha256']:
        raise ValueError('Bundle checksum differs from its committed pin')


def download(pin, destination, *, baseline_platform=None, candidate_name=None, candidate_version=None):
    validate_pin(pin, baseline_platform=baseline_platform, candidate_name=candidate_name,
                 candidate_version=candidate_version)
    if destination.exists() or destination.is_symlink():
        raise ValueError('Download destination must be fresh')
    command = ['curl', '--fail', '--silent', '--show-error', '--location',
               '--proto', '=https', '--proto-redir', '=https', '--connect-timeout', '20',
               '--max-time', '300', '--max-filesize', str(pin['size_bytes'])]
    if candidate_name is not None:
        # A package host may serve the selected archive directly. Never follow a
        # redirect to a different origin (or another mutable path on this host).
        command.extend(['--max-redirs', '0'])
    command.extend(['--output', str(destination), pin['url']])
    subprocess.run(command, check=True, timeout=310)
    verify_bundle(destination, pin)


class ExpandedReader:
    def __init__(self, stream):
        self.stream = stream
        self.remaining = MAX_EXPANDED_BYTES

    def read(self, size):
        if size < 0 or size > self.remaining:
            raise ValueError('Bundle exceeds expanded capacity')
        data = self.stream.read(size)
        self.remaining -= len(data)
        return data


def unpack_bundle(path, pin, output, platform, candidate=False):
    # Integrity is checked before even opening the tar. Native baselines contain
    # flat store files; standalone candidates contain packages/<store file> and
    # one root LICENSE. Nested package archives remain opaque hashed inputs.
    verify_bundle(path, pin)
    if candidate and platform != 'linux-amd64':
        raise ValueError('Standalone candidates are qualified only for Linux amd64')
    output.mkdir()
    seen = set()
    license_seen = False
    total = 0
    with gzip.open(path, 'rb') as compressed:
        with tarfile.open(fileobj=ExpandedReader(compressed), mode='r|') as archive:
            for member in archive:
                name = member.name
                if not candidate and name in {'.', './'} and member.isdir():
                    continue
                if not candidate and name.startswith('./'):
                    name = name[2:]
                if (name != PurePosixPath(name).as_posix() or '\\' in name
                        or len(seen) >= MAX_MEMBERS or not member.isfile()):
                    raise ValueError('Bundle contains a linked, nested, duplicate or unsafe member')
                if candidate and name == 'LICENSE':
                    if license_seen or not 0 < member.size <= 65536:
                        raise ValueError('Candidate requires one bounded root LICENSE')
                    license_seen = True
                    if len(archive.extractfile(member).read(65537)) != member.size:
                        raise ValueError('Candidate LICENSE is truncated')
                    total += member.size
                    if total > MAX_EXPANDED_BYTES:
                        raise ValueError('Candidate bundle exceeds expanded capacity')
                    continue
                if candidate:
                    if not name.startswith('packages/'):
                        raise ValueError('Candidate store file must use packages/ prefix')
                    name = name.removeprefix('packages/')
                if '/' in name or name in seen:
                    raise ValueError('Bundle contains a nested or duplicate store file')
                if name != 'packages-store-manifest.json' and not re.fullmatch(r'dockpipe-[A-Za-z0-9._+-]+\.tar\.gz(?:\.sha256)?', name):
                    raise ValueError('Bundle contains an unlisted store file type')
                seen.add(name)
                total += member.size
                if member.size < 0 or total > MAX_EXPANDED_BYTES:
                    raise ValueError('Bundle declared files exceed expanded capacity')
                if name == 'packages-store-manifest.json' and member.size > composer.MAX_MANIFEST_BYTES:
                    raise ValueError('Bundle manifest exceeds catalog capacity')
                with archive.extractfile(member) as source, (output / name).open('xb') as target:
                    copied = 0
                    while block := source.read(1 << 20):
                        copied += len(block)
                        if copied > member.size:
                            raise ValueError('Bundle member grew beyond declared size')
                        target.write(block)
                    if copied != member.size:
                        raise ValueError('Bundle member is truncated')
    if candidate and not license_seen:
        raise ValueError('Candidate bundle is missing its root LICENSE')
    manifest_path = output / 'packages-store-manifest.json'
    raw = manifest_path.read_bytes()
    if hashlib.sha256(raw).hexdigest() != pin['manifest_sha256']:
        raise ValueError('Bundle manifest differs from its committed pin')
    manifest = read_json(manifest_path, composer.MAX_MANIFEST_BYTES)
    entries, names = composer.validate_store(manifest)
    if manifest['platform'] != platform:
        raise ValueError('Bundle platform differs from the selected target')
    expected = {'packages-store-manifest.json', *names}
    if not expected.issubset(seen) or not seen.issubset(expected | {name + '.sha256' for name in names}):
        raise ValueError('Bundle contains missing or unlisted archives')
    for name in names:
        sidecar = output / (name + '.sha256')
        if sidecar.exists():
            regular_file(sidecar, 1024)
            digest = next((entry['sha256'] for entry in entries.values() if entry['tarball'] == name), None)
            if digest is None:
                digest = manifest['packages']['core']['sha256']
            if sidecar.read_text().strip().lower() != digest.lower():
                raise ValueError('Bundle archive sidecar differs from its catalog')
    return manifest


MAIN_WORKFLOW_IDENTITY = ('workflow', 'dockpipe-repo-quality')


def compose_inputs(lock, platform, baseline, baseline_root, candidates, core, core_root,
                   main, main_root, output):
    if (platform not in PLATFORMS or baseline['platform'] != platform
            or core.get('platform') != platform or main.get('platform') != platform):
        raise ValueError('Selected core, main workflow and baseline must match a qualified native target')
    composer.validate_store(core)
    main_entries, _ = composer.validate_store(main, candidate=True)
    baseline_entries, _ = composer.validate_store(baseline)
    if set(main_entries) != {MAIN_WORKFLOW_IDENTITY} or MAIN_WORKFLOW_IDENTITY not in baseline_entries:
        raise ValueError('Main store must replace exactly the existing repository-quality workflow')
    main_entry = main_entries[MAIN_WORKFLOW_IDENTITY]
    old_entry = baseline_entries[MAIN_WORKFLOW_IDENTITY]
    if (not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', main_entry['version'])
            or not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', old_entry['version'])):
        raise ValueError('Main wrapper requires comparable semantic versions')
    new_version = tuple(map(int, main_entry['version'].split('.')))
    old_version = tuple(map(int, old_entry['version'].split('.')))
    if new_version < old_version:
        raise ValueError('Main wrapper cannot downgrade the baseline version')
    candidate_ownership = set().union(*(composer.validate_ownership(candidate['owned'])
                                        for candidate in lock['linux_candidates'].values()))
    if MAIN_WORKFLOW_IDENTITY in candidate_ownership:
        raise ValueError('External candidate cannot own the main repository-quality workflow')
    hidden_identities = validate_hidden_identities(lock['hidden_identities'], candidate_ownership)
    if (set(core['packages']) - {'core', 'resolvers', 'workflows'}
            or not core['packages'].get('core')
            or core['packages'].get('resolvers')
            or core['packages'].get('workflows')):
        raise ValueError('Main build may supply only its newly qualified core')
    base = copy.deepcopy(baseline)
    base['packages']['core'] = copy.deepcopy(core['packages']['core'])
    core_archive = core_root / core['packages']['core']['tarball']
    regular_file(core_archive, MAX_EXPANDED_BYTES)
    shutil.copyfile(core_archive, baseline_root / core_archive.name)
    candidate = {'schema': 1, 'platform': platform, 'packages': {
        'resolvers': [], 'workflows': [copy.deepcopy(main_entry)]}}
    owned = [list(MAIN_WORKFLOW_IDENTITY)]
    candidate_root = baseline_root.parent / ('candidate-archives-' + platform)
    candidate_root.mkdir()
    source = main_root / main_entry['tarball']
    regular_file(source, MAX_EXPANDED_BYTES)
    shutil.copyfile(source, candidate_root / source.name)
    if platform == 'linux-amd64':
        if set(candidates) != set(lock['linux_candidates']):
            raise ValueError('Linux store requires both independently published candidates')
        for name, (manifest, directory) in candidates.items():
            pin = lock['linux_candidates'][name]
            entries, _ = composer.validate_store(manifest, candidate=True)
            if manifest['platform'] != platform or set(entries) != composer.validate_ownership(pin['owned']):
                raise ValueError('Candidate platform or exact ownership differs from lock')
            for identity, entry in entries.items():
                if entry['version'] != pin['version'] or entry.get('min_dockpipe_version') != '0.6.17':
                    raise ValueError('Candidate version or minimum compiler differs from lock')
                candidate['packages'][composer.KINDS[identity[0]]].append(entry)
                source = directory / entry['tarball']
                regular_file(source, MAX_EXPANDED_BYTES)
                destination = candidate_root / source.name
                if destination.exists():
                    raise ValueError('Independent candidates collide on an archive')
                shutil.copyfile(source, destination)
            owned.extend(pin['owned'])
    else:
        if candidates:
            raise ValueError('Linux candidates cannot qualify another platform')
        # All other optional archives retain their verified baseline bytes.
        # The main-owned wrapper is always replaced; it must never fall back
        # to the old unpinned package on a retained platform.
    apply_hidden_policy(base, hidden_identities)
    apply_hidden_policy(candidate, hidden_identities)
    return composer.assemble(base, candidate, owned, baseline_root, candidate_root, output, MAX_EXPANDED_BYTES)


def ingest(lock_path, platform, core_root, main_root, output):
    lock = load_lock(lock_path)  # Validate every pin before any download.
    if platform not in PLATFORMS:
        raise ValueError('No qualified baseline exists for this platform')
    core = read_json(core_root / 'packages-store-manifest.json', composer.MAX_MANIFEST_BYTES)
    main = read_json(main_root / 'packages-store-manifest.json', composer.MAX_MANIFEST_BYTES)
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='pinned-stores-', dir=output.parent) as temporary:
        scratch = Path(temporary)
        bundle = scratch / 'baseline.tar.gz'
        download(lock['baselines'][platform], bundle, baseline_platform=platform)
        baseline_root = scratch / 'baseline'
        baseline = unpack_bundle(bundle, lock['baselines'][platform], baseline_root, platform)
        candidates = {}
        if platform == 'linux-amd64':
            for name, candidate in lock['linux_candidates'].items():
                bundle = scratch / (name + '.tar.gz')
                download(candidate['artifact'], bundle, candidate_name=name,
                         candidate_version=candidate['version'])
                directory = scratch / name
                manifest = unpack_bundle(bundle, candidate['artifact'], directory, platform, candidate=True)
                candidates[name] = manifest, directory
        return compose_inputs(lock, platform, baseline, baseline_root, candidates,
                              core, core_root, main, main_root, output)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--lock', type=Path, required=True)
    parser.add_argument('--platform', required=True)
    parser.add_argument('--core-store', type=Path, required=True)
    parser.add_argument('--main-store', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    try:
        manifest = ingest(args.lock, args.platform, args.core_store, args.main_store, args.out)
    except (OSError, ValueError, UnicodeError, tarfile.TarError, subprocess.SubprocessError) as error:
        parser.exit(1, f'Pinned store ingestion failed: {error}\n')
    print(f"Composed verified {manifest['platform']} store with independently pinned optional packages")


if __name__ == '__main__':
    main()
