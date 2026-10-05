#!/usr/bin/env python3
"""Validate complete stores and prepare the public release download catalog."""
import argparse
import hashlib
import json
import re
from pathlib import Path

PLATFORMS = ("linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64")


def digest(path):
    checksum = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            checksum.update(block)
    return checksum.hexdigest()


def verify_store(directory):
    manifest = json.loads((directory / "packages-store-manifest.json").read_text())
    packages = manifest["packages"]
    if not all(packages.get(kind) for kind in ("core", "workflows", "resolvers")):
        raise ValueError(f"Incomplete package store: {directory}")
    entries = [packages["core"], *packages["workflows"], *packages["resolvers"]]
    names = set()
    for entry in entries:
        name = entry["tarball"]
        if Path(name).name != name or name in names:
            raise ValueError(f"Invalid or duplicate store filename: {name}")
        names.add(name)
        path = directory / name
        if path.is_symlink() or digest(path) != entry["sha256"]:
            raise ValueError(f"Package checksum mismatch: {path}")
    return len(entries)


def prepare(directory, version, candidate="", source_sha=""):
    if candidate and (not re.fullmatch(re.escape(version) + r"-staging\.[0-9]+\.[0-9]+\.[0-9a-f]{12}", candidate)
                      or not re.fullmatch(r"[0-9a-f]{40}", source_sha)
                      or not candidate.endswith(source_sha[:12])):
        raise ValueError("Invalid staging candidate provenance")
    stores = {}
    for platform in PLATFORMS:
        store = directory / "stores" / platform
        stores[platform] = {
            "manifest": f"stores/{platform}/packages-store-manifest.json",
            "count": verify_store(store),
        }
        suffix = "zip" if platform.startswith("windows") else "tar.gz"
        required = [
            f"dockpipe_{version}_{platform.replace('-', '_')}.{suffix}",
            f"dockpipe-packages_{version}_{platform}.tar.gz",
        ]
        if platform.startswith("linux"):
            arch = platform.split("-")[1]
            required += [f"dockpipe_{version}_{arch}.deb"]
            required += [f"dockpipe_{version}_linux_{arch}.{ext}" for ext in ("rpm", "apk", "pkg.tar.zst")]
        for name in required:
            if not (directory / name).is_file():
                raise ValueError(f"Missing release artifact: {name}")
    catalog = {"schema": 1, "version": version, "stores": stores}
    if source_sha:
        catalog["source_sha"] = source_sha
    if candidate:
        catalog.update(channel="staging", candidate=candidate)
    (directory / "release-manifest.json").write_text(json.dumps(catalog, indent=2) + "\n")
    files = sorted(path for path in directory.iterdir() if path.is_file() and path.name != "SHA256SUMS.txt")
    (directory / "SHA256SUMS.txt").write_text("".join(f"{digest(path)}  {path.name}\n" for path in files))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("verify-store", "prepare"))
    parser.add_argument("directory", type=Path)
    parser.add_argument("--version")
    parser.add_argument("--candidate", default="")
    parser.add_argument("--source-sha", default="")
    args = parser.parse_args()
    if args.command == "verify-store":
        print(f"Verified {verify_store(args.directory)} packages in {args.directory}")
    else:
        if not args.version:
            parser.error("prepare requires --version")
        prepare(args.directory, args.version, args.candidate, args.source_sha)
