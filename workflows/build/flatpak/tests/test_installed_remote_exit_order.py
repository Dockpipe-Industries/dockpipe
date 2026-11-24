"""Post-proof exit ordering; generated processes only, no GUI or remote effects."""
from pathlib import Path
import importlib.util
import os
import subprocess
import sys
import tempfile
import threading
import time
import types
import unittest
from unittest.mock import Mock, patch

SCRIPTS = Path(__file__).resolve().parents[1] / 'assets/scripts'


def load(name, filename):
    specification = importlib.util.spec_from_file_location(name, SCRIPTS / filename)
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


RUNNER = load('remote_exit_order_runner_tests', 'installed-remote-expiry-run.py')
WATCHDOG = load('remote_exit_order_watchdog_tests', 'installed-remote-watchdog.py')


class ExitOrderTests(unittest.TestCase):
    def setUp(self):
        self.pause = Mock(token={'pid': 1234, 'starttime': '5678'}, deadline=time.monotonic() + 2)
        self.proof = {'schema': 'dockpipe.flatpak.gui-lease-expiry-observation/v1',
                      'gui': self.pause.token, 'before_hard_cutoff': True,
                      'core_closed': True, 'native_idle': True,
                      'cleanup_assisted': False, 'input_permitted': False}

    def test_order_requires_proof_then_guard_resume_normal_exit_then_owner_close(self):
        calls = []
        watchdog = Mock()
        watchdog.unassisted.side_effect = lambda: calls.append('unassisted')
        self.pause.close.side_effect = lambda: calls.append('resume')
        quit_gui = Mock(side_effect=lambda: calls.append('exit0-and-empty-journal'))
        def close_owner():
            calls.append('blocking-owner-close')
            return {'exact_cleanup': True}
        result = RUNNER.finish_unassisted_gui(self.proof, watchdog, self.pause, quit_gui, close_owner)
        self.assertEqual(['unassisted', 'resume', 'exit0-and-empty-journal', 'blocking-owner-close'], calls)
        self.assertEqual({'exact_cleanup': True}, result)

    def test_missing_invalid_late_or_assisted_proof_has_no_normal_close_effect(self):
        invalid = [None, {}, dict(self.proof, core_closed=False), dict(self.proof, native_idle=False),
                   dict(self.proof, cleanup_assisted=True), dict(self.proof, gui={}),
                   dict(self.proof, before_hard_cutoff=False), dict(self.proof, input_permitted=True)]
        for proof in invalid:
            watchdog, quit_gui, close_owner = Mock(), Mock(), Mock()
            with self.subTest(proof=proof), self.assertRaises(ValueError):
                RUNNER.finish_unassisted_gui(proof, watchdog, self.pause, quit_gui, close_owner)
            watchdog.unassisted.assert_not_called()
            quit_gui.assert_not_called()
            close_owner.assert_not_called()
        self.pause.close.assert_not_called()
        self.pause.deadline = time.monotonic() - 1
        with self.assertRaisesRegex(ValueError, 'never extend'):
            RUNNER.finish_unassisted_gui(self.proof, Mock(), self.pause, Mock(), Mock())

    def test_watchdog_or_resume_failure_never_calls_normal_owner_close(self):
        for stage in ['watchdog', 'resume']:
            watchdog, quit_gui, close_owner = Mock(), Mock(), Mock()
            self.pause.close.side_effect = None
            if stage == 'watchdog':
                watchdog.unassisted.side_effect = ValueError('actual assistance')
            else:
                self.pause.close.side_effect = ValueError('resume unavailable')
            with self.subTest(stage=stage), self.assertRaises(ValueError):
                RUNNER.finish_unassisted_gui(self.proof, watchdog, self.pause, quit_gui, close_owner)
            quit_gui.assert_not_called()
            close_owner.assert_not_called()

    def test_failed_exit_or_retained_journal_is_not_cleared_or_normal_owner_close(self):
        with tempfile.TemporaryDirectory() as directory:
            journal = Path(directory) / 'original-owned-reference.json'
            original = b'{"original-reference":"preserved"}'
            journal.write_bytes(original)
            for error in [ValueError('GUI exit nonzero'), ValueError('recovery reference retained'),
                          KeyboardInterrupt('cancelled during normal exit')]:
                close_owner = Mock()
                with self.subTest(error=str(error)), self.assertRaises(type(error)):
                    RUNNER.finish_unassisted_gui(self.proof, Mock(), self.pause,
                                                  Mock(side_effect=error), close_owner)
                close_owner.assert_not_called()
                self.assertEqual(original, journal.read_bytes())

    def test_owner_cleanup_failure_remains_failure_after_normal_gui_exit(self):
        quit_gui = Mock()
        with self.assertRaisesRegex(ValueError, 'exact cleanup unknown'):
            RUNNER.finish_unassisted_gui(self.proof, Mock(), self.pause, quit_gui,
                                          Mock(side_effect=ValueError('exact cleanup unknown')))
        quit_gui.assert_called_once()


@unittest.skipUnless(sys.platform.startswith('linux'), 'Actual owned kernel process groups require Linux')
class ActualOwnedExitTests(unittest.TestCase):
    def exercise(self, legacy_order):
        child = subprocess.Popen([sys.executable, '-I', '-c', 'input()'],
                                 stdin=subprocess.PIPE, stdout=subprocess.DEVNULL,
                                 stderr=subprocess.DEVNULL, start_new_session=True)
        token = WATCHDOG.identity(child.pid)
        group = WATCHDOG.OwnedGroup({'pid': child.pid, 'starttime': token['starttime']}, os.getpid())
        cutoff = time.monotonic() + 1
        intervention = threading.Event()
        watcher_done = threading.Event()
        def hard_cutoff():
            try:
                while time.monotonic() < cutoff:
                    if not group.exists():
                        return
                    time.sleep(0.005)
                if group.exists():
                    intervention.set()
                    group.stop(term_grace=0.05, kill_grace=0.05)
            finally:
                watcher_done.set()
        watcher = threading.Thread(target=hard_cutoff)
        watcher.start()
        pause = types.SimpleNamespace(token={'pid': child.pid, 'starttime': token['starttime']},
                                      deadline=cutoff, close=lambda: None)
        proof = {'schema': 'dockpipe.flatpak.gui-lease-expiry-observation/v1', 'gui': pause.token,
                 'before_hard_cutoff': True, 'core_closed': True, 'native_idle': True,
                 'cleanup_assisted': False, 'input_permitted': False}
        def quit_gui():
            if child.poll() is None:
                child.stdin.write(b'quit\n')
                child.stdin.flush()
            child.wait(timeout=0.3)
            RUNNER.require(child.returncode == 0, 'Generated normal exit was interrupted')
        def owner_close():
            time.sleep(1.2)
            return {'exact_cleanup': True}
        watchdog = Mock()
        watchdog.unassisted.side_effect = lambda: RUNNER.require(not intervention.is_set(), 'assisted')
        try:
            if legacy_order:
                watchdog.unassisted()
                pause.close()
                owner_close()
                with self.assertRaisesRegex(ValueError, 'interrupted'):
                    quit_gui()
                self.assertTrue(intervention.is_set())
            else:
                result = RUNNER.finish_unassisted_gui(proof, watchdog, pause, quit_gui, owner_close)
                self.assertEqual({'exact_cleanup': True}, result)
                self.assertEqual(0, child.returncode)
                self.assertFalse(intervention.is_set())
            self.assertTrue(watcher_done.wait(timeout=1))
        finally:
            if child.poll() is None:
                child.kill()
            child.wait(timeout=1)
            child.stdin.close()
            watcher.join(timeout=1)
        self.assertFalse(watcher.is_alive())
        self.assertFalse(Path('/proc', str(child.pid)).exists())

    def test_actual_exit_before_blocking_cleanup_avoids_hard_intervention(self):
        self.exercise(legacy_order=False)

    def test_actual_legacy_blocking_cleanup_crosses_same_cutoff_before_quit(self):
        self.exercise(legacy_order=True)


if __name__ == '__main__':
    unittest.main()
