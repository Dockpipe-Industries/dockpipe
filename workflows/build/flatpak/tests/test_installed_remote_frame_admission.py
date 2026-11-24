"""No GUI or pixels: verify ordering and the original capture deadline."""
from pathlib import Path
import importlib.util
import unittest
from unittest.mock import Mock, patch

SOURCE = Path(__file__).resolve().parents[1] / 'assets/scripts/installed-remote-expiry-run.py'
SPEC = importlib.util.spec_from_file_location('installed_remote_frame_admission_tests', SOURCE)
RUNNER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RUNNER)


class InstalledFrameAdmissionTests(unittest.TestCase):
    def setUp(self):
        self.now = 100.0
        self.calls = []
        self.candidate = {'capture_before_monotonic': 100.1}
        self.gate = {'phase': 0, 'local_capture_budget_milliseconds': 100,
                     'capture_before_monotonic': 100.1}
        self.facade = Mock()
        self.facade.command.side_effect = self.consume
        self.capture = Mock(side_effect=self.capture_image)
        self.verify = Mock(side_effect=self.verify_identity)

    def verify_identity(self):
        self.calls.append('verify exact installed CLI and module')
        return {'verified': True}

    def consume(self, action, **arguments):
        self.calls.append(action)
        self.assertEqual('consume-frame-admission', action)
        self.assertEqual(self.candidate, arguments['candidate'])
        return self.gate

    def capture_image(self):
        self.calls.append('capture')
        return 'generated in-memory image sentinel'

    def invoke(self):
        with patch.object(RUNNER.time, 'monotonic', side_effect=lambda: self.now):
            return RUNNER.consume_startup_frame(
                self.facade, self.candidate, self.verify, self.capture)

    def test_identity_admission_precedes_single_consume_and_capture(self):
        image, identities = self.invoke()
        self.assertEqual('generated in-memory image sentinel', image)
        self.assertEqual({'verified': True}, identities)
        self.assertEqual(['verify exact installed CLI and module',
                          'consume-frame-admission', 'capture'], self.calls)
        self.assertEqual(1, self.facade.command.call_count)

    def test_failed_identity_never_consumes_or_reads_pixels(self):
        self.verify.side_effect = ValueError('Exact installed module changed')
        with self.assertRaisesRegex(ValueError, 'module changed'):
            self.invoke()
        self.facade.command.assert_not_called()
        self.capture.assert_not_called()

    def test_slow_identity_cannot_manufacture_new_capture_budget(self):
        def slow_verification():
            self.now = 100.2
            return {'verified': True}
        self.verify.side_effect = slow_verification
        with self.assertRaisesRegex(ValueError, 'interval elapsed'):
            self.invoke()
        self.capture.assert_not_called()

    def test_slow_consume_ipc_fails_before_capture(self):
        def slow_consume(*_arguments, **_keywords):
            self.now = 100.2
            return self.gate
        self.facade.command.side_effect = slow_consume
        with self.assertRaisesRegex(ValueError, 'interval elapsed'):
            self.invoke()
        self.capture.assert_not_called()

    def test_slow_capture_is_failed_without_returning_image(self):
        def slow_capture():
            self.now = 100.2
            return 'generated sentinel'
        self.capture.side_effect = slow_capture
        with self.assertRaisesRegex(ValueError, 'receipt deadline'):
            self.invoke()

    def test_reply_cannot_change_phase_budget_or_deadline(self):
        for field, value in [('phase', 1), ('local_capture_budget_milliseconds', 101),
                             ('capture_before_monotonic', 100.2)]:
            self.gate[field] = value
            with self.subTest(field=field), self.assertRaisesRegex(ValueError, 'changed'):
                self.invoke()
            self.capture.assert_not_called()
            self.gate = {'phase': 0, 'local_capture_budget_milliseconds': 100,
                         'capture_before_monotonic': 100.1}


if __name__ == '__main__':
    unittest.main()
