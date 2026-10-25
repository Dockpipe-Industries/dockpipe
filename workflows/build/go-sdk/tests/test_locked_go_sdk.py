"""Locked SDK publication rejects changed content and ambiguous source identities."""
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / 'assets/scripts/build-locked.py'
SPEC = importlib.util.spec_from_file_location('locked_go_sdk', SCRIPT)
PRODUCER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PRODUCER)


class PublicGoSDKEntryPointTests(unittest.TestCase):
    def test_historical_source_retains_exact_legacy_entrypoints(self):
        with tempfile.TemporaryDirectory() as temporary:
            self.assertEqual(PRODUCER.PRODUCER.entrypoints(Path(temporary)), PRODUCER.PRODUCER.ENTRYPOINTS)

    def test_new_public_helper_is_an_explicit_export_root(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / 'src/lib/sdk/repotools').mkdir(parents=True)
            self.assertEqual(PRODUCER.PRODUCER.entrypoints(root),
                             (*PRODUCER.PRODUCER.ENTRYPOINTS, 'dockpipe/src/lib/sdk/repotools'))

    def test_public_helper_directory_alias_is_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            parent = root / 'src/lib/sdk'
            parent.mkdir(parents=True)
            (parent / 'repotools').symlink_to(root, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, 'directory alias'):
                PRODUCER.PRODUCER.entrypoints(root)


class LockedGoSDKTests(unittest.TestCase):
    def test_workflow_uses_literal_locked_source_without_shared_cache(self):
        root = SCRIPT.parents[5]
        lock = PRODUCER.read_lock(root / 'workflows/build/go-sdk/assets/source.lock.json')
        workflow = (root / '.github/workflows/go-sdk.yml').read_text()
        self.assertIn('ref: ' + lock['source_commit'] + '\n', workflow)
        self.assertNotRegex(workflow, r'ref:\s*\$\{\{')
        checkout = re.search(r'ref: [0-9a-f]{40}\n(?P<options>(?:          .+\n)+)', workflow)
        self.assertIsNotNone(checkout)
        self.assertIn('persist-credentials: false', checkout.group('options'))
        setup = re.search(r'uses: actions/setup-go@[^\n]+\n(?P<options>(?:        .+\n)+)', workflow)
        self.assertIsNotNone(setup)
        self.assertIn('cache: false', setup.group('options'))

    def test_actions_adapter_preserves_protected_publication_and_owned_operations(self):
        root = SCRIPT.parents[5]
        workflow = (root / '.github/workflows/go-sdk.yml').read_text()
        self.assertIn('environment: release', workflow)
        self.assertIn('if: inputs.publish', workflow)
        self.assertIn('workflows/build/go-sdk/inspect.yml -- --output "$GITHUB_OUTPUT"', workflow)
        self.assertIn('workflows/publish/go-sdk/config.yml -- --directory sdk-artifacts', workflow)
        self.assertNotIn('gh release create', workflow)
        self.assertNotIn("python3 - <<", workflow)

    def assert_local_launchers_have_one_preceding_checkout(self, workflow):
        jobs = re.split(r'^  ([a-z][a-z0-9-]*):\n', workflow.split('jobs:\n', 1)[1], flags=re.MULTILINE)
        for name, body in zip(jobs[1::2], jobs[2::2]):
            steps = re.split(r'^      - ', body, flags=re.MULTILINE)[1:]
            local_launchers = [index for index, step in enumerate(steps)
                               if 'workflows/shared/launch.py' in step or 'workflows/shared/bootstrap.py' in step]
            if not local_launchers:
                continue
            checkouts = [index for index, step in enumerate(steps)
                         if step.startswith('uses: actions/checkout@')]
            workspace_checkouts = [index for index in checkouts
                                   if not re.search(r'^          path:', steps[index], re.MULTILINE)]
            self.assertEqual(len(workspace_checkouts), 1, name + ' needs exactly one producer checkout')
            self.assertTrue(all(workspace_checkouts[0] < index for index in local_launchers), name)
            for index in checkouts:
                self.assertIn('persist-credentials: false', steps[index])
            if name == 'reproduce':
                self.assertEqual(len(checkouts), 2, 'One producer checkout and one literal SDK source checkout')
            elif name == 'publish':
                self.assertEqual(len(checkouts), 1)
                download = next(index for index, step in enumerate(steps)
                                if step.startswith('uses: actions/download-artifact@'))
                self.assertLess(workspace_checkouts[0], download)

    def test_every_local_launcher_has_exactly_one_preceding_producer_checkout(self):
        workflow = (SCRIPT.parents[5] / '.github/workflows/go-sdk.yml').read_text()
        self.assert_local_launchers_have_one_preceding_checkout(workflow)
        checkout = ('      - uses: actions/checkout@v7\n'
                    '        with:\n'
                    '          persist-credentials: false\n')
        publish = workflow.index('\n  publish:\n')
        missing_publish_checkout = workflow[:publish] + workflow[publish:].replace(checkout, '', 1)
        duplicate_reproduce_checkout = workflow.replace(checkout, checkout + checkout, 1)
        for broken in (missing_publish_checkout, duplicate_reproduce_checkout):
            with self.subTest(broken=broken), self.assertRaises(AssertionError):
                self.assert_local_launchers_have_one_preceding_checkout(broken)

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
                ('schema', True),
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
