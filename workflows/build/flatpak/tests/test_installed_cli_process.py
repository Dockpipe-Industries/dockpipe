"""Contained CLI attribution cannot be inferred from a successful host process."""
import importlib.util
from pathlib import Path
import subprocess
import unittest


SPEC = importlib.util.spec_from_file_location('installed_cli_process',
    Path(__file__).resolve().parents[1] / 'assets/scripts/installed-cli-process.py')
PROCESS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PROCESS)


class InstalledCliProcessTests(unittest.TestCase):
    def test_unobserved_command_cannot_qualify_namespace_or_cleanup(self):
        observer = PROCESS.InstalledCliProcessObserver(['/not/a/generated/child'], '1' * 64, '2' * 64)
        observer.observe()
        result = observer.finish()
        self.assertFalse(result['command_child_observed'])
        self.assertFalse(result['contained_cli_observed'])
        self.assertFalse(result['owned_processes_closed'])

    def test_exact_owned_host_child_can_close_without_qualifying_contained_cli(self):
        command = ['/usr/bin/sleep', '0.2']
        child = subprocess.Popen(command, start_new_session=True)
        try:
            observer = PROCESS.InstalledCliProcessObserver(command, '1' * 64, '2' * 64)
            observer.observe()
            self.assertEqual(observer.root[0], child.pid)
        finally:
            child.wait(timeout=5)
        result = observer.finish()
        self.assertTrue(result['command_child_observed'])
        self.assertTrue(result['owned_processes_closed'])
        self.assertFalse(result['contained_cli_observed'])


if __name__ == '__main__':
    unittest.main()
