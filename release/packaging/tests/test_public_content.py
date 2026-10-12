import importlib.util
import io
from pathlib import Path
import tarfile
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / 'check-public-content.py'
spec = importlib.util.spec_from_file_location('public_content', SCRIPT)
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)


class PublicContentTests(unittest.TestCase):
    def test_requires_operator_targets_and_allows_placeholders(self):
        examples = [
            'R2_BUCKET: ""',
            '"environment_id": "your-production-environment-id"',
            'R2_ENDPOINT_URL=https://your-account-id.r2.cloudflarestorage.com',
            'repository: https://github.com/Dockpipe-Industries/dockpipe',
        ]
        for example in examples:
            with self.subTest(example=example):
                self.assertEqual(checker.findings('workflows/example/config.yml', example), [])
        for private in [
            'R2_BUCKET: "dockpipe"',
            'REPOSITORY = "Dockpipe-Industries/dockpipe"',
            'https://' + 'a' * 32 + '.r2.cloudflarestorage.com',
            '"environment_id": "' + 'a2' * 13 + '"',
            '/home/' + 'private-operator/project',
        ]:
            with self.subTest(private=private):
                self.assertTrue(checker.findings('workflows/example/config.yml', private))

    def test_scans_package_contents_without_extracting(self):
        with tempfile.TemporaryDirectory() as temporary:
            archive = Path(temporary) / 'example.tar.gz'
            payload = b'R2_BUCKET: "dockpipe"\n'
            with tarfile.open(archive, 'w:gz') as stream:
                member = tarfile.TarInfo('workflows/example/config.yml')
                member.size = len(payload)
                stream.addfile(member, io.BytesIO(payload))
            entries = list(checker.archive_entries(archive))
            self.assertEqual(len(entries), 1)
            self.assertTrue(checker.findings(entries[0][0], entries[0][1].decode()))
            self.assertFalse((Path(temporary) / 'workflows').exists())

    def test_archive_vendor_scope_preserves_authored_configuration_checks(self):
        root = 'resolvers/example/assets/tooling/bin/'
        vendor_files = [
            root + '.deps/aws/aws/dist/awscli/examples/example.rst',
            root + '.deps/node/runtime/share/doc/example.md',
            root + '.deps/go/go/test/example.go',
            root + '.deps/wrangler/node_modules/example/index.js',
            root + '.deps/azure/azure/cli/example.py',
        ]
        authored_files = [
            root + 'aws',
            root + '.deps/aws/source.json',
            root + '.deps/azure/source.json',
            root + '.deps/wrangler/package-lock.json',
            root + '.deps/unknown/config.json',
            'workflows/example/config.yml',
            'workflows/example/assets/.deps/go/go/config.json',
            root + '.deps/go/go/../../config.json',
        ]
        payload = b'"environment_id": "' + b'a2' * 13 + b'"\n'
        with tempfile.TemporaryDirectory() as temporary:
            archive = Path(temporary) / 'bundled-tools.tar.gz'
            with tarfile.open(archive, 'w:gz') as stream:
                for name in vendor_files + authored_files:
                    member = tarfile.TarInfo(name)
                    member.size = len(payload)
                    stream.addfile(member, io.BytesIO(payload))
            entries = list(checker.archive_entries(archive))
            self.assertEqual([path for path, _ in entries],
                             ['packages/' + path for path in authored_files])
            for path, data in entries:
                self.assertTrue(checker.findings(path, data.decode()), path)

        # A tracked file cannot gain an exemption by imitating a vendor path.
        for path in vendor_files:
            self.assertTrue(checker.findings('packages/' + path, payload.decode()), path)
