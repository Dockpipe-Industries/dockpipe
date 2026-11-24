"""Header-only XWD failure diagnostics; no display, capture command or raw output."""

import json
import os
from pathlib import Path
import struct
import tempfile
import time
import unittest
from unittest.mock import patch

from test_owned_window_capture import UI, image_bytes, png_pixels


class XwdDiagnosticTests(unittest.TestCase):
    def rejected(self, raw, width=2, height=1):
        with self.assertRaises(UI.XwdLayoutError) as caught:
            UI._capture_png(raw, width, height, lambda: False, time.monotonic() + 2)
        self.assertIsInstance(caught.exception, ValueError)
        return caught.exception, UI.capture_failure_diagnostic(caught.exception)

    def test_each_original_layout_rejection_has_finite_typed_check_evidence(self):
        cases = ((0, 99, 'header-size'), (1, 6, 'file-version'), (2, 1, 'pixmap-format'),
                 (3, 8, 'pixmap-depth'), (4, 3, 'captured-width'), (5, 2, 'captured-height'),
                 (6, 1, 'x-offset'), (7, 2, 'byte-order'), (8, 64, 'bitmap-unit'),
                 (9, 2, 'bitmap-bit-order'), (10, 8, 'bitmap-pad'), (11, 16, 'bits-per-pixel'),
                 (12, 12, 'padded-stride'), (13, 3, 'visual-class'),
                 (14, 0xff, 'rgb-masks'), (19, 257, 'color-count'))
        for field, value, expected in cases:
            raw = bytearray(image_bytes())
            struct.pack_into('!I', raw, field * 4, value)
            with self.subTest(field=field):
                error, diagnostic = self.rejected(raw)
                self.assertEqual(str(error), 'Unsupported or inconsistent bounded XWD layout')
                self.assertIn(expected, diagnostic['failed_checks'])
                self.assertEqual(len(diagnostic['header_fields']), 25)
                self.assertTrue(all(type(value) is int for value in diagnostic['header_fields'].values()))
                self.assertLess(len(json.dumps(diagnostic).encode()), 8192)
        _, extra = self.rejected(image_bytes() + b'X')
        self.assertIn('exact-payload-length', extra['failed_checks'])
        raw = bytearray(image_bytes())
        raw[100] = 65
        _, unterminated = self.rejected(raw)
        self.assertIn('window-name-terminated', unterminated['failed_checks'])
        self.assertFalse(unterminated['window_name_terminated'])

    def test_window_title_color_records_pixels_and_exception_text_never_serialize(self):
        title = b'PRIVATE-WINDOW-TITLE-SENTINEL'
        colors = b'PRIVATE-COLOR-RECORD-SENTINEL'
        pixels = b'PRIVATE-PIXELS-SENTINEL'
        raw = bytearray(image_bytes())
        offset = 100 + len(title) + 1
        raw = raw[:100] + title + b'\0' + colors + pixels
        struct.pack_into('!I', raw, 0, offset)
        error, diagnostic = self.rejected(raw)
        error.args = ('PRIVATE-EXCEPTION-TEXT-SENTINEL',)
        rendered = json.dumps(UI.capture_failure_diagnostic(error))
        for secret in (title, colors, pixels, b'PRIVATE-EXCEPTION-TEXT-SENTINEL'):
            self.assertNotIn(secret.decode(), rendered)
        self.assertFalse(diagnostic['window_name_and_pixels_retained'])
        self.assertEqual(diagnostic['raw_length'], len(raw))

    def test_truncated_header_has_only_available_numeric_metadata(self):
        for raw in (b'', b'private text', image_bytes()[:99]):
            error, diagnostic = self.rejected(raw)
            self.assertEqual(str(error), 'Truncated XWD header')
            self.assertEqual(diagnostic['header_fields'], {})
            self.assertEqual(diagnostic['raw_length'], len(raw))
            self.assertEqual(diagnostic['required_header_bytes'], 100)
            self.assertFalse(diagnostic['header_complete'])
            self.assertNotIn('private text', json.dumps(diagnostic))

    def test_offset_outside_raw_and_maximal_header_values_remain_bounded(self):
        raw = bytearray(image_bytes())
        struct.pack_into('!I', raw, 0, 4096)
        _, diagnostic = self.rejected(raw)
        self.assertIsNone(diagnostic['window_name_terminated'])
        self.assertIn('window-name-terminated', diagnostic['unavailable_checks'])
        header = tuple([0xffffffff] * 25)
        error = UI.XwdLayoutError('xwd-layout-unsupported', header, 4096, 4096, 100, None)
        diagnostic = UI.capture_failure_diagnostic(error)
        self.assertIsNotNone(diagnostic)
        self.assertLessEqual(diagnostic['expected_total_bytes'], 1 << 45)
        self.assertLessEqual(diagnostic['expected_stride'], 1 << 45)
        self.assertLessEqual(len(diagnostic['failed_checks']), 18)
        self.assertLess(len(json.dumps(diagnostic).encode()), 8192)

    def test_unrelated_errors_or_hostile_attributes_are_not_observed(self):
        class Hostile(ValueError):
            def __getattribute__(self, name):
                raise AssertionError('No arbitrary exception attribute access')

            def __str__(self):
                raise AssertionError('No exception text conversion')

        self.assertIsNone(UI.capture_failure_diagnostic(Hostile()))
        self.assertIsNone(UI.capture_failure_diagnostic(ValueError('private')))
        error = UI.XwdLayoutError.__new__(UI.XwdLayoutError)
        self.assertIsNone(UI.capture_failure_diagnostic(error))

    def test_slot_accessor_ignores_hostile_exception_dictionary(self):
        raw = bytearray(image_bytes())
        struct.pack_into('!I', raw, 13 * 4, 5)
        error, expected = self.rejected(raw)
        class HostileKey:
            def __hash__(self):
                return hash('_xwd_reason')

            def __eq__(self, other):
                raise AssertionError('No diagnostic dictionary equality callback')

        error.__dict__ = {HostileKey(): 'private'}
        self.assertEqual(UI.capture_failure_diagnostic(error), expected)

    def test_malformed_slots_never_invoke_equality_iteration_or_rendering(self):
        class Hostile:
            def __eq__(self, other):
                raise AssertionError('No arbitrary equality callback')

            def __str__(self):
                raise AssertionError('No arbitrary text conversion')

        class HostileTuple(tuple):
            def __iter__(self):
                raise AssertionError('No tuple subclass iteration')

        header = struct.unpack('!25I', image_bytes()[:100])
        invalid = [
            (Hostile(), header, 2, 1, 109, True),
            ('xwd-layout-unsupported', HostileTuple(header), 2, 1, 109, True),
            ('xwd-layout-unsupported', (Hostile(), *header[1:]), 2, 1, 109, True),
            ('xwd-layout-unsupported', header, True, 1, 109, True),
            ('xwd-layout-unsupported', header, 2, 1, 1 << 64, True),
            ('xwd-layout-unsupported', header, 2, 1, 109, Hostile()),
            ('xwd-layout-unsupported', (1 << 32, *header[1:]), 2, 1, 109, True),
            ('xwd-header-truncated', header, 2, 1, 99, None),
        ]
        for values in invalid:
            self.assertIsNone(UI.capture_failure_diagnostic(UI.XwdLayoutError(*values)))

    def test_supported_decode_has_identical_exact_pixels_and_no_diagnostic(self):
        for bits in (24, 32):
            for order in (0, 1):
                image = UI._capture_png(image_bytes(bits, order), 2, 1,
                                        lambda: False, time.monotonic() + 2)
                self.assertEqual(png_pixels(image), b'\0\xff\0\0\0\xff\0')
        header = struct.unpack('!25I', image_bytes()[:100])
        forged = UI.XwdLayoutError('xwd-layout-unsupported', header, 2, 1, len(image_bytes()), True)
        self.assertIsNone(UI.capture_failure_diagnostic(forged))

    def test_synthetic_multivisual_style_header_remains_rejected(self):
        raw = bytearray(image_bytes())
        struct.pack_into('!I', raw, 13 * 4, 5)
        struct.pack_into('!I', raw, 10 * 4, 8)
        _, diagnostic = self.rejected(raw)
        self.assertIn('visual-class', diagnostic['failed_checks'])
        self.assertIn('bitmap-pad', diagnostic['failed_checks'])
        self.assertEqual(diagnostic['header_fields']['visual_class'], 5)

    def test_full_capture_rejection_exposes_header_metadata_and_never_publishes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            output = root / 'window.png'
            authority = root / 'authority'
            authority.write_bytes(b'private generated cookie fixture')
            owner = {'pid': 1234, 'starttime': '100', 'exe_sha256': 'a' * 64}
            window = {'id': 6291519, 'x': 12, 'y': 20, 'width': 2, 'height': 1}
            environment = {'DISPLAY': ':123', 'XAUTHORITY': str(authority)}
            tools = {name: {'path': '/usr/bin/' + name, 'sha256': 'b' * 64}
                     for name in ('xwd', 'xprop', 'xwininfo')}
            raw = bytearray(image_bytes())
            struct.pack_into('!I', raw, 13 * 4, 5)
            with patch.object(UI, '_capture_path', side_effect=lambda value: Path(value)), patch.object(
                    UI, '_capture_digest', return_value='b' * 64), patch.object(
                    os, 'access', return_value=True), patch.object(
                    UI, '_capture_environment', return_value=environment), patch.object(
                    UI, '_capture_window', return_value={'x11_pid': 2}), patch.object(
                    UI, '_capture_command', return_value=raw), patch.object(UI, '_capture_publish') as publish:
                with self.assertRaises(UI.XwdLayoutError) as caught:
                    UI.capture_owned_window(owner, owner, window, environment, tools, output)
                self.assertIn('visual-class', UI.capture_failure_diagnostic(caught.exception)['failed_checks'])
                publish.assert_not_called()
                self.assertFalse(output.exists())
