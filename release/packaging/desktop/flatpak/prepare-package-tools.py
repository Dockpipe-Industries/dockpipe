#!/usr/bin/env python3
"""Prepare pinned optional tools outside the offline SDK/package build.

Each result is an assets/tooling/bin tree. Packages copy only the tools declared
by their own recipe; none of these trees belongs in the base desktop bundle.
"""
import argparse
from contextlib import contextmanager
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import platform
import re
import shutil
import signal
import stat
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.request
import zipfile


MAX_TOOL_ARCHIVE_BYTES = 128 * 1024 * 1024
DOWNLOAD_TIMEOUT_SECONDS = 120
INPUT_TIMEOUT_SECONDS = 180


def checksum(path):
    digest = hashlib.sha256()
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(descriptor, "rb") as source:
        if not stat.S_ISREG(os.fstat(source.fileno()).st_mode):
            raise ValueError(f"Invalid optional tool archive: {path}")
        total_bytes = 0
        while chunk := source.read(1024 * 1024):
            total_bytes += len(chunk)
            if total_bytes > MAX_TOOL_ARCHIVE_BYTES:
                raise ValueError(f"Optional tool archive exceeds capacity: {path}")
            digest.update(chunk)
    return digest.hexdigest(), total_bytes


def require_unlinked_cache(cache, path):
    for directory in (cache, *cache.parents):
        if directory.is_symlink():
            raise ValueError(f"Linked optional tool cache parent: {directory}")
    if not cache.is_dir():
        raise ValueError(f"Optional tool cache is not a directory: {cache}")
    if path.is_symlink():
        raise ValueError(f"Linked optional tool cache file: {path}")


def report_mismatch(name, observed, size_bytes, expected):
    print(
        f"Checksum mismatch for {name}: observed sha256={observed}, bytes={size_bytes}; "
        f"expected sha256={expected}",
        file=sys.stderr,
    )


@contextmanager
def input_deadline():
    previous_handler = signal.getsignal(signal.SIGALRM)
    previous_delay, previous_interval = signal.getitimer(signal.ITIMER_REAL)
    started = time.monotonic()

    def expired(_signum, _frame):
        raise TimeoutError("Optional tool input exceeded its wall time limit")

    signal.signal(signal.SIGALRM, expired)
    delay = min(INPUT_TIMEOUT_SECONDS, previous_delay) if previous_delay else INPUT_TIMEOUT_SECONDS
    signal.setitimer(signal.ITIMER_REAL, delay)
    try:
        yield
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        signal.signal(signal.SIGALRM, previous_handler)
        if previous_delay:
            remaining = max(previous_delay - (time.monotonic() - started), 0.000001)
            signal.setitimer(signal.ITIMER_REAL, remaining, previous_interval)


def verified_input(name, source, cache):
    with input_deadline():
        return fetch_verified_input(name, source, cache)


def fetch_verified_input(name, source, cache):
    if not isinstance(name, str) or not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.+-]{0,254}", name):
        raise ValueError("Invalid optional tool filename")
    expected = source["sha256"]
    if not isinstance(expected, str) or not re.fullmatch(r"[0-9a-f]{64}", expected):
        raise ValueError("Invalid optional tool checksum")
    cache = Path(os.path.abspath(cache))
    path = cache / name
    require_unlinked_cache(cache, path)
    if path.exists():
        observed, size_bytes = checksum(path)
        if observed == expected:
            return path
        report_mismatch(name, observed, size_bytes, expected)

    for _attempt in range(2):
        path.unlink(missing_ok=True)
        temporary = None
        try:
            with tempfile.NamedTemporaryFile(
                mode="wb", prefix=f".{name}-", suffix=".download", dir=cache, delete=False,
            ) as output:
                temporary = Path(output.name)
                deadline = time.monotonic() + DOWNLOAD_TIMEOUT_SECONDS
                with urllib.request.urlopen(source["url"], timeout=60) as response:
                    total_bytes = 0
                    while chunk := response.read1(1024 * 1024):
                        if time.monotonic() > deadline:
                            raise ValueError(f"Optional tool download exceeded time limit: {name}")
                        total_bytes += len(chunk)
                        if total_bytes > MAX_TOOL_ARCHIVE_BYTES:
                            raise ValueError(f"Optional tool archive exceeds capacity: {name}")
                        output.write(chunk)
                if time.monotonic() > deadline:
                    raise ValueError(f"Optional tool download exceeded time limit: {name}")
            observed, size_bytes = checksum(temporary)
            if observed == expected:
                temporary.replace(path)
                return path
            report_mismatch(name, observed, size_bytes, expected)
        finally:
            if temporary is not None:
                temporary.unlink(missing_ok=True)

    raise ValueError(f"Checksum mismatch after a fresh retry: {path}")


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
