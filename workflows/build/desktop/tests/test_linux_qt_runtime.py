"""Failure boundaries for the native Qt runtime packaging contract."""
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


SCRIPT = Path(__file__).resolve().parents[4] / 'workflows/build/desktop/assets/scripts/bundle-linux-qt.py'
SPEC = importlib.util.spec_from_file_location("bundle_linux_qt", SCRIPT)
bundle = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(bundle)


class LinuxQtRuntimeTests(unittest.TestCase):
    def test_missing_transitive_dependency_is_fatal(self):
        with patch.object(bundle, "command", return_value="libQt6Core.so.6 => not found"):
            with self.assertRaisesRegex(RuntimeError, "Unresolved runtime"):
                bundle.dependencies(Path("fixture"))

    def test_glibc_baseline_rejects_newer_sdk_binary(self):
        with patch.object(bundle, "command", return_value="Name: GLIBC_2.36\nName: GLIBC_2.2.5"):
            with self.assertRaisesRegex(RuntimeError, "Ubuntu 22.04"):
                bundle.check_baseline(Path("fixture"))
        with patch.object(bundle, "command", return_value="Name: GLIBC_2.34\nName: GLIBC_2.2.5"):
            bundle.check_baseline(Path("fixture"))

    def test_private_debug_cannot_enter_public_tree(self):
        with tempfile.TemporaryDirectory() as directory:
            stage = Path(directory) / "public"
            with self.assertRaisesRegex(RuntimeError, "outside the public"):
                bundle.deploy(Path("launcher"), Path("qt"), stage, stage / "debug", "0.6.2")

    def test_mismatched_qt_sdk_rejected_before_deployment(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            with patch.object(bundle, "command", return_value="6.2.4"):
                with self.assertRaisesRegex(RuntimeError, "requires Qt 6.10.3"):
                    bundle.deploy(root / "launcher", root / "qt", root / "public", root / "private", "0.6.2")

    def test_system_qt_cannot_masquerade_as_pinned_sdk(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            linked = {"libQt6Core.so.6": Path("/usr/lib/libQt6Core.so.6.10.3")}
            with patch.object(bundle, "command", return_value="6.10.3"), \
                    patch.object(bundle, "dependencies", return_value=linked):
                with self.assertRaisesRegex(RuntimeError, "not built against"):
                    bundle.deploy(root / "launcher", root / "qt", root / "public", root / "private", "0.6.2")


    def test_compatibility_runtime_retains_license_and_actual_source_link(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            package_docs = root / "docs"
            license_file = package_docs / "libjpeg-turbo8/copyright"
            license_file.parent.mkdir(parents=True)
            license_file.write_bytes(b"Fixture runtime license\n")
            notices = root / "notices"
            notices.mkdir()
            with patch.object(bundle, "command", return_value="libjpeg-turbo 2.1.2-0ubuntu1"):
                bundle.copy_compatibility_notice("libjpeg-turbo8", notices, package_docs)
            self.assertEqual((notices / "libjpeg-turbo8/copyright").read_bytes(), license_file.read_bytes())
            provenance = (notices / "libjpeg-turbo8/README").read_text()
            self.assertIn("Source package: libjpeg-turbo 2.1.2-0ubuntu1", provenance)
            self.assertIn("https://launchpad.net/ubuntu/+source/libjpeg-turbo", provenance)
            self.assertNotIn("+source/tiff", provenance)

    def test_compatibility_runtime_missing_license_is_fatal(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            with patch.object(bundle, "command", return_value="libjpeg-turbo 2.1.2-0ubuntu1"):
                with self.assertRaisesRegex(RuntimeError, "Missing compatibility runtime copyright"):
                    bundle.copy_compatibility_notice("libjpeg-turbo8", root / "notices", root / "docs")
            self.assertFalse((root / "notices").exists())

    def test_compatibility_runtime_rejects_malformed_source_identity(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for identity in ("", "libjpeg-turbo", "../outside 1.0", "libjpeg-turbo 1.0 extra"):
                with self.subTest(identity=identity):
                    with patch.object(bundle, "command", return_value=identity):
                        with self.assertRaisesRegex(RuntimeError, "Invalid compatibility source identity"):
                            bundle.copy_compatibility_notice("libjpeg-turbo8", root / "notices", root / "docs")
            self.assertFalse((root / "notices").exists())


    def test_bundled_sonames_create_package_owned_dependency_metadata(self):
        self.assertEqual(bundle.shlibs_entry("libQt6Core.so.6", "0.6.2"),
                         "libQt6Core 6 dockpipe-desktop (= 0.6.2)")
        self.assertEqual(bundle.shlibs_entry("libicuuc.so.73", "0.6.2"),
                         "libicuuc 73 dockpipe-desktop (= 0.6.2)")
        self.assertEqual(bundle.shlibs_entry("libjpeg.so.8", "0.6.31"),
                         "libjpeg 8 dockpipe-desktop (= 0.6.31)")
        with self.assertRaisesRegex(RuntimeError, "Unsupported bundled SONAME"):
            bundle.shlibs_entry("libbad.so.6\nInjected", "0.6.2")


if __name__ == "__main__":
    unittest.main()
