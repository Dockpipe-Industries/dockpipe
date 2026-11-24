"""Exercise Linux lane admission without starting a GUI, guest or network service."""
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

OWNER = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(OWNER))
import desktop
import driver
import guest_environment


class LinuxAdmissionTests(unittest.TestCase):
    def test_guest_vm_only_request_is_blocked_before_prerequisites_or_launch(self):
        with tempfile.TemporaryDirectory() as temporary:
            request = {'run_id': 'fixture', 'plan_sha256': 'a' * 64, 'registry_sha256': 'b' * 64,
                       'target': {'id': 'ubuntu-guest', 'platform': 'linux-amd64-deb-ubuntu24.04',
                                  'kind': 'vm', 'artifact_sha256': 'c' * 64},
                       'configuration': {'operation': 'guided-install'},
                       'cases': [{'id': 'vm.install-linux', 'scenario': 'happy', 'layer': 'installed_e2e',
                                  'required_assertions': ['installer-started-from-gui', 'disk-only-restart']}]}
            import lifecycle
            with patch.object(driver, 'admit_configuration') as admission:
                with patch.object(lifecycle, 'run') as execution:
                    result = driver.execute(request, Path(temporary))
            admission.assert_not_called()
            execution.assert_not_called()
            self.assertEqual(result['cases'][0]['status'], 'blocked')
            self.assertIn('host GUI', result['cases'][0]['reason'])
            self.assertEqual(result['observed_target']['target_role'], 'guest-product')
            self.assertFalse(result['observed_target']['target_observed'])

    def test_host_provisioning_observation_cannot_credit_guest_vm_feature(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            baseline = root / 'baseline.json'
            baseline.write_text(json.dumps({'artifacts': [{'path': '/selected.deb', 'sha256': 'c' * 64}]}))
            pins = root / 'pins.json'
            pins.write_text('{}')
            request = {'run_id': 'fixture', 'plan_sha256': 'a' * 64, 'registry_sha256': 'b' * 64,
                       'target': {'id': 'ubuntu-guest', 'platform': 'linux-amd64-deb-ubuntu24.04',
                                  'kind': 'vm', 'artifact_sha256': 'c' * 64},
                       'configuration': {'operation': 'guided-install', 'baseline_receipt': str(baseline),
                                         'pins_file': str(pins), 'media_file': '/selected.iso'},
                       'cases': [{'id': 'install.cli', 'scenario': 'happy', 'layer': 'installed_e2e',
                                  'required_assertions': ['installed-cli-version']},
                                 {'id': 'vm.install-linux', 'scenario': 'happy', 'layer': 'installed_e2e',
                                  'required_assertions': ['installer-started-from-gui']}]}
            import lifecycle
            with patch.object(driver, 'admit_configuration', return_value={}):
                with patch.object(lifecycle, 'run', return_value=({
                        'installed-cli-version': True, 'installer-started-from-gui': True}, [], None)):
                    result = driver.execute(request, root)
            self.assertEqual(result['cases'][0]['status'], 'passed')
            self.assertEqual(result['cases'][1]['status'], 'blocked')
            self.assertEqual(result['cases'][1]['assertions'][0]['status'], 'failed')
            self.assertEqual(result['status'], 'blocked')

    def test_guest_product_target_cannot_substitute_host_gui_artifact(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            baseline = root / 'baseline.json'
            baseline.write_text(json.dumps({'artifacts': [{'path': '/selected.deb', 'sha256': 'c' * 64}]}))
            request = {'target': {'artifact_sha256': 'd' * 64},
                       'configuration': {'baseline_receipt': str(baseline)},
                       'cases': [{'id': 'install.cli', 'scenario': 'happy', 'layer': 'installed_e2e'}]}
            with patch.object(driver, 'admit_configuration', return_value={}):
                with self.assertRaisesRegex(ValueError, 'published baseline DEB'):
                    driver.execute(request, root)

    def test_socket_cleanup_never_recurses_into_unknown_runtime_entries(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / 'owned-runtime'
            root.mkdir(mode=0o700)
            nested = root / 'doc'
            nested.mkdir()
            canary = nested / 'preserved'
            canary.write_bytes(b'preserve')
            with patch.object(desktop.shutil, 'rmtree') as recursive_removal:
                with self.assertRaisesRegex(ValueError, 'unknown entries'):
                    desktop.remove_owned_socket_directory(root)
                recursive_removal.assert_not_called()
            self.assertEqual(b'preserve', canary.read_bytes())

    def test_deep_output_uses_short_private_socket_directory_and_owned_cleanup(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / ('deep-output-' * 12) / 'desktop'
            instance = desktop.OwnedDesktop(output, {'prepare_environment': lambda environment: None})
            with patch.object(desktop.subprocess, 'run', side_effect=RuntimeError('Stop before starting services')):
                with self.assertRaisesRegex(RuntimeError, 'Stop before starting services'):
                    instance.start()
            sockets = instance.socket_directory
            self.assertNotIn(output, sockets.parents)
            self.assertLess(len(str(sockets / 'accessibility-bus').encode()), 108)
            self.assertEqual(sockets.stat().st_mode & 0o777, 0o700)
            instance.stop()
            self.assertFalse(sockets.exists())
            self.assertTrue(output.is_dir())

    def test_missing_application_profile_refuses_before_desktop_effects(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / 'desktop'
            instance = desktop.OwnedDesktop(output, {})
            with patch.object(desktop.subprocess, 'run') as command:
                with self.assertRaisesRegex(ValueError, 'explicit application environment'):
                    instance.start()
            command.assert_not_called()
            self.assertFalse(output.exists())
            self.assertIsNone(instance.socket_directory)

    def test_generic_desktop_requires_no_native_vm_or_qt_selections(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / 'desktop'
            selections = {'xvfb': '/selected/Xvfb', 'prepare_environment': lambda environment: None}
            instance = desktop.OwnedDesktop(output, selections)
            observed = {}

            def stop_before_display(_command, **arguments):
                observed.update(arguments['env'])
                raise RuntimeError('Observed prepared environment before display launch')

            with patch.object(desktop.subprocess, 'run'):
                with patch.object(desktop.subprocess, 'Popen', side_effect=stop_before_display):
                    with self.assertRaisesRegex(RuntimeError, 'before display launch'):
                        instance.start()
            instance.stop()
            self.assertTrue(observed['DISPLAY'].startswith(':'))
            self.assertEqual(observed['XAUTHORITY'], str(output / 'Xauthority'))
            self.assertEqual(observed['TMPDIR'], str(output / 'temp'))
            self.assertEqual((output / 'temp').stat().st_mode & 0o777, 0o700)
            self.assertFalse(any(name.startswith('DOCKPIPE_VM_') for name in observed))
            self.assertNotIn('LD_LIBRARY_PATH', observed)
            self.assertNotIn('DOCKPIPE_PACKAGES_ROOT', observed)

    def test_native_guest_profile_retains_existing_provider_defaults(self):
        environment = {'DISPLAY': ':fixture', 'XAUTHORITY': '/fixture/authority'}
        selections = {'qt_prefix': '/fixture/qt', 'browser_sdk_prefix': '/fixture/browser',
                      'qemu_prefix': '/fixture/qemu', 'cli': '/fixture/dockpipe',
                      'store': '/fixture/packages', 'gui': '/fixture/dockpipe-launcher',
                      'guest_environment': {'DOCKPIPE_VM_BOOT_SOURCE': 'image'}}
        guest_environment.prepare_native_guest_environment(selections, environment)
        self.assertEqual(environment['DISPLAY'], ':fixture')
        self.assertEqual(environment['XAUTHORITY'], '/fixture/authority')
        self.assertEqual(environment['DOCKPIPE_VM_QEMU_REAL_BIN'], '/fixture/qemu/bin/qemu-system-x86_64')
        self.assertEqual(environment['DOCKPIPE_VM_ACCEL'], 'kvm')
        self.assertEqual(environment['DOCKPIPE_VM_MEMORY'], '4096')
        self.assertEqual(environment['DOCKPIPE_VM_CPUS'], '4')
        self.assertEqual(environment['DOCKPIPE_VM_GUEST_PREPARATION'], 'provided-only')
        self.assertEqual(environment['DOCKPIPE_VM_BOOT_SOURCE'], 'image')
        self.assertEqual(environment['LD_LIBRARY_PATH'], '/fixture/qt/lib:/fixture/browser/lib')

    def test_insufficient_memory_blocks_before_vm_start(self):
        with tempfile.TemporaryDirectory() as temporary:
            filesystem = type('Filesystem', (), {'f_bavail': 64 << 30, 'f_frsize': 1})()
            with patch.object(Path, 'read_text', return_value='MemAvailable: 1048576 kB\n'):
                with patch.object(guest_environment.os, 'statvfs', return_value=filesystem):
                    with self.assertRaisesRegex(RuntimeError, '6 GiB'):
                        guest_environment.resource_admission(temporary)

    def test_missing_disk_capacity_blocks_before_vm_start(self):
        with tempfile.TemporaryDirectory() as temporary:
            filesystem = type('Filesystem', (), {'f_bavail': 8 << 30, 'f_frsize': 1})()
            with patch.object(Path, 'read_text', return_value='MemAvailable: 16777216 kB\n'):
                with patch.object(guest_environment.os, 'statvfs', return_value=filesystem):
                    with self.assertRaisesRegex(RuntimeError, '32 GiB'):
                        guest_environment.resource_admission(temporary)

    def test_symlink_cannot_select_pinned_input(self):
        with tempfile.TemporaryDirectory() as temporary:
            target = Path(temporary) / 'target'
            target.write_text('fixture')
            link = Path(temporary) / 'link'
            link.symlink_to(target)
            with self.assertRaises(ValueError):
                driver.regular_absolute(str(link))

    def test_other_platform_cannot_acquire_linux_vm_lane(self):
        with tempfile.TemporaryDirectory() as temporary:
            request = {'schema': driver.SCHEMA, 'run_id': 'fixture', 'plan_sha256': 'a' * 64,
                'registry_sha256': 'b' * 64, 'target': {'id': 'windows', 'platform': 'windows-amd64-msi',
                'kind': 'vm', 'artifact_sha256': 'c' * 64}, 'configuration': {}, 'cases': []}
            path = Path(temporary) / 'request.json'
            path.write_text(json.dumps(request))
            with self.assertRaisesRegex(ValueError, 'Ubuntu'):
                driver.load_request(path)


if __name__ == '__main__':
    unittest.main()
