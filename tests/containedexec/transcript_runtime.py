"""Exact, sealed conformance reconstruction. Linux; call only inside run.py.

Two independent decoders/services share read-only dictionaries. A dictionary owner
must wait for all readers before releasing it. No outcomes or fixtures are cached.
"""
import ctypes
import fcntl
import hashlib
import json
import mmap
import os
from pathlib import Path
import select
import subprocess
import time

from shared_runtime_probe import seal

LIMIT = 16 << 20
SEALS = fcntl.F_SEAL_SEAL | fcntl.F_SEAL_WRITE | fcntl.F_SEAL_GROW | fcntl.F_SEAL_SHRINK


def check(ok, message='invalid transcript'):
    if not ok:
        raise ValueError(message)


def hcheck(data, expected):
    check(hashlib.sha256(data).hexdigest() == expected, 'artifact digest mismatch')


def verifyseal(fd, length, expected):
    with mmap.mmap(fd, length, prot=mmap.PROT_READ) as mapping:
        hcheck(mapping, expected)
    fcntl.fcntl(fd, fcntl.F_ADD_SEALS, SEALS)
    check(fcntl.fcntl(fd, fcntl.F_GET_SEALS) & SEALS == SEALS, 'sealing failed')


class Service:
    def __init__(self, helper, expected, timeout=25):
        self.timeout = timeout
        self.fd = seal(helper, expected)
        self.p = None
        try:
            self.p = subprocess.Popen(
                [f'/proc/self/fd/{self.fd}', 'serve'], stdin=subprocess.PIPE,
                stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                pass_fds=(self.fd,), bufsize=0)
            os.set_blocking(self.p.stdin.fileno(), False)
            os.set_blocking(self.p.stdout.fileno(), False)
            check(self.send({}) == {'ready': True}, 'splice handshake failed')
        except BaseException:
            self.close()
            raise

    def send(self, request):
        # Both pipe directions are nonblocking; the deadline also covers a hung
        # helper that never reads its input. The outer cgroup deadline is separate.
        deadline = time.monotonic() + self.timeout
        data = (json.dumps(request) + '\n').encode()
        check(len(data) <= 1 << 20, 'oversized request')
        result = bytearray()
        while data:
            left = deadline - time.monotonic()
            if left <= 0 or not select.select([], [self.p.stdin], [], left)[1]:
                raise TimeoutError('splice write deadline')
            try:
                count = os.write(self.p.stdin.fileno(), data)
            except BlockingIOError:
                continue
            check(count > 0, 'splice write EOF')
            data = data[count:]
        while not result.endswith(b'\n'):
            left = deadline - time.monotonic()
            if left <= 0 or not select.select([self.p.stdout], [], [], left)[0]:
                raise TimeoutError('splice response deadline')
            try:
                data = os.read(self.p.stdout.fileno(), 4096)
            except BlockingIOError:
                continue
            check(bool(data), 'splice EOF')
            result.extend(data)
            check(len(result) <= 1 << 20, 'oversized response')
        return json.loads(result)

    def fill(self, tokens, output, recipe):
        value = self.send(dict(Input=f'/proc/{os.getpid()}/fd/{tokens}',
                               Output=f'/proc/{os.getpid()}/fd/{output}',
                               Length=recipe['size'], Spans=recipe['spans']))
        check(not value['error'], 'splice rejected')

    def close(self):
        if self.p is not None:
            self.p.stdin.close()
            try:
                self.p.wait(timeout=min(self.timeout, 1))
            except subprocess.TimeoutExpired:
                self.p.kill()
                self.p.wait(timeout=1)
            self.p.stdout.close()
            self.p = None
        if self.fd is not None:
            os.close(self.fd)
            self.fd = None


class Decoder:
    def __init__(self, library, expected):
        self.fd = seal(library, expected)
        self.dicts = []
        try:
            self.lib = ctypes.CDLL(f'/proc/self/fd/{self.fd}')
            ptr, size = ctypes.c_void_p, ctypes.c_size_t
            for name, args, result in [
                ('ZSTD_createDCtx', [], ptr), ('ZSTD_freeDCtx', [ptr], size),
                ('ZSTD_createDDict', [ptr, size], ptr), ('ZSTD_freeDDict', [ptr], size),
                ('ZSTD_decompress_usingDDict', [ptr, ptr, size, ptr, size, ptr], size),
                ('ZSTD_isError', [size], ctypes.c_uint),
            ]:
                fn = getattr(self.lib, name)
                fn.argtypes, fn.restype = args, result
        except BaseException:
            os.close(self.fd)
            raise

    def dictionary(self, data):
        check(0 < len(data) <= LIMIT, 'oversized dictionary')
        dictionary = self.lib.ZSTD_createDDict(data, len(data))
        check(bool(dictionary), 'dictionary allocation failed')
        self.dicts.append(dictionary)
        return dictionary

    def release(self, dictionary):
        self.dicts.remove(dictionary)
        self.lib.ZSTD_freeDDict(dictionary)

    def raw(self, data, length, dictionary=None):
        check(0 < length <= LIMIT and 0 < len(data) <= LIMIT, 'oversized frame')
        fd = os.memfd_create('pipelang-transcript', os.MFD_ALLOW_SEALING)
        mapping = view = context = None
        try:
            os.ftruncate(fd, length)
            mapping = mmap.mmap(fd, length, flags=mmap.MAP_SHARED,
                                prot=mmap.PROT_READ | mmap.PROT_WRITE)
            view = (ctypes.c_char * length).from_buffer(mapping)
            context = self.lib.ZSTD_createDCtx()
            check(bool(context), 'context allocation failed')
            count = self.lib.ZSTD_decompress_usingDDict(
                context, view, length, data, len(data), dictionary)
            check(not self.lib.ZSTD_isError(count) and count == length, 'decode failed')
            return fd
        except BaseException:
            os.close(fd)
            raise
        finally:
            if context:
                self.lib.ZSTD_freeDCtx(context)
            view = None
            if mapping is not None:
                mapping.close()

    def splice(self, recipe, base, service, directory, reference=False, blobs=None):
        scaffold, tokens = base if base else (None, None)
        data = {}
        for kind in ['scaffold', 'tokens']:
            meta = recipe['encoded'][kind]
            path = Path(meta['path'])
            check(not path.is_absolute() and '..' not in path.parts, 'invalid payload path')
            check(meta['bytes'] <= LIMIT, 'oversized payload')
            if blobs is None:
                with (directory / path).open('rb') as file:
                    value = file.read(LIMIT + 1)
            else:
                value = blobs[kind]
            check(len(value) == meta['bytes'], 'payload size changed')
            hcheck(value, meta['sha256'])
            data[kind] = value
        output = tokenfd = None
        try:
            output = self.raw(data['scaffold'], recipe['size'], scaffold)
            # Validate the pre-splice scaffold as well as the final ELF.
            with mmap.mmap(output, recipe['size'], prot=mmap.PROT_READ) as mapping:
                hcheck(mapping, recipe['scaffold_sha256'])
                raw = mapping[:] if reference else None
            tokenfd = self.raw(data['tokens'], recipe['token_bytes'], tokens)
            verifyseal(tokenfd, recipe['token_bytes'], recipe['tokens_sha256'])
            with mmap.mmap(tokenfd, recipe['token_bytes'], prot=mmap.PROT_READ) as mapping:
                token_data = mapping[:] if reference else None
            service.fill(tokenfd, output, recipe)
            verifyseal(output, recipe['size'], recipe['sha256'])
            return output, (raw, token_data) if reference else None
        except BaseException:
            if output is not None:
                os.close(output)
            raise
        finally:
            if tokenfd is not None:
                os.close(tokenfd)

    def close(self):
        for dictionary in self.dicts[:]:
            self.release(dictionary)
        if self.fd is not None:
            os.close(self.fd)
            self.fd = None
