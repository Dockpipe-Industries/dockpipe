#!/usr/bin/env python3
"""Compose offline store metadata and optionally assemble verified archives.

The operator supplies trusted ownership separately from candidate metadata. Input
and output JSON use the catalog client's 8 MiB manifest bound. The validated
finite manifest entries and ownership set bound composition work.
"""
import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
import tempfile


MAX_MANIFEST_BYTES = 8 << 20
TOKEN = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._+-]*$")
SHA256 = re.compile(r"^[0-9a-fA-F]{64}$")
KINDS = {"workflow": "workflows", "resolver": "resolvers"}


def require_bounded_json(document):
    size = 0
    for fragment in json.JSONEncoder(allow_nan=False).iterencode(document):
        size += len(fragment.encode("utf-8"))
        if size > MAX_MANIFEST_BYTES:
            raise ValueError("JSON exceeds the 8 MiB manifest limit")


def valid_token(value):
    return isinstance(value, str) and TOKEN.fullmatch(value) and ".." not in value


def validate_entry(entry, kind, require_size):
    if not isinstance(entry, dict):
        raise ValueError("Package entry must be an object")
    name = entry.get("name")
    version = entry.get("version")
    if not valid_token(name) or not valid_token(version):
        raise ValueError("Invalid package name or version")
    prefix = "dockpipe-core-" if kind == "core" else f"dockpipe-{kind}-{name}-"
    if entry.get("tarball") != f"{prefix}{version}.tar.gz":
        raise ValueError("Unsafe or mismatched package tarball path")
    digest = entry.get("sha256")
    if not isinstance(digest, str) or not SHA256.fullmatch(digest):
        raise ValueError("Package requires a SHA-256 digest")
    size = entry.get("size_bytes")
    if "size_bytes" in entry or require_size:
        if type(size) is not int or size <= 0:
            raise ValueError("Package size_bytes must be a positive integer")
    if "hidden" in entry and type(entry["hidden"]) is not bool:
        raise ValueError("Package hidden metadata must be boolean")
    minimum = entry.get("min_dockpipe_version", "")
    if not isinstance(minimum, str) or (minimum and not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", minimum)):
        raise ValueError("Invalid minimum DockPipe version")
    return kind, name


def validate_store(document, candidate=False):
    require_bounded_json(document)
    if not isinstance(document, dict) or type(document.get("schema")) is not int or document["schema"] != 1:
        raise ValueError("Expected schema 1 store manifest")
    if not valid_token(document.get("platform")):
        raise ValueError("Store requires an explicit platform")
    packages = document.get("packages")
    if not isinstance(packages, dict):
        raise ValueError("Store packages must be an object")
    if candidate and "core" in packages:
        raise ValueError("Candidate cannot publish core")
    entries = {}
    tarballs = set()
    core = packages.get("core")
    if core is not None:
        validate_entry(core, "core", False)
        tarballs.add(core["tarball"])
    for kind, collection in KINDS.items():
        values = packages.get(collection, [])
        if not isinstance(values, list):
            raise ValueError("Package collections must be arrays")
        for entry in values:
            identity = validate_entry(entry, kind, candidate)
            if identity in entries:
                raise ValueError("Duplicate package identity")
            if entry["tarball"] in tarballs:
                raise ValueError("Colliding package tarball path")
            entries[identity] = entry
            tarballs.add(entry["tarball"])
    return entries, tarballs


def validate_ownership(owned):
    require_bounded_json(owned)
    if not isinstance(owned, (list, tuple)) or not owned:
        raise ValueError("Trusted ownership must be a nonempty list of kind/name pairs")
    identities = set()
    for pair in owned:
        if not isinstance(pair, (list, tuple)) or len(pair) != 2:
            raise ValueError("Ownership entries must be kind/name pairs")
        kind, name = pair
        if not isinstance(kind, str) or kind not in KINDS or not valid_token(name):
            raise ValueError("Invalid owned package identity")
        identity = kind, name
        if identity in identities:
            raise ValueError("Duplicate owned package identity")
        identities.add(identity)
    return identities


def compose(base, candidate, owned):
    """Return copied baseline metadata with exactly the trusted package set replaced.

    Legacy baseline entries may omit size_bytes; new candidates must declare it.
    Caller must separately stage every referenced archive and authenticate inputs.
    """
    identities = validate_ownership(owned)
    baseline_entries, baseline_tarballs = validate_store(base)
    candidate_entries, _ = validate_store(candidate, candidate=True)
    if base["platform"] != candidate["platform"]:
        raise ValueError("Candidate platform does not match baseline")
    if set(candidate_entries) != identities:
        raise ValueError("Candidate must supply exactly the complete trusted ownership set")

    # Even a reused owned archive path must not acquire different bytes. Immutable
    # archive names are retained by published consumers outside this composition.
    baseline_by_tarball = {
        entry["tarball"]: (identity, entry)
        for identity, entry in baseline_entries.items()
    }
    for identity, entry in candidate_entries.items():
        if entry["tarball"] in baseline_tarballs:
            previous_identity, previous = baseline_by_tarball.get(entry["tarball"], (None, {}))
            same_archive = (
                previous_identity == identity
                and previous["sha256"].lower() == entry["sha256"].lower()
                and previous.get("size_bytes") == entry.get("size_bytes")
            )
            if not same_archive:
                raise ValueError("Candidate collides with an existing immutable archive")

    output = copy.deepcopy(base)
    combined = {identity: entry for identity, entry in baseline_entries.items() if identity not in identities}
    combined.update(candidate_entries)
    for kind, collection in KINDS.items():
        output["packages"][collection] = [
            copy.deepcopy(combined[identity])
            for identity in sorted(combined)
            if identity[0] == kind
        ]
    validate_store(output)
    return output


def reject_duplicate_keys(pairs):
    document = {}
    for key, value in pairs:
        if key in document:
            raise ValueError("Duplicate JSON object key")
        document[key] = value
    return document


def read_json(path):
    with path.open("rb") as stream:
        data = stream.read(MAX_MANIFEST_BYTES + 1)
    if len(data) > MAX_MANIFEST_BYTES:
        raise ValueError("JSON exceeds the 8 MiB manifest limit")
    return json.loads(data, object_pairs_hook=reject_duplicate_keys)


def assemble(base, candidate, owned, baseline_root, candidate_root, output, max_bytes):
    """Stage a complete immutable store, retaining verified unrelated archives.

    The caller declares its aggregate archive capacity in bytes. This operation
    does not download, authenticate a publisher, or update any remote pointer.
    """
    if type(max_bytes) is not int or max_bytes <= 0:
        raise ValueError('Store assembly requires a positive archive byte budget')
    manifest = compose(base, candidate, owned)
    ownership = validate_ownership(owned)
    entries, _ = validate_store(manifest)
    core = manifest['packages'].get('core')
    if core is not None:
        entries[('core', core['name'])] = core
    if output.exists() or output.is_symlink():
        raise ValueError('Store assembly requires a fresh output directory')
    output.parent.mkdir(parents=True, exist_ok=True)
    temporary = Path(tempfile.mkdtemp(prefix='.store-assembly-', dir=output.parent))
    total = 0
    try:
        for identity, entry in sorted(entries.items()):
            source_root = candidate_root if identity in ownership else baseline_root
            source = source_root / entry['tarball']
            if any(path.is_symlink() for path in (source, *source.parents)):
                raise ValueError('Store archive path contains a link')
            descriptor = os.open(source, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
            with os.fdopen(descriptor, 'rb') as reader:
                info = os.fstat(reader.fileno())
                if not stat.S_ISREG(info.st_mode):
                    raise ValueError('Store archive must be a regular file')
                if info.st_size <= 0 or info.st_size > max_bytes - total:
                    raise ValueError('Store archives exceed the declared byte budget')
                expected_size = entry.get('size_bytes', info.st_size)
                if info.st_size != expected_size:
                    raise ValueError('Store archive size differs from metadata')
                destination = temporary / entry['tarball']
                checksum = hashlib.sha256()
                copied = 0
                with destination.open('xb') as writer:
                    while block := reader.read(min(1 << 20, max_bytes - total + 1)):
                        total += len(block)
                        copied += len(block)
                        if total > max_bytes or copied > expected_size:
                            raise ValueError('Store archive grew beyond the declared byte budget or size')
                        checksum.update(block)
                        writer.write(block)
                if copied != expected_size or checksum.hexdigest() != entry['sha256'].lower():
                    raise ValueError('Store archive checksum or size differs from metadata')
        encoded = json.dumps(manifest, indent=2, sort_keys=True, allow_nan=False).encode() + b'\n'
        if len(encoded) > MAX_MANIFEST_BYTES:
            raise ValueError('Formatted store manifest exceeds 8 MiB')
        (temporary / 'packages-store-manifest.json').write_bytes(encoded)
        if output.exists() or output.is_symlink():
            raise ValueError('Store output appeared during assembly')
        temporary.rename(output)
    finally:
        if temporary.exists():
            shutil.rmtree(temporary)
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", type=Path, required=True)
    parser.add_argument("--candidate", type=Path, required=True)
    parser.add_argument("--owned", type=Path, required=True, help="Trusted operator JSON list of [kind, name] pairs")
    parser.add_argument("--out", type=Path, required=True, help="Fresh manifest path, or fresh store directory with archive options")
    parser.add_argument('--baseline-archives', type=Path)
    parser.add_argument('--candidate-archives', type=Path)
    parser.add_argument('--max-bytes', type=int)
    args = parser.parse_args()
    try:
        base, candidate, owned = read_json(args.base), read_json(args.candidate), read_json(args.owned)
        archive_options = (args.baseline_archives, args.candidate_archives, args.max_bytes)
        if any(value is not None for value in archive_options):
            if any(value is None for value in archive_options):
                raise ValueError('Archive assembly requires both archive roots and --max-bytes')
            assemble(base, candidate, owned, args.baseline_archives, args.candidate_archives, args.out, args.max_bytes)
            return
        output = compose(base, candidate, owned)
        encoded = json.dumps(output, indent=2, sort_keys=True, allow_nan=False).encode("utf-8") + b"\n"
        if len(encoded) > MAX_MANIFEST_BYTES:
            raise ValueError("Formatted output exceeds the 8 MiB manifest limit")
        with args.out.open("xb") as stream:
            stream.write(encoded)
    except (OSError, ValueError, TypeError, RecursionError) as error:
        parser.exit(1, f"Store composition failed: {error}\n")


if __name__ == "__main__":
    main()
