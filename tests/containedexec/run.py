#!/usr/bin/env python3
"""Run offline validation in a temporary, verified systemd cgroup (Linux only).

Example: run.py --output /tmp/proof/focused --cache /tmp/proof/cache --timeout 600 -- /absolute/go test ...
The outer runner never starts the workload until the inner runner verifies limits.
Reports are written before whole-unit cancellation; systemd owns final tree cleanup.
"""
import argparse
import json
import os
from pathlib import Path
import resource
import signal
import subprocess
import sys
import time
import uuid
from job import verify_job, performance_counters
from campaign import atomic_json, provenance


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', required=True)
    parser.add_argument('--cache', required=True)
    parser.add_argument('--timeout', type=float, default=30)
    parser.add_argument('--memory-high-mib', type=int, help='Optional soft reclaim threshold below the proactive stop')
    parser.add_argument('--inside', action='store_true', help=argparse.SUPPRESS)
    parser.add_argument('command', nargs=argparse.REMAINDER)
    args = parser.parse_args()
    command = args.command[1:] if args.command[:1] == ['--'] else args.command
    if args.memory_high_mib is not None and not 0 < args.memory_high_mib < 800:
        parser.error('memory-high-mib must be in (0,800)')
    if not command or not 0 < args.timeout <= 1800:
        parser.error('command and timeout in (0,1800] required')
    invocation_started = time.monotonic_ns()
    output = Path(args.output).resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    if not args.inside:
        if 'CONTAINED_JOB_SLICE' in os.environ:
            verify_job()
        unit = 'contained-check-' + uuid.uuid4().hex
        invocation = ['systemd-run', '--user', '--wait', '--pipe', '--collect', '--unit='+unit,
                      '-p', 'MemoryMax=1G', '-p', 'MemorySwapMax=0', '-p', 'TasksMax=128',
                      '-p', 'RuntimeMaxSec='+str(args.timeout+10), '-p', 'KillMode=control-group',
                      '-p', 'TimeoutStopSec=2', '--working-directory='+os.getcwd(),
                      sys.executable, '-B', str(Path(__file__).resolve()), '--inside', '--output', str(output),
                      '--cache', str(Path(args.cache).resolve()), '--timeout', str(args.timeout), '--']+command
        if 'CONTAINED_JOB_SLICE' in os.environ:
            position = invocation.index('--working-directory='+os.getcwd())
            invocation[position:position] = [
                '--slice=' + os.environ['CONTAINED_JOB_SLICE'],
                '-p', 'BindsTo=' + os.environ['CONTAINED_JOB_UNIT'],
                '-p', 'After=' + os.environ['CONTAINED_JOB_UNIT'],
                *['--setenv=' + name + '=' + os.environ[name] for name in
                  ['CONTAINED_JOB_SLICE', 'CONTAINED_JOB_GROUP', 'CONTAINED_JOB_UNIT']],
                '--setenv=PYTHONDONTWRITEBYTECODE=1']
        if args.memory_high_mib is not None:
            invocation[invocation.index('--working-directory='+os.getcwd()):invocation.index('--working-directory='+os.getcwd())] = ['-p', 'MemoryHigh='+str(args.memory_high_mib)+'M']
            separator = invocation.index('--', invocation.index('--inside'))
            invocation[separator:separator] = ['--memory-high-mib', str(args.memory_high_mib)]
        report_path = Path(str(output)+'.json')
        report_path.unlink(missing_ok=True)
        def interrupted(signum, frame):
            raise KeyboardInterrupt
        signal.signal(signal.SIGTERM, interrupted)
        signal.signal(signal.SIGINT, interrupted)
        try:
            with Path(str(output)+'.unit.log').open('w') as log:
                result = subprocess.run(invocation, stdout=log, stderr=subprocess.STDOUT, timeout=args.timeout+25)
        except subprocess.TimeoutExpired:
            result = subprocess.CompletedProcess(invocation, 124)
        finally:
            cleanup_started = time.monotonic_ns()
            # Narrow cleanup of this invocation's randomly named temporary unit.
            subprocess.run(['systemctl', '--user', 'stop', unit], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=5)
        report_path = Path(str(output)+'.json')
        report = json.loads(report_path.read_text()) if report_path.exists() else {'outcome': 'no report; unit failed'}
        group = report.get('cgroup')
        report['tree_removed'] = bool(group) and not Path(group).exists()
        report['unit_exit'] = result.returncode
        report['provenance'] = provenance()
        report['spans'] = [dict(name='containment_and_workload', start_ns=invocation_started, end_ns=cleanup_started, parent=None),
                           dict(name='cleanup', start_ns=cleanup_started, end_ns=time.monotonic_ns(), parent=None)]
        atomic_json(report_path, report)
        print(json.dumps({key:report.get(key) for key in ['outcome','exit','elapsed_s','compiler_sampled_peak_rss_kib','child_maxrss_kib','aggregate_peak_bytes','swap_current','tree_removed','unit_exit']}), flush=True)
        return 0 if result.returncode == 0 and report['tree_removed'] and report.get('exit') == 0 else 1

    group = Path('/sys/fs/cgroup') / Path('/proc/self/cgroup').read_text().strip().split('::', 1)[1].lstrip('/')
    if 'CONTAINED_JOB_SLICE' in os.environ:
        verify_job()
    expected = {'memory.max': 1073741824, 'memory.swap.max': 0, 'pids.max': 128}
    if args.memory_high_mib is not None:
        expected['memory.high'] = args.memory_high_mib*1024*1024
    actual = {name: int((group/name).read_text()) for name in expected}
    if actual != expected:
        raise RuntimeError('containment unavailable: '+repr(actual))
    unit = group.name
    properties = subprocess.check_output(['systemctl', '--user', 'show', unit, '-p', 'RuntimeMaxUSec', '-p', 'KillMode'], text=True, timeout=2)
    if 'KillMode=control-group\n' not in properties or 'RuntimeMaxUSec=infinity' in properties or 'RuntimeMaxUSec=' not in properties:
        raise RuntimeError('independent deadline/cleanup unavailable: '+properties)
    cache = Path(args.cache)
    cache.mkdir(parents=True, exist_ok=True)
    temporary = output.parent / 'tmp'
    temporary.mkdir(exist_ok=True)
    env = dict(os.environ, GOENV='off', GOTOOLCHAIN='local', GOPROXY='off', GOSUMDB='off', GOWORK='off',
               GOMAXPROCS='4', GOCACHE=str(cache), GOTMPDIR=str(temporary),
               TMPDIR=str(temporary), TMP=str(temporary), TEMP=str(temporary))
    # Campaign drivers use normal build settings. Explicit env commands inside
    # a test remain available for bounded adversarial probes.
    unexpected = {name: os.environ[name] for name in ('GOFLAGS', 'GOEXPERIMENT', 'GOOS', 'GOARCH', 'GOAMD64', 'GO386', 'GOARM', 'GOARM64', 'CGO_ENABLED', 'CGO_CFLAGS', 'CGO_CPPFLAGS', 'CGO_CXXFLAGS', 'CGO_LDFLAGS', 'CC', 'CXX', 'GODEBUG') if os.environ.get(name)}
    if unexpected:
        raise RuntimeError('non-default inherited build settings refused: ' + ', '.join(sorted(unexpected)))
    env.pop('GOGC', None)
    env.pop('GOMEMLIMIT', None)
    native_overrides = {name for name in ('CPATH', 'CPLUS_INCLUDE_PATH', 'C_INCLUDE_PATH',
                        'GCC_EXEC_PREFIX', 'COMPILER_PATH', 'LIBRARY_PATH', 'LD_PRELOAD', 'LD_LIBRARY_PATH')
                        if os.environ.get(name)}
    if native_overrides and any(arg.startswith('PIPELANG_TEST_CXX=') for arg in command):
        raise RuntimeError('undeclared native toolchain environment: ' + ', '.join(sorted(native_overrides)))
    atomic_json(Path(str(output)+'.json'), dict(command=command, cgroup=str(group), limits=actual,
        unit_properties=properties, outcome='started; final accounting unavailable', exit=None))
    performance_before = performance_counters(group)
    before = (group/'memory.events').read_text()
    swap_before = (group/'memory.swap.events').read_text()
    start = time.monotonic()
    compiler_rss = 0
    peak = 0
    outcome = 'completed'
    observed = set()
    active = []
    with Path(str(output)+'.output').open('w') as log:
        child = subprocess.Popen(command, stdout=log, stderr=subprocess.STDOUT, env=env, start_new_session=True)
        while child.poll() is None:
            current = int((group/'memory.current').read_text())
            peak = max(peak, current)
            active = []
            for pid in (group/'cgroup.procs').read_text().split():
                observed.add(int(pid))
                try:
                    comm = Path('/proc', pid, 'comm').read_text().strip()
                    status = Path('/proc', pid, 'status').read_text().splitlines()
                    current_rss = next((int(line.split()[1]) for line in status if line.startswith('VmRSS:')), 0)
                    active.append(dict(pid=int(pid), comm=comm, rss_kib=current_rss))
                    if comm == 'compile':
                        rss = next(int(line.split()[1]) for line in status if line.startswith('VmHWM:'))
                        compiler_rss = max(compiler_rss, rss)
                except (OSError, StopIteration):
                    pass
            if Path(str(output)+'.output').stat().st_size > 64 << 20:
                outcome = 'log_budget_stop'
                break
            if current >= 800*1024*1024:
                outcome = 'proactive_memory_stop'
                break
            if time.monotonic()-start >= args.timeout:
                outcome = 'timeout'
                break
            time.sleep(.005)
        report = dict(performance_before=performance_before, performance_after=performance_counters(group),
                      command=command, cgroup=str(group), limits=actual, unit_properties=properties,
                      cache=str(cache), flags={key:env[key] for key in ['GOTOOLCHAIN','GOPROXY','GOSUMDB','GOWORK','GOMAXPROCS','GOTMPDIR','TMPDIR','TMP','TEMP']},
                      elapsed_s=time.monotonic()-start, outcome=outcome, exit=child.poll(),
                      compiler_sampled_peak_rss_kib=compiler_rss,
                      child_maxrss_kib=resource.getrusage(resource.RUSAGE_CHILDREN).ru_maxrss,
                      aggregate_peak_bytes=int((group/'memory.peak').read_text()), sampled_peak_bytes=peak,
                      swap_current=int((group/'memory.swap.current').read_text()),
                      memory_events_before=before, memory_events_after=(group/'memory.events').read_text(),
                      swap_events_before=swap_before, swap_events_after=(group/'memory.swap.events').read_text(),
                      observed_pids=sorted(observed), active_at_last_sample=active,
                      memory_stat=(group/'memory.stat').read_text())
        report['workload_span'] = dict(name='workload', start_ns=int(start * 1e9), end_ns=time.monotonic_ns(), parent='containment_and_workload')
        atomic_json(Path(str(output)+'.json'), report)
        if outcome != 'completed':
            subprocess.run(['systemctl','--user','kill','--kill-who=all','--signal=KILL',unit], timeout=2)
            return 1
        return child.returncode


if __name__ == '__main__':
    sys.exit(main())
