#!/usr/bin/env python3
"""Fetch pinned client inputs; install only the CLI and its two plugins."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import tarfile
import urllib.request


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("architecture", choices=("amd64", "arm64"))
    parser.add_argument("destination", type=Path)
    parser.add_argument("--cache", type=Path, required=True)
    args = parser.parse_args()
    lock = json.loads(Path(__file__).with_name("container-tools.lock.json").read_text())
    args.cache.mkdir(parents=True, exist_ok=True)
    paths = {}
    for name, source in lock["tools"][args.architecture].items():
        path = args.cache / f"{args.architecture}-{name}"
        if not path.exists():
            temporary = path.with_suffix(".download")
            try:
                with urllib.request.urlopen(source["url"], timeout=60) as response, temporary.open("wb") as output:
                    shutil.copyfileobj(response, output)
                temporary.replace(path)
            finally:
                temporary.unlink(missing_ok=True)
        with path.open("rb") as stream:
            digest = hashlib.sha256()
            while chunk := stream.read(1024 * 1024):
                digest.update(chunk)
            actual = digest.hexdigest()
        if actual != source["sha256"]:
            raise SystemExit(f"Checksum mismatch for {name}: remove {path} and retry")
        paths[name] = path

    binary_dir = args.destination / "libexec/dockpipe"
    plugin_dir = args.destination / "lib/docker/cli-plugins"
    binary_dir.mkdir(parents=True, exist_ok=True)
    plugin_dir.mkdir(parents=True, exist_ok=True)
    with tarfile.open(paths["docker"]) as archive:
        member = archive.getmember("docker/docker")
        if not member.isfile():
            raise SystemExit("Docker CLI archive member must be a regular file")
        with archive.extractfile(member) as source, (binary_dir / "docker").open("wb") as output:
            shutil.copyfileobj(source, output)
    (binary_dir / "docker").chmod(0o755)
    for name in ("compose", "buildx"):
        output = plugin_dir / f"docker-{name}"
        shutil.copyfile(paths[name], output)
        output.chmod(0o755)


if __name__ == "__main__":
    main()
