"""Admit an exact, source-pinned DorkPipe editor test SDK directory."""
import hashlib
import json
import os
from pathlib import Path
import re
import stat

MAX_MANIFEST_BYTES = 64 << 10
MAX_SOURCE_BYTES = 16 << 20
MAX_ENTRIES = 64
FILES = (
    'LICENSE',
    'README.md',
    'extension.js',
    'extension.test.js',
    'images/icon.png',
    'language-configuration.json',
    'package-lock.json',
    'package.json',
    'snippets/pipelang.json',
    'syntaxes/pipelang.tmLanguage.json',
)
DIRECTORIES = {'images', 'snippets', 'syntaxes'}
SEMVER_CORE = r'(?:0|[1-9][0-9]*)'
SEMVER_PRERELEASE = r'(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)'
SEMVER = re.compile(
    SEMVER_CORE + r'\.' + SEMVER_CORE + r'\.' + SEMVER_CORE
    + r'(?:-' + SEMVER_PRERELEASE + r'(?:\.' + SEMVER_PRERELEASE + r')*)?'
    + r'(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?'
)


def unique_keys(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('Duplicate editor SDK JSON field')
        result[key] = value
    return result


def regular_bytes(path, maximum):
    if path.is_symlink() or any(parent.is_symlink() for parent in path.parents):
        raise ValueError('Editor SDK contains a linked file or parent')
    before = path.stat()
    if not stat.S_ISREG(before.st_mode) or before.st_size > maximum:
        raise ValueError('Editor SDK file is not a bounded regular file')
    flags = os.O_RDONLY | getattr(os, 'O_NOFOLLOW', 0)
    descriptor = os.open(path, flags)
    try:
        opened = os.fstat(descriptor)
        if not stat.S_ISREG(opened.st_mode) or (opened.st_dev, opened.st_ino) != (before.st_dev, before.st_ino):
            raise ValueError('Editor SDK file changed during opening')
        with os.fdopen(descriptor, 'rb', closefd=False) as stream:
            payload = stream.read(maximum + 1)
        after = os.fstat(descriptor)
        if (len(payload) > maximum or len(payload) != opened.st_size
                or (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns)
                != (opened.st_dev, opened.st_ino, opened.st_size, opened.st_mtime_ns)):
            raise ValueError('Editor SDK file grew or changed during reading')
        return payload
    finally:
        os.close(descriptor)


def read_json(path, maximum):
    return json.loads(regular_bytes(path, maximum), object_pairs_hook=unique_keys)


def source_inventory(source):
    if not source.is_absolute() or not source.is_dir() or source.is_symlink():
        raise ValueError('Editor SDK source must be an absolute unlinked directory')
    if any(parent.is_symlink() for parent in source.parents):
        raise ValueError('Editor SDK parent directory is linked')
    pending = [source]
    count = 0
    total = 0
    files = []
    while pending:
        directory = pending.pop()
        for path in sorted(directory.iterdir()):
            count += 1
            if count > MAX_ENTRIES:
                raise ValueError('Editor SDK exceeds entry capacity')
            if path.is_symlink():
                raise ValueError('Editor SDK contains a linked entry')
            relative = path.relative_to(source).as_posix()
            if path.is_dir():
                if relative not in DIRECTORIES:
                    raise ValueError('Editor SDK contains an unknown directory')
                pending.append(path)
                continue
            if relative == 'editor-source.json':
                continue
            if relative not in FILES:
                raise ValueError('Editor SDK contains an unknown source file')
            data = regular_bytes(path, MAX_SOURCE_BYTES - total)
            total += len(data)
            files.append({'path': relative, 'bytes': len(data),
                          'sha256': hashlib.sha256(data).hexdigest()})
    if {entry['path'] for entry in files} != set(FILES):
        raise ValueError('Editor SDK is missing a curated source file')
    return sorted(files, key=lambda entry: entry['path'])


def verify_editor_source(source, lock_path):
    lock = read_json(lock_path, MAX_MANIFEST_BYTES)
    required_lock = {'schema', 'editor_version', 'source_dorkpipe_sha',
                     'source_digest', 'sdk_manifest_sha256'}
    if not isinstance(lock, dict) or set(lock) != required_lock or type(lock['schema']) is not int or lock['schema'] != 1:
        raise ValueError('A complete editor SDK source lock is required')
    if (not isinstance(lock['editor_version'], str)
            or not SEMVER.fullmatch(lock['editor_version'])):
        raise ValueError('Editor SDK source lock has an invalid editor version')
    for field, size in (('source_dorkpipe_sha', 40), ('source_digest', 64),
                        ('sdk_manifest_sha256', 64)):
        if not isinstance(lock[field], str) or not re.fullmatch(r'[0-9a-f]{%d}' % size, lock[field]):
            raise ValueError('Editor SDK source lock has an invalid digest')
    source = Path(source)
    files = source_inventory(source)
    manifest_data = regular_bytes(source / 'editor-source.json', MAX_MANIFEST_BYTES)
    if hashlib.sha256(manifest_data).hexdigest() != lock['sdk_manifest_sha256']:
        raise ValueError('Editor SDK manifest differs from committed pin')
    manifest = json.loads(manifest_data, object_pairs_hook=unique_keys)
    required_manifest = {'schema', 'editor_version', 'source_dorkpipe_sha', 'source_digest', 'files'}
    if (not isinstance(manifest, dict) or set(manifest) != required_manifest
            or type(manifest['schema']) is not int or manifest['schema'] != 1
            or manifest['editor_version'] != lock['editor_version']
            or manifest['source_dorkpipe_sha'] != lock['source_dorkpipe_sha']
            or manifest['source_digest'] != lock['source_digest']
            or manifest['files'] != files):
        raise ValueError('Editor SDK manifest differs from curated source and lock')
    digest_input = json.dumps(files, sort_keys=True, separators=(',', ':')).encode()
    if hashlib.sha256(digest_input).hexdigest() != lock['source_digest']:
        raise ValueError('Editor SDK source digest differs from curated inventory')
    package = read_json(source / 'package.json', MAX_SOURCE_BYTES)
    if (not isinstance(package, dict)
            or package.get('version') != lock['editor_version']):
        raise ValueError('Editor SDK package version differs from pinned editor version')
    return source
