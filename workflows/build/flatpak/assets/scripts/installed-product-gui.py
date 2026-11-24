#!/usr/bin/env python3
"""Normal installed Flatpak GUI observations on an owned display and private buses."""
import hashlib
import configparser
import json
import os
from pathlib import Path
import signal
import stat
import shutil
import subprocess
import tempfile
import tarfile
import time

APP_ID = 'com.dockpipe.Dockpipe'


def create_private_runtime(prefix):
    """Keep Flatpak's Unix sockets short even under a deep inherited TMPDIR."""
    if prefix not in ['dockpipe-flatpak-runtime-', 'dockpipe-remote-runtime-']:
        raise ValueError('Select an existing owning Flatpak runtime scope')
    parent = Path('/tmp')
    parent_status = parent.lstat()
    if not stat.S_ISDIR(parent_status.st_mode) or parent.resolve(strict=True) != parent:
        raise ValueError('Private Flatpak runtime requires the canonical /tmp directory')
    directory = Path(tempfile.mkdtemp(prefix=prefix, dir=str(parent)))
    if directory.parent != parent or not directory.name.startswith(prefix) or directory.name == prefix:
        raise ValueError('New private Flatpak runtime escaped its expected /tmp scope: ' + str(directory))
    status = directory.lstat()
    if (not stat.S_ISDIR(status.st_mode) or status.st_uid != os.getuid()
            or stat.S_IMODE(status.st_mode) != 0o700 or directory.resolve(strict=True) != directory):
        raise ValueError('New Flatpak runtime is not an owner-private regular directory: ' + str(directory))
    descriptor = os.open(directory, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        opened = os.fstat(descriptor)
        if ((opened.st_dev, opened.st_ino) != (status.st_dev, status.st_ino)
                or opened.st_uid != os.getuid() or stat.S_IMODE(opened.st_mode) != 0o700):
            raise ValueError('New Flatpak runtime identity changed: ' + str(directory))
        with os.scandir(descriptor) as entries:
            if next(entries, None) is not None:
                raise ValueError('New Flatpak runtime was not fresh: ' + str(directory))
    finally:
        os.close(descriptor)
    current_parent = parent.lstat()
    if (current_parent.st_dev, current_parent.st_ino) != (parent_status.st_dev, parent_status.st_ino):
        raise ValueError('Private Flatpak runtime parent changed during creation')
    # Reserve the longest known Flatpak helper socket suffix, including a
    # ten-digit PID, under Linux's 108-byte sockaddr_un.sun_path capacity.
    helper_socket = directory / '.flatpak-helper/pkcs11-flatpak-2147483647'
    if len(os.fsencode(helper_socket)) >= 108:
        raise ValueError('Private Flatpak helper socket path exceeds its bound: ' + str(directory))
    return directory


def clear_native_runtime_environment(environment):
    """Remove native provider paths before starting private activation services."""
    for key in list(environment):
        if key.startswith(('QTWEBENGINE', 'DOCKPIPE_', 'QML')) or key in ['LD_LIBRARY_PATH', 'QT_PLUGIN_PATH']:
            del environment[key]


def wait_for(description, operation, timeout=30):
    deadline = time.monotonic() + timeout
    error = None
    while time.monotonic() < deadline:
        try:
            result = operation()
            if result:
                return result
        except (RuntimeError, OSError) as failure:
            error = failure
        time.sleep(0.1)
    raise RuntimeError(description + ' did not become ready: ' + str(error or 'not observed'))


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def select_private_accessibility_bus(desktop, environment):
    """Use the real bus selected by Flatpak's standard accessibility proxy."""
    import dbus
    session = dbus.bus.BusConnection(environment['DBUS_SESSION_BUS_ADDRESS'])
    launcher = session.get_object('org.a11y.Bus', '/org/a11y/bus')
    address = str(dbus.Interface(launcher, 'org.a11y.Bus').GetAddress(timeout=10))
    if not address.startswith('unix:path='):
        raise RuntimeError('Owned accessibility launcher returned an unexpected transport')
    socket = Path(address.removeprefix('unix:path=').split(',', 1)[0])
    runtime = Path(environment['XDG_RUNTIME_DIR']).resolve(strict=True)
    if not socket.resolve(strict=True).is_relative_to(runtime):
        raise RuntimeError('Accessibility launcher selected a bus outside the private runtime')
    session_daemon = dbus.Interface(session.get_object('org.freedesktop.DBus', '/org/freedesktop/DBus'),
                                    'org.freedesktop.DBus')
    service_pid = int(session_daemon.GetConnectionUnixProcessID('org.a11y.Bus', timeout=5))
    accessibility = dbus.bus.BusConnection(address)
    accessibility_daemon = dbus.Interface(accessibility.get_object('org.freedesktop.DBus', '/org/freedesktop/DBus'),
                                          'org.freedesktop.DBus')
    bus_pid = int(accessibility_daemon.GetConnectionUnixProcessID('org.freedesktop.DBus', timeout=5))
    for pid in [service_pid, bus_pid]:
        starttime = desktop.process_starttime(pid)
        if starttime is None:
            raise RuntimeError('Owned accessibility service vanished during admission')
        desktop.bus_identities[pid] = starttime
    desktop.selected_accessibility_connections = (session, accessibility, launcher)
    environment['AT_SPI_BUS_ADDRESS'] = address
    return address


def track_private_services(desktop):
    """Bind only service owners on this fixture's private session bus."""
    import dbus
    if not desktop.environment or 'DBUS_SESSION_BUS_ADDRESS' not in desktop.environment:
        return
    try:
        connection = dbus.bus.BusConnection(desktop.environment['DBUS_SESSION_BUS_ADDRESS'])
        daemon = dbus.Interface(connection.get_object('org.freedesktop.DBus', '/org/freedesktop/DBus'),
                                'org.freedesktop.DBus')
        names = ['org.a11y.Bus', 'org.freedesktop.Flatpak', 'org.freedesktop.portal.Desktop',
                 'org.freedesktop.impl.portal.desktop.gtk', 'org.freedesktop.impl.portal.desktop.gnome',
                 'org.freedesktop.portal.Documents',
                 'org.freedesktop.portal.PermissionStore', 'org.gtk.vfs.Daemon']
        for name in names:
            if daemon.NameHasOwner(name, timeout=2):
                pid = int(daemon.GetConnectionUnixProcessID(name, timeout=2))
                starttime = desktop.process_starttime(pid)
                if starttime is not None:
                    desktop.bus_identities[pid] = starttime
    except dbus.DBusException:
        return


def runtime_mounts(directory):
    prefix = str(directory) + '/'
    return [line.split()[4] for line in Path('/proc/self/mountinfo').read_text().splitlines()
            if line.split()[4].startswith(prefix)]


def owned_gui_pid(sandbox_pid, owner_pid, expected_gui_sha256):
    """Resolve the exact GUI below Flatpak's reported sandbox supervisor."""
    pending = [(sandbox_pid, 0)]
    seen = set()
    matches = []
    while pending and len(seen) < 512:
        pid, depth = pending.pop(0)
        if pid in seen:
            continue
        seen.add(pid)
        try:
            if os.getpgid(pid) != owner_pid:
                continue
            process = Path('/proc') / str(pid)
            executable = process / 'exe'
            if (Path(os.readlink(executable)).name == 'dockpipe-launcher'
                    and digest(executable) == expected_gui_sha256):
                matches.append(pid)
            if depth < 8:
                with (process / 'task' / str(pid) / 'children').open() as source:
                    children = source.read(8193)
                if len(children) > 8192:
                    raise RuntimeError('Owned GUI process tree exceeds its bound')
                pending.extend((int(value), depth + 1) for value in children.split() if value.isdigit())
        except OSError:
            continue
    if pending or len(matches) != 1:
        raise RuntimeError('Expected one exact GUI executable in the owned sandbox process tree')
    return matches[0]


def installed_launch_command(output, installation, workspace, environment, socket_directory, use_accessibility=True,
                             workspace_read_only=False):
    """Run the normal exported entrypoint, with no host execution or live network."""
    output = Path(output).resolve(strict=True)
    workspace = Path(workspace).resolve(strict=True)
    if environment.get('FLATPAK_USER_DIR') != str(Path(installation).resolve(strict=True)):
        raise ValueError('Normal GUI launch must retain the selected private installation')
    socket_directory = Path(socket_directory)
    if (socket_directory.is_symlink() or not socket_directory.is_dir()
            or socket_directory.stat().st_uid != os.getuid() or socket_directory.stat().st_mode & 0o077):
        raise ValueError('Accessibility sockets require the private owned bus directory')
    command = ['flatpak', 'run', '--user', '--branch=staging', '--unshare=network',
               '--no-documents-portal',
               '--nofilesystem=host', '--nofilesystem=home', '--filesystem=' + str(output),
               '--filesystem=' + str(workspace) + (':ro' if workspace_read_only else ''), '--cwd=' + str(workspace),
               '--filesystem=' + str(socket_directory),
               '--nosocket=wayland', '--socket=x11', '--no-talk-name=org.freedesktop.Flatpak']
    if not use_accessibility:
        command.append('--no-a11y-bus')
    for name in ['HOME', 'XDG_CONFIG_HOME', 'XDG_DATA_HOME', 'XDG_STATE_HOME', 'XDG_CACHE_HOME',
                 'DISPLAY', 'XAUTHORITY', 'AT_SPI_BUS_ADDRESS']:
        command.append('--env=' + name + '=' + environment[name])
    for name in ['LD_LIBRARY_PATH', 'LD_PRELOAD', 'LD_AUDIT', 'QT_PLUGIN_PATH', 'QT_QPA_PLATFORM_PLUGIN_PATH',
                 'QML_IMPORT_PATH', 'QML2_IMPORT_PATH', 'DOCKPIPE_PACKAGES_ROOT', 'DOCKPIPE_REPO_ROOT',
                 'QTWEBENGINE_DISABLE_SANDBOX', 'QTWEBENGINE_CHROMIUM_FLAGS', 'QTWEBENGINEPROCESS_PATH']:
        command.append('--unset-env=' + name)
    command.extend(['--env=QT_QPA_PLATFORM=xcb',
                    '--env=QT_LINUX_ACCESSIBILITY_ALWAYS_ON=' + environment.get('QT_LINUX_ACCESSIBILITY_ALWAYS_ON', '1'),
                    '--env=QT_ACCESSIBILITY=1', '--env=DOCKPIPE_SYSTEM_ROOT=/app/share/dockpipe',
                    '--env=DOCKPIPE_BIN=/app/bin/dockpipe', '--command=dockpipe-launcher',
                    APP_ID, '--allow-second-instance'])
    return command


def admitted_child(environment, expected_commit, expected_gui_sha256, owner_pid):
    result = subprocess.run(['flatpak', 'ps', '--columns=instance,child-pid,application,branch,commit:f'],
                            env=environment, check=True, capture_output=True, text=True, timeout=5)
    matches = []
    for line in result.stdout.splitlines():
        values = line.split()
        if len(values) == 5 and values[2:4] == [APP_ID, 'staging']:
            pid = int(values[1])
            if os.getpgid(pid) == owner_pid:
                matches.append((values[0], pid))
    if len(matches) != 1:
        raise RuntimeError('Expected exactly one live installed candidate process for the selected commit')
    instance, sandbox_pid = matches[0]
    pid = owned_gui_pid(sandbox_pid, owner_pid, expected_gui_sha256)
    # Flatpak 1.14 prints only twelve commit characters even for commit:f.
    # Admit the full immutable identity from the owned process namespace instead.
    metadata = configparser.ConfigParser(interpolation=None)
    path = Path('/proc') / str(pid) / 'root/.flatpak-info'
    with path.open() as source:
        text = source.read(65537)
    if len(text) > 65536:
        raise RuntimeError('Owned Flatpak process metadata exceeds its bound')
    metadata.read_string(text)
    if (metadata.get('Application', 'name', fallback='') != APP_ID
            or metadata.get('Instance', 'app-commit', fallback='') != expected_commit):
        raise RuntimeError('Owned Flatpak namespace differs from the full selected app commit')
    executable = Path('/proc') / str(pid) / 'exe'
    if digest(executable) != expected_gui_sha256:
        raise RuntimeError('Flatpak child executable differs from the installed GUI artifact')
    namespace_pids = [pid]
    for line in (Path('/proc') / str(pid) / 'status').read_text().splitlines():
        if line.startswith('NSpid:'):
            namespace_pids = [int(value) for value in line.split()[1:]]
    with (Path('/proc') / str(pid) / 'environ').open('rb') as source:
        environment_bytes = source.read((1 << 20) + 1)
    if len(environment_bytes) > 1 << 20:
        raise RuntimeError('Owned application environment exceeds its bound')
    selected_environment = {}
    for entry in environment_bytes.split(b'\0'):
        name, separator, value = entry.partition(b'=')
        if separator and name in {b'AT_SPI_BUS_ADDRESS', b'DBUS_SESSION_BUS_ADDRESS', b'QT_LINUX_ACCESSIBILITY_ALWAYS_ON',
                                  b'QTWEBENGINEPROCESS_PATH', b'QTWEBENGINE_DICTIONARIES_PATH',
                                  b'QTWEBENGINE_DISABLE_SANDBOX', b'QTWEBENGINE_CHROMIUM_FLAGS'}:
            selected_environment[name.decode()] = value.decode(errors='replace')
    return {'instance': instance, 'commit': expected_commit, 'sandbox_pid': sandbox_pid, 'host_pid': pid,
            'namespace_pids': namespace_pids, 'executable_sha256': expected_gui_sha256,
            'accessibility_environment': selected_environment,
            'kernel_starttime': (Path('/proc') / str(pid) / 'stat').read_text().rsplit(')', 1)[1].split()[19]}


class GuiActions:
    def __init__(self, ui_module, identity):
        import pyatspi
        self.atspi = pyatspi
        self.registry = pyatspi.Registry
        self.ui = ui_module
        allowed = set(identity['namespace_pids']) | {identity['host_pid']}
        desktop_applications = list(self.registry.getDesktop(0))
        applications = [application for application in desktop_applications
                        if application.get_process_id() in allowed]
        if len(applications) != 1:
            observed_pids = [application.get_process_id() for application in desktop_applications[:32]]
            raise RuntimeError('Private accessibility bus must expose exactly the admitted installed GUI; observed PIDs '
                               + str(observed_pids))
        self.pid = applications[0].get_process_id()

    def visible(self, name, role):
        matches = [node for node, _ in self.ui.nodes(self.registry, self.pid)
                   if node.name == name and node.getRoleName() == role
                   and node.getState().contains(self.atspi.STATE_SHOWING)]
        if len(matches) != 1:
            raise RuntimeError('Expected one visible owned ' + role + ': ' + name)
        return matches[0]

    def activate(self, node):
        interface = node.queryAction()
        names = [interface.getName(index) for index in range(interface.nActions)]
        allowed = [index for index, name in enumerate(names)
                   if name.lower() in {'click', 'press', 'activate', 'toggle', 'select'}]
        if len(allowed) != 1 or not interface.doAction(allowed[0]):
            raise RuntimeError('Expected one supported accessibility action for ' + node.name + ': ' + str(names))

    def click(self, name, role='push button'):
        self.activate(self.visible(name, role))

    def snapshot(self, destination):
        destination.write_text(json.dumps(self.ui.tree(self.registry, self.pid), indent=2) + '\n')
        destination.chmod(0o600)

    def open_settings(self):
        self.click('Settings')
        return wait_for('Normal Settings dialog', lambda: self.visible('Show Dashboard above Apps', 'check box'))

    def toggle_and_save_setting(self):
        checkbox = self.open_settings()
        before = checkbox.getState().contains(self.atspi.STATE_CHECKED)
        self.activate(checkbox)
        after = checkbox.getState().contains(self.atspi.STATE_CHECKED)
        if before == after:
            raise RuntimeError('Settings checkbox did not change through the normal UI')
        self.click('Save')
        return after

    def setting_retained(self, expected):
        checkbox = self.open_settings()
        actual = checkbox.getState().contains(self.atspi.STATE_CHECKED)
        self.click('Cancel')
        if actual != expected:
            raise RuntimeError('Saved setting did not survive normal installed GUI relaunch')
        return True

    def workflow_catalog(self, titles):
        self.click('Workflows', 'list item')
        self.click('Workflows')
        def catalog_rows():
            names = [node.name for node, _ in self.ui.nodes(self.registry, self.pid)
                     if node.getRoleName() == 'list item'
                     and node.getState().contains(self.atspi.STATE_SHOWING)]
            return names if all(any(title in name for name in names) for title in titles) else None
        rows = wait_for('Selected project workflow catalog', catalog_rows)
        self.click('Cancel')
        return rows

    def quit_normally(self):
        # The standard File menu item invokes the normal quit/confirmation path.
        node = self.ui.select(self.registry, self.pid, 'Quit', 'menu item')
        self.activate(node)
def write_private_json(path, value):
    with Path(path).open('x') as output:
        json.dump(value, output, indent=2)
        output.write('\n')
    Path(path).chmod(0o600)


def supervise_observer(command, environment, log, timeout=75):
    import importlib.util
    specification = importlib.util.spec_from_file_location(
        'flatpak_observer_commands', Path(__file__).with_name('verify-installed-host.py'))
    commands = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(commands)
    return commands.run_command(command, environment, log, timeout)


def run_native_observer(request):
    """This disposable child alone imports the native accessibility library."""
    import resource
    import sys
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
    if set(request) != {'output', 'environment', 'identity', 'owner_pid', 'phase', 'expected_setting'}:
        raise ValueError('Invalid bounded native observer request')
    if request['phase'] not in ['first', 'reopen']:
        raise ValueError('Unknown installed GUI observation phase')
    output = Path(request['output']).resolve(strict=True)
    os.chdir(output)
    environment = request['environment']
    if (not isinstance(environment, dict) or len(environment) > 64
            or any(not isinstance(value, str) or len(value) > 4096 for value in environment.values())):
        raise ValueError('Native observer environment exceeds its bound')
    os.environ.clear()
    os.environ.update(environment)
    expected = request['identity']
    actual = admitted_child(environment, expected['commit'], expected['executable_sha256'], request['owner_pid'])
    if any(actual[key] != expected[key] for key in ['host_pid', 'kernel_starttime', 'instance', 'commit']):
        raise RuntimeError('Original GUI identity changed before observation')
    # Prove the selected private bus still responds before native library import.
    import dbus
    connection = dbus.bus.BusConnection(environment['AT_SPI_BUS_ADDRESS'])
    daemon = dbus.Interface(connection.get_object('org.freedesktop.DBus', '/org/freedesktop/DBus'),
                            'org.freedesktop.DBus')
    if not daemon.NameHasOwner('org.freedesktop.DBus', timeout=5):
        raise RuntimeError('Private accessibility bus is unavailable')
    linux = Path(__file__).resolve().parents[5] / 'workflows/qualify/product/platforms/linux'
    sys.path.insert(0, str(linux))
    import ui
    driver = wait_for('Owned GUI accessibility application', lambda: GuiActions(ui, actual))
    if request['phase'] == 'first':
        wait_for('Normal main window', lambda: driver.visible('Settings', 'push button'))
        driver.snapshot(output / 'desktop/normal-window.json')
        rows = driver.workflow_catalog(['Product authored fixture', 'Product packaged fixture'])
        driver.snapshot(output / 'desktop/first-project-catalog.json')
        selected = driver.toggle_and_save_setting()
        result = {'normal_gui_started': True, 'selected_project_workflows_visible': bool(rows),
                  'expected_setting': selected, 'catalog_rows': rows[:128]}
    else:
        retained = driver.setting_retained(request['expected_setting'])
        rows = driver.workflow_catalog(['Product authored fixture', 'Product packaged fixture'])
        driver.snapshot(output / 'desktop/reopened-project-catalog.json')
        result = {'settings_retained': retained, 'project_reopen': bool(rows), 'catalog_rows': rows[:128]}
    driver.quit_normally()
    write_private_json(output / ('observer-' + request['phase'] + '-result.json'), result)


def guided_windows(environment, identity):
    import re
    result = subprocess.run(['/usr/bin/xdotool', 'search', '--pid', str(identity['namespace_pids'][-1])],
                            env=environment, capture_output=True, text=True, timeout=5)
    windows = []
    for value in result.stdout.splitlines()[:32]:
        if not value.isdigit():
            continue
        identifier = int(value)
        properties = subprocess.run(['/usr/bin/xprop', '-id', str(identifier), '_NET_WM_PID', 'WM_CLASS', 'WM_NAME'],
                                    env=environment, capture_output=True, text=True, timeout=5).stdout
        if not re.search(r'_NET_WM_PID\(CARDINAL\) = ' + str(identity['namespace_pids'][-1]) + r'\b', properties):
            continue
        geometry = subprocess.run(['/usr/bin/xwininfo', '-id', str(identifier)], env=environment,
                                  capture_output=True, text=True, timeout=5).stdout
        if 'Map State: IsViewable' not in geometry:
            continue
        values = {}
        for key, label in [('x', 'Absolute upper-left X'), ('y', 'Absolute upper-left Y'),
                           ('width', 'Width'), ('height', 'Height')]:
            match = re.search(re.escape(label) + r':\s*(-?[0-9]+)', geometry)
            if not match:
                break
            values[key] = int(match.group(1))
        if len(values) == 4:
            windows.append({'id': identifier, 'properties': properties[:2048], **values})
    return windows


def guided_capture(output, environment, number):
    raw = output / ('guided-shot-' + str(number).zfill(3) + '.xwd')
    image = raw.with_suffix('.png')
    subprocess.run(['/usr/bin/xwd', '-silent', '-root', '-out', str(raw)], env=environment,
                   check=True, capture_output=True, timeout=10)
    if raw.stat().st_size > 16 << 20:
        raise RuntimeError('Owned display capture exceeded its fixed geometry bound')
    # XWD v7 layout is defined by X11/XWDFile.h. Only this owned Xvfb's fixed
    # TrueColor ZPixmap tuple is accepted; conversion runs outside the controller.
    conversion = """from PIL import Image
from pathlib import Path
import struct
import sys
raw = Path(sys.argv[1]).read_bytes()
header = struct.unpack('!25I', raw[:100])
assert header[1:6] == (7, 2, 24, 1600, 1000)
assert header[6] == 0 and header[7] in (0, 1)
assert header[11] in (24, 32) and header[12] == 6400
assert header[14:17] == (0xff0000, 0xff00, 0xff)
offset = header[0] + header[19] * 12
assert 100 <= offset < len(raw) and len(raw) - offset == 6400 * 1000
encoding = ('BGR' if header[11] == 24 else 'BGRX') if header[7] == 0 else ('RGB' if header[11] == 24 else 'XRGB')
image = Image.frombytes('RGB', (1600, 1000), raw[offset:], 'raw', encoding, 6400, 1)
image.save(sys.argv[2])
"""
    subprocess.run(['/usr/bin/python3', '-c', conversion, str(raw), str(image)],
                   check=True, capture_output=True, timeout=10)
    image.chmod(0o600)
    raw.unlink()
    return {'path': image.name, 'sha256': digest(image), 'captured_at_ns': time.time_ns()}


def validate_guided_text(value, workspace, retained_profile, browser_urls=None):
    allowed = ['Product authored fixture', 'Product packaged fixture', '# guided integration fixture\n']
    if retained_profile is not None:
        allowed = []
    if browser_urls is not None:
        allowed = browser_urls
    if value not in allowed:
        raise ValueError('Guided text must be an exact generated fixture value')


def guided_session(output, desktop, launch, process, workspace, expected_commit, expected_gui_sha256,
                   retained_profile=None, browser_fixture=None):
    """Agent-guided inputs require a recent observed snapshot; no pass flags are accepted."""
    output = Path(output)
    marker = workspace / 'product-workflow-result.txt'
    if retained_profile is None and browser_fixture is None:
        marker.unlink(missing_ok=True)  # Only the generated marker from this invocation's CLI baseline.
    workflow = workspace / 'workflows/authored-fixture/config.yml'
    original_workflow = workflow.read_bytes() if browser_fixture is None else b''
    settings = Path(desktop.environment['HOME']) / '.var/app' / APP_ID / 'config/dockpipe/dockpipe-launcher/launcher.json'
    settings_before = json.loads(settings.read_text())
    identities = []
    observations = []
    actions = []
    phase = 'first'
    first_exit = None
    quit_requested = False
    first_settings = None
    run_input_ns = None
    snapshot_number = 0
    sequence = 1
    deadline = time.monotonic() + 780

    def admit():
        identity = wait_for('Exact guided GUI identity', lambda: admitted_child(
            desktop.environment, expected_commit, expected_gui_sha256, process.pid))
        if browser_fixture is not None:
            metadata = wait_for('Runtime-local HTTP fixture', lambda: browser_fixture['module'].metadata(
                output, 1 if phase == 'first' else 2))
            browser_fixture['metadata'] = metadata
            browser_fixture['module'].admit_server(output, metadata, identity, process.pid,
                                                   browser_fixture['python_sha256'])
        identities.append(identity)
        write_private_json(output / ('owned-guided-' + phase + '-kernel-journal.json'),
                           {'gui': identity, 'wrapper_pid': process.pid,
                            'owned_processes': {str(item.pid): desktop.process_starttime(item.pid)
                                                for item in desktop.processes},
                            'owned_buses': desktop.bus_identities})
        return identity

    identity = admit()
    wait_for('Normal installed GUI window', lambda: guided_windows(desktop.environment, identity))
    latest = None
    while time.monotonic() < deadline and sequence <= 96:
        if process.poll() is not None and not quit_requested:
            raise RuntimeError('The owned normal GUI exited unexpectedly with status ' + str(process.returncode))
        if browser_fixture is not None and process.poll() is None:
            browser_fixture['module'].record_children(output, identity)
        snapshot_number += 1
        latest = guided_capture(output, desktop.environment, snapshot_number)
        windows = guided_windows(desktop.environment, identity) if process.poll() is None else []
        ready = {'schema': 'dockpipe.flatpak.guided-ui/v1', 'phase': phase, 'next_sequence': sequence,
                 'snapshot': latest, 'windows': windows, 'identity': identity,
                 'display': desktop.environment['DISPLAY'], 'xauthority': desktop.environment['XAUTHORITY'],
                 'tool_sha256': {name: digest('/usr/bin/' + name) for name in ['xwd', 'xdotool', 'xprop', 'xwininfo']}}
        if browser_fixture is not None:
            ready['browser_fixture'] = browser_fixture['metadata']
        temporary = output / 'guided-ready.next'
        temporary.write_text(json.dumps(ready, indent=2) + '\n')
        temporary.chmod(0o600)
        temporary.replace(output / 'guided-ready.json')
        command_path = output / ('guided-action-' + str(sequence).zfill(3) + '.json')
        while not command_path.exists() and time.monotonic() < deadline:
            time.sleep(0.1)
        if not command_path.exists():
            raise RuntimeError('Guided GUI observation deadline expired; no result inferred')
        if command_path.is_symlink() or command_path.stat().st_size > 8192:
            raise ValueError('Invalid bounded guided action')
        command = json.loads(command_path.read_text())
        base = {'sequence', 'basis_sha256', 'action', 'purpose'}
        if not base.issubset(command) or command['sequence'] != sequence or command['basis_sha256'] != latest['sha256']:
            raise ValueError('Guided action must bind the current observed snapshot and sequence')
        if command['action'] != 'capture' and time.time_ns() - latest['captured_at_ns'] > 120 * 10**9:
            raise ValueError('Observe a fresh snapshot before acting')
        action = command['action']
        fields = {'click': {'window_id', 'x', 'y'}, 'scroll': {'window_id', 'x', 'y', 'delta'}, 'key': {'window_id', 'key'},
                  'text': {'window_id', 'text'}, 'observe': {'items'}, 'capture': set(),
                  'relaunch': set(), 'finish': set()}
        if (action not in fields or set(command) != base | fields[action]
                or not isinstance(command['purpose'], str) or not 1 <= len(command['purpose']) <= 256):
            raise ValueError('Unknown guided action fields or missing observation purpose')
        event = {'sequence': sequence, 'phase': phase, 'command': command,
                 'basis_snapshot': latest, 'dispatched_at_ns': time.time_ns()}
        if action in ['click', 'key', 'text', 'scroll']:
            actual = admitted_child(desktop.environment, expected_commit, expected_gui_sha256, process.pid)
            if actual['host_pid'] != identity['host_pid'] or actual['kernel_starttime'] != identity['kernel_starttime']:
                raise RuntimeError('Original guided GUI identity changed')
            selected = [window for window in windows if window['id'] == command.get('window_id')]
            if len(selected) != 1:
                raise ValueError('Input must target one currently visible owned GUI window')
            window = selected[0]
            subprocess.run(['/usr/bin/xdotool', 'windowfocus', '--sync', str(window['id'])],
                           env=desktop.environment, check=True, timeout=5)
            if action in ['click', 'scroll']:
                x, y = command['x'], command['y']
                if not (isinstance(x, int) and isinstance(y, int) and window['x'] <= x < window['x'] + window['width']
                        and window['y'] <= y < window['y'] + window['height']):
                    raise ValueError('Click escaped the observed owned window geometry')
                operation = ['click', '1']
                if action == 'scroll':
                    delta = command['delta']
                    if browser_fixture is None or not isinstance(delta, int) or delta == 0 or abs(delta) > 5:
                        raise ValueError('Browser scroll must remain within five wheel events')
                    operation = ['click', '--repeat', str(abs(delta)), '--delay', '100', '4' if delta > 0 else '5']
                subprocess.run(['/usr/bin/xdotool', 'mousemove', '--sync', str(x), str(y), *operation],
                               env=desktop.environment, check=True, timeout=5)
            elif action == 'key':
                allowed = ['Escape', 'Return', 'space', 'Tab', 'ctrl+q', 'ctrl+p', 'ctrl+s', 'ctrl+a',
                           'ctrl+o', 'ctrl+Home', 'ctrl+End', 'Home', 'End', 'Up', 'Down', 'Left', 'Right']
                if browser_fixture is not None:
                    allowed.extend(['ctrl+l', 'ctrl+t', 'ctrl+w', 'ctrl+Tab', 'ctrl+shift+Tab', 'ctrl+r', 'Page_Down', 'Page_Up'])
                if command['key'] not in allowed:
                    raise ValueError('Guided key is outside the generated UI fixture scope')
                if command['key'] == 'ctrl+q':
                    quit_requested = True
                subprocess.run(['/usr/bin/xdotool', 'key', '--clearmodifiers', command['key']],
                               env=desktop.environment, check=True, timeout=5)
            else:
                browser_urls = browser_fixture['module'].urls(browser_fixture['metadata']) if browser_fixture else None
                validate_guided_text(command['text'], workspace, retained_profile, browser_urls)
                subprocess.run(['/usr/bin/xdotool', 'type', '--clearmodifiers', '--delay', '20', '--', command['text']],
                               env=desktop.environment, check=True, timeout=10)
            if command['purpose'] == 'run-authored-fixture':
                run_input_ns = event['dispatched_at_ns']
            time.sleep(0.4)
        elif action == 'observe':
            if not isinstance(command.get('items'), list) or len(command['items']) > 20:
                raise ValueError('Record bounded individual visible controls, not a blanket result')
            for item in command['items']:
                roles = ['shell-label', 'workflow-row', 'checkbox', 'button']
                if browser_fixture is not None:
                    roles.extend(['browser-page', 'browser-tab', 'browser-state', 'download-row'])
                if (set(item) - {'text', 'role', 'state'} or not isinstance(item.get('text'), str)
                        or len(item['text']) > 256 or item.get('role') not in roles):
                    raise ValueError('Invalid individual visual observation')
            observations.append({'phase': phase, 'snapshot': latest, 'items': command['items']})
        elif action == 'capture':
            pass
        elif action == 'relaunch':
            if phase != 'first':
                raise ValueError('Only one normal fixture relaunch is allowed')
            first_exit = process.wait(timeout=15)
            if first_exit != 0:
                raise RuntimeError('First normal GUI quit did not exit zero')
            if browser_fixture is not None:
                browser_fixture['module'].request_stop(output, 1)
                browser_fixture['module'].require_closed(output)
            first_settings = json.loads(settings.read_text())
            phase = 'reopen'
            process = launch(desktop, desktop.environment, workspace)
            quit_requested = False
            desktop.processes.append(process)
            identity = admit()
            wait_for('Reopened normal GUI window', lambda: guided_windows(desktop.environment, identity))
        elif action == 'finish':
            if phase != 'reopen' or first_exit != 0 or process.wait(timeout=15) != 0:
                raise RuntimeError('Both GUI runs must quit normally before finishing')
            if browser_fixture is not None:
                browser_fixture['module'].request_stop(output, 2)
                browser_fixture['module'].require_closed(output)
            actions.append(event)
            break
        else:
            raise ValueError('Unknown guided GUI action')
        actions.append(event)
        write_private_json(output / ('guided-event-' + str(sequence).zfill(3) + '.json'), event)
        sequence += 1
    else:
        raise RuntimeError('Guided UI action/deadline bound exhausted')

    def saw(selected_phase, role, text, state=None):
        return any(record['phase'] == selected_phase and any(
            item['role'] == role and item['text'] == text and (state is None or item.get('state') == state)
            for item in record['items']) for record in observations)
    final_settings = json.loads(settings.read_text())
    workflow_changed = browser_fixture is None and workflow.read_bytes() == b'# guided integration fixture\n' + original_workflow
    executed = (run_input_ns is not None and marker.is_file() and marker.read_text() == 'product-fixture'
                and marker.stat().st_mtime_ns >= run_input_ns)
    settings_retained = (settings_before.get('defaultDashboardVisible') is False
                         and first_settings.get('defaultDashboardVisible') is True
                         and final_settings.get('defaultDashboardVisible') is True
                         and saw('reopen', 'checkbox', 'Show Dashboard above Apps', 'checked'))
    retained_workflow = False
    if retained_profile is not None:
        retained_workflow = (digest(workflow) == retained_profile['workflow_sha256']
                             and workflow.read_bytes() == original_workflow)
        settings_retained = (settings_before.get('defaultDashboardVisible') is True
                             and first_settings.get('defaultDashboardVisible') is True
                             and final_settings.get('defaultDashboardVisible') is True
                             and saw('first', 'checkbox', 'Show Dashboard above Apps', 'checked')
                             and saw('reopen', 'checkbox', 'Show Dashboard above Apps', 'checked'))
    title = ['Product authored fixture', 'Product packaged fixture']
    result = {'observer_method': 'agent-guided-visual-keyboard', 'normal_gui_started': all(
        saw('first', 'shell-label', name) for name in ['Apps', 'Workflows', 'Settings']),
        'settings_retained': settings_retained, 'selected_project_workflows_visible': all(
            saw('first', 'workflow-row', name) for name in title),
        'project_reopen': all(saw('reopen', 'workflow-row', name) for name in title)
                          and final_settings.get('projectFolder') == str(workspace),
        'edit_save_run': workflow_changed and executed, 'gui_exit_normal': True,
        'identities': identities, 'remote_session_opened': False, 'host_bridge_allowed': False,
        'vm_provider_installed': False, 'atspi_qualified': False,
        'independent_readback': {'workflow_sha256': digest(workflow), 'settings_sha256': digest(settings),
                                 'edited_comment_exact': workflow_changed, 'new_gui_run_marker': executed},
        'observations': observations, 'input_events': len(actions)}
    if retained_profile is not None:
        result['retained_profile'] = {'stage': retained_profile['stage'],
                                     'authored_workflow_unchanged': retained_workflow,
                                     'initial_settings_sha256': retained_profile['settings_sha256'],
                                     'seed_profile_used': False}
    if browser_fixture is not None:
        result['browser'] = browser_fixture['module'].evaluate(output, Path(desktop.environment['HOME']),
                                                              final_settings, first_settings, observations)
    write_private_json(output / 'guided-observations.json', {'events': actions, 'observations': observations})
    return result


def admit_retained_profile(selected, workspace):
    expected = {'home', 'stage', 'settings_sha256', 'workflow_sha256', 'workspace_access'}
    if (not isinstance(selected, dict) or set(selected) != expected
            or selected['stage'] not in ['prepare-a', 'verify-b']
            or selected['workspace_access'] != 'read-only-original'):
        raise ValueError('Select the exact retained-profile stage and input identities')
    home = Path(selected['home'])
    workspace = Path(workspace)
    for path in [home, workspace]:
        if (not path.is_absolute() or path.is_symlink() or not path.is_dir()
                or path.stat().st_uid != os.getuid() or path.stat().st_mode & 0o022):
            raise ValueError('Retained profile and workspace require private owned directories')
    settings = home / '.var/app' / APP_ID / 'config/dockpipe/dockpipe-launcher/launcher.json'
    workflow = workspace / 'workflows/authored-fixture/config.yml'
    for path, identity in [(settings, 'settings_sha256'), (workflow, 'workflow_sha256')]:
        owner = home if identity == 'settings_sha256' else workspace
        if (path.is_symlink() or not path.is_file() or path.stat().st_size > 1 << 20
                or not path.resolve(strict=True).is_relative_to(owner)
                or digest(path) != selected[identity]):
            raise ValueError('Retained profile input changed: ' + identity)
    if json.loads(settings.read_text()).get('defaultDashboardVisible') is not True:
        raise ValueError('Retained fixture requires its A-authored saved Dashboard setting')
    if json.loads(settings.read_text()).get('projectFolder') != str(workspace):
        raise ValueError('Retained profile must select the exact protected original project path')
    return home


def qualify(output, installation, workspace, xvfb, expected_commit, expected_gui_sha256, method='atspi',
            retained_profile=None, browser_fixture=None):
    """Save/relaunch through the installed entrypoint with an explicit observation method."""
    import sys
    root = Path(__file__).resolve().parents[5]
    linux = root / 'workflows/qualify/product/platforms/linux'
    sys.path.insert(0, str(linux))
    from desktop import OwnedDesktop
    output = Path(output).resolve(strict=True)
    installation = Path(installation).resolve(strict=True)
    workspace = Path(workspace).resolve(strict=True)
    if browser_fixture is not None:
        import importlib.util
        import re
        if (method != 'guided' or retained_profile is not None or not isinstance(browser_fixture, dict)
                or set(browser_fixture) != {'python_sha256'}
                or not isinstance(browser_fixture['python_sha256'], str)
                or not re.fullmatch(r'[0-9a-f]{64}', browser_fixture['python_sha256'])):
            raise ValueError('Select the explicit guided browser fixture and pinned Platform Python')
        specification = importlib.util.spec_from_file_location('installed_browser_actions',
                            Path(__file__).with_name('installed-browser.py'))
        browser = importlib.util.module_from_spec(specification)
        specification.loader.exec_module(browser)
        browser_fixture = dict(browser_fixture, module=browser)
        browser.prepare(output)
    retained_home = None
    if retained_profile is not None:
        if method != 'guided':
            raise ValueError('Retained-profile migration requires the explicit guided method')
        retained_home = admit_retained_profile(retained_profile, workspace)
        if retained_home.parent != output.parent:
            raise ValueError('Retained-profile stages must remain within their private invocation root')
    selections = {key: '/unused-contained-provider' for key in
                  ['qt_prefix', 'browser_sdk_prefix', 'qemu_prefix', 'cli', 'store', 'gui']}
    selections.update(xvfb=str(xvfb), workspace_path=str(workspace))
    runtime_directory = None
    runtime_identity = None

    def prepare(environment):
        nonlocal runtime_directory, runtime_identity
        clear_native_runtime_environment(environment)
        home = retained_home or output / 'gui-home'
        if retained_home is None:
            home.mkdir(mode=0o700)
        if browser_fixture is not None:
            (home / 'Downloads').mkdir(mode=0o700)
        runtime_directory = create_private_runtime('dockpipe-flatpak-runtime-')
        status = runtime_directory.stat()
        runtime_identity = (status.st_dev, status.st_ino)
        environment.update(HOME=str(home), FLATPAK_USER_DIR=str(installation), FLATPAK_GL_DRIVERS='default',
                           XDG_RUNTIME_DIR=str(runtime_directory), GIO_USE_VFS='local', GVFS_DISABLE_FUSE='1')

    launches = []
    def launch(desktop, environment, selected_workspace):
        if not launches and retained_profile is None:
            # Flatpak selects its per-app XDG namespace beneath the private HOME,
            # independently of the host services' XDG config directory.
            profile = Path(environment['HOME']) / '.var/app' / APP_ID / 'config/dockpipe/dockpipe-launcher'
            profile.mkdir(mode=0o700, parents=True, exist_ok=True)
            shutil.copyfile(desktop.output / 'config/dockpipe/dockpipe-launcher/launcher.json', profile / 'launcher.json')
            (profile / 'launcher.json').chmod(0o600)
            if browser_fixture is not None:
                user_directories = profile.parents[1] / 'user-dirs.dirs'
                user_directories.write_text('XDG_DOWNLOAD_DIR=' + json.dumps(str(Path(environment['HOME']) / 'Downloads')) + '\n')
                user_directories.chmod(0o600)
        if not launches and method == 'atspi':
            select_private_accessibility_bus(desktop, environment)
        if method == 'guided':
            environment.update(QT_LINUX_ACCESSIBILITY_ALWAYS_ON='0', QT_ACCESSIBILITY='0', NO_AT_BRIDGE='1')
        number = len(launches) + 1
        with (desktop.output / ('installed-gui-' + str(number) + '.log')).open('xb') as log:
            command = installed_launch_command(output, installation, selected_workspace, environment,
                                               desktop.socket_directory, use_accessibility=method == 'atspi',
                                               workspace_read_only=retained_profile is not None)
            if browser_fixture is not None:
                command = browser_fixture['module'].launch_command(command, output, number)
            process = subprocess.Popen(command,
                                       env=environment, cwd=workspace, stdout=log, stderr=log, start_new_session=True)
        launches.append(process)
        return process

    selections.update(prepare_environment=prepare, normal_gui_launcher=launch)
    desktop = OwnedDesktop(output / 'desktop', selections)
    result = {'normal_gui_started': False, 'settings_retained': False, 'selected_project_workflows_visible': False,
              'project_reopen': False, 'gui_exit_normal': False, 'remote_session_opened': False,
              'host_bridge_allowed': False, 'vm_provider_installed': False}
    identities = []
    try:
        process = desktop.start()
        if method == 'guided':
            result.update(guided_session(output, desktop, launch, process, workspace,
                                         expected_commit, expected_gui_sha256, retained_profile, browser_fixture))
            identities.extend(result['identities'])
            shutil.copyfile(Path(desktop.environment['HOME']) / '.var/app' / APP_ID / 'config/dockpipe/dockpipe-launcher/launcher.json',
                            output / 'guided-settings-readback.json')
            shutil.copyfile(workspace / 'workflows/authored-fixture/config.yml',
                            output / 'guided-authored-readback.yml')
            evidence = [*output.glob('guided-shot-*.png'), *output.glob('guided-event-*.json'),
                        output / 'guided-observations.json', output / 'guided-settings-readback.json',
                        output / 'guided-authored-readback.yml']
            if browser_fixture is not None:
                evidence.extend(output.glob('browser-*'))
            if sum(path.stat().st_size for path in evidence) > 128 << 20:
                raise RuntimeError('Guided evidence exceeded its 128MiB bound')
            with tarfile.open(output / 'guided-evidence.tar.gz', 'x:gz') as archive:
                for path in evidence:
                    if path.is_symlink() or not path.is_file():
                        raise RuntimeError('Guided evidence must remain regular invocation-owned files')
                    archive.add(path, arcname=path.name, recursive=False)
            return result
        def observe(phase, expected_setting=None):
            identity = wait_for('Exact installed GUI identity', lambda: admitted_child(
                desktop.environment, expected_commit, expected_gui_sha256, process.pid))
            identities.append(identity)
            journal = {'gui': identity, 'wrapper_pid': process.pid,
                       'owned_processes': {str(item.pid): desktop.process_starttime(item.pid)
                                           for item in desktop.processes},
                       'owned_buses': desktop.bus_identities, 'runtime_directory': str(runtime_directory),
                       'runtime_identity': runtime_identity}
            write_private_json(output / ('owned-' + phase + '-kernel-journal.json'), journal)
            request = {'output': str(output), 'environment': desktop.environment, 'identity': identity,
                       'owner_pid': process.pid, 'phase': phase, 'expected_setting': expected_setting}
            path = output / ('observer-' + phase + '-request.json')
            write_private_json(path, request)
            supervise_observer(['/usr/bin/python3', str(Path(__file__).resolve()), '--observe', str(path)],
                               desktop.environment, output / ('observer-' + phase + '.log'))
            return json.loads((output / ('observer-' + phase + '-result.json')).read_text())
        first = observe('first')
        result.update(normal_gui_started=first['normal_gui_started'],
                      selected_project_workflows_visible=first['selected_project_workflows_visible'])
        process.wait(timeout=15)
        if process.returncode != 0:
            raise RuntimeError('Normal installed GUI first exit failed')
        process = launch(desktop, desktop.environment, workspace)
        desktop.processes.append(process)
        reopened = observe('reopen', first['expected_setting'])
        result.update(settings_retained=reopened['settings_retained'], project_reopen=reopened['project_reopen'])
        process.wait(timeout=15)
        if process.returncode != 0:
            raise RuntimeError('Normal installed GUI reopened exit failed')
        result['gui_exit_normal'] = True
    finally:
        result['identities'] = identities
        if browser_fixture is not None:
            for number in range(1, len(launches) + 1):
                browser_fixture['module'].request_stop(output, number)
        # A failing UI observation still owns its freshly launched process group.
        # Normal exit is required for acceptance; cleanup cannot create a pass.
        for process in launches:
            if process.poll() is None:
                try:
                    os.killpg(process.pid, signal.SIGTERM)
                except ProcessLookupError:
                    pass
        track_private_services(desktop)
        write_private_json(output / 'owned-cleanup-kernel-journal.json',
                           {'owned_buses': desktop.bus_identities, 'gui': identities,
                            'owned_processes': {str(item.pid): desktop.process_starttime(item.pid)
                                                for item in desktop.processes}})
        desktop.stop()
        if browser_fixture is not None and (output / 'browser-owned-children.json').exists():
            try:
                browser_fixture['module'].require_closed(output)
            except RuntimeError as error:
                result['cleanup_error'] = str(error)
                result['gui_exit_normal'] = False
        if runtime_directory is not None:
            result['runtime_directory'] = str(runtime_directory)
            result['runtime_identity'] = runtime_identity
            try:
                status = runtime_directory.lstat()
                if runtime_directory.is_symlink() or (status.st_dev, status.st_ino) != runtime_identity:
                    raise RuntimeError('Owned Flatpak runtime directory identity changed')
                deadline = time.monotonic() + 3
                while runtime_mounts(runtime_directory) and time.monotonic() < deadline:
                    time.sleep(0.1)
                if runtime_mounts(runtime_directory):
                    raise RuntimeError('Owned runtime still contains a mount; retained for exact reconciliation')
                shutil.rmtree(runtime_directory)
                result['runtime_directory_cleaned'] = not runtime_directory.exists()
            except (OSError, RuntimeError) as error:
                result['cleanup_error'] = str(error)
                result['gui_exit_normal'] = False
        (output / 'gui-observed.json').write_text(json.dumps(result, indent=2) + '\n')
        (output / 'gui-observed.json').chmod(0o600)
    return result


def main():
    import argparse
    parser = argparse.ArgumentParser(description=__doc__)
    selected = parser.add_mutually_exclusive_group(required=True)
    selected.add_argument('--request', type=Path)
    selected.add_argument('--observe', type=Path)
    arguments = parser.parse_args()
    path = arguments.observe or arguments.request
    if not path.is_absolute() or path.is_symlink() or not path.is_file() or path.stat().st_size > 65536:
        parser.error('Select the bounded private GUI request')
    request = json.loads(path.read_text())
    if arguments.observe:
        run_native_observer(request)
        return
    required = {'output', 'installation', 'workspace', 'xvfb', 'expected_commit', 'expected_gui_sha256'}
    if (not required.issubset(request) or set(request) - required - {'method', 'retained_profile', 'browser_fixture'}
            or request.get('method', 'atspi') not in ['atspi', 'guided']):
        parser.error('Invalid installed GUI request')
    def interrupted(_number, _frame):
        raise KeyboardInterrupt
    previous = signal.signal(signal.SIGTERM, interrupted)
    try:
        qualify(**request)
    finally:
        signal.signal(signal.SIGTERM, previous)


if __name__ == '__main__':
    main()
