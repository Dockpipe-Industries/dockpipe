"""Observe only an exact child invocation and its contained CLI descendants."""
import configparser
import hashlib
import os
from pathlib import Path
import time


def starttime(pid):
    try:
        return (Path('/proc') / str(pid) / 'stat').read_text().rsplit(')', 1)[1].split()[19]
    except FileNotFoundError:
        return None


class InstalledCliProcessObserver:
    def __init__(self, command, expected_commit, expected_cli_sha256):
        self.command = command
        self.expected_commit = expected_commit
        self.expected_cli_sha256 = expected_cli_sha256
        self.root = None
        self.processes = {}
        self.admitted_cli = {}
        self.host_namespaces = {name: os.readlink('/proc/self/ns/' + name) for name in ['pid', 'net']}

    def observe(self):
        if self.root is None:
            children = Path('/proc/self/task') / str(os.getpid()) / 'children'
            for value in children.read_text().split()[:32]:
                pid = int(value)
                try:
                    arguments = (Path('/proc') / value / 'cmdline').read_bytes().rstrip(b'\0').split(b'\0')
                    expected = [os.fsencode(argument) for argument in self.command]
                    if arguments == expected and os.getpgid(pid) == pid:
                        self.root = (pid, starttime(pid))
                        break
                except (FileNotFoundError, ProcessLookupError):
                    continue
        if self.root is None:
            return
        pending = [self.root[0]]
        seen = set()
        while pending:
            if len(seen) >= 128:
                raise RuntimeError('Installed CLI process tree exceeded its 128-process bound')
            pid = pending.pop(0)
            if pid in seen:
                continue
            seen.add(pid)
            try:
                identity = starttime(pid)
                if identity is None or os.getpgid(pid) != self.root[0]:
                    continue
                if pid == self.root[0] and identity != self.root[1]:
                    continue
                process = Path('/proc') / str(pid)
                key = str(pid) + ':' + identity
                if key not in self.processes and len(self.processes) >= 128:
                    raise RuntimeError('Installed CLI observed identities exceeded their 128-process bound')
                self.processes[key] = {'pid': pid, 'starttime': identity}
                children = process / 'task' / str(pid) / 'children'
                pending.extend(int(value) for value in children.read_text().split())
                try:
                    executable = os.readlink(process / 'exe')
                except PermissionError:
                    continue
                if executable != '/app/libexec/dockpipe/dockpipe':
                    continue
                metadata = configparser.ConfigParser(interpolation=None)
                with (process / 'root/.flatpak-info').open() as stream:
                    contents = stream.read(65537)
                if len(contents) > 65536:
                    raise RuntimeError('Installed CLI namespace metadata exceeded its bound')
                metadata.read_string(contents)
                namespaces = {name: os.readlink(process / 'ns' / name) for name in ['pid', 'net']}
                with (process / 'exe').open('rb') as stream:
                    executable_bytes = stream.read((256 << 20) + 1)
                if len(executable_bytes) > 256 << 20:
                    raise RuntimeError('Installed CLI executable exceeded its bound')
                observed_sha256 = hashlib.sha256(executable_bytes).hexdigest()
                if (metadata.get('Application', 'name', fallback='') != 'com.dockpipe.Dockpipe'
                        or metadata.get('Instance', 'app-commit', fallback='') != self.expected_commit
                        or observed_sha256 != self.expected_cli_sha256
                        or any(namespaces[name] == self.host_namespaces[name] for name in namespaces)):
                    raise RuntimeError('Observed CLI differs from the selected contained app artifact')
                if starttime(pid) != identity:
                    continue
                self.admitted_cli[key] = {**self.processes[key], 'namespaces': namespaces,
                                          'app_commit': self.expected_commit,
                                          'executable_sha256': observed_sha256}
            except (FileNotFoundError, ProcessLookupError):
                continue

    def finish(self):
        deadline = time.monotonic() + 5
        remaining = []
        while time.monotonic() < deadline:
            remaining = [value for value in self.processes.values()
                         if starttime(value['pid']) == value['starttime']]
            if not remaining:
                break
            time.sleep(0.05)
        return {'command_child_observed': self.root is not None,
                'contained_cli_observed': bool(self.admitted_cli),
                'admitted_cli': list(self.admitted_cli.values()),
                'owned_processes': list(self.processes.values()),
                'remaining_original_processes': remaining,
                'owned_processes_closed': self.root is not None and not remaining}
