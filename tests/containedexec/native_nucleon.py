"""Optional complete-executable representation using a local binary SDK."""
import ctypes
import fcntl
import hashlib
import json
import mmap
import os
from pathlib import Path
import re
import shutil
import tempfile
import time

from nucleon_sdk import SDK
from shared_runtime_probe import containment, digest, dump, seal
from transcript_runtime import check, verifyseal

HERE = Path(__file__).resolve().parent
VERSION = 'native-nucleon-whole-file-abi1-v1'
LIMIT = 16 << 20


def initialize(directory, library):
    containment()
    check(library is not None and library.is_absolute() and library.is_file(), 'absolute Nucleon SDK library required')
    library = library.resolve()
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    check(not directory.is_symlink() and directory.stat().st_mode & 0o777 == 0o700, 'private native store required')
    sources = {name: digest(HERE / name) for name in
               ['native_artifacts.py', 'native_nucleon.py', 'nucleon_sdk.py', 'shared_runtime_probe.py', 'transcript_runtime.py']}
    sdk_hash = digest(library)
    namespace = hashlib.sha256(json.dumps([VERSION, sources, sdk_hash], sort_keys=True).encode()).hexdigest()
    root = directory / namespace
    root.mkdir(mode=0o700, exist_ok=True)
    with (root / '.init.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        if not (root / 'support.json').exists():
            with tempfile.TemporaryDirectory(prefix='.sdk-', dir=root) as temp:
                target = Path(temp) / 'libnucleon.so'
                shutil.copy2(library, target)
                check(digest(target) == sdk_hash, 'SDK changed during initialization')
                sdk = SDK(target, sdk_hash)
                try:
                    version = sdk.lib.nucleon_version().decode()
                finally:
                    sdk.close()
                os.replace(target, root / 'libnucleon.so')
                dump(root / 'support.json', {'libnucleon.so': sdk_hash})
                dump(root / 'sdk-version.json', {'version': version, 'abi': 1})
    return dict(codec='nucleon', root=str(root), support=json.loads((root / 'support.json').read_text()),
                sources=sources, plan=None, sdk_source=str(library), sdk_sha256=sdk_hash)


class Store:
    def __init__(self, cache, config):
        self.cache, self.config, self.root = cache, config, Path(config['root'])
        check(digest(Path(config['sdk_source'])) == config['sdk_sha256'], 'SDK source changed; initialize again')
        self.sdk = SDK(self.root / 'libnucleon.so', config['support']['libnucleon.so'])
        self.events = []

    def record(self, key):
        check(bool(re.fullmatch('[0-9a-f]{64}', key)), 'invalid artifact key')
        path = self.cache / key / 'record.json'
        check(not path.is_symlink() and not path.parent.is_symlink(), 'linked native record')
        record = json.loads(path.read_text())
        check(record['Key'] == key and record['Version'] == 'pipelang-native-validation-v2', 'native identity changed')
        return record['BinarySHA256']

    def prepare(self, key):
        expected = self.record(key)
        target = self.root / key
        recipe_path = target / 'recipe.json'
        if recipe_path.exists():
            check(not target.is_symlink() and not recipe_path.is_symlink(), 'linked Nucleon recipe')
            recipe = json.loads(recipe_path.read_text())
            check(recipe['key'] == key and recipe['sha256'] == expected, 'native recipe identity changed')
            check(recipe['codec'] == VERSION, 'Nucleon recipe codec changed')
            return recipe
        with (self.root / ('.' + key + '.lock')).open('a') as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            if recipe_path.exists():
                return self.prepare(key)
            start = time.monotonic()
            binary = self.cache / key / 'program.test'
            check(not binary.is_symlink(), 'linked native binary')
            size = binary.stat().st_size
            recipe = dict(key=key, sha256=expected, size=size, codec=VERSION)
            if not 0 < size <= LIMIT:
                recipe.update(unsupported=True, reason='native size outside codec bounds')
            else:
                fd = seal(binary, expected)
                try:
                    with mmap.mmap(fd, size, prot=mmap.PROT_READ) as source:
                        packed = self.sdk.encode(source[:])
                finally:
                    os.close(fd)
                recipe.update(packed_bytes=len(packed), packed_sha256=hashlib.sha256(packed).hexdigest())
                proof_fd = self.decode(packed, recipe)
                os.close(proof_fd)
            with tempfile.TemporaryDirectory(prefix='.prepare-', dir=self.root) as temp:
                stage = Path(temp) / key
                stage.mkdir(mode=0o700)
                if not recipe.get('unsupported'):
                    (stage / 'program.nuc').write_bytes(packed)
                dump(stage / 'recipe.json', recipe)
                os.rename(stage, target)
            self.events.append(dict(key=key, codec='nucleon', prepared=True, unsupported=recipe.get('unsupported', False),
                                    bytes=size, stored_bytes=recipe.get('packed_bytes', 0), seconds=time.monotonic()-start))
            return recipe

    def decode(self, packed, recipe):
        check(0 < recipe['size'] <= LIMIT, 'Nucleon recipe size')
        fd = os.memfd_create('pipelang-nucleon', os.MFD_ALLOW_SEALING)
        try:
            os.ftruncate(fd, recipe['size'])
            with mmap.mmap(fd, recipe['size']) as output:
                view = (ctypes.c_char * recipe['size']).from_buffer(output)
                try:
                    self.sdk.decode_into(packed, view, recipe['size'])
                finally:
                    del view
            verifyseal(fd, recipe['size'], recipe['sha256'])
            return fd
        except BaseException:
            os.close(fd)
            raise

    def acquire(self, key, expected):
        check(self.record(key) == expected, 'requested native digest changed')
        recipe = self.prepare(key)
        if recipe.get('unsupported'):
            return None
        start = time.monotonic()
        path = self.root / key / 'program.nuc'
        check(not path.is_symlink() and 24 <= path.stat().st_size <= SDK.LIMIT + 16408, 'Nucleon packed extent')
        packed = path.read_bytes()
        check(len(packed) == recipe['packed_bytes'] and hashlib.sha256(packed).hexdigest() == recipe['packed_sha256'], 'Nucleon packed digest changed')
        fd = self.decode(packed, recipe)
        self.events.append(dict(key=key, codec='nucleon', prepared=False, bytes=recipe['size'],
                                stored_bytes=len(packed), seconds=time.monotonic()-start))
        return fd

    def close(self):
        self.sdk.close()
