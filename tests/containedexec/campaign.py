"""Crash-consistent verification evidence. Receipts, never indexes, admit work.

Linux flock is the single-writer authority; PID/boot metadata is provenance only.
All evidence is retained, including abandoned temporary files and failed attempts.
"""
import contextlib
import fcntl
import hashlib
import json
import os
from pathlib import Path
import platform
import stat
import sys
import threading
import time
import uuid

SCHEMA = 'pipelang-campaign-v1'


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()


def digest(value):
    return hashlib.sha256(canonical(value)).hexdigest()


def file_identity(path):
    path = Path(path)
    if path.resolve() != path.absolute():
        raise ValueError('linked dependency/artifact path: ' + str(path))
    before = path.lstat()
    if not stat.S_ISREG(before.st_mode):
        raise ValueError('dependency/artifact is not a regular file: ' + str(path))
    h = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1 << 20), b''):
            h.update(block)
    after = path.lstat()
    if (before.st_ino, before.st_size, before.st_mtime_ns, before.st_ctime_ns) != (after.st_ino, after.st_size, after.st_mtime_ns, after.st_ctime_ns):
        raise ValueError('file changed during hashing: ' + str(path))
    return dict(sha256=h.hexdigest(), bytes=after.st_size, executable=bool(after.st_mode & 0o111), executable_mode=after.st_mode & 0o111)


def fingerprint(paths, settings=None):
    """Explicit conservative closure including additions, deletions and file modes.

    Directories must be source-only roots, never caches. No stat-only digest cache.
    Symlinks fail closed rather than silently excluding transitive inputs.
    """
    entries = {}
    for raw in sorted(map(str, paths)):
        root = Path(raw).absolute()
        if root.is_symlink():
            raise ValueError('linked dependency root: ' + str(root))
        if not root.exists():
            entries[str(root)] = None
        elif root.is_file():
            entries[str(root)] = file_identity(root)
        else:
            for directory, dirs, files in os.walk(root, followlinks=False):
                dirs[:] = sorted(d for d in dirs if d not in {'__pycache__', '.git', 'node_modules'})
                for name in dirs:
                    if (Path(directory) / name).is_symlink():
                        raise ValueError('linked dependency directory')
                for name in sorted(files):
                    path = Path(directory) / name
                    entries[str(path)] = file_identity(path)
    return dict(digest=digest(dict(files=entries, settings=settings)), files=entries, settings=settings)


def provenance():
    return dict(utc=time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime()),
                boot_id=Path('/proc/sys/kernel/random/boot_id').read_text().strip(),
                monotonic_ns=time.monotonic_ns())


def host_identity():
    # Boot ID and instantaneous CPU frequency are deliberately not identities.
    policies = {}
    for name in ('scaling_governor', 'scaling_min_freq', 'scaling_max_freq'):
        for path in sorted(Path('/sys/devices/system/cpu/cpufreq').glob('policy*/' + name)):
            policies[str(path)] = path.read_text().strip()
    return dict(system=platform.system(), release=platform.release(), version=platform.version(),
                machine=platform.machine(), cpu=Path('/proc/cpuinfo').read_text().split('flags')[0].split('cpu MHz')[0],
                cpu_policy=policies, affinity=sorted(os.sched_getaffinity(0)),
                memory_total=Path('/proc/meminfo').read_text().splitlines()[0],
                python=sys.version, cgroup_filesystem='cgroup-v2')


def sync_directory(path):
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def atomic_json(path, value, hook=lambda phase: None):
    path = Path(path)
    data = canonical(value) + b'\n'
    temporary = path.with_name('.' + path.name + '.' + uuid.uuid4().hex + '.pending')
    with temporary.open('xb') as stream:
        stream.write(data)
        stream.flush()
        hook('written')
        os.fsync(stream.fileno())
        hook('synced')
    os.replace(temporary, path)
    hook('renamed')
    sync_directory(path.parent)
    hook('committed')
    return len(data)


def sealed(value):
    return dict(schema=SCHEMA, payload=value, sha256=digest(value))


def read_sealed(path):
    record = json.loads(Path(path).read_text())
    if record.get('schema') != SCHEMA or record.get('sha256') != digest(record.get('payload')):
        raise ValueError('invalid evidence envelope: ' + str(path))
    return record['payload']


def resource_accepted(report):
    limits = report.get('limits', {})
    if (report.get('exit') != 0 or report.get('unit_exit') != 0 or report.get('outcome') != 'completed'
            or report.get('tree_removed') is not True or not report.get('cgroup')
            or Path(report['cgroup']).exists() or report.get('swap_current') != 0
            or limits.get('memory.max') != 1 << 30 or limits.get('memory.swap.max') != 0
            or limits.get('pids.max') != 128):
        return False
    def events(text):
        return {key: int(value) for key, value in (line.split() for line in text.splitlines())}
    try:
        before, after = events(report['memory_events_before']), events(report['memory_events_after'])
        swap_before, swap_after = events(report['swap_events_before']), events(report['swap_events_after'])
        return (all(after[k] == before[k] for k in ('max', 'oom', 'oom_kill'))
                and swap_before == swap_after)
    except (KeyError, ValueError):
        return False


class Campaign:
    def __init__(self, root, mode='fresh'):
        self.root = Path(root).absolute()
        if not Path(root).is_absolute() or self.root.is_symlink():
            raise ValueError('absolute non-linked campaign root required')
        self.root.mkdir(parents=True, exist_ok=True, mode=0o700)
        if self.root.stat().st_mode & 0o077:
            raise ValueError('campaign root must be private (0700)')
        self.lock = (self.root / 'writer.lock').open('a+')
        try:
            fcntl.flock(self.lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BaseException:
            self.lock.close()
            raise RuntimeError('campaign already has a live writer')
        self.mutex = threading.RLock()
        self.serialized_bytes = 0
        self.mode = mode
        self.current = {}
        self.history = {}
        self.sequence = 0
        self.manifest_path = self.root / 'manifest.json'
        try:
            for name in ('attempts', 'manifests', 'receipts'):
                (self.root / name).mkdir(exist_ok=True, mode=0o700)
            if self.manifest_path.exists():
                if mode != 'resume':
                    raise ValueError('fresh requires a new campaign directory')
                self.manifest = read_sealed(self.manifest_path)
            elif mode == 'resume':
                raise ValueError('no committed campaign manifest; use fresh to recover initial publication')
            else:
                self.manifest = dict(id=uuid.uuid4().hex, stages={}, inventory={}, revision=0, created=provenance())
                self._manifest()
            self.lock.seek(0)
            self.lock.truncate()
            json.dump(dict(pid=os.getpid(), **provenance()), self.lock)
            self.lock.flush()
            os.fsync(self.lock.fileno())
            self.recover()
        except BaseException:
            self.close()
            raise

    def __enter__(self):
        return self

    def __exit__(self, *args):
        self.close()

    def close(self):
        if not self.lock.closed:
            fcntl.flock(self.lock, fcntl.LOCK_UN)
            self.lock.close()

    def write(self, path, value):
        self.serialized_bytes += atomic_json(path, sealed(value))

    def _manifest(self):
        # Immutable revision first, mutable head last.
        self.write(self.root / 'manifests' / ('%06d.json' % self.manifest['revision']), self.manifest)
        self.write(self.manifest_path, self.manifest)

    def register(self, stage, identity, inventory, parents=()):
        with self.mutex:
            if len(inventory) != len(set(inventory)) or not inventory:
                raise ValueError('empty or duplicate logical inventory')
            if any(p not in self.current for p in parents):
                raise ValueError('missing current stage dependency')
            value = dict(identity=identity, inventory=list(inventory),
                         parents={p: self.current[p] for p in parents})
            key = digest(value)
            old = self.manifest['stages'].get(stage)
            if old != dict(key=key, **value):
                self.manifest['revision'] += 1
                self.manifest['stages'][stage] = dict(key=key, **value)
                self._manifest()
                self.event('stage_registered', stage=stage, previous=old.get('key') if old else None, key=key)
            self.current[stage] = key
            return key

    def event(self, event, **data):
        # Accelerator only. A torn final event never affects receipt admission.
        line = canonical(dict(event=event, **data, **provenance())) + b'\n'
        with (self.root / 'events.jsonl').open('ab') as stream:
            stream.write(line)
        self.serialized_bytes += len(line)

    def recover(self):
        for path in sorted((self.root / 'attempts').glob('*/state.json')):
            try:
                state = read_sealed(path)
            except (ValueError, OSError):
                self.event('corrupt_attempt', path=str(path))
                continue
            self.sequence = max(self.sequence, state.get('sequence', 0))
            history_key = (state['stage'], tuple(state['cases']))
            if self.history.get(history_key, {}).get('sequence', -1) < state.get('sequence', 0):
                self.history[history_key] = state
            if state['state'] in ('planned', 'running'):
                if state.get('job_group') and Path(state['job_group']).exists():
                    raise RuntimeError('previous campaign workload may still be live; wait for job cleanup')
                receipt = self.root / 'receipts' / (state['id'] + '.json')
                try:
                    committed = read_sealed(receipt)
                    passed = committed['id'] == state['id'] and committed['state'] == 'passed'
                except (ValueError, OSError, KeyError):
                    passed = False
                state.update(state='passed' if passed else 'interrupted', recovered=provenance())
                self.write(path, state)
                self.event('recovered', id=state['id'], state=state['state'])

    def begin(self, stage, cases, inputs, retry_of=None):
        with self.mutex:
            if stage not in self.current or not cases or len(cases) != len(set(cases)):
                raise ValueError('unregistered stage or invalid cases')
            if not set(cases) <= set(self.manifest['stages'][stage]['inventory']):
                raise ValueError('attempt cases outside inventory')
            identity = uuid.uuid4().hex
            directory = self.root / 'attempts' / identity
            directory.mkdir(mode=0o700)
            sync_directory(directory.parent)
            previous = self.history.get((stage, tuple(cases)))
            self.sequence += 1
            supersedes = previous.get('supersedes', []) + [previous['id']] if previous else []
            state = dict(id=identity, sequence=self.sequence, supersedes=supersedes, stage=stage, stage_key=self.current[stage], cases=cases,
                         inputs=inputs, retry_of=retry_of or (previous['id'] if previous else None), job_group=os.environ.get('CONTAINED_JOB_GROUP'), state='planned', started=provenance())
            self.write(directory / 'state.json', state)
            state['state'] = 'running'
            self.write(directory / 'state.json', state)
            self.history[(stage, tuple(cases))] = state
            return state

    def finish(self, attempt, report, artifacts, inputs_after, complete_cases, result, accepted=True):
        with self.mutex:
            compact_result = dict(result)
            compact_result.pop('report', None)
            state = dict(attempt, finished=provenance(), report=report, result=compact_result)
            ok = (accepted and resource_accepted(report) and inputs_after == attempt['inputs']
                  and complete_cases == attempt['cases'] and self.current[attempt['stage']] == attempt['stage_key'])
            try:
                state['artifacts'] = {str(Path(p).absolute()): file_identity(p) for p in artifacts}
                ok = ok and bool(state['artifacts'])
            except (ValueError, OSError) as error:
                state['artifact_error'] = str(error)
                ok = False
            state.update(state='passed' if ok else 'failed', inputs_after=inputs_after, complete_cases=complete_cases)
            # Publish receipt only after cleanup, inventory and artifact checks.
            self.write(self.root / 'receipts' / (state['id'] + '.json'), state)
            self.write(self.root / 'attempts' / state['id'] / 'state.json', state)
            self.event('finished', id=state['id'], stage=state['stage'], state=state['state'])
            return state

    def accepted(self, stage, inputs, selected=None, summary_only=False):
        accepted = {}
        for path in sorted((self.root / 'receipts').glob('*.json')):
            try:
                receipt = read_sealed(path)
                if selected is not None and not set(receipt['cases']) & set(selected):
                    continue
                if (receipt['stage'] != stage or receipt['stage_key'] != self.current[stage]
                        or receipt['state'] != 'passed' or receipt['inputs'] != inputs
                        or receipt['inputs_after'] != inputs or receipt['complete_cases'] != receipt['cases']
                        or not resource_accepted(receipt['report']) or not receipt['artifacts']):
                    continue
                if any(file_identity(p) != value for p, value in receipt['artifacts'].items()):
                    self.event('invalidated', id=receipt['id'], reason='artifact drift')
                    continue
                if not set(receipt['cases']) <= set(self.manifest['stages'][stage]['inventory']):
                    continue
                # Reconciliation needs identity/precedence only. Validate the complete
                # receipt and every artifact above before dropping its large payload.
                retained = ({'id': receipt['id'], 'supersedes': receipt.get('supersedes', [])}
                            if summary_only else receipt)
                for case in receipt['cases']:
                    if case in accepted:
                        old = accepted[case]
                        if old['id'] in receipt.get('supersedes', []):
                            accepted[case] = retained
                        elif receipt['id'] not in old.get('supersedes', []):
                            raise RuntimeError('overlapping successful proofs: ' + case)
                    else:
                        accepted[case] = retained
            except (OSError, ValueError, KeyError, TypeError) as error:
                self.event('invalidated', path=str(path), reason=str(error))
        return accepted

    def reconcile(self, stage, inputs):
        accepted = self.accepted(stage, inputs, summary_only=True)
        expected = self.manifest['stages'][stage]['inventory']
        result = dict(stage=stage, key=self.current[stage], expected=len(expected), accepted=len(accepted),
                      missing=[case for case in expected if case not in accepted], serialized_bytes=self.serialized_bytes)
        self.write(self.root / (stage + '-index.json'), result)
        return result


class InputGuard:
    """Hash once, then use kernel change notification as mutation protection.

    Watches are installed before hashing. Any write, rename, metadata change,
    deletion, new child, lost watch or queue overflow fails closed for this run,
    even if bytes were restored. A new process always rehashes all bytes. This is
    not a persistent stat/mtime digest cache. Linux is already required by run.py.
    """
    def __init__(self, paths, settings=None):
        import ctypes
        self.paths = list(paths)
        self.mutex = threading.Lock()
        self.drift = False
        libc = ctypes.CDLL(None, use_errno=True)
        self.fd = libc.inotify_init1(os.O_NONBLOCK | os.O_CLOEXEC)
        if self.fd < 0:
            raise OSError(ctypes.get_errno(), 'input watch unavailable')
        mask = 0x2 | 0x4 | 0x8 | 0x40 | 0x80 | 0x100 | 0x200 | 0x400 | 0x800
        directories = set()
        for raw in paths:
            path = Path(raw).absolute()
            if path.is_dir():
                directories.add(path)
                for directory, dirs, _ in os.walk(path, followlinks=False):
                    dirs[:] = [d for d in dirs if d not in {'__pycache__', '.git', 'node_modules'}]
                    directories.update(Path(directory) / d for d in dirs)
            else:
                # Watch exact existing files to avoid unrelated writes in a
                # shared build-cache directory. Missing inputs watch the parent.
                directories.add(path if path.exists() else path.parent)
        try:
            for path in directories:
                if libc.inotify_add_watch(self.fd, os.fsencode(path), mask) < 0:
                    raise OSError(ctypes.get_errno(), 'cannot watch ' + str(path))
            self.identity = fingerprint(paths, settings)
            self.check()
        except BaseException:
            self.close()
            raise

    def check(self):
        with self.mutex:
            try:
                while os.read(self.fd, 1 << 20):
                    self.drift = True
            except BlockingIOError:
                pass
            if self.drift:
                raise RuntimeError('dependency changed or input watch lost; restart and rehash')
            return self.identity['digest']

    def close(self):
        if self.fd >= 0:
            os.close(self.fd)
            self.fd = -1


class BuildStore:
    """Verified build objects; corrupt generations remain preserved on repair."""
    def __init__(self, root, budget_bytes=4 << 30):
        self.root = Path(root)
        if not self.root.is_absolute() or self.root.is_symlink():
            raise ValueError('absolute private build store required')
        self.root.mkdir(parents=True, exist_ok=True, mode=0o700)
        if self.root.stat().st_mode & 0o077:
            raise ValueError('build store must be private')
        self.budget = budget_bytes

    def get(self, key):
        self.reason = 'no_object_for_current_inputs'
        try:
            record = read_sealed(self.root / (key + '.json'))
            if Path(record['object']).name != record['object'] or not record['object'].startswith(key + '-'):
                return None
            path = self.root / record['object'] / 'program.test'
            if record['key'] == key and file_identity(path) == record['binary']:
                self.reason = 'verified_content_and_mode'
                return path
            self.reason = 'binary_or_identity_mismatch'
        except FileNotFoundError:
            self.reason = 'missing_object_or_index'
        except (OSError, ValueError, KeyError):
            self.reason = 'invalid_object_metadata'
        return None

    def publish(self, key, binary, evidence):
        import shutil
        with (self.root / 'population.lock').open('a+') as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            prior = self.get(key)
            if prior:
                return prior
            identity = file_identity(binary)
            used = sum(p.stat().st_size for p in self.root.rglob('*') if p.is_file())
            if used + identity['bytes'] + 4096 > self.budget:
                raise RuntimeError('build-store disk budget exhausted; preserved objects were not deleted')
            directory = self.root / (key + '-' + uuid.uuid4().hex)
            directory.mkdir(mode=0o700)
            destination = directory / 'program.test'
            with Path(binary).open('rb') as src, destination.open('xb') as dst:
                shutil.copyfileobj(src, dst)
                dst.flush(); os.fsync(dst.fileno())
            destination.chmod(0o500)
            # Executable mode is part of identity; go test emits an executable.
            destination_identity = file_identity(destination)
            if destination_identity['sha256'] != identity['sha256'] or destination_identity['bytes'] != identity['bytes']:
                raise ValueError('build object changed during publication')
            sync_directory(directory)
            atomic_json(self.root / (key + '.json'), sealed(dict(key=key, object=directory.name,
                        binary=destination_identity, evidence=evidence, published=provenance())))
            return destination
