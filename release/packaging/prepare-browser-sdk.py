#!/usr/bin/env python3
"""Prepare an exact public Spatial-Web Browser SDK from a committed matrix lock.

All three assets require publisher attestations, including --local-assets input.
Qt remains an independently supplied exact-version runtime. This command never
selects a channel, resolves a branch, or overwrites an installed SDK prefix.
"""
import argparse
import gzip
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys
import tarfile
import tempfile
import time
from urllib.parse import quote
from urllib.request import HTTPRedirectHandler, build_opener

REPOSITORY = "Dockpipe-Industries/Spatial-Web"
PRODUCT = "SpatialWebBrowserSDK"
VERSION = "0.1.0"
QT_VERSION = "6.10.3"
PLATFORMS = {"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64"}
MAX_METADATA_BYTES = 4 << 20
MAX_ARCHIVE_BYTES = 512 << 20
MAX_INSTALLED_BYTES = 512 << 20
MAX_FILES = 10000
MAX_TAR_BYTES = MAX_INSTALLED_BYTES + MAX_FILES * 4096 + MAX_METADATA_BYTES
TRANSFER_SECONDS = 300
SIDECARS = {"consumer-lock.json", "consumer-receipt.json"}


class BoundedTarInfo(tarfile.TarInfo):
    @classmethod
    def frombuf(cls, buffer, encoding, errors):
        member = super().frombuf(buffer, encoding, errors)
        extended_headers = {tarfile.XHDTYPE, tarfile.XGLTYPE,
                            tarfile.GNUTYPE_LONGNAME, tarfile.GNUTYPE_LONGLINK}
        if member.type in extended_headers and member.size > MAX_METADATA_BYTES:
            raise ValueError("SDK extended tar header exceeds capacity")
        return member


class BoundedTarReader:
    """Bound decompression including padding and extended tar headers."""

    def __init__(self, source):
        self.source = source
        self.total = 0

    def read(self, size):
        if size < 0 or size > MAX_METADATA_BYTES:
            raise ValueError("SDK tar header read exceeds capacity")
        data = self.source.read(min(size, MAX_TAR_BYTES - self.total + 1))
        self.total += len(data)
        if self.total > MAX_TAR_BYTES:
            raise ValueError("SDK decompressed archive exceeds capacity")
        return data


class HTTPSRedirectHandler(HTTPRedirectHandler):
    def redirect_request(self, request, file_pointer, code, message, headers, new_url):
        if not new_url.startswith("https://"):
            raise ValueError("SDK asset redirected outside HTTPS")
        return super().redirect_request(request, file_pointer, code, message, headers, new_url)


def open_public(url):
    return build_opener(HTTPSRedirectHandler()).open(url, timeout=60)


def unique_keys(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"Duplicate JSON field: {key}")
        result[key] = value
    return result


def decode_json(data):
    return json.loads(data, object_pairs_hook=unique_keys)


def read_regular(path, limit):
    if path.is_symlink() or not path.is_file() or path.stat().st_size > limit:
        raise ValueError(f"Missing, linked or oversized input: {path.name}")
    with path.open("rb") as stream:
        data = stream.read(limit + 1)
    if len(data) > limit:
        raise ValueError("Input exceeds capacity")
    return data


def valid_digest(value):
    return isinstance(value, str) and re.fullmatch(r"[0-9a-f]{64}", value)


def copy_bounded(source, destination, limit):
    total = 0
    while True:
        block = source.read(min(1 << 20, limit - total + 1))
        if not block:
            return
        total += len(block)
        if total > limit:
            raise ValueError("Asset exceeds its admitted size")
        destination.write(block)


def canonical_path(name):
    if not isinstance(name, str):
        raise ValueError("Payload path must be a string")
    path = PurePosixPath(name)
    if (not name or name == "." or name != path.as_posix() or path.is_absolute()
            or ".." in path.parts or "\\" in name or ":" in name or "\x00" in name):
        raise ValueError("Unsafe SDK payload path")
    for part in path.parts:
        device = part.split(".", 1)[0].casefold()
        if (len(part) > 255 or part.endswith((".", " "))
                or any(ord(character) < 32 or character in '<>"|?*' for character in part)
                or device in {"con", "prn", "aux", "nul"}
                or re.fullmatch(r"(?:com|lpt)[0-9¹²³]", device)):
            raise ValueError("SDK payload path is not portable across supported platforms")
    if len(name) > 1024:
        raise ValueError("SDK payload path exceeds capacity")
    return path


def unpack(archive, destination, profile, entries):
    prefix = profile["prefix"]
    allowed_directories = set()
    for name in entries:
        allowed_directories.update(parent.as_posix() for parent in PurePosixPath(name).parents if parent.as_posix() != ".")
    seen = set()
    files = set()
    with gzip.open(archive, "rb") as compressed, tarfile.open(
            fileobj=BoundedTarReader(compressed), mode="r|", ignore_zeros=True, tarinfo=BoundedTarInfo) as bundle:
        for count, member in enumerate(bundle, 1):
            if count > MAX_FILES * 4:
                raise ValueError("SDK archive exceeds member capacity")
            raw_name = member.name.rstrip("/") if member.isdir() else member.name
            canonical_path(raw_name)
            if raw_name in seen:
                raise ValueError("Duplicate SDK archive member")
            seen.add(raw_name)
            if prefix and raw_name == prefix.rstrip("/") and member.isdir():
                continue
            if prefix and not raw_name.startswith(prefix):
                raise ValueError("SDK archive member lies outside its product prefix")
            name = raw_name[len(prefix):] if prefix else raw_name
            canonical_path(name)
            if member.isdir():
                if name not in allowed_directories:
                    raise ValueError("SDK archive contains an extra directory")
                continue
            if not member.isfile() or member.issparse() or name not in entries:
                raise ValueError("SDK archive contains a link, special file or extra file")
            entry = entries[name]
            if member.size != entry["bytes"]:
                raise ValueError("SDK archive member size differs from inventory")
            target = destination / name
            target.parent.mkdir(parents=True, exist_ok=True)
            with bundle.extractfile(member) as source, target.open("xb") as output:
                copy_bounded(source, output, entry["bytes"])
            checksum = hashlib.sha256()
            with target.open("rb") as installed:
                for block in iter(lambda: installed.read(1 << 20), b""):
                    checksum.update(block)
            if checksum.hexdigest() != entry["sha256"]:
                raise ValueError("SDK archive member checksum differs from inventory")
            target.chmod(0o755 if member.mode & 0o111 else 0o644)
            files.add(name)
    if files != set(entries):
        raise ValueError("SDK archive is missing inventory files")


def digest(path):
    checksum = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1 << 20), b""):
            checksum.update(block)
    return checksum.hexdigest()


def validate_pin(pin, name, maximum):
    if (not isinstance(pin, dict) or set(pin) != {"name", "sha256", "size_bytes"}
            or pin.get("name") != name or not valid_digest(pin.get("sha256"))
            or type(pin.get("size_bytes")) is not int or not 0 < pin["size_bytes"] <= maximum):
        raise ValueError(f"Invalid asset identity, size or checksum: {name}")


def validate_compiler(compiler, platform):
    expected = "GNU" if platform.startswith("linux-") else "AppleClang"
    if platform == "windows-amd64":
        expected = "MSVC"
    if (not isinstance(compiler, dict) or set(compiler) != {"id", "version"}
            or compiler["id"] != expected or not isinstance(compiler["version"], str)
            or not re.fullmatch(r"[0-9]+(?:\.[0-9]+){1,3}", compiler["version"])):
        raise ValueError("SDK compiler identity differs from the platform Qt ABI")


def load_lock(path, platform):
    if platform not in PLATFORMS:
        raise ValueError("Unsupported Browser SDK platform")
    data = read_regular(path, MAX_METADATA_BYTES)
    lock = decode_json(data)
    expected = {"schema": 1, "product": PRODUCT, "version": VERSION,
                "qt_version": QT_VERSION, "source_repository": REPOSITORY}
    if (not isinstance(lock, dict) or type(lock.get("schema")) is not int
            or any(lock.get(key) != value for key, value in expected.items())):
        raise ValueError("Browser SDK lock identity differs from the supported contract")
    source_sha = lock.get("source_sha")
    if not isinstance(source_sha, str) or not re.fullmatch(r"[0-9a-f]{40}", source_sha):
        raise ValueError("Browser SDK source requires an exact commit SHA")
    release = {"repository": REPOSITORY, "tag": "browser-sdk-0.1.0-staging." + source_sha}
    if lock.get("release") != release:
        raise ValueError("Browser SDK requires its exact immutable source candidate tag")
    matrix = lock.get("platforms")
    if not isinstance(matrix, dict) or set(matrix) != PLATFORMS:
        raise ValueError("Browser SDK lock must bind all five exact platforms")
    for name, entry in matrix.items():
        stem = PRODUCT + "-" + VERSION + "-" + name
        if not isinstance(entry, dict):
            raise ValueError("Browser SDK platform entry must be an object")
        fields = {"artifact", "metadata", "file_manifest", "compiler"}
        if name == "windows-amd64":
            fields.add("msvc_runtime")
        if set(entry) != fields:
            raise ValueError("Browser SDK platform entry fields differ from the supported contract")
        validate_pin(entry["artifact"], stem + ".tar.gz", MAX_ARCHIVE_BYTES)
        validate_pin(entry["metadata"], stem + ".release.json", MAX_METADATA_BYTES)
        validate_pin(entry["file_manifest"], stem + ".files.json", MAX_METADATA_BYTES)
        validate_compiler(entry["compiler"], name)
        if name == "windows-amd64" and entry["msvc_runtime"] != "dynamic-release-/MD":
            raise ValueError("Windows SDK requires the dynamic release MSVC runtime")
    selected = {key: value for key, value in lock.items() if key != "platforms"}
    selected.update(matrix[platform], platform=platform)
    return selected, data


def fetch_asset(lock, asset, destination, local_assets):
    limit = asset["size_bytes"]
    if local_assets is not None:
        source_path = local_assets / asset["name"]
        if source_path.is_symlink() or not source_path.is_file():
            raise ValueError("Local SDK asset must be a regular file")
        with source_path.open("rb") as source, destination.open("xb") as output:
            copy_bounded(source, output, limit)
    else:
        url = (f"https://github.com/{REPOSITORY}/releases/download/"
               f"{quote(lock['release']['tag'], safe='')}/{quote(asset['name'], safe='')}")
        deadline = time.monotonic() + TRANSFER_SECONDS
        with open_public(url) as response, destination.open("xb") as output:
            if not response.url.startswith("https://"):
                raise ValueError("Browser SDK asset redirected outside HTTPS")
            total = 0
            while True:
                if time.monotonic() > deadline:
                    raise ValueError("Browser SDK asset download timed out")
                block = response.read(min(1 << 20, limit - total + 1))
                if not block:
                    break
                total += len(block)
                if total > limit:
                    raise ValueError("Browser SDK asset exceeds its pinned size")
                output.write(block)
    if destination.stat().st_size != limit or digest(destination) != asset["sha256"]:
        raise ValueError("Browser SDK asset differs from its pinned checksum or size")


def verify_attestation(path, source_sha, workflow="browser-sdk.yml"):
    command = ["gh", "attestation", "verify", str(path), "--repo", REPOSITORY,
               "--signer-workflow", REPOSITORY + "/.github/workflows/" + workflow,
               "--source-digest", source_sha, "--source-ref", "refs/heads/dev"]
    subprocess.run(command, check=True, timeout=120, stdout=sys.stderr, stderr=sys.stderr)


def verify_metadata(lock, metadata):
    expected = {key: lock[key] for key in
                ("schema", "product", "version", "source_repository", "source_sha", "platform", "qt_version", "compiler")}
    expected.update(source_state="committed-snapshot", runtime_bundled=False, abi={"cxx_standard": 17})
    if (not isinstance(metadata, dict) or type(metadata.get("schema")) is not int
            or metadata.get("runtime_bundled") is not False
            or any(metadata.get(key) != value for key, value in expected.items())):
        raise ValueError("Browser SDK release metadata differs from its pinned source or ABI")
    if metadata.get("archive") != lock["artifact"]:
        raise ValueError("Browser SDK metadata does not bind its exact archive")
    manifest = {key: lock["file_manifest"][key] for key in ("name", "sha256")}
    if metadata.get("file_manifest") != manifest:
        raise ValueError("Browser SDK metadata does not bind its exact file inventory")
    checks = {"sdk-component-install", "focused-browser-tests", "relocated-external-consumer-compile-link-run"}
    if (not isinstance(metadata.get("checks"), list)
            or not all(isinstance(check, str) for check in metadata["checks"])
            or not checks.issubset(metadata["checks"])):
        raise ValueError("Browser SDK lacks the producer install, browser or relocation qualification")
    expected_runtime = lock.get("msvc_runtime")
    if metadata.get("msvc_runtime") != expected_runtime:
        raise ValueError("Browser SDK MSVC runtime identity differs")


def verify_manifest(data, reserved=(), package="SpatialWebBrowser"):
    manifest = decode_json(data)
    if (not isinstance(manifest, dict) or manifest.get("schema") != 1 or type(manifest.get("schema")) is not int
            or manifest.get("artifact_class") != "sdk" or not isinstance(manifest.get("files"), list)
            or not 0 < len(manifest["files"]) <= MAX_FILES):
        raise ValueError("Browser SDK inventory is invalid")
    reserved = SIDECARS | {name.casefold() for name in reserved}
    entries = {}
    folded = set()
    total = 0
    for entry in manifest["files"]:
        if not isinstance(entry, dict) or set(entry) != {"path", "bytes", "sha256"}:
            raise ValueError("Browser SDK inventory entry is invalid")
        name = canonical_path(entry["path"]).as_posix()
        if name.casefold() in folded or name.casefold() in reserved:
            raise ValueError("Browser SDK inventory contains a duplicate or reserved path")
        if type(entry["bytes"]) is not int or entry["bytes"] < 0 or not valid_digest(entry["sha256"]):
            raise ValueError("Browser SDK inventory size or digest is invalid")
        parts = PurePosixPath(name).parts
        base = parts[-1].casefold()
        if (any(part.casefold().startswith(("qt6", "qtwebengine", "spatialwebprojection", "spatialpipe", "forgepipe")) for part in parts)
                or base.startswith("libqt6")
                or any(part.casefold().startswith("qt") and part.casefold().endswith(".framework") for part in parts)
                or any(part.casefold() in {"plugins", "qml"} for part in parts)):
            raise ValueError("Browser-only SDK must not bundle Qt, Projection or spatial runtimes")
        total += entry["bytes"]
        if total > MAX_INSTALLED_BYTES:
            raise ValueError("Browser SDK installed inventory exceeds capacity")
        entries[name] = entry
        folded.add(name.casefold())
    required = {"include/SpatialWeb/Browser/BrowserWidget.h",
                f"lib/cmake/{package}/{package}Config.cmake",
                f"lib/cmake/{package}/{package}ConfigVersion.cmake",
                f"lib/cmake/{package}/{package}Targets.cmake"}
    if not required.issubset(entries):
        raise ValueError("Browser SDK lacks its public header or CMake package")
    for name in entries:
        if any(parent.as_posix().casefold() in folded for parent in PurePosixPath(name).parents):
            raise ValueError("Browser SDK inventory has a file used as a directory")
    return entries


def receipt_identity(lock):
    return {key: lock[key] for key in
            ("source_repository", "source_sha", "qt_version", "compiler", "release")}


def prepare(lock_path, platform, output, local_assets=None):
    lock, lock_bytes = load_lock(lock_path, platform)
    return prepare_selected(lock, lock_bytes, platform, output, local_assets)


def prepare_selected(lock, lock_bytes, platform, output, local_assets=None, *,
                     metadata_verifier=verify_metadata, manifest_verifier=verify_manifest,
                     attestation_verifier=None, identity_factory=receipt_identity, product=PRODUCT):
    # A dependency profile supplies its identity validators; transport, inventory
    # extraction and fresh-directory publication retain the same bounded rules.
    if attestation_verifier is None:
        attestation_verifier = verify_attestation
    if output.exists() or output.is_symlink():
        raise ValueError("Browser SDK destination must be fresh")
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".browser-sdk-", dir=output.parent) as temporary:
        root = Path(temporary)
        paths = {}
        for key in ("metadata", "file_manifest", "artifact"):
            asset = lock[key]
            path = root / asset["name"]
            fetch_asset(lock, asset, path, local_assets)
            paths[key] = path
        metadata_bytes = read_regular(paths["metadata"], MAX_METADATA_BYTES)
        metadata = decode_json(metadata_bytes)
        metadata_verifier(lock, metadata)
        manifest_bytes = read_regular(paths["file_manifest"], MAX_METADATA_BYTES)
        entries = manifest_verifier(manifest_bytes, [asset["name"] for asset in lock.values()
                                                    if isinstance(asset, dict) and "name" in asset])
        for path in paths.values():
            attestation_verifier(path, lock["source_sha"])
        destination = root / "installed"
        destination.mkdir()
        unpack(paths["artifact"], destination, {"prefix": ""}, entries)
        for key, data in (("metadata", metadata_bytes), ("file_manifest", manifest_bytes)):
            name = lock[key]["name"]
            if name in entries:
                raise ValueError("Browser SDK payload collides with its release sidecars")
            (destination / name).write_bytes(data)
        (destination / "consumer-lock.json").write_bytes(lock_bytes)
        receipt = {"schema": 1, "product": product, "version": VERSION,
                   "selected_platform": platform, "installed_prefix": str(output.resolve()),
                   "lock_sha256": hashlib.sha256(lock_bytes).hexdigest(),
                   "identity": identity_factory(lock),
                   "pins": {key: lock[key] for key in ("artifact", "metadata", "file_manifest")},
                   "attestations_verified": True, "runtime_bundled": False,
                   "files": len(entries), "installed_bytes": sum(item["bytes"] for item in entries.values())}
        if platform == "windows-amd64":
            receipt["identity"]["msvc_runtime"] = lock["msvc_runtime"]
        (destination / "consumer-receipt.json").write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n")
        if output.exists() or output.is_symlink():
            raise ValueError("Browser SDK destination appeared during preparation")
        destination.rename(output)
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--lock", type=Path, required=True)
    parser.add_argument("--platform", choices=sorted(PLATFORMS), required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--receipt", type=Path)
    parser.add_argument("--local-assets", type=Path)
    args = parser.parse_args()
    try:
        if args.receipt and (args.receipt.exists() or args.receipt.is_symlink()):
            raise ValueError("Browser SDK receipt destination must be fresh")
        receipt = prepare(args.lock, args.platform, args.output, args.local_assets)
        rendered = json.dumps(receipt, indent=2, sort_keys=True) + "\n"
        if args.receipt:
            args.receipt.parent.mkdir(parents=True, exist_ok=True)
            with args.receipt.open("x") as output:
                output.write(rendered)
    except (ValueError, OSError, tarfile.TarError, subprocess.SubprocessError) as error:
        parser.exit(1, f"Browser SDK preparation failed: {error}\n")
    print(rendered, end="")


if __name__ == "__main__":
    main()
