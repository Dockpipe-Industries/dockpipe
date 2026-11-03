#!/usr/bin/env python3
"""Build one native desktop image in CI and package its standard/offline editions."""

import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import uuid


SCRIPT_DIRECTORY = Path(__file__).resolve().parent
specification = importlib.util.spec_from_file_location("desktop_spk", SCRIPT_DIRECTORY / "build.py")
PACKAGING = importlib.util.module_from_spec(specification)
specification.loader.exec_module(PACKAGING)
QUALIFIER = PACKAGING.load_helper("synology_image_qualification", "workflows/package/synology-desktop-spk/assets/scripts/qualify_image.py")
NORMALIZER = PACKAGING.load_helper(
    "synology_archive_normalizer", "workflows/package/synology-desktop-spk/assets/scripts/normalize_archive.py",
)


def command(arguments, timeout=1800):
    subprocess.run(arguments, check=True, timeout=timeout)


def manifest_identity(manifest_bytes):
    if len(manifest_bytes) > 4 * 1024 * 1024:
        raise ValueError("Image manifest exceeds the metadata limit")
    manifest = json.loads(manifest_bytes)
    if not isinstance(manifest, dict) or not isinstance(manifest.get("config"), dict):
        raise ValueError("Expected a single-platform image manifest")
    config = manifest["config"].get("digest", "")
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", config):
        raise ValueError("Expected a single-platform image manifest with a config digest")
    return "sha256:" + hashlib.sha256(manifest_bytes).hexdigest(), config


def prebuild(version, architecture, source_sha, repository, binary, cli_deb, desktop_deb, output, image_directory):
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", version):
        raise ValueError("Expected a numeric release version")
    if architecture not in ("amd64", "arm64"):
        raise ValueError("Expected amd64 or arm64")
    if not re.fullmatch(r"[0-9a-f]{40}", source_sha):
        raise ValueError("Expected a full source commit")
    if not re.fullmatch(r"ghcr\.io/[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*", repository):
        raise ValueError("Expected a lowercase GHCR repository")
    cli, desktop, package_version = PACKAGING.DESKTOP_PACKAGING.validate_inputs(
        cli_deb, desktop_deb, architecture, PACKAGING.DEFAULT_WEBTOP_IMAGE,
    )
    if package_version != version:
        raise ValueError("Image packages must match the release version")
    PACKAGING.CLI_PACKAGING.validate_binary(binary, architecture)
    output.mkdir(parents=True, exist_ok=True)
    image_directory.mkdir(parents=True, exist_ok=True)
    label = PACKAGING.CLI_PACKAGING.ARCHITECTURES[architecture][0]
    basename = f"dockpipe-desktop_{version}_synology_{label}"
    image_tag = f"{repository}:build-{source_sha}-{architecture}"

    with tempfile.TemporaryDirectory(prefix=".prebuilt-", dir=image_directory) as temporary:
        temporary = Path(temporary)
        context = temporary / "context"
        (context / "packages").mkdir(parents=True)
        for name in PACKAGING.DESKTOP_ASSETS:
            content = PACKAGING.render_asset(Path("desktop") / name, architecture, PACKAGING.DEFAULT_WEBTOP_IMAGE)
            (context / name).write_bytes(content)
        shutil.copyfile(cli, context / "packages/cli.deb")
        shutil.copyfile(desktop, context / "packages/desktop.deb")
        oci = temporary / "image.oci.tar"
        builder = "dockpipe-synology-" + uuid.uuid4().hex
        command(["docker", "buildx", "create", "--name", builder, "--driver", "docker-container"], timeout=60)
        try:
            command([
                "docker", "buildx", "build", "--builder", builder, "--platform", "linux/" + architecture,
                "--provenance=false", "--sbom=false", "--tag", image_tag,
                "--label", "org.opencontainers.image.source=https://github.com/Dockpipe-Industries/dockpipe",
                "--label", "org.opencontainers.image.version=" + version,
                "--label", "org.opencontainers.image.revision=" + source_sha,
                "--output", f"type=oci,dest={oci}", str(context),
            ], timeout=3600)
        finally:
            command(["docker", "buildx", "rm", "--force", builder], timeout=60)
        raw = subprocess.run(
            ["skopeo", "inspect", "--raw", "oci-archive:" + str(oci)],
            check=True, capture_output=True, timeout=60,
        ).stdout
        registry_digest, image_id = manifest_identity(raw)
        offline_image = repository + ":offline-" + image_id.removeprefix("sha256:")
        docker_archive = temporary / "image.tar"
        command(["skopeo", "copy", "oci-archive:" + str(oci),
                 f"docker-archive:{docker_archive}:{offline_image}"], timeout=1200)
        if docker_archive.stat().st_size > PACKAGING.IMAGE_RECEIPT.MAX_ARCHIVE_BYTES:
            raise ValueError("Exported desktop image exceeds the archive capacity")
        compressed = temporary / "image.tar.gz"
        NORMALIZER.write_loadable_archive(docker_archive, compressed)
        receipt = {
            "schema": 1, "version": version, "architecture": architecture,
            "source_sha": source_sha, "image": repository + "@" + registry_digest,
            "image_id": image_id, "offline_image": offline_image,
            "inputs": {"cli_sha256": PACKAGING.IMAGE_RECEIPT.file_sha256(cli),
                       "desktop_sha256": PACKAGING.IMAGE_RECEIPT.file_sha256(desktop)},
            "archive_sha256": PACKAGING.IMAGE_RECEIPT.file_sha256(compressed),
        }
        receipt_path = temporary / "image.json"
        receipt_path.write_text(json.dumps(receipt, indent=2) + "\n")
        PACKAGING.IMAGE_RECEIPT.load_receipt(receipt_path, version, architecture, cli, desktop, compressed)
        QUALIFIER.qualify(compressed, receipt, temporary)
        staged_output = temporary / "packages"
        standard = PACKAGING.build(version, architecture, binary, cli, desktop, staged_output, image_receipt=receipt_path)
        offline = PACKAGING.build(version, architecture, binary, cli, desktop, staged_output,
                                  image_receipt=receipt_path, offline_archive=compressed)
        # GitHub release assets have a 2 GiB per-file limit.
        if offline.stat().st_size >= 2 * 1024**3:
            standard.unlink()
            offline.unlink()
            raise ValueError("Offline SPK exceeds the release asset size limit")
        standard = standard.replace(output / standard.name)
        offline = offline.replace(output / offline.name)
        shutil.copyfile(receipt_path, output / (basename + ".image.json"))
        shutil.copyfile(receipt_path, image_directory / (architecture + ".json"))
        oci.replace(image_directory / (architecture + ".oci.tar"))
    return standard, offline


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True)
    parser.add_argument("--architecture", choices=("amd64", "arm64"), required=True)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--cli-deb", type=Path, required=True)
    parser.add_argument("--desktop-deb", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--image-directory", type=Path, required=True)
    arguments = parser.parse_args()
    try:
        packages = prebuild(**vars(arguments))
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        parser.exit(1, f"Prebuilt Synology packaging failed: {error}\n")
    for package in packages:
        print("Built " + str(package))


if __name__ == "__main__":
    main()
