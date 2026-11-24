"""Read only bounded rectangles from this controller's authenticated Xvfb.

Live chrome must be captured as a rectangle at the X server. Generated-marker
frames may be inspected in memory only after provider coverage admission.
"""
from pathlib import Path
from contextlib import contextmanager
import ctypes
import math
import os
import re
import stat
import time


class XImage(ctypes.Structure):
    _fields_ = [
        ('width', ctypes.c_int), ('height', ctypes.c_int), ('xoffset', ctypes.c_int), ('format', ctypes.c_int),
        ('data', ctypes.c_void_p), ('byte_order', ctypes.c_int), ('bitmap_unit', ctypes.c_int),
        ('bitmap_bit_order', ctypes.c_int), ('bitmap_pad', ctypes.c_int), ('depth', ctypes.c_int),
        ('bytes_per_line', ctypes.c_int), ('bits_per_pixel', ctypes.c_int),
        ('red_mask', ctypes.c_ulong), ('green_mask', ctypes.c_ulong), ('blue_mask', ctypes.c_ulong),
        ('obdata', ctypes.c_void_p), ('functions', ctypes.c_void_p * 6),
    ]


class XWindowAttributes(ctypes.Structure):
    _fields_ = [
        ('x', ctypes.c_int), ('y', ctypes.c_int), ('width', ctypes.c_int), ('height', ctypes.c_int),
        ('border_width', ctypes.c_int), ('depth', ctypes.c_int), ('visual', ctypes.c_void_p),
        ('root', ctypes.c_ulong), ('window_class', ctypes.c_int), ('bit_gravity', ctypes.c_int),
        ('win_gravity', ctypes.c_int), ('backing_store', ctypes.c_int),
        ('backing_planes', ctypes.c_ulong), ('backing_pixel', ctypes.c_ulong),
        ('save_under', ctypes.c_int), ('colormap', ctypes.c_ulong), ('map_installed', ctypes.c_int),
        ('map_state', ctypes.c_int), ('all_event_masks', ctypes.c_long),
        ('your_event_mask', ctypes.c_long), ('do_not_propagate_mask', ctypes.c_long),
        ('override_redirect', ctypes.c_int), ('screen', ctypes.c_void_p),
    ]


class XErrorEvent(ctypes.Structure):
    _fields_ = [('type', ctypes.c_int), ('display', ctypes.c_void_p), ('resource_id', ctypes.c_ulong),
                ('serial', ctypes.c_ulong), ('error_code', ctypes.c_ubyte),
                ('request_code', ctypes.c_ubyte), ('minor_code', ctypes.c_ubyte)]


def require(condition, message):
    if not condition:
        raise ValueError(message)


def rectangle(value):
    require(isinstance(value, dict) and set(value) == {'x', 'y', 'width', 'height'}
            and all(type(number) is int for number in value.values()), 'Expected an exact integer capture rectangle')
    require(value['x'] >= 0 and value['y'] >= 0 and value['width'] > 0 and value['height'] > 0
            and value['x'] + value['width'] <= 1600 and value['y'] + value['height'] <= 1000,
            'Capture rectangle escapes the fixed owned Xvfb')
    return value


def window_rectangle(value):
    """Window metadata may extend past the fixed canvas; pixels never do."""
    require(isinstance(value, dict) and set(value) == {'x', 'y', 'width', 'height'}
            and all(type(number) is int for number in value.values()), 'Expected exact integer window geometry')
    require(-4096 <= value['x'] <= 4096 and -4096 <= value['y'] <= 4096
            and 0 < value['width'] <= 4096 and 0 < value['height'] <= 4096,
            'Original window metadata exceeds its bound')
    return value


def visible_intersection(window, selected):
    left = max(window['x'], selected['x'])
    top = max(window['y'], selected['y'])
    right = min(window['x'] + window['width'], selected['x'] + selected['width'])
    bottom = min(window['y'] + window['height'], selected['y'] + selected['height'])
    require(right > left and bottom > top, 'Original window has no visible owned capture intersection')
    return {'x': left, 'y': top, 'width': right - left, 'height': bottom - top}


def validate_image_layout(value, expected):
    require(value.width == expected['width'] and value.height == expected['height']
            and value.xoffset == 0 and value.format == 2 and value.depth == 24
            and value.byte_order in [0, 1] and value.bits_per_pixel in [24, 32]
            and (value.red_mask, value.green_mask, value.blue_mask) == (0xff0000, 0xff00, 0xff),
            'Unexpected owned XImage tuple')
    minimum_stride = (value.width * value.bits_per_pixel + 7) // 8
    require(minimum_stride <= value.bytes_per_line <= value.width * 4 + 16
            and value.bytes_per_line * value.height <= 8 << 20 and value.data,
            'Owned XImage storage exceeds its bound')
    return value.bytes_per_line * value.height


def decode_image(value, expected):
    length = validate_image_layout(value, expected)
    raw = ctypes.string_at(value.data, length)
    from PIL import Image
    encoding = ('BGR' if value.bits_per_pixel == 24 else 'BGRX') if value.byte_order == 0 else (
        'RGB' if value.bits_per_pixel == 24 else 'XRGB')
    return Image.frombytes('RGB', (value.width, value.height), raw, 'raw', encoding, value.bytes_per_line, 1)


class OwnedDisplayCapture:
    def __init__(self, environment, xvfb, private_root):
        private_root = Path(private_root).resolve(strict=True)
        require(private_root.stat().st_uid == os.getuid() and stat.S_IMODE(private_root.stat().st_mode) == 0o700,
                'Capture root must be the private owned fixture')
        authority = Path(environment['XAUTHORITY'])
        require(authority.resolve(strict=True) == authority and authority.is_relative_to(private_root)
                and not authority.is_symlink() and stat.S_ISREG(authority.stat().st_mode)
                and authority.stat().st_uid == os.getuid()
                and stat.S_IMODE(authority.stat().st_mode) == 0o600, 'Only the generated private Xauthority is admitted')
        display_name = environment['DISPLAY']
        require(re.fullmatch(':[0-9]{1,4}', display_name) is not None, 'Only a local owned Xvfb transport is admitted')
        process = Path('/proc') / str(xvfb['pid'])
        with (process / 'stat').open() as stream:
            raw_stat = stream.read(8193)
        require(len(raw_stat) <= 8192, 'Owned Xvfb kernel metadata exceeded bound')
        fields = raw_stat.rsplit(')', 1)[1].split()
        require(fields[19] == xvfb['starttime'] and int(fields[1]) == os.getpid(),
                'Selected Xvfb must be this controller\'s original child')
        with (process / 'cmdline').open('rb') as stream:
            raw_arguments = stream.read(65537)
        require(len(raw_arguments) <= 65536, 'Owned Xvfb arguments exceeded bound')
        arguments = raw_arguments.rstrip(b'\0').split(b'\0')
        require(len(arguments) <= 32 and len(arguments) >= 3 and arguments[1] == display_name.encode()
                and b'-auth' in arguments and arguments.index(b'-auth') + 1 < len(arguments)
                and arguments[arguments.index(b'-auth') + 1] == os.fsencode(authority)
                and b'1600x1000x24' in arguments, 'Owned Xvfb display/geometry/authentication binding changed')
        self.library = ctypes.CDLL('libX11.so.6')
        self.library.XOpenDisplay.argtypes = [ctypes.c_char_p]
        self.library.XOpenDisplay.restype = ctypes.c_void_p
        self.library.XDefaultRootWindow.argtypes = [ctypes.c_void_p]
        self.library.XDefaultRootWindow.restype = ctypes.c_ulong
        self.library.XGetImage.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.c_int, ctypes.c_int,
                                          ctypes.c_uint, ctypes.c_uint, ctypes.c_ulong, ctypes.c_int]
        self.library.XGetImage.restype = ctypes.POINTER(XImage)
        self.library.XDestroyImage.argtypes = [ctypes.POINTER(XImage)]
        self.library.XDestroyImage.restype = ctypes.c_int
        self.library.XCloseDisplay.argtypes = [ctypes.c_void_p]
        previous = os.environ.get('XAUTHORITY')
        os.environ['XAUTHORITY'] = str(authority)
        try:
            self.display = self.library.XOpenDisplay(display_name.encode())
        finally:
            if previous is None:
                os.environ.pop('XAUTHORITY', None)
            else:
                os.environ['XAUTHORITY'] = previous
        require(self.display, 'Could not open this controller\'s authenticated Xvfb')
        self.window = self.library.XDefaultRootWindow(self.display)
        self.last_rectangle = None
        self.active_frame = None
        self.library.XGetWindowAttributes.argtypes = [ctypes.c_void_p, ctypes.c_ulong,
                                                      ctypes.POINTER(XWindowAttributes)]
        self.library.XGetWindowAttributes.restype = ctypes.c_int
        self.library.XTranslateCoordinates.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.c_ulong,
            ctypes.c_int, ctypes.c_int, ctypes.POINTER(ctypes.c_int), ctypes.POINTER(ctypes.c_int),
            ctypes.POINTER(ctypes.c_ulong)]
        self.library.XTranslateCoordinates.restype = ctypes.c_int
        self.library.XInternAtom.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_int]
        self.library.XInternAtom.restype = ctypes.c_ulong
        self.library.XGetWindowProperty.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.c_ulong,
            ctypes.c_long, ctypes.c_long, ctypes.c_int, ctypes.c_ulong,
            ctypes.POINTER(ctypes.c_ulong), ctypes.POINTER(ctypes.c_int),
            ctypes.POINTER(ctypes.c_ulong), ctypes.POINTER(ctypes.c_ulong),
            ctypes.POINTER(ctypes.POINTER(ctypes.c_ubyte))]
        self.library.XGetWindowProperty.restype = ctypes.c_int
        self.library.XFree.argtypes = [ctypes.c_void_p]
        self.library.XFree.restype = ctypes.c_int
        self.library.XSetErrorHandler.argtypes = [ctypes.c_void_p]
        self.library.XSetErrorHandler.restype = ctypes.c_void_p
        self.library.XSync.argtypes = [ctypes.c_void_p, ctypes.c_int]
        self.library.XSync.restype = ctypes.c_int
        # Atom lookup creates nothing and happens before the fixture clock starts.
        self.pid_atom = self.library.XInternAtom(self.display, b'_NET_WM_PID', 1)

    @contextmanager
    def checked_x11(self):
        errors = []
        callback_type = ctypes.CFUNCTYPE(ctypes.c_int, ctypes.c_void_p, ctypes.POINTER(XErrorEvent))
        def record_error(display, event):
            if len(errors) < 8:
                errors.append((display, event.contents.error_code, event.contents.resource_id))
            return 0
        callback = callback_type(record_error)
        previous = self.library.XSetErrorHandler(ctypes.cast(callback, ctypes.c_void_p))
        try:
            yield
        finally:
            # Synchronous Xlib has the same native blocking limitation as the
            # existing XGetImage path; this is not hard per-call preemption.
            try:
                self.library.XSync(self.display, 0)
            finally:
                self.library.XSetErrorHandler(previous)
        require(not errors, 'Owned X11 window metadata returned a protocol error')

    def before_deadline(self, deadline):
        require(type(deadline) in [int, float] and math.isfinite(deadline)
                and time.monotonic() < deadline, 'Original capture or pause deadline elapsed')

    def window_snapshot(self, admitted, deadline, paused=False):
        require(isinstance(admitted, dict)
                and set(admitted) == {'id', 'x', 'y', 'width', 'height', 'gui'},
                'Exact original admitted window and GUI token are required')
        expected = window_rectangle({name: admitted[name] for name in ['x', 'y', 'width', 'height']})
        require(type(admitted['id']) is int and 0 < admitted['id'] < 1 << 32,
                'Invalid original X11 window ID')
        gui = admitted['gui']
        require(isinstance(gui, dict) and set(gui) == {'pid', 'starttime', 'namespace_pid'}
                and type(gui['pid']) is int and gui['pid'] > 1
                and type(gui['namespace_pid']) is int and gui['namespace_pid'] > 0
                and isinstance(gui['starttime'], str) and gui['starttime'].isdigit(),
                'Expected only the kernel-derived original GUI token')
        root = Path('/proc') / str(gui['pid'])
        with (root / 'stat').open() as stream:
            raw = stream.read(8193)
        require(len(raw) <= 8192, 'Owned GUI kernel metadata exceeded its bound')
        fields = raw.rsplit(')', 1)[1].split()
        require(len(fields) > 19 and fields[19] == gui['starttime'] and root.stat().st_uid == os.getuid()
                and ((fields[0] in ['T', 't']) if paused else fields[0] not in ['T', 't', 'Z']),
                'Original GUI identity or required stopped state changed')
        with (root / 'status').open() as stream:
            status = stream.read(65537)
        namespace = [line.split()[1:] for line in status.splitlines() if line.startswith('NSpid:')]
        require(len(status) <= 65536 and len(namespace) == 1 and namespace[0]
                and namespace[0][0] == str(gui['pid']) and namespace[0][-1] == str(gui['namespace_pid']),
                'Original GUI kernel namespace mapping changed')
        self.before_deadline(deadline)
        attributes = XWindowAttributes()
        require(self.library.XGetWindowAttributes(self.display, admitted['id'], ctypes.byref(attributes)),
                'Original X11 window attributes are unavailable')
        self.before_deadline(deadline)
        x, y, child = ctypes.c_int(), ctypes.c_int(), ctypes.c_ulong()
        require(self.library.XTranslateCoordinates(self.display, admitted['id'], self.window, 0, 0,
                ctypes.byref(x), ctypes.byref(y), ctypes.byref(child)),
                'Original window root coordinates are unavailable')
        self.before_deadline(deadline)
        actual_type, actual_format = ctypes.c_ulong(), ctypes.c_int()
        count, remaining = ctypes.c_ulong(), ctypes.c_ulong()
        data = ctypes.POINTER(ctypes.c_ubyte)()
        try:
            require(self.pid_atom != 0 and self.library.XGetWindowProperty(
                self.display, admitted['id'], self.pid_atom, 0, 1, 0, 6,
                ctypes.byref(actual_type), ctypes.byref(actual_format), ctypes.byref(count),
                ctypes.byref(remaining), ctypes.byref(data)) == 0,
                'Original window ownership property is unavailable')
            require(actual_type.value == 6 and actual_format.value == 32
                    and count.value == 1 and remaining.value == 0 and bool(data),
                    'Original window PID must be exactly one CARDINAL32 value')
            window_pid = ctypes.cast(data, ctypes.POINTER(ctypes.c_ulong))[0]
        finally:
            if data:
                self.library.XFree(data)
        self.before_deadline(deadline)
        require(window_pid == gui['namespace_pid'] and attributes.root == self.window
                and attributes.window_class == 1 and attributes.map_state == 2
                and attributes.depth == 24 and attributes.border_width == 0,
                'Original window ownership, root, visibility or layout changed')
        observed = {'x': x.value, 'y': y.value, 'width': attributes.width, 'height': attributes.height}
        require(observed == expected, 'Original window geometry changed')
        return dict(observed, id=admitted['id'], gui=dict(gui), root=self.window,
                    depth=24, border_width=0, map_state=2, window_class=1)

    @contextmanager
    def capture_bracketed(self, selected, admitted, capture_before, pause_before, facts=None):
        require(self.active_frame is None, 'Only one live owned frame may be retained')
        selected = rectangle(selected)
        require(isinstance(admitted, dict)
                and set(admitted) == {'id', 'x', 'y', 'width', 'height', 'gui'},
                'Exact original admitted window and GUI token are required')
        window_bounds = window_rectangle({name: admitted[name] for name in ['x', 'y', 'width', 'height']})
        require(all(type(value) in [int, float] and math.isfinite(value)
                    for value in [capture_before, pause_before]), 'Invalid original frame deadlines')
        visible = visible_intersection(window_bounds, selected)
        deadline = min(capture_before, pause_before)
        image = None
        frame = None
        started = time.monotonic()
        facts = {} if facts is None else facts
        facts.update(started_monotonic=started, capture_before_monotonic=capture_before,
                     pause_before_monotonic=pause_before, visible_owned_rect=visible)
        try:
            with self.checked_x11():
                before = self.window_snapshot(admitted, deadline)
                facts['window_before'] = before
                facts['image_requested_monotonic'] = time.monotonic()
                image = self.library.XGetImage(self.display, self.window, selected['x'], selected['y'],
                    selected['width'], selected['height'], ctypes.c_ulong(-1).value, 2)
                received = time.monotonic()
                facts['image_received_monotonic'] = received
                require(image, 'Owned X server did not return the requested bounded frame')
                self.before_deadline(deadline)
                validate_image_layout(image.contents, selected)
                after = self.window_snapshot(admitted, deadline)
                facts['window_after'] = after
                require(before == after, 'Owned window changed during frame capture')
            self.before_deadline(deadline)
            facts['capture_finished_monotonic'] = time.monotonic()
            frame = {'pointer': image, 'rectangle': dict(selected), 'before': before, 'after': after,
                     'visible_owned_rect': visible,
                     'started_monotonic': started, 'image_received_monotonic': received,
                     'finished_monotonic': time.monotonic(), 'capture_before_monotonic': capture_before,
                     'pause_before_monotonic': pause_before}
            self.active_frame = frame
            yield frame
        finally:
            self.active_frame = None
            if image:
                self.library.XDestroyImage(image)
                facts['image_destroyed_once'] = True
            if frame is not None:
                frame['pointer'] = None

    def decode_frame(self, frame):
        require(frame is self.active_frame and frame.get('pointer'), 'Owned frame is no longer live')
        return decode_image(frame['pointer'].contents, frame['rectangle'])

    def verify_paused_window(self, frame, deadline):
        require(frame is self.active_frame and frame.get('pointer'), 'Owned frame is no longer live')
        admitted = {name: frame['before'][name] for name in ['id', 'x', 'y', 'width', 'height', 'gui']}
        with self.checked_x11():
            observed = self.window_snapshot(admitted, deadline, paused=True)
        require(observed == frame['before'] == frame['after'],
                'Paused window differs from the exact capture-time bracket')
        return observed

    def capture(self, selected):
        selected = rectangle(selected)
        image = self.library.XGetImage(self.display, self.window, selected['x'], selected['y'],
                                      selected['width'], selected['height'], ctypes.c_ulong(-1).value, 2)
        require(image, 'Owned X server did not return the requested bounded rectangle')
        try:
            result = decode_image(image.contents, selected)
        finally:
            self.library.XDestroyImage(image)
        self.last_rectangle = dict(selected)
        return result

    def close(self):
        if self.display:
            self.library.XCloseDisplay(self.display)
            self.display = None
