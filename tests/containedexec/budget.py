"""Preservation-first disk accounting with bounded incremental updates."""
import ctypes
import errno
import json
import sqlite3
from collections.abc import MutableMapping
import fcntl
import os
from pathlib import Path
import shutil
import struct
import stat
import sys
import threading
from campaign import atomic_json


def within_roots(path, roots):
    """Test an exact path or ancestor in an already canonical root set."""
    path = Path(path)
    return path in roots or any(parent in roots for parent in path.parents)


def nonoverlapping_roots(paths):
    roots = set(paths)
    return sorted(path for path in roots if not any(parent in roots for parent in path.parents))


class _DirectoryEntries(MutableMapping):
    """Exact, directory-indexed metadata in compact in-memory database pages."""

    def __init__(self):
        self.directories = {}
        self.next_directory = 0
        # DiskBudget serializes access with its mutex after initialization.
        # This database never creates a file or escapes the owner's lifetime.
        self.database = sqlite3.connect(':memory:', isolation_level=None, check_same_thread=False)
        self.database.execute('CREATE TABLE entries (directory INTEGER, name BLOB, value BLOB, '
                              'PRIMARY KEY (directory, name)) WITHOUT ROWID')
        self.database.execute('CREATE TABLE directories (path BLOB PRIMARY KEY, identity INTEGER) WITHOUT ROWID')

    @classmethod
    def encode(cls, value):
        size, blocks, key = value
        if key is None and 0 <= size < 1 << 64 and 0 <= blocks < 1 << 64:
            # Both counters keep their complete unsigned value in 1..8 bytes.
            a = max(1, (size.bit_length() + 7) // 8)
            b = max(1, (blocks.bit_length() + 7) // 8)
            return (bytes([(a - 1) | ((b - 1) << 3)])
                    + size.to_bytes(a, 'little') + blocks.to_bytes(b, 'little'))
        return b'\xff' + json.dumps(value, separators=(',', ':')).encode('ascii')

    @classmethod
    def decode(cls, value):
        tag = value[0]
        if tag != 255:
            a, b = (tag & 7) + 1, (tag >> 3) + 1
            if tag > 63 or len(value) != 1 + a + b:
                raise ValueError('invalid metadata encoding')
            return (int.from_bytes(value[1:1 + a], 'little'),
                    int.from_bytes(value[1 + a:], 'little'), None)
        size, blocks, key = json.loads(value[1:])
        return size, blocks, None if key is None else tuple(key)

    @staticmethod
    def encode_name(name):
        raw = os.fsencode(name)
        # A tag keeps arbitrary raw byte names distinct from packed hex names.
        if (len(raw) in (64, 66) and (len(raw) == 64 or raw[64:] in (b'-a', b'-d'))
                and all(c in b'0123456789abcdef' for c in raw[:64])):
            return b'\1' + bytes.fromhex(raw[:64].decode('ascii')) + raw[64:]
        return b'\0' + raw

    @staticmethod
    def decode_name(encoded):
        if encoded[0] == 1:
            if (len(encoded) not in (33, 35)
                    or (len(encoded) == 35 and encoded[33:] not in (b'-a', b'-d'))):
                raise ValueError('invalid filename encoding')
            return encoded[1:33].hex() + os.fsdecode(encoded[33:])
        if encoded[0] != 0:
            raise ValueError('invalid filename encoding')
        return os.fsdecode(encoded[1:])

    def __getitem__(self, path):
        directory, name = os.path.split(path)
        row = self.database.execute('SELECT value FROM entries WHERE directory=? AND name=?',
                                    (self.directories[directory], self.encode_name(name))).fetchone()
        if row is None:
            raise KeyError(path)
        return self.decode(row[0])

    def __setitem__(self, path, value):
        directory, name = os.path.split(path)
        if directory not in self.directories:
            # Share immutable labels with the watch table, not filesystem state.
            directory = sys.intern(directory)
            self.directories[directory] = self.next_directory
            self.next_directory += 1
            self.database.execute('INSERT INTO directories VALUES (?, ?)',
                                  (os.fsencode(directory), self.directories[directory]))
        self.database.execute('INSERT OR REPLACE INTO entries VALUES (?, ?, ?)',
                              (self.directories[directory], self.encode_name(name), self.encode(value)))

    def __delitem__(self, path):
        directory, name = os.path.split(path)
        identity = self.directories[directory]
        changed = self.database.execute('DELETE FROM entries WHERE directory=? AND name=?',
                                        (identity, self.encode_name(name))).rowcount
        if not changed:
            raise KeyError(path)
        if self.database.execute('SELECT 1 FROM entries WHERE directory=? LIMIT 1', (identity,)).fetchone() is None:
            del self.directories[directory]
            self.database.execute('DELETE FROM directories WHERE path=?', (os.fsencode(directory),))

    def __iter__(self):
        for directory, identity in self.directories.items():
            for (name,) in self.database.execute('SELECT name FROM entries WHERE directory=?', (identity,)):
                yield os.path.join(directory, self.decode_name(name))

    def __len__(self):
        return self.database.execute('SELECT count(*) FROM entries').fetchone()[0]

    def close(self):
        self.database.close()

    def items(self):
        for directory, identity in self.directories.items():
            for name, value in self.database.execute('SELECT name, value FROM entries WHERE directory=?', (identity,)):
                yield os.path.join(directory, self.decode_name(name)), self.decode(value)

    def directories_under(self, root):
        raw = os.fsencode(root)
        prefix = raw + b'/'
        # Binary prefix range: '/' is immediately below '0'. This admits every
        # descendant byte name, including undecodable filenames, but no sibling.
        upper = prefix[:-1] + b'0'
        yield from self.database.execute(
            'SELECT path, identity FROM directories WHERE path=? AND ? '
            'UNION ALL SELECT path, identity FROM directories WHERE path>=? AND path<?',
            (raw, root != os.sep, prefix, upper))

    def paths_under(self, root):
        for directory, identity in self.directories_under(root):
            for (name,) in self.database.execute('SELECT name FROM entries WHERE directory=?', (identity,)):
                yield os.path.join(os.fsdecode(directory), self.decode_name(name))

    def values_under(self, root):
        exact = self.get(root)
        if exact is not None:
            yield exact
        for directory, identity in self.directories_under(root):
            for (value,) in self.database.execute('SELECT value FROM entries WHERE directory=?', (identity,)):
                yield self.decode(value)


class DiskBudget:
    def __init__(self, roots, output, limit_bytes, reserve_bytes=8 << 30, *, lock_roots=None, create_roots=True, allow_internal_links=False, record_population=True, memory_guard=None):
        self.memory_guard = memory_guard or (lambda: None)
        candidates = {Path(p).absolute() for p in roots if p is not None}
        for raw in candidates:
            if raw != raw.resolve():
                raise RuntimeError('linked or noncanonical budget root refused')
        self.roots = nonoverlapping_roots(candidates)
        self.root_set = set(self.roots)
        self.root_paths = {str(root): root for root in self.roots}
        # Cache only immutable path topology, never filesystem observations.
        # Canonical roots cannot contain a symlink in any component.
        self.root_ancestry = sorted({str(path) for root in self.roots for path in (root, *root.parents)})
        self.allow_internal_links = allow_internal_links
        self.hardlinks = {}
        self.hardlink_paths = {}
        self.links = set()
        self.allocated_used = 0
        self.peak = self.allocated_peak = 0
        self.limit, self.reserve = limit_bytes, reserve_bytes
        self.mutex, self.locks = threading.Lock(), []
        self.sizes, self.watches, self.used = _DirectoryEntries(), {}, 0
        self.libc = ctypes.CDLL(None, use_errno=True)
        self.fd = self.libc.inotify_init1(os.O_NONBLOCK | os.O_CLOEXEC)
        if self.fd < 0:
            raise RuntimeError('disk accounting watch unavailable')
        try:
            for root in self.roots:
                if create_roots:
                    root.mkdir(parents=True, exist_ok=True, mode=0o700)
                elif not (root.is_dir() or root.is_file()):
                    raise RuntimeError('missing declared storage root')
            for root in (self.roots if lock_roots is None else lock_roots):
                lock = (Path(root) / '.campaign-population.lock').open('a+')
                try:
                    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
                except BaseException:
                    lock.close()
                    raise RuntimeError('another campaign is populating this cache: ' + str(root))
                self.locks.append(lock)
            for root in self.roots:
                self.scan(str(root))
            self.drain()
            self.started = self.used
            self.record = Path(output) / 'population-budget.json'
            if record_population:
                atomic_json(self.record, dict(Limit=max(0, self.limit - self.used), Reserved=0))
            self.check()
        except BaseException:
            self.close()
            raise

    def size(self, path):
        try:
            info = os.lstat(path)
            if stat.S_ISLNK(info.st_mode):
                try:
                    target = Path(path).resolve()
                except (OSError, RuntimeError) as error:
                    raise RuntimeError('unresolved storage link refused') from error
                if not self.allow_internal_links or (target.exists() and not within_roots(target, self.root_set)):
                    raise RuntimeError('linked storage outside declared roots refused')
            elif not (stat.S_ISREG(info.st_mode) or stat.S_ISSOCK(info.st_mode) or stat.S_ISFIFO(info.st_mode)):
                raise RuntimeError('non-regular storage entry refused')
        except FileNotFoundError:
            info = None
        self.forget(path)
        if info:
            if stat.S_ISLNK(info.st_mode):
                self.links.add(path)
            key = (info.st_dev, info.st_ino) if info.st_nlink > 1 else None
            blocks = info.st_blocks * 512
            self.sizes[path] = (info.st_size, blocks, key)
            self.used += info.st_size
            if key is None:
                self.allocated_used += blocks
            else:
                # A write through one hardlink changes every pathname's logical
                # size; inode allocation is still charged only once.
                linked = self.hardlink_paths.setdefault(key, set())
                for other in linked:
                    old_size = self.sizes[other][0]
                    self.used += info.st_size - old_size
                    self.sizes[other] = (info.st_size, blocks, key)
                linked.add(path)
                prior, count = self.hardlinks.get(key, (0, 0))
                self.allocated_used += blocks - prior
                self.hardlinks[key] = (blocks, count + 1)

    def forget(self, path):
        self.links.discard(path)
        prior = self.sizes.pop(path, None)
        if prior is None:
            return
        size, blocks, key = prior
        self.used -= size
        if key is None:
            self.allocated_used -= blocks
        else:
            self.hardlink_paths[key].discard(path)
            current, count = self.hardlinks[key]
            if count == 1:
                self.allocated_used -= current
                del self.hardlinks[key]
                del self.hardlink_paths[key]
            else:
                self.hardlinks[key] = (current, count - 1)

    def scan(self, root):
        # Iterative scandir avoids pathlib.rglob's retained full-tree path set.
        self.memory_guard()
        if not os.path.isdir(root):
            self.size(root)
            directory = str(Path(root).parent)
            wd = self.libc.inotify_add_watch(self.fd, os.fsencode(directory), 0x2 | 0x4 | 0x8 | 0x40 | 0x80 | 0x100 | 0x200 | 0x400 | 0x800)
            if wd < 0:
                error = ctypes.get_errno()
                # A queued create event can outlive both a temporary entry and
                # its parent. Root loss and all other watch failures still fail.
                if error == errno.ENOENT and root not in self.root_paths and not os.path.lexists(root):
                    return
                raise RuntimeError(f'disk accounting watch unavailable: {directory}: {os.strerror(error)}')
            self.watches[wd] = sys.intern(directory)
            return
        pending = [root]
        while pending:
            directory = pending.pop()
            wd = self.libc.inotify_add_watch(self.fd, os.fsencode(directory), 0x2 | 0x4 | 0x8 | 0x40 | 0x80 | 0x100 | 0x200 | 0x400 | 0x800)
            if wd < 0:
                if not os.path.exists(directory):
                    continue
                raise RuntimeError('disk accounting watch lost')
            self.watches[wd] = sys.intern(directory)
            try:
                entries = os.scandir(directory)
            except FileNotFoundError:
                # A compiler can remove a newly watched temporary directory
                # before scandir opens it. Its parent deletion event remains
                # queued; never forgive loss of a declared root or substitution.
                if directory in self.root_paths or os.path.lexists(directory):
                    raise
                continue
            with entries:
                for index, entry in enumerate(entries):
                    if index % 256 == 0:
                        self.memory_guard()
                    if entry.is_dir(follow_symlinks=False):
                        pending.append(entry.path)
                    else:
                        self.size(entry.path)

    def drain(self):
        while True:
            self.memory_guard()
            try:
                data = os.read(self.fd, 1 << 20)
            except BlockingIOError:
                return
            offset = 0
            events = 0
            while offset < len(data):
                if events % 256 == 0:
                    self.memory_guard()
                events += 1
                wd, mask, cookie, length = struct.unpack_from('iIII', data, offset)
                name = os.fsdecode(data[offset + 16:offset + 16 + length].split(b'\0')[0])
                offset += 16 + length
                if mask & 0x4000:  # Queue overflow: never silently lose budget accounting.
                    raise RuntimeError('disk accounting overflow; restart to re-inventory')
                directory = self.watches.get(wd)
                if directory is None:
                    continue
                if not name:
                    if mask & (0x400 | 0x800 | 0x8000) and Path(directory) in self.root_set:
                        raise RuntimeError('watched storage root removed or moved')
                    if mask & 0x8000:
                        self.watches.pop(wd, None)
                    continue
                path = os.path.join(directory, name)
                if not within_roots(path, self.root_set):
                    continue
                if mask & 0x40000000:
                    if mask & (0x100 | 0x80) and os.path.isdir(path):
                        self.scan(path)
                    if mask & (0x200 | 0x40):
                        for prior in list(self.sizes.paths_under(path)):
                            self.forget(prior)
                else:
                    self.size(path)

    def check_roots(self):
        modes = {}
        try:
            for path in self.root_ancestry:
                mode = os.lstat(path).st_mode
                if stat.S_ISLNK(mode):
                    raise RuntimeError('declared storage root changed or disappeared')
                if path in self.root_paths:
                    modes[self.root_paths[path]] = mode
        except OSError as error:
            raise RuntimeError('declared storage root changed or disappeared') from error
        return modes

    def check(self, force=False):
        with self.mutex:
            self.drain()
            root_modes = self.check_roots()
            # An external target may appear without changing the link itself.
            for link in self.links:
                target = Path(link).resolve()
                if target.exists() and not within_roots(target, self.root_set):
                    raise RuntimeError('linked storage outside declared roots refused')
            allocated = self.allocated_used
            self.peak = max(self.peak, self.used)
            self.allocated_peak = max(self.allocated_peak, allocated)
            # Exact-file support inputs often share a directory. Sample each
            # distinct path once per check; never cache free space across checks
            # or combine different paths merely because they share a device.
            free_paths = {root if stat.S_ISDIR(root_modes[root]) else root.parent for root in self.roots}
            free = min(shutil.disk_usage(path).free for path in free_paths)
            if max(self.used, allocated) > self.limit or free < self.reserve:
                raise RuntimeError('disk budget/headroom exhausted; stop population and preserve all evidence')
            return dict(retained_bytes=self.used, initial_bytes=self.started, limit_bytes=self.limit,
                        allocated_file_bytes=allocated, sampled_logical_peak_bytes=self.peak,
                        sampled_allocated_peak_bytes=self.allocated_peak,
                        peak_kind='sampled file metadata; not exact temporary or physical-device peak',
                        allocation_policy='hardlinks deduplicated; shared extents not identified',
                        available_bytes=free, reserve_bytes=self.reserve,
                        policy='all objects and campaign references pinned; no deletion')

    def root_bytes(self, root):
        with self.mutex:
            return sum(entry[0] for entry in self.sizes.values_under(str(root)))

    def close(self):
        for lock in self.locks:
            lock.close()
        if self.fd >= 0:
            os.close(self.fd)
            self.fd = -1
        self.sizes.close()
