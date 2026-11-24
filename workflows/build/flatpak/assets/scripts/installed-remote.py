"""Normal installed Flatpak remote GUI qualification on an owned private display.

Preparation is offline. Remote lifecycle policy belongs to the pinned provider
fixture facade; this consumer owns only installed GUI identity and local input.
"""
from pathlib import Path
import argparse
import configparser
import functools
import hashlib
import http.server
import importlib.util
import json
import math
import os
import pwd
import select
import shutil
import signal
import ssl
import stat
import struct
import subprocess
import sys
import threading
import time


APP_ID = 'com.dockpipe.Dockpipe'


def require(condition, message):
    if not condition:
        raise ValueError(message)


def digest(path):
    value = hashlib.sha256()
    with Path(path).open('rb') as stream:
        while True:
            chunk = stream.read(65536)
            if not chunk:
                break
            value.update(chunk)
    return value.hexdigest()


def write(path, value):
    with Path(path).open('x') as stream:
        json.dump(value, stream, indent=2, allow_nan=False)
        stream.write('\n')
    Path(path).chmod(0o600)


def load_module(name, path):
    specification = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


def rectangle(value):
    require(isinstance(value, dict) and set(value) == {'x', 'y', 'width', 'height'}, 'Expected exact rectangle')
    require(all(type(number) in [int, float] and math.isfinite(number) for number in value.values())
            and value['width'] > 0 and value['height'] > 0, 'Invalid finite rectangle')
    return value


def inside(inner, outer, margin=0):
    return (outer['x'] + margin <= inner['x'] and outer['y'] + margin <= inner['y']
            and inner['x'] + inner['width'] <= outer['x'] + outer['width'] - margin
            and inner['y'] + inner['height'] <= outer['y'] + outer['height'] - margin)


def safe_focus_point(viewport, window, geometry):
    """Map a proven fit-mode viewer viewport into the exact native fixture.

    Both native rectangles are CoreGraphics top-left coordinates. No guessed
    screen dimensions, widget focus, or historical window geometry is accepted.
    """
    viewport = rectangle(viewport)
    window = rectangle(window)
    display = rectangle(geometry['display_rect'])
    fixture = rectangle(geometry['fixture_rect'])
    require(inside(viewport, window, 4), 'Viewer viewport escaped the admitted GUI window')
    require(inside(fixture, display, 4), 'Generated fixture is not wholly within the captured primary display')
    require(viewport['width'] >= 100 and viewport['height'] >= 100
            and fixture['width'] >= 100 and fixture['height'] >= 100, 'Viewer/fixture geometry is too small')
    scale = min(viewport['width'] / display['width'], viewport['height'] / display['height'])
    image = {'x': viewport['x'] + (viewport['width'] - display['width'] * scale) / 2,
             'y': viewport['y'] + (viewport['height'] - display['height'] * scale) / 2,
             'width': display['width'] * scale, 'height': display['height'] * scale}
    native_x = fixture['x'] + fixture['width'] / 2
    native_y = fixture['y'] + fixture['height'] / 2
    x = image['x'] + (native_x - display['x']) * scale
    y = image['y'] + (native_y - display['y']) * scale
    result = {'x': round(x), 'y': round(y)}
    require(image['x'] + 4 <= result['x'] < image['x'] + image['width'] - 4
            and image['y'] + 4 <= result['y'] < image['y'] + image['height'] - 4,
            'Safe focus point falls outside the actual fitted image')
    return result


def remote_preferences(path):
    settings = configparser.ConfigParser(interpolation=None)
    settings.read(path)
    values = settings['remote'] if settings.has_section('remote') else {}
    expected = {'desktopShareClipboard': 'false', 'desktopShareImages': 'false',
                'desktopShareFiles': 'false', 'desktopQuality': 'high', 'desktopFrameRate': '10',
                'desktopDecoder': 'software', 'desktopScale': 'fit'}
    require(all(values.get(name, '').lower() == value for name, value in expected.items()),
            'Normal GUI must save explicit safe capture/clipboard preferences before connection')
    require(values.get('desktopDisplaySource', 'existing') == 'existing'
            and values.get('desktopSwapControlCommand', 'false').lower() == 'false',
            'Only existing primary display and original Super-to-Command mapping are admitted')
    return {name: values[name] for name in expected}


def capture_owned_display(environment, redact=None):
    """Return owned-display pixels only in memory; persisted remote frames are forbidden."""
    result = subprocess.run(['/usr/bin/xwd', '-silent', '-root'], env=environment,
                            stdin=subprocess.DEVNULL, capture_output=True, check=True, timeout=5)
    require(len(result.stdout) <= 16 << 20, 'Owned display exceeded its fixed image bound')
    data = result.stdout
    require(len(data) >= 100, 'Invalid owned XWD image')
    header = struct.unpack('!25I', data[:100])
    require(header[1:6] == (7, 2, 24, 1600, 1000) and header[7] in [0, 1]
            and header[11] in [24, 32] and header[12] == 6400
            and header[14:17] == (0xff0000, 0xff00, 0xff), 'Unexpected owned X11 image tuple: ' + repr(header))
    offset = header[0] + header[19] * 12
    require(100 <= offset < len(data) and len(data) - offset == 6400 * 1000, 'Invalid bounded XWD payload')
    from PIL import Image, ImageDraw
    encoding = ('BGR' if header[11] == 24 else 'BGRX') if header[7] == 0 else ('RGB' if header[11] == 24 else 'XRGB')
    image = Image.frombytes('RGB', (1600, 1000), data[offset:], 'raw', encoding, 6400, 1)
    if redact is not None:
        value = rectangle(redact)
        ImageDraw.Draw(image).rectangle((value['x'], value['y'], value['x'] + value['width'],
                                        value['y'] + value['height']), fill='black')
    return image


def require_installed(configuration):
    require(configuration['app_id'] == APP_ID and configuration['branch'] == 'staging', 'Select the unchanged installed app')
    environment = {'PATH': '/usr/bin:/bin', 'HOME': str(configuration['private_home']),
                   'FLATPAK_USER_DIR': configuration['installation']}
    result = subprocess.run(['flatpak', 'info', '--user', '--show-location', APP_ID],
                            env=environment, capture_output=True, text=True, check=True, timeout=5)
    location = Path(result.stdout.strip())
    require(location.name == configuration['installed_commit'], 'Installed app commit changed')
    for name, relative in {'gui_sha256': 'libexec/dockpipe/dockpipe-launcher',
                           'ui_library_sha256': 'libexec/dockpipe/libdockpipe-launcher-ui.so',
                           'cli_sha256': 'libexec/dockpipe/dockpipe'}.items():
        require(digest(location / 'files' / relative) == configuration[name], 'Installed artifact changed: ' + name)
    return location


def inside_package_install(path):
    """Install from an isolated runtime-local TLS catalog with the unchanged /app CLI."""
    specification = json.loads(Path(path).read_text())
    output = Path(specification['output'])
    site = Path(specification['site'])
    export_environment = dict(os.environ, DOCKPIPE_PACKAGES_ROOT=specification['compiled_store'])
    with (output / 'export.stdout').open('xb') as stdout, (output / 'export.stderr').open('xb') as stderr:
        exported = subprocess.run(['/app/bin/dockpipe', 'package', 'build', 'store', '--out', str(site),
                                   '--only', 'resolvers'], env=export_environment,
                                  cwd=specification['producer'], stdin=subprocess.DEVNULL,
                                  stdout=stdout, stderr=stderr, timeout=45)
    require(exported.returncode == 0, 'Installed platform-owned provider catalog export failed')
    manifest = json.loads((site / 'packages-store-manifest.json').read_text())
    entries = [row for row in manifest['packages']['resolvers'] if row['name'] == 'dockpipe.desktop.native']
    require(len(entries) == 1 and entries[0]['version'] == '0.1.0', 'Expected one qualified provider archive')
    selected = entries[0]
    require(digest('/app/libexec/dockpipe/dockpipe') == specification['cli_sha256'], 'Installed CLI identity changed')

    class Handler(http.server.SimpleHTTPRequestHandler):
        def do_GET(self):
            if self.path not in self.server.allowed:
                self.send_error(404)
                return
            self.server.requests.append(self.path)
            if len(self.server.requests) > 32:
                self.send_error(503)
                return
            super().do_GET()

        def log_message(self, _format, *_arguments):
            pass

    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.load_cert_chain(specification['certificate'], specification['key'])
    server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), functools.partial(Handler, directory=str(site)))
    server.socket = context.wrap_socket(server.socket, server_side=True)
    server.allowed = {'/packages-store-manifest.json', '/' + selected['tarball']}
    server.requests = []
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    remote = 'https://127.0.0.1:' + str(server.server_port) + '/packages-store-manifest.json'
    environment = dict(os.environ, SSL_CERT_FILE=specification['certificate'],
                       DOCKPIPE_GLOBAL_ROOT=specification['global_root'],
                       DOCKPIPE_PACKAGES_ROOT=specification['package_root'])

    def invoke(name, arguments):
        with (output / (name + '.stdout')).open('xb') as stdout, (output / (name + '.stderr')).open('xb') as stderr:
            os.chmod(stdout.name, 0o600)
            os.chmod(stderr.name, 0o600)
            result = subprocess.run(['/app/bin/dockpipe', *arguments], env=environment,
                cwd=specification['workspace'], stdin=subprocess.DEVNULL, stdout=stdout, stderr=stderr, timeout=45)
        require(result.returncode == 0 and Path(stdout.name).stat().st_size <= 4 << 20
                and Path(stderr.name).stat().st_size <= 65536, 'Installed package command failed: ' + name)
        return json.loads(Path(stdout.name).read_text())

    try:
        invoke('remote-catalog', ['package', 'catalog', '--remote', remote])
        invoke('install', ['package', 'install', '--remote', remote, '--kind', 'resolver',
                          '--name', selected['name'], '--sha256', selected['sha256']])
        inventory = invoke('installed-packages', ['package', 'list', '--format', 'json'])
        catalog = invoke('installed-catalog', ['catalog', 'list', '--format', 'json'])
        resolvers = [row for row in catalog['resolver_details'] if row.get('name') == selected['name']
                     and row.get('version') == '0.1.0' and row.get('capability') == 'remote.desktop']
        require(len(resolvers) == 1, 'Installed Flatpak provider did not enter the normal catalog')
        display = resolvers[0]['surface']['native_display']
        module = Path(display['libraries']['linux'])
        require(display['interface'] == 1 and digest(module) == specification['module_sha256'],
                'Normal catalog selected another provider module')
        write(output / 'provider-installed.json', {'schema': 'dockpipe.flatpak.remote-provider-installed/v1',
            'provider': resolvers[0], 'module_sha256': digest(module), 'archive_sha256': selected['sha256'],
            'tls_requests': server.requests, 'cli_sha256': specification['cli_sha256'],
            'live_remote_effects': False, 'network_namespace': os.readlink('/proc/self/ns/net')})
    finally:
        server.shutdown()
        thread.join(timeout=2)
        server.server_close()


def owner_modules():
    repository = Path(__file__).resolve().parents[5]
    scripts = repository / 'workflows/qualify/product/assets/scripts'
    sys.path.insert(0, str(scripts))
    sys.path.insert(0, str(repository / 'workflows/qualify/product/drivers'))
    import contracts
    import process
    import portable
    gui = load_module('installed_remote_gui_owner', Path(__file__).with_name('installed-product-gui.py'))
    observer = load_module('installed_remote_cli_owner', Path(__file__).with_name('installed-cli-process.py'))
    return repository, contracts, process, portable, gui, observer


def pinned(record):
    require(isinstance(record, dict) and set(record) == {'path', 'sha256'}, 'Expected exact local artifact pin')
    path = Path(record['path'])
    require(path.is_absolute() and not path.is_symlink() and path.is_file()
            and digest(path) == record['sha256'], 'Selected artifact changed')
    return path


def private_launcher(command, environment, configuration, private_home, writable, readonly_sockets=()):
    """Further isolate the launcher without moving the canonical OS authority.

    Flatpak creates its app data directory before applying child environment
    overrides. Cover the account home in a new mount namespace, and bind only
    the already selected authority/state directories back at their original
    canonical paths. Descriptor mounts survive the private-home cover.
    """
    account_home = Path(pwd.getpwuid(os.getuid()).pw_dir)
    private_home = Path(private_home).resolve(strict=True)
    require(account_home.is_absolute() and account_home != Path('/')
            and not private_home.is_relative_to(account_home), 'Launcher backing must be outside the real account home')
    configuration = dict(configuration)
    security = account_home / '.local/state/dockpipe/security'
    state = Path(configuration['provider_configuration']['state'])
    require(state.is_absolute() and state.resolve(strict=True) == state
            and security.resolve(strict=True) == security, 'Canonical paired state/security references required')
    descriptors = []
    try:
        mounts = []
        for path in [state, security]:
            descriptor = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            descriptors.append(descriptor)
            if path.is_relative_to(account_home):
                destination = private_home / path.relative_to(account_home)
                destination.mkdir(mode=0o700, parents=True, exist_ok=True)
            mounts.extend(['--ro-bind-fd', str(descriptor), str(path)])
        wrapper = [str(pinned(configuration['bwrap'])), '--die-with-parent', '--unshare-user', '--unshare-pid',
                   '--uid', str(os.getuid()), '--gid', str(os.getgid()), '--ro-bind', '/', '/',
                   '--bind', str(private_home), str(account_home), '--proc', '/proc', '--dev', '/dev',
                   '--tmpfs', '/tmp']
        for path in writable:
            path = Path(path).resolve(strict=True)
            require(not path.is_relative_to(account_home), 'Writable launcher paths must be task-owned outside real home')
            descriptor = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            descriptors.append(descriptor)
            wrapper.extend(['--bind-fd', str(descriptor), str(path)])
        require(len(readonly_sockets) <= 1, 'Only the exact owned Xvfb socket may supplement private runtime mounts')
        for path in readonly_sockets:
            path = Path(path)
            require(path.parent == Path('/tmp/.X11-unix') and path.name.startswith('X')
                    and path.name[1:].isdigit() and not path.is_symlink(),
                    'Select only one concrete owned local X11 socket')
            descriptor = os.open(path, os.O_PATH | os.O_NOFOLLOW)
            descriptors.append(descriptor)
            require(stat.S_ISSOCK(os.fstat(descriptor).st_mode), 'Selected X11 object is not a socket')
            wrapper.extend(['--ro-bind-fd', str(descriptor), str(path)])
        installation = Path(configuration['installation']).resolve(strict=True)
        require(not installation.is_relative_to(account_home), 'Private installation must be outside real home')
        descriptor = os.open(installation, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        descriptors.append(descriptor)
        wrapper.extend(['--ro-bind-fd', str(descriptor), str(installation)])
        wrapper.extend(mounts)
        wrapper.extend(['--', *command])
        selected_environment = dict(environment, HOME=str(account_home))
        return wrapper, selected_environment, descriptors
    except BaseException:
        for descriptor in descriptors:
            os.close(descriptor)
        raise


def host_profile_identity():
    path = Path(pwd.getpwuid(os.getuid()).pw_dir) / '.var/app' / APP_ID
    try:
        value = path.lstat()
    except FileNotFoundError:
        return {'exists': False}
    return {'exists': True, 'device': value.st_dev, 'inode': value.st_ino, 'mode': value.st_mode,
            'uid': value.st_uid, 'mtime_ns': value.st_mtime_ns, 'ctime_ns': value.st_ctime_ns}


def readonly_mount(path):
    candidates = []
    for line in Path('/proc/self/mountinfo').read_text().splitlines():
        fields = line.split()
        point = Path(fields[4].replace('\\040', ' '))
        if path == point or path.is_relative_to(point):
            candidates.append((len(point.parts), fields[5].split(',')))
    require(candidates, 'Selected mount is not observable')
    return 'ro' in max(candidates, key=lambda item: item[0])[1]


def inside_launcher_preflight(path):
    selected = json.loads(Path(path).read_text())
    configuration = selected['configuration']
    identity = configparser.ConfigParser(interpolation=None)
    identity.read('/.flatpak-info')
    require(identity.get('Application', 'name') == APP_ID
            and identity.get('Instance', 'app-commit') == configuration['installed_commit'],
            'Preflight is not inside the selected genuine app deployment')
    require(os.getuid() == selected['uid'] and pwd.getpwuid(os.getuid()).pw_dir == selected['account_home'],
            'Canonical account identity changed')
    for name, relative in {'gui_sha256': 'dockpipe-launcher', 'cli_sha256': 'dockpipe',
                           'ui_library_sha256': 'libdockpipe-launcher-ui.so'}.items():
        require(digest(Path('/app/libexec/dockpipe') / relative) == configuration[name], 'Preflight B artifact changed')
    mounts = {}
    for name in ['state', 'security']:
        directory = Path(selected[name])
        actual = directory.stat()
        expected = selected['mount_identity'][name]
        require([actual.st_dev, actual.st_ino] == expected and readonly_mount(directory),
                'Selected canonical authority/state mount is not the exact read-only object')
        mounts[name] = {'path': str(directory), 'readonly': True, 'object_identity_verified': True}
    authority = subprocess.run(['/app/bin/dockpipe', 'remote', 'security-status', '--auth', 'never'],
        stdin=subprocess.DEVNULL, capture_output=True, text=True, check=True, timeout=8)
    require(len(authority.stdout) <= 65536, 'Public authority metadata exceeded bound')
    status = json.loads(authority.stdout)
    require(status.get('authority_available') is True and status.get('locked') is False,
            'Canonical OS authority is unavailable; no prompt or policy workaround allowed')
    require(os.environ['HOME'] == selected['child_home'], 'Contained HOME must stay private')
    profile = {name: os.environ.get(name, '') for name in ['HOME', 'XDG_CONFIG_HOME', 'XDG_DATA_HOME',
                                                          'XDG_CACHE_HOME', 'XDG_STATE_HOME', 'XDG_RUNTIME_DIR']}
    write(Path(selected['output']) / 'observed-profile.json', profile)
    private_app = str(Path(selected['account_home']) / '.var/app' / APP_ID)
    app_object = Path(private_app).stat()
    require([app_object.st_dev, app_object.st_ino] == selected['private_app_identity'],
            'Contained app home no longer maps to the new private backing')
    require(all(value.startswith(selected['output'] + '/') or value.startswith(private_app + '/')
                for name, value in profile.items() if name != 'XDG_RUNTIME_DIR'),
            'Contained writable profiles must remain in task output or covered private app home')
    runtime = Path(profile['XDG_RUNTIME_DIR'])
    runtime_identity = runtime.stat()
    require([runtime_identity.st_dev, runtime_identity.st_ino] != selected['host_runtime_identity']
            and not readonly_mount(runtime) and runtime_identity.st_uid == os.getuid()
            and runtime_identity.st_mode & 0o077 == 0, 'Contained runtime must be a new private writable mount')
    result = {'schema': 'dockpipe.flatpak.remote-launcher-preflight/v1', 'uid': os.getuid(),
              'account_home': selected['account_home'], 'child_profile': profile, 'mounts': mounts,
              'mount_namespace': os.readlink('/proc/self/ns/mnt'), 'authority_available': True,
              'locked': False, 'app_commit': configuration['installed_commit'],
              'cli_sha256': configuration['cli_sha256'], 'network_shared': False,
              'gui_started': False, 'capture_or_input_started': False}
    write(Path(selected['output']) / 'inside-preflight.json', result)


def launcher_preflight(configuration, output):
    _repository, _contracts, process, _portable, _gui, _observer = owner_modules()
    output.mkdir(mode=0o700)
    private_home = output / 'private-home'
    private_home.mkdir(mode=0o700)
    environment = profile_environment(private_home, configuration['installation'])
    private_app = private_home / '.var/app' / APP_ID
    private_app.mkdir(mode=0o700, parents=True)
    runtime = output / 'runtime'
    runtime.mkdir(mode=0o700)
    environment['XDG_RUNTIME_DIR'] = str(runtime)
    account_home = Path(pwd.getpwuid(os.getuid()).pw_dir)
    state = Path(configuration['provider_configuration']['state'])
    security = account_home / '.local/state/dockpipe/security'
    script = output / 'installed-remote.py'
    shutil.copyfile(__file__, script)
    request = output / 'preflight-request.json'
    selected = {'configuration': configuration, 'uid': os.getuid(), 'account_home': str(account_home),
                'state': str(state), 'security': str(security), 'child_home': environment['HOME'],
                'output': str(output),
                'private_app_identity': [private_app.stat().st_dev, private_app.stat().st_ino],
                'host_runtime_identity': [Path('/run/user/' + str(os.getuid())).stat().st_dev,
                                          Path('/run/user/' + str(os.getuid())).stat().st_ino],
                'mount_identity': {name: [path.stat().st_dev, path.stat().st_ino]
                                                       for name, path in [('state', state), ('security', security)]}}
    write(request, selected)
    command = ['flatpak', 'run', '--user', '--branch=' + configuration['branch'], '--unshare=network',
               '--nofilesystem=host', '--nofilesystem=home', '--nosocket=x11', '--nosocket=wayland',
               '--nosocket=session-bus', '--nosocket=system-bus', '--no-a11y-bus',
               '--no-talk-name=org.freedesktop.Flatpak', '--filesystem=' + str(output),
               '--filesystem=' + str(state) + ':ro', '--filesystem=' + str(security) + ':ro']
    command.extend('--env=' + name + '=' + value for name, value in environment.items()
                   if name in ['HOME', 'XDG_CONFIG_HOME', 'XDG_DATA_HOME', 'XDG_CACHE_HOME', 'XDG_STATE_HOME', 'XDG_RUNTIME_DIR', 'TMPDIR'])
    command.extend(['--command=/usr/bin/python3', APP_ID, '-I', str(script),
                    '--inside-launcher-preflight', str(request)])
    wrapper, launcher_environment, descriptors = private_launcher(command, environment, configuration,
                                                                  private_home, [output])
    baseline = host_profile_identity()
    write(output / 'launcher-command.json', {'arguments': wrapper, 'source_sha256': digest(script),
                                           'host_mount_namespace': os.readlink('/proc/self/ns/mnt')})
    logs = output / 'process'
    logs.mkdir(mode=0o700)
    # run_driver deliberately closes inherited descriptors; directory descriptor
    # mounts are explicit for this one launch, rather than ambient FD discovery.
    with (logs / 'stdout').open('xb') as stdout, (logs / 'stderr').open('xb') as stderr:
        try:
            result = subprocess.run(wrapper, env=launcher_environment, stdin=subprocess.DEVNULL,
                stdout=stdout, stderr=stderr, pass_fds=descriptors, start_new_session=True, timeout=30)
        finally:
            for descriptor in descriptors:
                os.close(descriptor)
    require(host_profile_identity() == baseline, 'Real default app profile metadata changed')
    require(result.returncode == 0, 'Read-only launcher preflight failed; inspect original output, no workaround')
    facts = json.loads((output / 'inside-preflight.json').read_text())
    require(facts['mount_namespace'] != os.readlink('/proc/self/ns/mnt'), 'Preflight did not enter a new mount namespace')
    facts['host_default_profile_unchanged'] = True
    facts['host_default_profile_baseline'] = baseline
    write(output / 'preflight-result.json', facts)
    return facts


def profile_environment(home, installation):
    home.mkdir(mode=0o700, exist_ok=True)
    environment = {'PATH': '/usr/local/bin:/usr/bin:/bin', 'LANG': 'C.UTF-8', 'LC_ALL': 'C.UTF-8',
                   'HOME': str(home), 'FLATPAK_USER_DIR': str(installation), 'FLATPAK_GL_DRIVERS': 'default',
                   'DOCKPIPE_VAULT_INJECT': '0', 'DOCKPIPE_INTERACTIVE': '0'}
    for name in ['config', 'data', 'cache', 'state', 'tmp']:
        path = home / name
        path.mkdir(mode=0o700, exist_ok=True)
        environment['TMPDIR' if name == 'tmp' else 'XDG_' + name.upper() + '_HOME'] = str(path)
    return environment


def verify_configuration(request):
    require(request['target']['kind'] == 'flatpak'
            and request['target']['platform'] == 'linux-amd64-flatpak-kde6.10', 'Select the native installed Flatpak target')
    configuration = request['configuration']
    required = {'bundle', 'installation', 'app_id', 'branch', 'installed_commit', 'gui_sha256',
                'ui_library_sha256', 'cli_sha256', 'runtime_commit', 'catalog_compiler', 'qualified_provider',
                'tls_certificate', 'tls_key', 'xvfb', 'bwrap', 'mode', 'provider_facade', 'provider_configuration'}
    require(required <= set(configuration) <= required | {'prepared_profile', 'accessibility_probe'}, 'Unknown installed remote configuration fields')
    require(configuration['mode'] in ['preflight', 'prepare', 'paired-prepare', 'live'], 'Select explicit preflight, offline preparation or reviewed live mode')
    require(digest(pinned(configuration['bundle'])) == request['target']['artifact_sha256'], 'Bundle identity differs from target')
    for name in ['catalog_compiler', 'tls_certificate', 'tls_key', 'xvfb', 'bwrap']:
        pinned(configuration[name])
    require(Path(configuration['tls_key']['path']).stat().st_mode & 0o077 == 0, 'Generated fixture TLS key must remain private')
    require(Path(configuration['installation']).is_absolute(), 'Select an explicit existing private Flatpak installation')
    provider = configuration['qualified_provider']
    require(set(provider) == {'root', 'inventory', 'producer', 'module_sha256'}, 'Expected qualified provider artifacts')
    inventory = json.loads(pinned(provider['inventory']).read_text())
    producer = json.loads(pinned(provider['producer']).read_text())
    require(producer.get('sdk_tests_run') is True and producer.get('platform_tests_run') is True
            and producer['profile']['profile'] == 'linux-amd64-flatpak-org.kde.Platform-6.10',
            'Provider must have completed its owning SDK and smaller Platform qualification')
    source = Path(provider['root'])
    require(source.is_absolute() and source.resolve(strict=True) == source, 'Select canonical qualified resolver root')
    expected = {row['path']: row['sha256'] for row in inventory['files']}
    actual = {path.relative_to(source).as_posix() for path in source.rglob('*') if path.is_file()}
    require(actual == set(expected) and len(expected) <= 64, 'Qualified provider inventory changed')
    for name, checksum in expected.items():
        path = source / name
        require(not path.is_symlink() and path.resolve(strict=True).is_relative_to(source)
                and digest(path) == checksum, 'Qualified provider payload differs: ' + name)
    require(digest(source / 'assets/display/libdockpipe-native-desktop-display.so') == provider['module_sha256'],
            'Qualified module identity changed')
    return configuration


def prepare_provider(configuration, output):
    """Compile/export the proper Flatpak resolver, then install through public B CLI."""
    _repository, _contracts, process, portable, _gui, observer_module = owner_modules()
    output.mkdir(mode=0o700)
    compiler = pinned(configuration['catalog_compiler'])
    environment = profile_environment(output / 'compiler-home', configuration['installation'])
    environment['DOCKPIPE_PACKAGES_ROOT'] = str(output / 'compiled-store')
    producer = output / 'producer'
    producer.mkdir(mode=0o700)
    (producer / 'dockpipe.config.json').write_text('{}\n')
    sources = producer / 'sources'
    sources.mkdir()
    copied = sources / 'dockpipe.desktop.native'
    shutil.copytree(configuration['qualified_provider']['root'], copied)
    inventory = json.loads(Path(configuration['qualified_provider']['inventory']['path']).read_text())
    for row in inventory['files']:
        require(digest(copied / row['path']) == row['sha256'], 'Copied provider payload changed')
    require(portable.command(compiler, ['package', 'compile', 'resolvers', '--from', str(sources)],
                            producer, environment, output, 'compile-provider'), 'Provider compile failed')
    site = output / 'catalog-site'
    global_root = output / 'private-user-data'
    global_root.mkdir(mode=0o700)
    package_root = global_root / 'packages'
    installer_output = output / 'installed'
    installer_output.mkdir(mode=0o700)
    workspace = output / 'consumer'
    workspace.mkdir(mode=0o700)
    (workspace / 'dockpipe.config.json').write_text('{}\n')
    script = output / 'installed-remote.py'
    shutil.copyfile(__file__, script)
    script.chmod(0o600)
    specification = {'output': str(installer_output), 'site': str(site), 'global_root': str(global_root),
                     'compiled_store': environment['DOCKPIPE_PACKAGES_ROOT'], 'producer': str(producer),
                     'package_root': str(package_root), 'workspace': str(workspace),
                     'certificate': configuration['tls_certificate']['path'], 'key': configuration['tls_key']['path'],
                     'cli_sha256': configuration['cli_sha256'],
                     'module_sha256': configuration['qualified_provider']['module_sha256']}
    specification_path = output / 'install-request.json'
    write(specification_path, specification)
    environment = profile_environment(output / 'installer-home', configuration['installation'])
    runtime = output / 'installer-runtime'
    runtime.mkdir(mode=0o700)
    environment['XDG_RUNTIME_DIR'] = str(runtime)
    command = ['flatpak', 'run', '--user', '--branch=' + configuration['branch'], '--unshare=network',
               '--nofilesystem=host', '--nofilesystem=home', '--nosocket=x11', '--nosocket=wayland',
               '--nosocket=session-bus', '--nosocket=system-bus', '--no-a11y-bus',
               '--no-talk-name=org.freedesktop.Flatpak',
               '--filesystem=' + str(output),
               '--filesystem=' + configuration['tls_certificate']['path'] + ':ro',
               '--filesystem=' + configuration['tls_key']['path'] + ':ro']
    command.extend('--env=' + name + '=' + value for name, value in environment.items()
                   if name in ['HOME', 'XDG_CONFIG_HOME', 'XDG_DATA_HOME', 'XDG_CACHE_HOME', 'XDG_STATE_HOME', 'XDG_RUNTIME_DIR', 'TMPDIR'])
    command.extend(['--command=/usr/bin/python3', APP_ID, '-I', str(script),
                    '--inside-package-install', str(specification_path)])
    logs = output / 'installed-process'
    logs.mkdir(mode=0o700)
    observer = observer_module.InstalledCliProcessObserver(command, configuration['installed_commit'], configuration['cli_sha256'])
    result = process.run_driver(command, logs, 120, environment, output, observe=observer.observe)
    journal = observer.finish()
    write(logs / 'process.json', result)
    write(logs / 'kernel-journal.json', journal)
    require(result['exit_code'] == 0 and not result['failure'] and journal['owned_processes_closed']
            and journal['contained_cli_observed'], 'Normal installed CLI/package fixture or exact process cleanup failed')
    facts = json.loads((installer_output / 'provider-installed.json').read_text())
    return {'package_root': str(package_root), 'workspace': str(workspace), 'installation': facts,
            'catalog_sha256': digest(installer_output / 'installed-catalog.stdout')}


class FacadeClient:
    """Consumer of the explicit package workflow protocol; no endpoint policy here."""
    def __init__(self, selection, configuration, output, protocol='legacy', wait_process=None):
        self.wait_process = wait_process
        self.wait_process_pid = None
        if wait_process is not None:
            require(isinstance(wait_process, subprocess.Popen) and wait_process.poll() is None,
                    'Facade wait progress requires its exact live owned Popen child')
            process_root = Path('/proc') / str(wait_process.pid)
            fields = (process_root / 'stat').read_text().rsplit(')', 1)[1].split()
            require(int(fields[1]) == os.getpid() and process_root.stat().st_uid == os.getuid(),
                    'Facade may reap only this controller\'s direct owned child')
            self.wait_process_pid = wait_process.pid
        protocols = {
            'legacy': ('dockpipe.desktop.fixture-facade/v1', '--fixture-facade'),
            'nonce': ('dockpipe.desktop.nonce-fixture-facade/v1', '--nonce-fixture-facade'),
            'nonce-v2': ('dockpipe.desktop.nonce-fixture-facade/v2', '--nonce-fixture-facade-v2'),
        }
        require(isinstance(protocol, str) and protocol in protocols,
                'Select one exact versioned provider facade protocol')
        self.expected_schema, self.protocol_argument = protocols[protocol]
        self.sequence = 1
        self.buffer = b''
        self.output = output
        output.mkdir(mode=0o700)
        cli = pinned(selection['cli'])
        archive = pinned(selection['archive'])
        member = selection['member']
        require(member and not member.startswith('/') and '..' not in Path(member).parts
                and '\\' not in member, 'Select exact contained facade workflow member')
        request = {'schema': self.expected_schema, 'run_id': output.parent.name,
                   'configuration': configuration}
        path = output / 'request.json'
        write(path, request)
        self.errors = (output / 'workflow.stderr').open('xb')
        command = [str(cli), '--workflow-file', 'tar://' + str(archive) + '##' + member, '--no-vault', '--',
                   self.protocol_argument, '--request', str(path), '--output', str(output / 'owner'),
                   '--execute-reviewed', '--approve-request-sha256', digest(path)]
        environment = profile_environment(output / 'profile', selection.get('installation', '/unused-private-installation'))
        environment.update(PYTHONDONTWRITEBYTECODE='1')
        write(output / 'environment.json', environment)
        self.process = subprocess.Popen(command, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
            stderr=self.errors, start_new_session=True, cwd=output, env=environment)
        self.events = []
        try:
            ready = self.event(15)
            require(ready.get('schema') == self.expected_schema and ready.get('stage') == 'ready'
                    and ready.get('check_only') is False, 'Fixture facade did not admit explicit reviewed protocol')
        except BaseException:
            self.stop()
            raise

    def event(self, seconds):
        deadline = time.monotonic() + seconds
        while b'\n' not in self.buffer:
            wait_process = getattr(self, 'wait_process', None)
            if wait_process is not None:
                require(wait_process.pid == self.wait_process_pid,
                        'Exact owned wait child identity changed')
                # Popen.poll uses waitpid(exact_pid, WNOHANG); it neither waits
                # for foreign children nor signals a process or process group.
                wait_process.poll()
            remaining = deadline - time.monotonic()
            require(remaining > 0, 'Fixture facade response deadline elapsed')
            interval = min(remaining, 0.02) if wait_process is not None else remaining
            ready, _, _ = select.select([self.process.stdout], [], [], interval)
            if not ready:
                continue
            require(time.monotonic() < deadline, 'Fixture facade response deadline elapsed')
            chunk = os.read(self.process.stdout.fileno(), 4096)
            require(chunk, 'Fixture facade ended; original cleanup remains unproven')
            self.buffer += chunk
            require(len(self.buffer) <= 65536, 'Fixture facade reply exceeded bound')
        line, self.buffer = self.buffer.split(b'\n', 1)
        event = json.loads(line)
        require(isinstance(event, dict), 'Invalid fixture facade reply')
        return event

    def command(self, action, **arguments):
        request = {'sequence': self.sequence, 'action': action, **arguments}
        self.process.stdin.write((json.dumps(request) + '\n').encode())
        self.process.stdin.flush()
        event = self.event(50)
        require(event.get('schema') == self.expected_schema
                and event.get('sequence') == self.sequence and event.get('action') == action
                and isinstance(event.get('result'), dict), 'Fixture facade response binding changed')
        write(self.output / ('reply-%02d.json' % self.sequence), event)
        self.sequence += 1
        return event['result']

    def stop(self):
        # EOF lets the owning facade reconcile only its original references.
        # An unknown result is preserved; no command is replayed here.
        if self.process.stdin is not None and not self.process.stdin.closed:
            try:
                self.process.stdin.close()
            except BrokenPipeError:
                pass
        forced = False
        try:
            self.process.wait(timeout=25)
        except subprocess.TimeoutExpired:
            forced = True
            os.killpg(self.process.pid, signal.SIGTERM)
            try:
                self.process.wait(timeout=2)
            except subprocess.TimeoutExpired:
                os.killpg(self.process.pid, signal.SIGKILL)
                self.process.wait(timeout=1)
        finally:
            self.errors.close()
            if self.process.stdout is not None:
                self.process.stdout.close()
        write(self.output / 'workflow-exit.json', {'exit_code': self.process.returncode,
              'forced_local_termination': forced, 'remote_cleanup_claimed': False})
        require(not forced, 'Provider facade exceeded its cleanup bound; preserve unknown original effects')


def private_accessibility_facts(environment, identity=None):
    """Read only the service selected by this fixture's own session bus."""
    import dbus
    facts = {'selected_environment_address': environment['AT_SPI_BUS_ADDRESS']}
    try:
        session = dbus.bus.BusConnection(environment['DBUS_SESSION_BUS_ADDRESS'])
        service = session.get_object('org.a11y.Bus', '/org/a11y/bus')
        address = str(dbus.Interface(service, 'org.a11y.Bus').GetAddress(timeout=3))
        facts['standard_address'] = address
        daemon = dbus.Interface(session.get_object('org.freedesktop.DBus', '/org/freedesktop/DBus'),
                                'org.freedesktop.DBus')
        service_pid = int(daemon.GetConnectionUnixProcessID('org.a11y.Bus', timeout=3))
        facts['service_pid'] = service_pid
        root = Path('/proc') / str(service_pid)
        allowed = {b'XDG_RUNTIME_DIR', b'DISPLAY', b'DBUS_SESSION_BUS_ADDRESS', b'AT_SPI_BUS_ADDRESS', b'HOME'}
        facts['service_environment'] = {}
        for entry in (root / 'environ').read_bytes().split(b'\0'):
            name, separator, value = entry.partition(b'=')
            if separator and name in allowed:
                facts['service_environment'][name.decode()] = value.decode()
        facts['service_mount_namespace'] = os.readlink(root / 'ns/mnt')
        require(address.startswith('unix:path='), 'Private bus must use a scoped filesystem socket')
        socket = Path(address.removeprefix('unix:path=').split(',', 1)[0])
        facts['socket_path'] = str(socket)
        facts['socket_exists'] = socket.exists()
        if socket.exists():
            value = socket.stat()
            facts['socket_identity'] = [value.st_dev, value.st_ino]
        if identity is not None:
            child_socket = Path('/proc') / str(identity['host_pid']) / 'root' / str(socket).lstrip('/')
            facts['socket_visible_in_actual_app_namespace'] = child_socket.exists()
            if child_socket.exists():
                value = child_socket.stat()
                facts['child_socket_identity'] = [value.st_dev, value.st_ino]
        connection = dbus.bus.BusConnection(address)
        selected = dbus.Interface(connection.get_object('org.freedesktop.DBus', '/org/freedesktop/DBus'),
                                  'org.freedesktop.DBus')
        facts['listener_pid'] = int(selected.GetConnectionUnixProcessID('org.freedesktop.DBus', timeout=3))
        facts['listener_responding'] = True
    except (dbus.DBusException, OSError, ValueError) as error:
        facts['error'] = str(error)
        facts['listener_responding'] = False
    return facts


def inspect_accessibility(path):
    """Disposable read-only native observer; never infer geometry from top-level bounds."""
    import resource
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
    selected = json.loads(Path(path).read_text())
    environment = selected['environment']
    require(isinstance(environment, dict) and len(environment) <= 64
            and all(isinstance(value, str) and len(value) <= 4096 for value in environment.values()),
            'Expected bounded private observer environment')
    os.environ.clear()
    os.environ.update(environment)
    _repository, _contracts, _process, _portable, gui, _observer = owner_modules()
    actual = gui.admitted_child(environment, selected['identity']['commit'],
                                selected['identity']['executable_sha256'], selected['owner_pid'])
    require(actual == selected['identity'], 'Actual GUI changed before private accessibility observation')
    import pyatspi
    applications = list(pyatspi.Registry.getDesktop(0))
    require(len(applications) <= 64, 'Private accessibility application list exceeded bound')
    allowed = set(actual['namespace_pids']) | {actual['host_pid']}
    matching = [application for application in applications if application.get_process_id() in allowed]
    facts = {'schema': 'dockpipe.flatpak.remote-accessibility-observation/v1',
             'applications': [{'pid': app.get_process_id(), 'name': app.name} for app in applications],
             'admitted_application_count': len(matching), 'native_viewer_geometry_proven': False}
    write(Path(selected['output']) / 'accessibility-result.json', facts)


def observe_accessibility(output, environment, identity, owner_pid):
    request = output / 'accessibility-request.json'
    write(request, {'environment': environment, 'identity': identity,
                    'owner_pid': owner_pid, 'output': str(output)})
    before = private_accessibility_facts(environment, identity)
    write(output / 'accessibility-observer-before.json', before)
    observations = []
    finished = threading.Event()

    def monitor():
        while not finished.is_set() and len(observations) < 160:
            sample = {'monotonic_ns': time.monotonic_ns()}
            socket_path = before.get('socket_path')
            if socket_path:
                try:
                    value = Path(socket_path).stat()
                    sample['socket_identity'] = [value.st_dev, value.st_ino]
                except FileNotFoundError:
                    sample['socket_absent'] = True
                lines = Path('/proc/net/unix').read_text().splitlines()
                sample['matching_kernel_listeners'] = [line.split()[:7] for line in lines
                                                       if line.endswith(' ' + socket_path)][:8]
            listener_pid = before.get('listener_pid')
            if listener_pid:
                try:
                    sample['listener_starttime'] = (Path('/proc') / str(listener_pid) / 'stat').read_text().rsplit(')', 1)[1].split()[19]
                except FileNotFoundError:
                    sample['listener_absent'] = True
            observations.append(sample)
            finished.wait(0.1)

    monitor_thread = threading.Thread(target=monitor, daemon=True)
    monitor_thread.start()
    with (output / 'accessibility.stdout').open('xb') as stdout, (output / 'accessibility.stderr').open('xb') as stderr:
        try:
            command = ['/usr/bin/strace', '-e', 'trace=connect', '-o', str(output / 'accessibility-connect.trace'),
                       '/usr/bin/python3', '-I', str(Path(__file__).resolve()),
                       '--inspect-accessibility', str(request)]
            process = subprocess.run(command, stdin=subprocess.DEVNULL, stdout=stdout, stderr=stderr,
                                     env=environment, timeout=12)
            status = {'exit_code': process.returncode, 'deadline_elapsed': False}
        except subprocess.TimeoutExpired:
            status = {'exit_code': None, 'deadline_elapsed': True}
        finally:
            finished.set()
            monitor_thread.join(timeout=2)
    write(output / 'accessibility-listener-monitor.json', observations)
    write(output / 'accessibility-observer-after.json', private_accessibility_facts(environment, identity))
    write(output / 'accessibility-observer-status.json', status)


def owned_descendants(wrapper_pid):
    """Only descendants of this invocation's live wrapper; never a process scan."""
    pending = [wrapper_pid]
    found = {}
    while pending:
        pid = pending.pop(0)
        if pid in found:
            continue
        require(len(found) < 256, 'Owned GUI descendant list exceeded its bound')
        root = Path('/proc') / str(pid)
        try:
            fields = (root / 'stat').read_text().rsplit(')', 1)[1].split()
            require(root.stat().st_uid == os.getuid(), 'Owned GUI descendant changed user identity')
            found[pid] = fields[19]
            data = (root / 'task' / str(pid) / 'children').read_text()
            require(len(data) <= 8192, 'Owned GUI child list exceeded its bound')
            for child in data.split():
                child_root = Path('/proc') / child
                try:
                    child_fields = (child_root / 'stat').read_text().rsplit(')', 1)[1].split()
                    if int(child_fields[1]) == pid:
                        pending.append(int(child))
                except FileNotFoundError:
                    continue
        except FileNotFoundError:
            continue
    return found


def owned_flatpak_process_pids(gui_pid, descendants):
    """Flatpak reports its sandbox init, which can be the GUI's PID-1 parent."""
    current = gui_pid
    selected = set()
    init_by_namespace_pid = {}
    gui_namespace = os.readlink(Path('/proc') / str(gui_pid) / 'ns/pid')
    for _ in range(8):
        if current not in descendants:
            break
        root = Path('/proc') / str(current)
        fields = (root / 'stat').read_text().rsplit(')', 1)[1].split()
        require(fields[19] == descendants[current], 'Owned Flatpak ancestor identity raced')
        if os.readlink(root / 'ns/pid') == gui_namespace:
            for line in (root / 'status').read_text().splitlines():
                if line.startswith('NSpid:'):
                    values = [int(value) for value in line.split()[1:]]
                    require(values and values[0] == current, 'Owned ancestor namespace mapping is unavailable')
                    selected.update(values)
                    if values[-1] == 1:
                        for value in values:
                            require(value not in init_by_namespace_pid or init_by_namespace_pid[value] == current,
                                    'Owned Flatpak namespace-init identity is ambiguous')
                            init_by_namespace_pid[value] = current
        current = int(fields[1])
    return selected, init_by_namespace_pid


def admitted_nested_gui(environment, expected_commit, expected_gui_sha256, wrapper_pid):
    """Map Flatpak namespace PID through exact owned descendants and kernel NSpid."""
    candidates = []
    descendants = owned_descendants(wrapper_pid)
    for pid, starttime in descendants.items():
        root = Path('/proc') / str(pid)
        try:
            executable = root / 'exe'
            if not os.readlink(executable).endswith('/dockpipe-launcher'):
                continue
            if digest(executable) != expected_gui_sha256:
                continue
            metadata = configparser.ConfigParser(interpolation=None)
            text = (root / 'root/.flatpak-info').read_text()
            require(len(text) <= 65536, 'Owned Flatpak metadata exceeded its bound')
            metadata.read_string(text)
            require(metadata.get('Application', 'name') == APP_ID
                    and metadata.get('Instance', 'app-commit') == expected_commit,
                    'Owned nested GUI immutable deployment identity changed')
            namespace_pids = None
            for line in (root / 'status').read_text().splitlines():
                if line.startswith('NSpid:'):
                    namespace_pids = [int(value) for value in line.split()[1:]]
            require(namespace_pids and namespace_pids[0] == pid,
                    'Owned nested GUI kernel PID mapping is unavailable')
            current = (root / 'stat').read_text().rsplit(')', 1)[1].split()[19]
            require(current == starttime, 'Owned nested GUI process identity raced')
            candidates.append((pid, starttime, metadata, namespace_pids))
        except (FileNotFoundError, ProcessLookupError):
            continue
    if not candidates:
        raise RuntimeError('Exact-image nested GUI descendant is not yet visible')
    require(len(candidates) == 1, 'Expected exactly one exact-image nested GUI descendant')
    pid, starttime, metadata, namespace_pids = candidates[0]
    inventory = subprocess.run(['flatpak', 'ps', '--columns=instance,child-pid,application,branch,commit:f'],
        env=environment, stdin=subprocess.DEVNULL, capture_output=True, text=True, check=True, timeout=3)
    require(len(inventory.stdout) <= 65536 and len(inventory.stderr) <= 8192,
            'Public Flatpak process inventory exceeded its bound')
    instance = metadata.get('Instance', 'instance-id')
    public_process_pids, init_by_namespace_pid = owned_flatpak_process_pids(pid, descendants)
    matches = []
    for line in inventory.stdout.splitlines():
        fields = line.split()
        if (len(fields) == 5 and fields[0] == instance and fields[1].isdigit()
                and int(fields[1]) in public_process_pids and fields[2:4] == [APP_ID, 'staging']
                and len(fields[4]) in [12, 64] and all(character in '0123456789abcdef' for character in fields[4])
                and expected_commit.startswith(fields[4])):
            matches.append(fields)
    if not matches:
        raise RuntimeError('Public Flatpak instance has not yet bound the owned nested GUI namespace; expected '
                           + repr({'instance': instance, 'namespace_pids': namespace_pids, 'public_process_pids': sorted(public_process_pids), 'commit': expected_commit})
                           + '; observed ' + repr(inventory.stdout.splitlines()[:8]))
    require(len(matches) == 1, 'Public Flatpak instance is ambiguous for the owned nested GUI namespace')
    public_child_pid = int(matches[0][1])
    require(public_child_pid in init_by_namespace_pid,
            'Public Flatpak child must map to its independently admitted owned namespace init')
    init_host_pid = init_by_namespace_pid[public_child_pid]
    require((Path('/proc') / str(pid) / 'stat').read_text().rsplit(')', 1)[1].split()[19] == starttime,
            'Nested GUI identity changed after public inventory')
    return {'instance': instance, 'commit': expected_commit, 'sandbox_pid': init_host_pid, 'host_pid': pid,
            'public_child_pid_namespace': public_child_pid, 'admitted_init_host_pid': init_host_pid,
            'namespace_pids': namespace_pids, 'executable_sha256': expected_gui_sha256,
            'accessibility_environment': {}, 'kernel_starttime': starttime}


def paired_prepare_action(action, windows, stage):
    """No Connect or viewer-local input exists in the paired metadata phase."""
    require(1 <= len(windows) <= 2, 'Paired preparation requires the exact main window and optional settings dialog')
    main = [window for window in windows if window['x'] == 0 and window['y'] == 0 and window['width'] == 1180]
    require(len(main) == 1, 'The immutable main-window layout is ambiguous')
    window = main[0]
    require(window['x'] == 0 and window['y'] == 0 and window['width'] == 1180,
            'Paired preparation requires the independently observed immutable app layout')
    purpose = action.get('purpose')
    if purpose == 'close-desktop-mode':
        require(stage == 'mode' and action.get('action') == 'key' and action.get('key') == 'Escape',
                'Only local desktop-mode dropdown Escape is admitted')
        return ['/usr/bin/xdotool', 'key', '--clearmodifiers', 'Escape'], 'selected'
    if purpose == 'close-paired-computers':
        require(stage == 'nodes' and action.get('action') == 'key' and action.get('key') == 'Escape',
                'Only one local paired-computer dropdown Escape is admitted')
        return ['/usr/bin/xdotool', 'key', '--clearmodifiers', 'Escape'], 'remote'
    if purpose == 'close-settings':
        require(stage == 'settings' and action.get('action') == 'key' and action.get('key') == 'Escape',
                'Only one local settings-dialog Escape is admitted')
        return ['/usr/bin/xdotool', 'key', '--clearmodifiers', 'Escape'], 'remote'
    require(action.get('action') == 'click' and type(action.get('x')) is int and type(action.get('y')) is int,
            'Paired metadata preparation admits only exact bounded local chrome clicks')
    x, y = action['x'], action['y']
    if purpose == 'show-remote-desktop':
        require(stage == 'initial' and 30 <= x <= 170 and 655 <= y <= 690,
                'Only the observed Remote Desktop sidebar item is admitted')
        following = 'remote'
    elif purpose == 'show-paired-computers':
        require(stage == 'remote' and 220 <= x <= 432 and 98 <= y <= 130,
                'Only the observed paired-computer dropdown is admitted; no selection or Connect')
        following = 'nodes'
    elif purpose == 'select-only-paired-computer':
        require(stage == 'nodes' and 220 <= x <= 432 and 108 <= y <= 128,
                'Only the independently observed single paired-computer row is admitted; no Connect')
        following = 'selected'
    elif purpose == 'show-desktop-mode':
        require(stage == 'selected' and 440 <= x <= 533 and 98 <= y <= 130,
                'Only the observed local desktop mode dropdown is admitted')
        following = 'mode'
    elif purpose == 'choose-view-mode':
        require(stage == 'mode' and 440 <= x <= 533 and 80 <= y <= 100,
                'Only the independently observed View item is admitted; no key or Connect')
        following = 'selected'
    elif purpose == 'show-remote-settings':
        require(stage == 'remote' and 1070 <= x <= 1155 and 145 <= y <= 175,
                'Only the observed remote Settings button is admitted')
        following = 'settings'
    elif purpose == 'normal-quit':
        require(stage in ['remote', 'selected'] and 1138 <= x <= 1155 and 15 <= y <= 32,
                'Only the observed main-window normal close is admitted')
        following = 'quit'
    else:
        raise ValueError('No connection, key entry or other action is admitted in paired metadata preparation')
    return ['/usr/bin/xdotool', 'mousemove', '--sync', str(x), str(y), 'click', '1'], following


def normal_gui(request, configuration, provider, output, repository, gui, operation=None):
    """Observe offline setup through the normal installed entrypoint.

    Live qualification is admitted separately after this saved profile and the
    exact provider identities have been reviewed. This observer cannot connect.
    """
    require((configuration['mode'] in ['prepare', 'paired-prepare'] and operation is None)
            or (configuration['mode'] == 'lease-expiry' and callable(operation)),
            'Live GUI route requires a separately reviewed, source-bound lifecycle operation')
    paired_prepare = configuration['mode'] in ['paired-prepare', 'lease-expiry']
    linux = repository / 'workflows/qualify/product/platforms/linux'
    sys.path.insert(0, str(linux))
    from desktop import OwnedDesktop
    selected = output / 'normal-gui'
    selected.mkdir(mode=0o700)
    home = selected / 'private-home'
    home.mkdir(mode=0o700)
    runtime = gui.create_private_runtime('dockpipe-remote-runtime-')
    selections = {'xvfb': str(pinned(configuration['xvfb'])), 'workspace_path': provider['workspace']}
    accessibility_probe = configuration.get('accessibility_probe', False)
    require(type(accessibility_probe) is bool, 'Accessibility probe selection must be boolean')
    launches = []
    quit_requested = False
    before = host_profile_identity()
    result = {'normal_gui_started': False, 'clipboard_preferences_verified': False,
              'remote_session_opened': False, 'capture_or_input_started': False,
              'frames_observed': False, 'generated_input_observed': False,
              'remote_closure_verified': False, 'local_cleanup_verified': False,
              'reason': 'Offline normal installed GUI setup only; live viewer remains unqualified'}

    def prepare(environment):
        gui.clear_native_runtime_environment(environment)
        environment.update(profile_environment(home, configuration['installation']))
        environment.update(XDG_RUNTIME_DIR=str(runtime), GVFS_DISABLE_FUSE='1', GIO_USE_VFS='local',
                           QT_LINUX_ACCESSIBILITY_ALWAYS_ON='1' if accessibility_probe else '0',
                           QT_ACCESSIBILITY='1' if accessibility_probe else '0',
                           NO_AT_BRIDGE='0' if accessibility_probe else '1')

    def launch(desktop, environment, workspace):
        profile = home / '.var/app' / APP_ID / 'config/dockpipe/dockpipe-launcher'
        profile.mkdir(mode=0o700, parents=True)
        retained = configuration.get('prepared_profile')
        if retained is not None:
            require(set(retained) == {'settings', 'launcher'}, 'Expected exact prepared private profile file pins')
            shutil.copyfile(pinned(retained['launcher']), profile / 'launcher.json')
            shutil.copyfile(pinned(retained['settings']), profile.parent / 'dockpipe-launcher.conf')
        else:
            shutil.copyfile(desktop.output / 'config/dockpipe/dockpipe-launcher/launcher.json', profile / 'launcher.json')
        if accessibility_probe:
            import dbus
            bus_deadline = time.monotonic() + 3
            while True:
                try:
                    gui.select_private_accessibility_bus(desktop, environment)
                    break
                except dbus.DBusException:
                    if time.monotonic() >= bus_deadline:
                        raise
                    time.sleep(0.1)
            write(selected / 'accessibility-before-launch.json', private_accessibility_facts(environment))
        command = gui.installed_launch_command(output, configuration['installation'], workspace,
                                               environment, desktop.socket_directory,
                                               use_accessibility=accessibility_probe)
        command.remove('--unset-env=DOCKPIPE_PACKAGES_ROOT')
        insertion = command.index('--command=dockpipe-launcher')
        command[insertion:insertion] = ['--filesystem=' + str(runtime),
                                       '--filesystem=' + provider['package_root'] + ':ro',
                                       '--env=DOCKPIPE_PACKAGES_ROOT=' + provider['package_root']]
        descriptors = []
        launcher_environment = environment
        if paired_prepare:
            require(not accessibility_probe and retained is not None,
                    'Paired preparation requires saved all-OFF preferences and no accessibility probe')
            remote_preferences(profile.parent / 'dockpipe-launcher.conf')
            command.remove('--unshare=network')
            insertion = command.index('--command=dockpipe-launcher')
            state = configuration['provider_configuration']['state']
            security = str(Path(pwd.getpwuid(os.getuid()).pw_dir) / '.local/state/dockpipe/security')
            command[insertion:insertion] = [
                '--filesystem=' + state + ':ro', '--filesystem=' + security + ':ro',
                '--env=DOCKPIPE_GLOBAL_ROOT=' + str(Path(state).parent)]
            command, launcher_environment, descriptors = private_launcher(
                command, environment, configuration, home,
                [output, Path(workspace), runtime, desktop.socket_directory, Path(provider['package_root'])],
                readonly_sockets=[Path('/tmp/.X11-unix') / ('X' + environment['DISPLAY'].removeprefix(':').split('.')[0])])
            write(selected / 'paired-launch-command.json', {'arguments': command,
                  'remote_effects_permitted': operation is not None, 'selected_state_read_only': True})
        with (selected / 'gui.log').open('xb') as log:
            try:
                process = subprocess.Popen(command, env=launcher_environment, cwd=workspace,
                    stdout=log, stderr=log, start_new_session=True, pass_fds=descriptors)
            finally:
                for descriptor in descriptors:
                    os.close(descriptor)
        launches.append(process)
        return process

    selections.update(prepare_environment=prepare, normal_gui_launcher=launch)
    desktop = OwnedDesktop(selected / 'desktop', selections)
    actions = []
    paired_stage = 'initial'
    try:
        process = desktop.start()
        admission = admitted_nested_gui if paired_prepare else gui.admitted_child
        identity = gui.wait_for('Exact installed remote GUI', lambda: admission(
            desktop.environment, configuration['installed_commit'], configuration['gui_sha256'], process.pid))
        write(selected / 'gui-identity.json', identity)
        gui.wait_for('Owned visible normal GUI', lambda: gui.guided_windows(desktop.environment, identity))
        result['normal_gui_started'] = True
        if operation is not None:
            result.update(operation(desktop, identity, process, home, selected))
            return result
        if accessibility_probe:
            write(selected / 'accessibility-after-launch.json',
                  private_accessibility_facts(desktop.environment, identity))
            observe_accessibility(selected, desktop.environment, identity, process.pid)
        deadline = time.monotonic() + 780
        for sequence in range(1, 65):
            if process.poll() is not None:
                require(quit_requested and process.returncode == 0, 'Normal GUI exited before normal-close observation')
                result['gui_exit_normal'] = True
                break
            current = admission(desktop.environment, configuration['installed_commit'],
                                         configuration['gui_sha256'], process.pid)
            require(current == identity, 'Installed GUI process identity changed')
            windows = gui.guided_windows(desktop.environment, identity)
            image_path = selected / ('offline-%02d.png' % sequence)
            capture_owned_display(desktop.environment).save(image_path)
            image_path.chmod(0o600)
            ready = {'sequence': sequence, 'snapshot': str(image_path), 'sha256': digest(image_path),
                     'windows': windows, 'identity': identity, 'offline_network': not paired_prepare,
                     'remote_actions_admitted': False}
            temporary = selected / 'ready.next'
            write(temporary, ready)
            temporary.replace(selected / 'ready.json')
            action_path = selected / ('action-%02d.json' % sequence)
            while not action_path.exists() and time.monotonic() < deadline:
                if process.poll() is not None:
                    require(quit_requested and process.returncode == 0, 'Owned GUI exited unexpectedly')
                    result['gui_exit_normal'] = True
                    break
                time.sleep(0.1)
            if result.get('gui_exit_normal'):
                break
            require(action_path.exists() and not action_path.is_symlink() and action_path.stat().st_size <= 4096,
                    'Offline guided action absent or invalid')
            action = json.loads(action_path.read_text())
            require(action['sequence'] == sequence and action['basis_sha256'] == ready['sha256'],
                    'Offline action must bind the current snapshot and sequence')
            require(isinstance(action.get('purpose'), str) and 1 <= len(action['purpose']) <= 256,
                    'Describe the observed local settings action')
            if paired_prepare:
                command, paired_stage = paired_prepare_action(action, windows, paired_stage)
                subprocess.run(command, env=desktop.environment, check=True, timeout=3)
                quit_requested = paired_stage == 'quit'
                if quit_requested:
                    settings = list(home.rglob('dockpipe-launcher.conf'))
                    require(len(settings) == 1, 'Expected one private paired GUI preference file')
                    result['preferences'] = remote_preferences(settings[0])
                    result['clipboard_preferences_verified'] = True
                actions.append(action)
                time.sleep(0.4)
                continue
            if action['action'] == 'finish':
                settings = list(home.rglob('dockpipe-launcher.conf'))
                require(len(settings) == 1, 'Expected one private normal GUI preference file')
                result['preferences'] = remote_preferences(settings[0])
                result['clipboard_preferences_verified'] = True
                # File/Quit is the standard application lifecycle route.
                subprocess.run(['/usr/bin/xdotool', 'key', '--clearmodifiers', 'ctrl+q'],
                               env=desktop.environment, check=True, timeout=5)
                quit_requested = True
                actions.append(action)
                time.sleep(0.5)
                continue
            require(action['action'] in ['click', 'key'], 'Only local settings click/key/finish actions admitted')
            if action['action'] == 'click':
                require(type(action.get('x')) is int and type(action.get('y')) is int,
                        'Click coordinates must be exact integers')
                require(any(window['x'] + 2 <= action['x'] < window['x'] + window['width'] - 2
                            and window['y'] + 2 <= action['y'] < window['y'] + window['height'] - 2
                            for window in windows), 'Click escaped the exact owned visible GUI windows')
                command = ['/usr/bin/xdotool', 'mousemove', '--sync', str(action['x']), str(action['y']), 'click', '1']
            else:
                allowed = {'Return', 'Escape', 'Tab', 'ISO_Left_Tab', 'Home', 'End', 'Up', 'Down', 'space'}
                require(action.get('key') in allowed, 'Only bounded local settings navigation keys admitted')
                command = ['/usr/bin/xdotool', 'key', '--clearmodifiers', action['key']]
            subprocess.run(command, env=desktop.environment, check=True, timeout=5)
            actions.append(action)
            time.sleep(0.2)
        else:
            raise RuntimeError('Offline GUI guided action count exhausted')
    finally:
        write(selected / 'guided-actions.json', actions)
        try:
            gui.track_private_services(desktop)
        finally:
            desktop.stop()
        if runtime.exists():
            cleanup_deadline = time.monotonic() + 3
            while gui.runtime_mounts(runtime) and time.monotonic() < cleanup_deadline:
                time.sleep(0.1)
            require(not gui.runtime_mounts(runtime), 'Private GUI runtime retains an owned mount')
            shutil.rmtree(runtime)
        require(host_profile_identity() == before, 'Real default app/profile metadata changed')
        result['local_cleanup_verified'] = all(process.poll() is not None for process in desktop.processes)
        result['host_default_profile_unchanged'] = True
        write(output / 'normal-gui-result.json', result)
    return result


def run(request_path, output):
    repository, contracts, _process, _portable, gui, _observer = owner_modules()
    request = contracts.read_json(request_path)
    configuration = verify_configuration(request)
    require(output.is_absolute() and not output.exists(), 'Fresh absolute installed GUI output required')
    output.mkdir(mode=0o700, parents=True)
    configuration = dict(configuration, private_home=str(output / 'identity-home'))
    require_installed(configuration)
    if configuration['mode'] == 'preflight':
        facts = launcher_preflight(configuration, output / 'launcher-preflight')
        write(output / 'normal-gui-result.json', facts)
        cases = [dict({name: case[name] for name in ['id', 'scenario', 'layer']}, status='blocked',
                      reason='Read-only genuine app launcher preflight; no normal GUI/capture/input exercised',
                      assertions=[], evidence=[{'path': 'normal-gui-result.json',
                                               'sha256': digest(output / 'normal-gui-result.json')}])
                 for case in request['cases']]
        receipt = {name: request[name] for name in ['run_id', 'plan_sha256', 'registry_sha256', 'target']}
        receipt.update(schema='dockpipe.product-integration.lane/v1', cases=cases)
        contracts.validate_receipt(receipt, request, output)
        write(output / 'result.json', receipt)
        return
    provider = prepare_provider(configuration, output / 'provider')
    write(output / 'provider-prepared.json', provider)
    result = normal_gui(request, configuration, provider, output, repository, gui)
    cases = []
    for case in request['cases']:
        row = {name: case[name] for name in ['id', 'scenario', 'layer']}
        row.update(status='blocked', reason='Requested remote GUI case was not observed', assertions=[], evidence=[])
        if case['id'] == 'remote.desktop.viewer' and case['scenario'] == 'happy' and case['layer'] == 'installed_e2e':
            observed = {'decoded-nonuniform-frames': result.get('frames_observed') is True,
                        'generated-physical-input': result.get('generated_input_observed') is True,
                        'core-and-native-closure': result.get('remote_closure_verified') is True
                            and result.get('local_cleanup_verified') is True}
            require(set(case['required_assertions']) == set(observed), 'Installed viewer assertion contract changed')
            row['assertions'] = [{'id': name, 'status': 'passed' if value else 'failed',
                                  'detail': 'Actual normal installed GUI and independent provider observations'}
                                 for name, value in observed.items()]
            if configuration['mode'] == 'live':
                row['status'] = 'passed' if all(observed.values()) else 'failed'
            row['reason'] = result.get('reason', 'Offline normal GUI preparation cannot qualify live remote control')
            row['evidence'] = [{'path': 'normal-gui-result.json', 'sha256': digest(output / 'normal-gui-result.json')},
                               {'path': 'provider-prepared.json', 'sha256': digest(output / 'provider-prepared.json')}]
        cases.append(row)
    receipt = {name: request[name] for name in ['run_id', 'plan_sha256', 'registry_sha256', 'target']}
    receipt.update(schema='dockpipe.product-integration.lane/v1', cases=cases)
    contracts.validate_receipt(receipt, request, output)
    write(output / 'result.json', receipt)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--request', type=Path)
    parser.add_argument('--output', type=Path)
    parser.add_argument('--inside-package-install', type=Path)
    parser.add_argument('--inspect-accessibility', type=Path)
    parser.add_argument('--inside-launcher-preflight', type=Path)
    arguments = parser.parse_args()
    if arguments.inspect_accessibility is not None:
        inspect_accessibility(arguments.inspect_accessibility)
        return
    if arguments.inside_package_install is not None:
        inside_package_install(arguments.inside_package_install)
        return
    if arguments.inside_launcher_preflight is not None:
        inside_launcher_preflight(arguments.inside_launcher_preflight)
        return
    require(arguments.request is not None and arguments.output is not None, 'Select request and fresh output')
    run(arguments.request, arguments.output)


if __name__ == '__main__':
    main()
