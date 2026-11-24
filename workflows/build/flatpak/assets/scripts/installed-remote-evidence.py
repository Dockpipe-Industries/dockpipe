"""Semantic admission for fresh normal-GUI, no-input lease-expiry evidence."""
import hashlib
import json
import math

ASSERTIONS = (
    'fresh-full-nonce-in-normal-installed-viewer',
    'original-capture-and-stop-deadlines',
    'unassisted-core-native-expiry',
    'normal-gui-exit-journal-cleared',
    'exact-independent-cleanup',
)


def require(condition, message):
    if not condition:
        raise ValueError(message)


def checksum(value):
    data = json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()
    return hashlib.sha256(data).hexdigest()


def number(value):
    require(type(value) in [int, float] and math.isfinite(value), 'Expected a finite observation clock')
    return value


def admit(plan, records):
    """No top-level passed flag substitutes for the bound semantic observations."""
    result = records['result']
    facts = result['facts']
    require(result['status'] == 'passed' and result['generated_remote_input_sent'] is False
            and result['installed_control_acceptance_claimed'] is False
            and result['product_case_acceptance_claimed'] is False,
            'Owning GUI run did not finish within the reviewed no-input scope')
    for key in ['normal_gui_started', 'clipboard_preferences_verified', 'remote_session_opened',
                'frames_observed', 'remote_closure_verified', 'local_cleanup_verified',
                'gui_exit_normal', 'host_default_profile_unchanged']:
        require(facts.get(key) is True, 'Missing actual GUI observation: ' + key)
    require(facts.get('generated_input_observed') is False, 'Input exceeds this View-only case')
    configuration = plan['configuration']
    gui = records['gui']
    require(gui['commit'] == configuration['installed_commit']
            and gui['executable_sha256'] == configuration['gui_sha256'], 'Observed installed GUI differs')
    token = {'pid': gui['host_pid'], 'starttime': gui['kernel_starttime']}
    control, module = records['control_cli'], records['module']
    require(control['executable_sha256'] == configuration['cli_sha256']
            and control['public_command'] == 'remote desktop --stdio'
            and control['selected_state_matches'] is True
            and control['request_or_credentials_read'] is False
            and module['module_sha256'] == configuration['qualified_provider']['module_sha256'],
            'Actual installed control CLI or loaded provider differs')
    timing, consumed, marker = records['timing'], records['consumed'], records['marker']
    before, after, paused = (timing[name] for name in ['window_before', 'window_after', 'window_paused'])
    require(before == after == paused and before['gui'] == dict(token, namespace_pid=gui['namespace_pids'][-1])
            and timing['validation_complete'] is True and timing['image_destroyed_once'] is True,
            'Capture-time and paused original window/GUI binding differs')
    binding, reference = records['binding'], records['reference']
    require(reference['target'] == plan['facade']['configuration']['desktop_target']
            and binding['display_id'] == plan['facade']['configuration']['capture_display_id']
            and binding['executable_sha256'] == records['fixture_build']['fixture']['executable_sha256'],
            'Original desktop or fixture selection differs')
    candidate = consumed['candidate']
    require(candidate['binding_sha256'] == checksum(binding)
            and candidate['reference_sha256'] == checksum(reference)
            and candidate['phase'] == 0 and consumed['clock_reset'] is False,
            'Single-use frame admission lost the full fixture/reference binding')
    matching = [event for event in records['events'] if event.get('action') == 'startup-frame-status'
                and event.get('result', {}).get('frame_admission') == candidate]
    require(len(matching) == 1, 'Original consumed running-frame observation is absent or ambiguous')
    startup = matching[0]['result']
    require(startup['reference'] == reference and startup['core']['reference'] == reference
            and startup['core']['state'] == 'running' and startup['core'].get('ack', 0) >= 1
            and startup['core'].get('lease_millis') == 0 and not startup['core'].get('signals')
            and checksum(startup['core']) == candidate['core_sha256']
            and checksum(startup['fixture_gate']) == candidate['gate_sha256'],
            'Consumed frame no longer binds the original running Core and full owner gate')
    receipt = number(candidate['received_monotonic'])
    deadline = number(candidate['capture_before_monotonic'])
    require(abs(deadline - receipt - 0.1) <= 0.000001
            and consumed['capture_before_monotonic'] == timing['capture_before_monotonic'] == deadline,
            'Original frame receipt100ms deadline changed')
    start, image_received, captured = (number(timing[key]) for key in
                                     ['started_monotonic', 'image_received_monotonic', 'capture_finished_monotonic'])
    pause = timing['pause']
    stopped = number(pause['pause_observed_monotonic'])
    six = number(timing['pause_before_monotonic'])
    budget = records['budget']
    require(six + 12 == number(budget['terminate_at_monotonic'])
            and budget['capture_expiry_bound_seconds'] == 52.5
            and budget['fixture_hard_close_seconds'] == 55, 'Original timing model changed')
    require(receipt <= number(module['verified_monotonic']) <= number(consumed['consumed_monotonic'])
            and receipt <= number(control['verified_monotonic']) <= number(consumed['consumed_monotonic'])
            <= start <= image_received <= captured < deadline
            and captured < six and captured <= stopped <= six and pause['gui'] == token
            and pause['input_permitted'] is False and pause['carrier_signalled'] is False,
            'Original capture100ms or actual observed-stop6s guard failed')
    require(stopped <= number(timing['decode_started_monotonic'])
            <= number(timing['decode_finished_monotonic']) <= number(timing['marker_verified_monotonic'])
            <= number(timing['full_gui_verified_monotonic']) < six + 12,
            'Full nonce/GUI validation was not completed while paused before the original cutoff')
    require(marker['schema'] == 'dockpipe.desktop.nonce-marker/v1'
            and marker['nonce'] == binding['nonce'] and marker['phase'] == 0
            and marker['matched_nonce_bits'] == 128 and marker['captured_pixels_persisted'] is False,
            'Complete fresh generated nonce/phase was not observed without persistence')
    rect, visible = marker['rect'], timing['visible_owned_rect']
    require(rect['x'] >= 196 and rect['y'] >= 250
            and rect['x'] + rect['width'] < before['width']
            and rect['y'] + rect['height'] < min(1000, before['height'])
            and rect['x'] >= visible['x'] and rect['y'] >= visible['y']
            and rect['x'] + rect['width'] <= visible['x'] + visible['width']
            and rect['y'] + rect['height'] <= visible['y'] + visible['height'],
            'Complete nonce escapes the original viewer body or visible owned intersection')
    proof = records['proof']
    require(proof['schema'] == 'dockpipe.flatpak.gui-lease-expiry-observation/v1'
            and proof['gui'] == token and proof['paused_at_monotonic'] == stopped
            and proof['before_hard_cutoff'] is True and proof['core_closed'] is True
            and proof['native_idle'] is True and proof['cleanup_assisted'] is False
            and proof['input_permitted'] is False
            and number(timing['full_gui_verified_monotonic']) <= number(proof['closure_observed_monotonic']) < six + 12,
            'Original unassisted Core/native closure before18s is unproven')
    watchdog, cleanup = records['watchdog'], records['cleanup']
    require(watchdog['run_id'] == plan['run_id'] and watchdog['failure'] == ''
            and watchdog['reason'] == 'owned GUI group already absent'
            and watchdog['owned_gui_group_absent'] is True
            and not watchdog['remaining'] and not watchdog['ownership_observation_errors']
            and cleanup['errors'] == [] and cleanup['primary_error'] is None,
            'Watchdog intervened or owning cleanup remained incomplete')
    require(records['journal']['empty'] is True and records['journal']['modified'] is False,
            'Normal GUI did not clear its own original recovery journal')
    owner_close = records['owner_close']
    require(owner_close['core_closed'] is True and owner_close['native_idle'] is True
            and owner_close['fixture_pid_absent'] is True and owner_close['errors'] == []
            and owner_close['emergency_close_attempted'] is False, 'Owner closure required assistance or remained unknown')
    post, host = records['postflight'], records['host']
    core, native = post['core'], post['native']
    require(post['schema'] == 'dockpipe.desktop.live-reconciliation/v1'
            and post['mode'] == 'postflight' and post['run_id'] == plan['run_id']
            and post['status'] == 'passed' and post['reference'] == reference
            and core['reference'] == reference and core['state'] == 'closed'
            and core.get('ack', 0) >= 1 and core.get('lease_millis') == 0 and not core.get('signals')
            and native['native_idle_proven'] is True and native['exact_fixture_pid_absent'] is True,
            'Independent exact Core/native/fixture closure is unproven')
    context = records['observer_context']
    require(host['same_runner_observation_namespace'] is True
            and host['observer_context'] == {key: context[key] for key in ['pid_namespace', 'mount_namespace', 'procfs_device']}
            and host['runner_observer_pid'] == context['observer_pid'],
            'Independent cleanup was observed from a different PID namespace/procfs')
    expected_pids = {item['pid'] for item in watchdog['tokens']}
    expected_pids.update(records['session'][name] for name in ['gui_pid', 'bus_pid', 'xvfb_pid'])
    require({item['pid'] for item in host['pids']} == expected_pids
            and all(item['proc_entry_absent'] is True for item in host['pids']),
            'Independent local readback omitted an original recorded process')
    require(host['all_recorded_pids_absent'] is True and host['retained_directories'] == []
            and host['display']['lock_absent'] is True and host['display']['socket_absent'] is True
            and host['recursive_deletion_performed'] is False,
            'Independent exact local cleanup is incomplete')
    return [{'id': name, 'status': 'passed', 'detail': 'Fresh bound semantic evidence verified'} for name in ASSERTIONS]
