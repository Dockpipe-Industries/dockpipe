#!/usr/bin/env python3
"""Select a shared native installer version from the published release history."""
import argparse
from pathlib import Path
import re


VERSION = r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"
RELEASE_TAG = re.compile(r"v" + VERSION + r"(?:-staging\.[1-9][0-9]*\.[1-9][0-9]*\.[0-9a-f]{12})?")


def next_version(base, tags):
    match = re.fullmatch(VERSION, base)
    if not match:
        raise ValueError("Release baseline must be X.Y.Z without leading zeroes")
    major, minor, patch = map(int, match.groups())
    for tag in tags:
        release = RELEASE_TAG.fullmatch(tag.strip())
        if not release:
            continue
        released_major, released_minor, released_patch = map(int, release.groups())
        if (released_major, released_minor) > (major, minor):
            raise ValueError("Release baseline is older than an already published release line")
        if (released_major, released_minor) == (major, minor):
            patch = max(patch, released_patch + 1)
    # Windows Installer compares three fields and limits the patch/build to 65535.
    if major > 255 or minor > 255 or patch > 65535:
        raise ValueError("Generated version exceeds Windows Installer limits; advance the release line")
    return f"{major}.{minor}.{patch}"


def metadata(base, tags, staging_suffix=""):
    version = next_version(base, tags)
    if staging_suffix and not re.fullmatch(r"staging\.[1-9][0-9]*\.[1-9][0-9]*\.[0-9a-f]{12}", staging_suffix):
        raise ValueError("Invalid staging provenance suffix")
    candidate = f"{version}-{staging_suffix}" if staging_suffix else ""
    return {"version": version, "candidate": candidate, "tag": "v" + (candidate or version)}


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", required=True)
    parser.add_argument("--tags", type=Path, required=True)
    parser.add_argument("--staging-suffix", default="")
    args = parser.parse_args()
    for name, value in metadata(args.base, args.tags.read_text().splitlines(), args.staging_suffix).items():
        print(f"{name}={value}")
