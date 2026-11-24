"""Inert proof/admission faults; no GUI, native effects or remote sessions."""
from pathlib import Path
import importlib.util
import json
import os
import tempfile
import unittest
from unittest.mock import patch

from PIL import Image

SOURCE = Path(__file__).resolve().parents[1] / 'assets/scripts/installed-remote-expiry-run.py'
SPEC = importlib.util.spec_from_file_location('installed_expiry_run_tests', SOURCE)
RUN = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RUN)


class ExpiryControllerTests(unittest.TestCase):
    def template(self):
        image = Image.new('RGB', (1600, 1000))
        for left, top, right, bottom in RUN.UI_TEXT_REGIONS.values():
            for y in range(top, bottom):
                for x in range(left, right):
                    image.putpixel((x, y), ((x + y) % 256, x % 251, y % 241))
        return image

    def test_actual_selected_node_view_and_ready_text_must_match_all_pixels(self):
        template = self.template()
        result = RUN.compare_preconnection_ui(template.copy(), template)
        self.assertEqual(set(RUN.UI_TEXT_REGIONS), set(result))
        for name, rectangle in RUN.UI_TEXT_REGIONS.items():
            changed = template.copy()
            changed.putpixel(rectangle[:2], (255, 255, 255))
            with self.subTest(name=name), self.assertRaises(ValueError):
                RUN.compare_preconnection_ui(changed, template)
        with self.assertRaises(ValueError):
            RUN.compare_preconnection_ui(Image.new('RGB', (800, 500)), template)
        with self.assertRaises(ValueError):
            RUN.compare_preconnection_ui(Image.new('RGB', (1600, 1000)), Image.new('RGB', (1600, 1000)))

    def test_chooser_cannot_silently_pick_foreign_duplicate_or_replaced_identity(self):
        target = {'node': 'fixture-peer', 'fingerprint': 'a' * 64}
        expected = {'node': target['node'], 'fingerprint': target['fingerprint'], 'status': 'paired'}
        RUN.one_paired_identity([expected], target)
        for value in [None, {}, [], [expected, expected], [dict(expected, node='foreign')],
                      [dict(expected, fingerprint='b' * 64)], [dict(expected, status='revoked')]]:
            with self.subTest(value=value), self.assertRaises(ValueError):
                RUN.one_paired_identity(value, target)

    def test_fresh_marker_cannot_bind_another_window_or_local_chrome(self):
        window = {'id': 10, 'x': 0, 'y': 0, 'width': 1180, 'height': 1008}
        marker = {'rect': {'x': 500, 'y': 400, 'width': 300, 'height': 180}}
        RUN.marker_inside_owned_view(marker, window, [window])
        for current in [[dict(window, id=11)], [dict(window, width=1200)], []]:
            with self.subTest(current=current), self.assertRaises(ValueError):
                RUN.marker_inside_owned_view(marker, window, current)
        with self.assertRaises(ValueError):
            RUN.marker_inside_owned_view({'rect': {'x': 500, 'y': 110, 'width': 300, 'height': 120}}, window, [window])

    def test_keyboard_never_becomes_remote_input_even_before_connect(self):
        operation = object.__new__(RUN.ExpiryOperation)
        operation.current = lambda: [{'x': 0, 'y': 0, 'width': 1180}]
        with patch.object(RUN.subprocess, 'run') as process:
            for key in ['Return', 'Home', 'Super_L+2']:
                with self.subTest(key=key), self.assertRaises(ValueError):
                    operation.local('key', key=key)
            process.assert_not_called()

    def test_postconnect_full_screenshot_is_refused_before_any_capture(self):
        operation = object.__new__(RUN.ExpiryOperation)
        operation.connected = True
        with self.assertRaises(ValueError):
            operation.screenshot_before_capture('forbidden')

    def test_private_request_duplicate_nonfinite_symlink_and_fifo_are_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            path = root / 'plan.json'
            for data in [b'{"key":1,"key":2}', b'{"key":NaN}']:
                path.write_bytes(data)
                path.chmod(0o600)
                with self.subTest(data=data), self.assertRaises(ValueError):
                    RUN.captured_json(path)
            alias = root / 'alias.json'
            alias.symlink_to(path)
            with self.assertRaises(OSError):
                RUN.captured_json(alias)
            fifo = root / 'fifo'
            os.mkfifo(fifo, 0o600)
            with self.assertRaises(ValueError):
                RUN.captured_json(fifo)


if __name__ == '__main__':
    unittest.main()
