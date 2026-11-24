"""Owned runtime-local browser fixtures and independent installed-GUI readbacks."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import time
import uuid

SOURCE = Path(__file__).with_name('browser-fixture.py')
SPEC = importlib.util.spec_from_file_location('installed_browser_http_fixture', SOURCE)
HTTP = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(HTTP)


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def write(path, value):
    with Path(path).open('x') as target:
        json.dump(value, target, indent=2)
        target.write('\n')
    Path(path).chmod(0o600)


def prepare(output):
    nonce = uuid.uuid4().hex
    configuration = {'schema': HTTP.SCHEMA, 'nonce': nonce}
    write(output / 'browser-fixture.json', configuration)
    shutil.copyfile(SOURCE, output / 'browser-fixture.py')
    (output / 'browser-fixture.py').chmod(0o600)
    wrapper = output / 'browser-start.sh'
    with wrapper.open('x') as target:
        target.write('#!/usr/bin/env bash\nset -euo pipefail\n'
                     'output=$1\nlaunch=$2\n'
                     '/usr/bin/python3 -I "$output/browser-fixture.py" --output "$output" --launch "$launch" '
                     '>"$output/browser-server-$launch.log" 2>&1 &\n'
                     'exec /app/bin/dockpipe-launcher --allow-second-instance\n')
    wrapper.chmod(0o600)
    return configuration


def launch_command(command, output, number):
    command = list(command)
    position = command.index('--command=dockpipe-launcher')
    command[position] = '--command=/usr/bin/env'
    # Keep the unchanged public app entrypoint as the fixture shell's exec target.
    command[-1:] = ['-u', 'BASH_ENV', '-u', 'ENV', '/usr/bin/bash', '--noprofile', '--norc',
                    str(output / 'browser-start.sh'), str(output), str(number)]
    if '--unshare=network' not in command or '--share=network' in command:
        raise ValueError('The browser fixture requires an isolated runtime-local network namespace')
    return command


def metadata(output, launch):
    path = output / ('browser-server-' + str(launch) + '.json')
    if not path.exists():
        return None
    if path.is_symlink() or path.stat().st_size > 4096:
        raise ValueError('Invalid bounded runtime-local server metadata')
    value = json.loads(path.read_text())
    expected = {'schema', 'nonce', 'launch', 'port', 'namespace_pid', 'payload_sha256'}
    selected = json.loads((output / 'browser-fixture.json').read_text())
    if (set(value) != expected or value['schema'] != HTTP.SCHEMA or value['nonce'] != selected['nonce']
            or value['launch'] != launch or not isinstance(value['port'], int) or not 1 <= value['port'] <= 65535
            or not isinstance(value['namespace_pid'], int) or not 1 <= value['namespace_pid'] <= 65535
            or value['payload_sha256'] != hashlib.sha256(HTTP.payload(value['nonce'])).hexdigest()):
        raise ValueError('Runtime-local fixture identity differs from its generated selection')
    return value


def urls(value):
    base = 'http://127.0.0.1:' + str(value['port']) + '/' + value['nonce'] + '/'
    return [base + 'a', base + 'b']


def admit_server(output, value, gui, owner_pid, python_sha256):
    pending = [gui['host_pid']]
    seen = set()
    matches = []
    while pending:
        if len(seen) >= 512:
            raise RuntimeError('Owned browser fixture process tree exceeds its bound')
        pid = pending.pop(0)
        if pid in seen:
            continue
        seen.add(pid)
        root = Path('/proc') / str(pid)
        try:
            if os.getpgid(pid) != owner_pid:
                continue
            status = (root / 'status').read_text()
            namespace = next((line.split()[1:] for line in status.splitlines() if line.startswith('NSpid:')), [])
            arguments = (root / 'cmdline').read_bytes().split(b'\0')
            expected = ['/usr/bin/python3', '-I', str(output / 'browser-fixture.py'), '--output', str(output),
                        '--launch', str(value['launch'])]
            if namespace and int(namespace[-1]) == value['namespace_pid']:
                if [item.decode() for item in arguments if item] != expected or digest(root / 'exe') != python_sha256:
                    raise RuntimeError('The owned browser HTTP process executable or arguments changed')
                start = (root / 'stat').read_text().rsplit(')', 1)[1].split()[19]
                namespaces = {name: os.readlink(root / 'ns' / name) for name in ['pid', 'net']}
                for name, identity in namespaces.items():
                    if identity != os.readlink(Path('/proc') / str(gui['host_pid']) / 'ns' / name):
                        raise RuntimeError('Browser fixture does not share the admitted app namespaces')
                    if identity == os.readlink(Path('/proc/self/ns') / name):
                        raise RuntimeError('Browser fixture escaped to a host namespace')
                matches.append({'host_pid': pid, 'namespace_pid': value['namespace_pid'],
                                'kernel_starttime': start, 'executable_sha256': python_sha256,
                                'namespaces': namespaces, 'app_commit': gui['commit'], 'port': value['port'],
                                'nonce': value['nonce'], 'launch': value['launch']})
            for task in (root / 'task').glob('*/children'):
                pending.extend(int(item) for item in task.read_text().split())
        except (FileNotFoundError, ProcessLookupError):
            continue
    if len(matches) != 1 or digest(output / 'browser-fixture.py') != digest(SOURCE):
        raise RuntimeError('Expected one exact runtime-local browser HTTP fixture')
    write(output / ('browser-server-' + str(value['launch']) + '-kernel-journal.json'), matches[0])
    return matches[0]


def record_children(output, gui):
    path = output / 'browser-owned-children.json'
    records = json.loads(path.read_text()) if path.exists() else {}
    pending = [gui['host_pid']]
    seen = set()
    while pending:
        if len(seen) >= 512 or len(records) >= 1024:
            raise RuntimeError('Owned browser child journal exceeded its process bound')
        pid = pending.pop(0)
        if pid in seen:
            continue
        seen.add(pid)
        root = Path('/proc') / str(pid)
        try:
            start = (root / 'stat').read_text().rsplit(')', 1)[1].split()[19]
            record = {'host_pid': pid, 'kernel_starttime': start, 'gui_ancestor': gui['host_pid']}
            try:
                record['executable'] = os.readlink(root / 'exe')
                record['executable_sha256'] = digest(root / 'exe')
            except PermissionError:
                record['sandbox_executable_unreadable'] = True
            records[str(pid) + ':' + start] = record
            for task in (root / 'task').glob('*/children'):
                pending.extend(int(item) for item in task.read_text().split())
        except (FileNotFoundError, ProcessLookupError):
            continue
    temporary = output / 'browser-owned-children.next'
    temporary.write_text(json.dumps(records, indent=2) + '\n')
    temporary.chmod(0o600)
    temporary.replace(path)


def require_closed(output):
    records = json.loads((output / 'browser-owned-children.json').read_text())
    deadline = time.monotonic() + 5
    while True:
        remaining = []
        for value in records.values():
            try:
                fields = (Path('/proc') / str(value['host_pid']) / 'stat').read_text().rsplit(')', 1)[1].split()
                if fields[19] == value['kernel_starttime']:
                    remaining.append(value['host_pid'])
            except FileNotFoundError:
                pass
        if not remaining:
            return
        if time.monotonic() >= deadline:
            raise RuntimeError('Original owned browser child cleanup remains unverified: ' + str(remaining))
        time.sleep(0.1)


def request_stop(output, launch):
    configuration = json.loads((output / 'browser-fixture.json').read_text())
    value = {'schema': HTTP.SCHEMA, 'nonce': configuration['nonce'], 'launch': launch, 'command': 'stop'}
    path = output / ('browser-stop-' + str(launch) + '.json')
    if path.exists():
        if path.is_symlink() or json.loads(path.read_text()) != value:
            raise ValueError('The fixture stop request identity changed')
    else:
        write(path, value)


def events(output, launch):
    path = output / ('browser-server-' + str(launch) + '.jsonl')
    if path.stat().st_size > 1 << 20:
        raise RuntimeError('Browser fixture evidence exceeded its log bound')
    lines = path.read_text().splitlines()
    if len(lines) > HTTP.MAX_EVENTS:
        raise RuntimeError('Browser fixture exceeded its event bound')
    return [json.loads(line) for line in lines]


def evaluate(output, home, settings, first_settings, observations):
    first = events(output, 1)
    second = events(output, 2)
    selected = metadata(output, 1)
    reopened = metadata(output, 2)
    def saw(phase, role, text, state=None):
        return any(record['phase'] == phase and any(item['role'] == role and item['text'] == text
                   and (state is None or item.get('state') == state) for item in record['items'])
                   for record in observations)
    def recorded(values, kind, page, present):
        return any(item.get('event') == kind and item.get('page') == page and item.get('present') is present
                   for item in values)
    browser = settings.get('browser', {})
    previous = first_settings.get('browser', {})
    directory = Path(browser.get('downloadDirectory', '/unselected-download-directory'))
    if not directory.is_absolute() or not directory.resolve().is_relative_to(home.resolve(strict=True)):
        raise RuntimeError('Browser download directory escaped the private generated HOME')
    filename = 'dockpipe-browser-' + selected['nonce'][:8] + '.txt'
    download = directory / filename
    downloaded = (download.is_file() and not download.is_symlink()
                  and digest(download) == selected['payload_sha256'])
    if downloaded:
        shutil.copyfile(download, output / 'browser-downloaded-fixture.txt')
        (output / 'browser-downloaded-fixture.txt').chmod(0o600)
    local_page = (recorded(first, 'dom', 'a', True) and recorded(first, 'dom', 'b', True)
                  and saw('first', 'browser-page', 'Dockpipe fixture A')
                  and saw('first', 'browser-page', 'Dockpipe fixture B'))
    tabs = (saw('first', 'browser-tab', 'Dockpipe fixture A')
            and saw('first', 'browser-tab', 'Dockpipe fixture B')
            and saw('first', 'browser-state', 'Generated DOM accepted ' + selected['nonce'][:8], 'retained after app navigation'))
    downloads = (downloaded and sum(item.get('route') == 'download.txt' for item in first) == 1
                 and saw('first', 'download-row', filename, 'Saved'))
    restart = (previous.get('askWhereToSave') is False and browser.get('askWhereToSave') is False
               and previous.get('downloadDirectory') == browser.get('downloadDirectory')
               and selected['port'] == reopened['port']
               and recorded(first, 'storage', 'a', False) and recorded(first, 'storage', 'b', True)
               and recorded(second, 'storage', 'a', False)
               and saw('reopen', 'browser-tab', 'New tab', 'sole empty tab')
               and saw('reopen', 'browser-state', 'Generated storage empty')
               and saw('reopen', 'checkbox', 'Ask where to save each file', 'unchecked'))
    result = {'local_page_loaded': local_page, 'tabs_and_downloads': tabs and downloads, 'profile_restart': restart,
              'temporary_browsing_session': True, 'persistent_browser_preferences': browser,
              'seeded_storage_observed_before_quit': recorded(first, 'storage', 'b', True),
              'seeded_storage_absent_after_restart': recorded(second, 'storage', 'a', False),
              'download_sha256': digest(download) if downloaded else None, 'download_path': str(download),
              'runtime_local_only': True, 'host_bridge_allowed': False}
    write(output / 'browser-independent-readback.json', result)
    return result
