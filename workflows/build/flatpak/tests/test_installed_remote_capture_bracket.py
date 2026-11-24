"""Direct Xlib metadata/layout fault fixtures; no display or pixels from an app."""
from pathlib import Path
import ctypes
import importlib.util
import os
import unittest
from unittest.mock import patch

SOURCE = Path(__file__).resolve().parents[1] / 'assets/scripts/installed-remote-capture.py'
SPEC = importlib.util.spec_from_file_location('remote_capture_bracket_tests', SOURCE)
CAPTURE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CAPTURE)


class GeneratedXlib:
    def __init__(self, clock):
        self.clock = clock
        self.attributes = {'width': 2, 'height': 1, 'root': 1000, 'window_class': 1,
                           'map_state': 2, 'depth': 24, 'border_width': 0}
        self.pid = os.getpid()
        self.property_type = 6
        self.property_format = 32
        self.property_count = 1
        self.property_tail = 0
        self.property_status = 0
        self.handler = None
        self.snapshot_count = 0
        self.property_allocations = 0
        self.property_frees = 0
        self.image_allocations = 0
        self.image_frees = 0
        self.after_image = None
        self.change_second_snapshot = False
        self.fail_attributes = False
        self.storage = ctypes.create_string_buffer(b'\x0a\x14\x1e\x00' * 2)
        self.image = CAPTURE.XImage(width=2, height=1, xoffset=0, format=2,
            data=ctypes.addressof(self.storage), byte_order=0, depth=24, bytes_per_line=8,
            bits_per_pixel=32, red_mask=0xff0000, green_mask=0xff00, blue_mask=0xff)
        self.pid_storage = (ctypes.c_ulong * 2)(self.pid, 0)

    def XSetErrorHandler(self, handler):
        previous = self.handler
        self.handler = handler
        return previous

    def XSync(self, display, discard):
        return 0

    def XGetWindowAttributes(self, display, window, output):
        self.snapshot_count += 1
        if self.fail_attributes:
            callback = ctypes.CFUNCTYPE(ctypes.c_int, ctypes.c_void_p,
                                        ctypes.POINTER(CAPTURE.XErrorEvent))(self.handler.value)
            error = CAPTURE.XErrorEvent(resource_id=window, error_code=3)
            callback(display, ctypes.pointer(error))
            return 0
        result = ctypes.cast(output, ctypes.POINTER(CAPTURE.XWindowAttributes)).contents
        for name, value in self.attributes.items():
            setattr(result, name, value)
        if self.change_second_snapshot and self.snapshot_count == 2:
            result.width += 1
        return 1

    def XTranslateCoordinates(self, display, window, root, x, y, output_x, output_y, child):
        ctypes.cast(output_x, ctypes.POINTER(ctypes.c_int))[0] = 0
        ctypes.cast(output_y, ctypes.POINTER(ctypes.c_int))[0] = 0
        return 1

    def XGetWindowProperty(self, display, window, atom, offset, length, delete, expected,
                           actual_type, actual_format, count, tail, output):
        assert length == 1 and delete == 0 and expected == 6
        ctypes.cast(actual_type, ctypes.POINTER(ctypes.c_ulong))[0] = self.property_type
        ctypes.cast(actual_format, ctypes.POINTER(ctypes.c_int))[0] = self.property_format
        ctypes.cast(count, ctypes.POINTER(ctypes.c_ulong))[0] = self.property_count
        ctypes.cast(tail, ctypes.POINTER(ctypes.c_ulong))[0] = self.property_tail
        self.pid_storage[0] = self.pid
        ctypes.cast(output, ctypes.POINTER(ctypes.POINTER(ctypes.c_ubyte)))[0] = ctypes.cast(
            self.pid_storage, ctypes.POINTER(ctypes.c_ubyte))
        self.property_allocations += 1
        return self.property_status

    def XFree(self, pointer):
        self.property_frees += 1

    def XGetImage(self, *arguments):
        self.image_allocations += 1
        if self.after_image is not None:
            self.clock[0] = self.after_image
        return ctypes.pointer(self.image)

    def XDestroyImage(self, pointer):
        self.image_frees += 1


class BracketTests(unittest.TestCase):
    def setUp(self):
        self.clock = [100.0]
        self.library = GeneratedXlib(self.clock)
        self.reader = CAPTURE.OwnedDisplayCapture.__new__(CAPTURE.OwnedDisplayCapture)
        self.reader.library = self.library
        self.reader.display = 101
        self.reader.window = 1000
        self.reader.pid_atom = 10
        self.reader.active_frame = None
        fields = Path('/proc/self/stat').read_text().rsplit(')', 1)[1].split()
        namespace = next(line.split()[1:] for line in Path('/proc/self/status').read_text().splitlines()
                         if line.startswith('NSpid:'))
        self.window = {'id': 42, 'x': 0, 'y': 0, 'width': 2, 'height': 1,
                       'gui': {'pid': os.getpid(), 'starttime': fields[19],
                               'namespace_pid': int(namespace[-1])}}
        self.library.pid = self.window['gui']['namespace_pid']
        self.rectangle = {'x': 0, 'y': 0, 'width': 2, 'height': 1}

    def capture(self, facts=None):
        return self.reader.capture_bracketed(self.rectangle, self.window, 100.1, 100.05, facts)

    def check_freed(self):
        self.assertEqual(self.library.image_allocations, self.library.image_frees)
        self.assertEqual(self.library.property_allocations, self.library.property_frees)
        self.assertIsNone(self.library.handler)

    def test_exact_bracket_keeps_raw_frame_live_only_inside_context(self):
        facts = {}
        with patch.object(CAPTURE.time, 'monotonic', side_effect=lambda: self.clock[0]):
            with self.capture(facts) as frame:
                self.assertEqual(frame['before'], frame['after'])
                self.assertEqual(0, self.library.image_frees)
                image = self.reader.decode_frame(frame)
                self.assertEqual((30, 20, 10), image.getpixel((0, 0)))
            with self.assertRaisesRegex(ValueError, 'no longer live'):
                self.reader.decode_frame(frame)
        self.check_freed()
        self.assertTrue(facts['image_destroyed_once'])

    def test_invalid_window_ownership_layout_or_pid_property_fails_and_frees(self):
        changes = [('property_type', 1), ('property_format', 8), ('property_count', 0),
                   ('property_count', 2), ('property_tail', 4), ('property_status', 1), ('pid', -1),
                   ('root', 999), ('map_state', 0), ('window_class', 2), ('depth', 32), ('border_width', 1)]
        for name, value in changes:
            self.setUp()
            if name in self.library.attributes:
                self.library.attributes[name] = value
            else:
                setattr(self.library, name, value)
            with self.subTest(name=name, value=value), \
                    patch.object(CAPTURE.time, 'monotonic', return_value=100.0):
                with self.assertRaises(ValueError):
                    with self.capture():
                        self.fail('Invalid metadata must not publish an admitted frame')
            self.check_freed()

    def test_geometry_change_protocol_error_and_invalid_image_free_exactly_once(self):
        for fault in ['geometry', 'BadWindow', 'image-layout']:
            self.setUp()
            if fault == 'geometry':
                self.library.change_second_snapshot = True
            elif fault == 'BadWindow':
                self.library.fail_attributes = True
            else:
                self.library.image.bytes_per_line = 1
            with self.subTest(fault=fault), patch.object(CAPTURE.time, 'monotonic', return_value=100.0):
                with self.assertRaises(ValueError):
                    with self.capture():
                        self.fail('Faulted frame was admitted')
            self.check_freed()

    def test_capture_enforces_both_original_deadlines_and_keeps_ram_timestamps(self):
        for capture_before, pause_before, received in [(100.03, 100.05, 100.04),
                                                       (100.1, 100.05, 100.06)]:
            self.setUp()
            self.library.after_image = received
            facts = {}
            with self.subTest(capture_before=capture_before), \
                    patch.object(CAPTURE.time, 'monotonic', side_effect=lambda: self.clock[0]):
                with self.assertRaisesRegex(ValueError, 'deadline elapsed'):
                    with self.reader.capture_bracketed(self.rectangle, self.window,
                                                      capture_before, pause_before, facts):
                        self.fail('Late frame was admitted')
            self.assertEqual(received, facts['image_received_monotonic'])
            self.check_freed()

    def test_exception_after_yield_destroys_frame_once(self):
        with patch.object(CAPTURE.time, 'monotonic', return_value=100.0):
            with self.assertRaisesRegex(ValueError, 'generated decode fault'):
                with self.capture():
                    raise ValueError('generated decode fault')
        self.check_freed()

    def test_window_clipped_below_canvas_preserves_exact_full_geometry(self):
        self.window['height'] = 1008
        self.library.attributes['height'] = 1008
        with patch.object(CAPTURE.time, 'monotonic', return_value=100.0):
            with self.capture() as frame:
                self.assertEqual(1008, frame['before']['height'])
                self.assertEqual(frame['before'], frame['after'])
                self.assertEqual({'x': 0, 'y': 0, 'width': 2, 'height': 1}, frame['visible_owned_rect'])
        self.check_freed()


if __name__ == '__main__':
    unittest.main()
