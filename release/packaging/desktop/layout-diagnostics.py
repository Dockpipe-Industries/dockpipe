#!/usr/bin/env python3
"""Bounded aggregate telemetry for one unchanged native desktop CTest suite."""
import argparse
import ctypes
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import time

INTERVAL_SECONDS = 5
MAX_SAMPLES = 4320  # Six-hour hosted-job ceiling; no per-process inspection.
SOURCE_PATHS = ('src/app/tooling/dockpipe-launcher',
                'release/dependencies/spatialweb-browser.lock.json')


def command(*arguments):
    return subprocess.check_output(arguments, text=True, timeout=10).strip()


def guard_source(source):
    if not re.fullmatch('[0-9a-f]{40}', source):
        raise ValueError('Product source must be a full lowercase commit identity')
    if command('uname', '-s') != 'Darwin' or command('uname', '-m') != 'x86_64':
        raise ValueError('Full-context layout diagnosis requires native macOS Intel')
    if command('git', 'cat-file', '-t', source) != 'commit':
        raise ValueError('Product source is not a commit')
    subprocess.run(['git', 'diff', '--quiet', source, 'HEAD', '--', *SOURCE_PATHS],
                   check=True, timeout=10)
    subprocess.run(['git', 'diff', '--quiet', 'HEAD', '--', *SOURCE_PATHS],
                   check=True, timeout=10)
    return {'schema': 1, 'product_source': source,
            'diagnostic_source': command('git', 'rev-parse', 'HEAD'),
            'launcher_tree': command('git', 'rev-parse', 'HEAD:' + SOURCE_PATHS[0]),
            'browser_lock_blob': command('git', 'rev-parse', 'HEAD:' + SOURCE_PATHS[1])}


class HostSampler:
    """Mach aggregate CPU ticks plus public VM page counters, never process data."""
    def __init__(self):
        self.library = ctypes.CDLL('/usr/lib/libSystem.B.dylib')
        self.library.mach_host_self.restype = ctypes.c_uint32
        self.library.host_statistics.argtypes = [ctypes.c_uint32, ctypes.c_int,
                                                 ctypes.POINTER(ctypes.c_uint32),
                                                 ctypes.POINTER(ctypes.c_uint32)]
        self.library.host_statistics.restype = ctypes.c_int
        self.library.mach_port_deallocate.argtypes = [ctypes.c_uint32, ctypes.c_uint32]
        self.library.mach_port_deallocate.restype = ctypes.c_int
        self.port = self.library.mach_host_self()
        if not self.port:
            raise RuntimeError('Aggregate host port unavailable')

    def sample(self):
        ticks = (ctypes.c_uint32 * 4)()
        count = ctypes.c_uint32(4)
        if self.library.host_statistics(self.port, 3, ticks, ctypes.byref(count)) != 0 or count.value != 4:
            raise RuntimeError('Aggregate CPU statistics unavailable')
        # vm_stat reports host page counters, not process identities or arguments.
        result = subprocess.run(['/usr/bin/vm_stat'], check=True, capture_output=True,
                                text=True, timeout=3, env=dict(os.environ, LC_ALL='C'))
        if len(result.stdout) > 32768:
            raise ValueError('Aggregate VM statistics exceed capacity')
        size = re.search(r'page size of (\d+) bytes', result.stdout)
        if not size:
            raise ValueError('Aggregate VM page size unavailable')
        page_size = int(size.group(1))
        pages = {}
        allowed = {'Pages free', 'Pages active', 'Pages inactive', 'Pages wired down',
                   'Pages occupied by compressor', 'Pages speculative'}
        for line in result.stdout.splitlines():
            match = re.fullmatch(r'([^:]+):\s+(\d+)\.', line)
            if match and match.group(1) in allowed:
                pages[match.group(1)] = int(match.group(2))
        if 'Pages free' not in pages:
            raise ValueError('Aggregate VM free-page count unavailable')
        return {'monotonic_seconds': time.monotonic(), 'unix_seconds': time.time(),
                'cpu_ticks': list(ticks),
                'page_size_bytes': page_size, 'pages': pages,
                'load_average': list(os.getloadavg())}

    def close(self):
        task = ctypes.c_uint32.in_dll(self.library, 'mach_task_self_').value
        self.library.mach_port_deallocate(task, self.port)


def write_sample(stream, sample, previous):
    sample = dict(sample)
    if previous is not None:
        differences = [(current - old) % (2 ** 32)
                       for current, old in zip(sample['cpu_ticks'], previous['cpu_ticks'])]
        total = sum(differences)
        sample['busy_cpu_percent'] = 100 * (total - differences[2]) / total if total else None
    stream.write(json.dumps(sample, separators=(',', ':')) + '\n')
    stream.flush()


def run_ctest(build, evidence, sampler_factory=HostSampler):
    evidence = Path(evidence)
    evidence.mkdir(parents=True, exist_ok=True)
    telemetry = evidence / 'host-resources.jsonl'
    sampler = sampler_factory()
    child = None
    previous_handler = signal.getsignal(signal.SIGTERM)
    sampling_failed = False

    def cancel(_signum, _frame):
        raise KeyboardInterrupt

    try:
        previous = sampler.sample()  # Fail admission before starting CTest.
        with telemetry.open('x', encoding='utf-8') as stream:
            write_sample(stream, previous, None)
            signal.signal(signal.SIGTERM, cancel)
            # No regex, ordering, parallelism, repeat or timeout override.
            child = subprocess.Popen(['ctest', '--test-dir', str(build), '--verbose'],
                                     start_new_session=True)
            samples = 1
            while True:
                try:
                    status = child.wait(timeout=INTERVAL_SECONDS)
                    break
                except subprocess.TimeoutExpired:
                    if sampling_failed:
                        continue
                    if samples >= MAX_SAMPLES:
                        print('Aggregate host sampling capacity exhausted', file=sys.stderr)
                        sampling_failed = True
                        continue
                    try:
                        sample = sampler.sample()
                        write_sample(stream, sample, previous)
                        previous = sample
                        samples += 1
                    except Exception:
                        print('Aggregate host sampling failed', file=sys.stderr)
                        sampling_failed = True
            return status if status or not sampling_failed else 1
    finally:
        if child is not None and child.poll() is None:
            os.killpg(child.pid, signal.SIGTERM)
            try:
                child.wait(timeout=5)
            except subprocess.TimeoutExpired:
                os.killpg(child.pid, signal.SIGKILL)
                child.wait(timeout=5)
        signal.signal(signal.SIGTERM, previous_handler)
        sampler.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest='action', required=True)
    guard = subparsers.add_parser('guard')
    guard.add_argument('--product-source', required=True)
    guard.add_argument('--evidence', required=True)
    run = subparsers.add_parser('ctest')
    run.add_argument('--build', required=True)
    run.add_argument('--evidence', required=True)
    arguments = parser.parse_args()
    try:
        if arguments.action == 'guard':
            receipt = guard_source(arguments.product_source)
            directory = Path(arguments.evidence)
            directory.mkdir(parents=True, exist_ok=True)
            with (directory / 'source.json').open('x', encoding='utf-8') as stream:
                json.dump(receipt, stream, indent=2)
                stream.write('\n')
            return 0
        return run_ctest(arguments.build, arguments.evidence)
    except KeyboardInterrupt:
        return 130
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        print(f'Layout diagnostic failed: {error}', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
