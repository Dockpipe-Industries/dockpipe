"""Opt-in local host workflow lifecycle checks through the installed CLI.

Container detach is outside this host-only lane. Running a CLI under this
supervisor never counts as product detach.
"""
from pathlib import Path
from datetime import datetime
import argparse
import json
import os
import platform
import re
import secrets
import shutil
import signal
import subprocess
import sys
import time

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'assets/scripts'))
from contracts import digest_file, read_json, require, validate_receipt, write_json
from process import MAX_LOG_BYTES, run_driver


ASSERTIONS = {
    ('happy', 'installed_e2e'): {'detached-run-observed', 'logs-and-result-bound', 'owned-process-tree-closed'},
    ('failure', 'component'): {'expected-failure-observed', 'no-unowned-effects', 'actionable-result'},
    ('cancellation', 'component'): {'cancellation-observed', 'owned-work-stopped', 'no-implicit-replay'},
    ('restart-recovery', 'component'): {'original-identity-retained', 'state-reconciled', 'no-duplicate-effects'},
}
OBSERVABILITY_ASSERTIONS = {'selected-environment-identified', 'system-and-run-state', 'bounded-refresh'}

WORKER = '''from pathlib import Path
import json
import os
import signal
import subprocess
import sys
import time

root = Path.cwd()
task = sys.argv[1]
token = os.environ['LIFECYCLE_TOKEN']
mode = os.environ['LIFECYCLE_MODE']

def publish(name, value):
    temporary = root / (name + '.new')
    temporary.write_text(json.dumps(value))
    temporary.replace(root / name)

if task == 'join':
    assert all((root / ('finished-' + name)).read_text() == token for name in ['first', 'second'])
    assert os.environ['LIFECYCLE_BRANCH'] == 'second'
    publish('result.json', {'token': token, 'joined': ['first', 'second'], 'last_branch': 'second'})
    print('joined:' + token, flush=True)
    raise SystemExit(0)

def identity(pid):
    stat = Path('/proc', str(pid), 'stat').read_text().rsplit(')', 1)[1].split()
    boot = Path('/proc/sys/kernel/random/boot_id').read_text().strip()
    return {'pid': pid, 'start_token': boot + ':' + stat[19]}

child = subprocess.Popen(['/usr/bin/python3', '-c', 'import time; time.sleep(120)'])
stopped = False

def request_stop(_number, _frame):
    global stopped
    stopped = True

signal.signal(signal.SIGTERM, request_stop)
signal.signal(signal.SIGINT, request_stop)
if mode == 'restart-recovery':
    with (root / ('starts-' + task)).open('a') as stream:
        stream.write(token + '\\n')
publish('started-' + task + '.json', {'task': task, 'token': token,
    'worker': identity(os.getpid()), 'child': identity(child.pid)})
print('started:' + task + ':' + token, flush=True)
deadline = time.monotonic() + 120
try:
    while not stopped and not (root / 'release').exists() and mode != 'failure':
        if time.monotonic() >= deadline:
            raise RuntimeError('Fixture safety deadline reached')
        time.sleep(0.02)
finally:
    child.terminate()
    child.wait(timeout=5)
if stopped:
    (root / ('cancelled-' + task)).write_text(token)
    raise SystemExit(143)
if mode == 'failure':
    raise SystemExit(7)
output = Path(os.environ['DOCKPIPE_OUTPUT_ROOT']) / (task + '.env')
output.parent.mkdir(parents=True, exist_ok=True)
output.write_text('LIFECYCLE_BRANCH=' + task + '\\n')
(root / ('finished-' + task)).write_text(token)
print('finished:' + task + ':' + token, flush=True)
'''


def identity(pid):
    try:
        fields = Path('/proc', str(pid), 'stat').read_text().rsplit(')', 1)[1].split()
    except FileNotFoundError:
        return None
    boot = Path('/proc/sys/kernel/random/boot_id').read_text().strip()
    return {'pid': pid, 'start_token': boot + ':' + fields[19]}


def alive(reference):
    return identity(reference['pid']) == reference


def wait_for(observe, seconds, process=None):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        result = observe()
        if result:
            return result
        if process is not None and process.poll() is not None:
            raise RuntimeError('Workflow exited before the required live observation')
        time.sleep(0.03)
    raise TimeoutError('Required lifecycle observation did not arrive before its deadline')


def checked_command(cli, arguments, workspace, environment, output, name):
    logs = output / name
    logs.mkdir(mode=0o700)
    result = run_driver([str(cli), *arguments], logs, 10, environment, workspace)
    write_json(logs / 'process.json', result)
    require(result['exit_code'] == 0 and not result['failure'], 'Public observation command failed: ' + name)
    return read_json(logs / 'driver.stdout')


def observed_members(snapshot, started, parent_pid, joined=False):
    require(isinstance(snapshot, dict) and snapshot.get('schema') == 'dockpipe.execution_snapshot.v1',
            'Unknown execution snapshot schema')
    require(isinstance(snapshot.get('executions'), list) and len(snapshot['executions']) <= 256
            and all(isinstance(record, dict) for record in snapshot['executions']),
            'Execution snapshot has invalid record shapes')
    expected_tasks = {'first', 'second', 'join'} if joined else {'first', 'second'}
    records = [record for record in snapshot.get('executions', [])
               if record.get('parent_pid') == parent_pid and record.get('task') in {'first', 'second', 'join'}]
    require(len(records) == len(expected_tasks) and {record.get('task') for record in records} == expected_tasks,
            'Declared tasks were duplicated, replayed or unexpectedly joined')
    selected = []
    for task in ['first', 'second']:
        reference = started[task]['worker']
        matches = [record for record in records
                   if record.get('task') == task and record.get('process') == reference
                   and record.get('parent_pid') == parent_pid]
        require(len(matches) == 1, 'Observation must bind one exact owned task process: ' + task)
        selected.append(matches[0])
    return selected


def fixture_sources(workspace, mode, token):
    source = workspace / 'workflows/lifecycle-proof'
    source.mkdir(parents=True)
    (source / 'worker.py').write_text(WORKER)
    for task in ['first', 'second', 'join']:
        (source / (task + '.sh')).write_text(
            'set -eu\nexec /usr/bin/python3 "$DOCKPIPE_SCRIPT_DIR/worker.py" ' + task + '\n')
    (source / 'config.yml').write_text(
        'name: lifecycle-proof\nvault: none\ndocker_preflight: false\nvars:\n'
        '  LIFECYCLE_TOKEN: ' + token + '\n  LIFECYCLE_MODE: ' + mode + '\nsteps:\n'
        '  - group:\n      mode: async\n      tasks:\n'
        '        - id: first\n          kind: host\n          cwd: repo\n          run: first.sh\n          outputs: first.env\n'
        '        - id: second\n          kind: host\n          cwd: repo\n          run: second.sh\n          outputs: second.env\n'
        '  - id: join\n    kind: host\n    cwd: repo\n    run: join.sh\n')
    return source


def owned_references(started):
    return [record[kind] for record in started.values() for kind in ['worker', 'child']]


def stop_fixture_group(process, original, references):
    """Fallback cleanup only; using it always prevents cancellation coverage."""
    witnesses = []
    for reference in [original, *references]:
        if not alive(reference):
            continue
        try:
            if os.getpgid(reference['pid']) == process.pid:
                witnesses.append(reference)
        except ProcessLookupError:
            continue
    if not witnesses:
        return
    try:
        os.killpg(process.pid, signal.SIGTERM)
    except ProcessLookupError:
        return
    deadline = time.monotonic() + 5
    while time.monotonic() < deadline and any(alive(reference) for reference in witnesses):
        process.poll()
        time.sleep(0.03)
    if any(alive(reference) for reference in witnesses):
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        pass


def exercise(cli, output, base_environment, mode, explicit_workdir=False, observe_resources=False):
    output.mkdir(mode=0o700)
    workspace = output / 'workspace'
    workspace.mkdir(mode=0o700)
    (workspace / 'dockpipe.config.json').write_text('{}\n')
    canary = output / 'outside-canary'
    canary.write_text('unrelated fixture remains unchanged')
    canary_sha = digest_file(canary)
    token = secrets.token_hex(16)
    source = fixture_sources(workspace, mode, token)
    environment = dict(base_environment, DOCKPIPE_EVENT_LOG=str(output / 'events.jsonl'),
                       DOCKPIPE_EVENT_INDEX=str(output / 'events-index.json'))
    command = [str(cli), '--workflow-file', str(source / 'config.yml')]
    if explicit_workdir:
        environment['DOCKPIPE_WORKDIR'] = str(workspace)
        command[1:1] = ['--workdir', str(workspace)]
    stdout_path = output / 'workflow.stdout'
    stderr_path = output / 'workflow.stderr'
    started = {}
    facts = {'mode': mode, 'token': token, 'explicit_workdir_binding': explicit_workdir,
             'supervisor_cleanup_used': False, 'error': ''}
    if mode == 'happy' and observe_resources:
        disk = os.statvfs('/')
        facts['independent_system'] = {'logical_cpus': os.cpu_count(),
            'memory_total': os.sysconf('SC_PHYS_PAGES') * os.sysconf('SC_PAGE_SIZE'),
            'disk_total': disk.f_blocks * disk.f_frsize}
    interrupted = False
    with stdout_path.open('xb') as stdout, stderr_path.open('xb') as stderr:
        stdout_path.chmod(0o600)
        stderr_path.chmod(0o600)
        process = subprocess.Popen(command,
            cwd=workspace, env=environment, stdin=subprocess.DEVNULL, stdout=stdout,
            stderr=stderr, start_new_session=True)
        original = identity(process.pid)
        require(original is not None, 'Started workflow process identity is unavailable')
        facts['cli_process'] = original
        write_json(output / 'workflow-started.json', {'cli_process': original})

        def check_logs():
            require(stdout_path.stat().st_size <= MAX_LOG_BYTES and stderr_path.stat().st_size <= MAX_LOG_BYTES,
                    'Workflow logs exceeded their per-stream bound')

        def read_started():
            check_logs()
            for task in ['first', 'second']:
                path = workspace / ('started-' + task + '.json')
                if path.exists():
                    record = read_json(path)
                    require(record.get('token') == token and record.get('task') == task,
                            'Fixture process identity is not bound to this invocation')
                    if task in started and started[task] != record:
                        facts.setdefault('additional_starts', []).append(record)
                        raise RuntimeError('Owned task was replayed with another process identity')
                    started[task] = record
            return len(started) == 2

        try:
            if mode != 'failure':
                wait_for(read_started, 10, process)
                require(all(alive(reference) for reference in owned_references(started)),
                        'Async tasks were not live together')
                active = checked_command(cli, ['runs', 'observe', '--workdir', str(workspace), '--json'],
                    workspace, environment, output, 'observe-active')
                records = observed_members(active, started, process.pid)
                require(all(record['state'] == 'running' for record in records), 'Async tasks are not observed running')
                facts['observed_execution_ids'] = [record['id'] for record in records]
                if mode == 'happy' and observe_resources:
                    facts['resource_observations'] = []
                    for index in range(3):
                        beginning = time.monotonic()
                        name = 'observe-resources-' + str(index)
                        snapshot = checked_command(cli, ['runs', 'observe', '--workdir', str(workspace),
                            '--resources', '--json'], workspace, environment, output, name)
                        observed_members(snapshot, started, process.pid)
                        facts['resource_observations'].append({
                            'snapshot': snapshot, 'elapsed_seconds': time.monotonic() - beginning,
                            'received_unix_seconds': time.time(),
                            'response_bytes': (output / name / 'driver.stdout').stat().st_size})
                        if index < 2:
                            time.sleep(0.1)
                if mode == 'cancellation':
                    process.send_signal(signal.SIGTERM)
                    facts['stop_requested_to_cli_only'] = True
                elif mode == 'restart-recovery':
                    process.send_signal(signal.SIGKILL)
                    require(process.wait(timeout=5) == -signal.SIGKILL, 'Original workflow owner did not crash as requested')
                    require(not alive(original), 'Original workflow owner remains live')
                    recovered = checked_command(cli, ['runs', 'observe', '--workdir', str(workspace), '--json'],
                        workspace, environment, output, 'observe-after-owner-crash')
                    recovered_records = observed_members(recovered, started, process.pid)
                    facts['original_identity_retained'] = (
                        [record['id'] for record in recovered_records] == facts['observed_execution_ids'])
                else:
                    (workspace / 'release').write_text(token)
            def exited():
                check_logs()
                return process.poll() is not None

            wait_for(exited, 15)
            read_started()
            require(len(started) == 2, 'Both owned task identities must be available')
            facts['exit_code'] = process.returncode
            check_logs()
            wait_for(lambda: not any(alive(reference) for reference in owned_references(started)), 5)
            final = checked_command(cli, ['runs', 'observe', '--workdir', str(workspace), '--json'],
                workspace, environment, output, 'observe-final')
            records = observed_members(final, started, process.pid, joined=mode == 'happy')
            facts['terminal_records'] = records
            facts['owned_processes_closed'] = True
            facts['outside_canary_unchanged'] = digest_file(canary) == canary_sha
            facts['no_join_result'] = not (workspace / 'result.json').exists()
            if mode == 'cancellation':
                facts['cancel_acknowledged'] = all((workspace / ('cancelled-' + task)).is_file()
                    and (workspace / ('cancelled-' + task)).read_text() == token for task in ['first', 'second'])
                time.sleep(0.2)
                repeated = checked_command(cli, ['runs', 'observe', '--workdir', str(workspace), '--json'],
                    workspace, environment, output, 'observe-after-cancel')
                again = observed_members(repeated, started, process.pid)
                facts['no_implicit_replay'] = ({record['id'] for record in records} == {record['id'] for record in again}
                    and all(record['state'] != 'running' for record in again)
                    and not (workspace / 'result.json').exists()
                    and all(read_json(workspace / ('started-' + task + '.json')) == started[task]
                            for task in ['first', 'second'])
                    and not any(alive(reference) for reference in owned_references(started)))
            elif mode == 'restart-recovery':
                repeated = checked_command(cli, ['runs', 'observe', '--workdir', str(workspace), '--json'],
                    workspace, environment, output, 'observe-after-recovery')
                recovered = observed_members(repeated, started, process.pid)
                facts['state_reconciled'] = (
                    [record['id'] for record in records] == facts['observed_execution_ids']
                    and recovered == records
                    and all(record['state'] == 'unverified' and record.get('reason') in {
                        'process_exited_or_inaccessible', 'process_identity_unavailable_or_changed'}
                        and record.get('exit_code') is None for record in records))
                facts['no_duplicate_effects'] = (
                    facts['no_join_result'] and facts['outside_canary_unchanged']
                    and all((workspace / ('starts-' + task)).read_text().splitlines() == [token]
                            and (workspace / ('cancelled-' + task)).is_file()
                            and (workspace / ('cancelled-' + task)).read_text() == token
                            and not (workspace / ('finished-' + task)).exists()
                            for task in ['first', 'second']))
            elif mode == 'happy':
                result = read_json(workspace / 'result.json')
                facts['result_bound'] = result == {'token': token, 'joined': ['first', 'second'], 'last_branch': 'second'}
                facts['stdout_bound'] = 'joined:' + token in stdout_path.read_text(errors='replace')
                events = checked_command(cli, ['runs', 'events', '--event-log', str(output / 'events.jsonl'), '--json'],
                    workspace, environment, output, 'events-public')
                facts['events_bound'] = bool(events) and all(record.get('log') == str(output / 'events.jsonl') for record in records)
        except (OSError, RuntimeError, ValueError, TimeoutError, subprocess.SubprocessError) as error:
            facts['error'] = str(error)
        except KeyboardInterrupt:
            interrupted = True
            facts['error'] = 'Lifecycle driver interrupted; no successful cancellation observation claimed'
        finally:
            references = owned_references(started)
            for record in facts.get('additional_starts', []):
                references.extend([record['worker'], record['child']])
            if process.poll() is None or any(alive(reference) for reference in references):
                facts['supervisor_cleanup_used'] = True
                stop_fixture_group(process, original, references)
            facts['remaining_owned_processes'] = [reference for reference in [original, *references]
                                                  if alive(reference)]
    facts['started'] = started
    with stderr_path.open('rb') as stream:
        facts['stderr'] = stream.read(4096).decode('utf-8', errors='replace')
    write_json(output / 'observations.json', facts)
    if interrupted:
        raise KeyboardInterrupt
    return facts


def scenario_assertions(scenario, facts):
    terminal = facts.get('terminal_records', [])
    if scenario == 'restart-recovery':
        crash_observed = (facts.get('exit_code') == -signal.SIGKILL and facts.get('error') == ''
                          and facts.get('supervisor_cleanup_used') is False
                          and facts.get('owned_processes_closed') is True
                          and facts.get('remaining_owned_processes') == [])
        return {'original-identity-retained': crash_observed and facts.get('original_identity_retained') is True,
                'state-reconciled': crash_observed and facts.get('state_reconciled') is True,
                'no-duplicate-effects': crash_observed and facts.get('no_duplicate_effects') is True}
    closed = (facts.get('owned_processes_closed') is True and facts.get('remaining_owned_processes') == []
              and facts.get('supervisor_cleanup_used') is False and facts.get('error') == ''
              and len(terminal) == 2 and all(record.get('state') in {'succeeded', 'failed', 'terminated', 'cancelled'}
                                            for record in terminal))
    expected_exit = {'happy': 0, 'failure': 7, 'cancellation': 143}[scenario]
    terminal_bound = len(terminal) == 2 and all(type(record.get('exit_code')) is int
                                              and record['exit_code'] == expected_exit for record in terminal)
    if scenario == 'happy':
        return {'detached-run-observed': False,
                'logs-and-result-bound': closed and terminal_bound and facts.get('exit_code') == 0
                    and facts.get('result_bound') is True and facts.get('stdout_bound') is True
                    and facts.get('events_bound') is True,
                'owned-process-tree-closed': closed}
    if scenario == 'cancellation':
        observed = (closed and terminal_bound and facts.get('exit_code') == 1
                    and facts.get('outside_canary_unchanged') is True and facts.get('stop_requested_to_cli_only') is True
                    and facts.get('cancel_acknowledged') is True and facts.get('no_join_result') is True)
        return {'cancellation-observed': observed, 'owned-work-stopped': closed,
                'no-implicit-replay': observed and facts.get('no_implicit_replay') is True}
    expected_failure = closed and terminal_bound and facts.get('exit_code') == 1 and 'exit status 7' in facts.get('stderr', '')
    return {'expected-failure-observed': expected_failure,
            'no-unowned-effects': closed and facts.get('outside_canary_unchanged') is True,
            'actionable-result': expected_failure}


def case_status(scenario, observed):
    if all(observed.values()):
        return 'passed'
    if scenario == 'happy' and all(value for name, value in observed.items() if name != 'detached-run-observed'):
        return 'blocked'
    return 'failed'


def captured_timestamp(value):
    if not isinstance(value, str):
        return None
    match = re.fullmatch(r'(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,9}))?Z', value)
    if match is None:
        return None
    fraction = (match[2] or '').ljust(6, '0')[:6]
    try:
        return datetime.fromisoformat(match[1] + '.' + fraction + '+00:00').timestamp()
    except ValueError:
        return None


def observability_assertions(facts):
    failed = {name: False for name in OBSERVABILITY_ASSERTIONS}
    lifecycle = scenario_assertions('happy', facts)
    if not lifecycle['logs-and-result-bound'] or not lifecycle['owned-process-tree-closed']:
        return failed
    samples = facts.get('resource_observations', [])
    if not isinstance(samples, list) or len(samples) != 3:
        return failed
    expected_boot = facts['started']['first']['worker']['start_token'].split(':', 1)[0]
    captures = []
    uptimes = []
    cpu_totals = []
    for sample in samples:
        if not isinstance(sample, dict) or not isinstance(sample.get('snapshot'), dict):
            return failed
        snapshot = sample['snapshot']
        system = snapshot.get('system', {})
        if not isinstance(system, dict):
            return failed
        if snapshot.get('machine') != 'local' or system.get('boot_id') != expected_boot:
            return failed
        if snapshot.get('truncated') is not False or snapshot.get('processes') != []:
            return failed
        try:
            records = observed_members(snapshot, facts['started'], facts['cli_process']['pid'])
        except (KeyError, ValueError):
            return failed
        for record in records:
            metrics = record.get('metrics', {})
            if not isinstance(metrics, dict):
                return failed
            if (record.get('state') != 'running' or record.get('machine') != 'local'
                    or metrics.get('pid') != record['process']['pid']
                    or metrics.get('start_token') != record['process']['start_token']
                    or metrics.get('machine') != 'local'
                    or type(metrics.get('memory_bytes')) is not int or metrics['memory_bytes'] <= 0
                    or type(metrics.get('cpu_seconds')) not in (int, float) or not metrics['cpu_seconds'] >= 0):
                return failed
        cpus = system.get('logical_cpus')
        total = system.get('memory_total')
        available = system.get('memory_available')
        uptime = system.get('uptime_seconds')
        cpu_total = system.get('cpu_total')
        if (type(cpus) is not int or cpus <= 0 or type(total) is not int or total <= 0
                or type(available) is not int or not 0 <= available <= total
                or type(uptime) not in (int, float) or not uptime >= 0
                or type(cpu_total) is not int or cpu_total < 0):
            return failed
        if any(system.get(name) != value for name, value in facts.get('independent_system', {}).items()):
            return failed
        if set(facts.get('independent_system', {})) != {'logical_cpus', 'memory_total', 'disk_total'}:
            return failed
        elapsed = sample.get('elapsed_seconds')
        size = sample.get('response_bytes')
        captured = captured_timestamp(snapshot.get('captured_at'))
        received = sample.get('received_unix_seconds')
        if type(elapsed) not in (int, float) or not 0 <= elapsed <= 5:
            return failed
        if type(size) is not int or not 0 < size <= 1024 * 1024:
            return failed
        if captured is None or type(received) not in (int, float) or not -0.5 <= received - captured <= 5:
            return failed
        captures.append(captured)
        uptimes.append(uptime)
        cpu_totals.append(cpu_total)
    fresh = (all(later > earlier for earlier, later in zip(captures, captures[1:]))
             and all(later > earlier for earlier, later in zip(uptimes, uptimes[1:]))
             and all(later > earlier for earlier, later in zip(cpu_totals, cpu_totals[1:])))
    return {'selected-environment-identified': True, 'system-and-run-state': True, 'bounded-refresh': fresh}


def run(request, output):
    require(platform.system() == 'Linux' and request['target']['platform'] == 'linux-portable'
            and request['target']['kind'] == 'local', 'This lane requires a local Linux portable target')
    require({'artifact'} <= set(request['configuration']) <= {'artifact', 'explicit_workdir'},
            'Lifecycle configuration requires artifact and optional explicit_workdir')
    explicit_workdir = request['configuration'].get('explicit_workdir', False)
    require(type(explicit_workdir) is bool, 'explicit_workdir must be a boolean')
    artifact = Path(request['configuration']['artifact'])
    require(artifact.is_absolute() and digest_file(artifact) == request['target']['artifact_sha256'],
            'Lifecycle artifact differs from selected pin')
    output.mkdir(mode=0o700, parents=True)
    installation = output / 'install'
    installation.mkdir(mode=0o700)
    cli = installation / 'dockpipe'
    shutil.copyfile(artifact, cli)
    cli.chmod(0o700)
    require(digest_file(cli) == request['target']['artifact_sha256'], 'Installed lifecycle CLI identity changed')
    home = output / 'home'
    home.mkdir(mode=0o700)
    environment = {'PATH': '/usr/local/bin:/usr/bin:/bin', 'HOME': str(home), 'LANG': 'C.UTF-8', 'LC_ALL': 'C.UTF-8'}
    for name in ['state', 'config', 'cache', 'data']:
        directory = home / name
        directory.mkdir(mode=0o700)
        environment['XDG_' + name.upper() + '_HOME'] = str(directory)
    rows = []
    uncertain = False
    scenarios = {}
    observe_resources = any(case['id'] == 'observability' and case['scenario'] == 'happy'
                            and case['layer'] == 'installed_e2e' for case in request['cases'])
    for case in request['cases']:
        row = {key: case[key] for key in ['id', 'scenario', 'layer']}
        row.update(status='blocked', reason='Host-only lifecycle lane does not implement this case', assertions=[], evidence=[])
        key = case['scenario'], case['layer']
        lifecycle_case = case['id'] == 'workflow.lifecycle' and key in ASSERTIONS
        observability_case = case['id'] == 'observability' and key == ('happy', 'installed_e2e')
        if not uncertain and (lifecycle_case or observability_case):
            expected = ASSERTIONS[key] if lifecycle_case else OBSERVABILITY_ASSERTIONS
            require(set(case['required_assertions']) == expected, 'Lifecycle/observation assertion contract changed')
            scenario = case['scenario']
            scenario_output = output / scenario
            if scenario not in scenarios:
                scenarios[scenario] = exercise(cli, scenario_output, environment, scenario,
                                               explicit_workdir, observe_resources)
            facts = scenarios[scenario]
            if lifecycle_case:
                observed = scenario_assertions(scenario, facts)
                row['status'] = case_status(scenario, observed)
            else:
                observed = observability_assertions(facts)
                row['status'] = 'passed' if all(observed.values()) else 'failed'
            row['reason'] = 'Actual async host workflow, public observation/logs and exact owned-process read-back'
            if scenario == 'restart-recovery':
                row['reason'] = 'Fresh public observers retain and reconcile the original workers after the workflow owner crashes'
            if observability_case:
                row['reason'] = ('Three bounded project-scoped resource refreshes bind exact fixture processes, '
                                 'kernel boot identity, independent CPU/memory/disk totals and advancing counters')
            if facts.get('error'):
                row['reason'] = ('Host lifecycle observation failed: ' + facts['error'])[:4096]
            elif row['status'] == 'blocked':
                row['reason'] = 'Host async/log/observe/closure observed; product detach is container-only and not exercised'
            row['assertions'] = [{'id': name, 'status': 'passed' if observed[name] else 'failed',
                                  'detail': row['reason']} for name in case['required_assertions']]
            paths = [path for path in scenario_output.rglob('*') if path.is_file() and not path.is_symlink()]
            require(len(paths) <= 64, 'Lifecycle evidence count exceeds contract')
            row['evidence'] = [{'path': path.relative_to(output).as_posix(), 'sha256': digest_file(path)} for path in paths]
            uncertain = bool(facts.get('remaining_owned_processes'))
        rows.append(row)
    receipt = {key: request[key] for key in ['run_id', 'plan_sha256', 'registry_sha256', 'target']}
    receipt.update(schema='dockpipe.product-integration.lane/v1', cases=rows,
                   target_role='local-installed-cli', product_detach_exercised=False)
    validate_receipt(receipt, request, output)
    write_json(output / 'result.json', receipt)
    return 2 if uncertain else 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--request', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    arguments = parser.parse_args()
    require(arguments.output.is_absolute() and not arguments.output.exists(), 'Select fresh absolute lifecycle output')
    request = read_json(arguments.request)
    require(request.get('schema') == 'dockpipe.product-integration.request/v1', 'Unknown lifecycle request schema')
    raise SystemExit(run(request, arguments.output))


def interrupt(_number, _frame):
    raise KeyboardInterrupt


if __name__ == '__main__':
    signal.signal(signal.SIGTERM, interrupt)
    main()
