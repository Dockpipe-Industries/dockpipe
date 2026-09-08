"""Bounded two-producer/four-consumer pipeline for current conformance exports."""
import concurrent.futures as futures
import os
from pathlib import Path
import queue
import re
import shutil
import subprocess
import tempfile
import time

from shared_runtime_probe import seal
from transcript_runtime import Decoder, Service, check


def execute(item, exports, scratch):
    key, fd, reconstruction = item
    begin = time.monotonic()
    try:
        names = []
        source = exports / key
        cases = sorted(p for p in source.glob('case*') if p.is_dir())
        check(bool(cases), 'no current cases')
        for case in cases:
            check(re.fullmatch(r'case\d{4}', case.name), 'invalid case name')
            with tempfile.TemporaryDirectory(prefix='native-case-', dir=scratch) as work:
                for path in case.iterdir():
                    shutil.copy2(path, Path(work) / ('generated_test.go' if path.name == 'checks.go' else path.name))
                shutil.copy2(source / 'go.mod', Path(work) / 'go.mod')
                test = 'TestCase' + case.name[4:]
                result = subprocess.run(
                    [f'/proc/self/fd/{fd}', '-test.run', '^' + test + '$',
                     '-test.count=1', '-test.timeout=25s', '-test.v'],
                    cwd=work, pass_fds=(fd,), capture_output=True, text=True, timeout=25)
                check(result.returncode == 0, result.stdout + result.stderr)
                got = re.findall(r'^=== RUN\s+(\S+)$', result.stdout, re.M)
                check(test in got and len(got) > 1 and 'PASS' in result.stdout, 'missing native tests')
                names.extend(got)
        return dict(key=key, cases=len(cases), native_tests=names,
                    reconstruct_s=reconstruction, worker_s=time.monotonic() - begin)
    finally:
        os.close(fd)


def replay(manifest, keys, directory, exports, scratch, raw=False):
    recipes = manifest['recipes']
    slots, available = [], queue.Queue()
    results, pending = [], set()
    maximum = 0
    rootdict = None
    try:
        for _ in range(2):
            decoder = Decoder(directory / 'libzstd.so', manifest['support']['libzstd.so'])
            try:
                service = Service(directory / 'splice', manifest['support']['splice'])
            except BaseException:
                decoder.close()
                raise
            slots.append((decoder, service))
            available.put((decoder, service))
        owner, service = slots[0]

        def prepare(key, base):
            fd, buffers = owner.splice(recipes[key], base, service, directory, reference=True)
            dictionaries = []
            try:
                for buffer in buffers:
                    dictionaries.append(owner.dictionary(buffer))
                return tuple(dictionaries)
            except BaseException:
                for dictionary in dictionaries:
                    owner.release(dictionary)
                raise
            finally:
                os.close(fd)

        def release(dictionaries):
            for dictionary in dictionaries:
                owner.release(dictionary)

        def reconstruct(key, base):
            decoder, service = available.get()
            start = time.monotonic()
            try:
                if raw:
                    fd = seal(Path(manifest['baseline'][key]), recipes[key]['sha256'])
                else:
                    fd, _ = decoder.splice(recipes[key], base, service, directory)
                return key, fd, time.monotonic() - start
            finally:
                available.put((decoder, service))

        if not raw:
            rootdict = prepare(manifest['root'], None)
        groups = {}
        for key in keys:
            groups.setdefault(recipes[key]['base'], []).append(key)
        with futures.ThreadPoolExecutor(max_workers=4) as consumers:
            with futures.ThreadPoolExecutor(max_workers=2) as producers:
                for base, group in sorted(groups.items(), key=lambda pair: pair[0] or ''):
                    active = None if raw or base is None else rootdict if base == manifest['root'] else prepare(base, rootdict)
                    building, todo, exhausted = set(), iter(group), False
                    try:
                        while not exhausted or building:
                            while not exhausted and len(pending) + len(building) < 8:
                                try:
                                    key = next(todo)
                                except StopIteration:
                                    exhausted = True
                                    break
                                building.add(producers.submit(reconstruct, key, active))
                                maximum = max(maximum, len(pending) + len(building))
                            if not building and exhausted:
                                break
                            done, _ = futures.wait(pending | building, return_when=futures.FIRST_COMPLETED)
                            for future in done:
                                if future in building:
                                    building.remove(future)
                                    item = future.result()
                                    try:
                                        pending.add(consumers.submit(execute, item, exports, scratch))
                                    except BaseException:
                                        os.close(item[1])
                                        raise
                                else:
                                    pending.remove(future)
                                    results.append(future.result())
                    finally:
                        # Wait even on errors: shared dictionaries must outlive every
                        # reader. Successful unsubmitted results still own an FD.
                        for future in building:
                            try:
                                item = future.result()
                            except BaseException:
                                continue
                            os.close(item[1])
                        if active is not None and active is not rootdict:
                            release(active)
            results.extend(future.result() for future in pending)
    finally:
        if rootdict is not None:
            release(rootdict)
        for decoder, service in slots:
            decoder.close()
            service.close()
    return dict(results=sorted(results, key=lambda row: row['key']), max_inflight=maximum,
                reconstruction_workers=2, native_workers=4)
