#!/usr/bin/env python3
"""Publish already-verified OCI artifacts, preserving the SPKs' exact image digests."""

import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile


DIRECTORY = Path(__file__).resolve().parent
specification = importlib.util.spec_from_file_location("prebuilt_images", DIRECTORY / "prebuild.py")
PREBUILD = importlib.util.module_from_spec(specification)
specification.loader.exec_module(PREBUILD)


def run(arguments, **options):
    return subprocess.run(arguments, check=True, timeout=1200, **options)


def publish(directory, artifacts, version, source_sha):
    with tempfile.TemporaryDirectory(prefix="dockpipe-registry-") as temporary:
        temporary = Path(temporary)
        authentication = temporary / "auth.json"
        anonymous = temporary / "anonymous.json"
        anonymous.write_text('{"auths":{}}\n')
        run(["skopeo", "login", "--authfile", str(authentication), "--username", os.environ["GITHUB_ACTOR"],
             "--password-stdin", "ghcr.io"], input=os.environ["GITHUB_TOKEN"].encode(), stdout=subprocess.DEVNULL)
        for architecture, label in (("amd64", "x64"), ("arm64", "arm64")):
            receipt_path = directory / (architecture + ".json")
            receipt = PREBUILD.PACKAGING.IMAGE_RECEIPT.load_receipt(
                receipt_path, version, architecture,
                artifacts / f"dockpipe_{version}_{architecture}.deb",
                artifacts / f"dockpipe-desktop_{version}_{architecture}.deb",
            )
            if receipt["source_sha"] != source_sha:
                raise ValueError("Image receipt is from a different source commit")
            public_receipt = artifacts / f"dockpipe-desktop_{version}_synology_{label}.image.json"
            if public_receipt.read_bytes() != receipt_path.read_bytes():
                raise ValueError("Public image receipt differs from the build artifact")
            oci = directory / (architecture + ".oci.tar")
            raw = run(["skopeo", "inspect", "--raw", "oci-archive:" + str(oci)], capture_output=True).stdout
            digest, image_id = PREBUILD.manifest_identity(raw)
            repository, expected_digest = receipt["image"].split("@")
            if digest != expected_digest or image_id != receipt["image_id"]:
                raise ValueError("OCI artifact does not match the admitted SPK image")
            destination = f"{repository}:{version}-{source_sha}-{architecture}"
            digest_file = temporary / "published.digest"
            run(["skopeo", "copy", "--authfile", str(authentication), "--preserve-digests",
                 "--digestfile", str(digest_file), "oci-archive:" + str(oci), "docker://" + destination])
            if digest_file.read_text().strip() != expected_digest:
                raise ValueError("Published image digest differs from the SPK reference")
            try:
                public = run(["skopeo", "inspect", "--authfile", str(anonymous), "--raw",
                              "docker://" + receipt["image"]], capture_output=True).stdout
            except subprocess.CalledProcessError as error:
                raise ValueError("The desktop image must be public on GHCR before standard SPKs can be published. "
                                 "Set the container package visibility to Public and retry publication.") from error
            if "sha256:" + hashlib.sha256(public).hexdigest() != expected_digest:
                raise ValueError("Anonymous registry readback returned a different image")
            print("Verified public " + receipt["image"])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--directory", type=Path, required=True)
    parser.add_argument("--artifacts", type=Path, required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--source-sha", required=True)
    arguments = parser.parse_args()
    try:
        publish(**vars(arguments))
    except (OSError, ValueError, KeyError, subprocess.SubprocessError) as error:
        parser.exit(1, f"Desktop image publication failed: {error}\n")


if __name__ == "__main__":
    main()
