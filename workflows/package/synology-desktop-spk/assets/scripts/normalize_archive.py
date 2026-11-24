"""Normalize our Skopeo Docker-save output without following legacy aliases."""

import gzip
import importlib.util
import io
import json
from pathlib import Path
import tarfile


SPECIFICATION = importlib.util.spec_from_file_location(
    "synology_image_archive_admission", Path(__file__).with_name("image_receipt.py")
)
ADMISSION = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(ADMISSION)

# Share the image admission capacity contract; normalization never relaxes it.
MAX_ARCHIVE_BYTES = ADMISSION.MAX_ARCHIVE_BYTES
MAX_MEMBERS = ADMISSION.MAX_MEMBERS
MAX_METADATA_BYTES = ADMISSION.MAX_METADATA_BYTES
MAX_EXTENSION_BYTES = ADMISSION.MAX_EXTENSION_BYTES


class SourceTarInfo(tarfile.TarInfo):
    def _proc_member(self, archive):
        archive.header_count = getattr(archive, "header_count", 0) + 1
        if archive.header_count > MAX_MEMBERS:
            raise ValueError("source archive exceeds member limit")
        if self.size < 0 or self.size > MAX_ARCHIVE_BYTES:
            raise ValueError("source archive member exceeds byte limit")
        extensions = (tarfile.XHDTYPE, tarfile.XGLTYPE, tarfile.GNUTYPE_LONGNAME)
        if self.type in extensions:
            archive.extension_bytes = getattr(archive, "extension_bytes", 0) + self.size
            if self.size > MAX_EXTENSION_BYTES or archive.extension_bytes > MAX_METADATA_BYTES:
                raise ValueError("source archive extension exceeds metadata limit")
        elif self.issym():
            if self.size != 0:
                raise ValueError("source archive symlink has a nonempty body")
        elif not self.isfile() and not self.isdir():
            raise ValueError("source archive contains unsupported member type")
        return super()._proc_member(archive)


class BoundedOutput:
    def __init__(self, stream):
        self.stream = stream
        self.bytes_written = 0

    def write(self, content):
        self.bytes_written += len(content)
        if self.bytes_written > MAX_ARCHIVE_BYTES:
            raise ValueError("normalized archive exceeds expanded byte limit")
        return self.stream.write(content)


def source_members(archive):
    members = {}
    for member in archive:
        name = ADMISSION.safe_member_name(member.name)
        if name in members:
            raise ValueError("duplicate source archive member")
        if member.sparse is not None:
            raise ValueError("source archive contains a sparse member")
        if member.size < 0 or member.size > MAX_ARCHIVE_BYTES:
            raise ValueError("source archive member exceeds byte limit")
        members[name] = member
    return members


def required_file(members, name):
    if not isinstance(name, str):
        raise ValueError("manifest reference must be a string")
    normalized = ADMISSION.safe_member_name(name)
    member = members.get(normalized)
    if member is None or not member.isfile():
        raise ValueError("manifest references a missing or nonregular member")
    return member


def canonical_manifest(archive, members, repo_tag=None):
    manifest_member = required_file(members, "manifest.json")
    if manifest_member.size > MAX_METADATA_BYTES:
        raise ValueError("source manifest exceeds metadata limit")
    with archive.extractfile(manifest_member) as stream:
        manifest = ADMISSION.parse_json(stream.read(manifest_member.size), "source manifest")
    if not isinstance(manifest, list) or len(manifest) != 1 or not isinstance(manifest[0], dict):
        raise ValueError("source archive must contain exactly one manifest image")
    image = manifest[0]
    config = required_file(members, image.get("Config"))
    layers = image.get("Layers")
    tags = image.get("RepoTags")
    if not isinstance(layers, list):
        raise ValueError("source manifest requires a Layers list")
    if tags is not None and not isinstance(tags, list):
        raise ValueError("source manifest RepoTags must be a list or null")
    if tags is not None and any(not isinstance(tag, str) for tag in tags):
        raise ValueError("source manifest RepoTags must contain strings")
    if manifest_member.size + config.size > MAX_METADATA_BYTES:
        raise ValueError("source configuration exceeds metadata limit")
    selected = [config]
    selected_names = {ADMISSION.safe_member_name(config.name)}
    if "manifest.json" in selected_names:
        raise ValueError("source manifest cannot reference itself")
    layer_names = []
    for layer in layers:
        member = required_file(members, layer)
        name = ADMISSION.safe_member_name(member.name)
        if name == "manifest.json":
            raise ValueError("source manifest cannot reference itself")
        layer_names.append(name)
        if name not in selected_names:
            selected.append(member)
            selected_names.add(name)
    if repo_tag is not None:
        tags = [repo_tag]
    result = [{"Config": ADMISSION.safe_member_name(config.name),
               "RepoTags": tags, "Layers": layer_names}]
    content = json.dumps(result, sort_keys=True, separators=(",", ":")).encode("utf-8")
    return content, selected


def add_regular_file(archive, name, size, source):
    member = tarfile.TarInfo(name)
    member.size = size
    member.mode = 0o644
    member.uid = 0
    member.gid = 0
    member.mtime = 0
    archive.addfile(member, source)


def write_loadable_archive(source_tar, destination_gzip, *, repo_tag=None):
    """Select regular manifest/config/layers; omit unused legacy link aliases.

    Source must be the uncompressed Docker-save tar produced by our Skopeo
    prebuild. The destination must not already exist and is removed on failure.
    Layer data flows directly between tar readers/writers; nothing is extracted.
    An optional canonical content-derived GHCR repo_tag replaces RepoTags only.
    """
    if repo_tag is not None:
        if not isinstance(repo_tag, str) or not ADMISSION.OFFLINE_IMAGE.fullmatch(repo_tag):
            raise ValueError("offline repository tag must be a canonical GHCR content tag")
    source_tar = Path(source_tar)
    destination_gzip = Path(destination_gzip)
    if source_tar.stat().st_size > MAX_ARCHIVE_BYTES:
        raise ValueError("source archive exceeds byte limit")
    created_destination = False
    try:
        with tarfile.open(source_tar, mode="r:", tarinfo=SourceTarInfo) as source:
            members = source_members(source)
            manifest, selected = canonical_manifest(source, members, repo_tag)
            with destination_gzip.open("xb") as destination:
                created_destination = True
                with gzip.GzipFile(filename="", mode="wb", fileobj=destination,
                                   compresslevel=1, mtime=0) as compressed:
                    with tarfile.open(fileobj=BoundedOutput(compressed), mode="w|",
                                      format=tarfile.PAX_FORMAT) as output:
                        add_regular_file(output, "manifest.json", len(manifest), io.BytesIO(manifest))
                        for member in selected:
                            with source.extractfile(member) as content:
                                add_regular_file(output, ADMISSION.safe_member_name(member.name),
                                                 member.size, content)
    except Exception:
        if created_destination:
            destination_gzip.unlink(missing_ok=True)
        raise
