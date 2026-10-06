#!/usr/bin/env python3
"""Prepare or publish a verified staging formula; never execute downloaded code."""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import urllib.error
import urllib.request

from staging_formula import ORIGIN, candidate_order, candidate_parts, parse_checksums, render


SOURCE = "Dockpipe-Industries/dockpipe"
TAP = "Dockpipe-Industries/homebrew-dockpipe"
FORMULA = "Formula/dockpipe-staging.rb"


def download(path):
    # curl matches the release installers and handles Cloudflare's public endpoint.
    return subprocess.check_output([
        "curl", "--fail", "--silent", "--show-error", "--retry", "2",
        "--connect-timeout", "15", "--max-time", "60", "--proto", "=https",
        f"{ORIGIN}/{path}",
    ])


def github(path, payload=None, allow_missing=False):
    headers = {"Accept": "application/vnd.github+json", "User-Agent": "dockpipe-homebrew-sync",
               "X-GitHub-Api-Version": "2022-11-28"}
    token = os.environ.get("GH_TOKEN")
    if token:
        headers["Authorization"] = f"Bearer {token}"
    data = None if payload is None else json.dumps(payload).encode()
    if payload is not None:
        headers["Content-Type"] = "application/json"
    request = urllib.request.Request(f"https://api.github.com/repos/{path}", data=data,
                                     headers=headers, method="GET" if payload is None else "PUT")
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        if allow_missing and error.code == 404:
            return None
        raise


def completed_candidate(pointer, catalog, run):
    candidate = pointer["candidate"]
    version, run_id, attempt, short_sha = candidate_parts(candidate)
    manifest = f"packages/candidates/{candidate}/release-manifest.json"
    if pointer.get("channel") != "staging" or pointer.get("version") != version or pointer.get("manifest") != manifest:
        raise ValueError("Staging pointer identity mismatch")
    source_sha = catalog.get("source_sha", "")
    if (catalog.get("schema") != 1 or catalog.get("channel") != "staging"
            or catalog.get("candidate") != candidate or catalog.get("version") != version
            or not re.fullmatch(r"[0-9a-f]{40}", source_sha) or not source_sha.startswith(short_sha)):
        raise ValueError("Staging catalog provenance mismatch")
    if (run.get("id") != int(run_id) or run.get("run_attempt") != int(attempt)
            or run.get("head_sha") != source_sha or run.get("head_branch") != "staging"
            or run.get("event") != "push" or run.get("path") != ".github/workflows/ci.yml"
            or run.get("repository", {}).get("full_name") != SOURCE):
        raise ValueError("Release does not match the staging CI run")
    return run.get("status") == "completed" and run.get("conclusion") == "success"


def latest_formula():
    pointer = json.loads(download("packages/latest.json"))
    candidate = pointer["candidate"]
    _, run_id, attempt, _ = candidate_parts(candidate)
    # Construct the path from the validated identity, never from an arbitrary pointer URL.
    base = f"packages/candidates/{candidate}"
    catalog_bytes = download(f"{base}/release-manifest.json")
    catalog = json.loads(catalog_bytes)
    run = github(f"{SOURCE}/actions/runs/{run_id}/attempts/{attempt}")
    if not completed_candidate(pointer, catalog, run):
        print("Staging publication has not completed successfully; leaving the tap unchanged.")
        return None
    checksums = parse_checksums(download(f"{base}/SHA256SUMS.txt").decode())
    if checksums.get("release-manifest.json") != hashlib.sha256(catalog_bytes).hexdigest():
        raise ValueError("Release catalog checksum mismatch")
    return candidate, render(candidate, checksums)


def require_forward_update(current, candidate, formula):
    if current == formula:
        return False
    if current:
        match = re.search(r'^  version "([^"]+)"$', current, re.MULTILINE)
        if not match:
            raise ValueError("Existing formula has no recognized candidate version")
        if candidate_order(candidate) < candidate_order(match[1]):
            raise ValueError("Refusing to roll back the staging formula")
        if candidate_order(candidate) == candidate_order(match[1]) and candidate != match[1]:
            raise ValueError("Conflicting identity for an existing candidate")
    return True


def output(name, value):
    if os.environ.get("GITHUB_OUTPUT"):
        with Path(os.environ["GITHUB_OUTPUT"]).open("a") as stream:
            stream.write(f"{name}={value}\n")


def prepare(directory):
    result = latest_formula()
    if result is None:
        output("changed", "false")
        return
    candidate, formula = result
    current_path = Path(FORMULA)
    current = current_path.read_text() if current_path.exists() else ""
    changed = require_forward_update(current, candidate, formula)
    output("changed", str(changed).lower())
    if not changed:
        print(f"Tap already contains {candidate}.")
        return
    directory.mkdir(parents=True, exist_ok=True)
    (directory / "dockpipe-staging.rb").write_text(formula)
    (directory / "candidate.txt").write_text(candidate + "\n")
    print(f"Prepared {candidate} for native Homebrew validation.")


def publish(directory):
    if os.environ.get("GITHUB_REPOSITORY") != TAP or os.environ.get("GITHUB_REF") != "refs/heads/main":
        raise ValueError("Publication requires the official tap's main branch")
    candidate = (directory / "candidate.txt").read_text().strip()
    formula = (directory / "dockpipe-staging.rb").read_text()
    latest = latest_formula()
    if latest is None or latest[0] != candidate:
        print("A different candidate is current; leaving the tap unchanged.")
        return
    if latest[1] != formula:
        raise ValueError("Tested formula differs from the current release metadata")
    endpoint = f"{TAP}/contents/{FORMULA}"
    existing = github(endpoint + "?ref=main", allow_missing=True)
    current = base64.b64decode(existing["content"]).decode() if existing else ""
    if not require_forward_update(current, candidate, formula):
        print("Formula already published.")
        return
    payload = {"message": f"Update DockPipe staging to {candidate}", "branch": "main",
               "content": base64.b64encode(formula.encode()).decode()}
    if existing:
        payload["sha"] = existing["sha"]
    result = github(endpoint, payload)
    verified = github(endpoint + "?ref=main")
    if base64.b64decode(verified["content"]).decode() != formula:
        raise ValueError("Published formula read-back differs from the tested content")
    print(f"Published {candidate}: {result['commit']['sha']}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("prepare", "publish"))
    parser.add_argument("--output", type=Path, default=Path("prepared"))
    arguments = parser.parse_args()
    if arguments.command == "prepare":
        prepare(arguments.output)
    else:
        publish(arguments.output)
