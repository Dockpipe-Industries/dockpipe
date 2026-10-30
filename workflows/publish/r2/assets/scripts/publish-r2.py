#!/usr/bin/env python3
"""Publish runtime artifacts and APT without changing the optional package catalog."""
import argparse
import hashlib
import json
import mimetypes
import os
from pathlib import Path
import re
import subprocess


def object_headers(source):
    mutable = source.name in {"InRelease", "Release", "Release.gpg", "Packages", "Packages.gz", "latest.json"}
    cache = "no-cache" if mutable else "public, max-age=31536000, immutable"
    if source.suffix in {".sh", ".ps1", ".gpg"} or source.name == "dockpipe-archive-keyring.gpg":
        cache = "no-cache"
    content_type = mimetypes.guess_type(source.name)[0] or "application/octet-stream"
    return content_type, cache


def upload(source, key, bucket, endpoint, dry_run):
    content_type, cache = object_headers(source)
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


def require_forward_version(bucket, endpoint, key, version):
    result = subprocess.run([
        "aws", "s3", "cp", f"s3://{bucket}/{key}", "-", "--endpoint-url", endpoint,
        "--region", "auto", "--only-show-errors",
    ], capture_output=True, text=True)
    if result.returncode:
        if "(404)" in result.stderr or "(NoSuchKey)" in result.stderr:
            return
        raise ValueError("Could not read the current release version; refusing publication")
    current = json.loads(result.stdout).get("version", "")
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", current):
        raise ValueError("Current release pointer has an invalid version")
    if tuple(map(int, version.split("."))) <= tuple(map(int, current.split("."))):
        raise ValueError("Native release version must advance before publishing")


def prepare(artifacts, apt, version, dry_run, candidate="", plan_directory=None):
    artifacts = artifacts.resolve(strict=True)
    apt = apt.resolve(strict=True)
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", version):
        raise ValueError("Expected a release version X.Y.Z")
    bucket = os.environ.get("DOCKPIPE_RELEASE_BUCKET") or os.environ.get("R2_BUCKET")
    endpoint = os.environ.get("R2_ENDPOINT_URL", "").rstrip("/")
    if not bucket or not endpoint or not endpoint.startswith("https://"):
        raise ValueError("DOCKPIPE_RELEASE_BUCKET and HTTPS R2_ENDPOINT_URL are required")
    channel = os.environ.get("DOCKPIPE_RELEASE_CHANNEL", "")
    expected_channel = "staging" if candidate else "stable"
    if channel != expected_channel:
        raise ValueError(f"Set DOCKPIPE_RELEASE_CHANNEL={expected_channel} for this publication")
    if candidate:
        if not re.fullmatch(re.escape(version) + r"-staging\.[0-9]+\.[0-9]+\.[0-9a-f]{12}", candidate):
            raise ValueError("Invalid staging candidate identity")
    prefix = os.environ.get("R2_PREFIX", "packages").strip("/")
    if candidate and prefix != "packages":
        raise ValueError("Staging requires the packages prefix")
    directory = "candidates" if candidate else "releases"
    identity = candidate or version
    suite = "staging" if candidate else "stable"
    version_prefix = "/".join(part for part in (prefix, directory, identity) if part)
    runtime_pointer_key = "/".join(part for part in (prefix, "runtime", "latest.json") if part)
    if not (apt / f"dists/{suite}/InRelease").is_file():
        raise ValueError("Signed APT repository is missing")
    if not (artifacts / "release-manifest.json").is_file():
        raise ValueError("Release catalog is missing")
    catalog = json.loads((artifacts / "release-manifest.json").read_text())
    if catalog.get("artifact_class") != "runtime":
        raise ValueError("Runtime publication requires an independent runtime release catalog")
    if candidate:
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
        require_forward_version(bucket, endpoint, runtime_pointer_key, version)
    payloads = []
    metadata = []

    def retain(path, key, immutable):
        digest = hashlib.sha256()
        with path.open("rb") as source:
            for block in iter(lambda: source.read(1024 * 1024), b""):
                digest.update(block)
        if immutable:
            content_type, cache_control = object_headers(path)
            payloads.append({"source": str(path), "key": key, "size": path.stat().st_size,
                             "sha256": digest.hexdigest(), "publish_last": False, "content_type": content_type,
                             "cache_control": cache_control})
        else:
            metadata.append({"source": str(path), "key": key, "size": path.stat().st_size,
                             "sha256": digest.hexdigest()})

    for path in sorted(artifacts.rglob("*")):
        if path.is_file() and path != artifacts / "release-manifest.json":
            retain(path, f"{version_prefix}/{path.relative_to(artifacts).as_posix()}", True)
    apt_files = [path for path in apt.rglob("*") if path.is_file()]
    order = {"Release": 1, "Release.gpg": 2, "InRelease": 3}

    def retain_apt(apt_prefix):
        for path in sorted(apt_files, key=lambda path: (order.get(path.name, 0), str(path))):
            relative = path.relative_to(apt)
            immutable = relative.parts[0] == "pool" or "by-hash" in relative.parts
            retain(path, f"{apt_prefix}/{relative.as_posix()}", immutable)

    retain_apt(f"{version_prefix}/apt" if candidate else "apt")
    # Preserve the original commit order, including the candidate catalog before
    # permanent APT metadata. Every immutable payload is verified before this list.
    retain(artifacts / "release-manifest.json", catalog_key, False)
    if candidate:
        retain_apt("apt")
    latest = artifacts.parent / "latest.json"
    pointer = {"artifact_class": "runtime", "version": version,
               "manifest": f"{version_prefix}/release-manifest.json"}
    if candidate:
        pointer.update(channel="staging", candidate=candidate)
    latest.write_text(json.dumps(pointer) + "\n")
    retain(latest, runtime_pointer_key, False)
    if plan_directory is None and os.environ.get("DOCKPIPE_BIN"):
        plan_directory = Path(subprocess.check_output(
            [os.environ["DOCKPIPE_BIN"], "scope", "artifacts", "r2-publication"], text=True).strip())
    plan_directory = plan_directory or artifacts.parent / "r2-bulk"
    plan_directory.mkdir(parents=True, exist_ok=True)
    manifest = plan_directory / "objects.json"
    manifest.write_text(json.dumps({"schema": 1, "objects": payloads}, indent=2) + "\n")
    identity_input = {"endpoint": endpoint, "bucket": bucket, "objects": payloads, "trusted": {}}
    bulk_identity = hashlib.sha256(json.dumps(identity_input, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    plan = {"bulk_identity": bulk_identity, "bucket": bucket, "endpoint": endpoint, "dry_run": dry_run,
            "manifest": str(manifest), "state": str(plan_directory / "state"), "metadata": metadata}
    plan_path = plan_directory / "publication.json"
    plan_path.write_text(json.dumps(plan, indent=2) + "\n")
    outputs = os.environ.get("DOCKPIPE_STEP_OUTPUTS_FILE")
    if not outputs and os.environ.get("DOCKPIPE_BIN"):
        outputs = subprocess.check_output(
            [os.environ["DOCKPIPE_BIN"], "scope", "artifacts", "publication.env"], text=True).strip()
    if outputs:
        environment = {"R2_BUCKET": bucket, "R2_ENDPOINT_URL": endpoint,
                       "R2_BULK_MANIFEST": str(manifest), "R2_BULK_STATE": plan["state"],
                       "R2_BULK_DRY_RUN": str(dry_run).lower(), "R2_BULK_SKIP": "false",
                       "R2_BULK_TRUSTED_RECEIPTS": "",
                       "DOCKPIPE_R2_PUBLICATION_PLAN": str(plan_path)}
        if any("\n" in value or "\r" in value for value in environment.values()):
            raise ValueError("Publication configuration cannot contain newlines")
        Path(outputs).write_text("".join(f"{key}={value}\n" for key, value in environment.items()))
    return plan


def publish_metadata(plan):
    if not plan["dry_run"]:
        # Only the package's final verified phase authorizes commit markers.
        state = json.loads(Path(plan["state"]).read_text())
        if (state.get("schema") != 1 or state.get("phase") != "verified-head"
                or state.get("bucket") != plan["bucket"] or state.get("endpoint") != plan["endpoint"]
                or state.get("identity") != plan["bulk_identity"]):
            raise ValueError("Bulk payload verification must complete before publishing metadata")
    # Freeze the entire metadata set across the asynchronous bulk stage before
    # publishing its first index, signature, catalog or channel pointer.
    for entry in plan["metadata"]:
        source = Path(entry["source"])
        digest = hashlib.sha256()
        with source.open("rb") as stream:
            for block in iter(lambda: stream.read(1024 * 1024), b""):
                digest.update(block)
        if source.stat().st_size != entry["size"] or digest.hexdigest() != entry["sha256"]:
            raise ValueError("Publication metadata changed after bulk preparation")
    for entry in plan["metadata"]:
        upload(Path(entry["source"]), entry["key"], plan["bucket"], plan["endpoint"], plan["dry_run"])



if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--phase", choices=("prepare", "metadata"), required=True)
    parser.add_argument("--artifacts", type=Path, default=Path("release/artifacts"))
    parser.add_argument("--apt", type=Path, default=Path("release/apt"))
    parser.add_argument("--version", default=os.environ.get("DOCKPIPE_ADAPTER_0", ""))
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--candidate", default="")
    parser.add_argument("--staging", action="store_true")
    args = parser.parse_args()
    if args.phase == "metadata":
        publish_metadata(json.loads(Path(os.environ["DOCKPIPE_R2_PUBLICATION_PLAN"]).read_text()))
    else:
        candidate = os.environ.get("DOCKPIPE_ADAPTER_1", "") if args.staging else args.candidate
        prepare(args.artifacts, args.apt, args.version, args.dry_run, candidate)
