#!/usr/bin/env python3
"""Publish versioned packages first, then APT metadata. Never delete old versions."""
import argparse
import json
import mimetypes
import os
from pathlib import Path
import re
import subprocess


def upload(source, key, bucket, endpoint, dry_run):
    mutable = source.name in {"InRelease", "Release", "Release.gpg", "Packages", "Packages.gz", "latest.json"}
    cache = "no-cache" if mutable else "public, max-age=31536000, immutable"
    if source.suffix in {".sh", ".ps1", ".gpg"} or source.name == "dockpipe-archive-keyring.gpg":
        cache = "no-cache"
    content_type = mimetypes.guess_type(source.name)[0] or "application/octet-stream"
    command = ["aws", "s3", "cp", str(source), f"s3://{bucket}/{key}",
               "--endpoint-url", endpoint, "--region", "auto", "--only-show-errors",
               "--content-type", content_type, "--cache-control", cache]
    if dry_run:
        print(f"{source} -> s3://{bucket}/{key}")
    else:
        subprocess.run(command, check=True)


def require_unpublished(bucket, endpoint, key):
    result = subprocess.run(["aws", "s3api", "head-object", "--bucket", bucket, "--key", key,
                             "--endpoint-url", endpoint, "--region", "auto"], capture_output=True, text=True)
    if result.returncode == 0:
        raise ValueError("This release is already published; advance VERSION instead of overwriting it")
    if "(404)" not in result.stderr and "(NoSuchKey)" not in result.stderr:
        raise ValueError("Could not verify publication state; refusing to write (provider output withheld)")


def publish(artifacts, apt, version, dry_run):
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", version):
        raise ValueError("Expected a release version X.Y.Z")
    bucket = os.environ.get("DOCKPIPE_RELEASE_BUCKET") or os.environ.get("R2_BUCKET")
    endpoint = os.environ.get("R2_ENDPOINT_URL")
    if not bucket or not endpoint or not endpoint.startswith("https://"):
        raise ValueError("DOCKPIPE_RELEASE_BUCKET and HTTPS R2_ENDPOINT_URL are required")
    prefix = os.environ.get("R2_PREFIX", "packages").strip("/")
    version_prefix = "/".join(part for part in (prefix, "releases", version) if part)
    if not (apt / "dists/stable/InRelease").is_file():
        raise ValueError("Signed APT repository is missing")
    if not (artifacts / "release-manifest.json").is_file():
        raise ValueError("Release catalog is missing")
    # Inspect the entire input before the first remote write.
    for directory in (artifacts, apt):
        for path in directory.rglob("*"):
            if path.is_symlink():
                raise ValueError(f"Refusing symlink: {path}")
    catalog_key = f"{version_prefix}/release-manifest.json"
    if not dry_run:
        require_unpublished(bucket, endpoint, catalog_key)
    for path in sorted(artifacts.rglob("*")):
        if path.is_symlink():
            raise ValueError(f"Refusing symlink: {path}")
        if path.is_file() and path != artifacts / "release-manifest.json":
            upload(path, f"{version_prefix}/{path.relative_to(artifacts).as_posix()}", bucket, endpoint, dry_run)
    # Pool files and by-hash indexes must exist before the signed current index changes.
    apt_files = [path for path in apt.rglob("*") if path.is_file()]
    order = {"Release": 1, "Release.gpg": 2, "InRelease": 3}
    for path in sorted(apt_files, key=lambda path: (order.get(path.name, 0), str(path))):
        if path.is_symlink():
            raise ValueError(f"Refusing symlink: {path}")
        upload(path, f"apt/{path.relative_to(apt).as_posix()}", bucket, endpoint, dry_run)
    # The catalog is the commit marker for a complete, immutable release.
    upload(artifacts / "release-manifest.json", catalog_key, bucket, endpoint, dry_run)
    latest = artifacts.parent / "latest.json"
    latest.write_text(json.dumps({"version": version, "manifest": f"{version_prefix}/release-manifest.json"}) + "\n")
    upload(latest, "/".join(part for part in (prefix, "latest.json") if part), bucket, endpoint, dry_run)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--artifacts", type=Path, required=True)
    parser.add_argument("--apt", type=Path, required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()
    publish(args.artifacts, args.apt, args.version, args.dry_run)
