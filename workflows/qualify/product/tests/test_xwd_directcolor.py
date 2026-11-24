"""Bounded identity DirectColor and unchanged TrueColor RGB/PNG regressions."""

import importlib.util
import json
import os
from pathlib import Path
import struct
import tempfile
import time
import unittest
from unittest.mock import patch
import zlib


SOURCE = Path(__file__).resolve().parents[1] / 'platforms/linux/ui.py'


def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


CANDIDATE = load('identity_directcolor_ui', SOURCE)


def generated_rgb(x, y):
    return ((x * 37 + y * 19) & 255, (x * 71 + y * 43 + 17) & 255,
            (x * 113 + y * 29 + 91) & 255)


def palette_bytes(*, flags=0, padding=73, reverse=False):
    indices = range(255, -1, -1) if reverse else range(256)
    return b''.join(struct.pack('!IHHHBB', index * 0x010101,
                                index * 257, index * 257, index * 257, flags, padding)
                    for index in indices)


def direct_xwd(width=5, height=3, *, order=0, stride=None, palette=None, pad_value=73):
    if stride is None:
        stride = width * 4
    if palette is None:
        palette = palette_bytes(padding=pad_value)
    header = (101, 7, 2, 24, width, height, 0, order, 32, order, 8, 24, stride,
              5, 0xff0000, 0xff00, 0xff, 8, 256, 256, width, height, 0, 0, 0)
    rows = []
    for y in range(height):
        encoded = bytearray()
        for x in range(width):
            red, green, blue = generated_rgb(x, y)
            pixel = (red << 16) | (green << 8) | blue
            encoded.extend(pixel.to_bytes(3, 'little' if order == 0 else 'big'))
        encoded.extend(b'\xa5' * (stride - len(encoded)))
        rows.append(encoded)
    return struct.pack('!25I', *header) + b'\0' + palette + b''.join(rows)


def png_scanlines(encoded, width, height):
    """Separate CRC/dimensions/filter oracle, not the candidate's pixel reader."""
    if encoded[:8] != b'\x89PNG\r\n\x1a\n':
        raise AssertionError('Invalid PNG signature')
    offset = 8
    compressed = bytearray()
    kinds = []
    while offset < len(encoded):
        length = struct.unpack('!I', encoded[offset:offset + 4])[0]
        kind = encoded[offset + 4:offset + 8]
        data = encoded[offset + 8:offset + 8 + length]
        checksum = struct.unpack('!I', encoded[offset + 8 + length:offset + 12 + length])[0]
        if zlib.crc32(kind + data) != checksum:
            raise AssertionError('PNG CRC differs')
        kinds.append(kind)
        if kind == b'IHDR':
            if struct.unpack('!2I5B', data) != (width, height, 8, 2, 0, 0, 0):
                raise AssertionError('PNG shape or channel encoding differs')
        elif kind == b'IDAT':
            compressed.extend(data)
        elif kind != b'IEND' or data:
            raise AssertionError('Unexpected PNG chunk')
        offset += length + 12
    if offset != len(encoded) or kinds != [b'IHDR', b'IDAT', b'IEND']:
        raise AssertionError('PNG trailing bytes or chunk order differs')
    rows = zlib.decompress(compressed)
    if len(rows) != height * (1 + width * 3):
        raise AssertionError('PNG decoded byte count differs')
    return rows


def expected_scanlines(width, height):
    return b''.join(b'\0' + b''.join(bytes(generated_rgb(x, y)) for x in range(width))
                    for y in range(height))


class DirectColorTests(unittest.TestCase):
    def decode(self, raw, width=5, height=3, *, cancelled=lambda: False, deadline=None):
        if deadline is None:
            deadline = time.monotonic() + 2
        return CANDIDATE._capture_png(raw, width, height, cancelled, deadline)

    def test_exact_varied_RGB_both_orders_and_every_bounded_nonminimal_stride(self):
        for order in (0, 1):
            for stride in range(15, 21):
                with self.subTest(order=order, stride=stride):
                    raw = direct_xwd(order=order, stride=stride)
                    image = self.decode(raw)
                    self.assertEqual(png_scanlines(image, 5, 3), expected_scanlines(5, 3))

    def test_actual_shape_stride1600_keeps_non_aligned_pixels_and_last_rows(self):
        raw = direct_xwd(400, 300, stride=1600)
        image = self.decode(raw, 400, 300, deadline=time.monotonic() + 10)
        self.assertEqual(png_scanlines(image, 400, 300), expected_scanlines(400, 300))

    def test_legacy_truecolor_palette_flags_and_varied_pixels_have_exact_scanlines(self):
        for order in (0, 1):
            width, height = 5, 3
            header = (101, 7, 2, 24, width, height, 0, order, 32, order, 32, 32, width * 4,
                      4, 0xff0000, 0xff00, 0xff, 8, 256, 256, width, height, 0, 0, 0)
            rows = bytearray()
            for y in range(height):
                for x in range(width):
                    red, green, blue = generated_rgb(x, y)
                    rows.extend(((red << 16) | (green << 8) | blue).to_bytes(4, 'little' if order == 0 else 'big'))
            raw = struct.pack('!25I', *header) + b'\0' + palette_bytes(flags=7, padding=29) + rows
            candidate = self.decode(raw, width, height)
            self.assertEqual(png_scanlines(candidate, width, height), expected_scanlines(width, height))

    def test_palette_order_is_irrelevant_only_when_unique_full_identity_is_proven(self):
        normal = self.decode(direct_xwd())
        reversed_records = self.decode(direct_xwd(palette=palette_bytes(reverse=True)))
        self.assertEqual(normal, reversed_records)

    def test_structural_palette_pad_is_ignored_but_never_channel_flags(self):
        reference = self.decode(direct_xwd(pad_value=0))
        for padding in (29, 73, 255):
            self.assertEqual(self.decode(direct_xwd(pad_value=padding)), reference)
        for flags in (1, 2, 3, 4, 5, 6, 7, 8, 255):
            with self.subTest(flags=flags), self.assertRaises(CANDIDATE.XwdLayoutError):
                self.decode(direct_xwd(palette=palette_bytes(flags=flags)))

    def test_nonidentity_ambiguous_duplicate_and_nondiagonal_tables_are_rejected(self):
        changes = ((0, '!I', 0x1000000), (0, '!I', 0x010100),
                   (12, '!I', 0), (4, '!H', 1), (6, '!H', 1), (8, '!H', 1))
        for offset, format_string, value in changes:
            table = bytearray(palette_bytes())
            struct.pack_into(format_string, table, offset, value)
            with self.subTest(offset=offset, format=format_string), self.assertRaises(CANDIDATE.XwdLayoutError) as caught:
                self.decode(direct_xwd(palette=table))
            diagnostic = CANDIDATE.capture_failure_diagnostic(caught.exception)
            self.assertEqual(diagnostic['reason'], 'xwd-identity-palette-rejected')
            self.assertEqual(diagnostic['failed_checks'], ['identity-directcolor-palette'])
            self.assertLess(len(json.dumps(diagnostic).encode()), 8192)

    def test_table_size_masks_and_new_format_tuple_must_be_exact(self):
        for field, value in ((0, 100), (1, 6), (2, 1), (3, 32), (4, 6), (5, 4),
                             (6, 1), (7, 2), (8, 16), (9, 2), (10, 32), (11, 32),
                             (13, 3), (14, 0xff), (15, 0xff), (16, 0xff00),
                             (17, 7), (18, 255), (19, 0), (19, 255), (19, 257)):
            raw = bytearray(direct_xwd())
            struct.pack_into('!I', raw, field * 4, value)
            with self.subTest(field=field, value=value), self.assertRaises(ValueError):
                self.decode(raw)

    def test_declared_stride_bounds_exact_length_and_trailing_bytes_are_enforced(self):
        original = direct_xwd()
        prefix = original[:101 + 256 * 12]
        for stride in (0, 1, 14, 21, 0xffffffff):
            raw = bytearray(prefix + b'\0' * (min(stride, 21) * 3))
            struct.pack_into('!I', raw, 12 * 4, stride)
            with self.subTest(stride=stride), self.assertRaises(ValueError):
                self.decode(raw)
        for raw in (original[:-1], original + b'X', original[:101 + 256 * 12 - 1]):
            with self.assertRaises(ValueError):
                self.decode(raw)

    def test_cancellation_and_deadline_during_palette_or_rows_never_return_PNG(self):
        raw = direct_xwd()
        checks = 0
        def cancel_palette():
            nonlocal checks
            checks += 1
            return checks == 3
        with self.assertRaises(CANDIDATE.CaptureCancelled):
            self.decode(raw, cancelled=cancel_palette)
        self.assertEqual(checks, 3)
        checks = 0
        def cancel_row():
            nonlocal checks
            checks += 1
            return checks == 11
        with self.assertRaises(CANDIDATE.CaptureCancelled):
            self.decode(raw, cancelled=cancel_row)
        self.assertEqual(checks, 11)
        with self.assertRaises(TimeoutError):
            self.decode(raw, deadline=time.monotonic() - 1)

    def test_full_capture_palette_cancellation_never_reaches_publication(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            output = root / 'window.png'
            authority = root / 'authority'
            authority.write_bytes(b'generated cookie fixture')
            owner = {'pid': 1234, 'starttime': '100', 'exe_sha256': 'a' * 64}
            window = {'id': 123, 'x': 0, 'y': 0, 'width': 5, 'height': 3}
            environment = {'DISPLAY': ':999', 'XAUTHORITY': str(authority)}
            tools = {name: {'path': '/usr/bin/' + name, 'sha256': 'b' * 64}
                     for name in ('xwd', 'xprop', 'xwininfo')}
            checks = 0
            def cancelled():
                nonlocal checks
                checks += 1
                return checks == 3
            with patch.object(CANDIDATE, '_capture_path', side_effect=lambda value: Path(value)), patch.object(
                    CANDIDATE, '_capture_digest', return_value='b' * 64), patch.object(
                    os, 'access', return_value=True), patch.object(
                    CANDIDATE, '_capture_environment', return_value=environment), patch.object(
                    CANDIDATE, '_capture_window', return_value={'x11_pid': 2}), patch.object(
                    CANDIDATE, '_capture_command', return_value=direct_xwd()), patch.object(
                    CANDIDATE, '_capture_publish') as publication:
                with self.assertRaises(CANDIDATE.CaptureCancelled):
                    CANDIDATE.capture_owned_window(owner, owner, window, environment, tools, output, cancelled=cancelled)
                publication.assert_not_called()
            self.assertFalse(output.exists())

    def test_independent_png_oracle_detects_crc_scanline_and_shape_errors(self):
        png = self.decode(direct_xwd())
        corrupted = bytearray(png)
        corrupted[30] ^= 1
        with self.assertRaises(AssertionError):
            png_scanlines(corrupted, 5, 3)
        with self.assertRaises(AssertionError):
            png_scanlines(png, 6, 3)
        self.assertNotEqual(png_scanlines(png, 5, 3), expected_scanlines(5, 2))


if __name__ == '__main__':
    unittest.main()
