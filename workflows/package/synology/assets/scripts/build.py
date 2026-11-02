#!/usr/bin/env python3
"""Build a reproducible DSM SPK from a Linux CLI, without host installation."""
import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import struct
import subprocess
import tarfile
import tempfile


ARCHITECTURES = {
    'amd64': ('x64', 'x86_64', 62),
    'arm64': ('arm64', 'armv8', 183),
}
ROOT = Path(__file__).resolve().parents[5]
INTERPRETERS = {
    'amd64': b'/lib64/ld-linux-x86-64.so.2\0',
    'arm64': b'/lib/ld-linux-aarch64.so.1\0',
}


def validate_binary(binary, architecture):
    """Check ELF identity and restrict dynamic dependencies to the glibc runtime."""
    if binary.is_symlink() or not binary.is_file():
        raise ValueError('The CLI must be a regular file, not a symlink')
    with binary.open('rb') as stream:
        header = stream.read(64)
        if len(header) != 64 or header[:7] != b'\x7fELF\x02\x01\x01':
            raise ValueError('Expected a little-endian ELF64 Linux CLI')
        executable_type, machine = struct.unpack_from('<HH', header, 16)
        if executable_type not in (2, 3) or machine != ARCHITECTURES[architecture][2]:
            raise ValueError('CLI architecture does not match the requested SPK')
        offset = struct.unpack_from('<Q', header, 32)[0]
        entry_size, count = struct.unpack_from('<HH', header, 54)
        size = os.fstat(stream.fileno()).st_size
        if not count or entry_size != 56 or offset < 64 or offset + entry_size * count > size:
            raise ValueError('Invalid ELF program header table')
        # ELF's 16-bit entry count bounds the scan; read one fixed-size entry at a time.
        dynamic = False
        for _ in range(count):
            stream.seek(offset)
            entry = stream.read(entry_size)
            offset += entry_size
            program_type = struct.unpack_from('<I', entry)[0]
            if program_type == 2:
                dynamic = True
            if program_type == 3:
                interpreter_offset = struct.unpack_from('<Q', entry, 8)[0]
                interpreter_size = struct.unpack_from('<Q', entry, 32)[0]
                expected = INTERPRETERS[architecture]
                if interpreter_size != len(expected) or interpreter_offset + interpreter_size > size:
                    raise ValueError('Unsupported ELF dynamic interpreter')
                stream.seek(interpreter_offset)
                if stream.read(interpreter_size) != expected:
                    raise ValueError('Unsupported ELF dynamic interpreter')
    if dynamic:
        # purego native authentication uses dlopen even with CGO_ENABLED=0.
        # Keep this runtime dependency visible rather than assuming CGO-free means static.
        result = subprocess.run(['readelf', '--dynamic', '--wide', '--', str(binary)],
                                capture_output=True, text=True, check=True, timeout=30)
        libraries = re.findall(r'\(NEEDED\).*\[([^\]]+)\]', result.stdout)
        if not libraries or set(libraries) - {'libc.so.6', 'libdl.so.2', 'libpthread.so.0'}:
            raise ValueError('Unsupported ELF dynamic libraries for DSM')


def add_bytes(archive, name, content, mode=0o644):
    member = tarfile.TarInfo(name)
    member.size = len(content)
    member.mode = mode
    archive.addfile(member, io.BytesIO(content))


def add_file(archive, name, path, mode=0o644):
    member = tarfile.TarInfo(name)
    member.size = path.stat().st_size
    member.mode = mode
    with path.open('rb') as stream:
        archive.addfile(member, stream)


def build(version, architecture, binary, output):
    if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', version):
        raise ValueError('Expected a numeric release version')
    if any(int(part) > 2**31 - 1 for part in version.split('.')):
        raise ValueError('Version exceeds DSM numeric field limits')
    validate_binary(binary, architecture)
    label, dsm_architecture, _ = ARCHITECTURES[architecture]
    output.mkdir(parents=True, exist_ok=True)
    destination = output / f'dockpipe_{version}_synology_{label}.spk'
    with tempfile.TemporaryDirectory(prefix='.synology-', dir=output) as temporary:
        temporary = Path(temporary)
        payload = temporary / 'package.tgz'
        with payload.open('wb') as stream:
            with gzip.GzipFile(filename='', mode='wb', fileobj=stream, mtime=0) as compressed:
                with tarfile.open(fileobj=compressed, mode='w', format=tarfile.USTAR_FORMAT) as archive:
                    add_file(archive, 'bin/dockpipe', binary, 0o755)
                    add_file(archive, 'share/doc/dockpipe/LICENSE', ROOT / 'LICENSE')
        checksum = hashlib.md5(usedforsecurity=False)
        with payload.open('rb') as stream:
            for block in iter(lambda: stream.read(1024 * 1024), b''):
                checksum.update(block)
        fields = {
            'package': 'dockpipe',
            'displayname': 'Dockpipe',
            'version': version + '-1',
            'arch': dsm_architecture,
            'os_min_ver': '7.0-40000',
            'maintainer': 'Dockpipe Industries',
            'maintainer_url': 'https://dockpipe.com',
            'description': (
                'Dockpipe CLI and remote worker. Use over SSH or from the Dockpipe desktop app. '
                'No DSM web GUI or automatic worker service is included.'
            ),
            'thirdparty': 'yes',
            'ctl_stop': 'no',
            'silent_install': 'yes',
            'silent_upgrade': 'yes',
            'silent_uninstall': 'yes',
            'checksum': checksum.hexdigest(),
        }
        info = ''.join(f'{key}="{value}"\n' for key, value in fields.items()).encode()
        # DSM owns extraction/removal. Scripts never alter user homes or global paths.
        noop = b'#!/bin/sh\nexit 0\n'
        lifecycle = b'''#!/bin/sh
case "$1" in
    start|stop|status|prestart|prestop)
        exit 0
        ;;
    *)
        exit 1
        ;;
esac
'''
        package = temporary / destination.name
        with tarfile.open(package, 'w', format=tarfile.USTAR_FORMAT) as archive:
            add_bytes(archive, 'INFO', info)
            add_file(archive, 'package.tgz', payload)
            privilege = {'defaults': {'run-as': 'package'}}
            add_bytes(archive, 'conf/privilege', (json.dumps(privilege, indent=2) + '\n').encode())
            for name in ('preinst', 'postinst', 'preuninst', 'postuninst', 'preupgrade', 'postupgrade'):
                add_bytes(archive, 'scripts/' + name, noop, 0o755)
            add_bytes(archive, 'scripts/start-stop-status', lifecycle, 0o755)
            icons = ROOT / 'src/dockpipe-app/resources/icons/hicolor'
            add_file(archive, 'PACKAGE_ICON.PNG', icons / '64x64/apps/dockpipe-launcher.png')
            add_file(archive, 'PACKAGE_ICON_256.PNG', icons / '256x256/apps/dockpipe-launcher.png')
        package.replace(destination)
    return destination


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('version')
    parser.add_argument('architecture', choices=ARCHITECTURES)
    parser.add_argument('--binary', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    arguments = parser.parse_args()
    try:
        destination = build(arguments.version, arguments.architecture, arguments.binary, arguments.output)
    except (ValueError, OSError, subprocess.SubprocessError) as error:
        parser.exit(1, f'Synology packaging failed: {error}\n')
    print('Built ' + str(destination))


if __name__ == '__main__':
    main()
