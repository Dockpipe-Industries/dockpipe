"""Qualify public project Git actions against one test-owned generated repository."""
import argparse
import hashlib
import os
from pathlib import Path
import platform
import re
import shutil
import signal
import subprocess
import sys
import time
import uuid


OWNER = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(OWNER / 'assets/scripts'))
from contracts import digest_file, read_json, require, write_json
from process import MAX_LOG_BYTES, terminate_owned


STALE_DIAGNOSTIC = 'the project changed since review; refresh before continuing'
FIXTURE_SCHEMA = 'dockpipe.product-git-fixture/v1'


def starttime(pid):
    try:
        return (Path('/proc') / str(pid) / 'stat').read_text().rsplit(')', 1)[1].split()[19]
    except FileNotFoundError:
        return None


def observe_tree(pid, identity, pins, records):
    pending = [(pid, identity)]
    seen = set()
    while pending:
        require(len(seen) < 128, 'Owned Git fixture process tree exceeded 128 processes')
        selected, expected = pending.pop(0)
        if selected in seen:
            continue
        seen.add(selected)
        try:
            actual = starttime(selected)
            if actual is None or (expected is not None and actual != expected):
                continue
            process = Path('/proc') / str(selected)
            key = str(selected) + ':' + actual
            if key not in records:
                require(len(records) < 512, 'Owned Git fixture identities exceeded their bound')
                with (process / 'exe').open('rb') as stream:
                    executable = stream.read((256 << 20) + 1)
                require(len(executable) <= 256 << 20, 'Owned executable exceeded its bound')
                executable_sha256 = hashlib.sha256(executable).hexdigest()
                require(executable_sha256 in pins, 'Unexpected executable in generated Git fixture tree')
                records[key] = {'pid': selected, 'starttime': actual,
                                'executable_sha256': executable_sha256}
            children = process / 'task' / str(selected) / 'children'
            pending.extend((int(value), None) for value in children.read_text().split())
        except (FileNotFoundError, ProcessLookupError):
            continue


def wait_closed(records, timeout=5):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        remaining = [value for value in records.values() if starttime(value['pid']) == value['starttime']]
        if not remaining:
            return True
        time.sleep(0.02)
    return False


def valid_snapshot(response, root):
    return (isinstance(response, dict) and response.get('root') == str(root)
            and isinstance(response.get('head'), str)
            and re.fullmatch(r'(?:[0-9a-f]{40}|[0-9a-f]{64})', response['head']) is not None
            and isinstance(response.get('snapshot'), str)
            and re.fullmatch(r'[0-9a-f]{64}', response['snapshot']) is not None
            and isinstance(response.get('changes'), list) and len(response['changes']) <= 128)


def public_request(cli, root, request, environment, output, name, pins):
    logs = output / name
    logs.mkdir(mode=0o700)
    path = logs / 'request.json'
    write_json(path, dict(request, workdir=str(root)))
    command = [str(cli), 'git', '--request', str(path)]
    write_json(logs / 'command.json', command)
    records = {}
    failure = ''
    with (logs / 'driver.stdout').open('xb') as stdout, (logs / 'driver.stderr').open('xb') as stderr:
        process = subprocess.Popen(command, stdin=subprocess.DEVNULL, stdout=stdout, stderr=stderr,
                                   env=environment, cwd=root, start_new_session=True)
        identity = starttime(process.pid)
        try:
            deadline = time.monotonic() + 20
            while process.poll() is None:
                observe_tree(process.pid, identity, pins, records)
                if time.monotonic() >= deadline:
                    failure = 'Public Git command exceeded its 20 second fixture deadline'
                    break
                if stdout.tell() > MAX_LOG_BYTES or stderr.tell() > MAX_LOG_BYTES:
                    failure = 'Public Git command exceeded its output bound'
                    break
                time.sleep(0.01)
        finally:
            if process.poll() is None:
                terminate_owned(process)
            process.wait(timeout=5)
    for stream in ['driver.stdout', 'driver.stderr']:
        path = logs / stream
        path.chmod(0o600)
        require(path.stat().st_size <= MAX_LOG_BYTES, 'Public Git diagnostic exceeded its bound')
    closed = bool(records) and wait_closed(records)
    write_json(logs / 'kernel-journal.json', {'processes': list(records.values()), 'owned_processes_closed': closed})
    write_json(logs / 'process.json', {'exit_code': process.returncode, 'failure': failure})
    response = read_json(logs / 'driver.stdout') if process.returncode == 0 and not failure else {}
    if process.returncode == 0 and not failure:
        require(valid_snapshot(response, root), 'Public Git command returned a malformed fixture snapshot')
    return process.returncode == 0 and not failure and closed, response


def admit_ready(ready, nonce, broker, runtime):
    require(isinstance(ready, dict) and ready.get('schema') == FIXTURE_SCHEMA and ready.get('nonce') == nonce
            and ready.get('pid') == broker.pid, 'Generated Git fixture ready identity differs')
    root = Path(ready['root'])
    require(root.is_absolute() and not root.is_symlink() and root.is_dir()
            and root.resolve(strict=True).is_relative_to(runtime.resolve(strict=True)),
            'Generated Git fixture escaped its private test temporary directory')
    require(root.stat().st_uid == os.getuid() and root.stat().st_mode & 0o777 == 0o700,
            'Generated Git fixture root is not private')
    return root


def wait_ready(path, broker, timeout=15):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if path.exists():
            return read_json(path)
        require(broker.poll() is None, 'Generated Git fixture broker exited before readiness')
        time.sleep(0.02)
    raise RuntimeError('Generated Git fixture readiness timed out')


def worktree_bytes(root):
    return {name: (root / name).read_bytes() for name in
            ['selected.txt', 'unrelated.txt', 'draft.txt', 'untracked-draft.txt']}


def stale_rejected(output):
    process = read_json(output / 'stale-commit/process.json')
    diagnostic = (output / 'stale-commit/driver.stderr').read_text().strip().splitlines()
    return (type(process.get('exit_code')) is int and process['exit_code'] == 1
            and process.get('failure') == '' and bool(diagnostic) and diagnostic[-1] == STALE_DIAGNOSTIC)


def process_identity(pid):
    process = Path('/proc') / str(pid)
    started = starttime(pid)
    try:
        parent = next(int(line.split()[1]) for line in (process / 'status').read_text().splitlines()
                      if line.startswith('PPid:'))
        executable = (process / 'exe').resolve(strict=True)
        command = (process / 'cmdline').read_bytes().split(b'\0')
    except (FileNotFoundError, ProcessLookupError):
        return None
    if started is None or starttime(pid) != started:
        return None
    return {'pid': pid, 'starttime': started, 'parent': parent,
            'executable': str(executable), 'status_command': command[5:7] == [b'status', b'--porcelain=v1']}


def cancel_review(cli, git, root, environment, output):
    logs = output / 'cancel-review'
    logs.mkdir(mode=0o700)
    request = logs / 'request.json'
    write_json(request, {'action': 'review-status', 'workdir': str(root)})
    command = [str(cli), 'git', '--request', str(request)]
    write_json(logs / 'command.json', command)
    shell = Path('/bin/sh').resolve(strict=True)
    sleep = Path(shutil.which('sleep', path='/usr/bin:/bin')).resolve(strict=True)
    ready = logs / 'filter-started.txt'
    filter_script = logs / 'clean-filter.sh'
    filter_script.write_text(f'#!{shell}\n'
                             'printf "%s\\n" "$$" >> "$DOCKPIPE_GIT_FILTER_READY"\n'
                             f'exec {sleep} 12\n')
    filter_script.chmod(0o700)
    attributes = root / '.git/info/attributes'
    attributes.parent.mkdir(exist_ok=True)
    with attributes.open('x') as stream:
        stream.write('draft.txt filter=dockpipe-fixture\n')
    filtered_environment = dict(environment, GIT_CONFIG_COUNT='2',
                                GIT_CONFIG_KEY_0='filter.dockpipe-fixture.clean',
                                GIT_CONFIG_VALUE_0='exec "$DOCKPIPE_GIT_FILTER_SCRIPT"',
                                GIT_CONFIG_KEY_1='filter.dockpipe-fixture.required', GIT_CONFIG_VALUE_1='true',
                                DOCKPIPE_GIT_FILTER_READY=str(ready), DOCKPIPE_GIT_FILTER_SCRIPT=str(filter_script))
    records = {}
    signal_sent = False
    fallback = False
    try:
        with (logs / 'driver.stdout').open('xb') as stdout, (logs / 'driver.stderr').open('xb') as stderr:
            process = subprocess.Popen(command, stdin=subprocess.DEVNULL, stdout=stdout, stderr=stderr,
                                       env=filtered_environment, cwd=root, start_new_session=True)
            identity = starttime(process.pid)
            try:
                deadline = time.monotonic() + 5
                while process.poll() is None and time.monotonic() < deadline:
                    if ready.is_file():
                        require(ready.stat().st_size <= 256, 'Generated Git filter readiness exceeded its bound')
                        filter_pid = ready.read_text().splitlines()
                        if not filter_pid:
                            time.sleep(0.02)
                            continue
                        require(len(filter_pid) == 1 and re.fullmatch(r'[1-9][0-9]{0,7}', filter_pid[0]),
                                'Generated Git filter did not start exactly once')
                        worker = process_identity(int(filter_pid[0]))
                        if worker is not None and worker['executable'] == str(sleep):
                            child = process_identity(worker['parent'])
                            require(child is not None and child['parent'] == process.pid
                                    and child['executable'] == str(git.resolve(strict=True)) and child['status_command']
                                    and starttime(process.pid) == identity,
                                    'Waiting clean filter was not a descendant of the exact public Git request')
                            records = {record['pid']: record for record in (child, worker)}
                            records[process.pid] = {'pid': process.pid, 'starttime': identity, 'executable': str(cli)}
                            process.send_signal(signal.SIGTERM)
                            signal_sent = True
                            try:
                                process.wait(timeout=5)
                            except subprocess.TimeoutExpired:
                                fallback = True
                            break
                    time.sleep(0.02)
            finally:
                if process.poll() is None:
                    fallback = True
                    terminate_owned(process)
    finally:
        attributes.unlink()

    closed = bool(records) and wait_closed(records)
    diagnostic = logs / 'driver.stderr'
    require(all((logs / stream).stat().st_size <= MAX_LOG_BYTES for stream in ('driver.stdout', 'driver.stderr')),
            'Git cancellation output exceeded its bound')
    exact = diagnostic.read_text().splitlines()
    cancelled = signal_sent and not fallback and process.returncode == 1 and bool(exact) and exact[-1] == 'context canceled'
    starts = ready.read_text().splitlines() if ready.is_file() and ready.stat().st_size <= 256 else []
    facts = {'signal_sent': signal_sent, 'fallback_cleanup': fallback, 'exit_code': process.returncode,
             'cancelled': cancelled, 'owned_processes_closed': closed, 'filter_started_once': len(starts) == 1}
    write_json(logs / 'kernel-journal.json', {'processes': list(records.values()), **facts})
    return facts


def run(request, output):
    configuration = request['configuration']
    require(platform.system() == 'Linux' and request['target']['platform'] == 'linux-portable'
            and request['target']['kind'] == 'local', 'Git workspace lane currently qualifies only Linux portable')
    require(set(configuration) == {'artifact', 'fixture_manifest', 'fixture_manifest_sha256'},
            'Select exact installed CLI and owning fixture manifest')
    artifact = Path(configuration['artifact'])
    require(artifact.is_absolute() and digest_file(artifact) == request['target']['artifact_sha256'],
            'Selected Git workspace CLI differs from its pin')
    manifest_path = Path(configuration['fixture_manifest'])
    require(manifest_path.is_absolute() and digest_file(manifest_path) == configuration['fixture_manifest_sha256'],
            'Fixture producer manifest differs from its pin')
    manifest = read_json(manifest_path)
    require(manifest.get('schema') == 'dockpipe.product-git-fixture-build/v1'
            and manifest.get('source_overlay_only') is True and manifest.get('configuration_fault_tests_passed') is True,
            'Select an owning overlay-built fixture with passing configuration faults')
    binary = Path(manifest['binary'])
    git = Path(manifest['git'])
    require(binary.is_absolute() and digest_file(binary) == manifest['binary_sha256'], 'Fixture binary differs from its pin')
    require(git.is_absolute() and digest_file(git) == manifest['git_sha256'], 'Fixture Git differs from its pin')
    output.mkdir(mode=0o700, parents=True)
    cli = output / 'dockpipe'
    shutil.copyfile(artifact, cli)
    cli.chmod(0o700)
    home = output / 'home'
    runtime = output / 'runtime'
    control = output / 'control'
    tools = output / 'tools'
    templates = output / 'templates'
    for path in [home, runtime, control, tools, templates]:
        path.mkdir(mode=0o700)
    (tools / 'git').symlink_to(git)
    nonce = uuid.uuid4().hex
    environment = {'PATH': str(tools) + os.pathsep + '/usr/bin:/bin', 'HOME': str(home),
                   'TMPDIR': str(runtime), 'GIT_CONFIG_NOSYSTEM': '1', 'GIT_CONFIG_GLOBAL': '/dev/null',
                   'GIT_TEMPLATE_DIR': str(templates), 'GIT_TERMINAL_PROMPT': '0', 'GIT_ALLOW_PROTOCOL': 'file',
                   'DOCKPIPE_VAULT_INJECT': '0', 'DOCKPIPE_INTERACTIVE': '0',
                   'DOCKPIPE_PRODUCT_GIT_FIXTURE': str(control / 'configuration.json')}
    for name in ['config', 'cache', 'data', 'state']:
        path = home / name
        path.mkdir(mode=0o700)
        environment['XDG_' + name.upper() + '_HOME'] = str(path)
    write_json(control / 'configuration.json', {'schema': FIXTURE_SCHEMA, 'nonce': nonce,
        'control_directory': str(control), 'git': str(git), 'git_sha256': manifest['git_sha256'], 'lifetime_seconds': 90})
    broker_records = {}
    facts = {}
    with (output / 'broker.stdout').open('xb') as stdout, (output / 'broker.stderr').open('xb') as stderr:
        broker = subprocess.Popen([str(binary), '-test.run=^TestProductWorkspaceGitFixtureBroker$', '-test.timeout=110s'],
                                  env=environment, cwd=output, stdin=subprocess.DEVNULL,
                                  stdout=stdout, stderr=stderr, start_new_session=True)
        identity = starttime(broker.pid)
        try:
            observe_tree(broker.pid, identity, {manifest['binary_sha256'], manifest['git_sha256']}, broker_records)
            ready = wait_ready(control / 'ready.json', broker)
            root = admit_ready(ready, nonce, broker, runtime)
            require(ready['git_sha256'] == manifest['git_sha256'], 'Broker Git differs from selected Git')
            initial = worktree_bytes(root)
            pins = {request['target']['artifact_sha256'], manifest['git_sha256']}
            reviewed_ok, reviewed = public_request(cli, root, {'action': 'review-status'}, environment, output, 'review', pins)
            require(reviewed_ok and reviewed.get('head') == ready['initial_head'], 'Initial public Git review failed')
            compared_ok, compared = public_request(cli, root, {'action': 'compare', 'snapshot': reviewed['snapshot'],
                'paths': ['selected.txt'], 'comparison': 'unstaged'}, environment, output, 'compare-selected', pins)
            committed_ok, committed = public_request(cli, root, {'action': 'commit', 'snapshot': reviewed['snapshot'],
                'paths': ['selected.txt'], 'message': 'Commit generated selected fixture'}, environment, output, 'commit-selected', pins)
            require(committed_ok, 'Public selected-path commit failed')
            after_commit = worktree_bytes(root)
            unchanged = all(after_commit[name] == initial[name] for name in initial)
            stale_bytes = after_commit['draft.txt'] + b'changed after generated review\n'
            (root / 'draft.txt').write_bytes(stale_bytes)
            before_failure = worktree_bytes(root)
            stale_ok, _ = public_request(cli, root, {'action': 'commit', 'snapshot': committed['snapshot'],
                'paths': ['draft.txt'], 'message': 'Must refuse stale generated review'}, environment, output, 'stale-commit', pins)
            refreshed_ok, refreshed = public_request(cli, root, {'action': 'review-status'}, environment, output, 'refresh', pins)
            preserved = (refreshed_ok and before_failure == worktree_bytes(root)
                         and refreshed.get('head') == committed.get('head'))
            changes = {value['path']: value['status'] for value in committed.get('changes', [])}
            facts = {'reviewed_head': reviewed['head'], 'committed_head': committed.get('head'),
                'selected_review_patch': compared_ok and '+selected generated change' in compared.get('review', {}).get('patch', ''),
                'snapshot_refreshed': committed.get('snapshot') != reviewed.get('snapshot') and committed.get('head') != reviewed.get('head')
                    and refreshed_ok and refreshed.get('snapshot') != committed.get('snapshot'),
                'unrelated_worktree_preserved': unchanged and changes.get('unrelated.txt') == 'MM'
                    and changes.get('draft.txt') == ' M' and changes.get('untracked-draft.txt') == '??',
                'stale_rejected': not stale_ok and stale_rejected(output),
                'stale_head_and_content_unchanged': preserved, 'fixture_nonce': nonce}
            if any(case['scenario'] == 'cancellation' for case in request['cases']):
                draft = root / 'draft.txt'
                # A size change lets Git classify the path as dirty without reading through the filter.
                draft.write_bytes(b'draft.txt test\n')
                try:
                    before = worktree_bytes(root)
                    before_ok, before_review = public_request(cli, root, {'action': 'review-status'}, environment,
                                                              output, 'review-before-cancel', pins)
                    cancellation = cancel_review(cli, git, root, environment, output)
                    after_ok, after_review = public_request(cli, root, {'action': 'review-status'}, environment,
                                                            output, 'review-after-cancel', pins)
                    cancellation['state_preserved'] = (before_ok and after_ok and before_review == after_review
                                                       and before == worktree_bytes(root))
                    facts['cancellation'] = cancellation
                finally:
                    draft.write_bytes(before_failure['draft.txt'])
        finally:
            if not (control / 'stop.json').exists():
                write_json(control / 'stop.pending.json', {'nonce': nonce, 'command': 'stop'})
                (control / 'stop.pending.json').rename(control / 'stop.json')
            try:
                broker.wait(timeout=15)
            finally:
                if broker.poll() is None:
                    terminate_owned(broker)
            write_json(output / 'broker-kernel-journal.json', {
                'processes': list(broker_records.values()), 'exit_code': broker.returncode,
                'owned_processes_closed': bool(broker_records) and wait_closed(broker_records)})
    closed = read_json(control / 'closed.json')
    oracle = read_json(control / 'oracle.json')
    cleanup = broker.returncode == 0 and closed.get('nonce') == nonce and closed.get('repo_absent') is True
    cleanup = cleanup and not Path(closed['root']).exists() and wait_closed(broker_records)
    selected = (oracle.get('nonce') == nonce and oracle.get('committed_paths') == ['selected.txt']
                and oracle.get('selected_head') == 'selected generated change\n'
                and oracle.get('head') == facts['committed_head'])
    unrelated = (oracle.get('unrelated_head') == 'unrelated.txt base\n'
                 and oracle.get('unrelated_index') == 'unrelated staged fixture\n'
                 and oracle.get('staged_paths') == ['unrelated.txt'])
    facts.update(selected_path_only_in_head=selected, unrelated_index_preserved=unrelated,
                 fixture_repo_removed=cleanup, broker_binary_sha256=manifest['binary_sha256'],
                 git_sha256=manifest['git_sha256'], cli_sha256=digest_file(cli))
    require(digest_file(cli) == request['target']['artifact_sha256'] and digest_file(git) == manifest['git_sha256'],
            'Qualification tool changed during execution')
    require(digest_file(binary) == manifest['binary_sha256']
            and digest_file(manifest_path) == configuration['fixture_manifest_sha256'],
            'Fixture producer artifact changed during execution')
    write_json(output / 'observations.json', facts)
    paths = [manifest_path, output / 'observations.json', output / 'broker-kernel-journal.json',
             *sorted(control.glob('*.json')), *sorted(output.glob('*/request.json')),
             *sorted(output.glob('*/driver.stdout')), *sorted(output.glob('*/driver.stderr')),
             *sorted(output.glob('*/process.json')), *sorted(output.glob('*/kernel-journal.json')),
             *sorted(output.glob('*/command.json')), output / 'broker.stdout', output / 'broker.stderr']
    paths.extend(sorted((output / 'cancel-review').glob('*.sh')))
    paths.extend(sorted((output / 'cancel-review').glob('filter-started.txt')))
    shutil.copyfile(manifest_path, output / 'fixture-build-manifest.json')
    paths[0] = output / 'fixture-build-manifest.json'
    proof = [{'path': str(path.relative_to(output)), 'sha256': digest_file(path)} for path in paths]
    rows = []
    for case in request['cases']:
        assertions = None
        reason = 'Actual public CLI selected-file commit and stale-review refusal in a test-owned repository'
        if case['id'] == 'git.workspace' and case['scenario'] == 'happy' and case['layer'] == 'installed_e2e':
            assertions = {'selected-path-commit': selected and facts['selected_review_patch'] and cleanup,
                          'snapshot-refreshed': facts['snapshot_refreshed'] and cleanup,
                          'unrelated-work-preserved': unrelated and facts['unrelated_worktree_preserved'] and cleanup}
        elif case['id'] == 'git.workspace' and case['scenario'] == 'failure' and case['layer'] == 'component':
            assertions = {'expected-failure-observed': facts['stale_rejected'] and cleanup,
                          'no-unowned-effects': facts['stale_head_and_content_unchanged'] and cleanup,
                          'actionable-result': facts['stale_rejected'] and cleanup}
        elif case['id'] == 'git.workspace' and case['scenario'] == 'cancellation' and case['layer'] == 'component':
            cancellation = facts['cancellation']
            assertions = {'cancellation-observed': cancellation['cancelled'],
                          'owned-work-stopped': cancellation['owned_processes_closed'] and cleanup,
                          'no-implicit-replay': cancellation['filter_started_once'] and cancellation['state_preserved']}
            reason = 'Exact CLI SIGTERM while a real Git status waited on its generated clean filter'
        row = {name: case[name] for name in ['id', 'scenario', 'layer']}
        row.update(status='blocked', reason='No generated fixture is implemented for this Git workspace case',
                   assertions=[], evidence=proof)
        if assertions is not None:
            require(set(assertions) == set(case['required_assertions']), 'Git workspace assertion contract changed')
            row.update(status='passed' if all(assertions.values()) else 'failed',
                       reason=reason,
                       assertions=[{'id': name, 'status': 'passed' if value else 'failed',
                                    'detail': 'See public API responses, independent broker oracle and exact generated repository cleanup'}
                                   for name, value in assertions.items()])
        rows.append(row)
    receipt = {name: request[name] for name in ['run_id', 'plan_sha256', 'registry_sha256', 'target']}
    receipt.update(schema='dockpipe.product-integration.lane/v1', cases=rows, observed=facts)
    write_json(output / 'result.json', receipt)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--request', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    arguments = parser.parse_args()
    require(arguments.output.is_absolute() and not arguments.output.exists(), 'Select fresh absolute output')
    request = read_json(arguments.request)
    require(request.get('schema') == 'dockpipe.product-integration.request/v1', 'Unknown request schema')
    run(request, arguments.output)


if __name__ == '__main__':
    main()
