#!/usr/bin/env python3
"""Install the separately released workflow runner selected by the repository lock."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import tarfile
import tempfile
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parents[2]
RELEASE_ORIGIN = 'https://github.com/Dockpipe-Industries/dockpipe/releases/download/'
MAX_ARCHIVE_BYTES = 64 * 1024 * 1024


def selected_artifact(root, system, machine):
    lock = json.loads((root / 'workflows/shared/bootstrap.lock.json').read_text())
    runner = lock['dockpipe']
    if lock.get('schema') != 1 or not isinstance(runner.get('version'), str):
        raise ValueError('The workflow bootstrap requires its explicit released version pin')
    architecture = {'x86_64': 'amd64', 'amd64': 'amd64', 'aarch64': 'arm64', 'arm64': 'arm64'}.get(machine.lower())
    target = system.lower() + '/' + str(architecture)
    artifact = runner['artifacts'].get(target)
    if artifact is None:
        raise ValueError('No released workflow bootstrap is locked for ' + target)
    prefix = RELEASE_ORIGIN + runner['release'] + '/'
    if not artifact['url'].startswith(prefix) or '/' in artifact['url'][len(prefix):]:
        raise ValueError('The bootstrap must name an exact official release asset')
    expected_name = 'dockpipe.exe' if system.lower() == 'windows' else 'dockpipe'
    if artifact['executable'] != expected_name:
        raise ValueError('Unexpected bootstrap executable name')
    if not re.fullmatch('[0-9a-f]{64}', artifact['sha256']):
        raise ValueError('The bootstrap requires an exact archive checksum')
    for field in ('size_bytes', 'executable_size_bytes'):
        if type(artifact[field]) is not int or not 0 < artifact[field] <= MAX_ARCHIVE_BYTES:
            raise ValueError('The bootstrap requires a bounded positive ' + field)
    return runner['version'], artifact


def install(root, output, archive=None, *, report=True):
    version, artifact = selected_artifact(root, platform.system(), platform.machine())
    output = output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    destination = output / artifact['executable']
    if destination.exists() or destination.is_symlink():
        raise ValueError('Use a fresh bootstrap output directory')
    with tempfile.TemporaryDirectory(prefix='bootstrap-', dir=output) as temporary:
        temporary = Path(temporary)
        payload = temporary / 'archive'
        if archive is not None:
            if not archive.is_file() or archive.stat().st_size != artifact['size_bytes']:
                raise ValueError('Bootstrap archive size differs from its lock')
            payload.write_bytes(archive.read_bytes())
        else:
            with urllib.request.urlopen(artifact['url'], timeout=60) as response, payload.open('xb') as target:
                remaining = artifact['size_bytes']
                while remaining:
                    block = response.read(min(1024 * 1024, remaining))
                    if not block:
                        raise ValueError('Bootstrap archive was truncated')
                    target.write(block)
                    remaining -= len(block)
                if response.read(1):
                    raise ValueError('Bootstrap archive exceeds its locked size')
        if hashlib.sha256(payload.read_bytes()).hexdigest() != artifact['sha256']:
            raise ValueError('Bootstrap archive checksum differs from its lock')
        if artifact['executable'].endswith('.exe'):
            with zipfile.ZipFile(payload) as bundle:
                members = bundle.infolist()
                if len(members) != 1 or members[0].filename != artifact['executable']:
                    raise ValueError('Bootstrap archive must contain only its executable')
                if members[0].file_size != artifact['executable_size_bytes']:
                    raise ValueError('Bootstrap executable size differs from its lock')
                data = bundle.read(members[0])
        else:
            with tarfile.open(payload) as bundle:
                members = bundle.getmembers()
                if len(members) != 1 or members[0].name != artifact['executable'] or not members[0].isfile():
                    raise ValueError('Bootstrap archive must contain only its regular executable')
                if members[0].size != artifact['executable_size_bytes']:
                    raise ValueError('Bootstrap executable size differs from its lock')
                with bundle.extractfile(members[0]) as source:
                    data = source.read()
        candidate = temporary / artifact['executable']
        candidate.write_bytes(data)
        candidate.chmod(0o755)
        actual = subprocess.check_output([str(candidate), '--version'], text=True, timeout=15).strip()
        if actual != version:
            raise ValueError('Bootstrap executable version differs from its lock')
        candidate.rename(destination)
    if report:
        environment_file = os.environ.get('GITHUB_ENV')
        if environment_file:
            with open(environment_file, 'a', encoding='utf-8') as environment:
                environment.write('DOCKPIPE_BOOTSTRAP_BIN=' + str(destination) + '\n')
        print(destination)
    return destination


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--archive', type=Path, help='Use an already downloaded locked archive')
    arguments = parser.parse_args()
    install(ROOT, arguments.output, arguments.archive)


if __name__ == '__main__':
    main()
