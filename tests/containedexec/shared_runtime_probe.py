#!/usr/bin/env python3
"""Bounded Go shared-library experiment; invoke build/run only through run.py.

Exports are current inputs from a complete v0.91 conformance invocation, never
cached outcomes. Go 1.25.13 forces DWARF omission with -linkshared; overriding
that fails in the linker. This is an unpromoted diagnostic prototype: retain the
original debug-bearing baseline binaries and include them in storage accounting.
Build serially. Replay uses sealed code, fresh processes and
fixture directories, and regenerating exports is required before retained reruns.
"""
import argparse
import concurrent.futures
import fcntl
import hashlib
import json
import os
import re
from pathlib import Path
import shutil
import subprocess
import tempfile
import time


def digest(p):
    h = hashlib.sha256()
    with p.open('rb') as f:
        for b in iter(lambda: f.read(1024 * 1024), b''):
            h.update(b)
    return h.hexdigest()


def dump(p, value):
    p.write_text(json.dumps(value, sort_keys=True, indent=2) + '\n')


def code_inputs(values):
    return {k: v for k, v in values.items() if k.endswith(".go") or k == "go.mod"}


def inputs(p, code_only=False):
    return {str(f.relative_to(p)): digest(f) for f in sorted(p.rglob('*'))
            if f.is_file() and f.name != 'experiment.json'
            and (not code_only or f.suffix == '.go' or f.name == 'go.mod')}


def containment():
    cg = Path('/sys/fs/cgroup') / Path('/proc/self/cgroup').read_text().strip().split('::', 1)[1].lstrip('/')
    assert {n: int((cg / n).read_text()) for n in ['memory.max', 'memory.swap.max', 'pids.max']} == {
        'memory.max': 1073741824, 'memory.swap.max': 0, 'pids.max': 128}
    return cg


def support(root):
    return {str(p): digest(p) for p in sorted((root / 'std-pkg').glob('*.so'))}


def validate_record(record, source, settings):
    if code_inputs(record["inputs"]) != inputs(source, code_only=True):
        raise ValueError("current source inputs changed")
    if record["settings_sha256"] != digest(settings):
        raise ValueError("build identity changed")


def validate_toolchain(identity):
    if not all(digest(Path(p)) == sha for p, sha in identity["toolchain"].items()):
        raise ValueError("toolchain changed")


def seal(p, expected):
    fd = os.memfd_create('pipelang-shared-experiment', os.MFD_ALLOW_SEALING)
    try:
        h = hashlib.sha256()
        with p.open('rb') as f:
            for b in iter(lambda: f.read(1024 * 1024), b''):
                h.update(b)
                view = memoryview(b)
                while view:
                    view = view[os.write(fd, view):]
        if h.hexdigest() != expected:
            raise ValueError('artifact digest mismatch: ' + str(p))
        seals = fcntl.F_SEAL_SEAL | fcntl.F_SEAL_SHRINK | fcntl.F_SEAL_GROW | fcntl.F_SEAL_WRITE
        fcntl.fcntl(fd, fcntl.F_ADD_SEALS, seals)
        assert fcntl.fcntl(fd, fcntl.F_GET_SEALS) & seals == seals
        return fd
    except BaseException:
        os.close(fd)
        raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=['build', 'run'])
    parser.add_argument('--root', required=True, type=Path)
    parser.add_argument('--exports', required=True, type=Path)
    parser.add_argument('--go', type=Path)
    parser.add_argument('--start', type=int, default=0)
    parser.add_argument('--stop', type=int, default=200)
    parser.add_argument('--lane', choices=['baseline', 'candidate'], default='candidate')
    parser.add_argument('--receipt', required=True, type=Path)
    args = parser.parse_args()
    if not __debug__:
        raise RuntimeError('optimized Python mode is unsupported')
    containment()
    root = args.root.resolve()
    exports = sorted(args.exports.iterdir())
    assert len(exports) == 200 and 0 <= args.start < args.stop <= 200
    assert not args.receipt.exists(), 'fresh receipt required'
    settings = root / 'settings.json'
    identity = json.loads(settings.read_text())
    assert support(root) == identity['shared_libraries'], 'support changed'
    # Check the complete pinned Go toolchain, C compiler/linker inputs and host
    # loader dependencies once per fresh unit, not just a version/path label.
    validate_toolchain(identity)
    # The evidence root retains a separate audit of the actual GCC helper
    # closure. This also invalidates on same-path helper replacements.
    validate_toolchain(json.loads((root / 'compiler-closure.json').read_text()))
    results = []
    started = time.monotonic()
    if args.mode == 'build':
        assert str(args.go) == identity['go']
        work = root / 'gopath/src/pipelang-generated-check'
        env = dict(os.environ, **identity['environment'])
        for source in exports[args.start:args.stop]:
            for p in work.iterdir():
                if p.name != 'oracle':
                    if p.is_dir(): shutil.rmtree(p)
                    else: p.unlink()
            assert digest(source / 'oracle/oracle.go') == digest(work / 'oracle/oracle.go')
            for p in source.iterdir():
                if p.name in ['oracle', 'experiment.json', 'go.mod']: continue
                if p.is_dir(): shutil.copytree(p, work / p.name)
                else: shutil.copy2(p, work / p.name)
            target = root / 'artifacts' / source.name
            target.mkdir(parents=True, mode=0o700, exist_ok=False)
            command = [str(args.go), 'test', '-c', '-p=1', '-linkshared', '-pkgdir=' + str(root / 'std-pkg'), '-o', str(target / 'program.test'), '.']
            begin = time.monotonic()
            result = subprocess.run(command, cwd=work, env=env, capture_output=True, timeout=30)
            (target / 'build.log').write_bytes(result.stdout + result.stderr)
            assert result.returncode == 0, str(target / 'build.log')
            assert support(root) == identity['shared_libraries'], 'build changed shared support'
            record = dict(inputs=inputs(source), settings_sha256=digest(settings), binary_sha256=digest(target / 'program.test'), command=command)
            dump(target / 'record.json', record)
            results.append(dict(key=source.name, elapsed_s=time.monotonic() - begin))
    else:
        shared_fds = []
        try:
            with tempfile.TemporaryDirectory(prefix='pipelang-shared-libs-') as libs:
                if args.lane == 'candidate':
                    for path, sha in identity['shared_libraries'].items():
                        fd = seal(Path(path), sha)
                        shared_fds.append(fd)
                        os.symlink('/proc/self/fd/' + str(fd), Path(libs) / Path(path).name)
                def execute(source):
                    record = json.loads((source / 'experiment.json').read_text())
                    target = root / 'artifacts' / source.name
                    meta = json.loads((target / 'record.json').read_text())
                    validate_record(meta, source, settings)
                    if args.lane == 'candidate':
                        executable, sha = target / 'program.test', meta['binary_sha256']
                    else:
                        executable, sha = Path(record['baseline']), record['sha256']
                    fd = seal(executable, sha)
                    begin = time.monotonic()
                    cases = sorted(p for p in source.glob('case*') if p.is_dir())
                    native_tests = []
                    try:
                        for case in cases:
                            with tempfile.TemporaryDirectory(prefix='pipelang-shared-case-') as cwd:
                                for p in case.iterdir():
                                    shutil.copy2(p, Path(cwd) / ('generated_test.go' if p.name == 'checks.go' else p.name))
                                shutil.copy2(source / 'go.mod', Path(cwd) / 'go.mod')
                                env = dict(os.environ, LD_LIBRARY_PATH=libs, LD_PRELOAD='', LD_AUDIT='')
                                result = subprocess.run(['/proc/self/fd/' + str(fd), '-test.run', '^Test' + case.name[0].upper() + case.name[1:] + '$', '-test.count=1', '-test.timeout=25s', '-test.v'], cwd=cwd, env=env, pass_fds=(fd, *shared_fds), capture_output=True, timeout=30)
                                assert result.returncode == 0, result.stdout.decode() + result.stderr.decode()
                                assert b'PASS' in result.stdout and ('=== RUN   Test' + case.name[0].upper() + case.name[1:]).encode() in result.stdout
                                native_tests.extend(re.findall(r'^=== RUN\s+(\S+)$', result.stdout.decode(), re.M))
                        return dict(key=source.name, cases=len(cases), native_tests=native_tests, elapsed_s=time.monotonic() - begin)
                    finally:
                        os.close(fd)
                with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
                    results = list(pool.map(execute, exports[args.start:args.stop]))
        finally:
            for fd in shared_fds: os.close(fd)
    dump(args.receipt, dict(mode=args.mode, lane=args.lane, start=args.start, stop=args.stop, elapsed_s=time.monotonic()-started, results=results))
    print(json.dumps(dict(mode=args.mode, lane=args.lane, bundles=len(results), elapsed_s=time.monotonic()-started)))


if __name__ == '__main__':
    main()
