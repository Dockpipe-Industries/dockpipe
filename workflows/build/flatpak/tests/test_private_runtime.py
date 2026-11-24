"""Inert Flatpak socket-path regression; no display, app or paired target."""
from pathlib import Path
import importlib.util
import os
import socket
import stat
import tempfile
import unittest
from unittest.mock import patch

SPECIFICATION = importlib.util.spec_from_file_location(
    'private_runtime_owner', Path(__file__).resolve().parents[1] / 'assets/scripts/installed-product-gui.py')
GUI = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(GUI)


class PrivateRuntimeTests(unittest.TestCase):
    def test_deep_inherited_tmpdir_does_not_relocate_either_owned_runtime(self):
        with tempfile.TemporaryDirectory(dir='/tmp') as temporary:
            deep = Path(temporary) / ('generated-parent-' * 10) / ('generated-child-' * 6)
            deep.mkdir(parents=True)
            with patch.dict(os.environ, {'TMPDIR': str(deep)}), patch.object(tempfile, 'tempdir', None):
                self.assertEqual(str(deep), tempfile.gettempdir())
                for prefix in ['dockpipe-flatpak-runtime-', 'dockpipe-remote-runtime-']:
                    runtime = GUI.create_private_runtime(prefix)
                    try:
                        self.assertEqual(Path('/tmp'), runtime.parent)
                        self.assertEqual(0o700, stat.S_IMODE(runtime.stat().st_mode))
                        self.assertEqual(os.getuid(), runtime.stat().st_uid)
                        self.assertEqual([], list(runtime.iterdir()))
                        child = runtime / '.flatpak-helper/pkcs11-flatpak-2147483647'
                        self.assertLess(len(os.fsencode(child)), 108)
                        self.assertFalse(any(deep.iterdir()))
                    finally:
                        runtime.rmdir()

    def test_actual_private_helper_socket_binds_but_old_deep_location_is_too_long(self):
        with tempfile.TemporaryDirectory(dir='/tmp') as temporary:
            deep = Path(temporary) / ('generated-parent-' * 10) / ('generated-child-' * 6)
            deep.mkdir(parents=True)
            with patch.dict(os.environ, {'TMPDIR': str(deep)}), patch.object(tempfile, 'tempdir', None):
                old_runtime = Path(tempfile.mkdtemp(prefix='old-runtime-'))
                runtime = GUI.create_private_runtime('dockpipe-remote-runtime-')
                self.assertEqual(deep, old_runtime.parent)
                self.assertGreaterEqual(len(os.fsencode(old_runtime / 'pkcs11-flatpak-2147483647')), 108)
                helper = runtime / '.flatpak-helper'
                helper.mkdir(mode=0o700)
                selected = helper / 'pkcs11-flatpak-2147483647'
                try:
                    with socket.socket(socket.AF_UNIX) as stream:
                        with self.assertRaises(OSError):
                            stream.bind(str(old_runtime / 'pkcs11-flatpak-2147483647'))
                        stream.bind(str(selected))
                        self.assertTrue(stat.S_ISSOCK(selected.lstat().st_mode))
                finally:
                    if os.path.lexists(selected):
                        selected.unlink()
                    helper.rmdir()
                    runtime.rmdir()
                    old_runtime.rmdir()

    def test_freshness_refuses_after_first_entry_without_materializing_the_directory(self):
        runtime = Path(tempfile.mkdtemp(prefix='dockpipe-remote-runtime-', dir='/tmp'))
        class FirstEntryOnly:
            consumed = 0

            def __enter__(self):
                return self

            def __exit__(self, *_arguments):
                return False

            def __next__(self):
                self.consumed += 1
                if self.consumed > 1:
                    raise AssertionError('Directory scan must stop after its first entry')
                return object()
        entries = FirstEntryOnly()
        try:
            with patch.object(GUI.tempfile, 'mkdtemp', return_value=str(runtime)), \
                    patch.object(GUI.os, 'scandir', return_value=entries):
                with self.assertRaises(ValueError):
                    GUI.create_private_runtime('dockpipe-remote-runtime-')
            self.assertEqual(1, entries.consumed)
            self.assertTrue(runtime.exists())
        finally:
            runtime.rmdir()

    def test_invalid_scope_is_rejected_before_directory_creation(self):
        with patch.object(GUI.tempfile, 'mkdtemp') as create:
            for prefix in ['', '../escape', 'unowned-runtime-', '/tmp/absolute']:
                with self.subTest(prefix=prefix), self.assertRaises(ValueError):
                    GUI.create_private_runtime(prefix)
            create.assert_not_called()

    def test_wrong_parent_symlink_mode_type_and_nonfresh_directory_are_retained(self):
        with tempfile.TemporaryDirectory(dir='/tmp') as temporary:
            root = Path(temporary)
            wrong_parent = root / 'dockpipe-remote-runtime-generated'
            wrong_parent.mkdir(mode=0o700)
            with patch.object(GUI.tempfile, 'mkdtemp', return_value=str(wrong_parent)):
                with self.assertRaises(ValueError):
                    GUI.create_private_runtime('dockpipe-remote-runtime-')
            self.assertTrue(wrong_parent.exists())
            for case in ['symlink', 'mode', 'file', 'nonfresh']:
                selected = Path(tempfile.mkdtemp(prefix='dockpipe-remote-runtime-', dir='/tmp'))
                try:
                    if case == 'symlink':
                        selected.rmdir()
                        selected.symlink_to(wrong_parent, target_is_directory=True)
                    elif case == 'mode':
                        selected.chmod(0o755)
                    elif case == 'file':
                        selected.rmdir()
                        selected.write_text('generated-canary')
                    else:
                        (selected / 'generated-canary').write_text('preserve')
                    with patch.object(GUI.tempfile, 'mkdtemp', return_value=str(selected)):
                        with self.subTest(case=case), self.assertRaises(ValueError):
                            GUI.create_private_runtime('dockpipe-remote-runtime-')
                    self.assertTrue(os.path.lexists(selected))
                finally:
                    if selected.is_symlink() or selected.is_file():
                        selected.unlink()
                    else:
                        for child in selected.iterdir():
                            child.unlink()
                        selected.rmdir()


if __name__ == '__main__':
    unittest.main()
