"""Framework lifecycle checks using explicitly supplied current native artifacts."""
import copy
import json
import os
from pathlib import Path
import shutil
import tempfile
import unittest

from native_artifacts import Store
from shared_runtime_probe import containment, digest, dump


@unittest.skipUnless(os.getenv('PIPELANG_NATIVE_TEST_CONFIG'), 'prepared current artifact required')
class NativeLifecycleTests(unittest.TestCase):
    def setUp(self):
        containment()
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.work = Path(self.temporary.name)
        self.config = json.loads(Path(os.environ['PIPELANG_NATIVE_TEST_CONFIG']).read_text())
        self.key = os.environ['PIPELANG_NATIVE_TEST_KEY']
        self.cache = self.work / 'cache'
        self.cache.mkdir(mode=0o700)
        root = self.work / 'representation'
        root.mkdir(mode=0o700)
        original = Path(self.config['root'])
        for name in self.config['support']:
            shutil.copy2(original / name, root / name)
        self.config['root'] = str(root)
        self.keys = []
        def copy_key(key):
            self.keys.append(key)
            shutil.copytree(original / key, root / key)
            (self.cache / key).mkdir()
            recipe = json.loads((root / key / 'recipe.json').read_text())
            dump(self.cache / key / 'record.json', dict(Key=key, Version='pipelang-native-validation-v2', BinarySHA256=recipe['sha256']))
            if recipe['base']:
                copy_key(recipe['base'])
        copy_key(self.key)
        self.store = Store(self.cache, self.config)
        self.addCleanup(self.store.close)

    def test_replay_needs_no_original_binaries_or_exports(self):
        expected = self.store.record(self.key)
        before = len(list(Path('/proc/self/fd').iterdir()))
        for _ in range(3):
            fd = self.store.acquire(self.key, expected)
            try:
                self.assertEqual(digest(Path('/proc/self/fd') / str(fd)), expected)
                with self.assertRaises(OSError):
                    os.pwrite(fd, b'changed', 0)
            finally:
                os.close(fd)
        self.assertEqual(len(list(Path('/proc/self/fd').iterdir())), before)

    def test_current_record_and_requested_digest_are_independent(self):
        with self.assertRaisesRegex(ValueError, 'requested native digest'):
            self.store.acquire(self.key, '0' * 64)
        record = self.cache / self.key / 'record.json'
        value = json.loads(record.read_text())
        value['BinarySHA256'] = '0' * 64
        dump(record, value)
        with self.assertRaisesRegex(ValueError, 'recipe identity'):
            self.store.acquire(self.key, '0' * 64)

    def test_payload_corruption_and_recipe_cycles_fail_closed(self):
        root = Path(self.config['root'])
        recipe_path = root / self.key / 'recipe.json'
        recipe = json.loads(recipe_path.read_text())
        payload = root / recipe['encoded']['tokens']['path']
        data = payload.read_bytes()
        payload.write_bytes(b'x' + data[1:])
        with self.assertRaises(ValueError):
            self.store.acquire(self.key, self.store.record(self.key))
        payload.write_bytes(data)
        recipe['base'] = self.key
        dump(recipe_path, recipe)
        with self.assertRaisesRegex(ValueError, 'cyclic'):
            self.store.acquire(self.key, self.store.record(self.key))

    def test_new_eligible_identity_prepares_without_manifest_edit(self):
        key = 'e' * 64
        path = self.cache / key
        path.mkdir()
        original = Path(os.environ['PIPELANG_NATIVE_TEST_CACHE']) / self.key / 'program.test'
        shutil.copy2(original, path / 'program.test')
        expected = digest(original)
        dump(path / 'record.json', dict(Key=key, Version='pipelang-native-validation-v2', BinarySHA256=expected))
        fd = self.store.acquire(key, expected)
        try:
            self.assertEqual(digest(Path('/proc/self/fd') / str(fd)), expected)
        finally:
            os.close(fd)
        self.assertTrue((Path(self.config['root']) / key / 'recipe.json').is_file())

    def test_new_unsupported_artifact_keeps_ordinary_path(self):
        key = 'f' * 64
        path = self.cache / key
        path.mkdir()
        binary = path / 'program.test'
        binary.write_bytes(b'unsupported native executable')
        expected = digest(binary)
        dump(path / 'record.json', dict(Key=key, Version='pipelang-native-validation-v2', BinarySHA256=expected))
        self.assertIsNone(self.store.acquire(key, expected))
        self.assertIsNone(self.store.acquire(key, expected))
        self.assertEqual(binary.read_bytes(), b'unsupported native executable')


if __name__ == '__main__':
    unittest.main()
