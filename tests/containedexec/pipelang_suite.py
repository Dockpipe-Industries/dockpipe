#!/usr/bin/env python3
"""Build current PipeLang tests, then execute the complete contained conformance inventory.

Executable reuse is opt-in. Every oracle and direct compiler resource probe runs.
Use shape-batch-size 1 to populate artifacts; larger warm groups reduce launch and
repeated toolchain verification costs without changing test selection or limits.
"""
import argparse
import concurrent.futures
import hashlib
import io
import json
from pathlib import Path
import re
import shutil
import subprocess
import sys
import time
from job import verify_job
from budget import DiskBudget
from estate import SharedBudget, storage_limit
from reporting import summarize
from scheduling import measured_plan, load_profile, observed_profile
from campaign import Campaign, BuildStore, InputGuard, atomic_json, digest, fingerprint, host_identity
from verification import StageRunner, POLICY, source_paths, toolchain_identity, dependency_guard


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
        elif name == 'TestV1130EnumsMemory':
            flush()
            for shape in range(3):
                label = name + '/choices' + str(shape)
                jobs.append(([label], '^' + name + '$/./^choices' + str(shape) + '$'))
        elif name in splits:
            flush()
            for start in range(0, splits[name], shape_batch_size):
                indices = list(range(start, min(start + shape_batch_size, splits[name])))
                prefix = 'shape' if name == 'TestV810TerminalTreeAllShapesAndPaths' else ''
                labels = [prefix + str(i) for i in indices]
                jobs.append(([name + '/' + label for label in labels], '^' + name + '$/^(' + '|'.join(labels) + ')$'))
        elif re.fullmatch(r'TestV(?:9[2-9]0|1000|1010|1020|1030|1040|1050|1060|1070|1080|1090|1100|1110|1120).*Memory', name):
            flush()
            for shape in range(108 if name.startswith("TestV1090") else 36 if name.startswith(("TestV1040", "TestV1050", "TestV1070", "TestV1080", "TestV1110")) else 12 if name.startswith(("TestV970", "TestV990", "TestV1020", "TestV1030", "TestV1060", "TestV1100", "TestV1120")) else 4):
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
    # Consume output incrementally; retain only unique identities and counters.
    lines = io.StringIO(logs) if isinstance(logs, str) else logs
    keys, hits, misses = set(), 0, 0
    for line in lines:
        for hit, key in re.findall(r'generated_compiled_artifact packages=\d+ cache_hit=(true|false) key=([0-9a-f]{64})\b', line):
            keys.add(key)
            hits += hit == 'true'
            misses += hit == 'false'
    if not keys:
        raise RuntimeError('no executed artifact inventory')
    entries = {}
    for key in sorted(keys):
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
    return dict(cache=str(cache), entries=entries, hits=hits, misses=misses,
                retained_bytes=sum(e['original_bytes'] + e['manifest_bytes'] for e in entries.values()),
                logical_binary_bytes=sum(e['binary_bytes'] for e in entries.values()),
                representation=str(representation) if representation else None)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--toolchain-read-buffer', action=argparse.BooleanOptionalAction, default=True, help='Reuse one 32-KiB toolchain hash buffer; disable for matched controls')
    parser.add_argument('--identity-profile', action='store_true', help='Opt-in exclusive identity attribution')
    parser.add_argument('--selection-file', type=Path, help='Explicit logical case sample; reports partial proof')
    scheduling = parser.add_mutually_exclusive_group()
    scheduling.add_argument('--schedule-profile', type=Path, help='Required current-input warm singleton timings for conservative groups')
    scheduling.add_argument('--auto-schedule-profile', type=Path, help='Optional warm timings; missing or stale hints fall back to singletons')
    parser.add_argument('--disk-budget-gib', type=int, default=96)
    parser.add_argument('--campaign-budget', type=Path, help=argparse.SUPPRESS)
    parser.add_argument('--build-store', type=Path, help='Persistent verified test-binary store (default sibling builds)')
    parser.add_argument('--native-build-cache', type=Path, help='Persistent native Go build cache (default per-output cache)')
    parser.add_argument('--mode', choices=['fresh', 'resume'], default='fresh')
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
    storage_limit(args.disk_budget_gib)
    verify_job()
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
    output.mkdir(parents=True, exist_ok=True, mode=0o700)
    native_build_cache = args.native_build_cache or output / 'native-build-cache'
    build_store = args.build_store or output.parent / 'builds'
    budget_roots = [args.cache, args.compiled_cache, build_store, output]
    if args.native_bundle:
        budget_roots.append(native_build_cache)
    if args.native_representation:
        budget_roots.append(args.representation_cache or args.compiled_cache.with_name(args.compiled_cache.name + '-representations'))
    if args.shared_export:
        budget_roots.append(args.shared_export)
    budget = (SharedBudget(args.campaign_budget, budget_roots, args.disk_budget_gib << 30)
              if args.campaign_budget else DiskBudget(budget_roots, output, args.disk_budget_gib << 30))
    campaign = Campaign(output / 'campaign', args.mode)
    started = time.monotonic()
    runner = Path(__file__).with_name('run.py').resolve()
    binary = output / 'pipelang.test'

    paths = source_paths()
    guard = dependency_guard(args.go, args.cache, output)
    snapshot = guard.check
    source_before = snapshot()
    toolchain = toolchain_identity(args.go)
    identity = dict(source=source_before, toolchain=toolchain['digest'], policy=POLICY,
                    host=host_identity(), high=700, native_bundle=args.native_bundle,
                    representation=args.native_representation, parallel_shapes=args.parallel_shapes,
                    audit=args.audit_generated, identity_profile=args.identity_profile, toolchain_read_buffer=args.toolchain_read_buffer, compiled_cache=str(args.compiled_cache))
    atomic_json(output / 'source-hashes.json', fingerprint(paths))
    bootstrap = StageRunner(campaign, 'bootstrap', identity, ['build'], args.cache, snapshot)
    store = BuildStore(build_store)
    # Execution-only Python/report edits do not require relinking the Go test binary.
    build_files = {p: v for p, v in guard.identity['files'].items()
                   if not p.startswith(str(runner.parent) + '/') or p.endswith('.go')}
    build_key = digest(dict(files=build_files, settings=POLICY, target='./src/lib/pipelang'))
    binary = store.get(build_key)
    build_hit = binary is not None
    build_reason = store.reason
    if binary is None:
        build = bootstrap.run(['build'], lambda d: ['env', 'GOENV=off', 'GOMEMLIMIT=600MiB', str(args.go), 'test', '-p', '1', '-c', '-o', str(d / 'pipelang.test'), './src/lib/pipelang'],
                              artifacts=lambda d: [d / 'pipelang.test'])
        if build['exit']:
            return build['exit']
        binary = store.publish(build_key, Path(build['directory']) / 'pipelang.test', build['receipt'])
    else:
        # A reused executable is an artifact, not a reused semantic test result.
        build = bootstrap.run(['build'], ['/usr/bin/true'], artifacts=lambda d: [binary])
        if build['exit']:
            return build['exit']
    listing_stage = StageRunner(campaign, 'discovery', dict(identity, binary=fingerprint([binary])['digest']), ['list'], args.cache, snapshot, parents=['bootstrap'])
    listing = listing_stage.run(['list'], [str(binary), '-test.list', '^(Test|Fuzz|Example)'])
    if listing['exit']:
        return listing['exit']
    tests = [s for s in (Path(listing['directory']) / 'unit.output').read_text().splitlines() if re.fullmatch(r'(?:Test|Fuzz|Example)\w*', s)]
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
    if args.selection_file:
        selected = json.loads(args.selection_file.read_text())
        jobs = [(names, pattern) for names, pattern in plan(tests, splits, 1) if names[0] in selected]
        if {n for names, _ in jobs for n in names} != set(selected):
            raise RuntimeError('sample contains undiscovered logical cases')
    scheduling_identity = dict(inputs=source_before, host=identity['host'], policy=POLICY,
                               workers=args.workers, native_bundle=args.native_bundle,
                               representation=args.native_representation, toolchain_read_buffer=args.toolchain_read_buffer, identity_profile=args.identity_profile)
    admitted_profile = {}
    schedule = dict(mode='singleton', status='disabled', admitted_cases=0)
    profile_path = args.schedule_profile or args.auto_schedule_profile
    if profile_path:
        if args.compiled_cache is None:
            parser.error('measured grouping requires retained executables')
        admitted_profile, admission = load_profile(profile_path, scheduling_identity, args.compiled_cache,
                                                   automatic=args.auto_schedule_profile is not None)
        schedule = dict(mode='automatic' if args.auto_schedule_profile else 'explicit',
                        profile=str(profile_path), admitted_cases=len(admitted_profile), **admission)
        jobs = measured_plan(jobs, admitted_profile)
    schedule.update(pairs=sum(len(names) == 2 for names, _ in jobs),
                    singletons=sum(len(names) == 1 for names, _ in jobs))
    atomic_json(output / 'scheduling.json', schedule)
    atomic_json(output / 'inventory.json', dict(tests=tests, jobs=jobs))
    cases = [case for names, _ in jobs for case in names]
    stage = StageRunner(campaign, 'suite', identity, cases, args.cache, snapshot, parents=['discovery'])

    representation_config = output / 'representation-config.json'
    if args.native_representation:
        representation_cache = args.representation_cache or args.compiled_cache.with_name(args.compiled_cache.name + '-representations')
        representation_stage = StageRunner(campaign, 'representation', identity, ['representation-init'], args.cache, snapshot)
        preparation = representation_stage.run(['representation-init'], [sys.executable, '-B', str(runner.parent / 'native_artifacts.py'), '--cache', str(args.compiled_cache), '--config', str(representation_config), '--directory', str(representation_cache), '--go', str(args.go), 'init'], artifacts=lambda d: [representation_config])
        if preparation['exit']:
            return preparation['exit']

    native_build_cache = args.native_build_cache or output / 'native-build-cache'
    if args.native_bundle:
        native_build_cache.mkdir(mode=0o700, exist_ok=True)



    def required_artifacts(directory):
        paths = [p for p in (directory / 'fixtures').rglob('*') if p.is_file()]
        if args.compiled_cache and (directory / 'unit.output').exists():
            with (directory / 'unit.output').open() as log:
                keys = {key for line in log for key in re.findall(r'generated_compiled_artifact .*key=([a-f0-9]{64})', line)}
            for key in keys:
                paths += [args.compiled_cache / key / 'record.json']
                if not args.native_representation:
                    paths += [args.compiled_cache / key / 'program.test']
        return paths

    def execute(job):
        budget.check()
        index, (names, pattern) = job
        def command_for(directory):
            return make_command(directory, names, pattern)
        row = stage.run(names, command_for, cwd=root / 'src/lib/pipelang', go_cases=True,
                        artifacts=required_artifacts)
        row.update(index=index, tests=names)
        if row['exit'] and len(names) > 1:
            retries = []
            for single, single_pattern in plan(tests, splits, 1):
                if single[0] in names:
                    retried = stage.run(single, lambda d, n=single, p=single_pattern: make_command(d, n, p),
                                        cwd=root / 'src/lib/pipelang', go_cases=True, artifacts=required_artifacts, retry_of=row['receipt'])
                    retried.update(index=index, tests=single, failed_group=row['receipt'])
                    retries.append(retried)
            return retries
        return [row]

    def make_command(directory, names, pattern):
        command = [str(binary), '-test.run', pattern, '-test.v', '-test.count=1', '-test.timeout=25s']
        environment = ['PIPELANG_CACHE_BUDGET_FILE=' + str(budget.record)]
        environment += ['PIPELANG_TOOLCHAIN_READ_BUFFER=' + ('1' if args.toolchain_read_buffer else '0')]
        if args.identity_profile:
            environment += ['PIPELANG_IDENTITY_PROFILE=1']
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
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(directory / 'fixtures')]
        if all(name.startswith('TestV970DepthThreeTerminalInitializersMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(directory / 'fixtures')]
        if all(name.startswith('TestV1090TerminalCombinedSelectorArmsMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(directory / 'fixtures')]
        if all(name.startswith('TestV1080TerminalInnerSelectorArmsMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(directory / 'fixtures')]
        if all(name.startswith('TestV1070TerminalSelectorValueArmsMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(directory / 'fixtures')]
        if all(name.startswith('TestV1060TerminalBooleanSelectorTestsMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(directory / 'fixtures')]
        if all(name.startswith('TestV1050DepthThreeTerminalConditionalTestsMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(directory / 'fixtures')]
        if all(name.startswith('TestV1040NestedTerminalConditionalTestsMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(directory / 'fixtures')]
        if all(name.startswith('TestV1030TerminalConditionalTestsMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(directory / 'fixtures')]
        if all(name.startswith('TestV1020TerminalBooleanSelectorInitializersMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(directory / 'fixtures')]
        if all(name.startswith('TestV1010StraightLineBooleanSelectorInitializersMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(directory / 'fixtures')]
        if all(name.startswith('TestV1000ArrowBooleanSelectorsMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(directory / 'fixtures')]
        if all(name.startswith('TestV990TerminalLeafBooleanSelectorsMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(directory / 'fixtures')]
        if all(name.startswith(('TestV1100StraightLineSelectorValueArmsMemory', 'TestV1110TerminalLeafSelectorValueArmsMemory', 'TestV1120ArrowSelectorValueArmsMemory', 'TestV1130EnumsMemory')) for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(directory / 'fixtures')]
        if all(name.startswith('TestV980ConditionalBooleanSelectorsMemory') for name in names):
            environment += ['PIPELANG_MEMORY_FIXTURES=' + str(directory / 'fixtures')]
        if environment:
            command = ['env'] + environment + command
        if args.native_representation:
            command = [sys.executable, '-B', str(runner.parent / 'native_artifacts.py'), '--cache', str(args.compiled_cache), '--config', str(representation_config), '--receipt', str(directory / 'native.json'), 'run', '--', *command]
        return command

    execution_started = time.monotonic()
    rows = []
    seen_receipts = set()
    for case, reference in stage.prior.items():
        if reference['id'] not in seen_receipts:
            receipt = stage.resume(case)
            row = dict(receipt['result'], report=receipt['report'], resumed=True, receipt=receipt['id'])
            row['index'] = next(i for i, (names, _) in enumerate(jobs) if case in names)
            rows.append(row)
            seen_receipts.add(receipt['id'])
    stage.reused = len(rows)
    pending = []
    for index, (names, pattern) in enumerate(jobs):
        missing = [name for name in names if name not in stage.prior]
        if missing == names:
            pending.append((index, (names, pattern)))
        elif missing:
            for single, single_pattern in plan(tests, splits, 1):
                if single[0] in missing:
                    pending.append((index, (single, single_pattern)))
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.workers) as pool:
        for future in concurrent.futures.as_completed([pool.submit(execute, job) for job in pending]):
            finished = future.result()
            rows.extend(finished)
            row = next((r for r in finished if r['exit']), finished[-1])
            if row['exit']:
                print('FAILED', row['index'], row['tests'], flush=True)
            elif len(rows) % 20 == 0:
                print('accepted', len(rows), 'of', len(jobs), flush=True)
    rows.sort(key=lambda r: r['index'])
    campaign.serialized_bytes += atomic_json(output / 'suite.json', rows)
    reconciliation = stage.finish()
    native_build_bytes = None
    native_build_removed = None
    if args.native_bundle:
        # This path was freshly created by this run, never supplied by a caller.
        native_build_bytes = budget.root_bytes(native_build_cache)
        native_build_removed = None
    summary = dict(reconciliation=reconciliation, build_cache_hit=build_hit, build_cache_reason=build_reason, build_key=build_key, native_representation=args.native_representation, native_bundle=args.native_bundle, native_build_cache_bytes=native_build_bytes, native_build_cache_removed=native_build_removed, discovered_tests=discovered_count, selected_tests=len(tests), test_family=args.test_family, partial_suite=bool(args.test_family or args.selection_file), units=len(rows), workers=args.workers, shape_batch_size=args.shape_batch_size, parallel_shapes=args.parallel_shapes,
                   failed=[r['index'] for r in rows if r['exit']], source_unchanged=snapshot() == source_before,
                   overall_elapsed_s=time.monotonic() - started, execution_elapsed_s=time.monotonic() - execution_started,
                   sum_unit_elapsed_s=sum(r['report'].get('elapsed_s', 0) for r in rows))
    if (args.compiled_cache and not summary['partial_suite'] and not summary['failed']
            and summary['source_unchanged'] and native_build_removed is not False
            and all(r['report'].get('tree_removed') and not Path(r['report']['cgroup']).exists() for r in rows)):
        # Only a complete successful run can describe the live retained set.
        # Digests were verified by the executing harness; this receipt does not
        # authorize deletion or replace revalidation before a cache migration.
        def log_lines():
            for row in rows:
                with (campaign.root / (row['label'] + '.output')).open() as log:
                    yield from log
        representation = Path(json.loads(representation_config.read_text())['root']) if args.native_representation else None
        inventory = artifact_inventory(args.compiled_cache, log_lines(), representation)
        (output / 'artifacts.json').write_text(json.dumps(inventory, indent=2) + '\n')
    summary['scheduling'] = schedule
    campaign.serialized_bytes += atomic_json(output / 'timing.json', summarize(rows, time.monotonic() - execution_started, args.workers, campaign.root))
    if args.compiled_cache:
        campaign.serialized_bytes += atomic_json(output / 'schedule-profile.json', observed_profile(rows, scheduling_identity, args.compiled_cache, inherited=admitted_profile))
    summary['disk_budget'] = budget.check(force=True)
    budget.close()
    summary['toolchain_unchanged'] = toolchain_identity(args.go) == toolchain
    summary['serialized_bytes'] = campaign.serialized_bytes
    summary['overall_elapsed_s'] = time.monotonic() - started
    campaign.close()
    (output / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
    print(json.dumps(summary), flush=True)
    return int(bool(summary['failed']) or not summary['source_unchanged'] or native_build_removed is False or reconciliation['missing'] or not summary['toolchain_unchanged'])


if __name__ == '__main__':
    sys.exit(main())
