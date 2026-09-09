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
import shutil
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
                # Cold native preparation must also fit the 30-second unit.
                # Preserve all families while partitioning independent choices.
                if version >= 87:
                    for choices in [0, 1, 2, 3, -1]:
                        suffix = 'choices' + str(choices)
                        if version == 91:
                            for branch in ['false', 'true']:
                                label = prefix + '/' + suffix + '/branch' + branch
                                jobs.append(([label], pattern + '/^' + suffix + '$/^branch' + branch + '$'))
                        else:
                            jobs.append(([prefix + '/' + suffix], pattern + '/^' + suffix + '$'))
                else:
                    jobs.append(([prefix], pattern))
        elif name in splits:
            flush()
            for start in range(0, splits[name], shape_batch_size):
                indices = list(range(start, min(start + shape_batch_size, splits[name])))
                prefix = 'shape' if name == 'TestV810TerminalTreeAllShapesAndPaths' else ''
                labels = [prefix + str(i) for i in indices]
                jobs.append(([name + '/' + label for label in labels], '^' + name + '$/^(' + '|'.join(labels) + ')$'))
        elif re.fullmatch(r'TestV(?:9[2-9]0|1000|1010|1020|1030|1040|1050|1060).*Memory', name):
            flush()
            for shape in range(36 if name.startswith(("TestV1040", "TestV1050")) else 12 if name.startswith(("TestV970", "TestV990", "TestV1020", "TestV1030", "TestV1060")) else 4):
                label = name + '/choices' + str(shape)
                jobs.append(([label], '^' + name + '$/./^choices' + str(shape) + '$'))
        elif name.endswith('Memory'):
            flush()
            jobs.append(([name], '^' + name + '$'))
        else:
            pending.append(name)
            if len(pending) == 1:
                flush()
    flush()
    return jobs


def cleanup_native_build_cache(path, rows):
    size = sum(p.stat().st_size for p in path.rglob('*') if p.is_file())
    safe = bool(rows) and all(r['report'].get('tree_removed') and not Path(r['report']['cgroup']).exists() for r in rows)
    if safe:
        shutil.rmtree(path)
    return size, safe


def artifact_inventory(cache, logs, representation=None):
    """Record artifacts actually verified/executed; never infer liveness from age."""
    hits = re.findall(r'generated_compiled_artifact packages=\d+ cache_hit=(true|false) key=([0-9a-f]{64})\b', logs)
    if not hits:
        raise RuntimeError('no executed artifact inventory')
    entries = {}
    for key in sorted({key for _, key in hits}):
        directory = cache / key
        record_path, binary = directory / 'record.json', directory / 'program.test'
        if directory.is_symlink() or record_path.is_symlink() or binary.is_symlink():
            raise RuntimeError('symlink in executed artifact inventory')
        record = json.loads(record_path.read_text())
        if (record.get('Version') != 'pipelang-native-validation-v2' or record.get('Key') != key
                or not re.fullmatch('[0-9a-f]{64}', record.get('BinarySHA256', ''))):
            raise RuntimeError('invalid executed artifact record')
        if binary.is_file():
            size = original_bytes = binary.stat().st_size
        else:
            if representation is None:
                raise RuntimeError('executed artifact has no retained representation')
            recipe_path = representation / key / 'recipe.json'
            if recipe_path.is_symlink() or recipe_path.parent.is_symlink():
                raise RuntimeError('linked representation recipe')
            recipe = json.loads(recipe_path.read_text())
            if (recipe.get('key') != key or recipe.get('sha256') != record['BinarySHA256']
                    or recipe.get('unsupported') or not 0 < recipe.get('size', 0) <= 16 << 20):
                raise RuntimeError('invalid executed representation identity')
            size, original_bytes = recipe['size'], 0
        entries[key] = dict(binary_sha256=record['BinarySHA256'], binary_bytes=size,
                            original_bytes=original_bytes, manifest_bytes=record_path.stat().st_size)
    return dict(cache=str(cache), entries=entries, hits=sum(hit == 'true' for hit, _ in hits),
                misses=sum(hit == 'false' for hit, _ in hits),
                retained_bytes=sum(e['original_bytes'] + e['manifest_bytes'] for e in entries.values()),
                logical_binary_bytes=sum(e['binary_bytes'] for e in entries.values()),
                representation=str(representation) if representation else None)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--go', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--cache', type=Path, required=True, help='Private Go build cache')
    parser.add_argument('--compiled-cache', type=Path, help='Private retained executable cache')
    parser.add_argument('--shape-batch-size', type=int, default=1)
    parser.add_argument('--parallel-shapes', action='store_true', help='Run up to four independent finite shapes within each unchanged contained unit')
    parser.add_argument('--audit-generated', action='store_true', help='Log queued generated source/fixture identities and native timings')
    parser.add_argument('--test-family', help='Run one discovered family and explicitly report partial-suite proof')
    parser.add_argument('--shared-export', type=Path, help='Fresh private v0.91 bundle exports for exact transcript replay or shared-runtime probes')
    parser.add_argument('--native-bundle', action=argparse.BooleanOptionalAction, default=None,
                        help='Bounded shared native test bundles across compatible generated families; default for retained-cache runs on Linux')
    parser.add_argument('--native-representation', action=argparse.BooleanOptionalAction, default=False,
                        help='Opt in to exact compressed native preparation/replay on Linux; ordinary execution is the default')
    parser.add_argument('--representation-cache', type=Path, help='Private representation store (default: sibling of compiled cache)')
    parser.add_argument('--workers', type=int, choices=[1, 2], default=2)
    args = parser.parse_args()
    if args.native_representation and (args.compiled_cache is None or sys.platform != 'linux'):
        parser.error('native representations require a Linux compiled cache')
    if args.native_bundle is None:
        args.native_bundle = args.compiled_cache is not None and sys.platform == 'linux'
    if not args.go.is_absolute() or args.shape_batch_size < 1 or args.shape_batch_size > 25:
        parser.error('absolute Go path and shape batch size 1..25 required')
    if args.shape_batch_size > 1 and args.compiled_cache is None:
        parser.error('larger shape batches require explicit compiled artifact reuse')
    if args.native_bundle and args.compiled_cache is None:
        parser.error('native bundles require explicit compiled artifact reuse')
    if args.shared_export and (not args.native_bundle or args.test_family != 'TestV910NestedTerminalInitializersLayouts'
                               or not args.shared_export.is_absolute() or args.shared_export.is_symlink()
                               or not args.shared_export.is_dir() or args.shared_export.stat().st_mode & 0o777 != 0o700
                               or any(args.shared_export.iterdir())):
        parser.error('shared export requires the complete v0.91 bundle family and a fresh absolute private directory')
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
        paths += sorted(p for p in (root / 'tests/containedexec').rglob('*')
                        if p.suffix in {'.py', '.go', '.json', '.md'} and '__pycache__' not in p.parts)
        paths += [root / 'go.mod', root / 'go.sum']
        return {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest() for p in paths if p.is_file()}

    source_before = snapshot()
    (output / 'source-hashes.json').write_text(json.dumps(source_before, indent=2) + '\n')

    def unit(label, command, cwd=root, build=False, grouped=False):
        prefix = output / label
        invocation = [sys.executable, str(runner), '--output', str(prefix), '--cache', str(args.cache), '--timeout', '30']
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
    discovered_count = len(tests)
    if args.test_family:
        if args.test_family not in tests:
            parser.error('test family was not discovered')
        tests = [args.test_family]
        splits = {name: count for name, count in splits.items() if name == args.test_family}
    jobs = plan(tests, splits, args.shape_batch_size)
    (output / 'inventory.json').write_text(json.dumps(dict(tests=tests, jobs=jobs), indent=2) + '\n')

    representation_config = output / 'representation-config.json'
    if args.native_representation:
        representation_cache = args.representation_cache or args.compiled_cache.with_name(args.compiled_cache.name + '-representations')
        preparation = unit('representation-init', [sys.executable, '-B', str(runner.parent / 'native_artifacts.py'), '--cache', str(args.compiled_cache), '--config', str(representation_config), '--directory', str(representation_cache), '--go', str(args.go), 'init'])
        if preparation['exit']:
            return preparation['exit']

    native_build_cache = output / 'native-build-cache'
    if args.native_bundle:
        native_build_cache.mkdir(mode=0o700)

    def execute(job):
        index, (names, pattern) = job
        command = [str(binary), '-test.run', pattern, '-test.v', '-test.count=1', '-test.timeout=25s']
        environment = []
        if args.audit_generated:
            environment += ['PIPELANG_BUNDLE_AUDIT=1', 'PIPELANG_PERFORMANCE_PROFILE=1']
        if args.native_bundle:
            environment += ['PIPELANG_NATIVE_BUNDLE=1', 'PIPELANG_BUNDLE_BUILD_CACHE=' + str(native_build_cache)]
        if args.parallel_shapes:
            environment += ['PIPELANG_PARALLEL_SHAPES=1']
            command += ['-test.parallel=4']
        if args.compiled_cache:
            environment += ['GOENV=off', 'PIPELANG_GENERATED_BATCH=1', 'PIPELANG_COMPILED_CACHE=' + str(args.compiled_cache)]
        if args.shared_export:
            environment += ['PIPELANG_SHARED_EXPORT=' + str(args.shared_export)]
        if all(name.startswith('TestV960DepthThreeStraightLineInitializersMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(output / 'fixtures')]
        if all(name.startswith('TestV970DepthThreeTerminalInitializersMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(output / 'fixtures-v097')]
        if all(name.startswith('TestV1060TerminalBooleanSelectorTestsMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(output / 'fixtures-v106')]
        if all(name.startswith('TestV1050DepthThreeTerminalConditionalTestsMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(output / 'fixtures-v105')]
        if all(name.startswith('TestV1040NestedTerminalConditionalTestsMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(output / 'fixtures-v104')]
        if all(name.startswith('TestV1030TerminalConditionalTestsMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(output / 'fixtures-v103')]
        if all(name.startswith('TestV1020TerminalBooleanSelectorInitializersMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(output / 'fixtures-v102')]
        if all(name.startswith('TestV1010StraightLineBooleanSelectorInitializersMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(output / 'fixtures-v101')]
        if all(name.startswith('TestV1000ArrowBooleanSelectorsMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(output / 'fixtures-v100')]
        if all(name.startswith('TestV990TerminalLeafBooleanSelectorsMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(output / 'fixtures-v099')]
        if all(name.startswith('TestV980ConditionalBooleanSelectorsMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(output / 'fixtures-v098')]
        if environment:
            command = ['env'] + environment + command
        if args.native_representation:
            command = [sys.executable, '-B', str(runner.parent / 'native_artifacts.py'), '--cache', str(args.compiled_cache), '--config', str(representation_config), '--receipt', str(output / ('native-%03d.json' % index)), 'run', '--', *command]
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
    native_build_bytes = None
    native_build_removed = None
    if args.native_bundle:
        # This path was freshly created by this run, never supplied by a caller.
        native_build_bytes = sum(p.stat().st_size for p in native_build_cache.rglob('*') if p.is_file())
        native_build_removed = None
    summary = dict(native_representation=args.native_representation, native_bundle=args.native_bundle, native_build_cache_bytes=native_build_bytes, native_build_cache_removed=native_build_removed, discovered_tests=discovered_count, selected_tests=len(tests), test_family=args.test_family, partial_suite=bool(args.test_family), units=len(rows), workers=args.workers, shape_batch_size=args.shape_batch_size, parallel_shapes=args.parallel_shapes,
                   failed=[r['index'] for r in rows if r['exit']], source_unchanged=snapshot() == source_before,
                   overall_elapsed_s=time.monotonic() - started, execution_elapsed_s=time.monotonic() - execution_started,
                   sum_unit_elapsed_s=sum(r['report'].get('elapsed_s', 0) for r in rows))
    if (args.compiled_cache and not summary['partial_suite'] and not summary['failed']
            and summary['source_unchanged'] and native_build_removed is not False
            and all(r['report'].get('tree_removed') and not Path(r['report']['cgroup']).exists() for r in rows)):
        # Only a complete successful run can describe the live retained set.
        # Digests were verified by the executing harness; this receipt does not
        # authorize deletion or replace revalidation before a cache migration.
        logs = '\n'.join((output / (r['label'] + '.output')).read_text() for r in rows)
        representation = Path(json.loads(representation_config.read_text())['root']) if args.native_representation else None
        inventory = artifact_inventory(args.compiled_cache, logs, representation)
        (output / 'artifacts.json').write_text(json.dumps(inventory, indent=2) + '\n')
    summary['overall_elapsed_s'] = time.monotonic() - started
    (output / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
    print(json.dumps(summary), flush=True)
    return int(bool(summary['failed']) or not summary['source_unchanged'] or native_build_removed is False)


if __name__ == '__main__':
    sys.exit(main())
