#!/usr/bin/env python3
"""Prepare the reviewed article and immutable public image links for publication."""
import argparse
import json
from pathlib import Path
import re
from urllib.parse import quote


def prepare(article, repository, source_sha, tag):
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository):
        raise ValueError("Invalid GitHub repository")
    if not re.fullmatch(r"[0-9a-f]{40}", source_sha):
        raise ValueError("Article images require an immutable source commit")
    if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+", tag):
        raise ValueError("Article publication requires a production release tag")

    article = Path(article)
    heading, body = article.read_text().split("\n", 1)
    if not heading.startswith("# ") or not heading[2:].strip():
        raise ValueError("Reviewed article must begin with its title")
    raw_root = f"https://raw.githubusercontent.com/{repository}/{source_sha}"

    def image_link(match):
        relative = Path(match.group(2))
        if relative.is_absolute() or ".." in relative.parts or relative.parts[0] != "assets":
            raise ValueError("Article images must be local reviewed assets")
        asset = article.parent / relative
        if asset.is_symlink() or not asset.is_file():
            raise ValueError(f"Missing regular article image: {relative}")
        return f"![{match.group(1)}]({raw_root}/release/articles/{quote(relative.as_posix())})"

    body = re.sub(r"!\[([^\]]*)\]\(([^)]+)\)", image_link, body)
    release_url = f"https://github.com/{repository}/releases/tag/{tag}"
    body = body.replace(f"https://github.com/{repository}/releases/latest", release_url)
    brand = article.parent / "assets/brand.png"
    if not brand.is_file() or brand.is_symlink():
        raise ValueError("Reviewed company image is missing")
    return {
        "title": heading[2:].strip(),
        "body_markdown": body.strip(),
        "main_image": f"{raw_root}/release/articles/assets/brand.png",
        "release_url": release_url,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--article", required=True)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--output", required=True)
    arguments = parser.parse_args()
    prepared = prepare(arguments.article, arguments.repository, arguments.source_sha, arguments.tag)
    Path(arguments.output).write_text(json.dumps(prepared, ensure_ascii=False, indent=2) + "\n")


if __name__ == "__main__":
    main()
