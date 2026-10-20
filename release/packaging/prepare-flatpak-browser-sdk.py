#!/usr/bin/env python3
"""Prepare the separately qualified KDE Flatpak Browser SDK from an exact lock.

Native Browser SDK locks are never accepted by this profile. All assets retain
public publisher attestation and bounded extraction requirements in local mode.
"""
import argparse
import importlib.util
import json
from pathlib import Path
import re
import subprocess


SPEC = importlib.util.spec_from_file_location(
    "flatpak_browser_transport", Path(__file__).with_name("prepare-browser-sdk.py"))
BASE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(BASE)
PRODUCT = "SpatialWebFlatpakBrowserSDK"
VERSION = "0.1.0"
PLATFORM = "linux-amd64-flatpak-kde-6.10"
PLATFORMS = {PLATFORM}
REPOSITORY = BASE.REPOSITORY
COMPILER = {"id": "GNU", "version": "15.2.0"}
QT_COMPONENTS = {name: "6.10.3" for name in
                 ("Qt6Core", "Qt6Gui", "Qt6Widgets", "Qt6Network", "Qt6PrintSupport")}
QT_COMPONENTS.update(Qt6WebEngineCore="6.10.2", Qt6WebEngineWidgets="6.10.2")
FLATPAK = {
    "sdk": {"ref": "runtime/org.kde.Sdk/x86_64/6.10",
            "commit": "58a3c36a9e2cbd3fb3507d88d9195b3dcb8e8cde7bce86a0a0a5700a28a0ea18"},
    "runtime": {"ref": "runtime/org.kde.Platform/x86_64/6.10",
                "commit": "31742ff0f905913c512ea732960bbdeffa8306dcbebca45eeee62bfed0ca03a3"},
    "base": {"ref": "app/io.qt.qtwebengine.BaseApp/x86_64/6.10",
             "commit": "dd4e9c5120bc85d02d5f68864dc838e3a17b73523bfe4e83b192f260b5d5133f"},
}
MAX_METADATA_BYTES = BASE.MAX_METADATA_BYTES
read_regular = BASE.read_regular
decode_json = BASE.decode_json
digest = BASE.digest


def load_lock(path, platform):
    if platform != PLATFORM:
        raise ValueError("Unsupported Flatpak Browser SDK platform")
    data = read_regular(path, MAX_METADATA_BYTES)
    lock = decode_json(data)
    expected = {"schema": 1, "product": PRODUCT, "version": VERSION,
                "platform": PLATFORM, "source_repository": REPOSITORY,
                "compiler": COMPILER, "qt_components": QT_COMPONENTS, "flatpak": FLATPAK}
    fields = set(expected) | {"source_sha", "release", "artifact", "metadata", "file_manifest"}
    if (not isinstance(lock, dict) or set(lock) != fields
            or type(lock.get("schema")) is not int
            or any(lock.get(key) != value for key, value in expected.items())):
        raise ValueError("Flatpak Browser SDK lock differs from its exact supported profile")
    source = lock["source_sha"]
    if not isinstance(source, str) or not re.fullmatch(r"[0-9a-f]{40}", source):
        raise ValueError("Flatpak Browser SDK requires an exact source SHA")
    release = {"repository": REPOSITORY, "tag": "flatpak-browser-sdk-0.1.0-staging." + source}
    if lock["release"] != release:
        raise ValueError("Flatpak Browser SDK requires its exact source release")
    stem = PRODUCT + "-" + VERSION + "-" + PLATFORM
    for key, suffix, limit in (("artifact", ".tar.gz", BASE.MAX_ARCHIVE_BYTES),
                               ("metadata", ".release.json", MAX_METADATA_BYTES),
                               ("file_manifest", ".files.json", MAX_METADATA_BYTES)):
        BASE.validate_pin(lock[key], stem + suffix, limit)
    return lock, data


def receipt_identity(lock):
    return {key: lock[key] for key in
            ("source_repository", "source_sha", "compiler", "qt_components", "flatpak", "release")}


def verify_metadata(lock, metadata):
    expected = receipt_identity(lock)
    expected.update(schema=1, product=PRODUCT, version=VERSION, platform=PLATFORM,
                    source_state="committed-snapshot", runtime_bundled=False,
                    abi={"cxx_standard": 17})
    # The release locator belongs to the consumer lock, not producer metadata.
    expected.pop("release")
    if (not isinstance(metadata, dict) or type(metadata.get("schema")) is not int
            or metadata.get("runtime_bundled") is not False
            or "qt_version" in metadata
            or any(metadata.get(key) != value for key, value in expected.items())):
        raise ValueError("Flatpak Browser SDK metadata differs from its exact source or component ABI")
    manifest = {key: lock["file_manifest"][key] for key in ("name", "sha256")}
    if metadata.get("archive") != lock["artifact"] or metadata.get("file_manifest") != manifest:
        raise ValueError("Flatpak Browser SDK metadata does not bind its exact payload")
    checks = {"sdk-component-install", "focused-browser-tests", "relocated-external-consumer-compile-link-run", "flatpak-installed-runtime-dom"}
    actual = metadata.get("checks")
    if not isinstance(actual, list) or not all(isinstance(item, str) for item in actual) or not checks.issubset(actual):
        raise ValueError("Flatpak Browser SDK lacks installed consumer qualification")


def verify_manifest(data, reserved=()):
    return BASE.verify_manifest(data, reserved, package="SpatialWebFlatpakBrowser")


def verify_attestation(path, source_sha):
    BASE.verify_attestation(path, source_sha, workflow="flatpak-browser-sdk.yml")


def prepare(lock_path, platform, output, local_assets=None):
    lock, data = load_lock(lock_path, platform)
    return BASE.prepare_selected(
        lock, data, platform, output, local_assets,
        metadata_verifier=verify_metadata, manifest_verifier=verify_manifest,
        attestation_verifier=verify_attestation, identity_factory=receipt_identity, product=PRODUCT)


def verify_refs(lock, command=None):
    """Read actual selected refs. No automatic update or fallback is permitted."""
    if command is None:
        command = subprocess.check_output
    for name, identity in lock["flatpak"].items():
        actual = command(["flatpak", "info", "--show-commit", identity["ref"]], text=True).strip()
        if actual != identity["commit"]:
            raise ValueError(f"Installed Flatpak {name} commit differs from its qualified lock")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--lock", type=Path, required=True)
    parser.add_argument("--platform", choices=sorted(PLATFORMS), default=PLATFORM)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--local-assets", type=Path)
    parser.add_argument("--receipt", type=Path)
    parser.add_argument("--verify-installed-refs", action="store_true")
    parser.add_argument("--print-ref-pins", action="store_true")
    args = parser.parse_args()
    try:
        lock, _ = load_lock(args.lock, args.platform)
        if args.print_ref_pins:
            for identity in lock["flatpak"].values():
                print(identity["ref"] + "\t" + identity["commit"])
        if args.verify_installed_refs:
            verify_refs(lock)
        if args.output:
            if args.receipt and (args.receipt.exists() or args.receipt.is_symlink()):
                raise ValueError("Receipt destination must be fresh")
            receipt = prepare(args.lock, args.platform, args.output, args.local_assets)
            rendered = json.dumps(receipt, indent=2, sort_keys=True) + "\n"
            if args.receipt:
                args.receipt.parent.mkdir(parents=True, exist_ok=True)
                args.receipt.write_text(rendered)
            print(rendered, end="")
        elif not args.verify_installed_refs and not args.print_ref_pins:
            raise ValueError("Specify --output, --verify-installed-refs or --print-ref-pins")
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        parser.exit(1, f"Flatpak Browser SDK preparation failed: {error}\n")


if __name__ == "__main__":
    main()
