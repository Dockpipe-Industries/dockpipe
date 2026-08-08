#!/usr/bin/python3
import hashlib
import json
import os
from pathlib import Path

if len(os.sys.argv) != 1:
    raise SystemExit("this purpose-specific generator accepts no arguments")

root = Path("/bundle")
recipe_sha256 = Path("/evidence/build-recipe.sha256").read_text(encoding="ascii").strip()

def digest(path: Path) -> str:
    value = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            value.update(chunk)
    return value.hexdigest()

def pin(relative: str) -> dict:
    path = root / relative
    return {"relative_path": relative, "sha256": digest(path), "mode": path.stat().st_mode & 0o777}

tools = [
    {
        "id": "qemu-img",
        **pin("bin/qemu-img"),
        "version": "qemu-img version 11.0.3",
    },
    {
        "id": "qemu-system-x86_64",
        **pin("bin/qemu-system-x86_64"),
        "version": "QEMU emulator version 11.0.3",
    },
]
tool_paths = {item["relative_path"] for item in tools}
runtime_files = [
    pin(path.relative_to(root).as_posix())
    for path in sorted(root.rglob("*"))
    if path.is_file() and path.relative_to(root).as_posix() not in tool_paths and path.name != "toolchain.json"
]
manifest = {
    "schema": "dockpipe.vm.toolchain.v1",
    "bundle_id": "dockpipe-vm-qemu-linux-amd64",
    "bundle_version": "11.0.3-linux-amd64.1",
    "os": "linux",
    "architecture": "amd64",
    "qemu_version": "11.0.3",
    "source": {
        "url": "https://download.qemu.org/qemu-11.0.3.tar.xz",
        "signature_url": "https://download.qemu.org/qemu-11.0.3.tar.xz.sig",
        "archive_sha256": "da5fcffc32762820568b828ed430a728864d34d50b6d2f30358597760cbb0523",
        "signer_fingerprint": "CEACC9E15534EBABB82D3FA03353C9CEF108B584",
    },
    "build_recipe_sha256": recipe_sha256,
    "tools": tools,
    "runtime_files": runtime_files,
}
(root / "toolchain.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
