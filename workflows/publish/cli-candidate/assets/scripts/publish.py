#!/usr/bin/env python3
"""Publish an exact, immutable store-free CLI bootstrap candidate."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess


SPEC = importlib.util.spec_from_file_location(
    "SHARED", Path(__file__).resolve().parents[5] / "workflows/publish/shared/github_release.py")
SHARED = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SHARED)

SPEC = importlib.util.spec_from_file_location(
    "PRODUCER", Path(__file__).resolve().parents[5] / "workflows/build/cli-candidate/assets/scripts/build.py")
PRODUCER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PRODUCER)
SOURCE_REPOSITORY = SHARED.SOURCE_REPOSITORY


def validate(directory, source_sha, repository=None):
    metadata_path = directory / "cli-candidate.json"
    if (directory.is_symlink() or metadata_path.is_symlink() or not metadata_path.is_file()
            or not 0 < metadata_path.stat().st_size <= 128 << 10):
        raise ValueError("CLI candidate requires bounded regular metadata")
    metadata = json.loads(metadata_path.read_text(encoding="utf-8"))
    if (not isinstance(metadata, dict) or metadata.get("schema") != 1
            or metadata.get("product") != "DockpipeCLI" or metadata.get("artifact_class") != "cli-only"
            or metadata.get("source_repository") != SOURCE_REPOSITORY or metadata.get("source_sha") != source_sha
            or metadata.get("source_state") != "committed-snapshot"
            or metadata.get("go_version") != PRODUCER.GO_VERSION
            or metadata.get("module_acquisition") != "fresh-public-proxy-and-checksum-database"):
        raise ValueError("CLI candidate source or qualification identity differs")
    version = metadata.get("version", "")
    if not isinstance(version, str):
        raise ValueError("CLI candidate version must be a string")
    release_line = (Path(__file__).resolve().parents[5] / "VERSION").read_text().strip()
    expected_version, tag = PRODUCER.validate_allocation(
        metadata.get("version_allocation"), release_line, source_sha, repository)
    if version != expected_version or metadata.get("tag") != tag:
        raise ValueError("CLI candidate version/tag differs from the exact source")
    artifacts = metadata.get("artifacts", {})
    if not isinstance(artifacts, dict) or set(artifacts) != set(PRODUCER.TARGETS):
        raise ValueError("CLI candidate requires exactly five platform archives")
    module_files = metadata.get("module_files", {})
    if (not isinstance(module_files, dict) or set(module_files) != {"go.mod", "go.sum"}
            or any(not isinstance(value, str) or not re.fullmatch(r"[0-9a-f]{64}", value)
                   for value in module_files.values())):
        raise ValueError("CLI candidate module pins are missing or invalid")
    checks = {"public-module-download", "module-integrity", "pure-go-five-platform-build",
              "native-linux-version", "native-python-package-source-hook", "native-python-host-step"}
    if not checks.issubset(metadata.get("checks", [])):
        raise ValueError("CLI candidate public fetch and Python hook checks are required")
    assets = {"cli-candidate.json": {"size": metadata_path.stat().st_size,
                                     "digest": "sha256:" + SHARED.digest(metadata_path)}}
    for target, entry in artifacts.items():
        operating_system, architecture = target.split("/")
        windows = operating_system == "windows"
        name = f"dockpipe_{version}_{operating_system}_{architecture}" + (".zip" if windows else ".tar.gz")
        if not isinstance(entry, dict) or entry.get("name") != name:
            raise ValueError("CLI archive name differs from its platform/version")
        path = directory / name
        if path.is_symlink() or not path.is_file() or not 0 < path.stat().st_size <= 64 << 20:
            raise ValueError("CLI archive requires bounded regular bytes")
        if (type(entry.get("size_bytes")) is not int or entry["size_bytes"] != path.stat().st_size
                or entry.get("sha256") != SHARED.digest(path)
                or entry.get("executable") != ("dockpipe.exe" if windows else "dockpipe")
                or type(entry.get("executable_size_bytes")) is not int
                or not 0 < entry["executable_size_bytes"] <= PRODUCER.MAX_BINARY_BYTES
                or not isinstance(entry.get("executable_sha256"), str)
                or not re.fullmatch(r"[0-9a-f]{64}", entry["executable_sha256"])):
            raise ValueError("CLI archive differs from its qualification receipt")
        assets[name] = {"size": path.stat().st_size, "digest": "sha256:" + entry["sha256"]}
    if {path.name for path in directory.iterdir()} != set(assets):
        raise ValueError("CLI candidate has unexpected or missing assets")
    return tag, assets


def publish(directory, repository=None):
    repository = SHARED.publication_target(repository)
    source_sha = os.environ.get("GITHUB_SHA", "")
    if (os.environ.get("GITHUB_REPOSITORY") != SOURCE_REPOSITORY or os.environ.get("GITHUB_REF") != "refs/heads/dev"
            or os.environ.get("GITHUB_EVENT_NAME") != "push" or not re.fullmatch(r"[0-9a-f]{40}", source_sha)):
        raise ValueError("CLI candidate publication requires an exact accepted dev push")
    tag, assets = validate(directory, source_sha, repository)
    query = """query($owner:String!,$name:String!,$ref:String!){
      repository(owner:$owner,name:$name){ref(qualifiedName:$ref){target{
        __typename oid ... on Tag {target{
          __typename oid ... on Tag {target{__typename oid}}
        }}
      }}}
    }"""
    arguments = ["gh", "api", "graphql", "-f", "query=" + query, "-f", "owner=" + repository.split("/")[0],
                 "-f", "name=" + repository.split("/")[1], "-f", "ref=refs/tags/" + tag]

    def verify_reference(allow_missing):
        state = json.loads(SHARED.run(arguments))
        if state.get("errors"):
            raise ValueError("Could not verify CLI candidate tag")
        SHARED.verify_tag(state["data"]["repository"]["ref"], source_sha, allow_missing=allow_missing)

    release = SHARED.find_release(repository, tag)
    verify_reference(release is None or release.get("draft") is True)
    if release is None:
        release = SHARED.create_release(
            repository, tag, source_sha, "CLI-only bootstrap candidate",
            "Immutable pure-Go CLI for checksum-pinned package producers. "
            "No package stores, Qt, installers, application release or production channel update.")
    release = SHARED.reconcile_assets(repository, directory, release, tag, source_sha, assets)
    verify_reference(release.get("draft") is True)
    if release.get("draft") is True:
        SHARED.run(["gh", "release", "edit", tag, "--repo", repository,
                    "--draft=false", "--prerelease", "--latest=false"])
    final = SHARED.read_release(repository, release["id"], tag)
    SHARED.verify_release(final, tag, source_sha, assets)
    if final.get("draft") is not False:
        raise ValueError("CLI candidate remains a draft")
    verify_reference(False)
    print(final["html_url"])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--directory", type=Path, required=True)
    parser.add_argument("--repository", help="Publication owner/name; otherwise require GITHUB_REPOSITORY")
    args = parser.parse_args()
    try:
        publish(args.directory, args.repository)
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        parser.exit(1, f"CLI candidate publication failed: {error}\n")


if __name__ == "__main__":
    main()
