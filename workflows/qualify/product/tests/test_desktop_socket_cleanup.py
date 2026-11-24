"""Private candidate tests: never traverse runtime trees."""
import importlib.util
from pathlib import Path
import socket
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('candidate_desktop', Path(__file__).resolve().parents[1] / 'platforms/linux/desktop.py')
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class SocketCleanupTests(unittest.TestCase):
    def test_known_socket_only_directory_can_be_removed(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / 'runtime'
            root.mkdir(mode=0o700)
            with socket.socket(socket.AF_UNIX) as owned:
                owned.bind(str(root / 'session-bus'))
                module.remove_owned_socket_directory(root)
            self.assertFalse(root.exists())

    def test_unknown_entries_refuse_before_any_unlink(self):
        for name in ['doc', 'dconf', '.flatpak-helper']:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary) / 'runtime'
                root.mkdir(mode=0o700)
                (root / name).mkdir()
                (root / name / 'protected').write_bytes(b'preserve')
                with socket.socket(socket.AF_UNIX) as owned:
                    owned.bind(str(root / 'session-bus'))
                    with self.assertRaises(ValueError):
                        module.remove_owned_socket_directory(root)
                    self.assertTrue((root / 'session-bus').exists())
                    self.assertEqual((root / name / 'protected').read_bytes(), b'preserve')

    def test_symlink_is_not_followed(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / 'runtime'
            root.mkdir(mode=0o700)
            alias = Path(temporary) / 'alias'
            alias.symlink_to(root)
            with self.assertRaises(OSError):
                module.remove_owned_socket_directory(alias)
            self.assertTrue(root.is_dir())

    def test_known_socket_disappearing_before_first_stat_is_benign(self):
        for name in ['session-bus', 'accessibility-bus']:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary) / 'runtime'
                root.mkdir(mode=0o700)
                real_stat = module.os.stat
                vanished = []
                def stat_after_bus_exit(selected, *arguments, **keywords):
                    if selected == name and not vanished:
                        (root / name).unlink()
                        vanished.append(name)
                    return real_stat(selected, *arguments, **keywords)
                with socket.socket(socket.AF_UNIX) as owned:
                    owned.bind(str(root / name))
                    with patch.object(module.os, 'stat', side_effect=stat_after_bus_exit):
                        module.remove_owned_socket_directory(root)
                self.assertEqual([name], vanished)
                self.assertFalse(root.exists())

    def test_disappeared_unknown_name_is_refused_before_stat_or_unlink(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / 'runtime'
            root.mkdir(mode=0o700)
            with patch.object(module.os, 'listdir', return_value=['unknown-helper']), \
                    patch.object(module.os, 'stat') as stat_entry, \
                    patch.object(module.os, 'unlink') as unlink_entry:
                with self.assertRaisesRegex(ValueError, 'unknown entries'):
                    module.remove_owned_socket_directory(root)
                stat_entry.assert_not_called()
                unlink_entry.assert_not_called()
            self.assertTrue(root.exists())

    def test_present_known_name_must_be_socket_not_file_directory_or_symlink(self):
        for kind in ['file', 'directory', 'symlink']:
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary) / 'runtime'
                root.mkdir(mode=0o700)
                canary = Path(temporary) / 'protected'
                canary.write_bytes(b'preserve')
                entry = root / 'session-bus'
                if kind == 'file':
                    entry.write_bytes(b'preserve-owned-entry')
                elif kind == 'directory':
                    entry.mkdir()
                else:
                    entry.symlink_to(canary)
                with self.assertRaisesRegex(ValueError, 'unknown entries'):
                    module.remove_owned_socket_directory(root)
                self.assertTrue(entry.exists())
                self.assertEqual(b'preserve', canary.read_bytes())

    def test_permission_failure_is_not_treated_as_disappearance(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / 'runtime'
            root.mkdir(mode=0o700)
            with socket.socket(socket.AF_UNIX) as owned:
                owned.bind(str(root / 'session-bus'))
                with patch.object(module.os, 'stat', side_effect=PermissionError('generated refusal')):
                    with self.assertRaises(PermissionError):
                        module.remove_owned_socket_directory(root)
                self.assertTrue((root / 'session-bus').exists())

    def test_replaced_socket_identity_is_still_refused(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / 'runtime'
            root.mkdir(mode=0o700)
            real_stat = module.os.stat
            observations = []
            with socket.socket(socket.AF_UNIX) as original, socket.socket(socket.AF_UNIX) as replacement:
                original.bind(str(root / 'session-bus'))
                def replace_before_second_stat(selected, *arguments, **keywords):
                    if selected == 'session-bus':
                        observations.append(selected)
                        if len(observations) == 2:
                            (root / 'session-bus').unlink()
                            replacement.bind(str(root / 'session-bus'))
                    return real_stat(selected, *arguments, **keywords)
                with patch.object(module.os, 'stat', side_effect=replace_before_second_stat):
                    with self.assertRaisesRegex(ValueError, 'changed before cleanup'):
                        module.remove_owned_socket_directory(root)
                self.assertTrue((root / 'session-bus').exists())

    def test_known_socket_disappearing_after_second_stat_is_benign(self):
        for name in ['session-bus', 'accessibility-bus']:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary) / 'runtime'
                root.mkdir(mode=0o700)
                real_unlink = module.os.unlink
                vanished = []
                def unlink_after_bus_exit(selected, *arguments, **keywords):
                    if selected == name and not vanished:
                        real_unlink(selected, *arguments, **keywords)
                        vanished.append(name)
                    return real_unlink(selected, *arguments, **keywords)
                with socket.socket(socket.AF_UNIX) as owned:
                    owned.bind(str(root / name))
                    with patch.object(module.os, 'unlink', side_effect=unlink_after_bus_exit):
                        module.remove_owned_socket_directory(root)
                self.assertEqual([name], vanished)
                self.assertFalse(root.exists())

    def test_unlink_permission_failure_is_not_ignored(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / 'runtime'
            root.mkdir(mode=0o700)
            with socket.socket(socket.AF_UNIX) as owned:
                owned.bind(str(root / 'session-bus'))
                with patch.object(module.os, 'unlink', side_effect=PermissionError('generated refusal')):
                    with self.assertRaises(PermissionError):
                        module.remove_owned_socket_directory(root)
                self.assertTrue((root / 'session-bus').exists())


if __name__ == '__main__':
    unittest.main()
