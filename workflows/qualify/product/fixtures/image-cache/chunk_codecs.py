"""Stable compression codec IDs for qualification snapshot chunks."""
from dataclasses import dataclass
from pathlib import Path
from typing import Callable
import gzip
import subprocess
import zlib


@dataclass(frozen=True)
class ChunkCodec:
    encode: Callable[[bytes], bytes]
    decode: Callable[[Path, int], bytes]


def identity_encode(data):
    return data


def identity_decode(source, length):
    with source.open("rb") as stream:
        return stream.read(length + 1)


def gzip_encode(data):
    return gzip.compress(data, compresslevel=3, mtime=0)


def gzip_decode(source, length):
    decoder = zlib.decompressobj(wbits=31)
    decoded = decoder.decompress(source.read_bytes(), length + 1)
    if not decoder.eof or decoder.unconsumed_tail or decoder.unused_data:
        raise ValueError("Invalid or oversized gzip chunk")
    return decoded


def zstd_encode(data):
    result = subprocess.run(["zstd", "-q", "-3", "-c"], input=data,
                            stdout=subprocess.PIPE, check=True, timeout=30)
    return result.stdout


def zstd_decode(source, length):
    command = ["timeout", "--kill-after=5", "30", "zstd", "-q", "-d", "-c", "--", str(source)]
    with subprocess.Popen(command, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE) as decoder:
        decoded = decoder.stdout.read(length + 1)
        if len(decoded) > length:
            decoder.kill()
            raise ValueError("Oversized Zstandard chunk")
        if decoder.wait(timeout=5) != 0:
            raise ValueError("Invalid Zstandard chunk")
    return decoded


CODECS = {
    "identity": ChunkCodec(identity_encode, identity_decode),
    "gzip": ChunkCodec(gzip_encode, gzip_decode),
    "zstd": ChunkCodec(zstd_encode, zstd_decode),
}
