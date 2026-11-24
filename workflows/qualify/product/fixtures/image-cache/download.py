"""Fetch immutable qualification metadata and a read-only image cache entry from R2."""
from pathlib import Path
import argparse
from concurrent.futures import ThreadPoolExecutor
from hashlib import sha256
import json
import os
import re
import stat
import subprocess
import sys
from tempfile import TemporaryDirectory


sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "assets/scripts"))
from contracts import DIGEST, digest_file, matches, read_json, require
from chunk_codecs import CODECS


MAX_IMAGE_BYTES = 64 * 1024**3
MAX_MANIFEST_BYTES = 1024 * 1024
CHUNK_BYTES = 16 * 1024**2


def verify(path, expected, limit, size=None):
    require(path.resolve(strict=True) == path and stat.S_ISREG(path.lstat().st_mode)
            and path.stat().st_uid == os.getuid() and path.stat().st_mode & 0o222 == 0,
            "Expected a canonical, owner-controlled read-only cache file")
    require(size is None or path.stat().st_size == size, "Downloaded image byte count differs from the manifest")
    require(digest_file(path, limit) == expected, "R2 download SHA-256 differs from its pin")


def read_private_file(path):
    information = path.lstat()
    require(path.is_absolute() and path.resolve(strict=True) == path
            and stat.S_ISREG(information.st_mode) and information.st_uid == os.getuid()
            and information.st_mode & 0o077 == 0 and information.st_size <= 4096,
            "Select an absolute, owner-only local credential or endpoint file")


def chunk_references(storage, size):
    require(storage["layout"] == "fixed-sha256" and storage["chunk_bytes"] == CHUNK_BYTES,
            "Unknown image storage layout")
    references = storage["chunks"]
    count = (size + CHUNK_BYTES - 1) // CHUNK_BYTES
    require(isinstance(references, list) and len(references) == count,
            "Snapshot chunk count does not match the image size")
    for index, reference in enumerate(references):
        expected = min(CHUNK_BYTES, size - index * CHUNK_BYTES)
        require(matches(DIGEST, reference["sha256"]) and reference["bytes"] == expected
                and matches(DIGEST, reference["stored_sha256"]) and reference["codec"] in CODECS
                and type(reference["stored_bytes"]) is int and 0 < reference["stored_bytes"] <= expected,
                "Snapshot has an invalid chunk codec, digest or length")
        if reference["codec"] == "identity":
            require(reference["stored_sha256"] == reference["sha256"] and reference["stored_bytes"] == expected,
                    "An identity chunk must be stored byte-for-byte")
        else:
            require(reference["stored_bytes"] < expected, "Compressed chunks must use fewer bytes")
    return references


def fetch(arguments):
    read_private_file(arguments.reader)
    read_private_file(arguments.endpoint)
    endpoint = arguments.endpoint.read_text().strip()
    require(re.fullmatch(r"https://[0-9a-f]{32}\.r2\.cloudflarestorage\.com", endpoint),
            "Expected the confirmed Cloudflare R2 HTTPS origin")
    require(re.fullmatch(r"[a-z0-9][a-z0-9-]{1,62}[a-z0-9]", arguments.bucket),
            "Select an explicit R2 qualification bucket")
    require(matches(DIGEST, arguments.manifest_sha256), "Pin the SHA-256 of the image manifest")
    environment = {key: os.environ[key] for key in ("PATH", "LANG", "LC_ALL") if key in os.environ}
    environment.update(AWS_PROFILE="qualification-reader", AWS_CONFIG_FILE=os.devnull,
                       AWS_SHARED_CREDENTIALS_FILE=str(arguments.reader), AWS_REGION="auto",
                       AWS_EC2_METADATA_DISABLED="true", AWS_PAGER="")
    cache = arguments.cache.absolute()
    cache.mkdir(mode=0o700, parents=True, exist_ok=True)
    require(cache == cache.resolve(strict=True) and cache.stat().st_uid == os.getuid()
            and cache.stat().st_mode & 0o077 == 0, "Select a private cache directory")

    def store(key, expected, limit, target, size=None):
        if not target.exists():
            with TemporaryDirectory(dir=cache) as staging:
                partial = Path(staging) / "download"
                # A full object fetch is required. HEAD metadata alone cannot establish the file hash.
                command = ["aws", "--endpoint-url", endpoint, "s3api", "get-object",
                           "--bucket", arguments.bucket, "--key", key, str(partial)]
                result = subprocess.run(command, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL,
                                        stderr=subprocess.PIPE, env=environment, timeout=900)
                require(result.returncode == 0, "R2 GetObject failed using the qualification reader profile")
                partial.chmod(0o400)
                verify(partial, expected, limit, size)
                try:
                    os.link(partial, target)
                except FileExistsError:
                    pass
        verify(target, expected, limit, size)

    manifest_digest = arguments.manifest_sha256
    manifest_path = cache / f"{manifest_digest}.json"
    store(f"manifests/sha256/{manifest_digest}.json", manifest_digest, MAX_MANIFEST_BYTES, manifest_path)
    manifest = read_json(manifest_path)
    require(manifest["schema"] == "dockpipe.qualification-image.v2", "Expected a chunked qualification snapshot")
    image = manifest["image"]
    digest = image["sha256"]
    require(matches(DIGEST, digest) and image["format"] == "qcow2"
            and type(image["bytes"]) is int and 0 < image["bytes"] <= MAX_IMAGE_BYTES,
            "Image metadata cannot be admitted to the SHA-256 cache")
    image_path = cache / f"{digest}.qcow2"
    references = chunk_references(manifest["storage"], image["bytes"])
    if not image_path.exists():
        parts = {entry["stored_sha256"]: entry["stored_bytes"] for entry in references}

        def fetch_part(item):
            chunk, length = item
            path = cache / f"{chunk}.chunk"
            store(f"chunks/sha256/{chunk}", chunk, CHUNK_BYTES, path, length)

        with ThreadPoolExecutor(max_workers=4) as workers:
            for _ in workers.map(fetch_part, parts.items()):
                pass
        with TemporaryDirectory(dir=cache) as staging:
            reconstructed = Path(staging) / "reconstructed.qcow2"
            with reconstructed.open("xb") as output:
                for entry in references:
                    part = cache / f'{entry["stored_sha256"]}.chunk'
                    decoded = CODECS[entry["codec"]].decode(part, entry["bytes"])
                    require(len(decoded) == entry["bytes"] and sha256(decoded).hexdigest() == entry["sha256"],
                            "Decoded chunk identity differs from the snapshot")
                    output.write(decoded)
            reconstructed.chmod(0o400)
            verify(reconstructed, digest, MAX_IMAGE_BYTES, image["bytes"])
            try:
                os.link(reconstructed, image_path)
            except FileExistsError:
                pass
    verify(image_path, digest, MAX_IMAGE_BYTES, image["bytes"])
    print(json.dumps({"manifest": str(manifest_path), "base": str(image_path),
                      "sha256": digest, "chunks": len(references)}))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--reader", required=True, type=Path)
    parser.add_argument("--endpoint", required=True, type=Path)
    parser.add_argument("--bucket", required=True)
    parser.add_argument("--manifest-sha256", required=True)
    parser.add_argument("--cache", required=True, type=Path)
    os.umask(0o077)
    fetch(parser.parse_args())
