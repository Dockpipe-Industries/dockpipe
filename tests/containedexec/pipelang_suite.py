#!/usr/bin/env python3
"""Build current PipeLang tests, then execute the complete contained conformance inventory.

Executable reuse is opt-in. Every oracle and direct compiler resource probe runs.
Use shape-batch-size 1 to populate artifacts; larger warm groups reduce launch and
repeated toolchain verification costs without changing test selection or limits.
"""
import argparse
import concurrent.futures
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys
import time


def plan(tests, splits, shape_batch_size):
    jobs, pending = [], []

    def flush():
        if pending:
            jobs.append((pending.copy(), '^(' + '|'.join(pending) + ')$'))
            pending.clear()

    for name in tests:
        if name == 'TestCompilerMemoryLocalSequences':
            flush()
            for version in [40, 72, 83, 84, 85, 86, 87, 88, 89, 90, 91]:
                prefix = name + f'/v0.{version}.0'
                pattern = '^' + name + r'$/^v0\.' + str(version) + r'\.0$'
                # v0.91 artifact population exceeds cumulative file-cache space
                # in one version unit. Preserve every family in fresh choice units.
                if version == 91:
                    for choices in [0, 1, 2, 3, -1]:
                        suffix = 'choices' + str(choices)
                        jobs.append(([prefix + '/' + suffix], pattern + '/^' + suffix + '$'))
                else:
                    jobs.append(([prefix], pattern))
        elif name in splits:
            flush()
            for start in range(0, splits[name], shape_batch_size):
                indices = list(range(start, min(start + shape_batch_size, splits[name])))
                jobs.append(([name + '/' + str(i) for i in indices], '^' + name + '$/^(' + '|'.join(map(str, indices)) + ')$'))
        elif name.endswith('Memory'):
            flush()
            jobs.append(([name], '^' + name + '$'))
        else:
            pending.append(name)
            if len(pending) == 15:
                flush()
    flush()
    return jobs


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--go', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--cache', type=Path, required=True, help='Private Go build cache')
    parser.add_argument('--compiled-cache', type=Path, help='Private retained executable cache')
    parser.add_argument('--shape-batch-size', type=int, default=1)
    parser.add_argument('--parallel-shapes', action='store_true', help='Run up to four independent finite shapes within each unchanged contained unit')
    parser.add_argument('--workers', type=int, choices=[1, 2], default=2)
    args = parser.parse_args()
    if not args.go.is_absolute() or args.shape_batch_size < 1 or args.shape_batch_size > 25:
        parser.error('absolute Go path and shape batch size 1..25 required')
    if args.shape_batch_size > 1 and args.compiled_cache is None:
        parser.error('larger shape batches require explicit compiled artifact reuse')
    if args.parallel_shapes and (args.compiled_cache is None or args.shape_batch_size == 1):
        parser.error('parallel shapes require grouped retained-artifact execution')
    for name in ['output', 'cache', 'compiled_cache']:
        value = getattr(args, name)
        if value is not None:
            setattr(args, name, value.resolve())
    root = Path(__file__).resolve().parents[2]
    output = args.output
    output.mkdir(parents=True, exist_ok=True)
    # Refuse overwriting prior receipts; each timed run has an independent record.
    if (output / 'inventory.json').exists() or (output / 'build.json').exists():
        parser.error('use a fresh output directory')
    started = time.monotonic()
    runner = Path(__file__).with_name('run.py').resolve()
    binary = output / 'pipelang.test'

    def snapshot():
        paths = sorted((root / 'src/lib/pipelang').rglob('*.go'))
        paths += sorted((root / 'tests/containedexec').glob('*'))
        paths += [root / 'go.mod', root / 'go.sum']
        return {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest() for p in paths if p.is_file()}

    source_before = snapshot()
    (output / 'source-hashes.json').write_text(json.dumps(source_before, indent=2) + '\n')

    def unit(label, command, cwd=root, build=False, grouped=False):
        prefix = output / label
        invocation = [sys.executable, str(runner), '--output', str(prefix), '--cache', str(args.cache), '--timeout', '900' if build else '180']
        if build or grouped:
            invocation += ['--memory-high-mib', '700']
        with Path(str(prefix) + '.runner.log').open('w') as log:
            rc = subprocess.run(invocation + ['--'] + command, cwd=cwd, stdout=log, stderr=subprocess.STDOUT).returncode
        report = json.loads(Path(str(prefix) + '.json').read_text())
        return dict(label=label, exit=rc, report=report)

    build = unit('build', [str(args.go), 'test', '-p', '1', '-c', '-o', str(binary), './src/lib/pipelang'], build=True)
    if build['exit']:
        return build['exit']
    listing = unit('list', [str(binary), '-test.list', '^(Test|Fuzz|Example)'])
    if listing['exit']:
        return listing['exit']
    tests = [s for s in (output / 'list.output').read_text().splitlines() if re.fullmatch(r'(?:Test|Fuzz|Example)\w*', s)]
    if not tests or len(tests) != len(set(tests)):
        raise RuntimeError('invalid discovered inventory')
    splits = {name: int(count) for name, count in re.findall(r'(Test\w+)=(\d+)', (runner.parent / 'README.md').read_text())}
    splits.update(TestV920NestedArrowMethodsTypes=4, TestV920NestedArrowMethodsCarriers=4, TestV920NestedArrowMethodsLayouts=4)
    if not set(splits) <= set(tests):
        raise RuntimeError('split inventory drift')
    jobs = plan(tests, splits, args.shape_batch_size)
    (output / 'inventory.json').write_text(json.dumps(dict(tests=tests, jobs=jobs), indent=2) + '\n')

    def execute(job):
        index, (names, pattern) = job
        command = [str(binary), '-test.run', pattern, '-test.v', '-test.count=1', '-test.timeout=170s']
        environment = []
        if args.parallel_shapes:
            environment += ['PIPELANG_PARALLEL_SHAPES=1']
            command += ['-test.parallel=4']
        if args.compiled_cache:
            environment += ['GOENV=off', 'PIPELANG_GENERATED_BATCH=1', 'PIPELANG_COMPILED_CACHE=' + str(args.compiled_cache)]
        if names == ['TestV960DepthThreeStraightLineInitializersMemory']:
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(output / 'fixtures')]
        if environment:
            command = ['env'] + environment + command
        grouped = args.shape_batch_size > 1 and len(names) > 1
        row = unit('batch-%03d' % index, command, cwd=root / 'src/lib/pipelang', grouped=grouped)
        row.update(index=index, tests=names)
        return row

    execution_started = time.monotonic()
    rows = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.workers) as pool:
        for row in pool.map(execute, enumerate(jobs)):
            rows.append(row)
            (output / 'suite.json').write_text(json.dumps(rows, indent=2) + '\n')
            if row['exit']:
                print('FAILED', row['index'], row['tests'], flush=True)
            elif len(rows) % 20 == 0:
                print('accepted', len(rows), 'of', len(jobs), flush=True)
    summary = dict(discovered_tests=len(tests), units=len(rows), workers=args.workers, shape_batch_size=args.shape_batch_size, parallel_shapes=args.parallel_shapes,
                   failed=[r['index'] for r in rows if r['exit']], source_unchanged=snapshot() == source_before,
                   overall_elapsed_s=time.monotonic() - started, execution_elapsed_s=time.monotonic() - execution_started,
                   sum_unit_elapsed_s=sum(r['report'].get('elapsed_s', 0) for r in rows))
    (output / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
    print(json.dumps(summary), flush=True)
    return int(bool(summary['failed']) or not summary['source_unchanged'])


if __name__ == '__main__':
    sys.exit(main())
