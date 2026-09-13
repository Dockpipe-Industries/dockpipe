"""One preservation-first storage owner for a complete verification campaign."""
import json
import os
from pathlib import Path
import signal
import threading
import time
from budget import DiskBudget
from campaign import atomic_json

DEFAULT_LIMIT = 96 << 30


def storage_limit(gib):
    if not 0 < gib <= 96:
        raise ValueError('disk cap must be 1..96 GiB; a higher cap needs a separately reviewed policy change')
    return gib << 30


def canonical_roots(paths):
    roots = []
    for raw in paths:
        path = Path(raw)
        if not path.is_absolute() or path != path.resolve():
            raise ValueError('absolute canonical storage root required: ' + str(path))
        if not (path.is_dir() or path.is_file()):
            raise ValueError('missing storage root: ' + str(path))
        if not any(path == r or r in path.parents for r in roots):
            roots = [r for r in roots if path not in r.parents]
            roots.append(path)
    return sorted(roots)


def campaign_scope(root, cache, compiled, native, baseline, support, preserved):
    # The whole root includes builds, every campaign generation, scratch and logs.
    # External stores, prior baseline and support cannot disappear from the union.
    return canonical_roots([root, cache, compiled, native, baseline, *support, *preserved])


class CampaignBudget:
    def __init__(self, roots, output, limit, reserve=8 << 30, on_failure=None, population_roots=None):
        if limit > DEFAULT_LIMIT or limit <= 0:
            raise ValueError('unapproved disk cap')
        self.output = Path(output)
        self.roots = canonical_roots(roots)
        self.path = self.output / 'storage-budget.json'
        self.stop = threading.Event()
        self.error = None
        self.on_failure = on_failure or (lambda: os.kill(os.getpid(), signal.SIGUSR1))
        scope = dict(version=1, roots=list(map(str, self.roots)), limit_bytes=limit,
                     reserve_bytes=reserve, scope='declared complete campaign estate',
                     product_budget='16/32 GiB targets remain unadopted')
        scope_path = self.output / 'storage-scope.json'
        if scope_path.exists() and json.loads(scope_path.read_text()) != scope:
            raise RuntimeError('storage scope or cap changed; existing campaign baseline is immutable')
        atomic_json(scope_path, scope)
        # Lock population roots only; source, tools and preserved roots are read-only.
        population_roots = canonical_roots(population_roots or [self.output])
        if any(not any(p.is_relative_to(r) for r in self.roots) for p in population_roots):
            raise ValueError('population root outside estate')
        try:
            self.budget = DiskBudget(self.roots, self.output, limit, reserve,
                                     lock_roots=population_roots, create_roots=False, allow_internal_links=True)
        except BaseException as error:
            atomic_json(self.path, dict(status='refused', error=str(error), roots=scope['roots']))
            raise
        self.record = self.budget.record
        self.publish()
        self.thread = threading.Thread(target=self.monitor, daemon=True)
        self.thread.start()

    def publish(self):
        state = self.budget.check()
        state.update(owner_pid=os.getpid(), heartbeat_ns=time.monotonic_ns(), status='watching',
                     roots=list(map(str, self.roots)), population_record=str(self.record))
        atomic_json(self.path, state)
        return state

    def monitor(self):
        while not self.stop.wait(.25):
            try:
                self.publish()
            except BaseException as error:
                self.error = error
                atomic_json(self.path, dict(status='failed', error=str(error), owner_pid=os.getpid()))
                self.on_failure()
                return

    def check(self):
        if self.error:
            raise RuntimeError('campaign storage accounting failed') from self.error
        return self.budget.check()

    def close(self):
        self.stop.set()
        self.thread.join()
        try:
            if not self.error:
                state = self.publish()
                state['status'] = 'complete'
                atomic_json(self.path, state)
        finally:
            self.budget.close()

    def __enter__(self):
        return self

    def __exit__(self, kind, value, traceback):
        self.close()


class SharedBudget:
    """Suite client: only its live direct controller can own its disk gate."""
    def __init__(self, path, roots, limit):
        self.path, self.roots, self.limit = Path(path), list(roots), limit
        state = self.check()
        self.record = Path(state['population_record'])

    def check(self, force=False):
        state = json.loads(self.path.read_text())
        if (state.get('status') != 'watching' or state.get('owner_pid') != os.getppid()
                or not 0 <= time.monotonic_ns() - state.get('heartbeat_ns', 0) <= 5_000_000_000
                or state.get('limit_bytes') != self.limit):
            raise RuntimeError('live campaign storage owner unavailable')
        owners = [Path(p) for p in state['roots']]
        if any(not any(Path(p).resolve().is_relative_to(r) for r in owners) for p in self.roots if p):
            raise RuntimeError('suite storage outside complete campaign scope')
        return state

    def root_bytes(self, root):
        # The controller owns the authoritative complete estate total. Sub-root
        # summaries are metadata-only and never become an independent admission.
        import os
        return sum(Path(d, f).lstat().st_size for d, _, names in os.walk(root) for f in names)

    def close(self):
        pass


def accepted_storage(output):
    """Re-inventory metadata after aggregate cleanup; never rewrite population credits."""
    scope = json.loads((output / 'storage-scope.json').read_text())
    receipt = json.loads((output / 'storage-budget.json').read_text())
    roots = canonical_roots(scope['roots'])
    if (receipt.get('status') != 'complete' or receipt.get('roots') != scope['roots']
            or receipt.get('limit_bytes') != scope['limit_bytes']
            or scope['limit_bytes'] > DEFAULT_LIMIT
            or max(receipt['sampled_logical_peak_bytes'], receipt['sampled_allocated_peak_bytes']) > scope['limit_bytes']):
        raise RuntimeError('complete campaign storage proof unavailable')
    budget = DiskBudget(roots, output, scope['limit_bytes'], scope['reserve_bytes'],
                        lock_roots=[], create_roots=False, allow_internal_links=True, record_population=False)
    try:
        return dict(budget.check(), roots=scope['roots'], status='accepted',
                    campaign_sampled_logical_peak_bytes=receipt['sampled_logical_peak_bytes'],
                    campaign_sampled_allocated_peak_bytes=receipt['sampled_allocated_peak_bytes'])
    finally:
        budget.close()
