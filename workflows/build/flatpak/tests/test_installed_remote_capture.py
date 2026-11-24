"""Inert rectangle/bitmap bounds; actual X server proof is separate."""
from pathlib import Path
import ctypes
import importlib.util
import unittest

SOURCE = Path(__file__).resolve().parents[1] / 'assets/scripts/installed-remote-capture.py'
SPEC = importlib.util.spec_from_file_location('installed_remote_capture_tests', SOURCE)
CAPTURE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CAPTURE)


class InstalledRemoteCaptureTests(unittest.TestCase):
    def test_only_fixed_canvas_integer_rectangles_are_admitted(self):
        expected = {'x': 195, 'y': 52, 'width': 985, 'height': 90}
        self.assertEqual(expected, CAPTURE.rectangle(expected))
        for change in [{'x': -1}, {'y': -1}, {'width': 0}, {'height': 1001}, {'x': 1000}, {'x': 1.2}, {'x': True}]:
            with self.subTest(change=change), self.assertRaises(ValueError):
                CAPTURE.rectangle(dict(expected, **change))

    def image(self, bits=32):
        data = ctypes.create_string_buffer(bytes([10, 20, 30, 0, 40, 50, 60, 0]))
        image = CAPTURE.XImage(width=2, height=1, xoffset=0, format=2, data=ctypes.addressof(data),
                               byte_order=0, depth=24, bytes_per_line=8, bits_per_pixel=bits,
                               red_mask=0xff0000, green_mask=0xff00, blue_mask=0xff)
        return image, data

    def test_exact_requested_rect_only_is_decoded(self):
        image, retained = self.image()
        decoded = CAPTURE.decode_image(image, {'width': 2, 'height': 1})
        self.assertTrue(retained)
        self.assertEqual((2, 1), decoded.size)
        self.assertEqual((30, 20, 10), decoded.getpixel((0, 0)))
        self.assertEqual((60, 50, 40), decoded.getpixel((1, 0)))

    def test_unexpected_storage_or_geometry_is_rejected_before_memory_read(self):
        for field, value in [('width', 3), ('height', 2), ('depth', 32), ('bits_per_pixel', 16),
                             ('red_mask', 0), ('bytes_per_line', 2), ('data', 0)]:
            image, retained = self.image()
            setattr(image, field, value)
            with self.subTest(field=field), self.assertRaises(ValueError):
                CAPTURE.decode_image(image, {'width': 2, 'height': 1})
            self.assertTrue(retained)


if __name__ == '__main__':
    unittest.main()
