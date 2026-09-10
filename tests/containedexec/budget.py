"""Preservation-first disk accounting with bounded incremental updates."""
import ctypes
import fcntl
import os
from pathlib import Path
import shutil
import struct
import threading
from campaign import atomic_json


class DiskBudget:
    def __init__(self, roots, output, limit_bytes, reserve_bytes=8 << 30):
        self.roots = list(dict.fromkeys(Path(p) for p in roots if p is not None))
        self.limit, self.reserve = limit_bytes, reserve_bytes
        self.mutex, self.locks = threading.Lock(), []
        self.sizes, self.watches, self.used = {}, {}, 0
        self.libc = ctypes.CDLL(None, use_errno=True)
        self.fd = self.libc.inotify_init1(os.O_NONBLOCK | os.O_CLOEXEC)
        if self.fd < 0:
            raise RuntimeError('disk accounting watch unavailable')
        for root in sorted(self.roots):
            root.mkdir(parents=True, exist_ok=True, mode=0o700)
            lock = (root / '.campaign-population.lock').open('a+')
            try:
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BaseException:
                lock.close()
                raise RuntimeError('another campaign is populating this cache: ' + str(root))
            self.locks.append(lock)
            self.scan(str(root))
        self.drain()
        self.started = self.used
        self.record = Path(output) / 'population-budget.json'
        atomic_json(self.record, dict(Limit=max(0, self.limit - self.used), Reserved=0))
        self.check()

    def size(self, path):
        try:
            current = os.lstat(path).st_size
        except FileNotFoundError:
            current = 0
        self.used += current - self.sizes.get(path, 0)
        if current:
            self.sizes[path] = current
        else:
            self.sizes.pop(path, None)

    def scan(self, root):
        # Iterative scandir avoids pathlib.rglob's retained full-tree path set.
        pending = [root]
        while pending:
            directory = pending.pop()
            wd = self.libc.inotify_add_watch(self.fd, os.fsencode(directory), 0x2 | 0x4 | 0x8 | 0x40 | 0x80 | 0x100 | 0x200 | 0x400 | 0x800)
            if wd < 0:
                if not os.path.exists(directory):
                    continue
                raise RuntimeError('disk accounting watch lost')
            self.watches[wd] = directory
            with os.scandir(directory) as entries:
                for entry in entries:
                    if entry.is_dir(follow_symlinks=False):
                        pending.append(entry.path)
                    elif entry.is_file(follow_symlinks=False):
                        self.size(entry.path)
                    elif entry.is_symlink():
                        raise RuntimeError('linked cache entry refused')

    def drain(self):
        while True:
            try:
                data = os.read(self.fd, 1 << 20)
            except BlockingIOError:
                return
            offset = 0
            while offset < len(data):
                wd, mask, cookie, length = struct.unpack_from('iIII', data, offset)
                name = os.fsdecode(data[offset + 16:offset + 16 + length].split(b'\0')[0])
                offset += 16 + length
                if mask & 0x4000:  # Queue overflow: never silently lose budget accounting.
                    raise RuntimeError('disk accounting overflow; restart to re-inventory')
                directory = self.watches.get(wd)
                if directory is None or not name:
                    continue
                path = os.path.join(directory, name)
                if mask & 0x40000000:
                    if mask & (0x100 | 0x80) and os.path.isdir(path):
                        self.scan(path)
                    if mask & (0x200 | 0x40):
                        for prior in [p for p in self.sizes if p.startswith(path + '/')]:
                            self.used -= self.sizes.pop(prior)
                else:
                    self.size(path)

    def check(self, force=False):
        with self.mutex:
            self.drain()
            free = min(shutil.disk_usage(root).free for root in self.roots)
            if self.used > self.limit or free < self.reserve:
                raise RuntimeError('disk budget/headroom exhausted; stop population and preserve all evidence')
            return dict(retained_bytes=self.used, initial_bytes=self.started, limit_bytes=self.limit,
                        available_bytes=free, reserve_bytes=self.reserve,
                        policy='all objects and campaign references pinned; no deletion')

    def root_bytes(self, root):
        prefix = str(root) + '/'
        return sum(size for path, size in self.sizes.items() if path.startswith(prefix))

    def close(self):
        for lock in self.locks:
            lock.close()
        os.close(self.fd)
