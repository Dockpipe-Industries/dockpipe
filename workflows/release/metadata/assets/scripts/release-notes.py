#!/usr/bin/env python3
"""Prepare release identity and install links; GitHub adds the changelog at publication."""
import argparse
from pathlib import Path
import re


RELEASE_TAG = r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-staging\.[1-9][0-9]*\.[1-9][0-9]*\.[0-9a-f]{12})?"


def render(repository, source_sha, tag, previous_tag=""):
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository):
        raise ValueError("Invalid GitHub repository")
    if not re.fullmatch(r"[0-9a-f]{40}", source_sha):
        raise ValueError("Release notes require an immutable source commit")
    if not re.fullmatch(RELEASE_TAG, tag):
        raise ValueError("Invalid release tag")
    if previous_tag and not re.fullmatch(RELEASE_TAG, previous_tag):
        raise ValueError("Invalid previous release tag")

    repository_url = f"https://github.com/{repository}"
    lines = [
        f"# Dockpipe {tag}",
        "",
        f"Source: [{source_sha[:12]}]({repository_url}/commit/{source_sha}).",
        "",
        f"See the [installation and upgrade guide]({repository_url}/blob/{source_sha}/docs/install.md) "
        "for supported platforms, package choices and known limitations.",
    ]
    if "-staging." in tag:
        lines.extend(["", "This is a staging candidate for qualification, not a stable release."])
    if previous_tag:
        lines.extend(["", f"[Changes since {previous_tag}]({repository_url}/compare/{previous_tag}...{source_sha})."])
    return "\n".join(lines) + "\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--previous-tag", default="")
    parser.add_argument("--output", type=Path, required=True)
    arguments = parser.parse_args()
    body = render(arguments.repository, arguments.source_sha, arguments.tag, arguments.previous_tag)
    arguments.output.write_text(body, encoding="utf-8")


if __name__ == "__main__":
    main()
