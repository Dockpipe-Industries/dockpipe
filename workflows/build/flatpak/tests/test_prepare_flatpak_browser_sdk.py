"""Exact Flatpak profile isolation, payload and installed readback boundaries."""
import copy
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch


def module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


SDK = module("flatpak_sdk_fixture", Path(__file__).resolve().parents[4] / 'workflows/build/flatpak/assets/scripts/prepare-flatpak-browser-sdk.py')
RUNTIME = module("flatpak_runtime_fixture", Path(__file__).resolve().parents[4] / 'workflows/dependencies/browser-sdk/assets/scripts/browser-runtime.py')


def encoded(value):
    return (json.dumps(value, indent=2) + "\n").encode()


def pin(name, data):
    return {"name": name, "size_bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}


class FlatpakBrowserSDKTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.assets = self.root / "assets"
        self.assets.mkdir()
        self.output = self.root / "sdk"
        self.lock_path = self.root / "flatpak.lock.json"
        self.payload = {
            "include/SpatialWeb/Browser/BrowserWidget.h": b"public API",
            "lib/libSpatialWebBrowser.so.0.1.0": b"qualified shared runtime",
            "share/SpatialWebBrowser/LICENSE": b"license",
        }
        package = "SpatialWebFlatpakBrowser"
        for suffix in ("Config", "ConfigVersion", "Targets"):
            self.payload[f"lib/cmake/{package}/{package}{suffix}.cmake"] = b"package fixture"
        self.lock = {"schema": 1, "product": SDK.PRODUCT, "version": SDK.VERSION,
                     "platform": SDK.PLATFORM, "source_repository": SDK.REPOSITORY,
                     "source_sha": "b" * 40, "compiler": copy.deepcopy(SDK.COMPILER),
                     "qt_components": copy.deepcopy(SDK.QT_COMPONENTS), "flatpak": copy.deepcopy(SDK.FLATPAK),
                     "release": {"repository": SDK.REPOSITORY,
                                 "tag": "flatpak-browser-sdk-0.1.0-staging." + "b" * 40}}
        self.metadata_changes = {}
        self.write_candidate()

    def tearDown(self):
        self.temporary.cleanup()

    def write_candidate(self):
        stem = SDK.PRODUCT + "-0.1.0-" + SDK.PLATFORM
        archive = self.assets / (stem + ".tar.gz")
        with tarfile.open(archive, "w:gz") as bundle:
            for name, data in self.payload.items():
                member = tarfile.TarInfo(name)
                member.size = len(data)
                bundle.addfile(member, io.BytesIO(data))
        self.lock["artifact"] = pin(archive.name, archive.read_bytes())
        inventory = {"schema": 1, "artifact_class": "sdk", "files": [
            {"path": name, "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}
            for name, data in self.payload.items()]}
        inventory_bytes = encoded(inventory)
        self.lock["file_manifest"] = pin(stem + ".files.json", inventory_bytes)
        self.metadata = SDK.receipt_identity(self.lock)
        self.metadata.pop("release")
        self.metadata.update(schema=1, product=SDK.PRODUCT, version=SDK.VERSION, platform=SDK.PLATFORM,
                             source_state="committed-snapshot", runtime_bundled=False,
                             abi={"cxx_standard": 17}, archive=self.lock["artifact"],
                             file_manifest={key: self.lock["file_manifest"][key] for key in ("name", "sha256")},
                             checks=["sdk-component-install", "focused-browser-tests",
                                     "relocated-external-consumer-compile-link-run", "flatpak-installed-runtime-dom"])
        self.metadata.update(self.metadata_changes)
        metadata_bytes = encoded(self.metadata)
        self.lock["metadata"] = pin(stem + ".release.json", metadata_bytes)
        (self.assets / self.lock["metadata"]["name"]).write_bytes(metadata_bytes)
        (self.assets / self.lock["file_manifest"]["name"]).write_bytes(inventory_bytes)
        self.write_lock()

    def write_lock(self):
        self.lock_path.write_bytes(encoded(self.lock))

    def prepare(self):
        with patch.object(SDK, "verify_attestation") as verifier:
            receipt = SDK.prepare(self.lock_path, SDK.PLATFORM, self.output, self.assets)
        self.assertEqual(3, verifier.call_count)
        return receipt

    def test_local_assets_still_attested_and_runtime_only_staging(self):
        receipt = self.prepare()
        self.assertEqual(SDK.PRODUCT, receipt["product"])
        self.assertEqual(SDK.QT_COMPONENTS, receipt["identity"]["qt_components"])
        self.assertEqual(SDK.FLATPAK, receipt["identity"]["flatpak"])
        self.assertNotIn("qt_version", receipt["identity"])
        self.assertEqual(hashlib.sha256(self.lock_path.read_bytes()).hexdigest(), receipt["lock_sha256"])
        destination = self.root / "runtime"
        RUNTIME.stage(self.output, self.lock_path, SDK.PLATFORM, destination, self.root / "licenses", sdk=SDK)
        self.assertEqual(["libSpatialWebBrowser.so.0.1.0"], [path.name for path in destination.iterdir()])
        with self.assertRaises(ValueError):
            RUNTIME.verify(self.output, self.lock_path, "linux-amd64")

    def test_profile_rejects_native_identity_and_single_qt_version(self):
        for change in ({"product": "SpatialWebBrowserSDK"}, {"platform": "linux-amd64"},
                       {"qt_version": "6.10.3"}):
            original = copy.deepcopy(self.lock)
            self.lock.update(change)
            self.write_lock()
            with self.assertRaises(ValueError):
                SDK.load_lock(self.lock_path, SDK.PLATFORM)
            self.lock = original

    def test_exact_refs_components_and_compiler_cannot_drift(self):
        for section, name, value in (("qt_components", "Qt6WebEngineCore", "6.10.3"),
                                     ("compiler", "version", "15.2.1"),
                                     ("flatpak", "base", {"ref": SDK.FLATPAK["base"]["ref"], "commit": "0" * 64})):
            original = copy.deepcopy(self.lock)
            self.lock[section][name] = value
            self.write_lock()
            with self.assertRaisesRegex(ValueError, "exact supported profile"):
                SDK.load_lock(self.lock_path, SDK.PLATFORM)
            self.lock = original

    def test_metadata_requires_component_identity_and_runtime_dom_proof(self):
        for change in ({"qt_version": "6.10.3"}, {"flatpak": {}}, {"compiler": {"id": "GNU", "version": "11.4.0"}},
                       {"checks": ["sdk-component-install", "focused-browser-tests", "relocated-external-consumer-compile-link-run"]}):
            self.metadata_changes = change
            self.write_candidate()
            with self.assertRaises(ValueError):
                self.prepare()
            self.assertFalse(self.output.exists())

    def test_installed_readback_detects_modified_binary_and_receipt(self):
        self.prepare()
        binary = self.output / "lib/libSpatialWebBrowser.so.0.1.0"
        original = binary.read_bytes()
        binary.write_bytes(b"changed")
        with self.assertRaisesRegex(ValueError, "installed file"):
            RUNTIME.verify(self.output, self.lock_path, SDK.PLATFORM, SDK)
        binary.write_bytes(original)
        receipt = self.output / "consumer-receipt.json"
        data = json.loads(receipt.read_text())
        data["identity"]["flatpak"]["base"]["commit"] = "0" * 64
        receipt.write_bytes(encoded(data))
        with self.assertRaisesRegex(ValueError, "receipt"):
            RUNTIME.verify(self.output, self.lock_path, SDK.PLATFORM, SDK)

    def test_archive_tamper_never_installs(self):
        archive = self.assets / self.lock["artifact"]["name"]
        archive.write_bytes(archive.read_bytes() + b"tamper")
        with self.assertRaises(ValueError):
            self.prepare()
        self.assertFalse(self.output.exists())

    def test_installed_flatpak_refs_require_exact_commits(self):
        commits = {value["ref"]: value["commit"] for value in SDK.FLATPAK.values()}
        calls = []
        def command(arguments, **kwargs):
            calls.append(arguments)
            return commits[arguments[-1]] + "\n"
        SDK.verify_refs(self.lock, command)
        self.assertEqual(3, len(calls))
        commits[SDK.FLATPAK["runtime"]["ref"]] = "0" * 64
        with self.assertRaisesRegex(ValueError, "runtime commit"):
            SDK.verify_refs(self.lock, command)

    def test_attestation_uses_separate_workflow_and_exact_dev_source(self):
        with patch.object(SDK.BASE.subprocess, "run") as command:
            SDK.verify_attestation(Path("asset"), "b" * 40)
        arguments = command.call_args.args[0]
        self.assertIn(SDK.REPOSITORY + "/.github/workflows/flatpak-browser-sdk.yml", arguments)
        self.assertIn("refs/heads/dev", arguments)
        self.assertIn("b" * 40, arguments)


if __name__ == "__main__":
    unittest.main()
