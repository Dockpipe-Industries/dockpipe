"""Pinned archive admission and mandatory native-test evidence without downloads."""
import hashlib
import importlib.util
import io
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest import mock


SCRIPT = Path(__file__).resolve().parents[1] / 'assets/scripts/qualify.py'
SPEC = importlib.util.spec_from_file_location('installed_cockpit_qualification', SCRIPT)
QUALIFICATION = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(QUALIFICATION)


class InstalledCockpitTests(unittest.TestCase):
    def archive(self, root, extra=None, missing_reader=False):
        archive = root / 'dockpipe-resolver-cockpit-0.1.0.tar.gz'
        members = [
            ('resolvers/cockpit/assets/ui/libdockpipe-cockpit.so', b'library', tarfile.REGTYPE),
        ]
        if not missing_reader:
            members.append(('resolvers/cockpit/assets/ui/cockpit-reader', b'reader', tarfile.REGTYPE))
        if extra:
            members.append(extra)
        with tarfile.open(archive, 'w:gz') as stream:
            for name, payload, kind in members:
                entry = tarfile.TarInfo(name)
                entry.type = kind
                entry.mode = 0o755
                entry.size = len(payload) if kind == tarfile.REGTYPE else 0
                if kind == tarfile.SYMTYPE:
                    entry.linkname = '/outside'
                stream.addfile(entry, io.BytesIO(payload) if kind == tarfile.REGTYPE else None)
        entry = {'name': 'cockpit', 'version': '0.1.0', 'tarball': archive.name,
                 'sha256': hashlib.sha256(archive.read_bytes()).hexdigest(), 'size_bytes': archive.stat().st_size}
        return {'schema': 1, 'platform': 'linux-amd64', 'packages': {'resolvers': [entry], 'workflows': []}}

    def test_verified_package_extracts_only_its_installed_assets(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            manifest = self.archive(root)
            assets, entry = QUALIFICATION.extract_cockpit(root, manifest, root / 'installed')
            self.assertEqual((assets / 'ui/cockpit-reader').read_bytes(), b'reader')
            self.assertEqual(entry, manifest['packages']['resolvers'][0])
            self.assertTrue((assets / 'ui/cockpit-reader').stat().st_mode & 0o111)

    def test_corruption_fails_before_extracting(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            manifest = self.archive(root)
            archive = root / manifest['packages']['resolvers'][0]['tarball']
            archive.write_bytes(b'corrupted')
            with self.assertRaisesRegex(ValueError, 'size|checksum'):
                QUALIFICATION.extract_cockpit(root, manifest, root / 'installed')
            self.assertFalse((root / 'installed').exists())

    def test_unsafe_members_and_missing_native_payload_fail(self):
        for extra in (
                ('resolvers/cockpit/../../escape', b'escape', tarfile.REGTYPE),
                ('resolvers/another/assets/file', b'wrong owner', tarfile.REGTYPE),
                ('resolvers/cockpit/assets/link', b'', tarfile.SYMTYPE),
                ('resolvers/cockpit/assets/ui/cockpit-reader', b'duplicate', tarfile.REGTYPE)):
            with self.subTest(member=extra[0]), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                manifest = self.archive(root, extra)
                with self.assertRaises(ValueError):
                    QUALIFICATION.extract_cockpit(root, manifest, root / 'installed')
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            manifest = self.archive(root, missing_reader=True)
            with self.assertRaisesRegex(ValueError, 'external package ABI'):
                QUALIFICATION.extract_cockpit(root, manifest, root / 'installed')

    def test_wrong_platform_and_member_budget_fail(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            manifest = self.archive(root)
            manifest['platform'] = 'linux-arm64'
            with self.assertRaisesRegex(ValueError, 'Linux amd64'):
                QUALIFICATION.extract_cockpit(root, manifest, root / 'wrong-platform')
            manifest['platform'] = 'linux-amd64'
            with mock.patch.object(QUALIFICATION, 'MAX_PACKAGE_MEMBERS', 1):
                with self.assertRaisesRegex(ValueError, 'excessive member'):
                    QUALIFICATION.extract_cockpit(root, manifest, root / 'too-many')

    def test_native_report_rejects_skips_missing_tests_and_failures(self):
        with tempfile.TemporaryDirectory() as temporary:
            report = Path(temporary) / 'tests.xml'
            good = ''.join(f'<testcase name="{name}"/>' for name in sorted(QUALIFICATION.REQUIRED_TESTS))
            report.write_text('<testsuite>' + good + '</testsuite>')
            QUALIFICATION.require_complete_ctest_report(report)
            for invalid in (
                    good.replace('/>', '><skipped/></testcase>', 1),
                    good.replace('/>', '><failure/></testcase>', 1),
                    good.replace('<testcase ', '<testcase status="notrun" ', 1),
                    good + '<testcase name="unexpected"/>', ''):
                report.write_text('<testsuite>' + invalid + '</testsuite>')
                with self.assertRaises(ValueError):
                    QUALIFICATION.require_complete_ctest_report(report)


if __name__ == '__main__':
    unittest.main()
