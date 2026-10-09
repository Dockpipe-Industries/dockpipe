#!/usr/bin/env python3
"""Prepare pinned optional tools outside the offline SDK/package build.

Each result is an assets/tooling/bin tree. Packages copy only the tools declared
by their own recipe; none of these trees belongs in the base desktop bundle.
"""
import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import platform
import shutil
import subprocess
import tarfile
import urllib.request
import zipfile


def checksum(path):
    digest = hashlib.sha256()
    with path.open("rb") as source:
        while chunk := source.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def verified_input(name, source, cache):
    path = cache / name
    if not path.exists():
        temporary = cache / (name + ".download")
        try:
            with urllib.request.urlopen(source["url"], timeout=60) as response:
                with temporary.open("wb") as output:
                    shutil.copyfileobj(response, output)
            temporary.replace(path)
        finally:
            temporary.unlink(missing_ok=True)
    if checksum(path) != source["sha256"]:
        raise ValueError(f"Checksum mismatch: {path}")
    return path


def safe_relative(name):
    path = PurePosixPath(name)
    if path.is_absolute() or ".." in path.parts or "\\" in name:
        raise ValueError(f"Unsafe dependency archive path: {name}")
    return path


def unpack_regular_files(path, destination, archive_format):
    # Do not extract links or special files. Node's public npm/npx links are
    # replaced by explicit wrappers below, and AWS uses its real dist binaries.
    if archive_format == "tar":
        with tarfile.open(path) as archive:
            for member in archive:
                relative = safe_relative(member.name)
                if not member.isfile():
                    continue
                output = destination / relative
                output.parent.mkdir(parents=True, exist_ok=True)
                with archive.extractfile(member) as source, output.open("wb") as target:
                    shutil.copyfileobj(source, target)
                output.chmod(0o755 if member.mode & 0o111 else 0o644)
    else:
        with zipfile.ZipFile(path) as archive:
            for member in archive.infolist():
                relative = safe_relative(member.filename)
                mode = member.external_attr >> 16
                if member.is_dir() or mode & 0o170000 == 0o120000:
                    continue
                output = destination / relative
                output.parent.mkdir(parents=True, exist_ok=True)
                with archive.open(member) as source, output.open("wb") as target:
                    shutil.copyfileobj(source, target)
                output.chmod(0o755 if mode & 0o111 else 0o644)


def wrapper(destination, command, body):
    path = destination / command
    path.write_text('#!/usr/bin/env bash\nset -euo pipefail\n'
                    'tools="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"\n'
                    + body + '\n')
    path.chmod(0o755)


def prepare(name, source, archive, output):
    destination = output / name
    destination.mkdir()
    vendor = destination / ".deps" / name
    vendor.mkdir(parents=True)
    if source["format"] == "binary":
        shutil.copyfile(archive, destination / name)
        (destination / name).chmod(0o755)
    else:
        unpack_regular_files(archive, vendor, source["format"])
        if name == "node":
            extracted = vendor / f'node-v{source["version"]}-linux-x64'
            extracted.rename(vendor / "runtime")
            wrapper(destination, "node", 'exec "$tools/.deps/node/runtime/bin/node" "$@"')
            for command, script in (("npm", "npm-cli.js"), ("npx", "npx-cli.js")):
                wrapper(destination, command,
                        'export PATH="$tools:$PATH"\n'
                        f'exec "$tools/.deps/node/runtime/bin/node" "$tools/.deps/node/runtime/lib/node_modules/npm/bin/{script}" "$@"')
        elif name == "go":
            wrapper(destination, "go", 'export GOROOT="$tools/.deps/go/go"\n'
                    'export GOTOOLCHAIN=local\nexec "$GOROOT/bin/go" "$@"')
        elif name == "aws":
            wrapper(destination, "aws", 'exec "$tools/.deps/aws/aws/dist/aws" "$@"')
        else:
            candidates = list(vendor.rglob(name))
            candidates = [path for path in candidates if path.is_file()]
            if len(candidates) != 1:
                raise ValueError(f"Expected one {name} binary, got {candidates}")
            relative = candidates[0].relative_to(destination)
            candidates[0].chmod(0o755)
            wrapper(destination, name, f'exec "$tools/{relative}" "$@"')
    (vendor / "source.json").write_text(json.dumps(source, indent=2) + "\n")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("output", type=Path)
    parser.add_argument("--cache", type=Path, required=True)
    parser.add_argument("--tools", nargs="+")
    parser.add_argument("--app", type=Path, help="Flatpak SDK build directory; also prepares Azure CLI")
    args = parser.parse_args()
    if platform.machine() != "x86_64":
        raise SystemExit("Optional dependency locks are qualified for Linux amd64 only")
    root = Path(__file__).resolve().parent
    lock = json.loads((root / "package-tools.lock.json").read_text())
    if args.output.exists():
        raise SystemExit("Use a fresh optional tool output directory")
    args.output.mkdir(parents=True)
    args.cache.mkdir(parents=True, exist_ok=True)
    for name in args.tools or lock["tools"]:
        source = lock["tools"][name]
        archive = verified_input(name, source, args.cache)
        prepare(name, source, archive, args.output)
    if "node" in (args.tools or lock["tools"]):
        node = args.output / "node/.deps/node/runtime"
        wrangler = args.output / "wrangler"
        vendor = wrangler / ".deps/wrangler"
        vendor.mkdir(parents=True)
        for filename in ("package.json", "package-lock.json"):
            shutil.copyfile(root / "wrangler" / filename, vendor / filename)
        subprocess.run([str(node / "bin/node"), str(node / "lib/node_modules/npm/bin/npm-cli.js"),
                        "ci", "--prefix", str(vendor), "--cache", str(args.cache / "npm"),
                        "--ignore-scripts", "--no-audit", "--no-fund"], check=True)
        # The compiler excludes links. No npm .bin links are needed at runtime.
        for path in vendor.rglob("*"):
            if path.is_symlink():
                path.unlink()
        wrapper(wrangler, "wrangler", 'exec "$tools/node" "$tools/.deps/wrangler/node_modules/wrangler/bin/wrangler.js" "$@"')
    if args.app:
        wheels = args.cache / "azure-wheels"
        wheels.mkdir(exist_ok=True)
        azure_lock = json.loads((root / "azure-wheels.lock.json").read_text())
        for source in azure_lock["wheels"]:
            verified_input(source["file"], source, wheels)
        azure = args.output / "azure"
        vendor = azure / ".deps/azure"
        vendor.mkdir(parents=True)
        subprocess.run(["flatpak", "build", "--unshare=network", "--nofilesystem=host",
                        "--nofilesystem=home", f"--filesystem={wheels.resolve()}:ro",
                        f"--filesystem={azure.resolve()}", str(args.app), "python3", "-m", "pip",
                        "install", "--no-index", "--no-compile", "--no-deps",
                        "--target", str(vendor.resolve()),
                        *[str((wheels / entry["file"]).resolve()) for entry in azure_lock["wheels"]]], check=True)
        wrapper(azure, "az", 'export PYTHONPATH="$tools/.deps/azure"\n'
                'export PYTHONDONTWRITEBYTECODE=1\nexec /usr/bin/python3 -m azure.cli "$@"')


if __name__ == "__main__":
    main()
