"""Optional pinned sterile Xvfb integration; never connects to an existing display."""
from pathlib import Path
import hashlib
import importlib.util
import json
import os
import secrets
import select
import signal
import stat
import subprocess
import sys
import tempfile
import textwrap
import time
import unittest

SCRIPTS = Path(__file__).resolve().parents[1] / 'assets/scripts'


def load(name, filename):
    specification = importlib.util.spec_from_file_location(name, SCRIPTS / filename)
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


CAPTURE = load('sterile_x11_capture_tests', 'installed-remote-capture.py')
EXPIRY = load('sterile_x11_pause_tests', 'installed-remote-expiry.py')

WINDOW_PROGRAM = textwrap.dedent('''
    import ctypes
    import json
    import os
    import sys

    library = ctypes.CDLL('libX11.so.6')
    library.XOpenDisplay.argtypes = [ctypes.c_char_p]
    library.XOpenDisplay.restype = ctypes.c_void_p
    library.XDefaultRootWindow.argtypes = [ctypes.c_void_p]
    library.XDefaultRootWindow.restype = ctypes.c_ulong
    library.XCreateSimpleWindow.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.c_int,
        ctypes.c_int, ctypes.c_uint, ctypes.c_uint, ctypes.c_uint, ctypes.c_ulong, ctypes.c_ulong]
    library.XCreateSimpleWindow.restype = ctypes.c_ulong
    library.XInternAtom.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_int]
    library.XInternAtom.restype = ctypes.c_ulong
    library.XChangeProperty.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.c_ulong,
        ctypes.c_ulong, ctypes.c_int, ctypes.c_int, ctypes.c_void_p, ctypes.c_int]
    library.XMapWindow.argtypes = [ctypes.c_void_p, ctypes.c_ulong]
    library.XMoveWindow.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.c_int, ctypes.c_int]
    library.XDestroyWindow.argtypes = [ctypes.c_void_p, ctypes.c_ulong]
    library.XSync.argtypes = [ctypes.c_void_p, ctypes.c_int]
    library.XCloseDisplay.argtypes = [ctypes.c_void_p]
    display = library.XOpenDisplay(os.environ['DISPLAY'].encode())
    assert display
    root = library.XDefaultRootWindow(display)
    height = int(os.environ.get('GENERATED_WINDOW_HEIGHT', '200'))
    assert height in [200, 1008]
    window = library.XCreateSimpleWindow(display, root, 10, 20, 300, height, 0, 0, 0x336699)
    assert window
    atom = library.XInternAtom(display, b'_NET_WM_PID', 0)
    pid = ctypes.c_ulong(os.getpid())
    library.XChangeProperty(display, window, atom, 6, 32, 0, ctypes.byref(pid), 1)
    library.XMapWindow(display, window)
    library.XSync(display, 0)
    print(json.dumps({'window_id': window, 'pid': os.getpid()}), flush=True)
    try:
        for line in sys.stdin:
            command = line.strip()
            if command == 'quit':
                break
            if command == 'move':
                library.XMoveWindow(display, window, 30, 40)
            elif command == 'destroy':
                library.XDestroyWindow(display, window)
                window = 0
            else:
                raise ValueError('Unknown generated fixture command')
            library.XSync(display, 0)
            print(json.dumps({'completed': command}), flush=True)
    finally:
        if window:
            library.XDestroyWindow(display, window)
        library.XCloseDisplay(display)
''')


@unittest.skipUnless(sys.platform.startswith('linux') and os.environ.get('DOCKPIPE_TEST_XVFB'),
                     'Explicit pinned sterile Xvfb input is required')
class SterileX11Tests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.binary = Path(os.environ['DOCKPIPE_TEST_XVFB'])
        expected = os.environ.get('DOCKPIPE_TEST_XVFB_SHA256')
        if not cls.binary.is_absolute() or hashlib.sha256(cls.binary.read_bytes()).hexdigest() != expected:
            raise ValueError('Explicit sterile Xvfb pin changed')
        xauth = Path('/usr/bin/xauth')
        if hashlib.sha256(xauth.read_bytes()).hexdigest() != 'f914cf066df79d535d12721ac117c555ba43efe9a575804d8cc296f5ef23a44b':
            raise ValueError('Sterile xauth pin changed')
        parent = Path(os.environ['DOCKPIPE_TEST_X11_OUTPUT'])
        parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        if parent.is_symlink() or parent.stat().st_uid != os.getuid() or stat.S_IMODE(parent.stat().st_mode) != 0o700:
            raise ValueError('Sterile test output must be an owned private directory')
        cls.output = Path(tempfile.mkdtemp(prefix='sterile-', dir=parent))
        cls.display_number = 780 + secrets.randbelow(100)
        cls.display = ':' + str(cls.display_number)
        for path in [Path('/tmp/.X' + str(cls.display_number) + '-lock'),
                     Path('/tmp/.X11-unix/X' + str(cls.display_number))]:
            if path.exists():
                raise RuntimeError('Selected fresh sterile display is occupied; do not connect or retry')
        authority = cls.output / 'Xauthority'
        authority.touch(mode=0o600)
        cls.environment = {'PATH': '/usr/bin:/bin', 'HOME': str(cls.output), 'LANG': 'C.UTF-8',
                           'DISPLAY': cls.display, 'XAUTHORITY': str(authority)}
        subprocess.run([str(xauth), '-f', str(authority), 'add', cls.display,
                        'MIT-MAGIC-COOKIE-1', secrets.token_hex(16)], env=cls.environment,
                       stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                       check=True, timeout=2)
        cls.log = (cls.output / 'Xvfb.log').open('xb')
        (cls.output / 'Xvfb.log').chmod(0o600)
        cls.server = subprocess.Popen([str(cls.binary), cls.display, '-screen', '0', '1600x1000x24',
            '-nolisten', 'tcp', '-noreset', '-auth', str(authority)], env=cls.environment,
            stdin=subprocess.DEVNULL, stdout=cls.log, stderr=subprocess.STDOUT, start_new_session=True)
        cls.reader = None
        try:
            fields = Path('/proc', str(cls.server.pid), 'stat').read_text().rsplit(')', 1)[1].split()
            deadline = time.monotonic() + 3
            while time.monotonic() < deadline:
                if cls.server.poll() is not None:
                    raise RuntimeError('Fresh pinned sterile Xvfb exited; preserve private log')
                try:
                    cls.reader = CAPTURE.OwnedDisplayCapture(cls.environment,
                        {'pid': cls.server.pid, 'starttime': fields[19]}, cls.output)
                    break
                except ValueError as error:
                    if str(error) != "Could not open this controller's authenticated Xvfb":
                        raise
                    time.sleep(0.02)
            if cls.reader is None:
                raise RuntimeError('Fresh pinned sterile Xvfb did not become ready')
        except BaseException:
            cls.stop_server()
            raise
        cls.receipts = []
        cls.clients = []
        cls.server_starttime = fields[19]

    @classmethod
    def stop_server(cls):
        if cls.reader is not None:
            cls.reader.close()
        if cls.server.poll() is None:
            cls.server.terminate()
        try:
            cls.server.wait(timeout=2)
        except subprocess.TimeoutExpired:
            cls.server.kill()
            cls.server.wait(timeout=1)
        cls.log.close()

    @classmethod
    def tearDownClass(cls):
        cls.stop_server()
        receipt = {'display': cls.display, 'xvfb_pid': cls.server.pid,
                   'xvfb_starttime': cls.server_starttime,
                   'exact_server_reaped': cls.server.returncode is not None,
                   'clients': cls.clients, 'facts': cls.receipts, 'pixels_persisted': False,
                   'existing_display_or_GUI_or_remote_used': False}
        (cls.output / 'metadata-receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')

    def setUp(self):
        self.error_log = (self.output / (self._testMethodName + '.stderr')).open('xb')
        height = 1008 if self._testMethodName == 'test_actual_clipped_bottom_window_preserves_visible_owned_intersection' else 200
        environment = dict(self.environment, GENERATED_WINDOW_HEIGHT=str(height))
        self.child = subprocess.Popen([sys.executable, '-I', '-c', WINDOW_PROGRAM],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=self.error_log,
            env=environment, text=True, start_new_session=True)
        self.addCleanup(self.stop_child)
        self.client_record = {'pid': self.child.pid, 'created_by_this_test': True}
        self.clients.append(self.client_record)
        selected = self.reply()
        self.assertEqual(self.child.pid, selected['pid'])
        fields = Path('/proc', str(self.child.pid), 'stat').read_text().rsplit(')', 1)[1].split()
        namespace = next(line.split()[1:] for line in Path('/proc', str(self.child.pid), 'status').read_text().splitlines()
                         if line.startswith('NSpid:'))
        self.window = {'id': selected['window_id'], 'x': 10, 'y': 20, 'width': 300, 'height': height,
                       'gui': {'pid': self.child.pid, 'starttime': fields[19],
                               'namespace_pid': int(namespace[-1])}}
        self.reader.pid_atom = self.reader.library.XInternAtom(self.reader.display, b'_NET_WM_PID', 1)
        self.pause = None

    def reply(self):
        ready, _, _ = select.select([self.child.stdout], [], [], 2)
        self.assertTrue(ready)
        line = self.child.stdout.readline(4097)
        self.assertTrue(0 < len(line) <= 4096)
        return json.loads(line)

    def command(self, value):
        self.child.stdin.write(value + '\n')
        self.child.stdin.flush()
        self.assertEqual({'completed': value}, self.reply())

    def stop_child(self):
        if self.pause is not None:
            self.pause.close()
        if self.child.poll() is None:
            try:
                self.child.stdin.write('quit\n')
                self.child.stdin.flush()
                self.child.wait(timeout=1)
            except (BrokenPipeError, subprocess.TimeoutExpired):
                self.child.kill()
        self.child.wait(timeout=1)
        self.child.stdin.close()
        self.child.stdout.close()
        self.error_log.close()
        self.assertFalse(Path('/proc', str(self.child.pid)).exists())
        self.client_record.update(reaped=True, proc_entry_absent=True, exit_code=self.child.returncode)

    def test_actual_xlib_bracket_pause_geometry_and_memory_decode(self):
        facts = {}
        self.pause = EXPIRY.GuiLeaseExpiry(
            {'pid': self.child.pid, 'starttime': self.window['gui']['starttime']}, time.monotonic() + 2)
        deadline = time.monotonic() + 0.1
        with self.reader.capture_bracketed({'x': 0, 'y': 0, 'width': 1600, 'height': 1000},
                                          self.window, deadline, deadline, facts) as frame:
            self.pause.pause()
            self.reader.verify_paused_window(frame, self.pause.deadline)
            image = self.reader.decode_frame(frame)
            self.assertEqual((51, 102, 153), image.getpixel((20, 30)))
            del image
        self.assertTrue(facts['image_destroyed_once'])
        self.receipts.append({'test': self._testMethodName, 'capture_seconds':
                              facts['capture_finished_monotonic'] - facts['started_monotonic'],
                              'window_snapshots_match': facts['window_before'] == facts['window_after']})

    def test_actual_destroyed_window_is_typed_failure_without_process_abort(self):
        self.command('destroy')
        deadline = time.monotonic() + 0.1
        with self.assertRaises(ValueError):
            with self.reader.capture_bracketed({'x': 0, 'y': 0, 'width': 1600, 'height': 1000},
                                              self.window, deadline, deadline):
                self.fail('Destroyed window was admitted')

    def test_actual_clipped_bottom_window_preserves_visible_owned_intersection(self):
        deadline = time.monotonic() + 0.1
        with self.reader.capture_bracketed({'x': 0, 'y': 0, 'width': 1600, 'height': 1000},
                                          self.window, deadline, deadline) as frame:
            self.assertEqual(1008, frame['before']['height'])
            self.assertEqual(frame['before'], frame['after'])
            self.assertEqual({'x': 10, 'y': 20, 'width': 300, 'height': 980}, frame['visible_owned_rect'])

    def test_actual_window_move_during_capture_refuses_and_frees_image(self):
        original = self.reader.library.XGetImage
        def capture_then_move(*arguments):
            image = original(*arguments)
            self.command('move')
            return image
        self.reader.library.XGetImage = capture_then_move
        facts = {}
        try:
            deadline = time.monotonic() + 0.1
            with self.assertRaisesRegex(ValueError, 'geometry changed'):
                with self.reader.capture_bracketed({'x': 0, 'y': 0, 'width': 1600, 'height': 1000},
                                                  self.window, deadline, deadline, facts):
                    self.fail('Raced window was admitted')
        finally:
            self.reader.library.XGetImage = original
        self.assertTrue(facts['image_destroyed_once'])


if __name__ == '__main__':
    unittest.main()
