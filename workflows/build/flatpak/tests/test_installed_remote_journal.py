"""Strict observation of generated Qt journal data; no app state is modified."""
from pathlib import Path
import importlib.util
import json
import tempfile
import unittest

SOURCE = Path(__file__).resolve().parents[1] / 'assets/scripts/installed-remote-journal.py'
SPEC = importlib.util.spec_from_file_location('installed_remote_journal_tests', SOURCE)
JOURNAL = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(JOURNAL)


class PrivateJournalTests(unittest.TestCase):
    def setUp(self):
        self.reference = {'schema': 'dockpipe.remote.desktop/v1', 'id': 'a' * 64, 'epoch': 'b' * 64,
                          'target': {'node': 'fixture-peer', 'fingerprint': 'c' * 64, 'generation': 'd' * 64}}

    def settings(self, values):
        raw = json.dumps(values, separators=(',', ':')).replace('"', '\\"')
        return ('[remote]\nuncertainDesktopSessions="@ByteArray(' + raw + ')"\n').encode()

    def test_exact_original_reference_and_empty_journals_are_read(self):
        self.assertEqual([self.reference], JOURNAL.decode_settings(self.settings([self.reference])))
        for data in [b'[remote]\n', b'[remote]\nuncertainDesktopSessions=@ByteArray()\n', self.settings([])]:
            with self.subTest(data=data):
                self.assertEqual([], JOURNAL.decode_settings(data))

    def test_foreign_shape_duplicates_and_invalid_identity_are_rejected(self):
        for value in [{}, dict(self.reference, id=True), dict(self.reference, epoch='unknown'),
                      dict(self.reference, target={}), dict(self.reference, extra=True)]:
            with self.subTest(value=value), self.assertRaises(ValueError):
                JOURNAL.decode_settings(self.settings([value]))
        with self.assertRaises(ValueError):
            JOURNAL.decode_settings(self.settings([self.reference, self.reference]))

    def test_non_byte_array_unknown_escape_and_duplicate_json_key_are_rejected(self):
        for data in [b'[remote]\nuncertainDesktopSessions=[]\n',
                     b'[remote]\nuncertainDesktopSessions=@ByteArray(\\q)\n',
                     b'[remote]\nuncertainDesktopSessions=@ByteArray([{"id":1,"id":2}])\n']:
            with self.subTest(data=data), self.assertRaises(ValueError):
                JOURNAL.decode_settings(data)

    def test_only_private_regular_nonsymlink_settings_are_read(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            path = root / 'settings.ini'
            path.write_bytes(self.settings([self.reference]))
            self.assertEqual([self.reference], JOURNAL.read(path, root))
            alias = root / 'alias.ini'
            alias.symlink_to(path)
            with self.assertRaises(ValueError):
                JOURNAL.read(alias, root)
            root.chmod(0o755)
            with self.assertRaises(ValueError):
                JOURNAL.read(path, root)


if __name__ == '__main__':
    unittest.main()
