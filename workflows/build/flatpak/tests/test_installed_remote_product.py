"""Generated semantic receipts only; never launch a GUI or paired operation."""
from pathlib import Path
import copy
import hashlib
import importlib.util
import json
import os
import tempfile
import unittest
from types import SimpleNamespace
from unittest.mock import patch

SCRIPTS = Path(__file__).resolve().parents[1] / 'assets/scripts'


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), SCRIPTS / (name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


evidence = load('installed-remote-evidence')
owner = load('installed-remote-product')


def generated_records():
    token = {'pid': 12345, 'starttime': '123456'}
    gui = {'commit': '1' * 64, 'executable_sha256': '2' * 64, 'host_pid': 12345,
           'kernel_starttime': '123456', 'namespace_pids': [12345, 2]}
    reference = {'id': '3' * 64, 'target': {'node': 'generated', 'fingerprint': '4' * 64, 'generation': '5' * 64}}
    binding = {'nonce': '6' * 32, 'display_id': 17, 'executable_sha256': '7' * 64}
    configuration = {'installed_commit': '1' * 64, 'gui_sha256': '2' * 64,
                     'cli_sha256': '8' * 64, 'qualified_provider': {'module_sha256': '9' * 64}}
    plan = {'run_id': 'generated-run', 'configuration': configuration,
            'facade': {'configuration': {'desktop_target': reference['target'], 'capture_display_id': 17}}}
    core = {'reference': reference, 'state': 'running', 'ack': 1, 'lease_millis': 0}
    gate = {'full_generated_fixture_gate': True}
    candidate = {'binding_sha256': evidence.checksum(binding), 'reference_sha256': evidence.checksum(reference),
                 'core_sha256': evidence.checksum(core), 'gate_sha256': evidence.checksum(gate),
                 'phase': 0, 'received_monotonic': 10.0, 'capture_before_monotonic': 10.1}
    window = {'id': 27, 'x': 0, 'y': 0, 'width': 1180, 'height': 1008,
              'gui': dict(token, namespace_pid=2)}
    facts = {key: True for key in ['normal_gui_started', 'clipboard_preferences_verified',
             'remote_session_opened', 'frames_observed', 'remote_closure_verified',
             'local_cleanup_verified', 'gui_exit_normal', 'host_default_profile_unchanged']}
    facts['generated_input_observed'] = False
    records = {
        'result': {'status': 'passed', 'facts': facts, 'generated_remote_input_sent': False,
                   'installed_control_acceptance_claimed': False, 'product_case_acceptance_claimed': False},
        'gui': gui, 'binding': binding, 'reference': reference, 'fixture_build': {'fixture': {'executable_sha256': '7' * 64}},
        'control_cli': {'executable_sha256': '8' * 64, 'public_command': 'remote desktop --stdio',
                        'selected_state_matches': True, 'request_or_credentials_read': False, 'verified_monotonic': 10.01},
        'module': {'module_sha256': '9' * 64, 'verified_monotonic': 10.01},
        'consumed': {'candidate': candidate, 'capture_before_monotonic': 10.1,
                     'consumed_monotonic': 10.015, 'clock_reset': False},
        'events': [{'action': 'startup-frame-status', 'result': {'reference': reference, 'core': core,
                    'fixture_gate': gate, 'frame_admission': candidate}}],
        'timing': {'window_before': copy.deepcopy(window), 'window_after': copy.deepcopy(window),
                   'window_paused': copy.deepcopy(window), 'validation_complete': True, 'image_destroyed_once': True,
                   'capture_before_monotonic': 10.1, 'started_monotonic': 10.02,
                   'image_received_monotonic': 10.03, 'capture_finished_monotonic': 10.04,
                   'pause_before_monotonic': 10.1, 'pause': {'gui': token, 'pause_observed_monotonic': 10.045,
                   'input_permitted': False, 'carrier_signalled': False}, 'decode_started_monotonic': 10.05,
                   'decode_finished_monotonic': 10.06, 'marker_verified_monotonic': 10.07,
                   'full_gui_verified_monotonic': 10.08,
                   'visible_owned_rect': {'x': 0, 'y': 0, 'width': 1180, 'height': 1000}},
        'marker': {'schema': 'dockpipe.desktop.nonce-marker/v1', 'nonce': '6' * 32, 'phase': 0,
                   'matched_nonce_bits': 128, 'captured_pixels_persisted': False,
                   'rect': {'x': 400, 'y': 300, 'width': 100, 'height': 60}},
        'budget': {'terminate_at_monotonic': 22.1, 'capture_expiry_bound_seconds': 52.5, 'fixture_hard_close_seconds': 55},
        'proof': {'schema': 'dockpipe.flatpak.gui-lease-expiry-observation/v1', 'gui': token,
                  'paused_at_monotonic': 10.045, 'before_hard_cutoff': True, 'core_closed': True,
                  'native_idle': True, 'cleanup_assisted': False, 'input_permitted': False,
                  'closure_observed_monotonic': 21.5},
        'watchdog': {'run_id': 'generated-run', 'failure': '', 'reason': 'owned GUI group already absent',
                     'owned_gui_group_absent': True, 'remaining': [], 'ownership_observation_errors': [],
                     'tokens': [{'pid': 12345}]},
        'cleanup': {'errors': [], 'primary_error': None}, 'journal': {'empty': True, 'modified': False},
        'owner_close': {'core_closed': True, 'native_idle': True, 'fixture_pid_absent': True,
                        'errors': [], 'emergency_close_attempted': False},
        'observer_context': {'pid_namespace': 'generated-namespace', 'mount_namespace': 'generated-mount', 'procfs_device': 17, 'observer_pid': 12344},
        'session': {'gui_pid': 12345, 'bus_pid': 12346, 'xvfb_pid': 12347},
        'host': {'observer_context': {'pid_namespace': 'generated-namespace', 'mount_namespace': 'generated-mount', 'procfs_device': 17},
                 'same_runner_observation_namespace': True, 'runner_observer_pid': 12344, 'pids': [{'pid': pid, 'proc_entry_absent': True} for pid in [12345, 12346, 12347]],
                 'all_recorded_pids_absent': True, 'retained_directories': [], 'recursive_deletion_performed': False,
                 'display': {'lock_absent': True, 'socket_absent': True}},
        'postflight': {'schema': 'dockpipe.desktop.live-reconciliation/v1', 'mode': 'postflight',
                       'run_id': 'generated-run', 'status': 'passed', 'reference': reference,
                       'core': dict(core, state='closed'),
                       'native': {'native_idle_proven': True, 'exact_fixture_pid_absent': True}},
    }
    return plan, records


class ProductEvidenceTests(unittest.TestCase):
    def test_complete_generated_semantics_admit_only_distinct_expiry_assertions(self):
        plan, records = generated_records()
        assertions = evidence.admit(plan, records)
        self.assertEqual(set(evidence.ASSERTIONS), {item['id'] for item in assertions})
        self.assertNotIn('generated-physical-input', evidence.ASSERTIONS)

    def test_top_level_pass_cannot_hide_timing_identity_input_or_cleanup_fault(self):
        changes = [
            ('marker', 'nonce', '0' * 32), ('marker', 'matched_nonce_bits', 127),
            ('marker', 'captured_pixels_persisted', True), ('marker', 'phase', 1),
            ('gui', 'commit', '0' * 64), ('control_cli', 'executable_sha256', '0' * 64),
            ('module', 'module_sha256', '0' * 64), ('module', 'verified_monotonic', 10.2), ('timing', 'capture_finished_monotonic', 10.11),
            ('timing', 'full_gui_verified_monotonic', 23.0), ('proof', 'cleanup_assisted', True),
            ('proof', 'closure_observed_monotonic', 22.2), ('watchdog', 'failure', 'intervened'),
            ('journal', 'empty', False), ('journal', 'modified', True),
            ('host', 'retained_directories', ['/tmp/generated-retained']),
        ]
        for section, key, value in changes:
            plan, records = generated_records()
            records[section][key] = value
            with self.subTest(section=section, key=key), self.assertRaises(ValueError):
                evidence.admit(plan, records)

    def test_core_gate_reference_geometry_and_actual_stop_cannot_be_rebound(self):
        changes = [lambda r: r['postflight']['core'].update(state='running'),
                   lambda r: r['postflight']['core'].update(lease_millis=1000),
                   lambda r: r['postflight'].update(reference={'id': 'foreign'}),
                   lambda r: r['events'][0]['result']['core'].update(ack=0),
                   lambda r: r['events'][0]['result']['fixture_gate'].update(changed=True),
                   lambda r: r['consumed']['candidate'].update(binding_sha256='0' * 64),
                   lambda r: r['timing']['window_after'].update(width=1100),
                   lambda r: r['timing']['pause'].update(pause_observed_monotonic=10.2),
                   lambda r: r['host']['pids'].pop(),
                   lambda r: r['host']['observer_context'].update(pid_namespace='hidden-namespace'),
                   lambda r: r['marker']['rect'].update(y=990)]
        for change in changes:
            plan, records = generated_records()
            change(records)
            with self.subTest(change=change), self.assertRaises(ValueError):
                evidence.admit(plan, records)

    def test_disabled_or_unrequested_case_launches_nothing(self):
        for requested in [True, False]:
            with tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / 'output'
                request = {'schema': 'dockpipe.product-integration.request/v1', 'run_id': 'generated-run',
                           'plan_sha256': '1' * 64, 'registry_sha256': '2' * 64,
                           'target': {'id': 'generated', 'kind': 'flatpak', 'platform': 'linux-amd64-flatpak-kde6.10',
                                      'artifact_sha256': '3' * 64},
                           'configuration': {'observer_context': owner.observer_context(), 'expiry_plan': '/missing/generated-plan', 'expiry_plan_sha256': '4' * 64,
                                             'execute_reviewed': not requested, 'host_profile': {}},
                           'cases': [{'id': owner.FEATURE if requested else 'remote.desktop.viewer',
                                      'scenario': 'failure', 'layer': 'installed_e2e',
                                      'required_assertions': list(evidence.ASSERTIONS)}]}
                with patch.object(owner, 'provider_readback') as readback, patch.object(owner, 'run_driver') as process:
                    owner.run(request, output)
                    readback.assert_not_called()
                    process.assert_not_called()
                result = json.loads((output / 'result.json').read_text())
                self.assertEqual('not_run', result['cases'][0]['status'])
                self.assertFalse(result['observed']['effectful_gui_invocation_started'])

    def test_old_run_or_changed_target_blocks_before_provider_or_gui(self):
        for changed in ['run_id', 'artifact']:
            with tempfile.TemporaryDirectory() as directory:
                payload = b'{"generated_private_plan":true}'
                plan = {'run_id': 'old-run' if changed == 'run_id' else 'generated-run',
                        'registry': {'sha256': '2' * 64},
                        'configuration': {'bundle': {'sha256': '0' * 64}}}
                request = {'schema': 'dockpipe.product-integration.request/v1', 'run_id': 'generated-run',
                           'plan_sha256': '1' * 64, 'registry_sha256': '2' * 64,
                           'target': {'id': 'generated', 'kind': 'flatpak', 'platform': 'linux-amd64-flatpak-kde6.10',
                                      'artifact_sha256': '3' * 64},
                           'configuration': {'observer_context': owner.observer_context(),
                                             'expiry_plan': '/generated/private-plan',
                                             'expiry_plan_sha256': hashlib.sha256(payload).hexdigest(),
                                             'execute_reviewed': True, 'host_profile': {'ID': 'generated', 'VERSION_ID': '1'}},
                           'cases': [{'id': owner.FEATURE, 'scenario': 'failure', 'layer': 'installed_e2e',
                                      'required_assertions': list(evidence.ASSERTIONS)}]}
                runner = SimpleNamespace(captured_json=lambda path: (payload, plan), verify_plan=lambda plan: None)
                original_load = owner.load
                def selected_load(name, path):
                    return runner if name == 'remote_product_runner' else original_load(name, path)
                with patch.object(owner, 'load', side_effect=selected_load), \
                        patch.object(owner, 'host_profile', return_value=request['configuration']['host_profile']), \
                        patch.object(owner, 'provider_readback') as readback, patch.object(owner, 'run_driver') as process:
                    output = Path(directory) / 'output'
                    owner.run(request, output)
                    readback.assert_not_called()
                    process.assert_not_called()
                self.assertEqual('blocked', json.loads((output / 'result.json').read_text())['cases'][0]['status'])

    def test_independent_cleanup_refuses_namespace_drift_and_retains_broken_links(self):
        with tempfile.TemporaryDirectory() as directory:
            operation = Path(directory)
            owner.write_json(operation / 'observer-namespace.json',
                             dict(owner.observer_context(), schema='dockpipe.private.gui-observer-namespace/v1',
                                  observer_pid=12344))
            owner.write_json(operation / 'watchdog-cleanup.json', {'tokens': [{'pid': 99999999}]})
            desktop = operation / 'normal-gui/desktop'
            desktop.mkdir(parents=True)
            with tempfile.TemporaryDirectory(prefix='dockpipe-remote-runtime-') as runtime_name, \
                    tempfile.TemporaryDirectory(prefix='dockpipe-bus-') as bus_name:
                runtime, bus = Path(runtime_name), Path(bus_name)
                runtime.rmdir()
                bus.rmdir()
                runtime.symlink_to('/missing-generated-runtime')
                bus.symlink_to('/missing-generated-bus')
                try:
                    owner.write_json(desktop / 'session-private.json', {
                        'gui_pid': 99999998, 'bus_pid': 99999997, 'xvfb_pid': 99999996,
                        'environment': {'XDG_RUNTIME_DIR': str(runtime),
                                        'DBUS_SESSION_BUS_ADDRESS': 'unix:path=' + str(bus) + '/session-bus',
                                        'DISPLAY': ':9876'}})
                    observed = owner.independent_host(operation)
                    self.assertEqual({str(runtime), str(bus)}, set(observed['retained_directories']))
                    self.assertTrue(os.path.lexists(runtime))
                    self.assertTrue(os.path.lexists(bus))
                    with patch.object(owner, 'observer_context', return_value={'pid_namespace': 'hidden', 'procfs_device': 0}):
                        with self.assertRaises(ValueError):
                            owner.independent_host(operation)
                finally:
                    runtime.unlink()
                    bus.unlink()

    def test_missing_plan_is_blocked_before_any_effect(self):
        with tempfile.TemporaryDirectory() as directory:
            request = {'schema': 'dockpipe.product-integration.request/v1', 'run_id': 'generated-run',
                       'plan_sha256': '1' * 64, 'registry_sha256': '2' * 64,
                       'target': {'id': 'generated', 'kind': 'flatpak', 'platform': 'linux-amd64-flatpak-kde6.10',
                                  'artifact_sha256': '3' * 64},
                       'configuration': {'observer_context': owner.observer_context(), 'expiry_plan': '/missing/generated-plan', 'expiry_plan_sha256': '4' * 64,
                                         'execute_reviewed': True, 'host_profile': {'ID': 'generated', 'VERSION_ID': '1'}},
                       'cases': [{'id': owner.FEATURE, 'scenario': 'failure', 'layer': 'installed_e2e',
                                  'required_assertions': list(evidence.ASSERTIONS)}]}
            with patch.object(owner, 'host_profile', return_value=request['configuration']['host_profile']), \
                    patch.object(owner, 'provider_readback') as readback, patch.object(owner, 'run_driver') as process:
                owner.run(request, Path(directory) / 'output')
                readback.assert_not_called()
                process.assert_not_called()
            result = json.loads((Path(directory) / 'output/result.json').read_text())
            self.assertEqual('blocked', result['cases'][0]['status'])
            self.assertFalse(result['observed']['effectful_gui_invocation_started'])


if __name__ == '__main__':
    unittest.main()
