#!/usr/bin/env python3
"""Validate complete stores and prepare the public release download catalog."""
import argparse
import hashlib
import json
import re
import shutil
from pathlib import Path

PLATFORMS = ("linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64")
FLATPAK_PLATFORM = "linux-amd64-flatpak-org.kde.Platform-6.10"


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


def stage_core(directory, destination):
    """Stage only the verified core archive into a fresh installer-owned store."""
    manifest = json.loads((directory / "packages-store-manifest.json").read_text())
    core = manifest["packages"]["core"]
    filename = core["tarball"]
    if not re.fullmatch(r"dockpipe-core-[A-Za-z0-9_.-]+\.tar\.gz", filename):
        raise ValueError(f"Invalid core filename: {filename}")
    archive = directory / filename
    if archive.is_symlink() or digest(archive) != core["sha256"]:
        raise ValueError(f"Core checksum mismatch: {archive}")
    # Refuse reused payloads: optional packages must never leak from an older build.
    destination.mkdir(parents=True, exist_ok=False)
    core_directory = destination / "packages" / "core"
    core_directory.mkdir(parents=True)
    shutil.copyfile(archive, core_directory / filename)


def prepare(directory, version, candidate="", source_sha=""):
    if candidate and (not re.fullmatch(re.escape(version) + r"-staging\.[0-9]+\.[0-9]+\.[0-9a-f]{12}", candidate)
                      or not re.fullmatch(r"[0-9a-f]{40}", source_sha)
                      or not candidate.endswith(source_sha[:12])):
        raise ValueError("Invalid staging candidate provenance")
    stores = {}
    downloads = {}
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
            desktop = f"dockpipe-desktop_{version}_{arch}.deb"
            required.append(desktop)
        elif platform.startswith("darwin"):
            desktop = f"dockpipe-desktop_{version}_{platform.replace('-', '_')}.dmg"
            required += [desktop, desktop.removesuffix(".dmg") + ".zip"]
        else:
            desktop = f"dockpipe_{version}_windows_amd64.msi"
            if not (directory / desktop).is_file():
                desktop = None
        downloads[platform] = {"cli": required[0], "desktop": desktop}
        for name in required:
            if not (directory / name).is_file():
                raise ValueError(f"Missing release artifact: {name}")
    flatpak_store = directory / "stores" / FLATPAK_PLATFORM
    flatpak_bundle = f"dockpipe-desktop_{version}_linux_amd64.flatpak"
    if flatpak_store.exists() or (directory / flatpak_bundle).exists():
        if not (directory / flatpak_bundle).is_file():
            raise ValueError("Flatpak store requires its tested desktop bundle")
        manifest = json.loads((flatpak_store / "packages-store-manifest.json").read_text())
        if manifest.get("platform") != FLATPAK_PLATFORM:
            raise ValueError("Flatpak store has an incorrect runtime target")
        stores[FLATPAK_PLATFORM] = {
            "manifest": f"stores/{FLATPAK_PLATFORM}/packages-store-manifest.json",
            "count": verify_store(flatpak_store),
        }
        downloads[FLATPAK_PLATFORM] = {"cli": flatpak_bundle, "desktop": flatpak_bundle}
    catalog = {"schema": 1, "version": version, "stores": stores, "downloads": downloads}
    if source_sha:
        catalog["source_sha"] = source_sha
    if candidate:
        catalog.update(channel="staging", candidate=candidate)
    (directory / "release-manifest.json").write_text(json.dumps(catalog, indent=2) + "\n")
    files = sorted(path for path in directory.iterdir() if path.is_file() and path.name != "SHA256SUMS.txt")
    (directory / "SHA256SUMS.txt").write_text("".join(f"{digest(path)}  {path.name}\n" for path in files))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("verify-store", "stage-core", "prepare"))
    parser.add_argument("directory", type=Path)
    parser.add_argument("--version")
    parser.add_argument("--destination", type=Path)
    parser.add_argument("--candidate", default="")
    parser.add_argument("--source-sha", default="")
    args = parser.parse_args()
    if args.command == "verify-store":
        print(f"Verified {verify_store(args.directory)} packages in {args.directory}")
    elif args.command == "stage-core":
        if args.destination is None:
            parser.error("stage-core requires --destination")
        stage_core(args.directory, args.destination)
    else:
        if not args.version:
            parser.error("prepare requires --version")
        prepare(args.directory, args.version, args.candidate, args.source_sha)
