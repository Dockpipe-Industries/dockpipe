#!/usr/bin/env python3
"""Verify the pinned ForgePipe SDK before the publisher attests its bytes."""
import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import sys
import tarfile
import urllib.parse
import urllib.request

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "packages/native-sdk/assets/scripts"))
from elf_artifacts import audit


HEADERS = {
    "ArcadeFlightDynamics.h", "FlightProjection.h", "SpatialGpuFoliage.h",
    "SpatialGpuRenderer.h", "SpatialMesh.h", "SpatialPaintOrder.h",
    "SpatialSunShadow.h", "SpatialSurfaceMaterial.h", "SpatialTerrain.h",
}
LIBRARIES = {
    "lib/libforgepipe-engine.so", "lib/libforgepipe-engine.so.0",
    "lib/libforgepipe-engine.so.0.1.0",
}
SDK_FILES = (
    {"include/forgepipe/" + name for name in HEADERS}
    | LIBRARIES
    | {"lib/cmake/ForgePipe/" + name for name in (
        "ForgePipeConfig.cmake", "ForgePipeConfigVersion.cmake",
        "ForgePipeTargets.cmake", "ForgePipeTargets-release.cmake",
    )}
    | {"share/forgepipe/" + name for name in (
        "LICENSE", "NOTICE", "SOURCE-PROVENANCE.json", "MATERIAL-NOTICES.md",
    )}
)
MAX_ARCHIVE_BYTES = 256 << 20
MAX_EXPANDED_BYTES = 512 << 20


def download(entry, directory, tag, maximum):
    parsed = urllib.parse.urlparse(entry["url"])
    prefix = "/Dockpipe-Industries/dockpipe/releases/download/" + tag + "/"
    if (parsed.scheme != "https" or parsed.netloc != "github.com"
            or not parsed.path.startswith(prefix) or parsed.query or parsed.fragment):
        raise ValueError("SDK asset must belong to the selected publisher release")
    name = parsed.path[len(prefix):]
    if not name or PurePosixPath(name).name != name or ":" in name or "\\" in name:
        raise ValueError("Invalid SDK asset name")
    size = entry["size_bytes"]
    if type(size) is not int or not 0 < size <= maximum:
        raise ValueError("SDK asset exceeds its byte budget")
    digest = entry["sha256"]
    if not isinstance(digest, str) or not re.fullmatch(r"[a-f0-9]{64}", digest):
        raise ValueError("SDK asset requires an exact digest")
    with urllib.request.urlopen(entry["url"], timeout=60) as response:
        payload = response.read(size + 1)
    if len(payload) != size or hashlib.sha256(payload).hexdigest() != digest:
        raise ValueError("SDK asset differs from its committed pin")
    path = directory / name
    path.write_bytes(payload)
    return path


def verify_archive(path, output, expected_exports):
    root = PurePosixPath("ForgePipe-SDK-0.1.0-Linux")
    seen = set()
    files = {}
    expanded = 0
    with tarfile.open(path, "r:gz") as archive:
        for index, member in enumerate(archive):
            if index >= 128:
                raise ValueError("SDK archive exceeds its member budget")
            name = member.name.rstrip("/") if member.isdir() else member.name
            parsed = PurePosixPath(name)
            if (name != parsed.as_posix() or parsed.is_absolute() or ".." in parsed.parts
                    or "\\" in name or ":" in name or name in seen):
                raise ValueError("Invalid or duplicate SDK archive member")
            seen.add(name)
            if parsed == root and member.isdir():
                continue
            if root not in parsed.parents:
                raise ValueError("SDK archive member is outside its owned root")
            relative = parsed.relative_to(root).as_posix()
            if member.isdir():
                if not any(item.startswith(relative + "/") for item in SDK_FILES):
                    raise ValueError("Unexpected SDK archive directory")
                continue
            if not member.isfile() or relative not in SDK_FILES:
                raise ValueError("Unapproved SDK file, source, or link")
            expanded += member.size
            if member.size > MAX_ARCHIVE_BYTES or expanded > MAX_EXPANDED_BYTES:
                raise ValueError("SDK archive exceeds its expanded byte budget")
            with archive.extractfile(member) as stream:
                payload = stream.read(member.size + 1)
            if len(payload) != member.size:
                raise ValueError("Truncated SDK member")
            files[relative] = payload
    if set(files) != SDK_FILES:
        raise ValueError("SDK archive inventory does not match the approved SDK surface")
    binary_payload = files["lib/libforgepipe-engine.so.0.1.0"]
    if any(files[name] != binary_payload for name in LIBRARIES):
        raise ValueError("SDK library aliases must contain identical regular-file bytes")
    binary = output / "audited-engine.so"
    binary.write_bytes(binary_payload)
    receipt = audit(binary, expected_exports)
    if any(name.startswith((".debug", ".zdebug"))
            or name in {".symtab", ".gnu_debugdata", ".gdb_index"}
            for name in receipt["sections"]):
        raise ValueError("Public SDK contains private symbols or debug information")
    return receipt


def prepare(lock_path, exports_path, output, tag):
    if not re.fullmatch(r"forgepipe-sdk-[a-zA-Z0-9._-]+", tag):
        raise ValueError("Expected a dedicated ForgePipe SDK release tag")
    lock = json.loads(lock_path.read_text())
    if (lock["version"] != "0.1.0" or lock["platform"] != "linux-amd64"
            or lock["qt_version"] != "6.10.3"
            or lock["source_repository"] != "Dockpipe-Industries/forgepipe"
            or lock["public_redistribution"] is not True
            or not re.fullmatch(r"[a-f0-9]{40}", lock["source_sha"] or "")):
        raise ValueError("SDK lock has not frozen the approved compatible release")
    output.mkdir(parents=True, exist_ok=False)
    archive = download(lock["artifact"], output, tag, MAX_ARCHIVE_BYTES)
    metadata_path = download(lock["metadata"], output, tag, 64 << 10)
    if archive.name != "ForgePipe-SDK-0.1.0-Linux.tar.gz" or metadata_path.name != "sdk-release.json":
        raise ValueError("Unexpected SDK publication asset names")
    metadata = json.loads(metadata_path.read_text())
    for field in ("version", "platform", "qt_version", "source_repository", "source_sha", "public_redistribution"):
        if metadata[field] != lock[field]:
            raise ValueError("SDK provenance differs from the committed lock: " + field)
    expected_archive = {"name": archive.name, "size_bytes": lock["artifact"]["size_bytes"],
                        "sha256": lock["artifact"]["sha256"]}
    if metadata["archive"] != expected_archive:
        raise ValueError("SDK provenance archive identity differs from its pin")
    expected_exports = [line for line in exports_path.read_text().splitlines() if line]
    receipt = verify_archive(archive, output, expected_exports)
    (output / "publisher-audit.json").write_text(json.dumps(receipt, indent=2) + "\n")
    # Only these two immutable inputs are attestation subjects. The extracted
    # library and detailed publisher audit stay outside the public SDK archive.
    (output / "subjects.txt").write_text(str(archive) + "\n" + str(metadata_path) + "\n")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--lock", required=True, type=Path)
    parser.add_argument("--exports", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--tag", required=True)
    arguments = parser.parse_args()
    prepare(arguments.lock, arguments.exports, arguments.output, arguments.tag)
