import hashlib
import importlib.util
import shutil
import subprocess
import tempfile
from pathlib import Path
import unittest


SPEC = importlib.util.spec_from_file_location(
    "installed_cockpit_boundary", Path(__file__).with_name("verify-installed-cockpit.py"))
CHECK = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECK)


class InstalledCockpitBoundaryTest(unittest.TestCase):
    def setUp(self):
        self.assets = Path("/owned/package/resolvers/cockpit/assets")
        self.paths = [
            "/usr/lib/x86_64-linux-gnu/libQt6Core.so.6.10.3",
            "/app/lib/x86_64-linux-gnu/libQt6WebEngineCore.so.6.10.2",
            "/app/lib/x86_64-linux-gnu/libQt6WebEngineWidgets.so.6.10.2",
            "/app/lib/libSpatialWebBrowser.so.0",
            "/app/libexec/dockpipe/libdockpipe-launcher-ui.so.1.0",
            str(self.assets / "ui/libdockpipe-cockpit.so"),
        ]

    def mappings(self, paths):
        return "\n".join("1000-2000 r-xp 0 00:00 1 " + path for path in paths)

    def test_exact_host_providers_and_plugin_required(self):
        result = CHECK.providers(self.mappings(self.paths), self.assets)
        self.assertEqual(6, len(result))
        with self.assertRaisesRegex(ValueError, "cockpit providers"):
            CHECK.providers(self.mappings(self.paths[:-1]), self.assets)

    def test_native_or_duplicate_qt_provider_rejected(self):
        native_qt = str(self.assets / "lib/libQt6WebEngineCore.so.6")
        changed = [native_qt if "libQt6WebEngineCore" in path else path for path in self.paths]
        for paths in (changed, self.paths + [native_qt]):
            with self.subTest(paths=paths):
                with self.assertRaisesRegex(ValueError, "qt_webengine_core providers"):
                    CHECK.providers(self.mappings(paths), self.assets)

    def test_build_tree_host_ui_rejected(self):
        paths = ["/build/libdockpipe-launcher-ui.so.1.0" if "libdockpipe-launcher-ui" in path else path
                 for path in self.paths]
        with self.assertRaisesRegex(ValueError, "host_ui providers"):
            CHECK.providers(self.mappings(paths), self.assets)

    def test_frame_completion_recovery_and_compositor_failures(self):
        valid = ("phase=product movement_frames=92 frame_p95_ms=16 compositor_failures=0\n"
                 "phase=synthetic-software-GL completed_frames=93\n"
                 "phase=synthetic-software-GL recovery_completed_frames=1\n")
        self.assertEqual(92, CHECK.rendering_result(valid)["product_movement_frames"])
        for text in (valid.replace("frames=92", "frames=1"),
                     valid.replace("compositor_failures=0", "compositor_failures=1"),
                     valid.replace("completed_frames=93", "completed_frames=0"),
                     valid.replace("recovery_completed_frames=1", "recovery_completed_frames=0")):
            with self.assertRaisesRegex(ValueError, "rendering proof"):
                CHECK.rendering_result(text)


class FlatpakDependencyPreservationTest(unittest.TestCase):
    @unittest.skipUnless(all(shutil.which(tool) for tool in ("cc", "strip", "readelf", "bash")),
                         "Native ELF tools are required for packaging qualification")
    def test_symbol_cleanup_preserves_attested_browser_bytes(self):
        script = Path(__file__).with_name("build.sh").read_text()
        cleanup = script.split("flatpak build \"$app\" bash -c '\n", 1)[1].split("\n'", 1)[0]
        with tempfile.TemporaryDirectory() as temporary:
            app = Path(temporary) / "app"
            library = app / "lib/libSpatialWebBrowser.so.0"
            library.parent.mkdir(parents=True)
            subprocess.run(
                ["cc", "-shared", "-fPIC", "-g", "-x", "c", "-o", str(library), "-"],
                input="int qualification_symbol(void) { return 42; }\n",
                text=True, check=True, capture_output=True)
            original = library.read_bytes()
            owned = [
                app / "libexec/dockpipe/dockpipe-launcher",
                app / "libexec/dockpipe/dockpipe-workspace-reader",
                app / "libexec/dockpipe/libdockpipe-launcher-ui.so.1.0",
                app / "lib/libXRes.so.1",
            ]
            for binary in owned:
                binary.parent.mkdir(parents=True, exist_ok=True)
                binary.write_bytes(original)
            subprocess.run(["bash", "-c", cleanup.replace("/app/", str(app) + "/")],
                           check=True, capture_output=True, text=True)
            self.assertEqual(hashlib.sha256(original).hexdigest(),
                             hashlib.sha256(library.read_bytes()).hexdigest())
            for binary in owned:
                with self.subTest(binary=binary.name):
                    self.assertNotEqual(original, binary.read_bytes())
                    sections = subprocess.check_output(
                        ["readelf", "--wide", "--sections", str(binary)], text=True)
                    self.assertNotIn(".debug_info", sections)
                    self.assertNotIn(".symtab", sections)


if __name__ == "__main__":
    unittest.main()
