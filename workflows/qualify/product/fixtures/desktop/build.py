#!/usr/bin/env python3
"""Export existing desktop helpers without starting a display, GUI or guest."""
import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import stat
import tarfile


OWNER = Path(__file__).resolve().parents[2]
VERSION = "0.8.0"
MODULE_NAMES = ("desktop.py", "ui.py")
MAXIMUM_MODULE_BYTES = 256 << 10


def read_source(path):
    original = path.lstat()
    if not stat.S_ISREG(original.st_mode):
        raise ValueError("Desktop fixture source must be a regular file")
    flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0) | getattr(os, "O_NONBLOCK", 0)
    descriptor = os.open(path, flags)
    with os.fdopen(descriptor, "rb") as stream:
        opened = os.fstat(stream.fileno())
        if (not stat.S_ISREG(opened.st_mode)
                or (original.st_dev, original.st_ino) != (opened.st_dev, opened.st_ino)):
            raise ValueError("Desktop fixture source changed while opening")
        contents = stream.read(MAXIMUM_MODULE_BYTES + 1)
    if len(contents) > MAXIMUM_MODULE_BYTES:
        raise ValueError("Desktop fixture source exceeds the module byte limit")
    return contents


def encode_json(value):
    return (json.dumps(value, sort_keys=True, indent=2) + "\n").encode("utf-8")


def archive_bytes(modules):
    manifest = {
        "schema": "dockpipe.product-desktop-fixture/v1",
        "version": VERSION,
        "platform": "linux-x11",
        "files": [
            {"path": name, "bytes": len(modules[name]),
             "sha256": hashlib.sha256(modules[name]).hexdigest()}
            for name in MODULE_NAMES
        ],
    }
    entries = {"manifest.json": encode_json(manifest), **modules}
    output = io.BytesIO()
    with gzip.GzipFile(fileobj=output, mode="wb", filename="", mtime=0) as compressed:
        with tarfile.open(fileobj=compressed, mode="w", format=tarfile.USTAR_FORMAT) as archive:
            for name in ("manifest.json", *MODULE_NAMES):
                information = tarfile.TarInfo(name)
                information.mode = 0o444
                information.size = len(entries[name])
                archive.addfile(information, io.BytesIO(entries[name]))
    return output.getvalue(), manifest


def produce(source_directory, output):
    modules = {name: read_source(source_directory / name) for name in MODULE_NAMES}
    artifact, manifest = archive_bytes(modules)
    for name, contents in modules.items():
        if read_source(source_directory / name) != contents:
            raise ValueError("Desktop fixture source changed during export")

    output.mkdir(mode=0o700, parents=True, exist_ok=False)
    target = output / ("dockpipe-product-desktop-fixture-" + VERSION + ".tar.gz")
    with target.open("xb") as stream:
        stream.write(artifact)
    target.chmod(0o600)
    receipt = {
        "schema": "dockpipe.product-desktop-fixture-build/v1",
        "artifact": str(target.resolve()),
        "artifact_sha256": hashlib.sha256(artifact).hexdigest(),
        "manifest": manifest,
        "desktop_started": False,
        "guest_started": False,
        "product_qualified": False,
    }
    (output / "result.json").write_bytes(encode_json(receipt))
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    arguments = parser.parse_args()
    if not arguments.output.is_absolute():
        parser.error("--output must select a fresh absolute directory")
    receipt = produce(OWNER / "platforms/linux", arguments.output)
    print(json.dumps(receipt, sort_keys=True))


if __name__ == "__main__":
    main()
