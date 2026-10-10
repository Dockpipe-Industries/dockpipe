#!/usr/bin/env python3
"""Prepare/replay current exported conformance artifacts in bounded Linux units.

The plan derives from current artifacts with bounded references and schedules.
Normal cache integration is owned by native_artifacts.py; this CLI also supports exports.
"""
import argparse
import concurrent.futures
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import sysconfig
import tempfile
import time

from shared_runtime_probe import containment, digest, dump, inputs, seal, validate_toolchain
from transcript_runtime import Decoder, Service, check
from transcript_replay import replay

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[1]
ENVIRONMENT = ['GOFLAGS', 'GOEXPERIMENT', 'CGO_ENABLED', 'GOOS', 'GOARCH', 'GOAMD64',
               'GODEBUG', 'GOFIPS140', 'GO_EXTLINK_ENABLED', 'GO_LDSO',
               'CGO_CFLAGS', 'CGO_CPPFLAGS', 'CGO_CXXFLAGS', 'CGO_LDFLAGS']


def source_identity(source):
    return hashlib.sha256(json.dumps(inputs(source, code_only=True), sort_keys=True).encode()).hexdigest()


def harness_inputs():
    paths = list(HERE.glob('transcript*.*')) + list((HERE / 'transcript').glob('*.go'))
    paths += [HERE / 'shared_runtime_probe.py', HERE / 'run.py']
    return {str(p.relative_to(REPO)): digest(p) for p in sorted(paths)}


def derive_plan(records, max_references=16, unit_size=32):
    """Bounded deterministic size-stratified selection; no corpus admission list."""
    check(1 <= max_references <= 16 and 1 <= unit_size <= 50, 'invalid plan bounds')
    keys = sorted(records, key=lambda k: (records[k].get('size', 0), k))
    check(bool(keys), 'empty artifact inventory')
    count = min(max_references, len(keys))
    anchors = [keys[i * len(keys) // count] for i in range(count)]
    root = anchors[0]
    recipes = {}
    for key in keys:
        base = None if key == root else root if key in anchors else min(
            anchors, key=lambda a: (abs(records[a].get('size', 0) - records[key].get('size', 0)), a))
        recipes[key] = dict(records[key], base=base)
    ordered = sorted(keys, key=lambda k: (recipes[k]['base'] or '', k))
    result = dict(schema=1, root=root, anchors=anchors, recipes=recipes,
                  schedule=[ordered[i:i + unit_size] for i in range(0, len(keys), unit_size)])
    validate_plan(result)
    return result


def validate_plan(plan):
    check(plan['schema'] == 1 and bool(plan['recipes']), 'unsupported artifact plan')
    keys, anchors, root = set(plan['recipes']), set(plan['anchors']), plan['root']
    check(root in anchors and anchors <= keys and 1 <= len(anchors) <= 16, 'invalid references')
    import re
    check(all(re.fullmatch('[0-9a-f]{64}', key) for key in keys), 'invalid key')
    check(bool(plan['schedule']) and all(0 < len(part) <= 50 for part in plan['schedule']), 'invalid schedule')
    scheduled = [key for part in plan['schedule'] for key in part]
    check(len(scheduled) == len(set(scheduled)) and set(scheduled) == keys, 'incomplete schedule')
    for key, value in plan['recipes'].items():
        base = value['base']
        check(base is None if key == root else base == root if key in anchors else base in anchors,
              'invalid reference topology')


def command(args, **kwargs):
    result = subprocess.run(list(map(str, args)), capture_output=True, timeout=25, **kwargs)
    check(result.returncode == 0, result.stderr.decode(errors='replace')[-4000:])
    return result.stdout


def identity(directory):
    value = json.loads((directory / 'identity.json').read_text())
    check(value['harness'] == harness_inputs(), 'harness source changed; prepare again')
    check(value['environment'] == {key: os.getenv(key, '') for key in ENVIRONMENT}, 'build settings changed')
    validate_toolchain(value)
    for path, expected in value['support'].items():
        check(digest(directory / path) == expected, 'support changed: ' + path)
    return value


def init(args, plan):
    check(set(p.name for p in args.exports.iterdir()) == set(plan['recipes']), 'incomplete export')
    check(args.exports.stat().st_mode & 0o777 == 0o700, 'private exports required')
    baseline, records = {}, {}
    for key, recipe in plan['recipes'].items():
        source = args.exports / key
        check(source_identity(source) == recipe['code_sha256'], 'source changed since planning')
        export = json.loads((source / 'experiment.json').read_text())
        binary = Path(export['baseline']).resolve()
        record = json.loads((binary.parent / 'record.json').read_text())
        check(export['key'] == key == record['Key'] and
              record['Version'] == 'pipelang-native-validation-v2' and
              record['BinarySHA256'] == export['sha256'] == digest(binary), 'baseline identity mismatch')
        baseline[key] = str(binary)
        records[key] = dict(sha256=export['sha256'], code_sha256=source_identity(source))
    toolchain = {}
    for part in ['bin', 'pkg/tool', 'src', 'go.env', 'VERSION']:
        path = args.go.parent.parent / part
        for file in path.rglob('*') if path.is_dir() else [path]:
            if file.is_file() and not file.name.endswith('_test.go'):
                toolchain[str(file)] = digest(file)
    # Python is reconstruction support too: include its complete installed
    # standard library and loaded shared-object closure, not just the executable.
    stdlib = Path(sysconfig.get_path('stdlib'))
    for file in stdlib.rglob('*'):
        if file.is_file() and '__pycache__' not in file.parts and 'site-packages' not in file.parts:
            toolchain[str(file.resolve())] = digest(file)
    for line in Path('/proc/self/maps').read_text().splitlines():
        name = line.split()[-1]
        if name.startswith('/') and '.so' in name and Path(name).is_file():
            toolchain[str(Path(name).resolve())] = digest(Path(name))
    # Capture native loader/codec support as well as the pinned Go closure.
    for file in [args.zstd, args.library, Path(sys.executable).resolve(),
                 Path('/lib/x86_64-linux-gnu/libc.so.6'), Path('/lib64/ld-linux-x86-64.so.2')]:
        toolchain[str(file)] = digest(file)
    check(b'v1.4.8' in command([args.zstd, '--version']), 'requires selected Zstandard 1.4.8')
    command([args.go, 'build', '-p=1', '-o', args.directory / 'splice',
             *sorted((HERE / 'transcript').glob('*.go'))], env=dict(os.environ, GOENV='off'))
    command([args.directory / 'splice', 'selftest'])
    shutil.copy2(args.library, args.directory / 'libzstd.so')
    shutil.copy2(args.zstd, args.directory / 'zstd')
    dump(args.directory / 'plan.json', plan)
    value = dict(schema=1, harness=harness_inputs(), toolchain=toolchain,
                 environment={key: os.getenv(key, '') for key in ENVIRONMENT},
                 support={name: digest(args.directory / name) for name in ['splice', 'libzstd.so', 'zstd', 'plan.json']},
                 baseline=baseline, records=records, scaffold_level=args.scaffold_level)
    dump(args.directory / 'identity.json', value)


def extract(key, value, args, work):
    binary = Path(value['baseline'][key])
    fd = seal(binary, value['records'][key]['sha256'])
    try:
        command([args.directory / 'splice', 'extract', f'/proc/self/fd/{fd}', work / key], pass_fds=(fd,))
    finally:
        os.close(fd)
    return work / key


def encode(args, plan, value):
    rows = []
    # A fresh unit handles two objects. Scratch files are owned by this call;
    # retained native caches and previous evidence are never pruned.
    for key in sorted(plan['recipes'])[args.part * 2:(args.part + 1) * 2]:
        begin = time.monotonic()
        check(source_identity(args.exports / key) == plan['recipes'][key]['code_sha256'], 'source changed')
        target = args.directory / 'payload' / key
        target.mkdir(parents=True, mode=0o700)
        with tempfile.TemporaryDirectory(prefix='encode-', dir=args.scratch) as temporary:
            work = Path(temporary)
            source = extract(key, value, args, work)
            base = plan['recipes'][key]['base']
            reference = extract(base, value, args, work) if base else None
            recipe = json.loads((source / 'splice.json').read_text())
            recipe.update(base=base, encoded={})
            for kind, filename, level in [('scaffold', 'scaffold', value['scaffold_level']),
                                          ('tokens', 'selected.tokens', 12)]:
                start = time.monotonic()
                patch = ['--patch-from=' + str(reference / filename)] if reference else []
                data = command([args.directory / 'zstd', '-q', '--single-thread', '--long=24',
                                '-' + str(level), '-c', *patch, source / filename])
                elapsed = time.monotonic() - start
                dictionary = ['-D', reference / filename] if reference else []
                decoded = command([args.directory / 'zstd', '-q', '-d', '-c', *dictionary], input=data)
                check(decoded == (source / filename).read_bytes(), 'preparation roundtrip failed')
                path = target / (kind + '.zst')
                path.write_bytes(data)
                recipe['encoded'][kind] = dict(path=str(path.relative_to(args.directory)),
                    bytes=len(data), sha256=digest(path), encode_s=elapsed)
            recipe['prepare_s'] = time.monotonic() - begin
            dump(target / 'recipe.json', recipe)
            rows.append(key)
    return dict(prepared=rows)


def manifest(args, plan, value):
    recipes = {key: json.loads((args.directory / 'payload' / key / 'recipe.json').read_text()) for key in plan['recipes']}
    for key, recipe in recipes.items():
        check(recipe['key'] == key and recipe['base'] == plan['recipes'][key]['base'] and
              recipe['sha256'] == value['records'][key]['sha256'], 'recipe mismatch')
        for part in recipe['encoded'].values():
            check(digest(args.directory / part['path']) == part['sha256'], 'payload changed')
    result = dict(schema=1, recipes=recipes, root=plan['root'], anchors=plan['anchors'],
                  schedule=plan['schedule'], baseline=value['baseline'], support=value['support'],
                  identity_sha256=digest(args.directory / 'identity.json'))
    dump(args.directory / 'manifest.json', result)
    return dict(objects=len(recipes), payload_bytes=sum(v['bytes'] for r in recipes.values() for v in r['encoded'].values()))


def run_unit(args):
    check(__debug__, 'optimized Python is unsupported')
    containment()
    plan = (derive_plan({p.name: dict(code_sha256=source_identity(p), size=Path(json.loads((p / 'experiment.json').read_text())['baseline']).stat().st_size) for p in args.exports.iterdir()}) if args.unit == 'init' else json.loads((args.directory / 'plan.json').read_text()))
    validate_plan(plan)
    begin = time.monotonic()
    if args.unit == 'init':
        init(args, plan)
        result = dict(initialized=True)
    else:
        value = identity(args.directory)
        if args.unit == 'encode':
            result = encode(args, plan, value)
        elif args.unit == 'finalize':
            result = manifest(args, plan, value)
        else:
            current = json.loads((args.directory / 'manifest.json').read_text())
            check(current['identity_sha256'] == digest(args.directory / 'identity.json'), 'identity changed')
            check(current['root'] == plan['root'] and current['schedule'] == plan['schedule'] and
                  current['anchors'] == plan['anchors'] and set(current['recipes']) == set(plan['recipes']) and
                  current['support'] == value['support'], 'manifest topology changed')
            # Validation costs are identical in packed/raw comparison lanes.
            keys = plan['schedule'][args.part]
            before = {}
            for key in keys:
                check(source_identity(args.exports / key) == plan['recipes'][key]['code_sha256'], 'current source changed')
                before[key] = inputs(args.exports / key)
            for key, recipe in current['recipes'].items():
                check(recipe['base'] == plan['recipes'][key]['base'] and
                      recipe['sha256'] == value['records'][key]['sha256'], 'recipe identity changed')
            validated = time.monotonic() - begin
            result = replay(current, keys, args.directory, args.exports, args.scratch, raw=args.raw)
            check(before == {key: inputs(args.exports / key) for key in keys}, 'current inputs changed during replay')
            result.update(validated_s=validated, current_inputs=before, raw=args.raw)
    result.update(elapsed_s=time.monotonic() - begin, passed=True)
    dump(args.receipt, result)


def inventory(root):
    return {str(p.relative_to(root)): dict(bytes=p.stat().st_size, sha256=digest(p))
            for p in sorted(root.rglob('*')) if p.is_file()}


def tree_size(root):
    count = total = 0
    for path in root.rglob('*'):
        if path.is_file():
            count += 1
            total += path.stat().st_size
    return dict(files=count, logical_bytes=total)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=['prepare', 'run'])
    for name in ['directory', 'exports', 'output', 'cache']:
        parser.add_argument('--' + name, type=Path, required=True)
    parser.add_argument('--go', type=Path)
    parser.add_argument('--zstd', type=Path, default=Path('/usr/bin/zstd'))
    parser.add_argument('--library', type=Path, default=Path('/usr/lib/x86_64-linux-gnu/libzstd.so.1.4.8'))
    parser.add_argument('--scaffold-level', type=int, choices=[9, 12], default=9,
                        help='9 is selected; 12 is the matched preparation control')
    parser.add_argument('--raw', action='store_true', help='Matched native baseline comparison')
    parser.add_argument('--unit', choices=['init', 'encode', 'finalize', 'replay'], help=argparse.SUPPRESS)
    parser.add_argument('--part', type=int, default=0, help=argparse.SUPPRESS)
    parser.add_argument('--receipt', type=Path, help=argparse.SUPPRESS)
    args = parser.parse_args()
    for name in ['directory', 'exports', 'output', 'cache', 'go', 'zstd', 'library']:
        path = getattr(args, name)
        if path is not None:
            setattr(args, name, path.resolve())
    args.scratch = args.output / 'scratch'
    if args.unit:
        run_unit(args)
        return 0
    if args.mode == 'prepare' and args.go is None:
        parser.error('preparation requires --go')
    args.output.mkdir(parents=True, mode=0o700, exist_ok=False)
    args.scratch.mkdir(mode=0o700)
    if args.mode == 'prepare':
        args.directory.mkdir(parents=True, mode=0o700, exist_ok=False)
    check(args.directory.stat().st_mode & 0o777 == 0o700, 'private representation directory required')
    cache_before = tree_size(args.cache)
    begin = time.monotonic()
    rows = []

    def unit(kind, part=0):
        label = f'{kind}-{part:03d}'
        prefix = args.output / label
        child = [sys.executable, '-B', str(Path(__file__).resolve()), *sys.argv[1:],
                 '--unit', kind, '--part', str(part), '--receipt', str(prefix) + '.result.json']
        invocation = [sys.executable, '-B', str(HERE / 'run.py'), '--output', str(prefix),
                      '--cache', str(args.cache), '--timeout', '30', '--memory-high-mib', '700', '--', *child]
        with Path(str(prefix) + '.runner.log').open('w') as log:
            code = subprocess.run(invocation, stdout=log, stderr=subprocess.STDOUT).returncode
        report = json.loads(Path(str(prefix) + '.json').read_text())
        row = dict(label=label, exit=code, report=report)
        check(code == 0 and report.get('tree_removed'), 'contained unit failed: ' + str(prefix))
        return row

    def dispatch(kind, count):
        # Keep only two units submitted; on failure do not dispatch untouched work.
        with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
            todo = iter(range(count))
            pending = {pool.submit(unit, kind, next(todo)) for _ in range(min(2, count))}
            while pending:
                done, pending = concurrent.futures.wait(pending, return_when=concurrent.futures.FIRST_COMPLETED)
                completed = [future.result() for future in done]
                rows.extend(completed)
                dump(args.output / 'units.json', rows)
                for _ in completed:
                    part = next(todo, None)
                    if part is not None:
                        pending.add(pool.submit(unit, kind, part))

    if args.mode == 'prepare':
        rows.append(unit('init'))
        dispatch('encode', (len(json.loads((args.directory / 'plan.json').read_text())['recipes']) + 1) // 2)
        rows.append(unit('finalize'))
    else:
        dispatch('replay', len(json.loads((args.directory / 'plan.json').read_text())['schedule']))
    elapsed = time.monotonic() - begin
    check(json.loads((args.directory / 'identity.json').read_text())['harness'] == harness_inputs(),
          'harness source changed during dispatch')
    dump(args.output / 'units.json', rows)
    # Runtime inputs are mandatory; original native files are optional in packed
    # replay. Count any retained originals and their records without making them
    # a hidden prerequisite of the compressed lane.
    value = json.loads((args.directory / 'identity.json').read_text())
    external_paths = set(value['toolchain'])
    original_paths = {Path(p) for p in value['baseline'].values()}
    original_paths |= {p.parent / 'record.json' for p in original_paths}
    external_paths |= {str(p) for p in original_paths if p.is_file()}
    external = {path: dict(bytes=Path(path).stat().st_size, sha256=digest(Path(path)))
                for path in sorted(external_paths)}
    representation = inventory(args.directory)
    storage = dict(representation=representation, external=external,
                   exports=inventory(args.exports), receipts_before_closing=inventory(args.output),
                   harness={p: dict(bytes=(REPO / p).stat().st_size, sha256=h) for p, h in value['harness'].items()},
                   compiler_cache_before=cache_before, compiler_cache_after=tree_size(args.cache))
    storage['category_bytes'] = {name: sum(v['bytes'] for v in storage[name].values())
                                for name in ['representation', 'external', 'exports', 'receipts_before_closing', 'harness']}
    summary = dict(passed=True, mode=args.mode, raw=args.raw,
        partial_suite=True, family='current exported artifacts',
        units=len(rows), workers=2, dispatch_elapsed_s=elapsed,
        accounting_elapsed_s=time.monotonic() - begin - elapsed,
        representation_bytes=storage['category_bytes']['representation'])
    # Closing receipts account for their own byte lengths without a recursive
    # self-hash. All other retained files have both hashes and logical sizes.
    for _ in range(8):
        dump(args.output / 'summary.json', summary)
        dump(args.output / 'storage.json', storage)
        closing = {name: (args.output / name).stat().st_size for name in ['summary.json', 'storage.json']}
        if storage.get('closing_receipt_bytes') == closing:
            break
        storage['closing_receipt_bytes'] = closing
    else:
        raise RuntimeError('closing storage size did not converge')
    print(json.dumps(dict(passed=True, mode=args.mode, units=len(rows), elapsed_s=elapsed)))
    return 0


if __name__ == '__main__':
    sys.exit(main())
