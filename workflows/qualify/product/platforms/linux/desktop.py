"""Owned X11/DBus desktop for an explicitly selected Linux product candidate."""
from pathlib import Path
import hashlib
import json
import os
import secrets
import shutil
import signal
import subprocess
import tempfile
import time


def digest(path):
    checksum = hashlib.sha256()
    with Path(path).open('rb') as stream:
        for block in iter(lambda: stream.read(4 << 20), b''):
            checksum.update(block)
    return checksum.hexdigest()


def remove_owned_socket_directory(directory):
    """Remove only the two generated bus sockets, never runtime trees or mounts."""
    import stat
    directory = Path(directory)
    descriptor = os.open(directory, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        identity = os.fstat(descriptor)
        if identity.st_uid != os.getuid() or stat.S_IMODE(identity.st_mode) != 0o700:
            raise ValueError('Owned socket directory identity is not private')
        entries = {}
        for name in os.listdir(descriptor):
            if name not in {'session-bus', 'accessibility-bus'}:
                raise ValueError('Retain runtime directory with unknown entries or mount directories')
            try:
                entry = os.stat(name, dir_fd=descriptor, follow_symlinks=False)
            except FileNotFoundError:
                # A stopped owned bus may unlink its known socket after listdir.
                continue
            if not stat.S_ISSOCK(entry.st_mode):
                raise ValueError('Retain runtime directory with unknown entries or mount directories')
            entries[name] = (entry.st_dev, entry.st_ino)
        # No recursion: Flatpak may create doc FUSE mounts and helper directories.
        for name, expected in entries.items():
            try:
                entry = os.stat(name, dir_fd=descriptor, follow_symlinks=False)
            except FileNotFoundError:
                continue
            if (entry.st_dev, entry.st_ino) != expected or not stat.S_ISSOCK(entry.st_mode):
                raise ValueError('Owned bus socket changed before cleanup')
            try:
                os.unlink(name, dir_fd=descriptor)
            except FileNotFoundError:
                # The same stopped bus may finish unlinking after the check.
                continue
        current = directory.lstat()
        if (current.st_dev, current.st_ino) != (identity.st_dev, identity.st_ino):
            raise ValueError('Owned socket directory path changed before removal')
        directory.rmdir()
    finally:
        os.close(descriptor)


MAXIMUM_SCREEN_DIMENSION = 4096
# Supports UHD 3840x2160 while bounding a padded 32-bit Xvfb framebuffer to 32 MiB.
MAXIMUM_SCREEN_PIXELS = 8 * 1024 * 1024


def screen_geometry(value):
    if not isinstance(value, dict) or set(value) != {'width', 'height'}:
        raise ValueError('Screen geometry requires exactly width and height')
    width = value['width']
    height = value['height']
    if (type(width) is not int or type(height) is not int
            or not 1 <= width <= MAXIMUM_SCREEN_DIMENSION
            or not 1 <= height <= MAXIMUM_SCREEN_DIMENSION
            or width * height > MAXIMUM_SCREEN_PIXELS):
        raise ValueError('Screen dimensions or pixel budget exceeded')
    return f'{width}x{height}x24'


class OwnedDesktop:
    def __init__(self, output, selections):
        self.output = Path(output)
        self.selections = selections
        self.processes = []
        self.bus_identities = {}
        self.environment = None
        self.socket_directory = None

    def start(self):
        geometry = screen_geometry(self.selections.get('screen_geometry', {'width': 1600, 'height': 1000}))
        prepare_environment = self.selections.get('prepare_environment')
        if not callable(prepare_environment):
            raise ValueError('Select an explicit application environment preparation hook.')
        self.output.mkdir(mode=0o700, parents=True, exist_ok=False)
        # Unix socket paths are bounded independently of an invocation's output depth.
        self.socket_directory = Path(tempfile.mkdtemp(prefix='dockpipe-bus-', dir='/tmp'))
        number = next((number for number in range(280, 330)
                       if not Path(f'/tmp/.X11-unix/X{number}').exists()
                       and not Path(f'/tmp/.X{number}-lock').exists()), None)
        if number is None:
            raise RuntimeError('No available private X11 display.')
        display = f':{number}'
        authority = self.output / 'Xauthority'
        authority.touch(mode=0o600, exist_ok=False)
        subprocess.run(['/usr/bin/xauth', '-f', str(authority), 'add', display,
                        'MIT-MAGIC-COOKIE-1', secrets.token_hex(16)], check=True,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=5)
        environment = {'PATH': '/usr/local/bin:/usr/bin:/bin', 'HOME': os.environ['HOME'],
                       'LANG': 'C.UTF-8', 'LC_ALL': 'C.UTF-8'}
        for key in ['LD_PRELOAD', 'LD_AUDIT', 'DISPLAY', 'XAUTHORITY', 'DBUS_SESSION_BUS_ADDRESS', 'AT_SPI_BUS_ADDRESS']:
            environment.pop(key, None)
        environment.update(DISPLAY=display, XAUTHORITY=str(authority), QT_QPA_PLATFORM='xcb',
                           QT_LINUX_ACCESSIBILITY_ALWAYS_ON='1', QT_ACCESSIBILITY='1',
                           GTK_MODULES='gail:atk-bridge', NO_AT_BRIDGE='0')
        for name in ['state', 'config', 'cache', 'data', 'runtime']:
            directory = self.output / name
            directory.mkdir(mode=0o700)
            environment['XDG_' + name.upper() + ('_DIR' if name == 'runtime' else '_HOME')] = str(directory)
        temporary = self.output / 'temp'
        temporary.mkdir(mode=0o700)
        environment['TMPDIR'] = str(temporary)
        prepare_environment(environment)
        if environment.get('DISPLAY') != display or environment.get('XAUTHORITY') != str(authority):
            raise ValueError('Application hooks must retain the owned display authority.')
        self.environment = environment
        with (self.output / 'xvfb.log').open('xb') as log:
            xvfb = subprocess.Popen([self.selections['xvfb'], display, '-screen', '0', geometry,
                                     '-nolisten', 'tcp', '-noreset', '-auth', str(authority)],
                                    env=environment, stdout=log, stderr=log, start_new_session=True)
        self.processes.append(xvfb)
        deadline = time.monotonic() + 5
        while not Path(f'/tmp/.X11-unix/X{number}').exists():
            if xvfb.poll() is not None or time.monotonic() >= deadline:
                raise RuntimeError('Private X11 server did not become ready.')
            time.sleep(0.05)
        bus = subprocess.run(['/usr/bin/dbus-daemon', '--session', '--fork',
                              '--address=unix:path=' + str(self.socket_directory / 'session-bus'),
                              '--print-address=1', '--print-pid=1'],
                             env=environment, capture_output=True, text=True, check=True, timeout=5)
        address, pid = bus.stdout.splitlines()
        environment['DBUS_SESSION_BUS_ADDRESS'] = address
        self.bus_pid = int(pid)
        self.bus_identities[self.bus_pid] = self.process_starttime(self.bus_pid)
        accessibility_bus = subprocess.run(['/usr/bin/dbus-daemon',
            '--config-file=/usr/share/defaults/at-spi2/accessibility.conf', '--fork',
            '--address=unix:path=' + str(self.socket_directory / 'accessibility-bus'),
            '--print-address=1', '--print-pid=1'], env=environment,
            capture_output=True, text=True, check=True, timeout=5)
        accessibility_address, accessibility_pid = accessibility_bus.stdout.splitlines()
        environment['AT_SPI_BUS_ADDRESS'] = accessibility_address
        self.accessibility_bus_pid = int(accessibility_pid)
        self.bus_identities[self.accessibility_bus_pid] = self.process_starttime(self.accessibility_bus_pid)
        with (self.output / 'accessibility.log').open('xb') as log:
            registry_environment = dict(environment, DBUS_SESSION_BUS_ADDRESS=accessibility_address)
            registry = subprocess.Popen(['/usr/libexec/at-spi2-registryd'], env=registry_environment,
                stdout=log, stderr=log, start_new_session=True)
        self.processes.append(registry)
        selected_workspace = self.selections.get('workspace_path')
        workspace = Path(selected_workspace) if selected_workspace else self.output / 'workspace'
        if selected_workspace:
            if not workspace.is_absolute() or workspace.is_symlink() or not workspace.is_dir():
                raise ValueError('Existing workspace must be an explicit absolute directory.')
        else:
            workspace.mkdir(mode=0o700)
            (workspace / 'dockpipe.config.json').write_text('{"schema":1}\n')
        settings = self.output / 'config/dockpipe/dockpipe-launcher'
        settings.mkdir(mode=0o700, parents=True)
        (settings / 'launcher.json').write_text(json.dumps({'uiMode': 'advanced', 'projectFolder': str(workspace),
            'defaultDashboardVisible': False, 'sidebarVisible': True, 'packageRemotes': [],
            'dashboardTour': {'completed': True, 'doNotShowAgain': True}}) + '\n')
        launcher = self.selections.get('normal_gui_launcher')
        if launcher is not None:
            if not callable(launcher):
                raise ValueError('Normal GUI launch hook must be callable.')
            gui = launcher(self, environment, workspace)
            if not isinstance(gui, subprocess.Popen):
                raise ValueError('Normal GUI launch hook must return its owned process.')
        else:
            with (self.output / 'gui.log').open('xb') as log:
                gui = subprocess.Popen([self.selections['gui'], '--allow-second-instance'], cwd=workspace,
                                       env=environment, stdout=log, stderr=log, start_new_session=True)
        self.processes.append(gui)
        (self.output / 'session-private.json').write_text(json.dumps({'environment': environment, 'gui_pid': gui.pid,
            'bus_pid': self.bus_pid, 'xvfb_pid': xvfb.pid, 'workspace': str(workspace)}, indent=2) + '\n')
        os.chmod(self.output / 'session-private.json', 0o600)
        return gui

    def stop(self):
        for process in reversed(self.processes):
            if process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=5)
        for pid, expected_starttime in reversed(self.bus_identities.items()):
            if self.process_starttime(pid) != expected_starttime:
                continue
            try:
                os.kill(pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
        if self.socket_directory is not None:
            remove_owned_socket_directory(self.socket_directory)
            self.socket_directory = None

    @staticmethod
    def process_starttime(pid):
        try:
            stat = Path('/proc', str(pid), 'stat').read_text()
        except FileNotFoundError:
            return None
        return stat.rsplit(')', 1)[1].split()[19]
