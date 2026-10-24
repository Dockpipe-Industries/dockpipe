#!/usr/bin/env python3
"""Publish one qualified native Display SDK as an immutable prerelease."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess


SPEC = importlib.util.spec_from_file_location(
    "launcher_candidate_publication", Path(__file__).with_name("publish-launcher-sdk.py"))
SHARED = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SHARED)
REPOSITORY = SHARED.REPOSITORY
PLATFORMS = {"linux-amd64", "windows-amd64", "darwin-amd64", "darwin-arm64"}
VERSION = "1.0.0"
MAX_ARCHIVE_BYTES = 4 << 20
MAX_METADATA_BYTES = 128 << 10


def archive_name(target):
    if target not in PLATFORMS:
        raise ValueError("Display SDK platform has no candidate qualification lane")
    return f"DockPipeDisplaySDK-{VERSION}-{target}.tar.gz"


def validate(directory, source_sha, target):
    archive = archive_name(target)
    names = {archive, "sdk-files.json", "sdk-release.json"}
    if directory.is_symlink() or {path.name for path in directory.iterdir()} != names:
        raise ValueError("Display SDK delivery has missing, linked or unexpected assets")
    assets = {}
    for name in sorted(names):
        path = directory / name
        maximum = MAX_ARCHIVE_BYTES if name == archive else MAX_METADATA_BYTES
        if path.is_symlink() or not path.is_file() or not 0 < path.stat().st_size <= maximum:
            raise ValueError("Display SDK delivery requires bounded regular files")
        assets[name] = {"size": path.stat().st_size, "digest": "sha256:" + SHARED.digest(path)}
    metadata = json.loads((directory / "sdk-release.json").read_text(encoding="utf-8"))
    required = {
        "schema": 1, "product": "DockPipeDisplaySDK", "version": VERSION,
        "platform": target, "qt_version": "6.10.3",
        "interface": "com.dockpipe.launcher.DisplayPackage/1.0",
        "source_repository": REPOSITORY, "source_sha": source_sha,
        "source_state": "committed-snapshot",
    }
    if not isinstance(metadata, dict) or any(metadata.get(key) != value for key, value in required.items()):
        raise ValueError("Display SDK source, platform or interface differs from the publishing run")
    if not re.fullmatch(r"[0-9a-f]{64}", metadata.get("source_digest", "")):
        raise ValueError("Display SDK source digest is missing or invalid")
    receipt = metadata.get("archive", {})
    manifest = metadata.get("file_manifest", {})
    compiler = metadata.get("compiler", {})
    if not all(isinstance(value, dict) for value in (receipt, manifest, compiler)):
        raise ValueError("Display SDK provenance records must be JSON objects")
    if (receipt.get("name") != archive or type(receipt.get("size_bytes")) is not int
            or receipt["size_bytes"] != assets[archive]["size"]
            or "sha256:" + receipt.get("sha256", "") != assets[archive]["digest"]):
        raise ValueError("Display SDK archive differs from its qualification receipt")
    if (manifest.get("name") != "sdk-files.json"
            or "sha256:" + manifest.get("sha256", "") != assets["sdk-files.json"]["digest"]):
        raise ValueError("Display SDK inventory differs from its qualification receipt")
    required_checks = {"sdk-component-install", "relocated-external-consumer-compile-link", "native-consumer-run"}
    if not required_checks.issubset(metadata.get("checks", [])):
        raise ValueError("Display SDK install, relocation and native execution checks are required")
    if any(not isinstance(compiler.get(key), str) or not 0 < len(compiler[key]) <= 128
           for key in ("id", "version")):
        raise ValueError("Display SDK compiler provenance is missing or unbounded")
    return assets


def publish(directory, target):
    source_sha = os.environ.get("GITHUB_SHA", "")
    if (os.environ.get("GITHUB_REPOSITORY") != REPOSITORY
            or os.environ.get("GITHUB_REF") != "refs/heads/dev"
            or os.environ.get("GITHUB_EVENT_NAME") != "push"
            or not re.fullmatch(r"[0-9a-f]{40}", source_sha)):
        raise ValueError("Display SDK publication requires an exact accepted dev push")
    assets = validate(directory, source_sha, target)
    tag = f"display-sdk-{VERSION}-{target}-staging.{source_sha}"
    query = """query($owner:String!,$name:String!,$ref:String!){
      repository(owner:$owner,name:$name){ref(qualifiedName:$ref){target{
        __typename oid ... on Tag {target{
          __typename oid ... on Tag {target{__typename oid}}
        }}
      }}}
    }"""
    query_arguments = [
        "gh", "api", "graphql", "-f", "query=" + query,
        "-f", "owner=Dockpipe-Industries", "-f", "name=dockpipe", "-f", "ref=refs/tags/" + tag,
    ]

    def verify_reference(allow_missing):
        state = json.loads(SHARED.run(query_arguments))
        if state.get("errors"):
            raise ValueError("Could not verify the Display SDK candidate tag")
        SHARED.verify_tag(state["data"]["repository"]["ref"], source_sha, allow_missing=allow_missing)

    release = SHARED.find_release(tag)
    verify_reference(release is None or release.get("draft") is True)
    if release is None:
        release = SHARED.create_release(
            tag, source_sha, f"Display SDK {VERSION} {target} candidate",
            "Qualified relocated native QWidget display interface SDK. "
            "No Launcher UI runtime, Dockpipe application release or production channel update.")
    release = SHARED.reconcile_assets(directory, release, tag, source_sha, assets)
    verify_reference(release.get("draft") is True)
    if release.get("draft") is True:
        SHARED.run(["gh", "release", "edit", tag, "--repo", REPOSITORY,
                    "--draft=false", "--prerelease", "--latest=false"])
    final = SHARED.read_release(release["id"], tag)
    SHARED.verify_release(final, tag, source_sha, assets)
    if final.get("draft") is not False:
        raise ValueError("Display SDK candidate remains a draft")
    verify_reference(False)
    print(final["html_url"])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--directory", type=Path, required=True)
    parser.add_argument("--platform", choices=sorted(PLATFORMS), required=True)
    arguments = parser.parse_args()
    try:
        publish(arguments.directory, arguments.platform)
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        parser.exit(1, f"Display SDK candidate publication failed: {error}\n")


if __name__ == "__main__":
    main()
