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


def publish(artifacts, apt, version, dry_run, candidate=""):
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", version):
        raise ValueError("Expected a release version X.Y.Z")
    bucket = os.environ.get("DOCKPIPE_RELEASE_BUCKET") or os.environ.get("R2_BUCKET")
    endpoint = os.environ.get("R2_ENDPOINT_URL")
    if not bucket or not endpoint or not endpoint.startswith("https://"):
        raise ValueError("DOCKPIPE_RELEASE_BUCKET and HTTPS R2_ENDPOINT_URL are required")
    if candidate:
        if bucket != "dockpipe-staging":
            raise ValueError("Staging publication requires dockpipe-staging")
        if not re.fullmatch(re.escape(version) + r"-staging\.[0-9]+\.[0-9]+\.[0-9a-f]{12}", candidate):
            raise ValueError("Invalid staging candidate identity")
    elif bucket == "dockpipe-staging":
        raise ValueError("The staging bucket requires a candidate identity")
    prefix = os.environ.get("R2_PREFIX", "packages").strip("/")
    if candidate and prefix != "packages":
        raise ValueError("Staging requires the packages prefix")
    directory = "candidates" if candidate else "releases"
    identity = candidate or version
    suite = "staging" if candidate else "stable"
    version_prefix = "/".join(part for part in (prefix, directory, identity) if part)
    if not (apt / f"dists/{suite}/InRelease").is_file():
        raise ValueError("Signed APT repository is missing")
    if not (artifacts / "release-manifest.json").is_file():
        raise ValueError("Release catalog is missing")
    if candidate:
        catalog = json.loads((artifacts / "release-manifest.json").read_text())
        sha = catalog.get("source_sha", "")
        if (catalog.get("candidate") != candidate or catalog.get("channel") != "staging"
                or catalog.get("version") != version or not re.fullmatch(r"[0-9a-f]{40}", sha)
                or not candidate.endswith(sha[:12])):
            raise ValueError("Staging catalog does not match the candidate")
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
        # Candidate APT pools must be immutable too: native versions can repeat.
        apt_prefix = f"{version_prefix}/apt" if candidate else "apt"
        upload(path, f"{apt_prefix}/{path.relative_to(apt).as_posix()}", bucket, endpoint, dry_run)
    # The catalog is the commit marker for a complete, immutable release.
    upload(artifacts / "release-manifest.json", catalog_key, bucket, endpoint, dry_run)
    latest = artifacts.parent / "latest.json"
    pointer = {"version": version, "manifest": f"{version_prefix}/release-manifest.json"}
    if candidate:
        pointer.update(channel="staging", candidate=candidate)
    latest.write_text(json.dumps(pointer) + "\n")
    upload(latest, "/".join(part for part in (prefix, "latest.json") if part), bucket, endpoint, dry_run)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--artifacts", type=Path, required=True)
    parser.add_argument("--apt", type=Path, required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--candidate", default="")
    args = parser.parse_args()
    publish(args.artifacts, args.apt, args.version, args.dry_run, args.candidate)
