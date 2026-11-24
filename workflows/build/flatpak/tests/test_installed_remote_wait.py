"""Actual owned-child reaping during a bounded provider-facade wait."""
from pathlib import Path
import importlib.util
import json
import os
import subprocess
import sys
import threading
import time
import types
import unittest
from unittest.mock import patch

SCRIPTS = Path(__file__).resolve().parents[1] / 'assets/scripts'


def load(name, filename):
    specification = importlib.util.spec_from_file_location(name, SCRIPTS / filename)
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


REMOTE = load('facade_owned_wait_tests', 'installed-remote.py')
WATCHDOG = load('facade_owned_wait_watchdog_tests', 'installed-remote-watchdog.py')


@unittest.skipUnless(sys.platform.startswith('linux'), 'Owned kernel process identity requires Linux')
class FacadeWaitTests(unittest.TestCase):
    def setUp(self):
        self.wrapper = subprocess.Popen(
            [sys.executable, '-I', '-c', 'import time; time.sleep(10)'],
            stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
            start_new_session=True)
        self.addCleanup(self.close_wrapper)
        read_fd, self.write_fd = os.pipe()
        self.reader = os.fdopen(read_fd, 'rb', buffering=0)
        self.addCleanup(self.close_pipe)
        self.client = REMOTE.FacadeClient.__new__(REMOTE.FacadeClient)
        self.client.buffer = b''
        self.client.wait_process = self.wrapper
        self.client.wait_process_pid = self.wrapper.pid
        self.client.process = types.SimpleNamespace(stdout=self.reader)

    def close_wrapper(self):
        if self.wrapper.poll() is None:
            self.wrapper.kill()
        self.wrapper.wait(timeout=1)

    def close_pipe(self):
        self.reader.close()
        if self.write_fd is not None:
            os.close(self.write_fd)

    def test_actual_watchdog_confirms_absence_while_controller_waits_for_facade(self):
        token = WATCHDOG.identity(self.wrapper.pid)
        group = WATCHDOG.OwnedGroup({'pid': self.wrapper.pid, 'starttime': token['starttime']}, os.getpid())
        outcome = {}
        def stop_owned_group():
            time.sleep(0.03)
            outcome['stopped'] = group.stop(term_grace=0.2, kill_grace=0.2)
        def reply():
            time.sleep(0.15)
            os.write(self.write_fd, b'{"generated_reply":true}\n')
        stop_thread = threading.Thread(target=stop_owned_group)
        reply_thread = threading.Thread(target=reply)
        stop_thread.start()
        reply_thread.start()
        try:
            result = self.client.event(1)
        finally:
            stop_thread.join(timeout=1)
            reply_thread.join(timeout=1)
        self.assertFalse(stop_thread.is_alive())
        self.assertTrue(outcome['stopped'])
        self.assertEqual([], group.remaining())
        self.assertFalse(group.exists())
        self.assertFalse(Path('/proc', str(self.wrapper.pid)).exists())
        self.assertEqual({'generated_reply': True}, result)

    def test_exact_child_poll_does_not_wait_for_or_reap_another_owned_child(self):
        other = subprocess.Popen([sys.executable, '-I', '-c', 'pass'],
                                 stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        try:
            time.sleep(0.05)
            os.write(self.write_fd, b'{"generated_reply":true}\n')
            self.client.event(1)
            self.assertIsNone(other.returncode)
            self.assertEqual(0, other.wait(timeout=1))
            self.assertIsNone(self.wrapper.poll())
        finally:
            if other.poll() is None:
                other.kill()
            other.wait(timeout=1)

    def test_cancellation_and_poll_failure_propagate_without_signals(self):
        for error in [KeyboardInterrupt('generated cancellation'), ValueError('generated poll failure')]:
            with self.subTest(error=type(error).__name__), \
                    patch.object(self.wrapper, 'poll', side_effect=error), \
                    patch.object(REMOTE.os, 'kill') as signal_pid, \
                    patch.object(REMOTE.os, 'killpg') as signal_group:
                with self.assertRaises(type(error)):
                    self.client.event(1)
                signal_pid.assert_not_called()
                signal_group.assert_not_called()

    def test_slow_callback_consumes_original_deadline_without_extension(self):
        now = [10.0]
        def slow_poll():
            now[0] = 12.0
        with patch.object(self.wrapper, 'poll', side_effect=slow_poll), \
                patch.object(REMOTE.time, 'monotonic', side_effect=lambda: now[0]), \
                patch.object(REMOTE.select, 'select') as select_ready:
            with self.assertRaisesRegex(ValueError, 'deadline elapsed'):
                self.client.event(1)
            select_ready.assert_not_called()

    def test_changed_child_identity_and_eof_are_not_success(self):
        self.client.wait_process_pid += 1
        with self.assertRaisesRegex(ValueError, 'identity changed'):
            self.client.event(1)
        self.client.wait_process_pid = self.wrapper.pid
        os.close(self.write_fd)
        self.write_fd = None
        with self.assertRaisesRegex(ValueError, 'facade ended'):
            self.client.event(1)

    def test_empty_pipe_retains_the_original_timeout(self):
        started = time.monotonic()
        with self.assertRaisesRegex(ValueError, 'deadline elapsed'):
            self.client.event(0.06)
        self.assertLess(time.monotonic() - started, 0.3)


if __name__ == '__main__':
    unittest.main()
