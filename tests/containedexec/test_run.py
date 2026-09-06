"""Integration probes; run with a user systemd manager, before compiler work."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
import uuid

RUNNER = Path(__file__).with_name('run.py').resolve()


class ContainmentTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix='contained-exec-proof-')
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)

    def run_case(self, source, timeout=3):
        result = subprocess.run([sys.executable, str(RUNNER), '--output', str(self.root/'case'),
                                 '--cache', str(self.root/'cache'), '--timeout', str(timeout),
                                 '--', sys.executable, '-c', source], capture_output=True, text=True, timeout=timeout+30)
        report = json.loads((self.root/'case.json').read_text())
        self.assertTrue(report['tree_removed'], report)
        self.assertEqual(report['limits'], {'memory.max':1<<30, 'memory.swap.max':0, 'pids.max':128})
        return result, report

    def test_success_and_descendant_cleanup(self):
        result, report = self.run_case('import subprocess; subprocess.Popen(["sleep","60"],start_new_session=True)')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(report['outcome'], 'completed')
        self.assertEqual(report['swap_current'], 0)

    def test_cancellation_includes_escaped_session(self):
        result, report = self.run_case('import subprocess,time; subprocess.Popen(["sleep","60"],start_new_session=True); time.sleep(60)', .2)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(report['outcome'], 'timeout')

    def test_proactive_memory_stop(self):
        result, report = self.run_case('import time; chunks=[]\nfor i in range(900):\n chunks.append(bytearray(1024*1024)); time.sleep(.002)\ntime.sleep(5)', 10)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(report['outcome'], 'proactive_memory_stop')
        self.assertEqual(report['swap_current'], 0)
        self.assertEqual(report['memory_events_before'], report['memory_events_after'])

    def test_refuses_uncontained_child(self):
        marker = self.root/'must-not-exist'
        result = subprocess.run([sys.executable, str(RUNNER), '--inside', '--output', str(self.root/'refused'),
                                 '--cache', str(self.root/'cache'), '--', sys.executable, '-c',
                                 'from pathlib import Path; Path('+repr(str(marker))+').touch()'], capture_output=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(marker.exists())

    def test_independent_deadline_without_monitor(self):
        unit = 'contained-deadline-' + uuid.uuid4().hex
        receipt = self.root/'group'
        source = 'from pathlib import Path; import subprocess,time; subprocess.Popen(["sleep","60"],start_new_session=True); Path('+repr(str(receipt))+').write_text(Path("/proc/self/cgroup").read_text()); time.sleep(60)'
        try:
            result = subprocess.run(['systemd-run','--user','--wait','--pipe','--collect','--unit='+unit,
                '-p','MemoryMax=1G','-p','MemorySwapMax=0','-p','TasksMax=128','-p','RuntimeMaxSec=1',
                '-p','KillMode=control-group','-p','TimeoutStopSec=1',sys.executable,'-c',source], capture_output=True, timeout=10)
        finally:
            subprocess.run(['systemctl','--user','stop',unit], capture_output=True, timeout=5)
        self.assertNotEqual(result.returncode, 0)
        group = Path('/sys/fs/cgroup')/receipt.read_text().strip().split('::')[1].lstrip('/')
        self.assertFalse(group.exists())


if __name__ == '__main__':
    unittest.main()
