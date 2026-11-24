"""Opt-in product receipt for the owning normal Flatpak GUI expiry workflow."""
from pathlib import Path
import argparse
import hashlib
import importlib.util
import json
import os
import platform
import signal
import sys

HERE = Path(__file__).resolve().parent
REPOSITORY = HERE.parents[4]
sys.path.insert(0, str(REPOSITORY / 'workflows/qualify/product/assets/scripts'))
from contracts import digest_file, read_json, read_json_document, require, write_json
from process import run_driver

FEATURE = 'remote.desktop.viewer.lease-expiry'


def load(name, path):
    specification = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


def host_profile():
    require(sys.platform == 'linux' and platform.machine() == 'x86_64', 'This owning GUI lane requires Linux amd64')
    values = {}
    with Path('/etc/os-release').open() as stream:
        data = stream.read(16385)
    require(len(data) <= 16384, 'OS identity exceeds its metadata bound')
    for line in data.splitlines():
        key, separator, value = line.partition('=')
        if separator and key in ['ID', 'VERSION_ID']:
            values[key] = value.strip('"')
    require(set(values) == {'ID', 'VERSION_ID'}, 'Exact executing host OS identity is unavailable')
    return values


def observer_context():
    return {'pid_namespace': os.readlink('/proc/self/ns/pid'),
            'mount_namespace': os.readlink('/proc/self/ns/mnt'),
            'procfs_device': Path('/proc').stat().st_dev}


def selected_case(case):
    return (case['id'], case['scenario'], case['layer']) == (FEATURE, 'failure', 'installed_e2e')


def provider_readback(plan, output, mode, reference=None, binding=None):
    """Invoke the versioned owner archive; no provider source or policy is copied here."""
    output.mkdir(mode=0o700)
    selection = plan['facade']['selection']
    request = {'schema': 'dockpipe.desktop.live-reconciliation/v1', 'run_id': plan['run_id'],
               'mode': mode, 'configuration': plan['facade']['configuration'],
               'reference': reference, 'binding': binding}
    request_path = output / 'request.json'
    write_json(request_path, request)
    consumer = load('remote_product_consumer', HERE / 'installed-remote.py')
    environment = consumer.profile_environment(output / 'profile', '/unused-private-installation')
    environment['PYTHONDONTWRITEBYTECODE'] = '1'
    logs = output / 'process'
    logs.mkdir(mode=0o700)
    command = [selection['cli']['path'], '--workflow-file',
               'tar://' + selection['archive']['path'] + '##' + selection['member'], '--no-vault', '--',
               '--reconcile', '--request', str(request_path), '--output', str(output / 'owner'),
               '--execute-reviewed', '--approve-request-sha256', digest_file(request_path)]
    process = run_driver(command, logs, 100, environment, output)
    write_json(logs / 'result.json', process)
    require(process['exit_code'] == 0 and not process['failure'],
            'Owner read-only reconciliation failed; preserve its exact references without replay')
    result = read_json(output / 'owner/result.json')
    require(result.get('run_id') == plan['run_id'] and result.get('mode') == mode,
            'Reconciliation did not bind this exact fresh invocation')
    return result


def independent_host(operation):
    context = read_json(operation / 'observer-namespace.json')
    current_context = observer_context()
    require(context.get('schema') == 'dockpipe.private.gui-observer-namespace/v1'
            and {key: context.get(key) for key in current_context} == current_context,
            'Cleanup observer namespace/procfs differs from the invocation that admitted original GUI host IDs')
    watchdog = read_json(operation / 'watchdog-cleanup.json')
    session = read_json(operation / 'normal-gui/desktop/session-private.json')
    pids = {token['pid'] for token in watchdog['tokens']}
    pids.update(session[name] for name in ['gui_pid', 'bus_pid', 'xvfb_pid'])
    require(len(pids) <= 256 and all(type(pid) is int and pid > 1 for pid in pids),
            'Recorded owned local PID inventory exceeds its bound')
    environment = session['environment']
    runtime = Path(environment['XDG_RUNTIME_DIR'])
    address = environment['DBUS_SESSION_BUS_ADDRESS']
    require(runtime.parent == Path('/tmp') and runtime.name.startswith('dockpipe-remote-runtime-')
            and address.startswith('unix:path=/tmp/dockpipe-bus-'), 'Unknown private runtime or bus scope')
    bus = Path(address.split('unix:path=', 1)[1].split('/session-bus', 1)[0])
    require(bus.parent == Path('/tmp') and bus.name.startswith('dockpipe-bus-'), 'Unknown owned socket directory')
    directories = [{'path': str(path), 'absent': not os.path.lexists(path)} for path in [runtime, bus]]
    display = environment['DISPLAY']
    require(display.startswith(':') and display[1:].isdigit(), 'Unknown owned display identity')
    number = display[1:]
    return {'schema': 'dockpipe.private.exact-host-cleanup/v1', 'observer_context': current_context,
            'runner_observer_pid': context['observer_pid'], 'same_runner_observation_namespace': True,
            'pids': [{'pid': pid, 'proc_entry_absent': not os.path.lexists(Path('/proc', str(pid)))} for pid in sorted(pids)],
            'all_recorded_pids_absent': all(not os.path.lexists(Path('/proc', str(pid))) for pid in pids),
            'directories': directories, 'retained_directories': [item['path'] for item in directories if not item['absent']],
            'display': {'name': display, 'lock_absent': not os.path.lexists(Path('/tmp', '.X' + number + '-lock')),
                        'socket_absent': not os.path.lexists(Path('/tmp/.X11-unix', 'X' + number))},
            'recursive_deletion_performed': False}


def semantic_records(plan, operation, output, postflight):
    paths = {'result': 'result.json', 'gui': 'normal-gui/gui-identity.json',
             'control_cli': 'actual-installed-control-cli.json', 'module': 'actual-loaded-provider.json',
             'timing': 'startup-frame-timings.json', 'marker': 'actual-gui-fresh-marker.json',
             'binding': 'provider-owner/owner/fixture-binding.json',
             'reference': 'provider-owner/owner/gui-original-reference.json',
             'consumed': 'provider-owner/owner/startup-frame-consumed.json',
             'proof': 'unassisted-expiry.json', 'watchdog': 'watchdog-cleanup.json', 'cleanup': 'operation-cleanup.json',
             'budget': 'provider-owner/owner/qualification-budget.json',
             'owner_close': 'provider-owner/owner/nonce-close.json'}
    records = {name: read_json(operation / path) for name, path in paths.items()}
    event_paths = sorted((operation / 'provider-owner/owner').glob('event-*.json'))
    require(len(event_paths) <= 40, 'Owner event inventory exceeds the finite command contract')
    records['events'] = [read_json(path) for path in event_paths]
    runner = load('remote_product_runner_evidence', HERE / 'installed-remote-expiry-run.py')
    records['fixture_build'] = runner.pinned_json(plan['facade']['configuration']['fixture_build'])
    journal = load('remote_product_journal', HERE / 'installed-remote-journal.py')
    home = operation / 'normal-gui/private-home'
    settings = home / '.var/app/com.dockpipe.Dockpipe/config/dockpipe/dockpipe-launcher.conf'
    records['journal'] = {'empty': not journal.read(settings, home), 'modified': False}
    records['host'] = independent_host(operation)
    records['observer_context'] = read_json(operation / 'observer-namespace.json')
    records['session'] = read_json(operation / 'normal-gui/desktop/session-private.json')
    records['postflight'] = postflight
    if not (output / 'independent-host.json').exists():
        write_json(output / 'independent-host.json', records['host'])
    write_json(output / 'journal-readback.json', records['journal'])
    # Bind the exact interpreted documents. These fresh records remain evidence,
    # not reusable acceptance receipts for another run or artifact.
    write_json(output / 'semantic-records.json', records)
    return records


def run(request, output):
    evidence = load('remote_product_evidence', HERE / 'installed-remote-evidence.py')
    configuration = request['configuration']
    require(set(configuration) == {'expiry_plan', 'expiry_plan_sha256', 'execute_reviewed', 'host_profile', 'observer_context'},
            'Select exact expiry plan+digest, explicit execution choice and independently observed host profile')
    require(type(configuration['execute_reviewed']) is bool, 'Execution selection must be boolean')
    selected_cases = [case for case in request['cases'] if selected_case(case)]
    for case in selected_cases:
        require(set(case['required_assertions']) == set(evidence.ASSERTIONS),
                'Requested expiry assertions differ from the versioned no-input case')
    output.mkdir(mode=0o700, parents=True)
    write_json(output / 'request-captured.json', request)
    status, reason, assertions = 'not_run', 'Explicit reviewed expiry execution is disabled', []
    operation = output / 'operation'
    effect_started = False
    try:
        require(request['schema'] == 'dockpipe.product-integration.request/v1', 'Unknown product request')
        supported_target = (request['target']['kind'] == 'flatpak'
                            and request['target']['platform'] in ['linux-amd64-flatpak-kde6.10',
                                                                  'linux-amd64-flatpak-bazzite-kde6.10'])
        if not supported_target:
            reason = 'This owner does not implement the selected installed platform/role'
        if configuration['execute_reviewed'] and selected_cases and supported_target:
            try:
                require(observer_context() == configuration['observer_context'],
                        'Current observer differs from the explicitly reviewed host namespace/procfs')
                require(host_profile() == configuration['host_profile'], 'Executing Linux host OS/profile changed')
                require((host_profile()['ID'] == 'bazzite')
                        == ('bazzite' in request['target']['platform']), 'Target Bazzite attribution differs from executing OS')
                runner = load('remote_product_runner', HERE / 'installed-remote-expiry-run.py')
                data, plan = runner.captured_json(Path(configuration['expiry_plan']))
                require(hashlib.sha256(data).hexdigest() == configuration['expiry_plan_sha256'], 'Reviewed expiry plan changed')
                require(plan['run_id'] == request['run_id'] and plan['registry']['sha256'] == request['registry_sha256']
                        and plan['configuration']['bundle']['sha256'] == request['target']['artifact_sha256'],
                        'Product run/registry/installed artifact does not match the exact expiry plan')
                runner.verify_plan(plan)
                write_json(output / 'expiry-plan-captured.json', plan)
                before = provider_readback(plan, output / 'preflight', 'preflight')
                require(before['status'] == 'passed', before['reason'])
                require(before['native']['native_idle_proven'] is True
                        and {key: before['native']['current_display'][key] for key in ['display_id', 'bounds']}
                        == {key: plan['expected_display'][key] for key in ['display_id', 'bounds']},
                        'Current native idle or selected display prerequisite changed')
            except (OSError, ValueError, RuntimeError, TimeoutError) as error:
                status, reason = 'blocked', 'Prerequisite unavailable: ' + str(error)
            else:
                consumer = load('remote_product_consumer_execute', HERE / 'installed-remote.py')
                environment = consumer.profile_environment(output / 'profile', '/unused-private-installation')
                logs = output / 'gui-process'
                logs.mkdir(mode=0o700)
                command = [sys.executable, str(HERE / 'installed-remote-expiry-run.py'),
                           '--plan', configuration['expiry_plan'], '--output', str(operation),
                           '--execute-reviewed', '--approve-plan-sha256', configuration['expiry_plan_sha256']]
                effect_started = True
                process = None
                postflight = None
                try:
                    process = run_driver(command, logs, 180, environment, output)
                    write_json(logs / 'result.json', process)
                finally:
                    reference_path = operation / 'provider-owner/owner/gui-original-reference.json'
                    binding_path = operation / 'provider-owner/owner/fixture-binding.json'
                    try:
                        if reference_path.exists() and binding_path.exists():
                            reference = read_json(reference_path)
                            binding = read_json(binding_path)
                            postflight = provider_readback(plan, output / 'postflight', 'postflight', reference, binding)
                    finally:
                        if (operation / 'watchdog-cleanup.json').exists():
                            write_json(output / 'independent-host.json', independent_host(operation))
                require(process is not None and process['exit_code'] == 0 and not process['failure'],
                        'Normal GUI expiry did not complete normally; preserve failed or unknown effects')
                require(postflight is not None, 'Original generated reference/fixture postflight is unavailable')
                records = semantic_records(plan, operation, output, postflight)
                assertions = evidence.admit(plan, records)
                status, reason = 'passed', 'Fresh installed GUI View-only expiry semantics and independent closure verified'
    except (Exception, KeyboardInterrupt) as error:
        status, reason = 'failed', 'Effectful expiry result is failed or unknown: ' + str(error)
    proof_paths = [output / 'request-captured.json']
    proof_paths += [path for path in [output / 'expiry-plan-captured.json', output / 'semantic-records.json',
                                    output / 'independent-host.json', output / 'journal-readback.json',
                                    output / 'gui-process/result.json', output / 'postflight/owner/result.json',
                                    output / 'preflight/owner/result.json', output / 'preflight/process/result.json',
                                    output / 'postflight/process/result.json'] if path.exists()]
    proof = [{'path': path.relative_to(output).as_posix(), 'sha256': digest_file(path)} for path in proof_paths]
    cases = []
    for case in request['cases']:
        selected = selected_case(case)
        require(not selected or set(case['required_assertions']) == set(evidence.ASSERTIONS),
                'Requested expiry assertions differ from the versioned no-input case')
        cases.append(dict({name: case[name] for name in ['id', 'scenario', 'layer']},
                          status=status if selected else 'not_run',
                          reason=reason if selected else 'This owner implements only the distinct View-only expiry case',
                          assertions=assertions if selected and status == 'passed' else [], evidence=proof))
    receipt = {name: request[name] for name in ['run_id', 'plan_sha256', 'registry_sha256', 'target']}
    receipt.update(schema='dockpipe.product-integration.lane/v1', cases=cases,
                   observed={'effectful_gui_invocation_started': effect_started, 'control_credit': False,
                             'availability_credit': False, 'historical_receipts_imported': False})
    write_json(output / 'result.json', receipt)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--request', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    require(args.output.is_absolute() and not args.output.exists(), 'Fresh private product output required')
    def interrupted(number, _frame):
        raise KeyboardInterrupt('Product expiry interrupted: ' + str(number))
    for selected in [signal.SIGTERM, signal.SIGHUP]:
        signal.signal(selected, interrupted)
    request, _data = read_json_document(args.request)
    run(request, args.output)


if __name__ == '__main__':
    main()
