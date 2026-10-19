#!/usr/bin/env python3
"""Verify and stage the exact public ForgePipe SDK used by Cockpit native builds."""
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import platform
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.parse
import urllib.request


MAX_ARCHIVE_BYTES = 256 << 20
MAX_METADATA_BYTES = 64 << 10
MAX_EXPANDED_BYTES = 1 << 30
MAX_MEMBERS = 5000
REPOSITORY = "Dockpipe-Industries/forgepipe"
PUBLISHER = "Dockpipe-Industries/dockpipe"


def validate_pin(pin, maximum):
    if not isinstance(pin, dict) or set(pin) != {"url", "sha256", "size_bytes"}:
        raise ValueError("SDK delivery needs an exact URL, SHA-256 and byte size")
    url = urllib.parse.urlsplit(pin["url"])
    if (url.scheme != "https" or url.netloc != "github.com" or url.query or url.fragment
            or not url.path.startswith(f"/{PUBLISHER}/releases/download/")):
        raise ValueError("SDK delivery must identify a fixed Dockpipe GitHub release asset")
    if not re.fullmatch(r"[0-9a-f]{64}", pin["sha256"]):
        raise ValueError("SDK delivery needs an exact SHA-256")
    if type(pin["size_bytes"]) is not int or not 0 < pin["size_bytes"] <= maximum:
        raise ValueError("SDK delivery exceeds its byte capacity")


def load_lock(path):
    if path.stat().st_size > MAX_METADATA_BYTES:
        raise ValueError("SDK lock exceeds metadata capacity")
    lock = json.loads(path.read_text())
    if (lock.get("schema") != 1 or lock.get("version") != "0.1.0"
            or lock.get("platform") != "linux-amd64" or lock.get("qt_version") != "6.10.3"
            or lock.get("source_repository") != REPOSITORY
            or lock.get("public_redistribution") is not True):
        raise ValueError("Unsupported ForgePipe SDK or Qt ABI contract")
    if not isinstance(lock.get("source_sha"), str) or not re.fullmatch(r"[0-9a-f]{40}", lock["source_sha"]):
        raise ValueError("Audited public SDK source commit has not been pinned")
    validate_pin(lock.get("artifact"), MAX_ARCHIVE_BYTES)
    validate_pin(lock.get("metadata"), MAX_METADATA_BYTES)
    return lock


def fetch(pin, destination):
    digest = hashlib.sha256()
    count = 0
    deadline = time.monotonic() + 300
    request = urllib.request.Request(pin["url"], headers={"User-Agent": "Dockpipe-Cockpit-SDK"})
    with urllib.request.urlopen(request, timeout=60) as response, destination.open("xb") as output:
        if urllib.parse.urlsplit(response.url).scheme != "https":
            raise ValueError("SDK download redirected outside HTTPS")
        while block := response.read(min(1 << 20, pin["size_bytes"] - count + 1)):
            if time.monotonic() > deadline:
                raise TimeoutError("SDK download exceeded its five-minute deadline")
            count += len(block)
            if count > pin["size_bytes"]:
                raise ValueError("SDK download exceeds its pinned byte size")
            digest.update(block)
            output.write(block)
    if count != pin["size_bytes"] or digest.hexdigest() != pin["sha256"]:
        raise ValueError("SDK download differs from its pinned size or SHA-256")


def verify_attestation(path):
    # The public repository's release workflow is the independent publisher
    # authority. A digest in editable metadata alone cannot authorize an SDK.
    subprocess.run(["gh", "attestation", "verify", str(path), "--repo", PUBLISHER,
                    "--signer-workflow", f"{PUBLISHER}/.github/workflows/release.yml"],
                   check=True, timeout=120, stdout=sys.stderr, stderr=sys.stderr)


def verify_metadata(metadata, lock):
    for key in ("version", "platform", "qt_version", "source_repository", "source_sha", "public_redistribution"):
        if metadata.get(key) != lock[key]:
            raise ValueError(f"SDK release metadata differs from lock: {key}")
    archive = metadata.get("archive", {})
    if (archive.get("name") != f"ForgePipe-SDK-{lock['version']}-Linux.tar.gz"
            or archive.get("sha256") != lock["artifact"]["sha256"]
            or archive.get("size_bytes") != lock["artifact"]["size_bytes"]):
        raise ValueError("SDK release metadata does not bind the pinned archive")


def unpack(archive_path, destination, version):
    root_name = f"ForgePipe-SDK-{version}-Linux"
    seen = set()
    total_bytes = 0
    with tarfile.open(archive_path, "r:gz") as archive:
        for member in archive:
            name = member.name.rstrip("/") if member.isdir() else member.name
            relative = PurePosixPath(name)
            owned_root = name == root_name and member.isdir()
            if (name != relative.as_posix() or relative.is_absolute() or ".." in relative.parts
                    or "\\" in name or ":" in name
                    or not (owned_root or name.startswith(root_name + "/"))):
                raise ValueError("SDK archive contains an unsafe or unowned path")
            if name in seen or len(seen) >= MAX_MEMBERS:
                raise ValueError("SDK archive contains duplicate paths or too many members")
            seen.add(name)
            target = destination / name
            if member.isdir():
                target.mkdir(parents=True, exist_ok=True)
            elif member.isfile():
                total_bytes += member.size
                if member.size < 0 or total_bytes > MAX_EXPANDED_BYTES:
                    raise ValueError("SDK archive exceeds expanded capacity")
                target.parent.mkdir(parents=True, exist_ok=True)
                with archive.extractfile(member) as source, target.open("xb") as output:
                    shutil.copyfileobj(source, output, length=1 << 20)
                target.chmod(0o755 if member.mode & 0o111 else 0o644)
            else:
                raise ValueError("SDK archive contains an unsupported member type")
    sdk = destination / root_name
    for required in ("lib/cmake/ForgePipe/ForgePipeConfig.cmake", "share/forgepipe/LICENSE",
                     "share/forgepipe/NOTICE", "share/forgepipe/SOURCE-PROVENANCE.json"):
        if not (sdk / required).is_file():
            raise ValueError(f"SDK archive lacks required payload: {required}")
    return sdk


def prepare(lock, output):
    if platform.system() != "Linux" or platform.machine() != "x86_64":
        raise ValueError("ForgePipe SDK qualification supports Linux amd64 only")
    if output.exists():
        raise ValueError("Use a fresh SDK preparation output directory")
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="forgepipe-", dir=output.parent) as temporary:
        stage = Path(temporary)
        archive = stage / "sdk.tar.gz"
        metadata = stage / "sdk-release.json"
        fetch(lock["metadata"], metadata)
        verify_attestation(metadata)
        verify_metadata(json.loads(metadata.read_text()), lock)
        fetch(lock["artifact"], archive)
        verify_attestation(archive)
        sdk = unpack(archive, stage / "unpacked", lock["version"])
        (sdk / "sdk-release.json").write_bytes(metadata.read_bytes())
        (sdk / "consumer-lock.json").write_text(json.dumps(lock, sort_keys=True, indent=2) + "\n")
        os.rename(sdk, output)
    return output


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--lock", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        print(prepare(load_lock(args.lock), args.output.resolve()))
    except (ValueError, OSError, tarfile.TarError, subprocess.SubprocessError) as error:
        parser.exit(2, f"ForgePipe SDK preparation failed: {error}\n")


if __name__ == "__main__":
    main()
