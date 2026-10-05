"""Small live probes for aggregate containment, before compiler verification."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

JOB = Path(__file__).with_name('job.py').resolve()
RUN = JOB.with_name('run.py')


class JobTests(unittest.TestCase):
    def test_nested_unit_and_escaped_process_are_in_shared_slice(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            command = [sys.executable, '-B', str(RUN), '--output', str(root/'child'),
                       '--cache', str(root/'cache'), '--timeout', '3', '--', sys.executable,
                       '-c', 'import subprocess; subprocess.Popen(["sleep","60"],start_new_session=True)']
            result = subprocess.run([sys.executable, '-B', str(JOB), '--output', str(root/'job'),
                                     '--timeout', '10', '--', *command], capture_output=True, text=True, timeout=30)
            report = json.loads((root/'job.json').read_text())
            child = json.loads((root/'child.json').read_text())
            self.assertEqual(result.returncode, 0, (result.stderr, report))
            self.assertIn(Path(report['cgroup']), Path(child['cgroup']).parents)
            self.assertEqual(report['limits']['memory.max'], 2 << 30)
            self.assertEqual(report['swap_current'], 0)
            self.assertTrue(report['tree_removed'])
            self.assertTrue(child['tree_removed'])

    def test_job_deadline_cancels_sibling_unit(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            command = [sys.executable, '-B', str(RUN), '--output', str(root/'child'),
                       '--cache', str(root/'cache'), '--timeout', '30', '--', 'sleep', '60']
            result = subprocess.run([sys.executable, '-B', str(JOB), '--output', str(root/'job'),
                                     '--timeout', '2', '--', *command], capture_output=True, text=True, timeout=20)
            report = json.loads((root/'job.json').read_text())
            child = json.loads((root/'child.json').read_text())
            self.assertNotEqual(result.returncode, 0)
            self.assertTrue(report['tree_removed'], report)
            self.assertFalse(Path(child['cgroup']).exists())

    def test_uncontained_coordinator_refused(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            result = subprocess.run([sys.executable, '-B', str(JOB), '--inside', '--output', str(root/'job'),
                                     '--', 'touch', str(root/'marker')], capture_output=True,
                                    env={k:v for k,v in os.environ.items() if not k.startswith('CONTAINED_JOB_')})
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse((root/'marker').exists())


if __name__ == '__main__':
    unittest.main()
