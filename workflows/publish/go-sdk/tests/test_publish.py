"""Locked Go SDK publication preserves exact assets through immutable transactions."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[4]


def load(name, relative):
    specification = importlib.util.spec_from_file_location(name, ROOT / relative)
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


PUBLISHER = load('go_sdk_publisher', 'workflows/publish/go-sdk/assets/scripts/publish.py')
TRANSACTIONS = load('sdk_transaction_fixtures', 'workflows/publish/shared/tests/test_sdk_release_transaction.py')
SOURCE = 'a' * 40


class GoSDKPublicationTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        identity = 'b' * 64
        stem = 'dockpipe-go-sdk-' + identity[:16]
        archive = self.root / (stem + '.tar.gz')
        archive.write_bytes(b'qualified SDK archive fixture')
        self.lock = {'schema': 1, 'source_commit': SOURCE, 'version': '0.1.0-source.' + identity[:16],
                     'source_digest': identity, 'manifest_asset': stem + '.json',
                     'archive_asset': archive.name, 'archive_bytes': archive.stat().st_size,
                     'archive_sha256': hashlib.sha256(archive.read_bytes()).hexdigest()}
        self.manifest = {'schema': 1, 'module': 'dockpipe', 'sdk_version': self.lock['version'],
                         'source_commit': SOURCE, 'source_digest': identity,
                         'artifact': archive.name, 'sha256': self.lock['archive_sha256'],
                         'bytes': archive.stat().st_size,
                         'targets': list(PUBLISHER.LOCKED.PRODUCER.TARGETS)}
        self.manifest_path = self.root / self.lock['manifest_asset']
        self.write_manifest()
        self.tag = 'dockpipe-go-sdk-' + self.lock['version']
        self.environment = {'GITHUB_REPOSITORY': PUBLISHER.SHARED.SOURCE_REPOSITORY,
                            'GITHUB_EVENT_NAME': 'workflow_dispatch', 'GITHUB_REF': 'refs/heads/dev',
                            'GITHUB_SHA': 'c' * 40, 'SDK_TAG': self.tag, 'SDK_COMMIT': SOURCE}

    def write_manifest(self):
        self.manifest_path.write_text(json.dumps(self.manifest))
        self.lock['manifest_sha256'] = hashlib.sha256(self.manifest_path.read_bytes()).hexdigest()

    def invoke(self, remote, destination='ExampleOperator/sdk-candidates'):
        assets = PUBLISHER.validate(self.root, self.lock)
        with patch.dict(os.environ, self.environment, clear=True), \
                patch.object(PUBLISHER.LOCKED, 'read_lock', return_value=self.lock), \
                patch.object(TRANSACTIONS, 'ASSETS', assets), \
                patch.object(PUBLISHER.SHARED, 'run', side_effect=remote.execute), patch('builtins.print'):
            PUBLISHER.publish(self.root, destination)

    def test_exact_locked_assets_are_required(self):
        self.assertEqual(set(PUBLISHER.validate(self.root, self.lock)),
                         {self.lock['manifest_asset'], self.lock['archive_asset']})
        archive = self.root / self.lock['archive_asset']
        archive.write_bytes(b'changed archive')
        with self.assertRaises(ValueError):
            PUBLISHER.validate(self.root, self.lock)

    def test_forged_source_or_target_metadata_cannot_reuse_manifest_hash(self):
        for field, value in [('source_commit', 'd' * 40), ('targets', ['linux/amd64'])]:
            with self.subTest(field=field):
                original = self.manifest[field]
                self.manifest[field] = value
                self.write_manifest()
                with self.assertRaises(ValueError):
                    PUBLISHER.validate(self.root, self.lock)
                self.manifest[field] = original
                self.write_manifest()

    def test_unexpected_files_and_archive_alias_are_rejected(self):
        extra = self.root / 'unexpected'
        extra.write_bytes(b'unexpected')
        with self.assertRaises(ValueError):
            PUBLISHER.validate(self.root, self.lock)
        extra.unlink()
        archive = self.root / self.lock['archive_asset']
        content = archive.read_bytes()
        archive.unlink()
        target = self.root.parent / (self.root.name + '-outside')
        target.write_bytes(content)
        self.addCleanup(target.unlink)
        archive.symlink_to(target)
        with self.assertRaises(ValueError):
            PUBLISHER.validate(self.root, self.lock)

    def test_wrong_dispatch_or_reproduction_identity_fails_before_network(self):
        for changes in ({'GITHUB_EVENT_NAME': 'push'}, {'SDK_TAG': 'other'},
                        {'SDK_COMMIT': 'd' * 40}, {'GITHUB_SHA': 'short'},
                        {'GITHUB_REPOSITORY': 'ExampleOperator/untrusted-source'}):
            with self.subTest(changes=changes), patch.dict(os.environ, self.environment | changes, clear=True), \
                    patch.object(PUBLISHER.LOCKED, 'read_lock', return_value=self.lock), \
                    patch.object(PUBLISHER.SHARED, 'run') as command:
                with self.assertRaises(ValueError):
                    PUBLISHER.publish(Path('not-read'))
                command.assert_not_called()

    def test_explicit_destination_and_source_tag_survive_create_and_readback(self):
        remote = TRANSACTIONS.Remote(PUBLISHER.SHARED, self.tag)
        self.invoke(remote)
        self.assertFalse(remote.release['draft'])
        self.assertEqual(remote.release['target_commitish'], SOURCE)
        for command in remote.calls:
            for value in command:
                if value.startswith('repos/'):
                    self.assertTrue(value.startswith('repos/ExampleOperator/sdk-candidates/'))
            self.assertNotIn('--clobber', command)
            self.assertNotIn('delete', command)

    def test_ambiguous_create_or_upload_resumes_without_replacement(self):
        for failure in ('lose_create', 'lose_upload'):
            with self.subTest(failure=failure):
                remote = TRANSACTIONS.Remote(PUBLISHER.SHARED, self.tag)
                setattr(remote, failure, True)
                with self.assertRaisesRegex(ValueError, 'Ambiguous'):
                    self.invoke(remote)
                self.invoke(remote)
                self.assertFalse(remote.release['draft'])
                self.assertEqual(sum('--method' in command for command in remote.calls), 1)


if __name__ == '__main__':
    unittest.main()
