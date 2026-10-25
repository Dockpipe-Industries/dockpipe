"""Native qualification must retain host ownership and propagate failed gates."""
import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile
import time
import signal
import unittest
from unittest import mock

SCRIPT = Path(__file__).resolve().parents[1] / 'assets/scripts/run.py'
SPECIFICATION = importlib.util.spec_from_file_location('native_qualification', SCRIPT)
qualification = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(qualification)


class DispatchTests(unittest.TestCase):
    def test_mismatched_or_unsupported_host_never_starts_native_builds(self):
        for system, requested in [('Linux', 'macos'), ('Windows', 'macos'), ('Darwin', 'windows')]:
            with self.subTest(system=system, requested=requested):
                with mock.patch.object(qualification.platform, 'system', return_value=system):
                    with mock.patch.object(qualification.subprocess, 'run') as run:
                        with self.assertRaises(SystemExit):
                            qualification.main(['--platform', requested])
                        run.assert_not_called()

    def test_failed_native_gate_propagates_and_jobs_reach_owning_script(self):
        with tempfile.TemporaryDirectory() as directory:
            for system, selected, executable in [('Darwin', 'macos', 'bash'), ('Windows', 'windows', 'pwsh')]:
                with self.subTest(system=system):
                    with mock.patch.dict(os.environ, {'DOCKPIPE_SOURCE_ROOT': directory, 'RUNNER_TEMP': directory}):
                        with mock.patch.object(qualification.platform, 'system', return_value=system):
                            failure = subprocess.CalledProcessError(1, ['native-gate'])
                            with mock.patch.object(qualification.subprocess, 'run', side_effect=failure) as run:
                                with self.assertRaises(subprocess.CalledProcessError):
                                    qualification.main(['--platform', selected, '--jobs', '2'])
                                self.assertEqual(run.call_args.args[0][0], executable)
                                self.assertEqual(run.call_args.args[0][-1], '2')
                                self.assertTrue(run.call_args.kwargs['check'])
                                self.assertEqual(run.call_args.kwargs['cwd'], Path(directory).resolve())

    def test_invalid_jobs_fail_before_dispatch(self):
        with mock.patch.object(qualification.subprocess, 'run') as run:
            with self.assertRaises(SystemExit):
                qualification.main(['--jobs', '0'])
            run.assert_not_called()


DMG_SPECIFICATION = importlib.util.spec_from_file_location('native_dmg_proof', SCRIPT.with_name('dmg-proof.py'))
dmg = importlib.util.module_from_spec(DMG_SPECIFICATION)
DMG_SPECIFICATION.loader.exec_module(dmg)


class DmgDeadlineTests(unittest.TestCase):
    def test_expired_deadline_never_starts_another_fixture(self):
        with mock.patch.object(dmg.subprocess, 'Popen') as start:
            with self.assertRaises(TimeoutError):
                dmg.run(['fixture'], time.monotonic() - 1)
            start.assert_not_called()

    def test_timeout_terminates_the_complete_fixture_process_group(self):
        child = mock.Mock(pid=12345)
        child.wait.side_effect = [subprocess.TimeoutExpired(['fixture'], 1), 0]
        with mock.patch.object(dmg.subprocess, 'Popen', return_value=child) as start:
            with mock.patch.object(dmg.os, 'killpg') as terminate:
                with self.assertRaises(subprocess.TimeoutExpired):
                    dmg.run(['fixture'], time.monotonic() + 10)
                self.assertTrue(start.call_args.kwargs['start_new_session'])
                terminate.assert_called_once_with(12345, signal.SIGTERM)

    def test_failed_fixture_remains_a_required_gate(self):
        child = mock.Mock()
        child.wait.return_value = 1
        with mock.patch.object(dmg.subprocess, 'Popen', return_value=child):
            with self.assertRaises(subprocess.CalledProcessError):
                dmg.run(['fixture'], time.monotonic() + 10)


if __name__ == '__main__':
    unittest.main()
