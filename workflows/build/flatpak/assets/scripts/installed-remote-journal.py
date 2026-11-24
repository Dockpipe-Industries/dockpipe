"""Read only the normal app's generated private desktop-reference journal."""
from pathlib import Path
import configparser
import json
import os
import re
import stat

MAXIMUM_BYTES = 256 * 1024
DIGEST = re.compile('[0-9a-f]{64}')
NODE = re.compile('[A-Za-z0-9][A-Za-z0-9_-]{0,63}')


def require(condition, message):
    if not condition:
        raise ValueError(message)


def unique_object(pairs):
    value = {}
    for key, item in pairs:
        require(key not in value, 'Duplicate journal JSON key')
        value[key] = item
    return value


def reference(value):
    require(isinstance(value, dict) and set(value) == {'schema', 'id', 'epoch', 'target'}
            and value['schema'] == 'dockpipe.remote.desktop/v1', 'Unknown app journal reference')
    target = value['target']
    require(isinstance(target, dict) and set(target) == {'node', 'fingerprint', 'generation'},
            'Unknown app journal target')
    require(isinstance(target['node'], str) and NODE.fullmatch(target['node']), 'Invalid app journal node')
    for item in [value['id'], value['epoch'], target['fingerprint'], target['generation']]:
        require(isinstance(item, str) and DIGEST.fullmatch(item), 'Invalid app journal identity digest')
    return value


def byte_array(value):
    """Decode only the Qt IniFormat byte-array representation used for JSON."""
    require(isinstance(value, str) and len(value) <= MAXIMUM_BYTES, 'App journal exceeds its byte bound')
    if value.startswith('"'):
        require(value.endswith('"'), 'Unterminated quoted app journal')
        value = value[1:-1]
    require(value.startswith('@ByteArray(') and value.endswith(')'), 'App journal is not a Qt byte array')
    encoded = value[len('@ByteArray('):-1]
    result = bytearray()
    index = 0
    escaped = {'"': 34, '\\': 92, 'n': 10, 'r': 13, 't': 9}
    while index < len(encoded):
        item = encoded[index]
        index += 1
        if item == '\\':
            require(index < len(encoded), 'Truncated Qt journal escape')
            item = encoded[index]
            index += 1
            if item == 'x':
                digits = ''
                while index < len(encoded) and len(digits) < 2 and encoded[index] in '0123456789abcdefABCDEF':
                    digits += encoded[index]
                    index += 1
                require(digits, 'Invalid Qt journal hex escape')
                result.append(int(digits, 16))
            else:
                require(item in escaped, 'Unsupported Qt journal escape')
                result.append(escaped[item])
        else:
            require(32 <= ord(item) <= 126, 'Expected only ASCII reference data in the app journal')
            result.append(ord(item))
    return bytes(result)


def reject_nonfinite(_value):
    raise ValueError('Nonfinite journal JSON')


def decode_settings(data):
    require(isinstance(data, bytes) and len(data) <= MAXIMUM_BYTES, 'App settings exceed the journal read bound')
    settings = configparser.ConfigParser(interpolation=None, strict=True)
    settings.optionxform = str
    settings.read_string(data.decode('utf-8'))
    if not settings.has_option('remote', 'uncertainDesktopSessions'):
        return []
    raw = byte_array(settings['remote']['uncertainDesktopSessions'])
    if not raw:
        return []
    values = json.loads(raw, object_pairs_hook=unique_object,
                        parse_constant=reject_nonfinite)
    require(isinstance(values, list) and len(values) <= 64, 'App journal must be a bounded reference array')
    result = [reference(value) for value in values]
    require(len({json.dumps(value, sort_keys=True) for value in result}) == len(result),
            'Duplicate references in the app journal')
    return result


def read(path, private_root):
    private_root = Path(private_root).resolve(strict=True)
    root_info = private_root.stat()
    require(root_info.st_uid == os.getuid() and stat.S_IMODE(root_info.st_mode) == 0o700,
            'App journal root must be this fixture\'s private directory')
    path = Path(path)
    require(path.is_absolute() and path.resolve(strict=True) == path and path.is_relative_to(private_root),
            'Read only the selected generated private app settings path')
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(descriptor, 'rb') as stream:
        before = os.fstat(stream.fileno())
        require(stat.S_ISREG(before.st_mode) and before.st_uid == os.getuid()
                and 0 <= before.st_size <= MAXIMUM_BYTES, 'App journal file identity/type/size changed')
        data = stream.read(MAXIMUM_BYTES + 1)
        after = os.fstat(stream.fileno())
    require((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns, before.st_ctime_ns)
            == (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_ctime_ns),
            'App journal changed while being observed; retry only the read')
    return decode_settings(data)
