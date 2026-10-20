#!/usr/bin/env python3
"""Publish the qualified host SDK as an immutable candidate, without channel promotion."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess


REPOSITORY = "Dockpipe-Industries/dockpipe"
ARCHIVE = "DockPipeLauncherSDK-1.1.0-Linux-amd64.tar.gz"
ASSETS = {ARCHIVE, "sdk-files.json", "sdk-release.json"}
MAX_ASSET_BYTES = 512 << 20
ROOT = Path(__file__).resolve().parents[2]


def browser_dependency():
    helper_path = ROOT / "release/packaging/prepare-browser-sdk.py"
    spec = importlib.util.spec_from_file_location("publisher_browser_preparer", helper_path)
    helper = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(helper)
    lock, _ = helper.load_lock(ROOT / "release/dependencies/spatialweb-browser.lock.json", "linux-amd64")
    return lock


def digest(path):
    checksum = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1 << 20), b""):
            checksum.update(block)
    return checksum.hexdigest()


def run(arguments):
    result = subprocess.run(arguments, capture_output=True, text=True, timeout=300)
    if result.returncode:
        status = re.search(r"\bHTTP ([1-5][0-9]{2})\b", result.stderr or "")
        details = f"exit {result.returncode}"
        if status:
            details += f"; HTTP {status.group(1)}"
        raise ValueError(f"Candidate operation failed: {arguments[0]} {arguments[1]} ({details})")
    if len(result.stdout) > 2 << 20:
        raise ValueError("Candidate operation response exceeds metadata capacity")
    return result.stdout


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
        "source_repository": REPOSITORY, "source_sha": source_sha,
        "source_state": "committed-snapshot",
    }
    if not isinstance(metadata, dict) or any(metadata.get(key) != value for key, value in required.items()):
        raise ValueError("SDK source and ABI identity differ from the publishing run")
    browser = browser_dependency()
    if (metadata.get("dependencies") != {"spatialweb_browser": browser}
            or metadata.get("compiler") != browser["compiler"]
            or metadata.get("abi") != {"cxx_standard": 17, "workspace_interface": "com.dockpipe.launcher.WorkspacePackage/2.0"}
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


def verify_release(release, tag, source_sha, assets):
    if (release.get("tag_name") != tag or release.get("target_commitish") != source_sha
            or release.get("prerelease") is not True):
        raise ValueError("Existing candidate differs from the qualified source identity")
    remote = release.get("assets", [])
    if len(remote) != len(assets) or {item.get("name") for item in remote} != set(assets):
        raise ValueError("Remote candidate inventory differs; published bytes are never replaced")
    for item in remote:
        if any(item.get(key) != value for key, value in assets[item["name"]].items()):
            raise ValueError("Remote candidate bytes differ; published bytes are never replaced")


def verify_tag(reference, source_sha, *, allow_missing=False):
    if reference is None and allow_missing:
        return
    target = reference.get("target") if isinstance(reference, dict) else None
    for _ in range(3):
        if not isinstance(target, dict):
            break
        if target.get("__typename") == "Commit" and target.get("oid") == source_sha:
            return
        if target.get("__typename") != "Tag":
            break
        target = target.get("target")
    raise ValueError("Candidate tag does not resolve to the exact qualified source commit")


def find_release(tag):
    # The tag endpoint can return 404 for an authenticated draft. The release
    # inventory includes drafts; bind its numeric ID before reading assets.
    for page in range(1, 6):
        endpoint = f"repos/{REPOSITORY}/releases?per_page=100&page={page}"
        releases = json.loads(run(["gh", "api", "--hostname", "github.com", endpoint]))
        if (not isinstance(releases, list) or len(releases) > 100
                or any(not isinstance(release, dict) for release in releases)):
            raise ValueError("Candidate release inventory is invalid")
        matches = [release for release in releases if release.get("tag_name") == tag]
        if len(matches) > 1:
            raise ValueError("Candidate release tag is ambiguous")
        if matches:
            release_id = matches[0].get("id")
            if type(release_id) is not int or release_id <= 0:
                raise ValueError("Candidate release ID is invalid")
            endpoint = f"repos/{REPOSITORY}/releases/{release_id}"
            release = json.loads(run(["gh", "api", "--hostname", "github.com", endpoint]))
            if not isinstance(release, dict) or release.get("id") != release_id or release.get("tag_name") != tag:
                raise ValueError("Candidate release identity changed during lookup")
            return release
        if len(releases) < 100:
            return None
    raise ValueError("Candidate release lookup exceeds capacity; no publication attempted")


def publish(directory):
    source_sha = os.environ.get("GITHUB_SHA", "")
    if (os.environ.get("GITHUB_REPOSITORY") != REPOSITORY
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
        "gh", "api", "graphql", "-f", "query=" + query, "-f", "owner=Dockpipe-Industries",
        "-f", "name=dockpipe", "-f", "tag=" + tag, "-f", "ref=refs/tags/" + tag,
    ]
    existing = json.loads(run(query_arguments))
    if existing.get("errors"):
        raise ValueError("Could not establish whether candidate already exists")
    release = find_release(tag)
    verify_tag(existing["data"]["repository"]["ref"], source_sha,
               allow_missing=release is None or release.get("draft") is True)
    if release is None:
        run([
            "gh", "release", "create", tag, *[str(directory / name) for name in sorted(ASSETS)],
            "--repo", REPOSITORY, "--target", source_sha, "--draft", "--prerelease",
            "--latest=false", "--title", "Launcher SDK 1.1.0 candidate",
            "--notes", "Qualified installed host SDK for SpatialPipe/Cockpit candidate builds. "
            "No Dockpipe application release or production channel update.",
        ])
        release = find_release(tag)
    if release is None:
        raise ValueError("Created candidate draft is unavailable in the release inventory")
    endpoint = f"repos/{REPOSITORY}/releases/{release['id']}"
    verify_release(release, tag, source_sha, assets)
    tag_state = json.loads(run(query_arguments))
    if tag_state.get("errors"):
        raise ValueError("Could not verify candidate tag")
    # GitHub may defer creating a new tag until the draft is published.
    verify_tag(tag_state["data"]["repository"]["ref"], source_sha,
               allow_missing=release["draft"])
    if release["draft"]:
        run(["gh", "release", "edit", tag, "--repo", REPOSITORY,
             "--draft=false", "--prerelease", "--latest=false"])
    final = json.loads(run(["gh", "api", endpoint]))
    verify_release(final, tag, source_sha, assets)
    if final["draft"]:
        raise ValueError("SDK candidate remains a draft")
    tag_state = json.loads(run(query_arguments))
    if tag_state.get("errors"):
        raise ValueError("Could not verify published candidate tag")
    verify_tag(tag_state["data"]["repository"]["ref"], source_sha)
    print(final["html_url"])


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--directory", type=Path, required=True)
    arguments = parser.parse_args()
    try:
        publish(arguments.directory)
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        parser.exit(1, f"SDK candidate publication failed: {error}\n")
