"""Offline qualification for the repository-quality package pin boundary."""

import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest import mock


SCRIPT = Path(__file__).resolve().parents[1] / 'assets/scripts/repo-quality-pinned.py'
spec = importlib.util.spec_from_file_location('repo_quality_pinned', SCRIPT)
wrapper = importlib.util.module_from_spec(spec)
spec.loader.exec_module(wrapper)

CATALOG_URL = ('https://packages.staging.dockpipe.com/packages/releases/'
               '0.6.18/linux-amd64/packages-store-manifest.json')
VERSION = '0.6.4'


FAKE_CLI = '''#!/usr/bin/env python3
import hashlib
import json
import os
from pathlib import Path
import shutil
import sys
import tarfile

args = sys.argv[1:]
if args[:2] == ['__state', 'package-runtime']:
    print(os.environ['TEST_SCOPE_DIR'])
elif args[:2] == ['package', 'install']:
    with Path(os.environ['TEST_INSTALL_MARKER']).open('a') as marker:
        marker.write('install\\n')
    kind = args[args.index('--kind') + 1]
    root = Path(os.environ['DOCKPIPE_GLOBAL_ROOT']) / 'packages' / (kind + 's')
    root.mkdir(parents=True, exist_ok=True)
    archive = root / Path(os.environ['TEST_ARCHIVE']).name
    shutil.copyfile(os.environ['TEST_ARCHIVE'], archive)
    if os.environ.get('TEST_CORRUPT_INSTALL'):
        archive.write_bytes(b'corrupt')
    print(json.dumps({'path': str(archive)}))
elif args[:2] == ['scope', 'artifacts']:
    print(os.environ['TEST_ARTIFACT_DIR'])
else:
    raise SystemExit('unexpected fake DockPipe command: ' + repr(args))
'''


class PinnedSummaryTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.scope = self.root / 'package-state'
        self.scope.mkdir()
        self.workdir = self.root / 'repo'
        self.workdir.mkdir()
        self.script = b'#!/usr/bin/env bash\nset -euo pipefail\nprintf "qualified\\n" > "$TEST_SUMMARY_MARKER"\n'
        self.archive = self.root / f'dockpipe-resolver-dorkpipe-{VERSION}.tar.gz'
        with tarfile.open(self.archive, 'w:gz') as output:
            member = tarfile.TarInfo(wrapper.SCRIPT_MEMBERS['resolver'])
            member.size = len(self.script)
            output.addfile(member, io.BytesIO(self.script))
        self.archive_sha = hashlib.sha256(self.archive.read_bytes()).hexdigest()
        self.entry = {'name': 'dorkpipe', 'version': VERSION, 'sha256': self.archive_sha,
                      'size_bytes': self.archive.stat().st_size, 'tarball': self.archive.name}
        self.catalog = json.dumps({'schema': 1, 'platform': 'linux-amd64', 'packages': {
            'core': None, 'resolvers': [self.entry], 'workflows': []}},
            sort_keys=True).encode()
        self.selection = {'catalog_url': CATALOG_URL,
                          'catalog_sha256': hashlib.sha256(self.catalog).hexdigest(),
                          'kind': 'resolver', 'version': VERSION,
                          'archive_sha256': self.archive_sha,
                          'archive_size_bytes': self.archive.stat().st_size,
                          'script_sha256': hashlib.sha256(self.script).hexdigest()}
        self.lock_path = self.root / 'source-lock.json'
        self.write_lock()
        self.cli = self.root / 'dockpipe'
        self.cli.write_text(FAKE_CLI)
        self.cli.chmod(0o755)
        self.marker = self.root / 'summary-ran'
        self.install_marker = self.root / 'installs'
        self.environment = mock.patch.dict(os.environ, {
            'TEST_SCOPE_DIR': str(self.scope), 'TEST_ARCHIVE': str(self.archive),
            'TEST_SUMMARY_MARKER': str(self.marker),
            'TEST_INSTALL_MARKER': str(self.install_marker),
            'TEST_ARTIFACT_DIR': str(self.root / 'ci-artifacts'),
            'DOCKPIPE_GLOBAL_ROOT': str(self.root / 'ambient-global'),
        })
        self.environment.start()
        self.addCleanup(self.environment.stop)

    def write_lock(self, selection=None):
        target = self.selection if selection is None else selection
        self.lock_path.write_text(json.dumps({'schema': 1, 'targets': {
            'linux-amd64': target}}))

    def run_wrapper(self, catalog=None):
        data = self.catalog if catalog is None else catalog
        return wrapper.run(self.lock_path, self.cli, self.workdir, 'linux-amd64',
                           fetcher=lambda url: data)

    def test_selected_archive_runs_with_competing_ambient_version(self):
        ambient = self.root / 'ambient-global/packages/resolvers'
        ambient.mkdir(parents=True)
        (ambient / 'dockpipe-resolver-dorkpipe-9.9.9.tar.gz').write_bytes(b'wrong version')
        self.run_wrapper()
        self.assertEqual(self.marker.read_text(), 'qualified\n')
        self.marker.unlink()
        self.run_wrapper()
        self.assertEqual(self.marker.read_text(), 'qualified\n')
        self.assertEqual(self.install_marker.read_text(), 'install\n')
        self.assertEqual((ambient / 'dockpipe-resolver-dorkpipe-9.9.9.tar.gz').read_bytes(),
                         b'wrong version')

    def test_corrupted_scoped_cache_fails_before_download_or_execution(self):
        self.run_wrapper()
        cached = (self.scope / 'verified-archives' / self.archive_sha / 'resolvers'
                  / self.archive.name)
        self.assertTrue(cached.is_file())
        self.marker.unlink()
        cached.write_bytes(b'corrupt')
        with self.assertRaisesRegex(ValueError, 'archive size differs'):
            self.run_wrapper()
        self.assertFalse(self.marker.exists())
        self.assertEqual(self.install_marker.read_text(), 'install\n')

    def test_workflow_pin_executes_only_its_verified_member(self):
        self.archive = self.root / f'dockpipe-workflow-dorkpipe-{VERSION}.tar.gz'
        with tarfile.open(self.archive, 'w:gz') as output:
            member = tarfile.TarInfo(wrapper.SCRIPT_MEMBERS['workflow'])
            member.size = len(self.script)
            output.addfile(member, io.BytesIO(self.script))
        self.archive_sha = hashlib.sha256(self.archive.read_bytes()).hexdigest()
        self.entry = {'name': 'dorkpipe', 'version': VERSION,
                      'sha256': self.archive_sha, 'size_bytes': self.archive.stat().st_size,
                      'tarball': self.archive.name}
        self.catalog = json.dumps({'schema': 1, 'platform': 'linux-amd64', 'packages': {
            'core': None, 'resolvers': [], 'workflows': [self.entry]}},
            sort_keys=True).encode()
        self.selection.update({'kind': 'workflow', 'catalog_sha256': hashlib.sha256(self.catalog).hexdigest(),
                               'archive_sha256': self.archive_sha,
                               'archive_size_bytes': self.archive.stat().st_size})
        self.write_lock()
        with mock.patch.dict(os.environ, {'TEST_ARCHIVE': str(self.archive)}):
            self.run_wrapper()
        self.assertEqual(self.marker.read_text(), 'qualified\n')

    def test_catalog_drift_and_row_drift_fail_before_install_or_execution(self):
        with self.assertRaisesRegex(ValueError, 'catalog differs'):
            self.run_wrapper(self.catalog + b' ')
        self.assertFalse(self.marker.exists())
        changed = dict(self.entry, sha256='0' * 64)
        changed_catalog = json.dumps({'schema': 1, 'platform': 'linux-amd64',
                                      'packages': {'resolvers': [changed]}}).encode()
        self.selection['catalog_sha256'] = hashlib.sha256(changed_catalog).hexdigest()
        self.write_lock()
        with self.assertRaisesRegex(ValueError, 'entry differs'):
            self.run_wrapper(changed_catalog)
        self.assertFalse(self.marker.exists())

    def test_corrupted_install_or_script_pin_fails_before_summary(self):
        with mock.patch.dict(os.environ, {'TEST_CORRUPT_INSTALL': '1'}):
            with self.assertRaisesRegex(ValueError, 'archive size differs'):
                self.run_wrapper()
        self.assertFalse(self.marker.exists())
        self.selection['script_sha256'] = '0' * 64
        self.write_lock()
        with self.assertRaisesRegex(ValueError, 'archived summary script differs'):
            self.run_wrapper()
        self.assertFalse(self.marker.exists())

    def test_unpublished_target_and_unsafe_urls_fail_closed(self):
        self.lock_path.write_text('{"schema":1,"targets":{}}')
        with self.assertRaisesRegex(ValueError, 'no published'):
            self.run_wrapper()
        for invalid in (CATALOG_URL.replace('https://', 'http://'),
                        CATALOG_URL.replace('https://', 'https://user:pass@'),
                        CATALOG_URL.replace('/releases/', '/latest/'),
                        CATALOG_URL.replace('/releases/', '/releases/../'),
                        CATALOG_URL.replace('packages.staging.dockpipe.com', 'other.example.com'),
                        CATALOG_URL + '?token=value'):
            with self.subTest(invalid=invalid):
                with self.assertRaises(ValueError):
                    wrapper.validate_catalog_url(invalid)
        self.assertFalse(self.marker.exists())


if __name__ == '__main__':
    unittest.main()
