"""Process admission and case attribution without starting a GUI or VM."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import Mock, patch

OWNER = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(OWNER))
import driver
import form
import lifecycle


class LinuxLifecycleTests(unittest.TestCase):
    def test_driver_exit_distinguishes_pending_coverage_from_failed_lifecycle(self):
        for facts, expected_exit in [({}, 0), ({'lifecycle_completed': True}, 0),
                                     ({'lifecycle_completed': False}, 2)]:
            with self.subTest(facts=facts), tempfile.TemporaryDirectory() as temporary:
                output = Path(temporary) / 'output'
                argv = ['driver.py', '--request', '/selected-request.json', '--output', str(output)]
                result = {'observed_target': facts, 'status': 'blocked',
                          'cases': [{'status': 'passed'}, {'status': 'manual_pending'}]}

                with patch.object(sys, 'argv', argv), patch.object(driver, 'load_request'), \
                     patch.object(driver, 'execute', return_value=result):
                    self.assertEqual(driver.main(), expected_exit)

                self.assertEqual(json.loads((output / 'result.json').read_text()), result)

    def test_disposed_installer_accessibility_node_defers_launch(self):
        from gi.repository import GLib

        stale_node = GLib.Error.new_literal(GLib.quark_from_string('atspi_error'),
                                            'No such object path for disposed control', 1)
        desktop = Mock(output=Path('/private/display'))
        with patch.object(form.ui, 'connect', return_value=(Mock(), {})), \
             patch.object(form, 'installer_button', side_effect=stale_node), \
             patch.object(form.subprocess, 'run') as run:
            self.assertIsNone(form.launch_filled_installer(desktop, Path('/installer.iso'), Path('/guest.qcow2')))

        run.assert_not_called()

    def test_only_selected_new_disk_process_is_admitted(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            disk = directory / 'new-disk.qcow2'
            process = subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(30)',
                '-uuid', 'new-fixture', '-drive', f'if=none,file={disk},format=qcow2',
                '-netdev', 'user,id=net0,hostfwd=tcp::23001-:22'])
            try:
                pidfile = directory / 'qemu-fixture.pid'
                pidfile.write_text(str(process.pid))
                selected = lifecycle.read_process(pidfile, disk, Path(sys.executable))
                self.assertEqual(selected['pid'], process.pid)
                self.assertTrue(selected['no_installer_media'])
                self.assertTrue(lifecycle.process_alive(selected))
                self.assertIsNone(lifecycle.read_process(pidfile, directory / 'historical.qcow2', Path(sys.executable)))
                selected['starttime'] = 'not-this-process'
                self.assertFalse(lifecycle.process_alive(selected))
            finally:
                process.terminate()
                process.wait(timeout=5)

    def test_pid_reference_symlink_is_never_admitted(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            target = directory / 'target'
            target.write_text(str(12345))
            link = directory / 'qemu-fixture.pid'
            link.symlink_to(target)
            self.assertIsNone(lifecycle.read_process(link, directory / 'disk', Path(sys.executable)))

    def test_observation_cannot_pass_other_scenario_or_family(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            baseline = output / 'baseline.json'
            baseline.write_text(json.dumps({'artifacts': [
                {'path': '/selected.deb', 'sha256': 'c' * 64}]}))
            pins = output / 'pins.json'
            pins.write_text('{}')
            configuration = {'operation': 'guided-install', 'baseline_receipt': str(baseline),
                             'pins_file': str(pins), 'media_file': '/selected.iso'}
            request = {'run_id': 'fixture', 'plan_sha256': 'a' * 64, 'registry_sha256': 'b' * 64,
                       'target': {'artifact_sha256': 'c' * 64}, 'configuration': configuration,
                       'cases': [{'id': identity, 'scenario': scenario, 'layer': 'installed_e2e',
                                  'required_assertions': ['installed-cli-version']}
                                 for identity, scenario in [('install.cli', 'happy'),
                                                            ('install.cli', 'failure'),
                                                            ('remote.security', 'happy')]]}
            with patch.object(driver, 'admit_configuration', return_value={'guest_started': False}):
                with patch.object(lifecycle, 'run', return_value=({'installed-cli-version': True}, [], None)):
                    result = driver.execute(request, output)
            self.assertEqual([case['status'] for case in result['cases']],
                             ['passed', 'manual_pending', 'manual_pending'])
            self.assertEqual(result['status'], 'blocked')
            request['cases'] = request['cases'][:1]
            with patch.object(driver, 'admit_configuration', return_value={'guest_started': False}):
                with patch.object(lifecycle, 'run', return_value=(
                        {'installed-cli-version': True}, [], 'Owned guest remains active.')):
                    incomplete = driver.execute(request, output)
            self.assertEqual(incomplete['cases'][0]['status'], 'passed')
            self.assertEqual(incomplete['status'], 'blocked')
            request['cases'][0]['id'] = 'remote.security'
            with patch.object(driver, 'admit_configuration', return_value={'guest_started': False}):
                with patch.object(lifecycle, 'run') as execution:
                    driver.execute(request, output)
            execution.assert_not_called()


if __name__ == '__main__':
    unittest.main()
