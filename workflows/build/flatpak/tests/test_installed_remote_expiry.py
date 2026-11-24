"""Generated local-process fault tests; these do not prove remote expiry."""
from pathlib import Path
import importlib.util
import os
import subprocess
import sys
import time
import unittest

SOURCE = Path(__file__).resolve().parents[1] / 'assets/scripts/installed-remote-expiry.py'
SPEC = importlib.util.spec_from_file_location('installed_remote_expiry_tests', SOURCE)
EXPIRY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(EXPIRY)


@unittest.skipUnless(sys.platform.startswith('linux') and hasattr(os, 'pidfd_open'),
                     'Linux pidfd process identity is required')
class GuiLeaseExpiryTests(unittest.TestCase):
    def setUp(self):
        self.child = subprocess.Popen([sys.executable, '-I', '-c', 'import time; time.sleep(30)'],
                                      stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL,
                                      stderr=subprocess.DEVNULL, start_new_session=True)
        current = EXPIRY.process_identity(self.child.pid)
        self.token = {name: current[name] for name in ['pid', 'starttime']}
        self.probe = None
        self.reference = {'session': 'generated-test-reference'}
        self.core = {'reference': self.reference, 'state': 'closed', 'ack': 1,
                     'lease_millis': 0, 'signals': []}
        self.native = {'native_idle_proven': True, 'active_channels': [],
                       'unclassified_worker_children': []}

    def tearDown(self):
        if self.probe is not None:
            self.probe.close()
        if self.child.poll() is None:
            self.child.terminate()
        self.child.wait(timeout=3)

    def pause(self):
        self.probe = EXPIRY.GuiLeaseExpiry(self.token, time.monotonic() + 3)
        result = self.probe.pause()
        self.assertFalse(result['input_permitted'])
        self.assertFalse(result['carrier_signalled'])

    def test_only_exact_stopped_gui_can_observe_unassisted_closure(self):
        self.pause()
        result = self.probe.observe(self.reference, self.core, self.native)
        self.assertTrue(result['before_hard_cutoff'])
        self.assertFalse(result['cleanup_assisted'])
        self.assertFalse(result['gui_viewer_acceptance_claimed'])
        with self.assertRaises(ValueError):
            self.probe.observe(self.reference, self.core, self.native)

    def test_cleanup_assistance_cannot_be_reported_as_expiry(self):
        for intervention in ['core-close', 'carrier-signal', 'watchdog-assisted', 'gui-resume']:
            with self.subTest(intervention=intervention):
                self.pause()
                self.probe.intervention(intervention)
                with self.assertRaises(ValueError):
                    self.probe.observe(self.reference, self.core, self.native)
                self.probe.close()
                self.probe = None

    def test_missing_or_wrong_reference_and_nonidle_native_are_rejected(self):
        self.pause()
        for core, native in [
                [dict(self.core, reference={'session': 'foreign'}), self.native],
                [dict(self.core, state='running'), self.native],
                [dict(self.core, ack=True), self.native],
                [self.core, dict(self.native, active_channels=[1])],
                [self.core, {}]]:
            with self.subTest(core=core, native=native), self.assertRaises(ValueError):
                self.probe.observe(self.reference, core, native)

    def test_deadline_and_stale_identity_cannot_admit_pause(self):
        with self.assertRaises(ValueError):
            EXPIRY.GuiLeaseExpiry(dict(self.token, starttime='0'), time.monotonic() + 3)
        with self.assertRaises(ValueError):
            EXPIRY.GuiLeaseExpiry(self.token, time.monotonic() - 1)
        self.assertIsNone(self.child.poll())

    def test_cutoff_cannot_be_extended_by_late_success_metadata(self):
        self.pause()
        self.probe.deadline = time.monotonic() - 1
        with self.assertRaises(ValueError):
            self.probe.observe(self.reference, self.core, self.native)


if __name__ == '__main__':
    unittest.main()
