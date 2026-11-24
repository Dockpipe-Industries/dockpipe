"""Bounded AT-SPI actions scoped to one owned Dockpipe process."""
import json
import os
from pathlib import Path


def connect(session_path):
    session = json.loads(Path(session_path).read_text())
    os.environ.update(session['environment'])
    import pyatspi
    return pyatspi, session


def wait_for(description, condition, seconds=10):
    import time
    from gi.repository import GLib

    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        try:
            observed = condition()
        except (GLib.Error, LookupError):
            observed = None
        if observed:
            return observed
        time.sleep(0.15)
    raise RuntimeError(description)


def nodes(registry, pid):
    queue = [(application, 0) for application in registry.getDesktop(0)
             if application.get_process_id() == pid]
    visited = 0
    while queue and visited < 4000:
        node, depth = queue.pop(0)
        visited += 1
        yield node, depth
        if depth < 18:
            try:
                queue.extend((child, depth + 1) for child in node if child is not None)
            except Exception:
                pass


def select(registry, pid, name, role):
    matches = [node for node, _ in nodes(registry, pid)
               if node.name == name and node.getRoleName() == role]
    if len(matches) != 1:
        raise RuntimeError(f'Expected one owned {role}: {name}; found {len(matches)}.')
    return matches[0]


def action(node, expected):
    interface = node.queryAction()
    matches = [index for index in range(interface.nActions) if interface.getName(index) == expected]
    if len(matches) != 1 or not interface.doAction(matches[0]):
        raise RuntimeError(f'Owned accessibility action failed: {node.name} / {expected}.')


def tree(registry, pid):
    result = []
    for node, depth in nodes(registry, pid):
        item = {'depth': depth, 'name': node.name, 'role': node.getRoleName()}
        try:
            item['states'] = [state.value_name for state in node.getState().getStates()]
            item['attributes'] = node.getAttributes()
        except Exception:
            pass
        try:
            text = node.queryText()
            item['text'] = text.getText(0, min(text.characterCount, 4096))
        except Exception:
            pass
        result.append(item)
    return result


class CaptureCancelled(RuntimeError):
    """The caller cancelled capture; no image is published."""


def _capture_check(cancelled, deadline):
    import time
    if cancelled():
        raise CaptureCancelled('Owned-window capture cancelled')
    if time.monotonic() >= deadline:
        raise TimeoutError('Owned-window capture deadline exceeded')


def _capture_digest(path, cancelled, deadline):
    import hashlib
    import stat
    with path.open('rb') as stream:
        before = os.fstat(stream.fileno())
        if not stat.S_ISREG(before.st_mode) or not 0 < before.st_size <= 512 << 20:
            raise ValueError('Executable must be an ordinary file within 512 MiB')
        digest = hashlib.sha256()
        while True:
            _capture_check(cancelled, deadline)
            block = stream.read(1 << 20)
            if not block:
                break
            digest.update(block)
        after = os.fstat(stream.fileno())
        current = path.stat()
        if (after.st_dev, after.st_ino) != (current.st_dev, current.st_ino):
            raise ValueError('Executable path changed during admission')
        if (before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns, before.st_ctime_ns) != (
                after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_ctime_ns):
            raise ValueError('Executable changed during admission')
    return digest.hexdigest()


def _capture_process(owner, cancelled, deadline):
    import re
    if (not isinstance(owner, dict) or set(owner) != {'pid', 'starttime', 'exe_sha256'}
            or type(owner['pid']) is not int or not 2 <= owner['pid'] <= 0x7fffffff
            or not isinstance(owner['starttime'], str) or re.fullmatch('[0-9]{1,20}', owner['starttime']) is None
            or not isinstance(owner['exe_sha256'], str)
            or re.fullmatch('[0-9a-f]{64}', owner['exe_sha256']) is None):
        raise ValueError('Capture requires an exact current process identity')
    root = Path('/proc') / str(owner['pid'])
    with (root / 'stat').open('rb') as stream:
        before = stream.read(8193)
    if len(before) > 8192 or before.rsplit(b')', 1)[1].split()[19].decode() != owner['starttime']:
        raise ValueError('Owned process kernel identity differs')
    if root.stat().st_uid != os.getuid():
        raise ValueError('Capture process belongs to another operator')
    if _capture_digest(root / 'exe', cancelled, deadline) != owner['exe_sha256']:
        raise ValueError('Owned process executable digest differs')
    with (root / 'stat').open('rb') as stream:
        after = stream.read(8193)
    if len(after) > 8192 or after.rsplit(b')', 1)[1].split()[19].decode() != owner['starttime']:
        raise ValueError('Owned process changed during admission')
    with (root / 'status').open('r') as stream:
        status = stream.read(65537)
    return _capture_namespace_pid(status, owner['pid'])


def _capture_namespace_pid(status, host_pid):
    import re
    matches = re.findall(r'^NSpid:\s*([0-9 \t]+)$', status, re.MULTILINE)
    if len(status) > 65536 or len(matches) != 1:
        raise ValueError('Owned process PID namespace mapping is unavailable')
    values = matches[0].split()
    if not 1 <= len(values) <= 32 or any(len(value) > 10 for value in values):
        raise ValueError('Owned process PID namespace mapping exceeds its bound')
    identifiers = [int(value) for value in values]
    if not 1 <= len(identifiers) <= 32 or identifiers[0] != host_pid or any(not 1 <= value <= 0x7fffffff for value in identifiers):
        raise ValueError('Owned process PID namespace mapping differs')
    return identifiers[-1]


def _capture_path(value):
    if not isinstance(value, (str, Path)):
        raise ValueError('Capture paths must be strings or paths')
    path = Path(value)
    if not path.is_absolute() or path.resolve(strict=True) != path:
        raise ValueError('Capture paths must be absolute and contain no symbolic links')
    return path


def _capture_environment(environment, display_owner, cancelled, deadline):
    import re
    import stat
    if (not isinstance(environment, dict) or set(environment) != {'DISPLAY', 'XAUTHORITY'}
            or any(not isinstance(value, str) for value in environment.values())):
        raise ValueError('Capture environment must select only DISPLAY and XAUTHORITY')
    if re.fullmatch(r':[0-9]{1,5}', environment['DISPLAY']) is None:
        raise ValueError('Capture requires a local private display')
    authority = _capture_path(environment['XAUTHORITY'])
    status = authority.stat()
    parent = authority.parent.stat()
    if (not stat.S_ISREG(status.st_mode) or status.st_nlink != 1 or status.st_uid != os.getuid()
            or stat.S_IMODE(status.st_mode) & 0o077 or not 0 < status.st_size <= 65536
            or parent.st_uid != os.getuid() or stat.S_IMODE(parent.st_mode) & 0o077):
        raise ValueError('Capture Xauthority and parent must be private and operator-owned')
    _capture_process(display_owner, cancelled, deadline)
    with (Path('/proc') / str(display_owner['pid']) / 'cmdline').open('rb') as stream:
        arguments = stream.read(65537)
    if len(arguments) > 65536:
        raise ValueError('Display command line exceeds its bound')
    arguments = arguments.rstrip(b'\0').split(b'\0')
    if (not 3 <= len(arguments) <= 64 or os.fsencode(environment['DISPLAY']) not in arguments[1:]
            or arguments.count(b'-auth') != 1):
        raise ValueError('Display process does not bind the selected private transport')
    index = arguments.index(b'-auth')
    if index + 1 >= len(arguments) or arguments[index + 1] != os.fsencode(authority):
        raise ValueError('Display authority differs from the selected private file')
    return {'DISPLAY': environment['DISPLAY'], 'XAUTHORITY': str(authority), 'LC_ALL': 'C'}


def _capture_command(arguments, environment, maximum, cancelled, deadline):
    import selectors
    import signal
    import subprocess
    import time
    _capture_check(cancelled, deadline)
    process = subprocess.Popen(arguments, env=environment, stdin=subprocess.DEVNULL,
                               stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, start_new_session=True)
    collected = bytearray()
    try:
        with selectors.DefaultSelector() as selector:
            selector.register(process.stdout, selectors.EVENT_READ)
            while selector.get_map():
                _capture_check(cancelled, deadline)
                for key, _ in selector.select(0.02):
                    block = os.read(key.fd, min(65536, maximum + 1 - len(collected)))
                    if not block:
                        selector.unregister(key.fileobj)
                        continue
                    collected.extend(block)
                    if len(collected) > maximum:
                        raise ValueError('Capture tool output exceeds its byte bound')
        while process.poll() is None:
            _capture_check(cancelled, deadline)
            try:
                process.wait(timeout=0.02)
            except subprocess.TimeoutExpired:
                pass
        if process.returncode != 0:
            raise ValueError('Pinned capture tool failed')
        return bytes(collected)
    finally:
        # This dedicated session contains only this invocation's tool children.
        try:
            os.killpg(process.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        cleanup_deadline = time.monotonic() + 0.5
        while time.monotonic() < cleanup_deadline:
            process.poll()
            try:
                os.killpg(process.pid, 0)
            except ProcessLookupError:
                break
            time.sleep(0.01)
        else:
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
        try:
            process.wait(timeout=0.5)
        finally:
            process.stdout.close()


def _capture_window(owner, window, tools, environment, cancelled, deadline):
    import re
    x11_pid = _capture_process(owner, cancelled, deadline)
    identifier = str(window['id'])
    properties = _capture_command([tools['xprop'], '-id', identifier, '_NET_WM_PID'],
                                  environment, 4096, cancelled, deadline).decode('ascii')
    match = re.fullmatch(r'_NET_WM_PID\(CARDINAL\) = ([0-9]+)\s*', properties)
    if match is None or int(match[1]) != x11_pid:
        raise ValueError('Selected window PID differs from the owned process')
    information = _capture_command([tools['xwininfo'], '-id', identifier], environment,
                                   16384, cancelled, deadline).decode('ascii')
    patterns = {'x': r'Absolute upper-left X:\s*(-?[0-9]+)',
                'y': r'Absolute upper-left Y:\s*(-?[0-9]+)',
                'width': r'Width:\s*([0-9]+)', 'height': r'Height:\s*([0-9]+)'}
    for name, pattern in patterns.items():
        matches = re.findall(pattern, information)
        if len(matches) != 1 or int(matches[0]) != window[name]:
            raise ValueError('Selected window geometry changed')
    if len(re.findall(r'Map State:\s*IsViewable\b', information)) != 1:
        raise ValueError('Selected window is not currently viewable')
    return {'x11_pid': x11_pid}


class XwdLayoutError(ValueError):
    """A rejected XWD layout with numeric metadata, never captured image bytes."""

    __slots__ = ('_xwd_reason', '_xwd_header', '_xwd_width', '_xwd_height',
                 '_xwd_raw_length', '_xwd_name_terminated')

    def __init__(self, reason, header, width, height, raw_length, name_terminated):
        message = 'Unsupported or inconsistent bounded XWD layout'
        if type(reason) is str and reason == 'xwd-header-truncated':
            message = 'Truncated XWD header'
        if type(reason) is str and reason == 'xwd-identity-palette-rejected':
            message = 'Unsupported XWD identity DirectColor palette'
        super().__init__(message)
        self._xwd_reason = reason
        self._xwd_header = header
        self._xwd_width = width
        self._xwd_height = height
        self._xwd_raw_length = raw_length
        self._xwd_name_terminated = name_terminated


_XWD_DIAGNOSTIC_SLOTS = tuple(
    XwdLayoutError.__dict__[name]
    for name in ('_xwd_reason', '_xwd_header', '_xwd_width', '_xwd_height',
                 '_xwd_raw_length', '_xwd_name_terminated')
)
_XWD_HEADER_FIELDS = (
    'header_size', 'file_version', 'pixmap_format', 'pixmap_depth',
    'pixmap_width', 'pixmap_height', 'xoffset', 'byte_order', 'bitmap_unit',
    'bitmap_bit_order', 'bitmap_pad', 'bits_per_pixel', 'bytes_per_line',
    'visual_class', 'red_mask', 'green_mask', 'blue_mask', 'bits_per_rgb',
    'colormap_entries', 'ncolors', 'window_width', 'window_height',
    'window_x', 'window_y', 'window_bdrwidth',
)


def capture_failure_diagnostic(error):
    """Return bounded typed XWD metadata, or None for an unrelated/invalid error.

    Exact type/slot checks avoid arbitrary attributes, exception text, user-defined
    equality, raw XWD/window names, colormap entries and pixel serialization.
    """
    if type(error) is not XwdLayoutError:
        return None
    try:
        reason, header, width, height, raw_length, terminated = (
            descriptor.__get__(error, XwdLayoutError) for descriptor in _XWD_DIAGNOSTIC_SLOTS
        )
    except AttributeError:
        return None
    if (type(reason) is not str or reason not in ('xwd-header-truncated', 'xwd-layout-unsupported',
                                                'xwd-identity-palette-rejected')
            or type(width) is not int or not 1 <= width <= 4096
            or type(height) is not int or not 1 <= height <= 4096
            or type(raw_length) is not int or not 0 <= raw_length <= 0x7fffffffffffffff
            or terminated is not None and type(terminated) is not bool):
        return None
    result = {'schema': 'dockpipe.xwd-layout-diagnostic/v1', 'reason': reason,
              'requested_width': width, 'requested_height': height,
              'raw_length': raw_length, 'header_fields': {},
              'window_name_and_pixels_retained': False}
    if reason == 'xwd-header-truncated':
        if header is not None or raw_length >= 100 or terminated is not None:
            return None
        # An incomplete header is not reinterpreted as text or partial raw bytes.
        result.update(header_complete=False, required_header_bytes=100)
        return result
    if (type(header) is not tuple or len(header) != 25
            or any(type(value) is not int or not 0 <= value <= 0xffffffff for value in header)
            or raw_length < 100):
        return None
    offset, version, image_format, depth, actual_width, actual_height = header[:6]
    terminator_available = 1 <= offset <= raw_length
    if terminator_available and terminated is None:
        return None
    if not terminator_available and terminated is not None:
        return None
    bits, stride, colors = header[11], header[12], header[19]
    expected_stride = ((width * bits + 31) // 32) * 4
    payload_offset = offset + colors * 12
    expected_total = payload_offset + stride * height
    if any(not 0 <= value <= 1 << 45 for value in (expected_stride, payload_offset, expected_total)):
        return None
    if reason == 'xwd-identity-palette-rejected':
        if not _capture_identity_directcolor_header(header, width, height, raw_length, terminated):
            return None
        result.update(header_complete=True, header_fields=dict(zip(_XWD_HEADER_FIELDS, header)),
                      encoded_row_minimum=width * 3, maximum_stride=width * 4,
                      payload_offset=payload_offset, expected_total_bytes=expected_total,
                      window_name_terminated=terminated,
                      failed_checks=['identity-directcolor-palette'], unavailable_checks=[])
        return result
    checks = (
        ('header-size', 101 <= offset <= 4096),
        ('file-version', version == 7),
        ('pixmap-format', image_format == 2),
        ('pixmap-depth', depth in (24, 32)),
        ('captured-width', actual_width == width),
        ('captured-height', actual_height == height),
        ('x-offset', header[6] == 0),
        ('byte-order', header[7] in (0, 1)),
        ('bitmap-unit', header[8] in (8, 16, 32)),
        ('bitmap-bit-order', header[9] in (0, 1)),
        ('bitmap-pad', header[10] == 32),
        ('bits-per-pixel', bits in (24, 32)),
        ('padded-stride', stride == expected_stride),
        ('visual-class', header[13] == 4),
        ('rgb-masks', header[14:17] == (0xff0000, 0xff00, 0xff)),
        ('color-count', colors <= 256),
        ('exact-payload-length', raw_length == expected_total),
        ('window-name-terminated', terminated),
    )
    if all(passed is True for _, passed in checks):
        return None
    result.update(header_complete=True, header_fields=dict(zip(_XWD_HEADER_FIELDS, header)),
                  expected_stride=expected_stride, payload_offset=payload_offset,
                  expected_total_bytes=expected_total, window_name_terminated=terminated,
                  failed_checks=[name for name, passed in checks if passed is False],
                  unavailable_checks=[name for name, passed in checks if passed is None])
    return result


def _capture_identity_directcolor_header(header, width, height, raw_length, terminated):
    """Admit only the measured synthetic DirectColor layout, not general LUTs."""
    offset, version, image_format, depth, actual_width, actual_height = header[:6]
    stride = header[12]
    return (101 <= offset <= 4096 and version == 7 and image_format == 2 and depth == 24
            and (actual_width, actual_height) == (width, height) and header[6] == 0
            and header[7] in (0, 1) and header[8] == 32 and header[9] in (0, 1)
            and header[10] == 8 and header[11] == 24 and width * 3 <= stride <= width * 4
            and header[13] == 5 and header[14:17] == (0xff0000, 0xff00, 0xff)
            and header[17] == 8 and header[18] == 256 and header[19] == 256
            and raw_length == offset + 256 * 12 + stride * height and terminated is True)


def _capture_identity_directcolor_palette(raw, offset, cancelled, deadline):
    """Prove all component indices map identically before applying RGB masks."""
    import struct
    seen = set()
    for record in range(256):
        if record % 32 == 0:
            _capture_check(cancelled, deadline)
        pixel, red, green, blue, flags, _padding = struct.unpack_from('!IHHHBB', raw, offset + record * 12)
        index = pixel & 255
        if (pixel != index * 0x010101 or index in seen
                or (red, green, blue) != (index * 257, index * 257, index * 257)
                or flags != 0):
            return False
        seen.add(index)
    _capture_check(cancelled, deadline)
    return len(seen) == 256


def _capture_png(raw, width, height, cancelled, deadline):
    """Decode bounded TrueColor or proven identity DirectColor into RGB PNG."""
    import struct
    import zlib
    if not 1 <= width <= 4096 or not 1 <= height <= 4096:
        raise ValueError('XWD geometry exceeds its capture bound')
    if len(raw) < 100:
        raise XwdLayoutError('xwd-header-truncated', None, width, height, len(raw), None)
    header = struct.unpack('!25I', raw[:100])
    offset, version, image_format, depth, actual_width, actual_height = header[:6]
    byte_order, bits_per_pixel, stride = header[7], header[11], header[12]
    color_count = header[19]
    payload_offset = offset + color_count * 12
    pixel_bytes = bits_per_pixel // 8
    expected_stride = ((width * bits_per_pixel + 31) // 32) * 4
    legacy_layout_unsupported = (not 101 <= offset <= 4096 or version != 7 or image_format != 2 or depth not in (24, 32)
            or (actual_width, actual_height) != (width, height) or header[6] != 0
            or byte_order not in (0, 1) or header[8] not in (8, 16, 32)
            or header[9] not in (0, 1) or header[10] != 32 or bits_per_pixel not in (24, 32)
            or stride != expected_stride or header[13] != 4
            or header[14:17] != (0xff0000, 0xff00, 0xff)
            or color_count > 256 or len(raw) != payload_offset + stride * height
            or raw[offset - 1] != 0)
    if legacy_layout_unsupported:
        terminated = raw[offset - 1] == 0 if 1 <= offset <= len(raw) else None
        if not _capture_identity_directcolor_header(header, width, height, len(raw), terminated):
            raise XwdLayoutError('xwd-layout-unsupported', header, width, height, len(raw), terminated)
        if not _capture_identity_directcolor_palette(raw, offset, cancelled, deadline):
            raise XwdLayoutError('xwd-identity-palette-rejected', header, width, height, len(raw), terminated)
    compressor = zlib.compressobj()
    compressed = bytearray()
    for y in range(height):
        _capture_check(cancelled, deadline)
        row = bytearray(1 + width * 3)
        start = payload_offset + y * stride
        for x in range(width):
            source = start + x * pixel_bytes
            pixel = int.from_bytes(raw[source:source + pixel_bytes], 'little' if byte_order == 0 else 'big')
            destination = 1 + x * 3
            row[destination:destination + 3] = bytes(((pixel >> 16) & 255, (pixel >> 8) & 255, pixel & 255))
        compressed.extend(compressor.compress(row))
    compressed.extend(compressor.flush())
    def chunk(kind, contents):
        return struct.pack('!I', len(contents)) + kind + contents + struct.pack('!I', zlib.crc32(kind + contents))
    return (b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('!2I5B', width, height, 8, 2, 0, 0, 0))
            + chunk(b'IDAT', compressed) + chunk(b'IEND', b''))


def _capture_publish(output, image, cancelled, deadline):
    """Commit complete private bytes atomically without replacing any preimage."""
    import tempfile
    descriptor, name = tempfile.mkstemp(prefix='.owned-window-', suffix='.tmp', dir=output.parent)
    temporary = Path(name)
    created = os.fstat(descriptor)
    try:
        with os.fdopen(descriptor, 'wb') as stream:
            stream.write(image)
            stream.flush()
            _capture_check(cancelled, deadline)
        current = temporary.lstat()
        if (current.st_dev, current.st_ino) != (created.st_dev, created.st_ino):
            raise ValueError('Capture temporary path changed before publication')
        # A hard link publishes complete bytes atomically and fails on any preimage.
        os.link(temporary, output, follow_symlinks=False)
    finally:
        try:
            current = temporary.lstat()
        except FileNotFoundError:
            current = None
        if current is not None:
            if (current.st_dev, current.st_ino) != (created.st_dev, created.st_ino):
                raise ValueError('Capture temporary path changed; foreign inode preserved')
            temporary.unlink()


def capture_owned_window(owner, display_owner, window, environment, tools, output,
                         cancelled=lambda: False, timeout=15):
    """Capture one admitted current window; never capture a root screen or send input.

    Callers supply fresh observed process/window identities and pinned tools. The
    capture contract permits up to 4096x4096 pixels (64 MiB raw), with an aggregate
    deadline of at most 30 seconds. Each tool cleanup has a further one-second budget.
    """
    import hashlib
    import math
    import re
    import stat
    import time
    if (not callable(cancelled) or type(timeout) not in (int, float)
            or not math.isfinite(timeout) or not 0 < timeout <= 30):
        raise ValueError('Capture requires a cancellation callback and bounded deadline')
    deadline = time.monotonic() + timeout
    if (not isinstance(window, dict) or set(window) != {'id', 'x', 'y', 'width', 'height'}
            or any(type(window[name]) is not int for name in window)
            or not 0 < window['id'] <= 0xffffffff
            or not 1 <= window['width'] <= 4096 or not 1 <= window['height'] <= 4096
            or any(not -32768 <= window[name] <= 32767 for name in ('x', 'y'))):
        raise ValueError('Capture window identity or geometry exceeds its bound')
    if not isinstance(tools, dict) or set(tools) != {'xprop', 'xwininfo', 'xwd'}:
        raise ValueError('Capture requires exactly three pinned X11 tools')
    selected_tools = {}
    for name, pin in tools.items():
        if (not isinstance(pin, dict) or set(pin) != {'path', 'sha256'}
                or not isinstance(pin['path'], str) or not isinstance(pin['sha256'], str)
                or re.fullmatch('[0-9a-f]{64}', pin['sha256']) is None):
            raise ValueError('Invalid X11 capture tool pin')
        path = _capture_path(pin['path'])
        if not os.access(path, os.X_OK) or _capture_digest(path, cancelled, deadline) != pin['sha256']:
            raise ValueError('Pinned X11 capture tool digest differs')
        selected_tools[name] = str(path)
    if not isinstance(output, (str, Path)):
        raise ValueError('Capture output must be an absolute path')
    output = Path(output)
    parent = _capture_path(str(output.parent))
    parent_status = parent.stat()
    if (not output.is_absolute() or output.name in ('', '.', '..') or output.exists() or output.is_symlink()
            or parent_status.st_uid != os.getuid() or stat.S_IMODE(parent_status.st_mode) & 0o077):
        raise ValueError('Capture output must be fresh beneath a private owned directory')
    selected_environment = _capture_environment(environment, display_owner, cancelled, deadline)
    window_admission = _capture_window(owner, window, selected_tools, selected_environment, cancelled, deadline)
    authority_before = Path(environment['XAUTHORITY']).stat()
    maximum = 4096 + 256 * 12 + window['height'] * (((window['width'] * 32 + 31) // 32) * 4)
    raw = _capture_command([selected_tools['xwd'], '-silent', '-id', str(window['id'])],
                           selected_environment, maximum, cancelled, deadline)
    image = _capture_png(raw, window['width'], window['height'], cancelled, deadline)
    _capture_environment(environment, display_owner, cancelled, deadline)
    if _capture_window(owner, window, selected_tools, selected_environment, cancelled, deadline) != window_admission:
        raise ValueError('Window namespace identity changed during capture')
    authority_after = Path(environment['XAUTHORITY']).stat()
    if (authority_before.st_dev, authority_before.st_ino, authority_before.st_mtime_ns, authority_before.st_ctime_ns) != (
            authority_after.st_dev, authority_after.st_ino, authority_after.st_mtime_ns, authority_after.st_ctime_ns):
        raise ValueError('Private display authority changed during capture')
    for name, pin in tools.items():
        if _capture_digest(Path(selected_tools[name]), cancelled, deadline) != pin['sha256']:
            raise ValueError('Pinned capture tool changed during capture')
    _capture_check(cancelled, deadline)
    _capture_publish(output, image, cancelled, deadline)
    return {'schema': 'dockpipe.product-owned-window-capture/v1', 'path': str(output),
            'sha256': hashlib.sha256(image).hexdigest(), 'owner': dict(owner),
            'display_owner': dict(display_owner), 'window': dict(window),
            'window_admission': window_admission,
            'tool_sha256': {name: pin['sha256'] for name, pin in tools.items()},
            'captured_at_ns': time.time_ns(), 'scope': 'owned-window', 'input_sent': False}
