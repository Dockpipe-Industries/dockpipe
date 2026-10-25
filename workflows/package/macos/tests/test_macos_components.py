import importlib.util
from pathlib import Path
import plistlib
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[4] / 'workflows/package/macos/assets/scripts/configure-components.py'
SPEC = importlib.util.spec_from_file_location("macos_components", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class MacosComponentsTest(unittest.TestCase):
    def configure(self, components):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "components.plist"
            with path.open("wb") as stream:
                plistlib.dump(components, stream)
            MODULE.configure_components(path)
            with path.open("rb") as stream:
                return plistlib.load(stream)

    def test_targets_application_after_nested_framework(self):
        framework = {"RootRelativeBundlePath": "Applications/DockPipe.app/Contents/Frameworks/QtCore.framework"}
        application = {"RootRelativeBundlePath": "Applications/DockPipe.app", "BundleIsRelocatable": True}
        result = self.configure([framework, application])
        self.assertEqual(result[0], framework)
        self.assertIs(result[1]["BundleIsRelocatable"], False)

    def test_adds_missing_relocation_key(self):
        result = self.configure([{"RootRelativeBundlePath": "Applications/DockPipe.app"}])
        self.assertIs(result[0]["BundleIsRelocatable"], False)

    def test_missing_or_ambiguous_application_fails_closed(self):
        application = {"RootRelativeBundlePath": "Applications/DockPipe.app"}
        for components in [[], [application, application], {"unexpected": "object"}]:
            with self.subTest(components=components), self.assertRaises(ValueError):
                self.configure(components)
