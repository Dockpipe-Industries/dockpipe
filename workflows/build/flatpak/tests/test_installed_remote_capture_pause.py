"""Pause ordering and refusal tests; no actual GUI or remote operation."""
from contextlib import contextmanager
from pathlib import Path
import importlib.util
import unittest
from unittest.mock import Mock, patch

SOURCE = Path(__file__).resolve().parents[1] / 'assets/scripts/installed-remote-expiry-run.py'
SPEC = importlib.util.spec_from_file_location('remote_capture_pause_tests', SOURCE)
RUNNER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RUNNER)


class CapturePauseTests(unittest.TestCase):
    def setUp(self):
        self.now = 100.0
        self.events = []
        self.window = {'id': 42, 'x': 0, 'y': 0, 'width': 1180, 'height': 1000}
        self.frame = {'before': self.window, 'after': self.window,
                      'visible_owned_rect': {'x': 0, 'y': 0, 'width': 1180, 'height': 1000}}
        self.token = {'pid': 1234, 'starttime': '5678'}
        self.pause = Mock(token=self.token, deadline=112.06)
        self.pause.current.return_value = {'state': 'T'}
        def stop_gui():
            self.events.append('pause')
            self.now += 0.01
            return {'gui': self.token, 'pause_observed_monotonic': self.now}
        self.pause.pause.side_effect = stop_gui
        self.candidate = {'capture_before_monotonic': 100.1}
        self.facade = Mock()
        self.facade.command.return_value = {'phase': 0, 'local_capture_budget_milliseconds': 100,
                                           'capture_before_monotonic': 100.1}
        self.verify = Mock(side_effect=lambda: self.events.append('CLI-and-module') or {'verified': True})
        self.reader = Mock()
        self.reader.capture_bracketed = self.bracketed
        self.reader.verify_paused_window.side_effect = lambda *_args: self.events.append('paused-window') or self.window
        self.reader.decode_frame.side_effect = lambda *_args: self.events.append('decode') or 'generated-RAM-sentinel'
        self.marker = Mock()
        self.marker.locate.side_effect = lambda *_args: self.events.append('whole-nonce') or {
            'rect': {'x': 600, 'y': 500, 'width': 200, 'height': 120}}
        self.watchdog = Mock()
        self.facts = {}
        self.accept_expiry = Mock(side_effect=lambda: self.events.append('expiry-may-now-be-observed'))

    @contextmanager
    def bracketed(self, rectangle, admitted, capture_before, pause_before, facts):
        self.assertEqual(100.1, capture_before)
        self.assertAlmostEqual(100.06, pause_before)
        self.events.extend(['window-before', 'capture-RAM', 'window-after'])
        facts['image_received_monotonic'] = self.now
        try:
            yield self.frame
        finally:
            self.events.append('free-image')

    def current_windows(self):
        self.events.append('full-GUI-and-geometry')
        return [self.window]

    def execute(self):
        with patch.object(RUNNER.time, 'monotonic', side_effect=lambda: self.now), \
                patch.object(RUNNER, 'write') as persist:
            with RUNNER.capture_and_pause(self.facade, self.candidate, self.verify, self.reader,
                                          self.window, self.pause, self.watchdog, self.facts) as values:
                frame, identities, paused = values
                self.assertTrue(identities['verified'])
                self.assertEqual(self.token, paused['gui'])
                RUNNER.validate_paused_frame(self.reader, frame, self.pause, self.marker,
                                             'a' * 32, self.current_windows, self.facts)
            self.accept_expiry()
            persist.assert_not_called()

    def test_capture_then_observed_pause_then_full_validation_precedes_expiry(self):
        self.execute()
        self.assertEqual(['CLI-and-module', 'window-before', 'capture-RAM', 'window-after',
                          'pause', 'paused-window', 'decode', 'whole-nonce', 'full-GUI-and-geometry',
                          'free-image', 'expiry-may-now-be-observed'], self.events)
        self.assertTrue(self.facts['validation_complete'])

    def test_observed_pause_after_six_or_changed_original_token_never_decodes(self):
        for fault in ['late', 'changed-token']:
            self.setUp()
            def bad_pause():
                return {'gui': {} if fault == 'changed-token' else self.token,
                        'pause_observed_monotonic': 100.07 if fault == 'late' else 100.01}
            self.pause.pause.side_effect = bad_pause
            with self.subTest(fault=fault), self.assertRaisesRegex(ValueError, 'within six seconds'):
                self.execute()
            self.reader.decode_frame.assert_not_called()
            self.accept_expiry.assert_not_called()
            self.assertIn('free-image', self.events)

    def test_paused_window_drift_or_lost_stopped_state_cannot_accept_expiry(self):
        for fault in ['window', 'resumed']:
            self.setUp()
            if fault == 'window':
                self.reader.verify_paused_window.side_effect = ValueError('capture bracket changed')
            else:
                self.pause.current.return_value = {'state': 'S'}
            with self.subTest(fault=fault), self.assertRaises(ValueError):
                self.execute()
            self.accept_expiry.assert_not_called()
            self.assertIn('free-image', self.events)

    def test_nonce_phase_ambiguity_decode_and_geometry_failures_never_accept_expiry(self):
        for fault in ['nonce', 'phase', 'ambiguity', 'decode', 'geometry']:
            self.setUp()
            if fault == 'decode':
                self.reader.decode_frame.side_effect = ValueError('generated storage fault')
            elif fault == 'geometry':
                self.marker.locate.return_value = {'rect': {'x': 0, 'y': 0, 'width': 200, 'height': 120}}
                self.marker.locate.side_effect = None
            else:
                self.marker.locate.side_effect = ValueError('generated ' + fault + ' refusal')
            with self.subTest(fault=fault), self.assertRaises(ValueError):
                self.execute()
            self.accept_expiry.assert_not_called()
            self.assertNotIn('validation_complete', self.facts)
            self.assertIn('free-image', self.events)

    def test_slow_post_pause_decode_keeps_original_eighteen_second_cutoff(self):
        def slow_decode(_frame):
            self.now = 112.07
            return 'generated-RAM-sentinel'
        self.reader.decode_frame.side_effect = slow_decode
        with self.assertRaisesRegex(ValueError, 'original cutoff'):
            self.execute()
        self.accept_expiry.assert_not_called()
        self.assertIn('free-image', self.events)

    def test_clipped_bottom_keeps_full_geometry_and_requires_nonce_inside_visible_intersection(self):
        self.window['height'] = 1008
        self.execute()
        self.assertTrue(self.facts['validation_complete'])
        self.setUp()
        self.window['height'] = 1008
        self.frame['visible_owned_rect']['height'] = 600
        with self.assertRaisesRegex(ValueError, 'visible owned intersection'):
            self.execute()
        self.accept_expiry.assert_not_called()


if __name__ == '__main__':
    unittest.main()
