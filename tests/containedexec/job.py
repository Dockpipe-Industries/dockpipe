#!/usr/bin/env python3
"""Bound a validation coordinator and all run.py units in one temporary slice.

The small outside supervisor only samples counters and writes one receipt. Work,
report aggregation and child launchers run in the capped coordinator service.
No persistent unit files or machine configuration are changed.
"""
import argparse
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time
import uuid
from campaign import atomic_json

LIMITS = {'memory.max': 2 << 30, 'memory.high': 1536 << 20,
          'memory.swap.max': 0, 'pids.max': 384}


def current_group():
    return Path('/sys/fs/cgroup') / Path('/proc/self/cgroup').read_text().strip().split('::', 1)[1].lstrip('/')


def verify_job():
    """Fail closed before work; environment names alone do not prove containment."""
    group = Path(os.environ['CONTAINED_JOB_GROUP'])
    if group not in current_group().parents:
        raise RuntimeError('workload is outside its aggregate job cgroup')
    actual = {name: int((group / name).read_text()) for name in LIMITS}
    if actual != LIMITS or group.name != os.environ['CONTAINED_JOB_SLICE']:
        raise RuntimeError('aggregate containment unavailable: ' + repr(actual))
    return group


def write_report(path, report):
    atomic_json(path, report)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--timeout', type=float, default=21600)
    parser.add_argument('--inside', action='store_true', help=argparse.SUPPRESS)
    parser.add_argument('command', nargs=argparse.REMAINDER)
    args = parser.parse_args()
    command = args.command[1:] if args.command[:1] == ['--'] else args.command
    if not command or not 0 < args.timeout <= 21600:
        parser.error('command and timeout in (0,21600] required')
    if args.inside:
        verify_job()
        group = current_group()
        expected = {'memory.max': 512 << 20, 'memory.swap.max': 0, 'pids.max': 64}
        if {name: int((group / name).read_text()) for name in expected} != expected:
            raise RuntimeError('coordinator containment unavailable')
        os.execvpe(command[0], command, os.environ)

    output = args.output.resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    report_path = Path(str(output) + '.json')
    if report_path.exists():
        parser.error('use a fresh job receipt path')
    identity = uuid.uuid4().hex
    slice_name = 'containedjob' + identity + '.slice'
    unit = 'contained-coordinator-' + identity + '.service'
    properties = ['MemoryMax', 't', str(LIMITS['memory.max']),
                  'MemoryHigh', 't', str(LIMITS['memory.high']),
                  'MemorySwapMax', 't', '0', 'TasksMax', 't', '384']
    report = dict(command=command, slice=slice_name, coordinator=unit,
                  limits=LIMITS, outcome='starting', exit=None, tree_removed=False)
    group = None
    child = None
    start = time.monotonic()

    def interrupted(signum, frame):
        raise KeyboardInterrupt

    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGINT, interrupted)
    try:
        subprocess.run(['busctl', '--user', '--timeout=5', 'call', 'org.freedesktop.systemd1',
                        '/org/freedesktop/systemd1', 'org.freedesktop.systemd1.Manager',
                        'StartTransientUnit', 'ssa(sv)a(sa(sv))', slice_name, 'fail',
                        '4', *properties, '0'], check=True, stdout=subprocess.DEVNULL, timeout=10)
        relative = subprocess.check_output(['systemctl', '--user', 'show', slice_name,
                                           '--property=ControlGroup', '--value'], text=True, timeout=5).strip()
        if not relative.startswith('/') or relative == '/':
            raise RuntimeError('aggregate cgroup unavailable')
        group = Path('/sys/fs/cgroup') / relative.lstrip('/')
        actual = {name: int((group / name).read_text()) for name in LIMITS}
        if actual != LIMITS:
            raise RuntimeError('aggregate limits unavailable: ' + repr(actual))
        report.update(cgroup=str(group), memory_events_before=(group / 'memory.events').read_text())
        invocation = ['systemd-run', '--user', '--wait', '--pipe', '--collect', '--unit=' + unit,
                      '--slice=' + slice_name, '-p', 'MemoryMax=512M', '-p', 'MemorySwapMax=0',
                      '-p', 'TasksMax=64', '-p', 'RuntimeMaxSec=' + str(args.timeout),
                      '-p', 'KillMode=control-group', '-p', 'TimeoutStopSec=2',
                      '--setenv=CONTAINED_JOB_SLICE=' + slice_name,
                      '--setenv=CONTAINED_JOB_GROUP=' + str(group),
                      '--setenv=CONTAINED_JOB_UNIT=' + unit,
                      '--setenv=PYTHONDONTWRITEBYTECODE=1', '--working-directory=' + os.getcwd(),
                      sys.executable, '-B', str(Path(__file__).resolve()), '--inside',
                      '--output', str(output), '--timeout', str(args.timeout), '--', *command]
        with Path(str(output) + '.output').open('w') as log:
            child = subprocess.Popen(invocation, stdout=log, stderr=subprocess.STDOUT)
            last_write = 0
            report['outcome'] = 'running'
            while True:
                report.update(elapsed_s=time.monotonic() - start,
                              aggregate_peak_bytes=int((group / 'memory.peak').read_text()),
                              memory_current=int((group / 'memory.current').read_text()),
                              swap_current=int((group / 'memory.swap.current').read_text()),
                              memory_events_after=(group / 'memory.events').read_text())
                if child.poll() is not None:
                    report.update(exit=child.returncode, outcome='completed' if child.returncode == 0 else 'failed')
                    break
                if report['memory_current'] >= 1800 << 20:
                    report['outcome'] = 'aggregate_memory_stop'
                    break
                if report['elapsed_s'] >= args.timeout + 5:
                    report['outcome'] = 'timeout'
                    break
                if report['elapsed_s'] - last_write >= 5 or last_write == 0:
                    write_report(report_path, report)
                    last_write = report['elapsed_s']
                time.sleep(.1)
    except BaseException as error:
        report.update(outcome='interrupted' if isinstance(error, KeyboardInterrupt) else 'error', error=str(error))
    finally:
        stopped = subprocess.run(['systemctl', '--user', 'stop', slice_name],
                                 stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=10)
        if child is not None:
            child.wait(timeout=10)
        report['tree_removed'] = group is not None and not group.exists() and stopped.returncode == 0
        write_report(report_path, report)
    print(json.dumps(report), flush=True)
    return int(report['outcome'] != 'completed' or not report['tree_removed'])


if __name__ == '__main__':
    sys.exit(main())
