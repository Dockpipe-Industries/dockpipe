"""Observe a fresh, guided installation through the normal owned Dockpipe GUI.

The operator or UI agent performs visible installer, login and power actions.
This module never starts QEMU or accepts a caller-supplied passed receipt.
"""
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
from pathlib import Path
import re
import secrets
import shlex
import subprocess
import threading
import time

from desktop import OwnedDesktop, digest
from guest_environment import prepare_native_guest_environment, resource_admission
from guest import Guest
from guest_enrollment import GuestEnrollment
from form import launch_filled_installer
import seed


class SeedServer:
    def __init__(self, directory):
        directory = Path(directory)

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                if self.path not in ['/user-data', '/meta-data']:
                    self.send_error(404)
                    return
                content = (directory / self.path[1:]).read_bytes()
                self.send_response(200)
                self.send_header('Content-Type', 'text/plain')
                self.send_header('Content-Length', str(len(content)))
                self.end_headers()
                self.wfile.write(content)

            def log_message(self, *_arguments):
                pass

        self.server = HTTPServer(('127.0.0.1', 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)

    def start(self):
        self.thread.start()
        return f'http://10.0.2.2:{self.server.server_port}/user-data'

    def stop(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)


def read_process(pidfile, disk, qemu):
    """Admit only an owning-workspace process using this invocation's new disk."""
    if pidfile.is_symlink() or not pidfile.is_file():
        return None
    text = pidfile.read_text().strip()
    if not text.isdecimal():
        raise RuntimeError('Invalid owning workflow process identity.')
    pid = int(text)
    process = Path('/proc') / str(pid)
    try:
        executable = process.joinpath('exe').resolve(strict=True)
        arguments = process.joinpath('cmdline').read_bytes().decode().split('\0')[:-1]
        starttime = process.joinpath('stat').read_text().rsplit(')', 1)[1].split()[19]
    except FileNotFoundError:
        return None
    if executable != qemu.resolve(strict=True):
        return None
    selected = {flag: [arguments[index + 1] for index, value in enumerate(arguments[:-1]) if value == flag]
                for flag in ['-uuid', '-drive', '-netdev', '-cdrom', '-name', '-m', '-smp', '-display']}
    if not any(f'file={disk},' in drive for drive in selected['-drive']):
        return None
    if len(selected['-uuid']) != 1 or len(selected['-netdev']) != 1:
        raise RuntimeError('Owning guest lacks one exact machine/network identity.')
    port = re.search(r'hostfwd=tcp::([0-9]+)-:22(?:,|$)', selected['-netdev'][0])
    if port is None:
        raise RuntimeError('Owning guest lacks its expected isolated SSH forwarding.')
    return {'pid': pid, 'starttime': starttime, 'selected_arguments': selected,
            'ssh_port': int(port.group(1)),
            'no_installer_media': '-cdrom' not in arguments
            and not any('.iso' in argument.lower() for argument in arguments)}


def process_alive(identity):
    try:
        stat = Path('/proc', str(identity['pid']), 'stat').read_text()
    except FileNotFoundError:
        return False
    return stat.rsplit(')', 1)[1].split()[19] == identity['starttime']


def observe_process(workspace, disk, qemu, excluded=()):
    matches = []
    for pidfile in workspace.rglob('qemu-*.pid'):
        process = read_process(pidfile, disk, qemu)
        if process and (process['pid'], process['starttime']) not in excluded:
            matches.append(process)
    if len(matches) > 1:
        raise RuntimeError('Multiple owning guests use the selected new disk; reconcile before continuing.')
    return matches[0] if matches else None


def wait_until(observe, deadline, gui):
    while time.monotonic() < deadline:
        if gui.poll() is not None:
            raise RuntimeError('Owned normal Dockpipe GUI exited before lifecycle completion.')
        result = observe()
        if result:
            return result
        time.sleep(2)
    raise TimeoutError('Guided lifecycle deadline reached; unobserved assertions remain pending.')


def run(request, output, pins, media, baseline, timeout_seconds):
    output = Path(output)
    enrollment = None
    configuration = request.get('configuration', {})
    if 'guest_enrollment' in configuration:
        selected = configuration['guest_enrollment']
        grant = {key: selected[key] for key in ('grant_file', 'grant_sha256', 'host_state')}
        enrollment = GuestEnrollment(grant, request['run_id'], pins['cli'], output)
        enrollment_debs = [{'path': selected[key], 'sha256': selected[key + '_sha256']}
                           for key in ('worker_deb', 'desktop_deb')]

    disk = output / 'guest/disk.qcow2'
    disk.parent.mkdir(mode=0o700)
    if disk.exists():
        raise RuntimeError('The guided installer only admits a new destination.')
    private = output / 'private-seed'
    seed.prepare(private, request['run_id'])
    server = SeedServer(private)
    desktop_selections = dict(pins)
    desktop_selections['prepare_environment'] = lambda environment: prepare_native_guest_environment(pins, environment)
    desktop = OwnedDesktop(output / 'desktop', desktop_selections)
    assertions = {}
    evidence = []
    active = None
    cleanup_admitted = True
    deadline = time.monotonic() + timeout_seconds

    def record(name, value):
        path = output / (name + '.json')
        path.write_text(json.dumps(value, indent=2) + '\n')
        path.chmod(0o600)
        evidence.append({'path': path.name, 'sha256': digest(path)})

    def guide(phase, instructions):
        path = output / 'GUIDED-ACTIONS.md'
        path.write_text(f'# {phase}\n\n{instructions}\n')
        path.chmod(0o600)

    def capture(name):
        from PIL import ImageGrab
        path = output / (name + '.png')
        ImageGrab.grab(xdisplay=desktop.environment['DISPLAY']).save(path)
        path.chmod(0o600)
        evidence.append({'path': path.name, 'sha256': digest(path)})

    def release_enrollment():
        nonlocal enrollment
        owned = enrollment
        enrollment = None
        if owned is not None:
            receipt = owned.stop()
            if receipt is not None:
                record('guest-enrollment-release', receipt)
            return receipt

        return None

    try:
        url = server.start()
        resource_admission(output)
        gui = desktop.start()
        workspace = Path(json.loads((desktop.output / 'session-private.json').read_text())['workspace'])
        qemu = Path(pins['qemu_prefix']) / 'bin/qemu-system-x86_64'
        cleanup_admitted = False
        guide('Install through the normal GUI',
              'Use the private desktop/session descriptor in desktop/session-private.json. '
              'Open VM > Actions > Start Linux VM. Select Install from ISO, installer '
              f'`{media}`, NEW destination `{disk}`, 64G persistent disk. '
              'Leave the filled form open: the adapter checks resources and presses its normal '
              'Launch button. In the actual Ubuntu '
              'installer use English, US keyboard, wired network, skip installer updates, '
              f'Install Ubuntu > Automated installation > `{url}` > Validate > Install. '
              'Only the new fixture disk is disposable. Do not launch QEMU independently.')
        record('normal-gui-launch', wait_until(
            lambda: launch_filled_installer(desktop, media, disk), deadline, gui))
        active = wait_until(lambda: observe_process(workspace, disk, qemu), deadline, gui)
        if active['selected_arguments']['-cdrom'] != [str(media)]:
            raise RuntimeError('Installer did not start with the exact admitted ISO.')
        record('installer-process', active)
        capture('installer-started')
        record('host-provisioning-prerequisite', {
            'role': 'fixture-provisioning', 'qualified_guest_feature': False,
            'host_gui': pins['gui'], 'host_gui_sha256': digest(pins['gui']),
            'host_cli': pins['cli'], 'host_cli_sha256': digest(pins['cli'])})
        guest = Guest(private / 'fixture-private.json', active['ssh_port'], private / 'known-hosts')
        installed = wait_until(guest.observe_install, deadline, gui)
        record('installed-os', installed)
        product = guest.install_baseline(baseline, output)
        record('published-baseline-installed', product)
        expected_version = product['version_output'].splitlines()[0]
        cli = guest.exercise_cli(output)
        record('installed-cli', cli)
        for assertion in ['installed-cli-version', 'initialized-project', 'host-workflow-executed']:
            assertions[assertion] = cli[assertion]
        if enrollment is not None:
            enrollment_product = guest.install_enrollment_product(enrollment_debs, output)
            record('guest-enrollment-product', enrollment_product)
            expected_version = enrollment_product['version_output'].splitlines()[0]
            record('guest-enrollment', enrollment.start(guest))

        marker = secrets.token_hex(16)
        remote_instructions = ''
        if enrollment is not None:
            remote_instructions = (f'The temporary workflow worker is `{enrollment.node}`. '
                                   'Run any separate delivered-workflow qualification before typing the marker. '
                                   'Observing the marker and launcher ends the enrollment phase.\n\n')

        guide('Verify embedded input and installed desktop',
              remote_instructions + 'Log into the embedded guest with its private generated fixture credentials '
              '(do not log or paste credentials into receipts). Complete first-login prompts '
              'without OS upgrade. Open a guest terminal THROUGH THE EMBEDDED DISPLAY and type:\n\n'
              f"`printf '%s' '{marker}' > /tmp/dockpipe-embedded-input.txt; dockpipe-launcher &`\n\n"
              'Do not issue this command over SSH. Leave the installed Dockpipe window visible.')

        def input_observed():
            if enrollment is not None:
                enrollment.require_running()

            process_query = '''
from pathlib import Path
expected = Path('/usr/bin/dockpipe-launcher').resolve()
for process in list(Path('/proc').iterdir())[:4096]:
    if not process.name.isdecimal():
        continue
    try:
        if (process / 'exe').resolve(strict=True) == expected:
            print(process.name)
    except (FileNotFoundError, PermissionError):
        continue
'''
            result = guest.run('cat /tmp/dockpipe-embedded-input.txt; '
                               'printf "\\n---GUI---\\n"; python3 -c ' + shlex.quote(process_query))
            if result.returncode != 0:
                return None
            text, processes = result.stdout.decode().split('\n---GUI---\n', 1)
            pids = processes.splitlines()
            if text != marker or not pids:
                return None
            return {'marker_matched': True, 'guest_gui_pids': pids}

        record('embedded-input', wait_until(input_observed, deadline, gui))
        capture('embedded-installed-desktop')
        if enrollment is not None:
            receipt = release_enrollment()
            if not receipt['worker_transport_running_at_release']:
                raise RuntimeError('The enrolled guest worker exited before the release phase.')

        guide('Graceful shutdown', 'Use the EMBEDDED guest normal Power Off menu and confirmation. '
              'Do not terminate QEMU. The adapter independently waits for the exact process to exit.')
        wait_until(lambda: not process_alive(active), deadline, gui)
        record('first-stop', {'process_absent': True, 'identity': active})
        previous = active
        active = None
        resource_admission(output)
        guide('Disk-only restart', 'Use normal VM > Actions > Start Linux VM. Clear the installer '
              'ISO and install destination while visible, select Boot existing disk image, '
              f'choose SAME disk `{disk}`, persistent mode, empty SSH user. Launch and choose '
              'Open interactive desktop. Preserve the machine identity. No installer media may remain.')
        excluded = {(previous['pid'], previous['starttime'])}
        active = wait_until(lambda: observe_process(workspace, disk, qemu, excluded), deadline, gui)
        record('restart-process', active)
        if not active['no_installer_media']:
            raise RuntimeError('Disk-only restart retained installer media; failed assertion preserved.')
        if active['selected_arguments']['-uuid'] != previous['selected_arguments']['-uuid']:
            raise RuntimeError('Disk-only restart changed the persistent machine identity.')
        guest = Guest(private / 'fixture-private.json', active['ssh_port'], private / 'known-hosts')
        restarted = wait_until(guest.observe_install, deadline, gui)
        if restarted['boot_id'] == installed['boot_id']:
            raise RuntimeError('Disk-only restart did not produce a new guest boot identity.')
        version = guest.run('dockpipe --version')
        if version.returncode != 0 or version.stdout.decode().splitlines()[0] != expected_version:
            raise RuntimeError('The selected installed CLI did not survive restart.')
        if 'guest_enrollment' in configuration:
            executables = guest.run('sha256sum /usr/bin/dockpipe /usr/bin/dockpipe-launcher')
            hashes = [line.split()[0] for line in executables.stdout.decode().splitlines()]
            if executables.returncode != 0 or hashes != [enrollment_product['cli_sha256'],
                                                        enrollment_product['desktop_sha256']]:
                raise RuntimeError('The installed enrollment product digests changed after restart.')

            restarted['cli_sha256'], restarted['desktop_sha256'] = hashes

        record('disk-only-restart', restarted)
        capture('disk-only-embedded-login')
        guide('Final graceful shutdown', 'Use the EMBEDDED guest normal Power Off menu and confirmation. '
              'Keep the installed disk. The adapter checks exact-process absence before cleanup.')
        wait_until(lambda: not process_alive(active), deadline, gui)
        record('final-stop', {'process_absent': True, 'identity': active, 'disk_retained': disk.is_file()})
        active = None
        cleanup_admitted = True
        return assertions, evidence, None
    except (RuntimeError, TimeoutError, OSError, ValueError, subprocess.SubprocessError) as error:
        reason = str(error)
        try:
            release_enrollment()
        except (RuntimeError, OSError, ValueError, subprocess.SubprocessError) as cleanup_error:
            reason += f' Guest enrollment cleanup: {cleanup_error}'

        record('incomplete-lifecycle', {'reason': reason, 'active_guest': active,
                                      'assertions_observed': assertions})
        return assertions, evidence, reason
    finally:
        try:
            release_enrollment()
        finally:
            server.stop()
            if cleanup_admitted or (active is not None and not process_alive(active)):
                desktop.stop()
            else:
                guide('Reconcile retained owned guest',
                      'The lifecycle failed with a live guest. Its exact identity is in incomplete-lifecycle.json '
                      'and private GUI descriptor in desktop/session-private.json. Use its normal power menu '
                      'before releasing the guest slot.')
