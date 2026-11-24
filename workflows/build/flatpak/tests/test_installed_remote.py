"""Inert installed-GUI identity, private launcher and coordinate admission checks."""
from pathlib import Path
import importlib.util
import io
import struct
import os
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

SOURCE = Path(__file__).resolve().parents[1] / 'assets/scripts/installed-remote.py'
SPEC = importlib.util.spec_from_file_location('installed_remote_tests', SOURCE)
REMOTE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(REMOTE)


class InstalledRemoteTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)

    def tearDown(self):
        self.temporary.cleanup()

    def test_fit_mapping_selects_current_fixture_center_with_letterboxing(self):
        viewport = {'x': 100, 'y': 100, 'width': 800, 'height': 600}
        window = {'x': 0, 'y': 0, 'width': 1000, 'height': 800}
        geometry = {'display_rect': {'x': 0, 'y': 0, 'width': 1600, 'height': 900},
                    'fixture_rect': {'x': 400, 'y': 225, 'width': 800, 'height': 450}}
        self.assertEqual({'x': 500, 'y': 400}, REMOTE.safe_focus_point(viewport, window, geometry))

    def test_mapping_rejects_other_display_clipped_viewport_and_nonfinite_bounds(self):
        viewport = {'x': 100, 'y': 100, 'width': 800, 'height': 600}
        window = {'x': 0, 'y': 0, 'width': 1000, 'height': 800}
        geometry = {'display_rect': {'x': 0, 'y': 0, 'width': 1600, 'height': 900},
                    'fixture_rect': {'x': 1800, 'y': 225, 'width': 800, 'height': 450}}
        with self.assertRaises(ValueError):
            REMOTE.safe_focus_point(viewport, window, geometry)
        geometry['fixture_rect']['x'] = 400
        with self.assertRaises(ValueError):
            REMOTE.safe_focus_point(dict(viewport, x=-10), window, geometry)
        with self.assertRaises(ValueError):
            REMOTE.safe_focus_point(dict(viewport, width=float('nan')), window, geometry)

    def test_preferences_require_all_three_explicit_clipboards_off(self):
        path = self.root / 'settings.conf'
        settings = ('[remote]\ndesktopShareClipboard=false\ndesktopShareImages=false\n'
                    'desktopShareFiles=false\ndesktopQuality=high\ndesktopFrameRate=10\n'
                    'desktopDecoder=software\ndesktopScale=fit\n')
        path.write_text(settings)
        REMOTE.remote_preferences(path)
        for name in ['desktopShareClipboard', 'desktopShareImages', 'desktopShareFiles']:
            path.write_text(settings.replace(name + '=false', name + '=true'))
            with self.subTest(name=name), self.assertRaises(ValueError):
                REMOTE.remote_preferences(path)

    def test_outer_launcher_hides_real_home_and_exposes_only_exact_ro_directory_fds(self):
        account = self.root / 'account'
        state = account / '.local/share/dockpipe/remote'
        security = account / '.local/state/dockpipe/security'
        state.mkdir(mode=0o700, parents=True)
        security.mkdir(mode=0o700, parents=True)
        private = self.root / 'private'
        private.mkdir(mode=0o700)
        configuration = {'provider_configuration': {'state': str(state)},
                         'installation': str(self.root),
                         'bwrap': {'path': '/usr/bin/bwrap', 'sha256': REMOTE.digest('/usr/bin/bwrap')}}
        with patch.object(REMOTE.pwd, 'getpwuid', return_value=SimpleNamespace(pw_dir=str(account))):
            command, environment, descriptors = REMOTE.private_launcher(
                ['flatpak', 'run', 'selected-app'], {'HOME': str(private)}, configuration, private, [self.root])
        try:
            self.assertEqual(str(account), environment['HOME'])
            self.assertIn('--unshare-user', command)
            self.assertIn('--unshare-pid', command)
            self.assertEqual(3, command.count('--ro-bind-fd'))
            self.assertIn('--tmpfs', command)
            self.assertIn('--bind-fd', command)
            self.assertEqual(4, len(descriptors))
            self.assertEqual(state.stat().st_ino, os.fstat(descriptors[0]).st_ino)
            self.assertEqual(security.stat().st_ino, os.fstat(descriptors[1]).st_ino)
            self.assertIn(['--bind', str(private), str(account)],
                          [command[index:index + 3] for index in range(len(command) - 2)])
        finally:
            for descriptor in descriptors:
                os.close(descriptor)

    def test_owned_xwd_decodes_actual_24bit_and_32bit_rows_in_memory(self):
        for bits_per_pixel in [24, 32]:
            header = [107, 7, 2, 24, 1600, 1000, 0, 0, 32, 0, 8, bits_per_pixel,
                      6400, 5, 0xff0000, 0xff00, 0xff, 8, 256, 0, 1600, 1000, 0, 0, 0]
            pixels = bytearray(6400 * 1000)
            pixels[:3] = bytes([10, 20, 30])
            data = struct.pack('!25I', *header) + b'owned\0\0' + pixels
            response = SimpleNamespace(stdout=data)
            with self.subTest(bits_per_pixel=bits_per_pixel), patch.object(REMOTE.subprocess, 'run', return_value=response):
                image = REMOTE.capture_owned_display({'DISPLAY': ':owned'})
                self.assertEqual((30, 20, 10), image.getpixel((0, 0)))
                self.assertEqual((1600, 1000), image.size)

    def test_owned_xwd_rejects_unexpected_geometry_before_decoding(self):
        header = [107, 7, 2, 24, 1920, 1080, 0, 0, 32, 0, 8, 32,
                  7680, 5, 0xff0000, 0xff00, 0xff, 8, 256, 0, 1920, 1080, 0, 0, 0]
        with patch.object(REMOTE.subprocess, 'run', return_value=SimpleNamespace(stdout=struct.pack('!25I', *header))), \
                self.assertRaises(ValueError):
            REMOTE.capture_owned_display({'DISPLAY': ':owned'})

    def test_facade_response_cannot_change_protocol_sequence_action_or_shape(self):
        valid = {'schema': 'dockpipe.desktop.fixture-facade/v1', 'sequence': 1,
                 'action': 'gate', 'result': {'gate_nonce': 'generated'}}
        for changed in [dict(valid, schema='another-protocol'), dict(valid, sequence=2),
                        dict(valid, action='close'), dict(valid, result=[]), dict(valid, result=None)]:
            client = object.__new__(REMOTE.FacadeClient)
            client.sequence = 1
            client.expected_schema = 'dockpipe.desktop.fixture-facade/v1'
            client.output = self.root
            client.process = SimpleNamespace(stdin=io.BytesIO())
            client.event = lambda _timeout, reply=changed: reply
            with self.subTest(reply=changed), self.assertRaises(ValueError):
                client.command('gate', purpose='focus')
            self.assertEqual(1, client.sequence)
            self.assertFalse(list(self.root.glob('reply-*.json')))

    def test_outer_launcher_refuses_writable_real_home(self):
        account = self.root / 'account'
        account.mkdir()
        private = account / 'private'
        private.mkdir()
        with patch.object(REMOTE.pwd, 'getpwuid', return_value=SimpleNamespace(pw_dir=str(account))), \
                self.assertRaises(ValueError):
            REMOTE.private_launcher([], {}, {}, private, [])


if __name__ == '__main__':
    unittest.main()
