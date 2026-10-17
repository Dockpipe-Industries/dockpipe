#!/usr/bin/env python3
"""Run DorkPipe's repository summary only from a verified, selected resolver."""

import hashlib
import io
import json
import os
from pathlib import Path
import platform
import re
import stat
import subprocess
import sys
import tarfile
import tempfile
from urllib.parse import urlsplit
from urllib.request import HTTPRedirectHandler, Request, build_opener


MAX_LOCK_BYTES = 64 << 10
MAX_CATALOG_BYTES = 8 << 20
MAX_ARCHIVE_BYTES = 512 << 20
MAX_SCRIPT_BYTES = 1 << 20
SCRIPT_MEMBERS = {
    'resolver': 'resolvers/dorkpipe/assets/scripts/repo-ci-summary.sh',
    'workflow': 'workflows/dorkpipe/assets/scripts/repo-ci-summary.sh',
}
MAX_ARCHIVE_MEMBERS = 32768
SHA256 = re.compile(r'[0-9a-f]{64}\Z')
VERSION = re.compile(r'[0-9]+\.[0-9]+\.[0-9]+\Z')
ALLOWED_HOSTS = {'packages.staging.dockpipe.com', 'packages.dockpipe.com'}


def reject_duplicate_keys(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('duplicate JSON key')
        result[key] = value
    return result


def read_bounded_json(path, maximum):
    if path.is_symlink() or not path.is_file() or path.stat().st_size > maximum:
        raise ValueError('source lock is missing or too large')
    with path.open('rb') as stream:
        data = stream.read(maximum + 1)
    if len(data) > maximum:
        raise ValueError('source lock grew beyond capacity')
    return json.loads(data, object_pairs_hook=reject_duplicate_keys)


def current_platform():
    systems = {'linux': 'linux', 'darwin': 'darwin', 'win32': 'windows'}
    machines = {'x86_64': 'amd64', 'AMD64': 'amd64', 'aarch64': 'arm64', 'arm64': 'arm64'}
    system = systems.get(sys.platform)
    machine = machines.get(platform.machine())
    if system is None or machine is None:
        raise ValueError('no qualified package platform for this host')
    if system == 'linux' and (os.environ.get('FLATPAK_ID') or Path('/.flatpak-info').exists()):
        with Path('/.flatpak-info').open('rb') as stream:
            info = stream.read(65537)
        if len(info) > 65536:
            raise ValueError('Flatpak runtime metadata exceeds capacity')
        section = ''
        runtime = None
        for line in info.decode('utf-8').splitlines():
            line = line.strip()
            if line.startswith('[') and line.endswith(']'):
                section = line
            elif section == '[Application]' and line.startswith('runtime='):
                if runtime is not None:
                    raise ValueError('duplicate Flatpak runtime')
                runtime = line.partition('=')[2].removeprefix('runtime/').split('/')
        if runtime is None or len(runtime) != 3 or runtime[1] != {'amd64': 'x86_64', 'arm64': 'aarch64'}[machine]:
            raise ValueError('invalid Flatpak runtime metadata')
        if not all(re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._+-]*', item) for item in (runtime[0], runtime[2])):
            raise ValueError('invalid Flatpak runtime identity')
        return f'linux-{machine}-flatpak-{runtime[0]}-{runtime[2]}'
    return f'{system}-{machine}'


def validate_catalog_url(raw):
    if not isinstance(raw, str):
        raise ValueError('resolver catalog URL is missing')
    parsed = urlsplit(raw)
    if (parsed.scheme != 'https' or parsed.hostname not in ALLOWED_HOSTS
            or parsed.netloc != parsed.hostname or parsed.query or parsed.fragment
            or not parsed.path.endswith('/packages-store-manifest.json')):
        raise ValueError('resolver catalog must be an immutable package-host store URL')
    parts = parsed.path.split('/')
    if (any(not part or part in {'.', '..', 'latest', 'current', 'main', 'dev', 'staging'}
            for part in parts[1:]) or '%' in parsed.path or '\\' in parsed.path):
        raise ValueError('resolver catalog path is unsafe or mutable')
    return parsed


class SameOriginRedirect(HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, message, headers, new_url):
        original = urlsplit(request.full_url)
        target = urlsplit(new_url)
        if target.scheme != 'https' or target.netloc != original.netloc:
            raise ValueError('resolver catalog redirect changed origin')
        validate_catalog_url(new_url)
        return super().redirect_request(request, fp, code, message, headers, new_url)


def fetch_catalog(url):
    opener = build_opener(SameOriginRedirect())
    request = Request(url, headers={'User-Agent': 'dockpipe/repo-quality-pin'})
    with opener.open(request, timeout=20) as response:
        data = response.read(MAX_CATALOG_BYTES + 1)
    if len(data) > MAX_CATALOG_BYTES:
        raise ValueError('resolver catalog exceeds capacity')
    return data


def select_entry(lock, platform_name, fetcher=fetch_catalog):
    if not isinstance(lock, dict) or set(lock) != {'schema', 'targets'} or lock['schema'] != 1:
        raise ValueError('unsupported repository-quality source lock')
    targets = lock['targets']
    if not isinstance(targets, dict) or platform_name not in targets:
        raise ValueError(f'no published DorkPipe package pin for {platform_name}')
    selection = targets[platform_name]
    required = {'catalog_url', 'catalog_sha256', 'kind', 'version', 'archive_sha256',
                'archive_size_bytes', 'script_sha256'}
    if not isinstance(selection, dict) or set(selection) != required:
        raise ValueError('incomplete DorkPipe package pin')
    validate_catalog_url(selection['catalog_url'])
    if (not isinstance(selection['kind'], str) or selection['kind'] not in SCRIPT_MEMBERS
            or not isinstance(selection['version'], str) or not VERSION.fullmatch(selection['version'])
            or any(not isinstance(selection[field], str) or not SHA256.fullmatch(selection[field])
                   for field in ('catalog_sha256', 'archive_sha256', 'script_sha256'))
            or type(selection['archive_size_bytes']) is not int
            or not 0 < selection['archive_size_bytes'] <= MAX_ARCHIVE_BYTES):
        raise ValueError('DorkPipe package requires exact kind, version, hashes and bounded size')
    data = fetcher(selection['catalog_url'])
    if len(data) > MAX_CATALOG_BYTES or hashlib.sha256(data).hexdigest() != selection['catalog_sha256']:
        raise ValueError('resolver catalog differs from its source lock')
    catalog = json.loads(data, object_pairs_hook=reject_duplicate_keys)
    if (not isinstance(catalog, dict) or catalog.get('schema') != 1
            or catalog.get('platform') != platform_name):
        raise ValueError('resolver catalog platform or schema differs')
    packages = catalog.get('packages')
    collection = selection['kind'] + 's'
    if not isinstance(packages, dict) or not isinstance(packages.get(collection), list):
        raise ValueError('resolver catalog is incomplete')
    matches = [entry for entry in packages[collection]
               if isinstance(entry, dict) and entry.get('name') == 'dorkpipe']
    if len(matches) != 1:
        raise ValueError('resolver catalog needs one selected DorkPipe entry')
    entry = matches[0]
    if (entry.get('version') != selection['version']
            or entry.get('sha256') != selection['archive_sha256']
            or entry.get('size_bytes') != selection['archive_size_bytes']
            or entry.get('tarball') != f'dockpipe-{selection["kind"]}-dorkpipe-{selection["version"]}.tar.gz'):
        raise ValueError('resolver catalog entry differs from its source lock')
    return selection


def sha256_file(path, expected_size):
    if path.is_symlink():
        raise ValueError('selected resolver archive is a link')
    # O_NOFOLLOW is unavailable on Windows; the explicit link check still
    # rejects ordinary linked inputs on that platform.
    descriptor = os.open(path, os.O_RDONLY | getattr(os, 'O_NOFOLLOW', 0))
    digest = hashlib.sha256()
    with os.fdopen(descriptor, 'rb') as stream:
        info = os.fstat(stream.fileno())
        if not stat.S_ISREG(info.st_mode) or info.st_size != expected_size:
            raise ValueError('selected resolver archive size differs')
        total = 0
        while block := stream.read(1 << 20):
            total += len(block)
            if total > expected_size:
                raise ValueError('selected resolver archive grew')
            digest.update(block)
        if total != expected_size:
            raise ValueError('selected resolver archive was truncated')
    return digest.hexdigest()


def archive_script_bytes(archive, kind):
    member_name = SCRIPT_MEMBERS[kind]
    with tarfile.open(archive, 'r:gz') as source:
        members = []
        expanded = 0
        count = 0
        for member in source:
            count += 1
            expanded += member.size
            if count > MAX_ARCHIVE_MEMBERS or expanded > 1 << 30:
                raise ValueError('resolver archive exceeds inspection capacity')
            if member.name == member_name:
                members.append(member)
        if len(members) != 1 or not members[0].isfile() or members[0].size > MAX_SCRIPT_BYTES:
            raise ValueError('selected resolver lacks one bounded summary script')
        with source.extractfile(members[0]) as stream:
            data = stream.read(MAX_SCRIPT_BYTES + 1)
    if len(data) != members[0].size:
        raise ValueError('selected summary script size differs')
    return data


def verified_cache_store(scope_root, digest, kind):
    cache_root = scope_root / 'verified-archives'
    store = cache_root / digest
    directory = store / (kind + 's')
    for path in (cache_root, store, directory):
        if path.is_symlink():
            raise ValueError('verified archive cache contains a link')
        path.mkdir(mode=0o700, exist_ok=True)
        if not path.is_dir() or not path.resolve(strict=True).is_relative_to(scope_root):
            raise ValueError('verified archive cache escaped package scope')
    return store, directory


def publish_archive(source, destination, expected_size, expected_digest):
    descriptor, temporary_name = tempfile.mkstemp(prefix='.incoming-', dir=destination.parent)
    temporary = Path(temporary_name)
    try:
        digest = hashlib.sha256()
        total = 0
        with os.fdopen(descriptor, 'wb') as writer, source.open('rb') as reader:
            while block := reader.read(1 << 20):
                total += len(block)
                if total > expected_size:
                    raise ValueError('selected package archive grew during cache publication')
                digest.update(block)
                writer.write(block)
            writer.flush()
            os.fsync(writer.fileno())
        if total != expected_size or digest.hexdigest() != expected_digest:
            raise ValueError('selected package archive changed during cache publication')
        if destination.exists() or destination.is_symlink():
            if sha256_file(destination, expected_size) != expected_digest:
                raise ValueError('verified archive cache was changed')
        else:
            os.replace(temporary, destination)
    finally:
        temporary.unlink(missing_ok=True)


def cli_output(command, environment, timeout=60):
    result = subprocess.run(command, env=environment, check=True, text=True,
                            stdout=subprocess.PIPE, timeout=timeout)
    return result.stdout.strip()


def run(lock_path, dockpipe_bin, workdir, platform_name=None, fetcher=fetch_catalog):
    lock = read_bounded_json(lock_path, MAX_LOCK_BYTES)
    selection = select_entry(lock, platform_name or current_platform(), fetcher)
    dockpipe_bin = dockpipe_bin.resolve(strict=True)
    workdir = workdir.resolve(strict=True)
    if not dockpipe_bin.is_file() or not os.access(dockpipe_bin, os.X_OK):
        raise ValueError('DockPipe CLI is unavailable')
    environment = os.environ.copy()
    scope = cli_output([str(dockpipe_bin), '__state', 'package-runtime',
                        '--workdir', str(workdir), '--owner', 'dockpipe-repo-quality',
                        '--ensure-private'], environment)
    scope_root = Path(scope).resolve(strict=True)
    if not scope_root.is_dir():
        raise ValueError('repository-quality package runtime is unavailable')
    with tempfile.TemporaryDirectory(prefix='selected-resolver-', dir=scope_root) as temporary:
        private = Path(temporary)
        store, cache_directory = verified_cache_store(
            scope_root, selection['archive_sha256'], selection['kind'])
        filename = f'dockpipe-{selection["kind"]}-dorkpipe-{selection["version"]}.tar.gz'
        cached_archive = cache_directory / filename
        private_global = private / 'global'
        private_global.mkdir()
        environment.update({'XDG_DATA_HOME': str(private / 'data'),
                            'DOCKPIPE_GLOBAL_ROOT': str(private_global),
                            'DOCKPIPE_SYSTEM_ROOT': str(private / 'system-empty')})
        if not cached_archive.exists() and not cached_archive.is_symlink():
            output = cli_output([str(dockpipe_bin), 'package', 'install', '--remote',
                                 selection['catalog_url'], '--kind', selection['kind'], '--name',
                                 'dorkpipe', '--sha256', selection['archive_sha256']],
                                environment, timeout=310)
            installed = json.loads(output, object_pairs_hook=reject_duplicate_keys)
            if not isinstance(installed, dict) or set(installed) != {'path'}:
                raise ValueError('package install did not return one archive path')
            archive = Path(installed['path'])
            if (not archive.is_absolute() or archive.parent.name != selection['kind'] + 's'
                    or archive.name != filename or not archive.is_relative_to(private_global)):
                raise ValueError('package install path escaped the isolated package store')
            if sha256_file(archive, selection['archive_size_bytes']) != selection['archive_sha256']:
                raise ValueError('installed package archive differs from its source lock')
            publish_archive(archive, cached_archive, selection['archive_size_bytes'],
                            selection['archive_sha256'])
        if sha256_file(cached_archive, selection['archive_size_bytes']) != selection['archive_sha256']:
            raise ValueError('verified archive cache differs from its source lock')
        script_bytes = archive_script_bytes(cached_archive, selection['kind'])
        if hashlib.sha256(script_bytes).hexdigest() != selection['script_sha256']:
            raise ValueError('archived summary script differs from its source lock')
        environment['DOCKPIPE_PACKAGES_ROOT'] = str(store)
        script = private / 'repo-ci-summary.sh'
        with script.open('xb') as target:
            target.write(script_bytes)
        if sha256_file(script, len(script_bytes)) != selection['script_sha256']:
            raise ValueError('materialized summary script differs from selected archive')
        environment['PATH'] = str(dockpipe_bin.parent) + os.pathsep + environment.get('PATH', '')
        subprocess.run(['bash', str(script)], cwd=workdir, env=environment,
                       check=True, timeout=120)


def main():
    lock_path = Path(__file__).resolve().parents[1] / 'source-lock.json'
    dockpipe = os.environ.get('DOCKPIPE_BIN')
    if not dockpipe:
        raise ValueError('DOCKPIPE_BIN must select the running DockPipe CLI')
    run(lock_path, Path(dockpipe), Path.cwd())


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, subprocess.SubprocessError, json.JSONDecodeError) as error:
        print(f'Repository quality pin failed: {error}', file=sys.stderr)
        raise SystemExit(1)
