"""Fault tests for the generic effect-bounded X11 owned-window primitive."""
import hashlib
import importlib.util
import os
from pathlib import Path
import struct
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch
import zlib


SOURCE = Path(__file__).resolve().parents[1] / 'platforms/linux/ui.py'
SPEC = importlib.util.spec_from_file_location('owned_window_ui', SOURCE)
UI = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(UI)


def image_bytes(bits=32, order=0, colors=0):
    header = [101, 7, 2, 24, 2, 1, 0, order, 32, order, 32, bits, 8,
              4, 0xff0000, 0xff00, 0xff, 8, 256, colors, 2, 1, 0, 0, 0]
    pixels = b''.join(value.to_bytes(bits // 8, 'little' if order == 0 else 'big')
                      for value in (0xff0000, 0x00ff00))
    return struct.pack('!25I', *header) + b'\0' + b'\0' * (colors * 12) + pixels.ljust(8, b'\0')


def png_pixels(image):
    offset = 8
    compressed = bytearray()
    while offset < len(image):
        length = struct.unpack('!I', image[offset:offset + 4])[0]
        kind = image[offset + 4:offset + 8]
        contents = image[offset + 8:offset + 8 + length]
        checksum = struct.unpack('!I', image[offset + 8 + length:offset + 12 + length])[0]
        assert checksum == zlib.crc32(kind + contents)
        if kind == b'IDAT':
            compressed.extend(contents)
        offset += length + 12
    return zlib.decompress(compressed)


class OwnedWindowCaptureTests(unittest.TestCase):
    def setUp(self):
        self.deadline = time.monotonic() + 5
        self.owner = {'pid': 1234, 'starttime': '100', 'exe_sha256': 'a' * 64}
        self.window = {'id': 6291519, 'x': 12, 'y': 20, 'width': 2, 'height': 1}
        self.environment = {'DISPLAY': ':123', 'XAUTHORITY': '/private/authority'}
        self.tools = {name: {'path': '/usr/bin/' + name, 'sha256': 'b' * 64}
                      for name in ('xwd', 'xprop', 'xwininfo')}

    def test_truecolor_orders_and_pixel_widths_have_exact_red_green_pixels(self):
        for bits in (24, 32):
            for order in (0, 1):
                for colors in (0, 256):
                    with self.subTest(bits=bits, order=order, colors=colors):
                        image = UI._capture_png(image_bytes(bits, order, colors), 2, 1,
                                                lambda: False, self.deadline)
                        self.assertEqual(png_pixels(image), b'\0\xff\0\0\0\xff\0')

    def test_maximum_header_and_color_records_remain_within_raw_contract(self):
        original = image_bytes(colors=256)
        expanded = bytearray(original[:100] + b'X' * (4096 - 101) + b'\0' + original[101:])
        struct.pack_into('!I', expanded, 0, 4096)
        self.assertEqual(len(expanded), 4096 + 256 * 12 + 8)
        image = UI._capture_png(expanded, 2, 1, lambda: False, self.deadline)
        self.assertEqual(png_pixels(image), b'\0\xff\0\0\0\xff\0')

    def test_header_layout_truncation_and_trailing_payload_are_refused(self):
        original = image_bytes()
        for field, value in ((0, 99), (1, 6), (2, 1), (3, 8), (4, 3), (5, 2),
                             (6, 1), (7, 2), (8, 64), (9, 2), (10, 8), (11, 16),
                             (12, 12), (13, 3), (14, 0xff), (19, 257)):
            damaged = bytearray(original)
            struct.pack_into('!I', damaged, field * 4, value)
            with self.subTest(field=field), self.assertRaises(ValueError):
                UI._capture_png(damaged, 2, 1, lambda: False, self.deadline)
        for damaged in (original[:99], original[:-1], original + b'X'):
            with self.assertRaises(ValueError):
                UI._capture_png(damaged, 2, 1, lambda: False, self.deadline)
        with self.assertRaises(ValueError):
            UI._capture_png(original, 4097, 1, lambda: False, self.deadline)

    def test_pid_namespace_mapping_is_derived_never_caller_supplied(self):
        self.assertEqual(UI._capture_namespace_pid('Name:\tapp\nNSpid:\t1234\t2\n', 1234), 2)
        self.assertEqual(UI._capture_namespace_pid('NSpid:\t1234\n', 1234), 1234)
        for status in ('NSpid:\t9999\t2\n', 'NSpid:\t1234\t0\n', '',
                       'NSpid:\t1234\nNSpid:\t1234\n'):
            with self.assertRaises(ValueError):
                UI._capture_namespace_pid(status, 1234)

    def test_current_process_requires_kernel_start_and_executable_digest(self):
        raw = Path('/proc/self/stat').read_text()
        owner = {'pid': os.getpid(), 'starttime': raw.rsplit(')', 1)[1].split()[19],
                 'exe_sha256': hashlib.sha256(Path('/proc/self/exe').read_bytes()).hexdigest()}
        self.assertGreater(UI._capture_process(owner, lambda: False, self.deadline), 0)
        for field, value in (('starttime', '0'), ('exe_sha256', '0' * 64)):
            with self.subTest(field=field), self.assertRaises(ValueError):
                UI._capture_process({**owner, field: value}, lambda: False, self.deadline)

    def test_foreign_pid_geometry_drift_and_hidden_window_are_refused(self):
        information = b'Absolute upper-left X: 12\nAbsolute upper-left Y: 20\nWidth: 2\nHeight: 1\nMap State: IsViewable\n'
        commands = [b'_NET_WM_PID(CARDINAL) = 2\n', information]
        with patch.object(UI, '_capture_process', return_value=2), patch.object(
                UI, '_capture_command', side_effect=commands) as command:
            UI._capture_window(self.owner, self.window, {name: name for name in self.tools},
                               self.environment, lambda: False, self.deadline)
            self.assertEqual(command.call_args_list[0].args[0], ['xprop', '-id', '6291519', '_NET_WM_PID'])
        for responses in ([b'_NET_WM_PID(CARDINAL) = 1234\n'],
                          [commands[0], information.replace(b'Width: 2', b'Width: 3')],
                          [commands[0], information.replace(b'IsViewable', b'IsUnMapped')]):
            with patch.object(UI, '_capture_process', return_value=2), patch.object(
                    UI, '_capture_command', side_effect=responses), self.assertRaises(ValueError):
                UI._capture_window(self.owner, self.window, {name: name for name in self.tools},
                                   self.environment, lambda: False, self.deadline)

    def test_private_display_auth_command_binding_is_required(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            authority = root / 'authority'
            authority.write_bytes(b'fixture')
            authority.chmod(0o600)
            environment = {'DISPLAY': ':123', 'XAUTHORITY': str(authority)}
            for command in (b'Xvfb\0:124\0-auth\0' + os.fsencode(authority) + b'\0',
                            b'Xvfb\0:123\0-auth\0/other/authority\0'):
                with patch.object(UI, '_capture_process'), patch.object(Path, 'open') as opened:
                    opened.return_value.__enter__.return_value.read.return_value = command
                    with self.assertRaises(ValueError):
                        UI._capture_environment(environment, self.owner, lambda: False, self.deadline)
            authority.chmod(0o644)
            with self.assertRaisesRegex(ValueError, 'private'):
                UI._capture_environment(environment, self.owner, lambda: False, self.deadline)

    def test_capture_only_publishes_after_post_admission_and_never_uses_root(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / 'window.png'
            authority = Path(temporary) / 'authority'
            authority.write_bytes(b'fixture')
            self.environment['XAUTHORITY'] = str(authority)
            with patch.object(UI, '_capture_path', side_effect=lambda value: Path(value)), patch.object(
                    UI, '_capture_digest', return_value='b' * 64), patch.object(os, 'access', return_value=True), patch.object(
                    UI, '_capture_environment', return_value=self.environment), patch.object(
                    UI, '_capture_window') as admission, patch.object(UI, '_capture_command', return_value=image_bytes()) as command:
                receipt = UI.capture_owned_window(self.owner, self.owner, self.window, self.environment, self.tools, output)
                self.assertEqual(admission.call_count, 2)
                self.assertEqual(command.call_args.args[0], ['/usr/bin/xwd', '-silent', '-id', '6291519'])
                self.assertEqual(command.call_args.args[2], 4096 + 256 * 12 + 8)
                self.assertEqual(receipt['sha256'], hashlib.sha256(output.read_bytes()).hexdigest())
                self.assertEqual(output.stat().st_mode & 0o777, 0o600)
                self.assertFalse(receipt['input_sent'])
                output.unlink()
                admission.side_effect = [None, ValueError('Window changed')]
                with self.assertRaisesRegex(ValueError, 'Window changed'):
                    UI.capture_owned_window(self.owner, self.owner, self.window, self.environment, self.tools, output)
                self.assertFalse(output.exists())

    def test_changed_tool_and_authority_refuse_without_publishing(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / 'window.png'
            authority = Path(temporary) / 'authority'
            authority.write_bytes(b'fixture')
            self.environment['XAUTHORITY'] = str(authority)
            for changed in ('tool', 'authority', 'namespace'):
                with self.subTest(changed=changed), patch.object(
                        UI, '_capture_path', side_effect=lambda value: Path(value)), patch.object(
                        UI, '_capture_digest', side_effect=['b' * 64] * 3 + ['a' * 64]
                        if changed == 'tool' else None, return_value='b' * 64), patch.object(
                        os, 'access', return_value=True), patch.object(
                        UI, '_capture_environment', return_value=self.environment), patch.object(
                        UI, '_capture_window', side_effect=[{'x11_pid': 2}, {'x11_pid': 3}]
                        if changed == 'namespace' else None, return_value={'x11_pid': 2}), patch.object(
                        UI, '_capture_command') as command:
                    def capture(*arguments):
                        if changed == 'authority':
                            authority.write_bytes(b'changed authority')
                        return image_bytes()
                    command.side_effect = capture
                    with self.assertRaises(ValueError):
                        UI.capture_owned_window(self.owner, self.owner, self.window,
                                                self.environment, self.tools, output)
                    self.assertFalse(output.exists())

    def test_capture_refuses_preimage_and_invalid_pins_before_any_tool(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / 'preimage.png'
            output.write_bytes(b'preserved')
            with patch.object(UI, '_capture_path', side_effect=lambda value: Path(value)), patch.object(
                    UI, '_capture_digest', return_value='b' * 64), patch.object(
                    os, 'access', return_value=True), patch.object(UI, '_capture_command') as command:
                with self.assertRaises(ValueError):
                    UI.capture_owned_window(self.owner, self.owner, self.window,
                                            self.environment, self.tools, output)
                self.assertEqual(output.read_bytes(), b'preserved')
                command.assert_not_called()
            output.unlink()
            with patch.object(UI, '_capture_path', side_effect=lambda value: Path(value)), patch.object(
                    UI, '_capture_digest', return_value='a' * 64), patch.object(
                    os, 'access', return_value=True), patch.object(UI, '_capture_command') as command:
                with self.assertRaisesRegex(ValueError, 'digest'):
                    UI.capture_owned_window(self.owner, self.owner, self.window,
                                            self.environment, self.tools, output)
                command.assert_not_called()
                self.assertFalse(output.exists())

    @unittest.skipUnless(hasattr(os, 'fork'), 'Process-group fault fixture requires POSIX')
    def test_cancel_releases_inherited_pipe_and_owned_tool_descendant(self):
        with tempfile.TemporaryDirectory() as temporary:
            marker = Path(temporary) / 'child.pid'
            script = """import os, sys, time
from pathlib import Path
child = os.fork()
if child:
    os._exit(0)
Path(sys.argv[1]).write_text(str(os.getpid()))
time.sleep(10)
"""
            with self.assertRaises(UI.CaptureCancelled):
                UI._capture_command([sys.executable, '-c', script, str(marker)], {}, 4096,
                                    marker.exists, time.monotonic() + 5)
            child = int(marker.read_text())
            status = Path('/proc') / str(child) / 'stat'
            if status.exists():
                # An orphan zombie can await PID1 reaping, but cannot execute or hold pipes.
                self.assertEqual(status.read_text().rsplit(')', 1)[1].split()[0], 'Z')

    def test_publication_is_atomic_private_and_never_replaces_raced_preimage(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / 'image.png'
            image = b'complete image bytes'
            real_link = os.link
            observations = []
            def publish(source, destination, **options):
                self.assertFalse(output.exists())
                self.assertEqual(Path(source).read_bytes(), image)
                self.assertEqual(Path(source).stat().st_mode & 0o777, 0o600)
                observations.append('complete-private-before-publication')
                return real_link(source, destination, **options)
            with patch.object(os, 'link', side_effect=publish):
                UI._capture_publish(output, image, lambda: False, self.deadline)
            self.assertEqual(output.read_bytes(), image)
            self.assertEqual(output.stat().st_nlink, 1)
            self.assertEqual(observations, ['complete-private-before-publication'])
            self.assertEqual(list(Path(temporary).glob('.owned-window-*')), [])
            output.unlink()
            def race(source, destination, **options):
                output.write_bytes(b'foreign raced preimage')
                return real_link(source, destination, **options)
            with patch.object(os, 'link', side_effect=race), self.assertRaises(FileExistsError):
                UI._capture_publish(output, image, lambda: False, self.deadline)
            self.assertEqual(output.read_bytes(), b'foreign raced preimage')
            self.assertEqual(list(Path(temporary).glob('.owned-window-*')), [])

    def test_partial_write_failure_and_cancel_publish_nothing_and_release_temp(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / 'image.png'
            real_fdopen = os.fdopen
            class FailedWriter:
                def __init__(self, descriptor):
                    self.stream = real_fdopen(descriptor, 'wb')
                def __enter__(self):
                    return self
                def __exit__(self, *arguments):
                    self.stream.close()
                def write(self, contents):
                    self.stream.write(contents[:3])
                    self.stream.flush()
                    self_test.assertFalse(output.exists())
                    raise OSError('Synthetic partial write')
            self_test = self
            with patch.object(os, 'fdopen', side_effect=lambda descriptor, mode: FailedWriter(descriptor)):
                with self.assertRaisesRegex(OSError, 'partial write'):
                    UI._capture_publish(output, b'complete', lambda: False, self.deadline)
            self.assertFalse(output.exists())
            self.assertEqual(list(Path(temporary).glob('.owned-window-*')), [])
            with self.assertRaises(UI.CaptureCancelled):
                UI._capture_publish(output, b'complete', lambda: True, self.deadline)
            self.assertFalse(output.exists())
            self.assertEqual(list(Path(temporary).glob('.owned-window-*')), [])

    def test_malformed_selection_types_fail_deliberately_and_nonfinite_deadlines_refuse(self):
        for invalid in (None, [], 'identity', {'pid': []}):
            with self.assertRaises(ValueError):
                UI._capture_process(invalid, lambda: False, self.deadline)
        for invalid in (None, [], {'DISPLAY': 123, 'XAUTHORITY': None}):
            with self.assertRaises(ValueError):
                UI._capture_environment(invalid, self.owner, lambda: False, self.deadline)
        for name, values in (('window', (None, [], {'id': []})),
                             ('tools', (None, [], {'xwd': None, 'xprop': [], 'xwininfo': 123})),
                             ('timeout', (float('inf'), float('nan'), True, '15'))):
            for value in values:
                arguments = dict(owner=self.owner, display_owner=self.owner, window=self.window,
                                 environment=self.environment, tools=self.tools, output='/private/window.png')
                arguments[name] = value
                with self.subTest(name=name, value=value), self.assertRaises(ValueError):
                    UI.capture_owned_window(**arguments)

    def test_cancel_timeout_output_exhaustion_and_tool_error_reap_exact_child(self):
        real_popen = subprocess.Popen
        children = []
        def start(*args, **kwargs):
            child = real_popen(*args, **kwargs)
            children.append(child)
            return child
        cancellation_checks = iter((False, True))
        cases = [(['-c', 'import time; time.sleep(10)'], 16, lambda: next(cancellation_checks), self.deadline, UI.CaptureCancelled),
                 (['-c', 'import sys; sys.stdout.buffer.write(b"x"*100)'], 16, lambda: False, self.deadline, ValueError),
                 (['-c', 'raise SystemExit(7)'], 16, lambda: False, self.deadline, ValueError)]
        with patch.object(subprocess, 'Popen', side_effect=start):
            for arguments, maximum, cancelled, deadline, error in cases:
                with self.assertRaises(error):
                    UI._capture_command([sys.executable, *arguments], {}, maximum, cancelled, deadline)
                self.assertIsNotNone(children[-1].poll())
                self.assertTrue(children[-1].stdout.closed)
        with patch.object(subprocess, 'Popen') as start:
            with self.assertRaises(TimeoutError):
                UI._capture_command([sys.executable], {}, 16, lambda: False, time.monotonic() - 1)
            with self.assertRaises(UI.CaptureCancelled):
                UI._capture_command([sys.executable], {}, 16, lambda: True, self.deadline)
            start.assert_not_called()
        with self.assertRaises(UI.CaptureCancelled):
            UI._capture_png(image_bytes(), 2, 1, lambda: True, self.deadline)


if __name__ == '__main__':
    unittest.main()
