"""Network-free consumer boundary tests; only publisher attestation is mocked."""
import copy
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest
from unittest.mock import patch


HELPER = Path(__file__).resolve().parents[1] / "prepare-browser-sdk.py"
SPEC = importlib.util.spec_from_file_location("prepare_browser_sdk", HELPER)
SDK = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SDK)
SOURCE_SHA = "a" * 40


def checksum(data):
    return hashlib.sha256(data).hexdigest()


def encoded(data):
    return (json.dumps(data, indent=2) + "\n").encode()


def pin(name, data):
    return {"name": name, "size_bytes": len(data), "sha256": checksum(data)}


class BrowserSDKPreparationTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.assets = self.root / "assets"
        self.assets.mkdir()
        self.lock_path = self.root / "browser.lock.json"
        self.output = self.root / "installed"
        self.lock = {
            "schema": 1,
            "product": SDK.PRODUCT,
            "version": "0.1.0",
            "qt_version": "6.10.3",
            "source_repository": SDK.REPOSITORY,
            "source_sha": SOURCE_SHA,
            "release": {"repository": SDK.REPOSITORY, "tag": "browser-sdk-0.1.0-staging." + SOURCE_SHA},
            "platforms": {},
        }
        self.payload = {
            "include/SpatialWeb/Browser/BrowserWidget.h": b"// browser public header\n",
            "lib/cmake/SpatialWebBrowser/SpatialWebBrowserConfig.cmake": b"# config\n",
            "lib/cmake/SpatialWebBrowser/SpatialWebBrowserConfigVersion.cmake": b"# version\n",
            "lib/cmake/SpatialWebBrowser/SpatialWebBrowserTargets.cmake": b"# targets\n",
            "share/SpatialWebBrowser/LICENSE": b"license fixture\n",
        }
        for platform in sorted(SDK.PLATFORMS):
            self.make_candidate(platform)
        self.write_lock()

    def tearDown(self):
        self.temporary.cleanup()

    def write_lock(self):
        self.lock_path.write_bytes(encoded(self.lock))

    def make_candidate(self, platform, *, payload=None, members=None, metadata_update=None):
        payload = self.payload if payload is None else payload
        stem = SDK.PRODUCT + "-0.1.0-" + platform
        archive_name = stem + ".tar.gz"
        archive_path = self.assets / archive_name
        with tarfile.open(archive_path, "w:gz") as bundle:
            if members is None:
                for name, data in payload.items():
                    item = tarfile.TarInfo(name)
                    item.size = len(data)
                    item.mode = 0o644
                    bundle.addfile(item, io.BytesIO(data))
            else:
                for item, data in members:
                    bundle.addfile(item, io.BytesIO(data) if data is not None else None)
        archive = pin(archive_name, archive_path.read_bytes())
        inventory = {"schema": 1, "artifact_class": "sdk", "files": [
            {"path": name, "bytes": len(data), "sha256": checksum(data)}
            for name, data in payload.items()
        ]}
        manifest_bytes = encoded(inventory)
        manifest = pin(stem + ".files.json", manifest_bytes)
        compiler_id = "GNU" if platform.startswith("linux-") else "AppleClang"
        if platform == "windows-amd64":
            compiler_id = "MSVC"
        compiler = {"id": compiler_id, "version": "11.4.0"}
        metadata = {
            "schema": 1, "product": SDK.PRODUCT, "version": "0.1.0", "platform": platform,
            "qt_version": "6.10.3", "source_repository": SDK.REPOSITORY, "source_sha": SOURCE_SHA,
            "source_state": "committed-snapshot", "runtime_bundled": False,
            "compiler": compiler, "abi": {"cxx_standard": 17},
            "msvc_runtime": "dynamic-release-/MD" if platform == "windows-amd64" else None,
            "archive": archive, "file_manifest": {"name": manifest["name"], "sha256": manifest["sha256"]},
            "checks": ["sdk-component-install", "focused-browser-tests", "relocated-external-consumer-compile-link-run"],
        }
        if metadata_update:
            metadata.update(metadata_update)
        metadata_bytes = encoded(metadata)
        self.assets.joinpath(manifest["name"]).write_bytes(manifest_bytes)
        self.assets.joinpath(stem + ".release.json").write_bytes(metadata_bytes)
        entry = {"artifact": archive, "metadata": pin(stem + ".release.json", metadata_bytes),
                 "file_manifest": manifest, "compiler": compiler}
        if platform == "windows-amd64":
            entry["msvc_runtime"] = "dynamic-release-/MD"
        self.lock["platforms"][platform] = entry

    def prepare(self, platform="linux-amd64"):
        self.write_lock()
        with patch.object(SDK, "verify_attestation") as verifier:
            receipt = SDK.prepare(self.lock_path, platform, self.output, self.assets)
        return receipt, verifier

    def test_all_five_platforms_preserve_lock_and_receipt(self):
        for platform in sorted(SDK.PLATFORMS):
            with self.subTest(platform=platform):
                self.output = self.root / platform
                receipt, verifier = self.prepare(platform)
                self.assertEqual(3, verifier.call_count)
                self.assertEqual({SOURCE_SHA}, {call.args[1] for call in verifier.call_args_list})
                self.assertEqual(platform, receipt["selected_platform"])
                self.assertEqual(checksum(self.lock_path.read_bytes()), receipt["lock_sha256"])
                self.assertEqual(self.lock["platforms"][platform]["compiler"], receipt["identity"]["compiler"])
                self.assertEqual(self.lock["platforms"][platform]["artifact"], receipt["pins"]["artifact"])
                self.assertEqual(str(self.output.resolve()), receipt["installed_prefix"])
                self.assertEqual(self.payload["include/SpatialWeb/Browser/BrowserWidget.h"],
                                 self.output.joinpath("include/SpatialWeb/Browser/BrowserWidget.h").read_bytes())
                self.assertEqual(self.lock_path.read_bytes(), self.output.joinpath("consumer-lock.json").read_bytes())
                self.assertEqual(receipt, json.loads(self.output.joinpath("consumer-receipt.json").read_text()))

    def test_rejects_partial_matrix_and_moving_release(self):
        del self.lock["platforms"]["darwin-arm64"]
        with self.assertRaisesRegex(ValueError, "five exact platforms"):
            self.prepare()
        self.make_candidate("darwin-arm64")
        self.lock["release"]["tag"] = "latest"
        with self.assertRaisesRegex(ValueError, "immutable"):
            self.prepare()
        self.assertFalse(self.output.exists())

    def test_exact_qt_source_compiler_and_windows_runtime(self):
        original = copy.deepcopy(self.lock)
        mutations = [
            lambda lock: lock.update(qt_version="6.10.2"),
            lambda lock: lock.update(source_sha="main"),
            lambda lock: lock["platforms"]["linux-amd64"].update(compiler={"id": "Clang", "version": "11.0.0"}),
            lambda lock: lock["platforms"]["windows-amd64"].update(msvc_runtime="static-/MT"),
        ]
        for mutate in mutations:
            self.lock = copy.deepcopy(original)
            mutate(self.lock)
            with self.assertRaises(ValueError):
                self.prepare()
            self.assertFalse(self.output.exists())

    def test_pinned_assets_reject_modified_bytes(self):
        entry = self.lock["platforms"]["linux-amd64"]
        for key in ("artifact", "metadata", "file_manifest"):
            path = self.assets / entry[key]["name"]
            original = path.read_bytes()
            path.write_bytes(original + b"changed")
            with self.assertRaisesRegex(ValueError, "pinned|admitted"):
                self.prepare()
            self.assertFalse(self.output.exists())
            path.write_bytes(original)

    def test_metadata_rejects_wrong_abi_source_runtime_and_missing_checks(self):
        for updates in [
            {"qt_version": "6.10.2"}, {"source_sha": "b" * 40}, {"runtime_bundled": True},
            {"compiler": {"id": "GNU", "version": "13.0.0"}},
            {"abi": {"cxx_standard": 20}}, {"checks": []}, {"checks": [{}]},
        ]:
            with self.subTest(updates=updates):
                self.make_candidate("linux-amd64", metadata_update=updates)
                with self.assertRaises(ValueError):
                    self.prepare()
                self.assertFalse(self.output.exists())

    def test_local_assets_still_require_attestation(self):
        with patch.object(SDK, "verify_attestation", side_effect=subprocess.CalledProcessError(1, "gh")) as verifier:
            with self.assertRaises(subprocess.CalledProcessError):
                SDK.prepare(self.lock_path, "linux-amd64", self.output, self.assets)
        verifier.assert_called_once()
        self.assertFalse(self.output.exists())
        self.assertEqual([], list(self.root.glob(".browser-sdk-*")))

    def test_verifier_binds_exact_publisher_workflow_commit_and_dev(self):
        with patch.object(SDK.subprocess, "run") as runner:
            SDK.verify_attestation(Path("asset.tar.gz"), SOURCE_SHA)
        command = runner.call_args.args[0]
        self.assertIn(SDK.REPOSITORY + "/.github/workflows/browser-sdk.yml", command)
        self.assertEqual(SOURCE_SHA, command[command.index("--source-digest") + 1])
        self.assertEqual("refs/heads/dev", command[command.index("--source-ref") + 1])
        self.assertTrue(runner.call_args.kwargs["check"])

    def test_rejects_symlinks_and_missing_payload(self):
        link = tarfile.TarInfo("include/SpatialWeb/Browser/BrowserWidget.h")
        link.type = tarfile.SYMTYPE
        link.linkname = "../../outside"
        self.make_candidate("linux-amd64", members=[(link, None)])
        with self.assertRaisesRegex(ValueError, "link"):
            self.prepare()
        self.assertFalse(self.output.exists())
        self.make_candidate("linux-amd64", members=[])
        with self.assertRaisesRegex(ValueError, "missing"):
            self.prepare()

    def test_rejects_traversal_duplicates_and_case_collisions(self):
        for unsafe in ["../escape", "/absolute", "C:/escape", "include/a\\b", "include/CON.txt", "include/end."]:
            with self.subTest(unsafe=unsafe):
                payload = dict(self.payload, **{unsafe: b"bad"})
                self.make_candidate("linux-amd64", payload=payload)
                with self.assertRaises(ValueError):
                    self.prepare()
        payload = dict(self.payload, **{"INCLUDE/SpatialWeb/Browser/BrowserWidget.h": b"collision"})
        self.make_candidate("linux-amd64", payload=payload)
        with self.assertRaisesRegex(ValueError, "duplicate"):
            self.prepare()
        self.assertFalse(self.root.joinpath("escape").exists())

    def test_no_qt_or_optional_spatial_runtime_bundling(self):
        for forbidden in ["lib/libQt6Core.so.6", "bin/Qt6Core.dll", "lib/QtWebEngineCore.framework/a", "lib/QtCore.framework/a",
                          "plugins/platforms/qxcb.so", "lib/cmake/SpatialWebProjection/config.cmake"]:
            self.make_candidate("linux-amd64", payload=dict(self.payload, **{forbidden: b"runtime"}))
            with self.assertRaisesRegex(ValueError, "must not bundle"):
                self.prepare()
        self.assertFalse(self.output.exists())

    def test_fresh_destination_and_local_asset_link(self):
        self.output.mkdir()
        self.output.joinpath("preserve").write_text("original")
        with self.assertRaisesRegex(ValueError, "fresh"):
            self.prepare()
        self.assertEqual("original", self.output.joinpath("preserve").read_text())
        self.output.joinpath("preserve").unlink()
        self.output.rmdir()
        name = self.lock["platforms"]["linux-amd64"]["metadata"]["name"]
        original = self.assets / name
        original.rename(self.assets / "original")
        original.symlink_to("original")
        with self.assertRaisesRegex(ValueError, "regular"):
            self.prepare()

    def test_wrong_member_checksum_duplicate_and_special_files(self):
        members = []
        for name, data in self.payload.items():
            item = tarfile.TarInfo(name)
            item.size = len(data)
            members.append((item, data))
        bad = list(members)
        first, data = bad[0]
        bad[0] = (first, b"x" * len(data))
        self.make_candidate("linux-amd64", members=bad)
        with self.assertRaisesRegex(ValueError, "checksum"):
            self.prepare()
        self.make_candidate("linux-amd64", members=members + [members[0]])
        with self.assertRaisesRegex(ValueError, "Duplicate"):
            self.prepare()
        for kind in (tarfile.LNKTYPE, tarfile.FIFOTYPE, tarfile.CHRTYPE):
            special = tarfile.TarInfo("include/SpatialWeb/Browser/BrowserWidget.h")
            special.type = kind
            special.linkname = "elsewhere"
            self.make_candidate("linux-amd64", members=[(special, None)])
            with self.assertRaises(ValueError):
                self.prepare()
        self.assertFalse(self.output.exists())

    def test_inventory_and_extended_headers_capacity(self):
        with patch.object(SDK, "MAX_INSTALLED_BYTES", 1):
            with self.assertRaisesRegex(ValueError, "capacity"):
                self.prepare()
        with patch.object(SDK, "MAX_FILES", 1):
            with self.assertRaisesRegex(ValueError, "inventory"):
                self.prepare()
        header = tarfile.TarInfo("header")
        header.type = tarfile.XHDTYPE
        header.size = SDK.MAX_METADATA_BYTES + 1
        with self.assertRaisesRegex(ValueError, "header exceeds"):
            SDK.BoundedTarInfo.frombuf(header.tobuf(), "utf-8", "surrogateescape")
        self.assertFalse(self.output.exists())

    def test_duplicate_json_and_decompressed_archive_bound(self):
        self.lock_path.write_text('{"schema": 1, "schema": 1}')
        with self.assertRaisesRegex(ValueError, "Duplicate"):
            SDK.load_lock(self.lock_path, "linux-amd64")
        with patch.object(SDK, "MAX_TAR_BYTES", 100):
            with self.assertRaisesRegex(ValueError, "decompressed"):
                self.prepare()
        self.assertFalse(self.output.exists())


if __name__ == "__main__":
    unittest.main()
