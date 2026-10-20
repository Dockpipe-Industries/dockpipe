#!/usr/bin/env python3
"""Read back a verified SDK against its committed lock and stage runtime files only."""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil


PREPARER = Path(__file__).resolve().parents[1] / "prepare-browser-sdk.py"
SPEC = importlib.util.spec_from_file_location("desktop_browser_preparer", PREPARER)
SDK = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SDK)


def verify(prefix, lock_path, platform, sdk=SDK):
    lock, lock_bytes = sdk.load_lock(lock_path, platform)
    if prefix.is_symlink() or not prefix.is_dir():
        raise ValueError("Browser SDK must be an installed regular prefix")
    if sdk.read_regular(prefix / "consumer-lock.json", sdk.MAX_METADATA_BYTES) != lock_bytes:
        raise ValueError("Browser SDK consumer lock differs from the committed lock")
    metadata_bytes = sdk.read_regular(prefix / lock["metadata"]["name"], sdk.MAX_METADATA_BYTES)
    inventory_bytes = sdk.read_regular(prefix / lock["file_manifest"]["name"], sdk.MAX_METADATA_BYTES)
    for key, data in (("metadata", metadata_bytes), ("file_manifest", inventory_bytes)):
        if len(data) != lock[key]["size_bytes"] or hashlib.sha256(data).hexdigest() != lock[key]["sha256"]:
            raise ValueError("Browser SDK sidecar differs from the committed pin")
    metadata = sdk.decode_json(metadata_bytes)
    sdk.verify_metadata(lock, metadata)
    entries = sdk.verify_manifest(inventory_bytes, [lock[key]["name"] for key in ("artifact", "metadata", "file_manifest")])
    expected = set(entries) | {lock["metadata"]["name"], lock["file_manifest"]["name"],
                               "consumer-lock.json", "consumer-receipt.json"}
    actual = set()
    for path in prefix.rglob("*"):
        if path.is_symlink():
            raise ValueError("Browser SDK installed prefix contains a link")
        if path.is_file():
            actual.add(path.relative_to(prefix).as_posix())
    if actual != expected:
        raise ValueError("Browser SDK installed inventory has missing or extra files")
    for name, entry in entries.items():
        path = prefix / name
        if not path.is_file() or path.stat().st_size != entry["bytes"] or sdk.digest(path) != entry["sha256"]:
            raise ValueError("Browser SDK installed file differs from its admitted inventory")
    receipt = sdk.decode_json(sdk.read_regular(prefix / "consumer-receipt.json", sdk.MAX_METADATA_BYTES))
    identity = sdk.receipt_identity(lock)
    if platform == "windows-amd64":
        identity["msvc_runtime"] = lock["msvc_runtime"]
    expected_receipt = {"schema": 1, "product": sdk.PRODUCT, "version": sdk.VERSION,
                        "selected_platform": platform, "identity": identity,
                        "lock_sha256": hashlib.sha256(lock_bytes).hexdigest(),
                        "pins": {key: lock[key] for key in ("artifact", "metadata", "file_manifest")},
                        "attestations_verified": True, "runtime_bundled": False,
                        "files": len(entries), "installed_bytes": sum(entry["bytes"] for entry in entries.values())}
    if (not isinstance(receipt, dict) or receipt.get("attestations_verified") is not True
            or any(receipt.get(key) != value for key, value in expected_receipt.items())):
        raise ValueError("Browser SDK receipt differs from the selected committed identity")
    return receipt, entries


def stage(prefix, lock_path, platform, runtime=None, licenses=None, sdk=SDK):
    receipt, entries = verify(prefix, lock_path, platform, sdk)
    if runtime:
        names = []
        for name in entries:
            path = Path(name)
            if platform.startswith("linux-"):
                selected = path.parent.as_posix() == "lib" and path.name.startswith("libSpatialWebBrowser.so")
            elif platform.startswith("darwin-"):
                selected = path.parent.as_posix() == "lib" and path.name.startswith("libSpatialWebBrowser") and path.suffix == ".dylib"
            else:
                selected = path.parent.as_posix() == "bin" and path.name == "SpatialWebBrowser.dll"
            if selected:
                names.append(name)
        if not names:
            raise ValueError("Browser SDK has no selected platform shared runtime")
        runtime.mkdir(parents=True, exist_ok=True)
        for name in names:
            destination = runtime / Path(name).name
            if destination.exists() or destination.is_symlink():
                raise ValueError("Browser runtime destination already exists")
            shutil.copy2(prefix / name, destination)
    if licenses:
        notices = [name for name in entries if name.startswith("share/SpatialWebBrowser/")]
        if not notices:
            raise ValueError("Browser SDK lacks installed license notices")
        licenses.mkdir(parents=True, exist_ok=False)
        for name in notices:
            destination = licenses / Path(name).relative_to("share/SpatialWebBrowser")
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(prefix / name, destination)
        (licenses / "consumer-receipt.json").write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n")
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--sdk", type=Path, required=True)
    parser.add_argument("--lock", type=Path, default=Path(__file__).resolve().parents[2] / "dependencies/spatialweb-browser.lock.json")
    parser.add_argument("--profile", choices=("native", "flatpak"), default="native")
    parser.add_argument("--platform", required=True)
    parser.add_argument("--runtime", type=Path)
    parser.add_argument("--licenses", type=Path)
    args = parser.parse_args()
    sdk = SDK
    if args.profile == "flatpak":
        spec = importlib.util.spec_from_file_location(
            "desktop_flatpak_browser_preparer", PREPARER.with_name("prepare-flatpak-browser-sdk.py"))
        sdk = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(sdk)
    try:
        receipt = stage(args.sdk, args.lock, args.platform, args.runtime, args.licenses, sdk)
    except (OSError, ValueError) as error:
        parser.exit(1, f"Browser runtime preparation failed: {error}\n")
    print(json.dumps(receipt, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
