"""Independent deadline for only an admitted, newly owned installed GUI group.

This helper owns local process lifetime. Provider/Core session policy and remote
cleanup remain with the pinned owning workflow. No remote command runs here.
"""
from pathlib import Path
import argparse
import configparser
import hashlib
import json
import os
import re
import selectors
import signal
import stat
import sys
import time


MAXIMUM_TRACKED = 256


def require(condition, message):
    if not condition:
        raise ValueError(message)


def identity(pid):
    require(type(pid) is int and pid > 1, 'Expected a concrete owned process ID')
    root = Path('/proc') / str(pid)
    try:
        with (root / 'stat').open() as stream:
            data = stream.read(8193)
        require(len(data) <= 8192, 'Kernel process metadata exceeded its bound')
        fields = data.rsplit(')', 1)[1].split()
        return {'pid': pid, 'starttime': fields[19], 'parent': int(fields[1]),
                'group': int(fields[2]), 'session': int(fields[3]), 'uid': root.stat().st_uid}
    except (FileNotFoundError, ProcessLookupError):
        return None


def same_process(token):
    current = identity(token['pid'])
    return current is not None and current['starttime'] == token['starttime']


def sha256(path):
    result = hashlib.sha256()
    total = 0
    with Path(path).open('rb') as stream:
        for chunk in iter(lambda: stream.read(65536), b''):
            total += len(chunk)
            require(total <= 256 << 20, 'Owned executable exceeded artifact bound')
            result.update(chunk)
    return result.hexdigest()


class OwnedGroup:
    def __init__(self, wrapper, controller_pid):
        require(isinstance(wrapper, dict) and set(wrapper) == {'pid', 'starttime'},
                'Expected exact generated wrapper process token')
        current = identity(wrapper['pid'])
        require(current is not None and current['starttime'] == wrapper['starttime']
                and current['parent'] == controller_pid and current['uid'] == os.getuid()
                and current['group'] == current['pid'] and current['session'] == current['pid'],
                'Wrapper is not a newly owned child session of this controller')
        self.group = current['pid']
        self.tokens = {current['pid']: current}
        self.groups = {current['group']: current}
        self.observation_errors = []
        self.observe()

    def admit_current(self, current):
        require(current['uid'] == os.getuid(), 'Owned child changed user identity')
        if current['group'] not in self.groups:
            # The provider intentionally starts its local carrier with setsid().
            # Admit only a new group led by this already discovered child itself.
            sessions = {token['session'] for token in self.groups.values()}
            require(current['group'] == current['pid']
                    and (current['session'] in sessions or current['session'] == current['pid']),
                    'Owned child joined an unproved process group/session')
            self.groups[current['group']] = current
        require(current['session'] == self.groups[current['group']]['session'],
                'Owned group session identity changed')
        self.tokens[current['pid']] = current

    def observe(self):
        pending = list(self.tokens.values())
        visited = set()
        while pending:
            token = pending.pop(0)
            if token['pid'] in visited:
                continue
            current = identity(token['pid'])
            if current is None or current['starttime'] != token['starttime']:
                continue
            self.admit_current(current)
            visited.add(token['pid'])
            path = Path('/proc') / str(token['pid']) / 'task' / str(token['pid']) / 'children'
            try:
                data = path.read_text()
            except FileNotFoundError:
                continue
            require(len(data) <= 8192, 'Owned process child list exceeded bound')
            for value in data.split():
                child = identity(int(value))
                if child is None or child['parent'] != token['pid']:
                    continue
                if child['pid'] not in self.tokens:
                    require(len(self.tokens) < MAXIMUM_TRACKED, 'Owned GUI tree exceeded its process bound')
                else:
                    require(self.tokens[child['pid']]['starttime'] == child['starttime'],
                            'Observed child process ID was reused')
                self.admit_current(child)
                pending.append(child)

    def remaining(self):
        return [token for token in self.tokens.values() if same_process(token)]

    def signal(self, selected_signal, already_signalled=()):
        sent = set()
        for group, leader in self.groups.items():
            if group in already_signalled:
                continue
            survivors = []
            for token in self.tokens.values():
                current = identity(token['pid'])
                if current is not None and current['starttime'] == token['starttime'] and current['group'] == group:
                    require(current['session'] == leader['session'], 'Owned group changed session before signalling')
                    survivors.append(current)
            if not survivors:
                # Never signal a reused numeric group without an original live
                # kernel identity anchoring this task's ownership.
                continue
            try:
                os.killpg(group, selected_signal)
                sent.add(group)
            except ProcessLookupError:
                pass
        return sent

    def exists(self):
        for group in self.groups:
            try:
                os.killpg(group, 0)
                return True
            except ProcessLookupError:
                continue
        return False

    def observe_for_cleanup(self):
        try:
            self.observe()
        except ValueError as error:
            message = str(error)
            if message not in self.observation_errors and len(self.observation_errors) < 16:
                self.observation_errors.append(message)

    def stop(self, term_grace=2.0, kill_grace=1.0):
        self.observe_for_cleanup()
        for selected_signal, grace in [(signal.SIGTERM, term_grace), (signal.SIGKILL, kill_grace)]:
            signalled = set()
            until = time.monotonic() + grace
            while time.monotonic() < until:
                self.observe_for_cleanup()
                signalled.update(self.signal(selected_signal, signalled))
                if not self.exists():
                    return True
                time.sleep(0.02)
        return not self.exists()


def admit_gui(value, group):
    require(isinstance(value, dict) and set(value) == {'pid', 'starttime', 'commit', 'sha256'},
            'Expected exact immutable GUI process identity')
    require(all(isinstance(value[name], str) and re.fullmatch('[0-9a-f]{64}', value[name]) is not None
                for name in ['commit', 'sha256']), 'Immutable GUI commit/artifact pins must be exact digests')
    group.observe()
    token = group.tokens.get(value['pid'])
    require(token is not None and token['starttime'] == value['starttime'] and same_process(token),
            'Selected GUI is not in the newly owned process tree')
    process = Path('/proc') / str(value['pid'])
    metadata = configparser.ConfigParser(interpolation=None)
    with (process / 'root/.flatpak-info').open() as stream:
        raw = stream.read(65537)
    require(len(raw) <= 65536, 'Flatpak identity exceeded bound')
    metadata.read_string(raw)
    require(metadata.get('Application', 'name') == 'com.dockpipe.Dockpipe'
            and metadata.get('Instance', 'app-commit') == value['commit']
            and sha256(process / 'exe') == value['sha256'], 'Owned GUI differs from the selected installed artifact')


def write(path, value):
    with path.open('x') as stream:
        json.dump(value, stream, indent=2, allow_nan=False)
        stream.write('\n')
    path.chmod(0o600)


def run(request_path, output):
    require(sys.platform.startswith('linux'), 'This installed Flatpak watchdog requires Linux kernel identities')
    require(os.getsid(os.getpid()) == os.getpid() and os.getpgrp() == os.getpid(),
            'Watchdog must have its own independent session before capture is admitted')
    require(request_path.is_absolute() and not request_path.is_symlink()
            and stat.S_IMODE(request_path.stat().st_mode) == 0o600
            and request_path.stat().st_uid == os.getuid() and request_path.stat().st_size <= 8192,
            'Expected a new private generated watchdog request')
    request = json.loads(request_path.read_text())
    require(isinstance(request, dict) and set(request) == {'schema', 'run_id', 'pid_namespace', 'wrapper', 'gui'},
            'Unknown owned watchdog request')
    require(request['schema'] == 'dockpipe.product.owned-gui-watchdog/v1'
            and request['pid_namespace'] == os.readlink('/proc/self/ns/pid'), 'Watchdog must share the admitted host PID namespace')
    require(output.is_absolute() and not output.exists()
            and output.parent.resolve(strict=True) == request_path.parent.resolve(strict=True)
            and output.parent.stat().st_uid == os.getuid()
            and stat.S_IMODE(output.parent.stat().st_mode) == 0o700,
            'Fresh watchdog output must share the generated request private parent')
    output.mkdir(mode=0o700)
    group = OwnedGroup(request['wrapper'], os.getppid())
    admit_gui(request['gui'], group)
    write(output / 'admitted.json', {'run_id': request['run_id'], 'group': group.group,
                                    'tokens': list(group.tokens.values()), 'groups': list(group.groups.values()), 'gui': request['gui']})
    print(json.dumps({'schema': request['schema'], 'run_id': request['run_id'], 'stage': 'ready'}), flush=True)
    selector = selectors.DefaultSelector()
    selector.register(sys.stdin, selectors.EVENT_READ)
    cutoff = time.monotonic() + 30
    armed = False
    failure = ''
    previous_handlers = {}

    def interrupted(_number, _frame):
        raise KeyboardInterrupt('Owned watchdog interrupted; stopping its admitted GUI/carrier groups')

    for selected_signal in [signal.SIGTERM, signal.SIGHUP]:
        previous_handlers[selected_signal] = signal.signal(selected_signal, interrupted)
    reason = 'unarmed controller deadline'
    try:
        while time.monotonic() < cutoff:
            group.observe()
            temporary = output / 'latest.next'
            write(temporary, {'tokens': list(group.tokens.values()), 'groups': list(group.groups.values()),
                              'observed_monotonic_ns': time.monotonic_ns()})
            temporary.replace(output / 'latest.json')
            if not group.exists():
                reason = 'owned GUI group already absent'
                break
            if not selector.select(min(0.05, max(0, cutoff - time.monotonic()))):
                continue
            data = os.read(sys.stdin.fileno(), 4096)
            if not data:
                reason = 'controller pipe closed'
                break
            require(not armed and len(data) <= 4096 and data.endswith(b'\n') and data.count(b'\n') == 1,
                    'Watchdog admits one complete arm message; no deadline extensions')
            arm = json.loads(data)
            require(isinstance(arm, dict) and set(arm) == {'run_id', 'terminate_at_monotonic_ns'}
                    and arm['run_id'] == request['run_id'] and type(arm['terminate_at_monotonic_ns']) is int,
                    'Watchdog arm binding changed')
            selected = arm['terminate_at_monotonic_ns'] / 1_000_000_000
            require(time.monotonic() < selected <= time.monotonic() + 18,
                    'Watchdog cutoff must be independently bounded before capture')
            cutoff = selected
            armed = True
            reason = 'armed hard cutoff'
    except BaseException as error:
        failure = str(error)
        raise
    finally:
        selector.close()
        intervention = {'schema': request['schema'], 'run_id': request['run_id'],
                        'stage': 'stopping', 'reason': reason, 'monotonic_ns': time.monotonic_ns()}
        try:
            write(output / 'intervention.json', intervention)
            print(json.dumps(intervention), flush=True)
        finally:
            # A failed diagnostic write must never skip the owned hard cutoff.
            stopped = group.stop()
        for selected_signal, previous in previous_handlers.items():
            signal.signal(selected_signal, previous)
        write(output / 'result.json', {'schema': request['schema'], 'run_id': request['run_id'],
            'reason': reason, 'armed': armed, 'failure': failure, 'owned_gui_group_absent': stopped,
            'ownership_observation_errors': group.observation_errors,
            'remaining': group.remaining(), 'tokens': list(group.tokens.values()),
            'groups': list(group.groups.values()),
            'remote_cleanup_claimed': False})
    require(stopped, 'Owned GUI cleanup remains unconfirmed; preserve original resource identities')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--request', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    arguments = parser.parse_args()
    run(arguments.request, arguments.output)


if __name__ == '__main__':
    main()
