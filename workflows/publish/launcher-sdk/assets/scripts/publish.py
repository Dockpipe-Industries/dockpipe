#!/usr/bin/env python3
"""Publish the qualified host SDK as an immutable candidate, without channel promotion."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess


ARCHIVE = "DockPipeLauncherSDK-1.1.0-Linux-amd64.tar.gz"
ASSETS = {ARCHIVE, "sdk-files.json", "sdk-release.json"}
MAX_ASSET_BYTES = 512 << 20
ROOT = Path(__file__).resolve().parents[5]

SPEC = importlib.util.spec_from_file_location(
    "SHARED", Path(__file__).resolve().parents[5] / "workflows/publish/shared/github_release.py")
SHARED = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SHARED)
SOURCE_REPOSITORY = SHARED.SOURCE_REPOSITORY

digest = SHARED.digest


def browser_dependency():
    helper_path = ROOT / "workflows/dependencies/browser-sdk/assets/scripts/prepare-browser-sdk.py"
    spec = importlib.util.spec_from_file_location("publisher_browser_preparer", helper_path)
    helper = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(helper)
    lock, _ = helper.load_lock(ROOT / "workflows/dependencies/browser-sdk/assets/spatialweb-browser.lock.json", "linux-amd64")
    return lock


def validate(directory, source_sha):
    if {path.name for path in directory.iterdir()} != ASSETS:
        raise ValueError("SDK delivery has missing or unexpected assets")
    assets = {}
    for name in sorted(ASSETS):
        path = directory / name
        if path.is_symlink() or not path.is_file() or not 0 < path.stat().st_size <= MAX_ASSET_BYTES:
            raise ValueError("SDK delivery requires bounded regular files")
        if name.endswith(".json") and path.stat().st_size > 2 << 20:
            raise ValueError("SDK metadata exceeds capacity")
        assets[name] = {"size": path.stat().st_size, "digest": "sha256:" + digest(path)}
    metadata = json.loads((directory / "sdk-release.json").read_text())
    required = {
        "schema": 1, "product": "DockPipeLauncherSDK", "version": "1.1.0",
        "platform": "linux-amd64", "qt_version": "6.10.3",
        "source_repository": SOURCE_REPOSITORY, "source_sha": source_sha,
        "source_state": "committed-snapshot",
    }
    if not isinstance(metadata, dict) or any(metadata.get(key) != value for key, value in required.items()):
        raise ValueError("SDK source and ABI identity differ from the publishing run")
    browser = browser_dependency()
    if (metadata.get("dependencies") != {"spatialweb_browser": browser}
            or metadata.get("compiler") != browser["compiler"]
            or metadata.get("abi") != {"cxx_standard": 17, "workspace_interface": "com.dockpipe.launcher.WorkspacePackage/2.0",
                                       "display_interface": "com.dockpipe.launcher.DisplayPackage/1.0"}
            or not isinstance(metadata.get("source_digest"), str)
            or not re.fullmatch(r"[0-9a-f]{64}", metadata["source_digest"])):
        raise ValueError("SDK Browser dependency, compiler or workspace interface differs from the committed ABI")
    archive = metadata.get("archive", {})
    if (not isinstance(archive, dict) or archive.get("name") != ARCHIVE
            or archive.get("size_bytes") != assets[ARCHIVE]["size"]
            or "sha256:" + archive.get("sha256", "") != assets[ARCHIVE]["digest"]):
        raise ValueError("SDK archive differs from its qualification receipt")
    manifest = metadata.get("file_manifest", {})
    if (not isinstance(manifest, dict) or manifest.get("name") != "sdk-files.json"
            or "sha256:" + manifest.get("sha256", "") != assets["sdk-files.json"]["digest"]):
        raise ValueError("SDK inventory differs from its qualification receipt")
    required_checks = {"sdk-component-install", "shared-runtime-symbol-cleanup", "relocated-external-consumer-compile-link"}
    if (not isinstance(metadata.get("checks"), list)
            or not all(isinstance(check, str) for check in metadata["checks"])
            or not required_checks.issubset(metadata["checks"])):
        raise ValueError("SDK install and external-consumer checks are required")
    return assets


def publish(directory, repository=None):
    repository = SHARED.publication_target(repository)
    source_sha = os.environ.get("GITHUB_SHA", "")
    if (os.environ.get("GITHUB_REPOSITORY") != SOURCE_REPOSITORY
            or os.environ.get("GITHUB_REF") != "refs/heads/dev"
            or os.environ.get("GITHUB_EVENT_NAME") != "push"
            or not re.fullmatch(r"[0-9a-f]{40}", source_sha)):
        raise ValueError("SDK publication requires an exact accepted dev push")
    assets = validate(directory, source_sha)
    tag = "launcher-sdk-1.1.0-staging." + source_sha
    # GraphQL establishes the tag target; the REST inventory below discovers
    # drafts without treating a tag-endpoint 404 as release absence.
    query = """query($owner:String!,$name:String!,$tag:String!,$ref:String!){
      repository(owner:$owner,name:$name){
        release(tagName:$tag){tagName}
        ref(qualifiedName:$ref){target{
          __typename oid ... on Tag {target{
            __typename oid ... on Tag {target{__typename oid}}
          }}
        }}
      }
    }"""
    query_arguments = [
        "gh", "api", "graphql", "-f", "query=" + query, "-f", "owner=" + repository.split("/")[0],
        "-f", "name=" + repository.split("/")[1], "-f", "tag=" + tag, "-f", "ref=refs/tags/" + tag,
    ]
    existing = json.loads(SHARED.run(query_arguments))
    if existing.get("errors"):
        raise ValueError("Could not establish whether candidate already exists")
    release = SHARED.find_release(repository, tag)
    SHARED.verify_tag(existing["data"]["repository"]["ref"], source_sha,
               allow_missing=release is None or release.get("draft") is True)
    if release is None:
        release = SHARED.create_release(
            repository, tag, source_sha, "Launcher SDK 1.1.0 candidate",
            "Qualified installed host SDK for SpatialPipe/Cockpit candidate builds. "
            "No Dockpipe application release or production channel update.")
    release = SHARED.reconcile_assets(repository, directory, release, tag, source_sha, assets)
    tag_state = json.loads(SHARED.run(query_arguments))
    if tag_state.get("errors"):
        raise ValueError("Could not verify candidate tag")
    # GitHub may defer creating a new tag until the draft is published.
    SHARED.verify_tag(tag_state["data"]["repository"]["ref"], source_sha,
               allow_missing=release["draft"])
    if release["draft"]:
        SHARED.run(["gh", "release", "edit", tag, "--repo", repository,
             "--draft=false", "--prerelease", "--latest=false"])
    final = SHARED.read_release(repository, release["id"], tag)
    SHARED.verify_release(final, tag, source_sha, assets)
    if final["draft"]:
        raise ValueError("SDK candidate remains a draft")
    tag_state = json.loads(SHARED.run(query_arguments))
    if tag_state.get("errors"):
        raise ValueError("Could not verify published candidate tag")
    SHARED.verify_tag(tag_state["data"]["repository"]["ref"], source_sha)
    print(final["html_url"])


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--directory", type=Path, required=True)
    parser.add_argument("--repository", help="Publication owner/name; otherwise require GITHUB_REPOSITORY")
    arguments = parser.parse_args()
    try:
        publish(arguments.directory, arguments.repository)
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        parser.exit(1, f"SDK candidate publication failed: {error}\n")
