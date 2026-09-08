#!/usr/bin/env python3
"""Normal retained-artifact preparation and sealed descriptor transport (Linux).

The Go caller owns source/toolchain/settings identity and current oracles. This
layer owns only exact byte representation. Every workload runs inside run.py.
"""
import argparse
import array
import concurrent.futures
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import socket
import subprocess
import sys
import tempfile
import threading
import time

from shared_runtime_probe import containment, digest, dump, seal
from transcript_runtime import Decoder, Service, check, SEALS
from transcript_suite import command, derive_plan

HERE = Path(__file__).resolve().parent
VERSION = 'native-scaffold9-token12-v1'


def snapshot(cache):
    records = {}
    for path in sorted(cache.glob('*/record.json')):
        key = path.parent.name
        if not re.fullmatch('[0-9a-f]{64}', key):
            continue
        record = json.loads(path.read_text())
        check(record['Key'] == key and record['Version'] == 'pipelang-native-validation-v2', 'invalid native record')
        binary = path.parent / 'program.test'
        if binary.is_file():
            records[key] = dict(size=binary.stat().st_size, sha256=record['BinarySHA256'])
    return derive_plan(records) if records else None


def initialize(directory, go, cache):
    containment()
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    check(not directory.is_symlink() and directory.stat().st_mode & 0o777 == 0o700, 'private native store required')
    sources = {p.name: digest(p) for p in sorted((HERE / 'transcript').glob('*.go'))}
    sources.update({p.name: digest(p) for p in [HERE / 'native_artifacts.py', HERE / 'transcript_runtime.py', HERE / 'transcript_suite.py', HERE / 'shared_runtime_probe.py']})
    support = dict(zstd=str(Path(shutil.which('zstd')).resolve()), library='/usr/lib/x86_64-linux-gnu/libzstd.so.1.4.8')
    hashes = {name: digest(Path(path)) for name, path in support.items()}
    namespace = hashlib.sha256(json.dumps([VERSION, sources, hashes, digest(go)], sort_keys=True).encode()).hexdigest()
    root = directory / namespace
    root.mkdir(mode=0o700, exist_ok=True)
    with (root / '.init.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        if not (root / 'support.json').exists():
            command([go, 'build', '-p=1', '-o', root / 'splice', *sorted((HERE / 'transcript').glob('*.go'))])
            command([root / 'splice', 'selftest'])
            shutil.copy2(support['library'], root / 'libzstd.so')
            shutil.copy2(support['zstd'], root / 'zstd')
            dump(root / 'support.json', {name: digest(root / name) for name in ['splice', 'libzstd.so', 'zstd']})
    return dict(root=str(root), support=json.loads((root / 'support.json').read_text()), plan=snapshot(cache), sources=sources)


class Store:
    def __init__(self, cache, config):
        self.cache, self.config = cache, config
        self.root = Path(config['root'])
        for name, expected in config['support'].items():
            check(digest(self.root / name) == expected, 'native support changed')
        self.decoder = Decoder(self.root / 'libzstd.so', config['support']['libzstd.so'])
        try:
            self.service = Service(self.root / 'splice', config['support']['splice'])
        except BaseException:
            self.decoder.close()
            raise
        self.events = []

    def record(self, key):
        check(bool(re.fullmatch('[0-9a-f]{64}', key)), 'invalid artifact key')
        path = self.cache / key / 'record.json'
        check(not path.is_symlink() and not path.parent.is_symlink(), 'linked native record')
        record = json.loads(path.read_text())
        check(record['Key'] == key and record['Version'] == 'pipelang-native-validation-v2', 'native identity changed')
        return record['BinarySHA256']

    def prepare(self, key, ancestors=()):
        check(key not in ancestors and len(ancestors) <= 2, 'cyclic native recipe')
        expected = self.record(key)
        target = self.root / key
        recipe_path = target / 'recipe.json'
        if recipe_path.exists():
            recipe = json.loads(recipe_path.read_text())
            check(recipe['key'] == key and recipe['sha256'] == expected, 'native recipe identity changed')
            return recipe
        # References are prepared before taking the leaf lock, so two concurrent
        # units cannot deadlock while publishing the same deterministic DAG.
        plan = self.config['plan']
        base = plan['recipes'][key]['base'] if plan and key in plan['recipes'] else None
        reference = self.prepare(base, (*ancestors, key)) if base else None
        if reference and reference.get('unsupported'):
            base, reference = None, None
        with (self.root / ('.' + key + '.lock')).open('a') as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            if recipe_path.exists():
                return self.prepare(key, ancestors)
            start = time.monotonic()
            binary = self.cache / key / 'program.test'
            check(not binary.is_symlink(), 'linked native binary')
            if not 0 < binary.stat().st_size <= 16 << 20:
                target.mkdir(mode=0o700)
                recipe = dict(key=key, sha256=expected, unsupported=True, reason='native size outside codec bounds')
                dump(recipe_path, recipe)
                return recipe
            fd = seal(binary, expected)
            try:
                with tempfile.TemporaryDirectory(prefix='.prepare-', dir=self.root) as temporary:
                    work = Path(temporary)
                    extracted = work / key
                    result = subprocess.run([str(self.root / 'splice'), 'extract', f'/proc/self/fd/{fd}', str(extracted)],
                                            pass_fds=(fd,), capture_output=True, timeout=25)
                    if result.returncode:
                        # The codec admits a subset of debug-bearing ELF. Any
                        # unsupported form remains an ordinary native executable.
                        recipe = dict(key=key, sha256=expected, unsupported=True, reason='unsupported ELF/DEFLATE form')
                    else:
                        recipe = json.loads((extracted / 'splice.json').read_text())
                        recipe.update(base=base, encoded={})
                        reference_dir = None
                        if reference:
                            reference_dir = work / 'reference'
                            reference_dir.mkdir()
                            reference_fd, buffers = self.reconstruct(base, reference=True)
                            os.close(reference_fd)
                            for filename, data in zip(['scaffold', 'selected.tokens'], buffers):
                                (reference_dir / filename).write_bytes(data)
                        for kind, filename, level in [('scaffold', 'scaffold', 9), ('tokens', 'selected.tokens', 12)]:
                            patch = ['--patch-from=' + str(reference_dir / filename)] if reference_dir else []
                            data = command([self.root / 'zstd', '-q', '--single-thread', '--long=24', '-' + str(level), '-c', *patch, extracted / filename])
                            dictionary = ['-D', reference_dir / filename] if reference_dir else []
                            check(command([self.root / 'zstd', '-q', '-d', '-c', *dictionary], input=data) == (extracted / filename).read_bytes(), 'native preparation roundtrip failed')
                            (work / (kind + '.zst')).write_bytes(data)
                            recipe['encoded'][kind] = dict(path=key + '/' + kind + '.zst', bytes=len(data), sha256=hashlib.sha256(data).hexdigest())
                    stage = work / 'publish'
                    stage.mkdir(mode=0o700)
                    for path in work.glob('*.zst'):
                        shutil.copy2(path, stage / path.name)
                    dump(stage / 'recipe.json', recipe)
                    os.rename(stage, target)
            finally:
                os.close(fd)
            self.events.append(dict(key=key, prepared=True, unsupported=recipe.get('unsupported', False), seconds=time.monotonic() - start))
            return recipe

    def reconstruct(self, key, reference=False, ancestors=()):
        check(key not in ancestors and len(ancestors) <= 2, 'cyclic native recipe')
        recipe = self.prepare(key, ancestors)
        check(not recipe.get('unsupported'), 'unsupported reference')
        dictionaries = []
        try:
            if recipe['base']:
                fd, buffers = self.reconstruct(recipe['base'], reference=True, ancestors=(*ancestors, key))
                os.close(fd)
                for data in buffers:
                    dictionaries.append(self.decoder.dictionary(data))
            return self.decoder.splice(recipe, tuple(dictionaries) or None, self.service, self.root, reference=reference)
        finally:
            # Synchronous readers complete before dictionary release, including
            # failure paths. Each of the two stores owns its dictionaries.
            for dictionary in dictionaries:
                self.decoder.release(dictionary)

    def acquire(self, key, expected):
        check(self.record(key) == expected, 'requested native digest changed')
        recipe = self.prepare(key)
        if recipe.get('unsupported'):
            return None
        start = time.monotonic()
        fd, _ = self.reconstruct(key)
        self.events.append(dict(key=key, prepared=False, seconds=time.monotonic() - start))
        return fd

    def close(self):
        self.service.close()
        self.decoder.close()


def run(cache, config, receipt, argv):
    containment()
    for name, expected in config['sources'].items():
        path = HERE / 'transcript' / name if name.endswith('.go') else HERE / name
        check(digest(path) == expected, 'native preparation source changed; initialize again')
    stores = []
    import queue
    available = queue.Queue()
    with tempfile.TemporaryDirectory(prefix='pipelang-native-') as temporary:
        path = str(Path(temporary) / 'socket')
        listener = socket.socket(socket.AF_UNIX, socket.SOCK_SEQPACKET)
        listener.bind(path)
        listener.listen(8)
        listener.settimeout(.1)
        stopped = threading.Event()
        try:
            for _ in range(2):
                store = Store(cache, config)
                stores.append(store)
                available.put(store)
            def handle(connection):
                store = available.get()
                fd = None
                try:
                    connection.settimeout(25)
                    request = json.loads(connection.recv(4096))
                    # Nested cache probes deliberately use private cache roots.
                    if request['cache'] != str(cache):
                        connection.send(b'fallback')
                        return
                    fd = store.acquire(request['key'], request['sha256'])
                    if fd is None:
                        connection.send(b'fallback')
                    else:
                        connection.sendmsg([b'ok'], [(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', [fd]))])
                except Exception as error:
                    try:
                        connection.send(('error: ' + str(error)).encode()[:4096])
                    except OSError:
                        pass
                finally:
                    if fd is not None:
                        os.close(fd)
                    connection.close()
                    available.put(store)
            def accept():
                with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
                    while not stopped.is_set():
                        try:
                            connection, _ = listener.accept()
                        except socket.timeout:
                            continue
                        pool.submit(handle, connection)
            server = threading.Thread(target=accept)
            server.start()
            try:
                result = subprocess.run(argv, env=dict(os.environ, PIPELANG_NATIVE_SOCKET=path), timeout=25)
                return result.returncode
            finally:
                stopped.set()
                server.join(timeout=26)
                check(not server.is_alive(), 'native server deadline')
        finally:
            listener.close()
            dump(receipt, [event for store in stores for event in store.events])
            for store in stores:
                store.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=['init', 'run'])
    parser.add_argument('--cache', required=True, type=Path)
    parser.add_argument('--config', required=True, type=Path)
    parser.add_argument('--directory', type=Path)
    parser.add_argument('--go', type=Path)
    parser.add_argument('--receipt', type=Path)
    parser.add_argument('command', nargs=argparse.REMAINDER)
    args = parser.parse_args()
    if args.mode == 'init':
        dump(args.config, initialize(args.directory, args.go, args.cache))
        return 0
    return run(args.cache, json.loads(args.config.read_text()), args.receipt, args.command[1:] if args.command[:1] == ['--'] else args.command)


if __name__ == '__main__':
    sys.exit(main())
