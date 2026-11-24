"""Opt-in normal installed-GUI viewer lease-loss fault; never sends remote input.

A private plan binds the exact app/provider/native/fixture artifacts. The first
live invocation still requires review of that concrete plan and effect list.
"""
from pathlib import Path
from contextlib import contextmanager
import argparse
import hashlib
import importlib.util
import json
import os
import select
import signal
import stat
import subprocess
import sys
import time
import traceback

HERE = Path(__file__).resolve().parent
SCHEMA = 'dockpipe.flatpak.remote-lease-expiry-plan/v1'


def require(condition, message):
    if not condition:
        raise ValueError(message)


def consume_startup_admission(facade, candidate, verify_identity):
    """Verify the viewer before consuming the original, unextended frame gate."""
    identities = verify_identity()
    gate = facade.command('consume-frame-admission', candidate=candidate)
    require(isinstance(candidate, dict) and isinstance(gate, dict)
            and gate.get('phase') == 0
            and gate.get('local_capture_budget_milliseconds') == 100
            and gate.get('capture_before_monotonic') == candidate.get('capture_before_monotonic'),
            'Consumed frame admission changed its original phase or capture deadline')
    deadline = gate['capture_before_monotonic']
    require(time.monotonic() < deadline,
            'Original frame capture interval elapsed during viewer identity admission')
    return gate, identities


def consume_startup_frame(facade, candidate, verify_identity, capture):
    gate, identities = consume_startup_admission(facade, candidate, verify_identity)
    image = capture()
    deadline = gate['capture_before_monotonic']
    require(time.monotonic() < deadline,
            'Admitted fixture frame capture exceeded its original receipt deadline')
    return image, identities


@contextmanager
def capture_and_pause(facade, candidate, verify_identity, reader, admitted, pause, watchdog, facts):
    gate, identities = consume_startup_admission(facade, candidate, verify_identity)
    six_cutoff = pause.deadline - 12
    with reader.capture_bracketed({'x': 0, 'y': 0, 'width': 1600, 'height': 1000}, admitted,
                                  gate['capture_before_monotonic'], six_cutoff, facts) as frame:
        watchdog.unassisted()
        require(time.monotonic() < six_cutoff,
                'Admitted frame arrived after the unchanged six-second pause cutoff')
        paused = pause.pause()
        facts['pause'] = paused
        require(paused['gui'] == pause.token and paused['pause_observed_monotonic'] <= six_cutoff,
                'Original GUI stopped state was not observed within six seconds')
        facts['window_paused'] = reader.verify_paused_window(frame, pause.deadline)
        yield frame, identities, paused


def validate_paused_frame(reader, frame, pause, marker, nonce, current_windows, facts):
    require(pause.current()['state'] in ['T', 't'] and time.monotonic() < pause.deadline,
            'Original GUI must remain stopped before validating its captured frame')
    facts['decode_started_monotonic'] = time.monotonic()
    image = reader.decode_frame(frame)
    facts['decode_finished_monotonic'] = time.monotonic()
    try:
        observed = marker.locate(image, nonce, 0)
        facts['marker_verified_monotonic'] = time.monotonic()
        require(time.monotonic() < pause.deadline, 'Captured nonce validation exceeded the original cutoff')
        windows = current_windows()
        marker_inside_owned_view(observed, frame['before'], windows, frame['visible_owned_rect'])
        facts['full_gui_verified_monotonic'] = time.monotonic()
        require(pause.current()['state'] in ['T', 't'] and time.monotonic() < pause.deadline,
                'Original GUI resumed or expired before full captured-frame verification')
        facts['validation_complete'] = True
        return observed
    finally:
        del image


def finish_unassisted_gui(proof, watchdog, pause, quit_gui, close_owner):
    """Quit the already closed viewer before blocking on remote fixture cleanup."""
    require(isinstance(proof, dict)
            and proof.get('schema') == 'dockpipe.flatpak.gui-lease-expiry-observation/v1'
            and proof.get('gui') == pause.token
            and proof.get('before_hard_cutoff') is True
            and proof.get('core_closed') is True and proof.get('native_idle') is True
            and proof.get('cleanup_assisted') is False and proof.get('input_permitted') is False,
            'Only strict original unassisted Core/native proof permits normal GUI exit')
    require(time.monotonic() < pause.deadline,
            'Original GUI exit interval elapsed before resume; never extend the watchdog')
    watchdog.unassisted()
    pause.close()
    quit_gui()
    # The opaque fixture and all owner/native guards remain in place until this
    # exact cleanup. Do not disarm or extend the independent GUI watchdog.
    return close_owner()


def load(name, path):
    specification = importlib.util.spec_from_file_location(name, path)
    value = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(value)
    return value


def digest(path):
    descriptor = os.open(path, os.O_RDONLY | os.O_NONBLOCK)
    result = hashlib.sha256()
    with os.fdopen(descriptor, 'rb') as stream:
        before = os.fstat(stream.fileno())
        require(stat.S_ISREG(before.st_mode) and 0 <= before.st_size <= 2 * 1024 * 1024 * 1024,
                'Pinned artifact must be a bounded regular file')
        total = 0
        while True:
            chunk = stream.read(65536)
            if not chunk:
                break
            total += len(chunk)
            require(total <= 2 * 1024 * 1024 * 1024, 'Pinned artifact exceeded its byte bound')
            result.update(chunk)
        after = os.fstat(stream.fileno())
    require((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns, before.st_ctime_ns)
            == (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_ctime_ns),
            'Pinned artifact raced during hashing')
    return result.hexdigest()


def write(path, value):
    with Path(path).open('x') as stream:
        os.chmod(stream.name, 0o600)
        json.dump(value, stream, indent=2, allow_nan=False)
        stream.write('\n')


def captured_json(path):
    path = Path(path)
    require(path.is_absolute(), 'Select an absolute private plan path')
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(descriptor, 'rb') as stream:
        before = os.fstat(stream.fileno())
        require(stat.S_ISREG(before.st_mode) and before.st_uid == os.getuid()
                and not before.st_mode & 0o077 and 0 < before.st_size <= 256 * 1024,
                'Select only bounded private regular plan metadata')
        data = stream.read(256 * 1024 + 1)
        after = os.fstat(stream.fileno())
    require((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns)
            == (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns), 'Private plan metadata raced')
    return data, json.loads(data, object_pairs_hook=unique_object, parse_constant=reject_constant)


def unique_object(pairs):
    value = {}
    for key, item in pairs:
        require(key not in value, 'Duplicate qualification metadata key')
        value[key] = item
    return value


def reject_constant(_value):
    raise ValueError('Nonfinite qualification metadata is not admitted')


def pinned_json(record):
    require(isinstance(record, dict) and set(record) == {'path', 'sha256'}, 'Expected one exact metadata pin')
    path = Path(record['path'])
    require(path.is_absolute() and not path.is_symlink(), 'Selected metadata path must be absolute and regular')
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(descriptor, 'rb') as stream:
        before = os.fstat(stream.fileno())
        require(stat.S_ISREG(before.st_mode) and 0 < before.st_size <= 2 * 1024 * 1024,
                'Selected metadata exceeds its regular-file bound')
        data = stream.read(2 * 1024 * 1024 + 1)
        after = os.fstat(stream.fileno())
    require((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns, before.st_ctime_ns)
            == (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_ctime_ns)
            and hashlib.sha256(data).hexdigest() == record['sha256'], 'Selected metadata changed or raced')
    return json.loads(data, object_pairs_hook=unique_object, parse_constant=reject_constant)


def bound_native_sources(plan):
    binding = pinned_json(plan['source_binding'])
    require(isinstance(binding, dict) and binding.get('schema') == 'dockpipe.private.remote-lease-expiry-source-binding/v1',
            'Unknown native source-binding evidence')
    evidence = binding.get('build_evidence')
    require(isinstance(evidence, list) and len(evidence) == 3, 'Exact native receipt/snapshot and Flatpak inventory required')
    snapshot, receipt, flatpak = [pinned_json(record) for record in evidence]
    native_spec = pinned_json(plan['facade']['configuration']['native_spec'])
    require(snapshot['source_sha256'] == receipt['source_sha256'] == binding['native_build_source_sha256']
            and receipt['channel_helper_sha256'] == native_spec['images']['channel']['sha256'] == binding['native_channel_sha256']
            and receipt['module_sha256'] == native_spec['images']['module']['sha256'] == binding['native_module_sha256']
            and binding['flatpak_module_sha256'] == plan['configuration']['qualified_provider']['module_sha256'],
            'The reviewed lease/stop source does not bind the selected native/Flatpak images')
    rows = binding.get('bound_sources')
    required_sources = {'channel/authority.go', 'channel/protocol.go', 'channel/session.go', 'channel/transport.go',
        'channel/child.go', 'native/worker/NativeWorker.mm', 'native/worker/WorkerChannel.h',
        'native/worker/WorkerProtocol.h', 'native/worker/WorkerCleanup.h', 'native/worker/MediaWriter.h',
        'native/worker/ChildProcess.cpp', 'native/NativeDesktopBackend.mm'}
    require(isinstance(rows, list) and len(rows) == 12 and all(isinstance(row, dict) for row in rows)
            and {row.get('path') for row in rows} == required_sources,
            'The exact twelve reviewed lease/transport/stop source bindings are required')
    frozen = {row['path']: row['sha256'] for row in flatpak['files']}
    for row in rows:
        require(snapshot['files'].get(row['path']) == frozen.get(row['path']) == row['sha256'],
                'Native and Flatpak source snapshots differ for a reviewed lease/stop primitive')
    viewer_rows = binding.get('flatpak_only_sources')
    require(isinstance(viewer_rows, list) and len(viewer_rows) == 2
            and {row.get('path') for row in viewer_rows if isinstance(row, dict)}
                == {'source/DesktopViewerSurface.cpp', 'source/ProcessChannel.cpp'},
            'The actual Qt viewer renewal/channel implementation must bind the qualified Flatpak module')
    for row in viewer_rows:
        require(frozen.get(row['path']) == row['sha256'], 'Qualified viewer renewal/channel source changed')
    return binding


def verify_plan(plan):
    required = {'schema', 'run_id', 'source_pins', 'configuration', 'provider_receipt', 'facade',
                'source_binding', 'expected_display', 'ui_template', 'installation_baseline', 'registry', 'effects', 'controller_sha256'}
    require(isinstance(plan, dict) and set(plan) == required and plan['schema'] == SCHEMA,
            'Unknown exact no-input expiry plan')
    registry = pinned_json(plan['registry'])
    require(isinstance(registry, dict), 'Captured product registry metadata must be an object')
    require(isinstance(plan['configuration'], dict) and isinstance(plan['facade'], dict)
            and set(plan['facade']) in [{'selection', 'configuration'}, {'selection', 'configuration', 'protocol'}]
            and isinstance(plan['facade']['configuration'], dict), 'Unknown explicit GUI/provider selections')
    require(plan['facade'].get('protocol', 'nonce') in ['nonce', 'nonce-v2'],
            'Select an explicit versioned no-input nonce facade protocol')
    require(digest(__file__) == plan['controller_sha256'], 'Expiry controller source changed')
    selection = plan['facade'].get('selection')
    require(isinstance(selection, dict) and set(selection) == {'cli', 'archive', 'member'}
            and isinstance(selection['member'], str) and selection['member']
            and not selection['member'].startswith('/') and '\\' not in selection['member']
            and '..' not in Path(selection['member']).parts, 'Select one exact contained provider workflow member')
    for record in [selection['cli'], selection['archive']]:
        require(isinstance(record, dict) and set(record) == {'path', 'sha256'}
                and Path(record['path']).is_absolute() and digest(record['path']) == record['sha256'],
                'Provider workflow/CLI artifact changed')
    require(selection['cli'] == plan['facade']['configuration'].get('operator_cli'),
            'Compiled workflow and owner operator must use the same explicitly reviewed CLI')
    require(plan['facade']['configuration'].get('qualification_mode') == 'lease-expiry-view-only',
            'This controller admits no generated input mode')
    pins = plan['source_pins']
    require(isinstance(pins, list) and 12 <= len(pins) <= 64, 'Expected bounded complete qualification source pins')
    paths = set()
    for record in pins:
        require(isinstance(record, dict) and set(record) == {'path', 'sha256'}
                and isinstance(record['path'], str) and Path(record['path']).is_absolute()
                and digest(record['path']) == record['sha256'], 'Selected source/artifact pin changed')
        require(record['path'] not in paths, 'Duplicate qualification source pin')
        paths.add(record['path'])
    repository = HERE.parents[4]
    required_paths = {str(HERE / name) for name in ['installed-remote.py', 'installed-remote-watchdog.py',
        'installed-remote-expiry.py', 'installed-remote-capture.py', 'installed-remote-journal.py',
        'installed-product-gui.py', 'installed-cli-process.py']}
    required_paths.update(str(repository / relative) for relative in [
        'workflows/qualify/product/platforms/linux/desktop.py',
        'workflows/qualify/product/assets/scripts/contracts.py',
        'workflows/qualify/product/assets/scripts/process.py',
        'workflows/qualify/product/drivers/portable.py'])
    require(required_paths <= paths, 'An executing qualification source/helper is missing its exact pin')
    template = plan['ui_template']
    require(isinstance(template, dict) and set(template) == {'image', 'identity', 'result', 'cleanup', 'actions'},
            'Expected the independently observed normal-GUI View-mode fixture evidence')
    identity = pinned_json(template['identity'])
    result = pinned_json(template['result'])
    cleanup = pinned_json(template['cleanup'])
    actions = pinned_json(template['actions'])
    require(identity['commit'] == plan['configuration']['installed_commit']
            and identity['executable_sha256'] == plan['configuration']['gui_sha256']
            and result.get('status') == 'passed' and result['facts']['gui_exit_normal'] is True
            and result['facts']['clipboard_preferences_verified'] is True
            and result['facts']['remote_session_opened'] is False
            and cleanup['recorded_pids_absent'] is True and cleanup['runtime_absent'] is True
            and cleanup['socket_directory_absent'] is True and cleanup['real_default_profile_absent'] is True,
            'The View template is not bound to a normally closed, read-only exact installed-GUI fixture')
    require(isinstance(actions, list) and len(actions) == 6
            and actions[-2].get('purpose') == 'choose-view-mode'
            and actions[-2].get('action') == 'click' and actions[-2].get('x') == 480 and actions[-2].get('y') == 90
            and actions[-1].get('purpose') == 'normal-quit', 'Actual mouse-only View selection evidence is required')
    image = template['image']
    require(isinstance(image, dict) and set(image) == {'path', 'sha256'}
            and digest(image['path']) == image['sha256'], 'Actual observed View-mode template changed')
    display = plan['expected_display']
    require(isinstance(display, dict) and set(display) == {'display_id', 'bounds', 'evidence'}
            and display['display_id'] == plan['facade']['configuration']['capture_display_id'],
            'Fixture and independently selected main display differ')
    display_evidence = pinned_json(display['evidence'])
    require(display_evidence.get('status') == 'passed' and display_evidence['display']['display_id'] == display['display_id']
            and display_evidence['display']['bounds'] == display['bounds']
            and display_evidence['fixture_ui_launched'] is False and display_evidence['capture_input_clipboard_started'] is False,
            'The selected main display lacks the independent read-only native metadata proof')
    bound_native_sources(plan)
    baseline = pinned_json(plan['installation_baseline'])
    require(isinstance(baseline, dict), 'An independently installed app tuple is required')
    configuration = plan['configuration']
    for key in ['app_id', 'branch', 'installation', 'installed_commit', 'gui_sha256', 'ui_library_sha256',
                'cli_sha256', 'browser_sha256', 'runtime_commit']:
        require(baseline.get(key) == configuration.get(key) and baseline.get(key) is not None,
                'Selected installed app tuple differs from its accepted baseline: ' + key)
    bundle = configuration.get('bundle')
    require(isinstance(bundle, dict) and set(bundle) == {'path', 'sha256'}
            and digest(bundle['path']) == bundle['sha256'] == baseline.get('bundle_sha256'),
            'Immutable bundle bytes differ from the selected installed baseline')
    return {'provider': pinned_json(plan['provider_receipt'])['facts']}


UI_TEXT_REGIONS = {
    'selected-paired-node': (230, 104, 425, 128),
    'view-mode': (448, 104, 522, 128),
    'provider-ready': (220, 225, 745, 249),
}


def compare_preconnection_ui(current, template):
    require(current.mode == template.mode == 'RGB' and current.size == template.size == (1600, 1000),
            'Actual pre-connection GUI geometry/pixel format differs from the observed template')
    facts = {}
    for name, rectangle in UI_TEXT_REGIONS.items():
        expected = template.crop(rectangle).tobytes()
        observed = current.crop(rectangle).tobytes()
        require(len(set(expected)) >= 16, 'The observed UI template must contain actual nonuniform local text')
        require(observed == expected, 'Actual normal GUI ' + name + ' differs from the independently observed View-mode template')
        facts[name] = {'matched': True, 'pixel_sha256': hashlib.sha256(observed).hexdigest()}
    return facts


def marker_inside_owned_view(observed, original_window, current_windows, visible_owned_rect=None):
    matches = [window for window in current_windows if window['id'] == original_window['id']]
    require(len(matches) == 1 and all(matches[0][key] == original_window[key]
            for key in ['x', 'y', 'width', 'height']), 'Original installed GUI window identity/geometry changed')
    rectangle = observed['rect']
    require(rectangle['x'] >= 196 and rectangle['y'] >= 250
            and rectangle['x'] + rectangle['width'] < original_window['width']
            and rectangle['y'] + rectangle['height'] < min(1000, original_window['height']),
            'Fresh whole-nonce marker is outside the actual owned GUI viewer body')
    if visible_owned_rect is not None:
        visible = visible_owned_rect
        require(rectangle['x'] >= visible['x'] and rectangle['y'] >= visible['y']
                and rectangle['x'] + rectangle['width'] <= visible['x'] + visible['width']
                and rectangle['y'] + rectangle['height'] <= visible['y'] + visible['height'],
                'Whole fresh nonce extends outside the capture-time visible owned intersection')


def one_paired_identity(value, target):
    expected = {'node': target['node'], 'fingerprint': target['fingerprint'], 'status': 'paired'}
    require(isinstance(value, list) and value == [expected],
            'Normal GUI chooser must contain exactly the independently admitted paired identity')



class Watchdog:
    def __init__(self, module, desktop, identity, process, output, run_id):
        self.output = output
        self.gui_wrapper = process
        output.mkdir(mode=0o700)
        token = module.identity(process.pid)
        require(token is not None, 'Original owned GUI wrapper is unavailable')
        request = {'schema': 'dockpipe.product.owned-gui-watchdog/v1', 'run_id': run_id,
                   'pid_namespace': os.readlink('/proc/self/ns/pid'),
                   'wrapper': {'pid': token['pid'], 'starttime': token['starttime']},
                   'gui': {'pid': identity['host_pid'], 'starttime': identity['kernel_starttime'],
                           'commit': identity['commit'], 'sha256': identity['executable_sha256']}}
        write(output / 'request.json', request)
        self.log = (output / 'stderr').open('xb')
        self.process = subprocess.Popen([sys.executable, '-I', str(HERE / 'installed-remote-watchdog.py'),
            '--request', str(output / 'request.json'), '--output', str(output / 'observer')],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=self.log,
            env={'PATH': '/usr/bin:/bin', 'HOME': desktop.environment['HOME'], 'LANG': 'C.UTF-8'},
            start_new_session=True)
        try:
            ready, _, _ = select.select([self.process.stdout], [], [], 3)
            require(ready, 'Independent GUI watchdog did not become ready')
            data = self.process.stdout.readline(4097)
            require(len(data) <= 4096, 'Independent GUI watchdog response exceeded its bound')
            event = json.loads(data)
            require(event.get('stage') == 'ready' and event.get('run_id') == run_id,
                    'Independent GUI watchdog binding changed')
            self.run_id = run_id
        except BaseException:
            self.process.stdin.close()
            try:
                self.process.wait(timeout=5)
            finally:
                self.process.stdout.close()
                self.log.close()
            raise

    def arm(self, cutoff):
        self.process.stdin.write((json.dumps({'run_id': self.run_id,
            'terminate_at_monotonic_ns': int(cutoff * 1_000_000_000)}) + '\n').encode())
        self.process.stdin.flush()
        self.unassisted()

    def unassisted(self):
        require(self.process.poll() is None and not (self.output / 'observer/intervention.json').exists(),
                'Watchdog assistance occurred before independent capture expiry')

    def finish(self):
        if self.process.stdin is not None and not self.process.stdin.closed:
            self.process.stdin.close()
        deadline = time.monotonic() + 5
        while self.process.poll() is None and time.monotonic() < deadline:
            # Reap the controller's GUI child so its zombie cannot keep the
            # independently checked owned process group artificially present.
            self.gui_wrapper.poll()
            time.sleep(0.02)
        self.process.wait(timeout=max(0.01, deadline - time.monotonic()))
        self.process.stdout.close()
        self.log.close()
        result = json.loads((self.output / 'observer/result.json').read_text())
        require(result['owned_gui_group_absent'] is True and not result['remaining']
                and not result['ownership_observation_errors'], 'Exact GUI/carrier cleanup remains unproven')
        return result


def module_loaded(identity, expected_path, expected_sha256):
    root = Path('/proc') / str(identity['host_pid'])
    lines = (root / 'maps').read_text().splitlines()
    require(len(lines) <= 8192, 'Owned GUI module map exceeded its bound')
    selected = set()
    for line in lines:
        fields = line.split(maxsplit=5)
        if len(fields) == 6 and fields[5] == expected_path:
            selected.add(fields[5])
    require(len(selected) == 1, 'Expected exactly one actual GUI-loaded desktop provider module')
    path = root / 'root' / next(iter(selected)).lstrip('/')
    require(digest(path) == expected_sha256, 'Actual GUI-loaded provider module differs from the qualified artifact')
    return {'module_sha256': expected_sha256, 'mapped_path': next(iter(selected))}


def control_cli_identity(consumer, wrapper_pid, expected_sha256, expected_state):
    """Observe only this GUI's installed public desktop-control CLI, never its data."""
    matches = []
    for pid, starttime in consumer.owned_descendants(wrapper_pid).items():
        root = Path('/proc') / str(pid)
        try:
            if not os.readlink(root / 'exe').endswith('/dockpipe') or digest(root / 'exe') != expected_sha256:
                continue
            with (root / 'cmdline').open('rb') as stream:
                data = stream.read(65537)
            require(len(data) <= 65536, 'Owned installed CLI argument metadata exceeded its bound')
            arguments = data.rstrip(b'\0').split(b'\0')
            if arguments[1:4] != [b'remote', b'desktop', b'--stdio']:
                continue
            require(arguments.count(b'--state') == 1 and arguments.index(b'--state') + 1 < len(arguments)
                    and arguments[arguments.index(b'--state') + 1] == os.fsencode(expected_state),
                    'Actual installed GUI control CLI selected another paired-state scope')
            require((root / 'stat').read_text().rsplit(')', 1)[1].split()[19] == starttime,
                    'Original installed control CLI kernel identity changed')
            matches.append({'host_pid': pid, 'kernel_starttime': starttime,
                            'executable_sha256': expected_sha256, 'public_command': 'remote desktop --stdio',
                            'selected_state_matches': True, 'request_or_credentials_read': False})
        except (FileNotFoundError, ProcessLookupError):
            continue
    require(len(matches) == 1, 'Expected exactly one actual installed GUI desktop-control CLI')
    return matches[0]


def gui_scope_environment(identity, expected):
    """Inspect only generated app-scope keys; never retain the process environment."""
    path = Path('/proc') / str(identity['host_pid']) / 'environ'
    with path.open('rb') as stream:
        data = stream.read((1 << 20) + 1)
    require(len(data) <= 1 << 20, 'Owned GUI environment metadata exceeded its bound')
    observed = {}
    for entry in data.split(b'\0'):
        name, separator, value = entry.partition(b'=')
        key = name.decode(errors='replace')
        if separator and key in expected:
            require(key not in observed, 'Duplicate generated GUI scope environment key')
            observed[key] = value.decode()
    require(observed == expected, 'Actual installed GUI paired-state/package/CLI/home scope differs from its private launch')
    return {'matched_keys': sorted(expected), 'actual_values_retained': False}


class ExpiryOperation:
    def __init__(self, plan, output, consumer, gui, provider):
        self.plan = plan
        self.output = output
        self.consumer = consumer
        self.gui = gui
        self.provider = provider
        self.journal = load('expiry_private_journal', HERE / 'installed-remote-journal.py')
        self.expiry = load('expiry_private_pause', HERE / 'installed-remote-expiry.py')
        self.capture = load('expiry_private_capture', HERE / 'installed-remote-capture.py')
        self.watchdog_module = load('expiry_private_watchdog', HERE / 'installed-remote-watchdog.py')
        self.facade = None
        self.watchdog = None
        self.pause = None
        self.connected = False
        self.capture_reader = None

    def current(self):
        observed = self.consumer.admitted_nested_gui(self.desktop.environment,
            self.plan['configuration']['installed_commit'], self.plan['configuration']['gui_sha256'], self.process.pid)
        require(observed == self.identity, 'Original installed GUI identity changed')
        return self.gui.guided_windows(self.desktop.environment, observed)

    def local(self, action, x=None, y=None, key=None):
        windows = self.current()
        if action == 'click':
            require(type(x) is int and type(y) is int and 0 <= x < 1180 and 0 <= y < 240,
                    'Expiry observer may click only the established local chrome')
            command = ['/usr/bin/xdotool', 'mousemove', '--sync', str(x), str(y), 'click', '1']
        else:
            raise ValueError('No keyboard event is admitted by the view-only expiry controller')
        require(any(window['x'] == 0 and window['y'] == 0 and window['width'] == 1180 for window in windows),
                'The original app chrome geometry changed')
        subprocess.run(command, env=self.desktop.environment, stdin=subprocess.DEVNULL,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True, timeout=1)

    def screenshot_before_capture(self, name):
        require(not self.connected, 'Never persist a whole GUI screenshot after capture begins')
        path = self.output / (name + '.png')
        self.consumer.capture_owned_display(self.desktop.environment).save(path)
        path.chmod(0o600)
        return {'path': str(path), 'sha256': digest(path)}

    def prepare_ui(self):
        # The exact immutable app layout was observed in normal paired preparation.
        windows = self.current()
        require(len(windows) == 1, 'Preparation requires exactly one original owned main window')
        self.main_window = dict(windows[0])
        time.sleep(3)
        subprocess.run(['/usr/bin/xdotool', 'mousemove', '--sync', '90', '674', 'click', '1'],
                       env=self.desktop.environment, check=True, timeout=1)
        time.sleep(2)
        self.screenshot_before_capture('before-peer-selection')
        self.local('click', x=320, y=115)
        # Match the deliberate, separately observed popup gestures. A second
        # rapid click was observed to leave the displayed row unselected.
        time.sleep(0.9)
        self.screenshot_before_capture('paired-computer-dropdown')
        # The plan's independently observed catalog must have exactly this one
        # eligible node. No text typing, alias fallback or arbitrary menu key.
        self.local('click', x=320, y=117)
        time.sleep(0.5)
        self.local('click', x=480, y=115)
        time.sleep(0.9)
        self.local('click', x=480, y=90)
        time.sleep(0.3)
        self.screenshot_before_capture('selected-peer-view-mode')
        self.verify_preconnection_ui()

    def verify_preconnection_ui(self):
        require(not self.connected and len(self.current()) == 1,
                'The pre-connection template requires exactly the original unobstructed GUI')
        from PIL import Image
        record = self.plan['ui_template']['image']
        require(digest(record['path']) == record['sha256'], 'Observed pre-connection UI template changed')
        current = self.consumer.capture_owned_display(self.desktop.environment)
        with Image.open(record['path']) as source:
            template = source.convert('RGB')
        facts = compare_preconnection_ui(current, template)
        del current
        del template
        return facts

    def reference(self):
        values = self.journal.read(self.settings, self.home)
        require(len(values) <= 1, 'Unexpected existing/multiple GUI references; never touch foreign sessions')
        return values[0] if values else None

    def __call__(self, desktop, identity, process, home, selected):
        self.desktop, self.identity, self.process, self.home = desktop, identity, process, home
        self.settings = home / '.var/app' / self.consumer.APP_ID / 'config/dockpipe/dockpipe-launcher.conf'
        require(not self.journal.read(self.settings, home), 'Fresh GUI journal is not empty')
        self.consumer.remote_preferences(self.settings)
        scope = gui_scope_environment(identity, {'HOME': str(home), 'DOCKPIPE_BIN': '/app/bin/dockpipe',
            'DOCKPIPE_GLOBAL_ROOT': str(Path(self.plan['facade']['configuration']['state']).parent),
            'DOCKPIPE_PACKAGES_ROOT': self.provider['package_root']})
        write(self.output / 'actual-gui-scope.json', scope)
        result = {'clipboard_preferences_verified': True, 'remote_session_opened': False,
                  'capture_or_input_started': False, 'generated_input_observed': False,
                  'frames_observed': False, 'remote_closure_verified': False, 'gui_exit_normal': False}
        startup_facts = {'clock_domain': 'host-observer-monotonic', 'validation_complete': False}
        self.facade = self.consumer.FacadeClient(self.plan['facade']['selection'],
            self.plan['facade']['configuration'], self.output / 'provider-owner',
            protocol=self.plan['facade'].get('protocol', 'nonce'), wait_process=process)
        try:
            inspection = self.facade.command('inspect')
            one_paired_identity(inspection['paired_nodes'], self.plan['facade']['configuration']['desktop_target'])
            write(self.output / 'prelaunch-owner-inspection.json', inspection)
            self.prepare_ui()
            write(self.output / 'actual-preconnection-ui.json', self.verify_preconnection_ui())
            self.watchdog = Watchdog(self.watchdog_module, desktop, identity, process,
                                     self.output / 'watchdog', self.plan['run_id'])
            # These preparations do not read pixels. Keep archive hashing and
            # capture setup outside the fixture's unchanged startup interval.
            marker = self.marker_contract()
            xvfb = next(child for child in desktop.processes if child.args[0] == desktop.selections['xvfb'])
            self.capture_reader = self.capture.OwnedDisplayCapture(desktop.environment,
                {'pid': xvfb.pid, 'starttime': desktop.process_starttime(xvfb.pid)}, desktop.output)
            prepared = self.facade.command('prepare')
            one_paired_identity(prepared['paired_nodes'], self.plan['facade']['configuration']['desktop_target'])
            require(prepared['catalog']['target'] == self.plan['facade']['configuration']['desktop_target'],
                    'The original GUI peer selection differs from current admitted catalog')
            admitted = self.facade.command('capture-gate')
            require(admitted['geometry']['display_rect'] == self.plan['expected_display']['bounds'],
                    'Current selected main display bounds changed; no capture admitted')
            budget = admitted['budget']
            self.watchdog.arm(budget['terminate_at_monotonic'])
            # Opening the original pidfd sends no signal. Prepare it before the
            # final frame so capture-to-pause has no extra identity setup.
            self.pause = self.expiry.GuiLeaseExpiry({'pid': identity['host_pid'],
                'starttime': identity['kernel_starttime']}, budget['terminate_at_monotonic'])
            require(time.monotonic() < budget['input_before_monotonic'], 'Fresh capture admission expired before Connect')
            self.verify_preconnection_ui()
            self.connected = True  # No whole screenshot/keyboard is permitted from this point.
            self.local('click', x=580, y=115)
            result['connection_requested'] = True
            reference = None
            until = min(budget['input_before_monotonic'], time.monotonic() + 5)
            while time.monotonic() < until:
                reference = self.reference()
                if reference is not None:
                    break
                time.sleep(0.03)
            require(reference is not None, 'Original GUI-created Core reference was not journaled before the cutoff')
            bound = self.facade.command('bind-reference', reference=reference)
            status = bound['core']
            reuse_running_gate = self.plan['facade'].get('protocol') == 'nonce-v2'
            startup = None
            while time.monotonic() < budget['input_before_monotonic']:
                if reuse_running_gate:
                    startup = self.facade.command('startup-frame-status')
                    status = startup['core']
                elif status['state'] in ['queued', 'opening']:
                    status = self.facade.command('expiry-status')['core']
                if status['state'] not in ['queued', 'opening']:
                    break
                time.sleep(0.03)
            require(status['state'] == 'running', 'Original GUI reference did not become running before startup cutoff')
            result.update(remote_session_opened=True, capture_or_input_started=True)
            if reuse_running_gate:
                require(startup is not None and isinstance(startup.get('frame_admission'), dict),
                        'Running startup observation has no original single-use frame admission')
                window = {name: self.main_window[name] for name in ['id', 'x', 'y', 'width', 'height']}
                window['gui'] = {'pid': identity['host_pid'], 'starttime': identity['kernel_starttime'],
                                 'namespace_pid': identity['namespace_pids'][-1]}
                with capture_and_pause(self.facade, startup['frame_admission'], self.verify_installed_viewer,
                                       self.capture_reader, window, self.pause, self.watchdog,
                                       startup_facts) as (frame, identities, paused):
                    write(self.output / 'gui-pause.json', paused)
                    observed = validate_paused_frame(self.capture_reader, frame, self.pause,
                                                      marker, prepared['binding']['nonce'],
                                                      self.current, startup_facts)
            else:
                identities = self.verify_installed_viewer()
                gate = self.facade.command('frame-gate', phase=0)
                frame_deadline = time.monotonic() + gate['local_capture_budget_milliseconds'] / 1000
                image = self.capture_reader.capture({'x': 0, 'y': 0, 'width': 1600, 'height': 1000})
                require(time.monotonic() < frame_deadline,
                        'Admitted fixture frame capture exceeded its fresh coverage gate')
                observed = marker.locate(image, prepared['binding']['nonce'], 0)
                del image
                marker_inside_owned_view(observed, self.main_window, self.current())
            write(self.output / 'actual-installed-control-cli.json', identities['control_cli'])
            write(self.output / 'actual-loaded-provider.json', identities['provider'])
            write(self.output / 'actual-gui-fresh-marker.json', observed)
            result['frames_observed'] = True
            if not reuse_running_gate:
                self.watchdog.unassisted()
                require(time.monotonic() <= budget['terminate_at_monotonic'] - 12,
                        'Viewer startup left insufficient unassisted local-lease observation lifetime; never extend the cutoff')
                write(self.output / 'gui-pause.json', self.pause.pause())
            proof = None
            while time.monotonic() < budget['terminate_at_monotonic']:
                self.watchdog.unassisted()
                observation = self.facade.command('expiry-status')
                if observation['native'] is not None:
                    self.watchdog.unassisted()
                    proof = self.pause.observe(reference, observation['core'], observation['native'])
                    self.watchdog.unassisted()
                    write(self.output / 'unassisted-expiry.json', proof)
                    break
                time.sleep(0.1)
            require(proof is not None, 'No unassisted native expiry was proven before the hard cutoff')

            def quit_gui():
                self.local('click', x=1146, y=24)
                process.wait(timeout=3)
                require(process.returncode == 0, 'Original installed GUI did not quit normally')
                require(not self.journal.read(self.settings, home),
                        'Normal GUI retained its original recovery reference; preserve it without manual clearing')
                result['gui_exit_normal'] = True

            closed = finish_unassisted_gui(
                proof, self.watchdog, self.pause, quit_gui, lambda: self.facade.command('close'))
            self.pause = None
            require(closed['core_closed'] and closed['native_idle'] and closed['fixture_pid_absent']
                    and not closed['emergency_close_attempted'] and not closed['errors'],
                    'Exact original Core/native/fixture closure is unproven')
            result['remote_closure_verified'] = True
            result['reason'] = 'Actual normal installed viewer rendered fresh generated nonce; unassisted lease-loss closure observed; no input'
            return result
        finally:
            primary_error = sys.exc_info()[1]
            cleanup_errors = []
            resources = [
                ('independent-watchdog', self.watchdog.finish if self.watchdog is not None else None),
                ('original-GUI-pause', self.pause.close if self.pause is not None else None),
                ('owned-display-reader', self.capture_reader.close if self.capture_reader is not None else None),
                ('provider-owner', self.facade.stop if self.facade is not None else None),
            ]
            for name, close in resources:
                if close is None:
                    continue
                try:
                    facts = close()
                    if name == 'independent-watchdog':
                        write(self.output / 'watchdog-cleanup.json', facts)
                except (Exception, KeyboardInterrupt) as error:
                    cleanup_errors.append({'resource': name, 'error': str(error)})
            write(self.output / 'operation-cleanup.json', {'errors': cleanup_errors,
                  'primary_error': str(primary_error) if primary_error is not None else None})
            # Facts contain clocks/window metadata only. Raw XImage/PIL buffers
            # were freed by the frame context, even when verification failed.
            write(self.output / 'startup-frame-timings.json', startup_facts)
            if primary_error is None:
                require(not cleanup_errors, 'Some original owned resources failed independent cleanup')

    def verify_installed_viewer(self):
        control = control_cli_identity(
            self.consumer, self.process.pid, self.plan['configuration']['cli_sha256'],
            self.plan['facade']['configuration']['state'])
        display = self.provider['installation']['provider']['surface']['native_display']
        require(display['interface'] == 1, 'The normal installed catalog viewer interface changed')
        provider = module_loaded(self.identity, display['libraries']['linux'],
                                 self.plan['configuration']['qualified_provider']['module_sha256'])
        verified = time.monotonic()
        return {'control_cli': dict(control, verified_monotonic=verified),
                'provider': dict(provider, verified_monotonic=verified)}

    def marker_contract(self):
        import tarfile
        archive = self.plan['facade']['selection']['archive']
        require(digest(archive['path']) == archive['sha256'], 'Provider archive changed before marker observation')
        name = 'workflows/remote-desktop.qualify-live/assets/scripts/marker_contract.py'
        with tarfile.open(archive['path']) as bundle:
            matches = [member for member in bundle.getmembers() if member.name == name]
            require(len(matches) == 1 and matches[0].isfile() and matches[0].size <= 65536,
                    'Exact versioned marker contract member is unavailable')
            data = bundle.extractfile(matches[0]).read(65537)
        path = self.output / 'provider-marker-contract.py'
        path.write_bytes(data)
        path.chmod(0o600)
        return load('expiry_provider_marker_contract', path)


def require_runtime(configuration):
    require(isinstance(configuration.get('platform_ref'), str)
            and isinstance(configuration.get('runtime_commit'), str)
            and len(configuration['runtime_commit']) == 64, 'Exact installed runtime reference/commit required')
    environment = {'PATH': '/usr/bin:/bin', 'HOME': configuration['private_home'],
                   'FLATPAK_USER_DIR': configuration['installation']}
    observed = subprocess.run(['/usr/bin/flatpak', 'info', '--user', '--show-commit', configuration['platform_ref']],
        env=environment, stdin=subprocess.DEVNULL, capture_output=True, text=True, timeout=5)
    require(observed.returncode == 0 and len(observed.stdout) <= 1024 and len(observed.stderr) <= 8192
            and observed.stdout.strip() == configuration['runtime_commit'], 'Installed platform runtime commit changed')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--plan', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--execute-reviewed', action='store_true')
    parser.add_argument('--approve-plan-sha256')
    arguments = parser.parse_args()
    data, plan = captured_json(arguments.plan)
    verified = verify_plan(plan)
    if not arguments.execute_reviewed:
        print(json.dumps({'check_only': True, 'plan_sha256': hashlib.sha256(data).hexdigest(),
                          'fixture_ui_launched': False, 'capture_or_input_started': False}))
        return
    require(arguments.approve_plan_sha256 == hashlib.sha256(data).hexdigest(), 'Exact reviewed live plan digest required')
    require(arguments.output.is_absolute() and not arguments.output.exists(), 'Fresh private fault output required')

    def interrupted(number, _frame):
        raise KeyboardInterrupt('View-only expiry operation interrupted: ' + str(number))

    for selected_signal in [signal.SIGTERM, signal.SIGHUP]:
        signal.signal(selected_signal, interrupted)
    arguments.output.mkdir(mode=0o700)
    write(arguments.output / 'plan-captured.json', plan)
    write(arguments.output / 'observer-namespace.json', {
        'schema': 'dockpipe.private.gui-observer-namespace/v1',
        'pid_namespace': os.readlink('/proc/self/ns/pid'),
        'mount_namespace': os.readlink('/proc/self/ns/mnt'),
        'procfs_device': Path('/proc').stat().st_dev, 'observer_pid': os.getpid()})
    try:
        consumer = load('expiry_gui_consumer', HERE / 'installed-remote.py')
        configuration = dict(plan['configuration'], mode='lease-expiry', private_home=str(arguments.output / 'identity-home'))
        Path(configuration['private_home']).mkdir(mode=0o700)
        installed_location = consumer.require_installed(configuration)
        require(digest(installed_location / 'files/lib/libSpatialWebBrowser.so.0.1.0') == configuration['browser_sha256'],
                'Installed browser dependency differs from the admitted immutable app tuple')
        require_runtime(configuration)
        provider = verified['provider']
        repository, contracts, process, portable, gui, observer = consumer.owner_modules()
        operation = ExpiryOperation(plan, arguments.output, consumer, gui, provider)
        result = consumer.normal_gui({}, configuration, provider, arguments.output, repository, gui, operation=operation)
        verify_plan(plan)
        require_runtime(configuration)
        require(result['frames_observed'] and result['remote_closure_verified'] and result['gui_exit_normal']
                and result['local_cleanup_verified'], 'Actual expiry fault acceptance remained incomplete')
        write(arguments.output / 'result.json', {'status': 'passed', 'facts': result,
            'generated_remote_input_sent': False, 'installed_control_acceptance_claimed': False,
            'product_case_acceptance_claimed': False, 'registry_sha256': plan['registry']['sha256']})
    except BaseException as error:
        write(arguments.output / 'result.json', {'status': 'failed', 'error': str(error),
              'traceback': traceback.format_exc(), 'installed_control_acceptance_claimed': False,
              'product_case_acceptance_claimed': False, 'registry_sha256': plan['registry']['sha256']})
        raise


if __name__ == '__main__':
    main()
