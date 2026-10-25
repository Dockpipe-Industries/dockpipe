"""Desktop installed SDK readback and runtime-only staging tests."""
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


def module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


RUNTIME = module("browser_runtime", Path(__file__).resolve().parents[4] / 'workflows/dependencies/browser-sdk/assets/scripts/browser-runtime.py')
FIXTURES = module("browser_fixtures", Path(__file__).resolve().parents[4] / 'workflows/dependencies/browser-sdk/tests/test_prepare_browser_sdk.py')
BUNDLE = module("browser_linux_bundle", Path(__file__).resolve().parents[4] / 'workflows/build/desktop/assets/scripts/bundle-linux-qt.py')
PROTECTOR = module("browser_mac_protector", Path(__file__).resolve().parents[4] / 'workflows/package/macos/assets/scripts/protect-owned-binaries.py')


class BrowserRuntimeTest(unittest.TestCase):
    # Share fixture construction; packaging tests remain separate from the
    # preparer's boundary suite. Only the attestation call is mocked.
    setUp = FIXTURES.BrowserSDKPreparationTest.setUp
    tearDown = FIXTURES.BrowserSDKPreparationTest.tearDown
    write_lock = FIXTURES.BrowserSDKPreparationTest.write_lock
    make_candidate = FIXTURES.BrowserSDKPreparationTest.make_candidate
    prepare = FIXTURES.BrowserSDKPreparationTest.prepare

    def test_stages_runtime_and_notices_without_headers_cmake_or_import_libs(self):
        for platform, library in [
            ("linux-amd64", "lib/libSpatialWebBrowser.so.0.1.0"),
            ("darwin-arm64", "lib/libSpatialWebBrowser.0.1.0.dylib"),
            ("windows-amd64", "bin/SpatialWebBrowser.dll"),
        ]:
            with self.subTest(platform=platform):
                payload = dict(self.payload, **{library: b"shared binary", "lib/SpatialWebBrowser.lib": b"import library"})
                self.make_candidate(platform, payload=payload)
                self.output = self.root / ("sdk-" + platform)
                self.prepare(platform)
                target = self.root / ("runtime-" + platform)
                licenses = self.root / ("notices-" + platform)
                receipt = RUNTIME.stage(self.output, self.lock_path, platform, target, licenses)
                self.assertEqual([Path(library).name], [path.name for path in target.iterdir()])
                self.assertTrue(licenses.joinpath("LICENSE").is_file())
                self.assertTrue(licenses.joinpath("consumer-receipt.json").is_file())
                self.assertEqual(platform, receipt["selected_platform"])

    def test_readback_rejects_changed_file_receipt_and_new_lock(self):
        self.prepare()
        header = self.output / "include/SpatialWeb/Browser/BrowserWidget.h"
        original = header.read_bytes()
        header.write_bytes(b"x" * len(original))
        with self.assertRaisesRegex(ValueError, "installed file"):
            RUNTIME.verify(self.output, self.lock_path, "linux-amd64")
        header.write_bytes(original)
        receipt = self.output / "consumer-receipt.json"
        receipt_bytes = receipt.read_bytes()
        receipt.write_text('{}')
        with self.assertRaisesRegex(ValueError, "receipt"):
            RUNTIME.verify(self.output, self.lock_path, "linux-amd64")
        receipt.write_bytes(receipt_bytes)
        self.lock_path.write_bytes(self.lock_path.read_bytes() + b"\n")
        with self.assertRaisesRegex(ValueError, "committed lock"):
            RUNTIME.verify(self.output, self.lock_path, "linux-amd64")

    def test_readback_rejects_injected_library_and_alias(self):
        self.prepare()
        extra = self.output / "lib/extra.so"
        extra.write_bytes(b"injected")
        with self.assertRaisesRegex(ValueError, "inventory"):
            RUNTIME.verify(self.output, self.lock_path, "linux-amd64")
        extra.unlink()
        extra.symlink_to("../consumer-lock.json")
        with self.assertRaisesRegex(ValueError, "link"):
            RUNTIME.verify(self.output, self.lock_path, "linux-amd64")

    def test_mac_symbol_audit_includes_shipped_reader_and_rejects_escape(self):
        app = self.root / "DockPipe.app"
        launcher = app / "Contents/MacOS/DockPipe"
        library = app / "Contents/Frameworks/libdockpipe-launcher-ui.1.dylib"
        reader = launcher.parent / "dockpipe-workspace-reader"
        launcher.parent.mkdir(parents=True)
        library.parent.mkdir(parents=True)
        for path in (launcher, library, reader):
            path.write_bytes(b"owned Mach-O fixture")
        self.assertEqual({launcher, library, reader}, set(PROTECTOR.owned_binaries(app)))
        outside = self.root / "outside"
        outside.write_bytes(b"preserve")
        reader.unlink()
        reader.symlink_to(outside)
        with self.assertRaisesRegex(ValueError, "reader escapes"):
            PROTECTOR.owned_binaries(app)

    def test_arm_glibc_ceiling_is_explicit_and_does_not_relax_amd64(self):
        with patch.object(BUNDLE, "command", return_value="Name: GLIBC_2.39"):
            BUNDLE.check_baseline(Path("fixture"), BUNDLE.BASELINES["linux-arm64"])
            with self.assertRaisesRegex(RuntimeError, "Ubuntu 22.04"):
                BUNDLE.check_baseline(Path("fixture"), BUNDLE.BASELINES["linux-amd64"])
        with patch.object(BUNDLE, "command", return_value="Name: GLIBC_2.40"):
            with self.assertRaisesRegex(RuntimeError, "Ubuntu 24.04"):
                BUNDLE.check_baseline(Path("fixture"), BUNDLE.BASELINES["linux-arm64"])


if __name__ == "__main__":
    unittest.main()
