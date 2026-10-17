#!/usr/bin/env python3
"""Build a Linux amd64 compiler bootstrap without package or channel publication."""
import argparse
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile


MAX_HISTORY_BYTES = 8 << 20
MAX_ARCHIVE_BYTES = 128 << 20
MAX_EXECUTABLE_BYTES = 256 << 20


def digest(path):
    checksum = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1 << 20), b''):
            checksum.update(block)
    return checksum.hexdigest()


def verify_compiler(cli, version, root):
    """Exercise the formerly skipped asset-only leaf through the produced CLI."""
    root.mkdir(parents=True)
    environment = dict(os.environ)
    for variable in ('DOCKPIPE_PACKAGES_ROOT', 'DOCKPIPE_SOURCE_ROOT', 'DOCKPIPE_CORE_ROOT'):
        environment.pop(variable, None)
    environment.update(DOCKPIPE_GLOBAL_ROOT=str(root / 'global'), DOCKPIPE_SYSTEM_ROOT=str(root / 'system'),
                       XDG_STATE_HOME=str(root / 'state'), DOCKPIPE_BIN=str(cli))
    project = root / 'project'
    project.mkdir()
    (project / 'dockpipe.config.json').write_text(
        '{"schema":1,"compile":{"workflows":[]},"packages":{"namespace":"bootstrapfixture"}}\n'
    )
    source = root / 'vendor' / 'assets-only'
    (source / 'assets').mkdir(parents=True)
    (source / 'config.yml').write_text('name: assets-only\nnamespace: bootstrapfixture\nsteps: []\n')
    (source / 'package.yml').write_text(
        'schema: 1\nkind: resolver\nname: assets-only\nnamespace: bootstrapfixture\nversion: 1.0.0\nhidden: true\n'
        f'min_dockpipe_version: {version}\n'
    )
    payload = b'asset-only bootstrap regression\n'
    (source / 'assets/payload.txt').write_bytes(payload)
    store = root / 'store'
    subprocess.run([str(cli), 'package', 'compile', 'resolvers', '--workdir', str(project),
                    '--from', str(source.parent), '--force'], env=environment, check=True, timeout=120)
    subprocess.run([str(cli), 'package', 'build', 'store', '--workdir', str(project), '--only', 'resolvers',
                    '--out', str(store), '--version', '1.0.0'], env=environment, check=True, timeout=120)
    manifest = json.loads((store / 'packages-store-manifest.json').read_text())
    entries = manifest.get('packages', {}).get('resolvers', [])
    if manifest.get('schema') != 1 or manifest.get('platform') != 'linux-amd64' or len(entries) != 1:
        raise ValueError('Produced compiler did not export exactly one Linux amd64 resolver')
    entry = entries[0]
    archive = store / 'dockpipe-resolver-assets-only-1.0.0.tar.gz'
    expected = {'name': 'assets-only', 'version': '1.0.0', 'tarball': archive.name,
                'hidden': True, 'min_dockpipe_version': version}
    if any(entry.get(key) != value for key, value in expected.items()):
        raise ValueError('Produced compiler lost resolver identity or hidden/version metadata')
    if not archive.is_file() or archive.is_symlink() or entry.get('sha256') != digest(archive):
        raise ValueError('Produced resolver archive is missing or differs from its catalog')
    if entry.get('size_bytes') != archive.stat().st_size:
        raise ValueError('Produced resolver archive size differs from its catalog')
    with tarfile.open(archive, 'r:gz') as package:
        member = package.getmember('resolvers/assets-only/assets/payload.txt')
        if not member.isfile() or member.size != len(payload):
            raise ValueError('Produced resolver archive did not retain its authored asset')
        with package.extractfile(member) as stream:
            if stream.read(len(payload) + 1) != payload:
                raise ValueError('Produced resolver asset changed')


    # A neighboring hyphenated identity must never shadow the requested package.
    sibling = store / 'dockpipe-resolver-assets-only-neighbor-1.0.0.tar.gz'
    with tarfile.open(sibling, 'w:gz') as package:
        member = tarfile.TarInfo('resolvers/assets-only-neighbor/assets/payload.txt')
        member.size = len(payload)
        package.addfile(member, io.BytesIO(payload))

    consumer = root / 'consumer'
    consumer.mkdir()
    (consumer / 'dockpipe.config.json').write_text(json.dumps({
        'schema': 1, 'compile': {'workflows': []},
        'packages': {'sources': [{'kind': 'tarball_dir', 'path': str(store)}]},
    }) + '\n')
    scope = subprocess.run(
        [str(cli), 'scope', 'resolver', 'assets-only', 'assets-dir', '--workdir', str(consumer)],
        env=environment, check=True, timeout=120, capture_output=True, text=True,
    )
    assets = Path(scope.stdout.strip())
    if not assets.is_absolute() or not assets.is_dir() or (assets / 'payload.txt').read_bytes() != payload:
        raise ValueError('Produced compiler cannot consume the archived asset-only resolver')


def build(tags_path, output):
    root = Path(__file__).resolve().parents[3]
    source_sha = os.environ.get('GITHUB_SHA', '')
    run_id = os.environ.get('GITHUB_RUN_ID', '')
    attempt = os.environ.get('GITHUB_RUN_ATTEMPT', '')
    if not re.fullmatch(r'[0-9a-f]{40}', source_sha) or not re.fullmatch(r'[1-9][0-9]*', run_id) or not re.fullmatch(r'[1-9][0-9]*', attempt):
        raise ValueError('Compiler bootstrap requires exact GitHub run provenance')
    if os.environ.get('GITHUB_REF') != 'refs/heads/dev':
        raise ValueError('Compiler bootstrap requires the accepted dev branch')
    with tags_path.open('rb') as stream:
        history = stream.read(MAX_HISTORY_BYTES + 1)
    if len(history) > MAX_HISTORY_BYTES:
        raise ValueError('Release history exceeds 8 MiB')
    spec = importlib.util.spec_from_file_location('release_version', root / 'release/packaging/release-version.py')
    allocator = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(allocator)
    suffix = f'staging.{run_id}.{attempt}.{source_sha[:12]}'
    identity = allocator.metadata((root / 'VERSION').read_text().strip(), history.decode().splitlines(), suffix)
    output.mkdir(parents=True, exist_ok=False)
    with tempfile.TemporaryDirectory(prefix='compiler-build-', dir=output.parent) as temporary:
        work = Path(temporary)
        cli = work / 'dockpipe'
        environment = dict(os.environ, CGO_ENABLED='0', GOOS='linux', GOARCH='amd64',
                           GOAMD64='v1', GOTOOLCHAIN='local', GOFLAGS='', GOENV='off')
        subprocess.run(['go', 'build', '-trimpath', '-ldflags', f"-s -w -X main.Version={identity['version']}",
                        '-o', str(cli), './src/cmd'], cwd=root, env=environment, check=True, timeout=900)
        if not 0 < cli.stat().st_size <= MAX_EXECUTABLE_BYTES:
            raise ValueError('Produced CLI exceeds 256 MiB')
        archive = output / f"dockpipe_{identity['version']}_linux_amd64.tar.gz"
        with tarfile.open(archive, 'w:gz', format=tarfile.USTAR_FORMAT) as package:
            package.add(cli, arcname='dockpipe', recursive=False)
        if archive.stat().st_size > MAX_ARCHIVE_BYTES:
            raise ValueError('Produced archive exceeds the bootstrap download limit of 128 MiB')
        # Qualify the extracted delivery, rather than only the pre-archive binary.
        unpacked = work / 'unpacked'
        unpacked.mkdir()
        with tarfile.open(archive, 'r:gz') as package:
            first = package.next()
            if first is None or first.name != 'dockpipe' or not first.isfile() or first.linkname or package.next() is not None:
                raise ValueError('CLI archive must contain exactly one regular dockpipe executable')
            executable = unpacked / 'dockpipe'
            with package.extractfile(first) as source, executable.open('xb') as destination:
                while block := source.read(1 << 20):
                    destination.write(block)
            executable.chmod(0o755)
        version_output = subprocess.run([str(executable), '--version'], check=True, timeout=15,
                                        capture_output=True, text=True)
        if version_output.stdout.strip() != identity['version']:
            raise ValueError('Delivered CLI reports a different version')
        verify_compiler(executable, identity['version'], work / 'qualification')
    with (root / 'LICENSE').open('rb') as source:
        license_data = source.read((64 << 10) + 1)
    if not license_data or len(license_data) > 64 << 10:
        raise ValueError('Compiler license notice is missing or exceeds 64 KiB')
    license_path = output / 'LICENSE.txt'
    license_path.write_bytes(license_data)
    manifest = {
        'schema': 1, 'release_scope': 'compiler-bootstrap', 'version': identity['version'],
        'release_tag': identity['tag'], 'source_sha': source_sha, 'platform': 'linux-amd64',
        'license': {'name': license_path.name, 'sha256': digest(license_path), 'bytes': license_path.stat().st_size},
        'archive': {'name': archive.name, 'sha256': digest(archive), 'bytes': archive.stat().st_size},
        'qualification': {'asset_only_resolver_archive': True, 'asset_only_resolver_scope': True, 'exact_resolver_identity': True, 'hidden_metadata': True},
    }
    manifest_path = output / 'compiler-bootstrap.json'
    manifest_path.write_text(json.dumps(manifest, indent=2) + '\n')
    (output / 'SHA256SUMS.txt').write_text(''.join(f'{digest(path)}  {path.name}\n' for path in [archive, manifest_path, license_path]))
    (output / 'RELEASE_NOTES.md').write_text(
        f"Compiler bootstrap {identity['candidate']}\n\n"
        'Linux amd64 CLI only, verified through its delivered archive. This prerelease '
        'does not constitute a full DockPipe release. No package store, desktop, APT, '
        'SDK archive, or shared channel pointer is published.\n'
    )


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--tags', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    build(args.tags, args.out)
