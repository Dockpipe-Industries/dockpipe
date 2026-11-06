#!/usr/bin/env python3
"""Produce a deterministic full-desktop bundle without network or Docker access."""

import argparse
import gzip
import io
import os
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile


DEFAULT_WEBTOP_IMAGE = (
    "lscr.io/linuxserver/webtop:ubuntu-xfce@sha256:"
    "432475b977d9ff06f8f8adfb9608541bbde51a566ac0cbe62af44f33265e1a37"
)
ASSETS = Path(__file__).resolve().parents[1]
TEXT_ASSETS = ("Dockerfile", "compose.yml", ".env.example", ".dockerignore", "SETUP.md")


def package_metadata(path):
    """Ask dpkg to read control metadata; never execute package maintainer scripts."""
    resolved = Path(path).resolve(strict=True)
    if not resolved.is_file():
        raise ValueError(f"Not a DEB file: {resolved}")
    result = subprocess.run(
        ["dpkg-deb", "--show", "--showformat=${Package}\n${Version}\n${Architecture}\n", str(resolved)],
        check=True,
        capture_output=True,
        text=True,
        timeout=30,
    )
    fields = result.stdout.splitlines()
    if len(fields) != 3:
        raise ValueError(f"Unexpected DEB metadata: {resolved}")
    return resolved, tuple(fields)


def validate_inputs(cli, desktop, architecture, webtop_image):
    image_pattern = r"lscr\.io/linuxserver/webtop(?::ubuntu-xfce)?@sha256:[a-f0-9]{64}"
    if not re.fullmatch(image_pattern, webtop_image):
        raise ValueError("Webtop must be an Ubuntu XFCE image pinned with @sha256:<64 lowercase hex digits>")

    cli_path, cli_metadata = package_metadata(cli)
    desktop_path, desktop_metadata = package_metadata(desktop)
    for metadata, expected_name in ((cli_metadata, "dockpipe"), (desktop_metadata, "dockpipe-desktop")):
        name, version, package_architecture = metadata
        if name != expected_name:
            raise ValueError(f"Expected DEB package {expected_name}, found {name}")
        if package_architecture != architecture:
            raise ValueError(f"Expected {architecture} DEB, found {package_architecture}")
        if not re.fullmatch(r"[0-9][A-Za-z0-9.+~\-]*", version):
            raise ValueError("Package version must be a portable release filename component without an epoch")

    if cli_metadata[1] != desktop_metadata[1]:
        raise ValueError("CLI and desktop DEB versions must match exactly")
    return cli_path, desktop_path, cli_metadata[1]


def add_member(archive, name, size, source):
    info = tarfile.TarInfo(name)
    info.size = size
    info.mode = 0o644
    info.mtime = 0
    info.uid = 0
    info.gid = 0
    archive.addfile(info, source)


def stage_bundle(cli, desktop, architecture, webtop_image, output_directory):
    cli_path, desktop_path, version = validate_inputs(cli, desktop, architecture, webtop_image)
    output_directory = Path(output_directory).resolve()
    output_directory.mkdir(parents=True, exist_ok=True)
    name = f"dockpipe-desktop_{version}_synology_{architecture}"
    destination = output_directory / f"{name}.tar.gz"
    temporary_path = None
    try:
        with tempfile.NamedTemporaryFile(dir=output_directory, prefix=f".{name}.", delete=False) as temporary:
            temporary_path = Path(temporary.name)
            with gzip.GzipFile(fileobj=temporary, mode="wb", filename="", mtime=0) as compressed:
                with tarfile.open(fileobj=compressed, mode="w|", format=tarfile.USTAR_FORMAT) as archive:
                    for asset in TEXT_ASSETS:
                        content = (ASSETS / asset).read_text()
                        content = content.replace("@WEBTOP_IMAGE@", webtop_image).replace("@ARCH@", architecture)
                        encoded = content.encode("utf-8")
                        add_member(archive, f"{name}/{asset}", len(encoded), io.BytesIO(encoded))
                    for package_name, package_path in (("cli.deb", cli_path), ("desktop.deb", desktop_path)):
                        with package_path.open("rb") as package:
                            add_member(archive, f"{name}/packages/{package_name}", os.fstat(package.fileno()).st_size, package)
        temporary_path.replace(destination)
    finally:
        if temporary_path is not None:
            temporary_path.unlink(missing_ok=True)
    return destination


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cli-deb", required=True, type=Path)
    parser.add_argument("--desktop-deb", required=True, type=Path)
    parser.add_argument("--arch", required=True, choices=("amd64", "arm64"))
    parser.add_argument("--webtop-image", default=DEFAULT_WEBTOP_IMAGE)
    parser.add_argument("--output-dir", required=True, type=Path)
    arguments = parser.parse_args()
    try:
        bundle = stage_bundle(
            arguments.cli_deb,
            arguments.desktop_deb,
            arguments.arch,
            arguments.webtop_image,
            arguments.output_dir,
        )
    except (ValueError, OSError, subprocess.SubprocessError) as error:
        parser.exit(1, f"Cannot stage Synology desktop: {error}\n")
    print(bundle)


if __name__ == "__main__":
    main()
