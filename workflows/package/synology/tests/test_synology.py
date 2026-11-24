"""Check SPK structure, architecture admission, deterministic output and lifecycle safety."""
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import struct
import subprocess
import tarfile
import tempfile
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[4]
SCRIPT = ROOT / 'workflows/package/synology/assets/scripts/build.py'
SPECIFICATION = importlib.util.spec_from_file_location('synology_build', SCRIPT)
PACKAGING = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(PACKAGING)


class SynologyTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix='synology package spaces ')
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)

    def binary(self, architecture, program_type=1):
        content = bytearray(120)
        content[:7] = b'\x7fELF\x02\x01\x01'
        struct.pack_into('<HH', content, 16, 2, PACKAGING.ARCHITECTURES[architecture][2])
        struct.pack_into('<Q', content, 32, 64)
        struct.pack_into('<HH', content, 54, 56, 1)
        struct.pack_into('<I', content, 64, program_type)
        binary = self.root / ('dockpipe-' + architecture)
        binary.write_bytes(content)
        return binary

    def test_both_architectures_have_cli_and_unprivileged_dsm_metadata(self):
        for architecture, expected_arch in (('amd64', 'x86_64'), ('arm64', 'armv8')):
            with self.subTest(architecture=architecture):
                binary = self.binary(architecture)
                package = PACKAGING.build('0.6.34', architecture, binary, self.root)
                with tarfile.open(package) as archive:
                    members = archive.getmembers()
                    self.assertTrue(all(member.isfile() for member in members))
                    info = archive.extractfile('INFO').read().decode()
                    self.assertIn(f'arch="{expected_arch}"', info)
                    self.assertIn('version="0.6.34-1"', info)
                    self.assertIn('os_min_ver="7.0-40000"', info)
                    self.assertIn('ctl_stop="no"', info)
                    self.assertNotIn('adminport=', info)
                    privilege = json.load(archive.extractfile('conf/privilege'))
                    self.assertEqual(privilege, {'defaults': {'run-as': 'package'}})
                    payload_bytes = archive.extractfile('package.tgz').read()
                    checksum = hashlib.md5(payload_bytes, usedforsecurity=False).hexdigest()
                    self.assertIn(f'checksum="{checksum}"', info)
                    with tarfile.open(fileobj=io.BytesIO(payload_bytes), mode='r:gz') as payload:
                        self.assertEqual(payload.getnames(), ['bin/dockpipe', 'share/doc/dockpipe/LICENSE'])
                        self.assertEqual(payload.getmember('bin/dockpipe').mode, 0o755)
                        self.assertEqual(payload.extractfile('bin/dockpipe').read(), binary.read_bytes())
                        self.assertEqual(payload.extractfile('share/doc/dockpipe/LICENSE').read(),
                                         (ROOT / 'LICENSE').read_bytes())
                    for member in members:
                        if not member.name.startswith('scripts/'):
                            continue
                        self.assertEqual(member.mode, 0o755)
                        script = archive.extractfile(member).read()
                        subprocess.run(['sh', '-n'], input=script, check=True)
                        subprocess.run(['sh', '-s', '--', 'status'], input=script, check=True)
                    # DSM 7 requires 64px and 256px package icons.
                    for name, dimension in (('PACKAGE_ICON.PNG', 64), ('PACKAGE_ICON_256.PNG', 256)):
                        icon = archive.extractfile(name).read()
                        self.assertEqual(struct.unpack('>II', icon[16:24]), (dimension, dimension))

    def test_build_is_reproducible_despite_input_mtime_and_permissions(self):
        binary = self.binary('amd64')
        first = PACKAGING.build('0.6.34', 'amd64', binary, self.root).read_bytes()
        binary.touch()
        binary.chmod(0o700)
        second = PACKAGING.build('0.6.34', 'amd64', binary, self.root).read_bytes()
        self.assertEqual(first, second)

    def test_invalid_inputs_do_not_replace_existing_package(self):
        binary = self.binary('amd64')
        destination = PACKAGING.build('0.6.34', 'amd64', binary, self.root)
        expected = destination.read_bytes()
        for content in (b'not ELF', binary.read_bytes()[:64]):
            binary.write_bytes(content)
            with self.assertRaises(ValueError):
                PACKAGING.build('0.6.34', 'amd64', binary, self.root)
            self.assertEqual(destination.read_bytes(), expected)
        self.assertFalse(list(self.root.glob('.synology-*')))

    def test_wrong_architecture_dynamic_binary_symlink_and_invalid_version_fail(self):
        binary = self.binary('amd64')
        with self.assertRaisesRegex(ValueError, 'architecture'):
            PACKAGING.build('0.6.34', 'arm64', binary, self.root)
        dynamic = self.binary('amd64', 3)
        with self.assertRaisesRegex(ValueError, 'dynamic interpreter'):
            PACKAGING.build('0.6.34', 'amd64', dynamic, self.root)
        dynamic = self.binary('amd64', 2)
        for libraries in ('', ' (NEEDED) Shared library: [libunexpected.so]'):
            result = subprocess.CompletedProcess([], 0, libraries)
            with patch.object(PACKAGING.subprocess, 'run', return_value=result):
                with self.assertRaisesRegex(ValueError, 'dynamic libraries'):
                    PACKAGING.build('0.6.34', 'amd64', dynamic, self.root)
        result = subprocess.CompletedProcess([], 0, ' (NEEDED) Shared library: [libc.so.6]')
        with patch.object(PACKAGING.subprocess, 'run', return_value=result):
            PACKAGING.build('0.6.34', 'amd64', dynamic, self.root)
        link = self.root / 'link'
        link.symlink_to(binary)
        with self.assertRaisesRegex(ValueError, 'regular file'):
            PACKAGING.build('0.6.34', 'amd64', link, self.root)
        for version in ('../0.6.34', '0.6.34\n', '0.6.34-staging.1', '0.6.2147483648'):
            with self.assertRaises(ValueError):
                PACKAGING.build(version, 'amd64', binary, self.root)

    def test_release_build_and_both_publishers_include_spks(self):
        phase = (ROOT / 'workflows/build/platform/assets/scripts/package-linux.sh').read_text()
        self.assertIn('workflows/package/synology/assets/scripts/build.py', phase)
        self.assertIn('--binary "$PLATFORM_BINARY" --output "$PLATFORM_OUTPUT"', phase)
        workflow = (ROOT / '.github/workflows/release.yml').read_text()
        self.assertEqual(workflow.count('release/artifacts/*.spk'), 2)

    def test_archive_failure_preserves_existing_package_and_removes_temporary_files(self):
        binary = self.binary('amd64')
        destination = PACKAGING.build('0.6.34', 'amd64', binary, self.root)
        expected = destination.read_bytes()
        with patch.object(PACKAGING, 'add_file', side_effect=OSError('fixture write failed')):
            with self.assertRaisesRegex(OSError, 'fixture write failed'):
                PACKAGING.build('0.6.34', 'amd64', binary, self.root)
        self.assertEqual(destination.read_bytes(), expected)
        self.assertFalse(list(self.root.glob('.synology-*')))

    def test_dynamic_inspection_timeout_does_not_create_output(self):
        binary = self.binary('amd64', 2)
        output = self.root / 'output'
        with patch.object(PACKAGING.subprocess, 'run', side_effect=subprocess.TimeoutExpired('readelf', 30)):
            with self.assertRaises(subprocess.TimeoutExpired):
                PACKAGING.build('0.6.34', 'amd64', binary, output)
        self.assertFalse(output.exists())


if __name__ == '__main__':
    unittest.main()
