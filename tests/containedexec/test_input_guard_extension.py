import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from campaign import InputGuard, fingerprint


class InputGuardExtensionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.source = Path(self.temp.name) / 'source'
        self.module = Path(self.temp.name) / 'module'
        self.source.mkdir()
        self.module.mkdir()
        (self.source / 'input').write_text('source')
        (self.module / 'input').write_text('module')

    def guard(self):
        guard = InputGuard([self.source], {'flags': 'normal'})
        self.addCleanup(guard.close)
        return guard

    def test_full_closure_identity_and_new_module_changes(self):
        guard = self.guard()
        guard.extend([self.module])
        self.assertEqual(guard.check(), fingerprint([self.source, self.module], {'flags': 'normal'})['digest'])
        (self.module / 'new').write_text('new')
        with self.assertRaises(RuntimeError):
            guard.check()

    def test_restored_existing_input_before_extension_is_rejected(self):
        guard = self.guard()
        path = self.source / 'input'
        path.write_text('changed')
        path.write_text('source')
        with self.assertRaises(RuntimeError):
            guard.extend([self.module])

    def test_both_closures_stay_watched_during_rehash(self):
        for directory in (self.source, self.module):
            with self.subTest(directory=directory):
                guard = self.guard()
                path = directory / 'input'
                original = path.read_bytes()

                def mutate_and_restore(paths, settings):
                    identity = fingerprint(paths, settings)
                    path.write_bytes(b'changed')
                    path.write_bytes(original)
                    return identity

                with patch('campaign.fingerprint', side_effect=mutate_and_restore):
                    with self.assertRaises(RuntimeError):
                        guard.extend([self.module])
                with self.assertRaises(RuntimeError):
                    guard.check()
                guard.close()

    def test_extension_failure_cannot_leave_an_accepted_partial_guard(self):
        guard = self.guard()
        with patch.object(guard, '_watch', side_effect=OSError('watch unavailable')):
            with self.assertRaises(OSError):
                guard.extend([self.module])
        with self.assertRaises(RuntimeError):
            guard.check()

    def test_releasing_file_rows_keeps_watches_and_full_rehash(self):
        guard = self.guard()
        before = guard.check()
        guard.release_file_inventory()
        self.assertEqual(guard.check(), before)
        guard.extend([self.module])
        self.assertEqual(guard.identity, fingerprint([self.source, self.module], {'flags': 'normal'}))
        guard.release_file_inventory()
        path = self.source / 'input'
        path.write_text('changed')
        path.write_text('source')
        with self.assertRaises(RuntimeError):
            guard.check()


if __name__ == '__main__':
    unittest.main()
