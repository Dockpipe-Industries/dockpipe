"""Locked SDK publication rejects changed content and ambiguous source identities."""
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / 'build-locked-go-sdk.py'
SPEC = importlib.util.spec_from_file_location('locked_go_sdk', SCRIPT)
PRODUCER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PRODUCER)


class LockedGoSDKTests(unittest.TestCase):
    def test_workflow_uses_literal_locked_source_without_shared_cache(self):
        root = SCRIPT.parents[2]
        lock = PRODUCER.read_lock(root / 'release/go-sdk-source.lock.json')
        workflow = (root / '.github/workflows/go-sdk.yml').read_text()
        self.assertIn('ref: ' + lock['source_commit'] + '\n', workflow)
        self.assertNotRegex(workflow, r'ref:\s*\$\{\{')
        checkout = re.search(r'ref: [0-9a-f]{40}\n(?P<options>(?:          .+\n)+)', workflow)
        self.assertIsNotNone(checkout)
        self.assertIn('persist-credentials: false', checkout.group('options'))
        setup = re.search(r'uses: actions/setup-go@[^\n]+\n(?P<options>(?:        .+\n)+)', workflow)
        self.assertIsNotNone(setup)
        self.assertIn('cache: false', setup.group('options'))

    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        digest = 'a' * 64
        stem = 'dockpipe-go-sdk-' + digest[:16]
        archive = self.root / (stem + '.tar.gz')
        archive.write_bytes(b'locked archive')
        self.lock = {
            'schema': 1,
            'source_commit': 'b' * 40,
            'version': '0.1.0-source.' + digest[:16],
            'source_digest': digest,
            'archive_asset': archive.name,
            'archive_sha256': hashlib.sha256(archive.read_bytes()).hexdigest(),
            'archive_bytes': archive.stat().st_size,
            'manifest_asset': stem + '.json',
        }
        self.manifest = {
            'sdk_version': self.lock['version'],
            'source_commit': self.lock['source_commit'],
            'source_digest': self.lock['source_digest'],
            'artifact': self.lock['archive_asset'],
            'sha256': self.lock['archive_sha256'],
            'bytes': self.lock['archive_bytes'],
        }
        self.write_manifest()

    def write_manifest(self):
        path = self.root / self.lock['manifest_asset']
        path.write_text(json.dumps(self.manifest))
        self.lock['manifest_sha256'] = hashlib.sha256(path.read_bytes()).hexdigest()

    def test_exact_content_is_accepted(self):
        PRODUCER.verify_output(self.root, self.lock)

    def test_modified_archive_rejected(self):
        (self.root / self.lock['archive_asset']).write_bytes(b'changed')
        with self.assertRaisesRegex(ValueError, 'differs from the committed lock'):
            PRODUCER.verify_output(self.root, self.lock)

    def test_modified_manifest_rejected(self):
        (self.root / self.lock['manifest_asset']).write_text('{}')
        with self.assertRaisesRegex(ValueError, 'differs from the committed lock'):
            PRODUCER.verify_output(self.root, self.lock)

    def test_wrong_provenance_rejected_even_with_updated_manifest_hash(self):
        self.manifest['source_commit'] = 'c' * 40
        self.write_manifest()
        with self.assertRaisesRegex(ValueError, 'metadata differs'):
            PRODUCER.verify_output(self.root, self.lock)

    def test_wrong_size_rejected(self):
        self.lock['archive_bytes'] += 1
        with self.assertRaisesRegex(ValueError, 'archive size differs'):
            PRODUCER.verify_output(self.root, self.lock)

    def test_lock_requires_commit_and_content_named_assets(self):
        lockfile = self.root / 'lock.json'
        for field, invalid in (
                ('source_commit', 'master'),
                ('manifest_sha256', 'latest'),
                ('version', '0.1.0'),
                ('archive_asset', '../archive.tar.gz'),
                ('archive_bytes', True)):
            with self.subTest(field=field):
                changed = dict(self.lock)
                changed[field] = invalid
                lockfile.write_text(json.dumps(changed))
                with self.assertRaises(ValueError):
                    PRODUCER.read_lock(lockfile)


if __name__ == '__main__':
    unittest.main()
