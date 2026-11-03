"""Admit immutable image receipts and bounded Docker-save archives without Docker."""

import gzip
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import tarfile


# Packaging capacity contract: a desktop image must fit an 8 GiB expanded save
# archive. Metadata is limited independently so layers never consume JSON memory.
MAX_ARCHIVE_BYTES = 8 * 1024 ** 3
MAX_MEMBERS = 4096
MAX_METADATA_BYTES = 4 * 1024 ** 2
MAX_RECEIPT_BYTES = 64 * 1024
MAX_EXTENSION_BYTES = 64 * 1024
SHA256 = re.compile(r"[0-9a-f]{64}\Z")
IMAGE_ID = re.compile(r"sha256:[0-9a-f]{64}\Z")
GHCR_REPOSITORY = r"ghcr\.io/[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)+"
IMAGE_REFERENCE = re.compile(GHCR_REPOSITORY + r"@sha256:[0-9a-f]{64}\Z")
OFFLINE_IMAGE = re.compile(GHCR_REPOSITORY + r":offline-[0-9a-f]{64}\Z")


def file_sha256(path):
    checksum = hashlib.sha256()
    with Path(path).open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            checksum.update(block)
    return checksum.hexdigest()


def parse_json(content, description):
    def unique_object(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError(f"duplicate JSON key in {description}: {key}")
            result[key] = value
        return result

    try:
        return json.loads(content, object_pairs_hook=unique_object)
    except (UnicodeError, json.JSONDecodeError, RecursionError) as error:
        raise ValueError(f"invalid JSON in {description}") from error


def safe_member_name(name):
    path = PurePosixPath(name)
    if not name or path.is_absolute() or "\\" in name or ".." in path.parts:
        raise ValueError("unsafe image archive member path")
    normalized = str(path)
    if normalized == ".":
        raise ValueError("unsafe image archive member path")
    return normalized


class BoundedArchiveStream:
    def __init__(self, stream):
        self.stream = stream
        self.bytes_read = 0

    def read(self, size):
        remaining = MAX_ARCHIVE_BYTES - self.bytes_read
        content = self.stream.read(min(size, remaining + 1))
        self.bytes_read += len(content)
        if self.bytes_read > MAX_ARCHIVE_BYTES:
            raise ValueError("image archive exceeds expanded byte limit")
        return content


class BoundedTarInfo(tarfile.TarInfo):
    def _proc_member(self, archive):
        # tarfile processes PAX/GNU extension bodies before yielding a member.
        # Admit their header sizes here, before that allocation takes place.
        archive.admitted_header_count = getattr(archive, "admitted_header_count", 0) + 1
        if archive.admitted_header_count > MAX_MEMBERS:
            raise ValueError("image archive exceeds member limit")
        if self.size < 0 or self.size > MAX_ARCHIVE_BYTES:
            raise ValueError("image archive member exceeds byte limit")
        extensions = (tarfile.XHDTYPE, tarfile.XGLTYPE, tarfile.GNUTYPE_LONGNAME)
        if self.type in extensions:
            extension_bytes = getattr(archive, "admitted_extension_bytes", 0) + self.size
            archive.admitted_extension_bytes = extension_bytes
            if self.size > MAX_EXTENSION_BYTES or extension_bytes > MAX_METADATA_BYTES:
                raise ValueError("image archive extension exceeds metadata limit")
        elif not self.isfile() and not self.isdir():
            raise ValueError("image archive contains unsupported member type")
        return super()._proc_member(archive)


def archive_metadata(path, config_name=None):
    members = {}
    metadata = {}
    metadata_bytes = 0
    try:
        with gzip.open(path, "rb") as compressed:
            bounded = BoundedArchiveStream(compressed)
            with tarfile.open(fileobj=bounded, mode="r|", tarinfo=BoundedTarInfo) as archive:
                for member in archive:
                    name = safe_member_name(member.name)
                    if name in members:
                        raise ValueError("duplicate image archive member")
                    if member.sparse is not None or (not member.isfile() and not member.isdir()):
                        raise ValueError("image archive contains unsupported member type")
                    if member.size < 0 or member.size > MAX_ARCHIVE_BYTES:
                        raise ValueError("image archive member exceeds byte limit")
                    members[name] = member.isfile()
                    if member.isfile() and (name.endswith(".json") or name == config_name):
                        metadata_bytes += member.size
                        if metadata_bytes > MAX_METADATA_BYTES:
                            raise ValueError("image archive exceeds JSON metadata limit")
                        with archive.extractfile(member) as stream:
                            metadata[name] = stream.read(member.size)
            # Verify gzip completion/CRC and bound expanded trailing padding too.
            while bounded.read(1024 * 1024):
                pass
    except (tarfile.TarError, EOFError, OSError) as error:
        raise ValueError("invalid compressed Docker-save archive") from error
    return members, metadata


def validate_archive(path, receipt):
    path = Path(path)
    if path.stat().st_size > MAX_ARCHIVE_BYTES:
        raise ValueError("image archive exceeds compressed byte limit")
    if file_sha256(path) != receipt["archive_sha256"]:
        raise ValueError("image archive SHA-256 does not match receipt")
    members, metadata = archive_metadata(path)
    manifest = parse_json(metadata.get("manifest.json", b"null"), "archive manifest")
    if not isinstance(manifest, list) or len(manifest) != 1 or not isinstance(manifest[0], dict):
        raise ValueError("image archive must contain exactly one manifest image")
    image = manifest[0]
    if image.get("RepoTags") != [receipt["offline_image"]]:
        raise ValueError("image archive tags do not match receipt offline_image")
    config_name = image.get("Config")
    layers = image.get("Layers")
    if not isinstance(config_name, str) or not isinstance(layers, list):
        raise ValueError("image archive manifest requires Config and Layers")
    config_name = safe_member_name(config_name)
    if config_name not in metadata and members.get(config_name, False):
        _, metadata = archive_metadata(path, config_name)
    config_bytes = metadata.get(config_name)
    if config_bytes is None or "sha256:" + hashlib.sha256(config_bytes).hexdigest() != receipt["image_id"]:
        raise ValueError("image archive configuration digest does not match receipt")
    config = parse_json(config_bytes, "image configuration")
    if not isinstance(config, dict):
        raise ValueError("image configuration must be an object")
    if config.get("os") != "linux" or config.get("architecture") != receipt["architecture"]:
        raise ValueError("image configuration platform does not match receipt")
    settings = config.get("config")
    if not isinstance(settings, dict) or settings.get("User") != "65532:65532":
        raise ValueError("image configuration must use user 65532:65532")
    labels = settings.get("Labels")
    if not isinstance(labels, dict):
        raise ValueError("image configuration requires identity labels")
    if labels.get("org.opencontainers.image.version") != receipt["version"]:
        raise ValueError("image configuration version label does not match receipt")
    if labels.get("org.opencontainers.image.revision") != receipt["source_sha"]:
        raise ValueError("image configuration revision label does not match receipt")
    rootfs = config.get("rootfs")
    diff_ids = rootfs.get("diff_ids") if isinstance(rootfs, dict) else None
    if not isinstance(diff_ids, list) or len(diff_ids) != len(layers):
        raise ValueError("image configuration layer count does not match manifest")
    if any(not isinstance(value, str) or not IMAGE_ID.fullmatch(value) for value in diff_ids):
        raise ValueError("image configuration contains invalid layer digests")
    for layer in layers:
        if not isinstance(layer, str) or not members.get(safe_member_name(layer), False):
            raise ValueError("image archive layer is missing or not a regular file")


def load_receipt(path, version, architecture, cli_deb, desktop_deb, offline_archive=None):
    """Return an admitted receipt; malformed or mismatched input raises ValueError."""
    with Path(path).open("rb") as stream:
        content = stream.read(MAX_RECEIPT_BYTES + 1)
    if len(content) > MAX_RECEIPT_BYTES:
        raise ValueError("image receipt exceeds byte limit")
    receipt = parse_json(content, "image receipt")
    if not isinstance(receipt, dict) or type(receipt.get("schema")) is not int or receipt["schema"] != 1:
        raise ValueError("image receipt requires schema 1")
    if architecture not in ("amd64", "arm64") or receipt.get("architecture") != architecture:
        raise ValueError("image receipt architecture does not match package")
    if not isinstance(version, str) or not version or receipt.get("version") != version:
        raise ValueError("image receipt version does not match package")
    for field, expression in (("source_sha", re.compile(r"[0-9a-f]{40}\Z")),
                              ("image", IMAGE_REFERENCE), ("image_id", IMAGE_ID),
                              ("archive_sha256", SHA256)):
        value = receipt.get(field)
        if not isinstance(value, str) or not expression.fullmatch(value):
            raise ValueError(f"image receipt has invalid {field}")
    expected_offline_image = receipt["image"].split("@", 1)[0] + ":offline-" + receipt["image_id"][7:]
    if receipt.get("offline_image") != expected_offline_image:
        raise ValueError("image receipt offline_image does not match content identity")
    inputs = receipt.get("inputs")
    if not isinstance(inputs, dict):
        raise ValueError("image receipt requires input hashes")
    for field, package in (("cli_sha256", cli_deb), ("desktop_sha256", desktop_deb)):
        if inputs.get(field) != file_sha256(package):
            raise ValueError(f"image receipt {field} does not match package")
    if offline_archive is not None:
        validate_archive(offline_archive, receipt)
    return receipt
