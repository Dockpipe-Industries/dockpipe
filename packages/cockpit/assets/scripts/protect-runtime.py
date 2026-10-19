#!/usr/bin/env python3
"""Remove private symbols from Cockpit runtime files and bind release proof."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess


def tools(repo):
    path = repo / "packages/native-sdk/assets/scripts/elf_artifacts.py"
    spec = importlib.util.spec_from_file_location("cockpit_elf", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def protect(assets, sdk, qt, private_debug, repo):
    assets = assets.resolve()
    private_debug = private_debug.resolve()
    if private_debug.is_relative_to(assets) or assets.is_relative_to(private_debug):
        raise ValueError("Private diagnostics and public assets must be separate trees")
    elf = tools(repo)
    for name in ("libdockpipe-cockpit.so", "cockpit-reader"):
        binary = assets / "ui" / name
        runtime = binary.with_name(name + ".public")
        elf.split(binary, runtime, private_debug / (name + ".debug"), elf.exports(binary))
        runtime.replace(binary)
    # Imported libraries/helpers may contain upstream symbols. Preserve only
    # executable runtime sections; our own diagnostics were retained above.
    for binary in sorted(assets.rglob("*")):
        if not binary.is_file() or binary.is_symlink():
            continue
        with binary.open("rb") as stream:
            is_elf = stream.read(4) == b"\x7fELF"
        if not is_elf:
            continue
        versions = subprocess.check_output(["readelf", "--version-info", str(binary)], text=True, timeout=60)
        glibc = {tuple(map(int, value.split("."))) for value in re.findall(r"GLIBC_([0-9.]+)", versions)}
        if glibc and max(glibc) > (2, 35):
            raise ValueError(f"{binary.name} requires glibc newer than Ubuntu 22.04")
        subprocess.run(["strip", "--strip-unneeded", str(binary)], check=True, timeout=60)
        relative_lib = os.path.relpath(assets / "lib", binary.parent)
        subprocess.run(["patchelf", "--force-rpath", "--set-rpath", f"$ORIGIN/{relative_lib}", str(binary)],
                       check=True, timeout=60)
        if any(name.startswith((".debug", ".zdebug")) or name in {".symtab", ".gnu_debugdata", ".gdb_index"}
               for name in elf.sections(binary)):
            raise ValueError(f"Private symbol sections remain in {binary.name}")
    notices = assets / "licenses"
    shutil.copytree(sdk / "share/forgepipe", notices / "forgepipe")
    shutil.copy2(repo / "LICENSE", notices / "Dockpipe-LICENSE")
    inventories = sorted((qt / "sbom").glob("*.spdx.json"))
    if not inventories:
        raise ValueError("Matching Qt license inventories are required")
    qt_notices = notices / "qt"
    qt_notices.mkdir()
    for inventory in inventories:
        shutil.copy2(inventory, qt_notices / inventory.name)
    for name in ("Qt-LGPL-3.txt", "Qt-GPL-3.txt"):
        shutil.copy2(repo / "release/packaging/desktop/licenses" / name, qt_notices / name)


def verify_runtime(assets, build, qt):
    assets = assets.resolve()
    build = build.resolve()
    qt = qt.resolve()
    environment = {"PATH": "/usr/bin:/bin", "LD_LIBRARY_PATH": f"{assets / 'lib'}:{build}"}
    for binary in sorted(assets.rglob("*")):
        if not binary.is_file() or binary.is_symlink():
            continue
        with binary.open("rb") as stream:
            if stream.read(4) != b"\x7fELF":
                continue
        dependencies = subprocess.check_output(["ldd", str(binary)], text=True, env=environment, timeout=60)
        if "not found" in dependencies:
            raise ValueError(f"Bundled Cockpit runtime has unresolved dependencies: {binary.name}")
        for name, path in re.findall(r"^\s*(\S+) => (/\S+) ", dependencies, re.MULTILINE):
            path = Path(path).resolve()
            if ((name.startswith(("libQt6", "libforgepipe-engine")) and not path.is_relative_to(assets))
                    or path.is_relative_to(qt)):
                raise ValueError(f"Bundled Cockpit dependency escapes its runtime closure: {name}")


def write_receipt(store, sdk):
    manifest = json.loads((store / "packages-store-manifest.json").read_text())
    entries = manifest["packages"]
    if (manifest["platform"] != "linux-amd64" or entries.get("core")
            or entries.get("workflows") or len(entries.get("resolvers", [])) != 1
            or entries["resolvers"][0]["name"] != "cockpit"):
        raise ValueError("Release helper must produce exactly resolver/cockpit")
    entry = entries["resolvers"][0]
    lock = json.loads((sdk / "consumer-lock.json").read_text())
    source_sha = os.environ.get("DOCKPIPE_RELEASE_SOURCE_SHA", "")
    if not re.fullmatch(r"[0-9a-f]{40}", source_sha):
        raise ValueError("Cockpit qualification needs an exact Dockpipe source SHA")
    receipt = {
        "schema": 1, "platform": "linux-amd64", "qt_version": "6.10.3",
        "sdk_sha256": lock["artifact"]["sha256"], "sdk_source_sha": lock["source_sha"],
        "source_sha": source_sha,
        "resolver": entry,
        "checks": ["desktop-ctest", "x11-spatial-gpu", "x11-package-rendering", "private-symbol-split",
                   "bundled-runtime-closure", "bundled-x11-package-rendering"],
    }
    archive = store / entry["tarball"]
    digest = hashlib.sha256()
    with archive.open("rb") as stream:
        for block in iter(lambda: stream.read(1 << 20), b""):
            digest.update(block)
    if archive.stat().st_size != entry["size_bytes"] or digest.hexdigest() != entry["sha256"]:
        raise ValueError("Cockpit compiled archive differs from its store manifest")
    (store / "cockpit-qualification.json").write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--assets", type=Path)
    parser.add_argument("--sdk", type=Path, required=True)
    parser.add_argument("--qt", type=Path)
    parser.add_argument("--private-debug", type=Path)
    parser.add_argument("--repo-root", type=Path, required=True)
    parser.add_argument("--write-receipt", type=Path)
    parser.add_argument("--verify-runtime", type=Path)
    parser.add_argument("--build", type=Path)
    args = parser.parse_args()
    if args.verify_runtime:
        if not all((args.build, args.qt)):
            parser.error("Runtime closure verification needs --build and --qt")
        verify_runtime(args.verify_runtime, args.build, args.qt)
    elif args.write_receipt:
        write_receipt(args.write_receipt, args.sdk)
    else:
        if not all((args.assets, args.qt, args.private_debug)):
            parser.error("Runtime protection needs --assets, --qt and --private-debug")
        protect(args.assets, args.sdk, args.qt, args.private_debug, args.repo_root)
