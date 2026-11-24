"""Actual generated-process ownership/failure tests; no GUI or remote effects."""
from pathlib import Path
import importlib.util
import os
import signal
import subprocess
import sys
import tempfile
import threading
import time
import unittest

SOURCE = Path(__file__).resolve().parents[1] / 'assets/scripts/installed-remote-watchdog.py'
SPEC = importlib.util.spec_from_file_location('installed_remote_watchdog_tests', SOURCE)
WATCHDOG = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(WATCHDOG)


@unittest.skipUnless(sys.platform.startswith('linux'), 'Linux kernel identity contract required')
class InstalledRemoteWatchdogTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.processes = []

    def tearDown(self):
        for process in self.processes:
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait(timeout=3)
        self.temporary.cleanup()

    def start(self, source):
        with (self.root / ('stdout-' + str(len(self.processes)))).open('xb') as stdout, \
                (self.root / ('stderr-' + str(len(self.processes)))).open('xb') as stderr:
            process = subprocess.Popen([sys.executable, '-I', '-c', source], stdout=stdout,
                                       stderr=stderr, start_new_session=True)
        self.processes.append(process)
        return process

    def token(self, process):
        value = WATCHDOG.identity(process.pid)
        return {'pid': process.pid, 'starttime': value['starttime']}

    def test_only_new_child_session_of_the_actual_controller_is_admitted(self):
        process = self.start('import time; time.sleep(30)')
        owned = WATCHDOG.OwnedGroup(self.token(process), os.getpid())
        self.assertEqual(process.pid, owned.group)
        for controller in [1, os.getpid() + 1]:
            with self.subTest(controller=controller), self.assertRaises(ValueError):
                WATCHDOG.OwnedGroup(self.token(process), controller)
        self.assertIsNone(process.poll())

    def test_stale_pid_token_does_not_signal_or_admit_live_process(self):
        process = self.start('import time; time.sleep(30)')
        token = self.token(process)
        token['starttime'] = str(int(token['starttime']) + 1)
        with self.assertRaises(ValueError):
            WATCHDOG.OwnedGroup(token, os.getpid())
        self.assertIsNone(process.poll())

    def test_controller_itself_cannot_be_reclassified_as_owned_gui(self):
        current = WATCHDOG.identity(os.getpid())
        with self.assertRaises(ValueError):
            WATCHDOG.OwnedGroup({'pid': os.getpid(), 'starttime': current['starttime']}, os.getpid())

    def test_stop_includes_provider_style_detached_carrier_group(self):
        child_record = self.root / 'detached-carrier.pid'
        source = ("import pathlib,signal,subprocess,sys,time\n"
                  "child=subprocess.Popen([sys.executable,'-I','-c','import time; time.sleep(30)'],start_new_session=True)\n"
                  "pathlib.Path(" + repr(str(child_record)) + ").write_text(str(child.pid))\n"
                  "def stop(_signal,_frame):\n"
                  "    child.wait(timeout=3)\n"
                  "    raise SystemExit(0)\n"
                  "signal.signal(signal.SIGTERM,stop)\n"
                  "time.sleep(30)\n")
        process = self.start(source)
        until = time.monotonic() + 3
        while not child_record.exists() and time.monotonic() < until:
            time.sleep(0.01)
        self.assertTrue(child_record.exists())
        child_pid = int(child_record.read_text())
        owned = WATCHDOG.OwnedGroup(self.token(process), os.getpid())
        self.assertIn(child_pid, owned.groups)
        waiter = threading.Thread(target=process.wait)
        waiter.start()
        try:
            self.assertTrue(owned.stop())
        finally:
            waiter.join(timeout=3)
            if WATCHDOG.same_process(owned.tokens[child_pid]):
                os.killpg(child_pid, signal.SIGKILL)
        self.assertFalse(owned.exists())
        self.assertFalse(owned.remaining())
        self.assertFalse(waiter.is_alive())
        with self.assertRaises(ProcessLookupError):
            os.kill(child_pid, 0)

    def test_gui_identity_cannot_point_outside_the_admitted_tree(self):
        process = self.start('import time; time.sleep(30)')
        owned = WATCHDOG.OwnedGroup(self.token(process), os.getpid())
        parent = WATCHDOG.identity(os.getpid())
        with self.assertRaises(ValueError):
            WATCHDOG.admit_gui({'pid': os.getpid(), 'starttime': parent['starttime'],
                                'commit': 'a' * 64, 'sha256': 'b' * 64}, owned)
        self.assertIsNone(process.poll())


if __name__ == '__main__':
    unittest.main()
