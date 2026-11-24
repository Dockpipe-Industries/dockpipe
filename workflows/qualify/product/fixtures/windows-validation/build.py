#!/usr/bin/env python3
"""Export one public pure validator without importing it or starting resources."""
import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import stat
import tarfile

VERSION = "0.1.0"
MODULE = "native_arguments.py"
MAXIMUM_MODULE_BYTES = 64 << 10


def read_source(path):
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        before = os.fstat(descriptor)
        if not stat.S_ISREG(before.st_mode) or not 0 < before.st_size <= MAXIMUM_MODULE_BYTES:
            raise ValueError("Validation source must be a bounded regular file")
        contents = os.read(descriptor, MAXIMUM_MODULE_BYTES + 1)
        after = os.fstat(descriptor)
        current = path.lstat()

        def identity(value):
            return value.st_dev, value.st_ino, value.st_size, value.st_mtime_ns, value.st_ctime_ns

        if (
            len(contents) != before.st_size
            or identity(before) != identity(after)
            or identity(before) != identity(current)
        ):
            raise ValueError("Validation source changed during export")
        return contents
    finally:
        os.close(descriptor)


def encoded(value):
    return (json.dumps(value, sort_keys=True, indent=2) + "\n").encode()


def produce(source, output):
    source = Path(source)
    output = Path(output)
    contents = read_source(source)
    manifest = {
        "schema": "dockpipe.product-windows-validation/v1",
        "version": VERSION,
        "api": "qemu-forwarding/v1",
        "files": [
            {
                "path": MODULE,
                "bytes": len(contents),
                "sha256": hashlib.sha256(contents).hexdigest(),
            },
        ],
    }
    buffer = io.BytesIO()
    with gzip.GzipFile(fileobj=buffer, mode="wb", filename="", mtime=0) as compressed:
        with tarfile.open(fileobj=compressed, mode="w", format=tarfile.USTAR_FORMAT) as archive:
            for name, data in [("manifest.json", encoded(manifest)), (MODULE, contents)]:
                member = tarfile.TarInfo(name)
                member.size = len(data)
                member.mode = 0o444
                archive.addfile(member, io.BytesIO(data))
    if read_source(source) != contents:
        raise ValueError("Validation source changed during export")

    output.mkdir(mode=0o700, parents=True, exist_ok=False)
    artifact = output / ("dockpipe-product-windows-validation-" + VERSION + ".tar.gz")
    artifact.write_bytes(buffer.getvalue())
    artifact.chmod(0o600)
    report = {
        "schema": "dockpipe.product-windows-validation-build/v1",
        "artifact": str(artifact.resolve()),
        "artifact_sha256": hashlib.sha256(buffer.getvalue()).hexdigest(),
        "manifest": manifest,
        "code_imported": False,
        "live_effects_started": False,
        "product_qualified": False,
    }
    (output / "result.json").write_bytes(encoded(report))
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    selected = parser.parse_args()
    if not selected.output.is_absolute():
        parser.error("Select a fresh absolute output")
    owner = Path(__file__).resolve().parents[2]
    print(json.dumps(produce(owner / "platforms/windows" / MODULE, selected.output), sort_keys=True))


if __name__ == "__main__":
    main()
