"""Read-only paired metadata navigation cannot open or type into a viewer."""
from pathlib import Path
import importlib.util
import unittest
import tempfile
from types import SimpleNamespace
from unittest.mock import patch

SOURCE = Path(__file__).resolve().parents[1] / 'assets/scripts/installed-remote.py'
SPEC = importlib.util.spec_from_file_location('installed_remote_paired_tests', SOURCE)
REMOTE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(REMOTE)


class PairedPreparationTests(unittest.TestCase):
    def setUp(self):
        self.windows = [{'x': 0, 'y': 0, 'width': 1180, 'height': 1008}]

    def click(self, purpose, x, y, stage):
        return REMOTE.paired_prepare_action({'action': 'click', 'purpose': purpose, 'x': x, 'y': y},
                                            self.windows, stage)

    def test_only_sidebar_settings_escape_and_normal_close_are_admitted(self):
        _, stage = self.click('show-remote-desktop', 90, 674, 'initial')
        _, stage = self.click('show-remote-settings', 1100, 160, stage)
        command, stage = REMOTE.paired_prepare_action(
            {'action': 'key', 'purpose': 'close-settings', 'key': 'Escape'}, self.windows, stage)
        self.assertEqual('Escape', command[-1])
        _, stage = self.click('normal-quit', 1146, 24, stage)
        self.assertEqual('quit', stage)

    def test_connect_and_guessed_viewer_clicks_cannot_be_admitted(self):
        for purpose in ['connect', 'show-remote-settings', 'show-remote-desktop', 'normal-quit']:
            with self.subTest(purpose=purpose), self.assertRaises(ValueError):
                self.click(purpose, 580, 160, 'remote')
        with self.assertRaises(ValueError):
            self.click('show-remote-desktop', 90, 674, 'remote')

    def test_pair_menu_can_be_read_and_dismissed_without_selection(self):
        _, stage = self.click('show-paired-computers', 300, 115, 'remote')
        self.assertEqual('nodes', stage)
        _, stage = REMOTE.paired_prepare_action(
            {'action': 'key', 'purpose': 'close-paired-computers', 'key': 'Escape'}, self.windows, stage)
        self.assertEqual('remote', stage)
        with self.assertRaises(ValueError):
            REMOTE.paired_prepare_action(
                {'action': 'key', 'purpose': 'close-paired-computers', 'key': 'Return'}, self.windows, 'nodes')

    def test_only_observed_pair_row_and_mode_dropdown_can_be_inspected(self):
        _, stage = self.click('select-only-paired-computer', 320, 117, 'nodes')
        self.assertEqual('selected', stage)
        _, stage = self.click('show-desktop-mode', 480, 115, stage)
        self.assertEqual('mode', stage)
        _, selected = self.click('choose-view-mode', 480, 90, stage)
        self.assertEqual('selected', selected)
        with self.assertRaises(ValueError):
            self.click('choose-view-mode', 580, 115, stage)
        _, stage = REMOTE.paired_prepare_action(
            {'action': 'key', 'purpose': 'close-desktop-mode', 'key': 'Escape'}, self.windows, stage)
        self.assertEqual('selected', stage)
        with self.assertRaises(ValueError):
            self.click('select-only-paired-computer', 580, 115, 'nodes')
        with self.assertRaises(ValueError):
            REMOTE.paired_prepare_action(
                {'action': 'key', 'purpose': 'close-desktop-mode', 'key': 'Return'}, self.windows, 'mode')

    def test_general_keyboard_navigation_and_wrong_layout_are_refused(self):
        for key in ['Return', 'space', 'Tab', 'Super_L+2']:
            with self.subTest(key=key), self.assertRaises(ValueError):
                REMOTE.paired_prepare_action({'action': 'key', 'purpose': 'close-settings', 'key': key},
                                             self.windows, 'settings')
        self.windows[0]['x'] = 10
        with self.assertRaises(ValueError):
            self.click('show-remote-desktop', 90, 674, 'initial')


class NestedGuiIdentityTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.process = self.root / 'proc/321'
        (self.process / 'root').mkdir(parents=True)
        (self.process / 'exe').write_bytes(b'generated GUI artifact')
        self.sha256 = REMOTE.digest(self.process / 'exe')
        self.commit = 'a' * 64
        self.metadata = self.process / 'root/.flatpak-info'
        self.metadata.write_text('[Application]\nname=com.dockpipe.Dockpipe\n[Instance]\n'
                                 'instance-id=generated-instance\napp-commit=' + self.commit + '\n')
        self.fields = ['S'] + ['0'] * 19
        self.fields[1] = '320'
        self.fields[19] = '777'
        (self.process / 'stat').write_text('321 (generated) ' + ' '.join(self.fields))
        (self.process / 'status').write_text('NSpid:\t321\t8\t4\n')
        parent = self.root / 'proc/320'
        parent.mkdir()
        parent_fields = ['S'] + ['0'] * 19
        parent_fields[19] = '776'
        (parent / 'stat').write_text('320 (generated-init) ' + ' '.join(parent_fields))
        (parent / 'status').write_text('NSpid:\t320\t7\t1\n')
        self.inventory = 'generated-instance 7 com.dockpipe.Dockpipe staging ' + self.commit[:12] + '\n'

    def admit(self, inventory=None):
        def selected_path(value):
            return self.root / 'proc' if value == '/proc' else Path(value)
        with patch.object(REMOTE, 'owned_descendants', return_value={321: '777', 320: '776'}), \
                patch.object(REMOTE, 'Path', side_effect=selected_path), \
                patch.object(REMOTE.os, 'readlink', side_effect=lambda path:
                    'pid:[generated]' if str(path).endswith('/ns/pid') else '/app/libexec/dockpipe/dockpipe-launcher'), \
                patch.object(REMOTE.subprocess, 'run', return_value=SimpleNamespace(
                    stdout=self.inventory if inventory is None else inventory, stderr='')):
            return REMOTE.admitted_nested_gui({}, self.commit, self.sha256, 300)

    def test_namespace_local_flatpak_pid_binds_full_owned_gui_identity(self):
        result = self.admit()
        self.assertEqual(321, result['host_pid'])
        self.assertEqual([321, 8, 4], result['namespace_pids'])
        self.assertEqual('777', result['kernel_starttime'])
        self.assertEqual(self.commit, result['commit'])

    def test_reported_namespace_init_parent_can_bind_without_becoming_the_gui(self):
        result = self.admit()
        self.assertEqual(320, result['sandbox_pid'])
        self.assertEqual(320, result['admitted_init_host_pid'])
        self.assertEqual(7, result['public_child_pid_namespace'])
        self.assertEqual(321, result['host_pid'])
        self.assertEqual([321, 8, 4], result['namespace_pids'])

    def test_foreign_flatpak_instance_or_unmapped_pid_cannot_bind(self):
        for inventory in [self.inventory.replace('generated-instance', 'foreign-instance'),
                          self.inventory.replace(' 7 ', ' 900 '),
                          self.inventory.replace(self.commit[:12], 'b' * 12)]:
            with self.subTest(inventory=inventory), self.assertRaises(RuntimeError):
                self.admit(inventory)

    def test_full_namespace_metadata_and_kernel_starttime_must_match(self):
        self.metadata.write_text(self.metadata.read_text().replace(self.commit, 'b' * 64))
        with self.assertRaises(ValueError):
            self.admit()
        self.metadata.write_text(self.metadata.read_text().replace('b' * 64, self.commit))
        self.fields[19] = '778'
        (self.process / 'stat').write_text('321 (generated) ' + ' '.join(self.fields))
        with self.assertRaises(ValueError):
            self.admit()


if __name__ == '__main__':
    unittest.main()
