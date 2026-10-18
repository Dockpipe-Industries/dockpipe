import ctypes
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'assets/scripts'))
from elf_artifacts import audit, split
from native_sdk import verify_release


class ArtifactTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)

    def command(self, *arguments):
        subprocess.run(arguments, check=True, capture_output=True, timeout=60)

    def test_split_retains_runtime_behavior_and_matched_diagnostics(self):
        source = self.root / 'sample.c'
        source.write_text('__attribute__((visibility("default"))) int api_add(int x) { return x + 7; }\n')
        binary = self.root / 'sample.so'
        self.command('cc', '-shared', '-fPIC', '-g', '-O2', '-fvisibility=hidden', '-Wl,--build-id', str(source), '-o', str(binary))
        runtime = self.root / 'runtime/sample.so'
        debug = self.root / 'private/sample.debug'
        receipt = split(binary, runtime, debug, ['api_add'])
        self.assertTrue(receipt['allocated_sections_unchanged'])
        self.assertEqual(ctypes.CDLL(str(runtime)).api_add(10), 17)
        self.assertEqual(audit(binary, ['api_add'])['build_id'], receipt['build_id'])
        with self.assertRaisesRegex(ValueError, 'allowlist'):
            audit(runtime, [])
        with self.assertRaisesRegex(ValueError, 'distinct new files'):
            split(binary, runtime, debug, ['api_add'])
        with self.assertRaisesRegex(ValueError, 'debug information'):
            split(runtime, self.root / 'new.so', self.root / 'new.debug', ['api_add'])

    def signed_fixture(self):
        private = self.root / 'private.pem'
        public = self.root / 'public.pem'
        self.command('openssl', 'genpkey', '-algorithm', 'ED25519', '-out', str(private))
        self.command('openssl', 'pkey', '-in', str(private), '-pubout', '-out', str(public))
        artifacts = self.root / 'artifacts'
        artifacts.mkdir()
        content = b'known runtime bytes'
        (artifacts / 'runtime.bin').write_bytes(content)
        manifest = self.root / 'manifest.json'
        manifest.write_text(json.dumps({'schema': 1, 'artifact_class': 'runtime', 'files': [
            {'path': 'runtime.bin', 'bytes': len(content), 'sha256': hashlib.sha256(content).hexdigest()}]}))
        signature = self.root / 'signature'
        self.command('openssl', 'pkeyutl', '-sign', '-inkey', str(private), '-rawin', '-in', str(manifest), '-out', str(signature))
        return manifest, signature, public, artifacts

    def test_signed_complete_artifact_and_tampering(self):
        inputs = self.signed_fixture()
        self.assertEqual(verify_release(*inputs, 1024)['artifact_class'], 'runtime')
        with self.assertRaisesRegex(ValueError, 'budget'):
            verify_release(*inputs, 1)
        (inputs[3] / 'extra').write_text('unlisted')
        with self.assertRaisesRegex(ValueError, 'Unlisted'):
            verify_release(*inputs, 1024)
        (inputs[3] / 'extra').unlink()
        (inputs[3] / 'extra').symlink_to(inputs[0])
        with self.assertRaisesRegex(ValueError, 'links'):
            verify_release(*inputs, 1024)
        (inputs[3] / 'extra').unlink()
        (inputs[3] / 'runtime.bin').write_bytes(b'tampered')
        with self.assertRaisesRegex(ValueError, 'content'):
            verify_release(*inputs, 1024)
        inputs[0].write_text('{}')
        with self.assertRaisesRegex(ValueError, 'signature'):
            verify_release(*inputs, 1024)
