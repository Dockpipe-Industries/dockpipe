"""Immutable candidate transactions shared by the repository's publishers.

Artifact-specific qualification and accepted-branch policy remain with callers.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess

# This pin identifies accepted artifact provenance; it is never a publication destination.
SOURCE_REPOSITORY = json.loads(Path(__file__).with_name('source-identity.json').read_text())['repository']
RELEASE_FIELDS = ("{id,tag_name,target_commitish,prerelease,draft,html_url,"
                  "assets:[.assets[]|{name,size,digest,state}]}")


def repository_name(value):
    if (not isinstance(value, str) or len(value) > 140
            or not re.fullmatch(r'[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?/[A-Za-z0-9_.-]{1,100}', value)
            or value.split('/')[1] in ('.', '..')):
        raise ValueError('Publication repository must be an explicit owner/name')
    return value


def publication_target(explicit=None):
    selected = explicit if explicit is not None else os.environ.get('GITHUB_REPOSITORY')
    return repository_name(selected)


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

def verify_release(release, tag, source_sha, assets, *, complete=True):
    if (not isinstance(release, dict) or type(release.get("id")) is not int
            or release["id"] <= 0 or type(release.get("draft")) is not bool
            or release.get("tag_name") != tag or release.get("target_commitish") != source_sha
            or release.get("prerelease") is not True):
        raise ValueError("Existing candidate differs from the qualified source identity")
    remote = release.get("assets")
    if not isinstance(remote, list) or len(remote) > len(assets):
        raise ValueError("Remote candidate inventory differs; published bytes are never replaced")
    seen = set()
    for item in remote:
        name = item.get("name") if isinstance(item, dict) else None
        if name not in assets or name in seen:
            raise ValueError("Remote candidate inventory differs; published bytes are never replaced")
        seen.add(name)
        if (item.get("state") != "uploaded"
                or any(item.get(key) != value for key, value in assets[name].items())):
            raise ValueError("Remote candidate bytes differ; published bytes are never replaced")
    if complete and seen != set(assets):
        raise ValueError("Remote candidate inventory differs; published bytes are never replaced")
    return seen

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

def read_release(repository, identity, tag):
    repository = repository_name(repository)
    if type(identity) is not int or identity <= 0:
        raise ValueError("Candidate release ID is invalid")
    endpoint = f"repos/{repository}/releases/{identity}"
    release = json.loads(run(["gh", "api", "--hostname", "github.com", endpoint,
                              "--jq", RELEASE_FIELDS]))
    if (not isinstance(release, dict) or release.get("id") != identity
            or release.get("tag_name") != tag):
        raise ValueError("Candidate release identity changed during lookup")
    return release

def find_release(repository, tag):
    repository = repository_name(repository)
    # Project each page on the server response before capturing stdout. Release
    # bodies and unrelated asset inventories are neither needed nor admitted.
    selection = "{count:length,matches:[.[]|select(.tag_name==" + json.dumps(tag) + ")|{id,tag_name}]}"
    for page in range(1, 6):
        endpoint = f"repos/{repository}/releases?per_page=100&page={page}"
        selected = json.loads(run(["gh", "api", "--hostname", "github.com", endpoint,
                                   "--jq", selection]))
        if (not isinstance(selected, dict) or set(selected) != {"count", "matches"}
                or type(selected["count"]) is not int or not 0 <= selected["count"] <= 100
                or not isinstance(selected["matches"], list)
                or len(selected["matches"]) > selected["count"]):
            raise ValueError("Candidate release inventory is invalid")
        matches = selected["matches"]
        if len(matches) > 1:
            raise ValueError("Candidate release tag is ambiguous")
        if matches:
            match = matches[0]
            if not isinstance(match, dict) or match.get("tag_name") != tag:
                raise ValueError("Candidate release lookup selected another tag")
            return read_release(repository, match.get("id"), tag)
        if selected["count"] < 100:
            return None
    raise ValueError("Candidate release lookup exceeds capacity; no publication attempted")

def create_release(repository, tag, source_sha, title, notes):
    repository = repository_name(repository)
    endpoint = f"repos/{repository}/releases"
    created = json.loads(run([
        "gh", "api", "--hostname", "github.com", "--method", "POST", endpoint,
        "-f", "tag_name=" + tag, "-f", "target_commitish=" + source_sha,
        "-f", "name=" + title, "-f", "body=" + notes,
        "-F", "draft=true", "-F", "prerelease=true", "-f", "make_latest=false",
        "--jq", "{id,tag_name}",
    ]))
    if not isinstance(created, dict) or created.get("tag_name") != tag:
        raise ValueError("Created candidate response lacks its exact ID/tag identity")
    # Never search the eventually consistent release list after a successful POST.
    return read_release(repository, created.get("id"), tag)

def reconcile_assets(repository, directory, release, tag, source_sha, assets):
    repository = repository_name(repository)
    seen = verify_release(release, tag, source_sha, assets, complete=not release["draft"])
    if release["draft"]:
        for name in sorted(set(assets) - seen):
            if name in seen:
                continue
            if release["draft"] is not True:
                raise ValueError("Candidate became published before draft reconciliation completed")
            run(["gh", "release", "upload", tag, str(directory / name), "--repo", repository])
            release = read_release(repository, release["id"], tag)
            seen = verify_release(release, tag, source_sha, assets, complete=not release["draft"])
    verify_release(release, tag, source_sha, assets)
    return release
