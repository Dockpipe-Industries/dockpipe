#!/usr/bin/env python3
"""Reproduce the vendored terminal assets without npm installation scripts."""

import argparse
import base64
import hashlib
import io
import json
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import urllib.request


ROOT = Path(__file__).resolve().parent
MAX_ARCHIVE_BYTES = 32 * 1024 * 1024


def package_archive(metadata, cache):
    archive_path = cache / (metadata["package"].replace("/", "-") + "-" + metadata["version"] + ".tgz")
    if archive_path.exists():
        if archive_path.stat().st_size > MAX_ARCHIVE_BYTES:
            raise ValueError("Cached package archive exceeds the download limit")
        data = archive_path.read_bytes()
    else:
        with urllib.request.urlopen(metadata["url"], timeout=60) as response:
            data = response.read(MAX_ARCHIVE_BYTES + 1)
        if len(data) > MAX_ARCHIVE_BYTES:
            raise ValueError("Package archive exceeds the download limit")
    actual_integrity = "sha512-" + base64.b64encode(hashlib.sha512(data).digest()).decode("ascii")
    if actual_integrity != metadata["integrity"]:
        raise ValueError("Archive integrity mismatch for " + metadata["package"])
    if not archive_path.exists():
        archive_path.write_bytes(data)
    return tarfile.open(fileobj=io.BytesIO(data), mode="r:gz")


def member_bytes(archive, name):
    member = archive.getmember("package/" + name)
    if not member.isfile() or member.size > MAX_ARCHIVE_BYTES:
        raise ValueError("Unexpected package asset: " + name)
    with archive.extractfile(member) as source:
        return source.read()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cache", type=Path, required=True, help="Directory for integrity-checked npm archives")
    parser.add_argument("--write", action="store_true", help="Replace assets; otherwise verify existing bytes")
    arguments = parser.parse_args()
    arguments.cache.mkdir(parents=True, exist_ok=True)
    node = shutil.which("node")
    if not node:
        raise RuntimeError("Node.js is required to run the pinned WebAssembly transpiler")
    packages = json.loads((ROOT / "provenance.json").read_text())
    compatibility = json.loads((ROOT / "compatibility.json").read_text())
    assets = {}
    with package_archive(packages[0], arguments.cache) as archive:
        upstream = member_bytes(archive, "lib/xterm.js")
        if hashlib.sha256(upstream).hexdigest() != compatibility["upstream_sha256"]:
            raise ValueError("Unexpected upstream xterm JavaScript")
        assets["xterm.css"] = member_bytes(archive, "css/xterm.css")
        assets["LICENSE"] = member_bytes(archive, "LICENSE")
    with package_archive(packages[1], arguments.cache) as archive:
        assets["addon-fit.js"] = member_bytes(archive, "lib/addon-fit.js")
        assets["LICENSE-addon-fit"] = member_bytes(archive, "LICENSE")
    with tempfile.TemporaryDirectory(prefix="dockpipe-xterm-") as temporary:
        working = Path(temporary)
        with package_archive(compatibility["tool"], arguments.cache) as archive:
            for name in ("bin/esbuild", "lib/main.js", "wasm_exec_node.js", "wasm_exec.js", "esbuild.wasm", "package.json"):
                destination = working / name
                destination.parent.mkdir(parents=True, exist_ok=True)
                destination.write_bytes(member_bytes(archive, name))
        (working / "xterm.upstream.js").write_bytes(upstream)
        subprocess.run(
            [node, "bin/esbuild", "xterm.upstream.js", *compatibility["arguments"], "--outfile=xterm.js"],
            cwd=working,
            check=True,
            timeout=120,
        )
        assets["xterm.js"] = (working / "xterm.js").read_bytes()
    if hashlib.sha256(assets["xterm.js"]).hexdigest() != compatibility["output_sha256"]:
        raise ValueError("Transformed xterm JavaScript does not match the reviewed output hash")
    for name, data in assets.items():
        destination = ROOT / name
        if arguments.write:
            destination.write_bytes(data)
        elif destination.read_bytes() != data:
            raise ValueError("Vendored asset differs from its reproducible output: " + name)
    print("Verified terminal assets and Chrome 90 compatibility transformation")


if __name__ == "__main__":
    main()
