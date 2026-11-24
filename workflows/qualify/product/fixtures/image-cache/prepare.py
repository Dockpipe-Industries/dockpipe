"""Prepare immutable R2 chunks and reconstruction manifests for clean VM snapshots."""
from hashlib import sha256
from pathlib import Path
import argparse
import json
import os
import stat
import subprocess
import sys


sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "assets/scripts"))
from contracts import DIGEST, IDENTIFIER, digest_file, matches, read_json, require, write_json
from chunk_codecs import CODECS


CHUNK_BYTES = 16 * 1024**2
MAX_IMAGE_BYTES = 64 * 1024**3


def bulk_entry(path, key, digest, metadata=False):
    return {
        "source": str(path), "key": key, "size": path.stat().st_size, "sha256": digest,
        "content_type": "application/json" if metadata else "application/octet-stream",
        "cache_control": "private, max-age=31536000, immutable", "publish_last": metadata,
    }


def verify_standalone_base(base):
    result = subprocess.run(["qemu-img", "info", "--output=json", str(base)], stdin=subprocess.DEVNULL,
                            stdout=subprocess.PIPE, check=True, timeout=15)
    info = json.loads(result.stdout)
    require(info["format"] == "qcow2" and not info.get("backing-filename") and not info.get("snapshots")
            and not info.get("dirty-flag") and not info["format-specific"]["data"].get("corrupt"),
            "Only standalone clean QCOW2 bases can be prepared for upload")


def prepare(source_pairs, output, preferred_codec):
    output = output.absolute()
    output.mkdir(mode=0o700, parents=True)
    chunks = output / "chunks"
    chunks.mkdir(mode=0o700)
    objects = {}
    prepared_chunks = {}
    snapshots = []

    for profile_path, base in source_pairs:
        profile = read_json(profile_path)
        require(profile["schema"] in {"dockpipe.qualification-image.v1", "dockpipe.qualification-image.v2"}
                and matches(IDENTIFIER, profile["name"]), "Select a named image profile")
        manifest = profile["image"]
        base = base.absolute()
        before = base.lstat()
        require(stat.S_ISREG(before.st_mode) and base.resolve(strict=True) == base
                and before.st_mode & 0o222 == 0
                and 0 < before.st_size <= MAX_IMAGE_BYTES and before.st_size == manifest["bytes"]
                and manifest["format"] == "qcow2" and matches(DIGEST, manifest["sha256"]),
                "Select the verified, read-only QCOW2 base described by the profile")
        verify_standalone_base(base)

        image_hash = sha256()
        references = []
        with base.open("rb") as stream:
            for data in iter(lambda: stream.read(CHUNK_BYTES), b""):
                image_hash.update(data)
                digest = sha256(data).hexdigest()
                if digest in prepared_chunks:
                    references.append(prepared_chunks[digest])
                    continue

                encoded = CODECS[preferred_codec].encode(data)
                codec = preferred_codec
                if len(encoded) >= len(data):
                    encoded = data
                    codec = "identity"
                stored_digest = sha256(encoded).hexdigest()
                key = f"chunks/sha256/{stored_digest}"
                if key not in objects:
                    part = chunks / stored_digest
                    with part.open("xb") as file:
                        file.write(encoded)
                    part.chmod(0o400)
                    objects[key] = bulk_entry(part, key, stored_digest)
                reference = {"sha256": digest, "bytes": len(data), "codec": codec,
                             "stored_sha256": stored_digest, "stored_bytes": len(encoded)}
                prepared_chunks[digest] = reference
                references.append(reference)

        after = base.lstat()
        require((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns)
                == (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns)
                and image_hash.hexdigest() == manifest["sha256"],
                "Clean image SHA-256 or identity changed during snapshot preparation")

        manifest.pop("key", None)
        profile["schema"] = "dockpipe.qualification-image.v2"
        profile["storage"] = {"layout": "fixed-sha256", "chunk_bytes": CHUNK_BYTES, "chunks": references}
        path = output / f'{profile["name"]}.json'
        write_json(path, profile)
        path.chmod(0o400)
        digest = digest_file(path)
        key = f"manifests/sha256/{digest}.json"
        objects[key] = bulk_entry(path, key, digest, metadata=True)
        snapshots.append({"name": profile["name"], "manifest_sha256": digest,
                          "image_sha256": manifest["sha256"], "chunks": len(references)})

    write_json(output / "bulk.json", {"schema": 1, "objects": list(objects.values())})
    write_json(output / "snapshots.json", {"snapshots": snapshots,
                                          "unique_objects": len(objects),
                                          "stored_bytes": sum(entry["size"] for entry in objects.values())})


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image", action="append", required=True, nargs=2, type=Path,
                        metavar=("PROFILE", "READONLY_BASE"))
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--codec", choices=CODECS, default="zstd")
    arguments = parser.parse_args()
    os.umask(0o077)
    prepare(arguments.image, arguments.output, arguments.codec)
