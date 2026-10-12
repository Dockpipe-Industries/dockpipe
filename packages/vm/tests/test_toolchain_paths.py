#!/usr/bin/env python3
"""Exercise QEMU path admission without source archives, Docker, or live writes."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


PACKAGE = Path(__file__).resolve().parents[1]
RECIPE = PACKAGE / "toolchains/qemu-11.0.3-linux-amd64"


class ToolchainPathTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="dockpipe-qemu-paths-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.environment = dict(os.environ)
        for name in list(self.environment):
            if name.startswith("DOCKPIPE_QEMU_"):
                self.environment.pop(name)
        self.paths = {
            "DOCKPIPE_QEMU_SOURCE_DIR": str(self.root / "inputs"),
            "DOCKPIPE_QEMU_BUILD_ROOT": str(self.root / "records"),
            "DOCKPIPE_QEMU_FINAL_ROOT": str(self.root / "toolchain"),
        }

    def check(self, paths):
        return subprocess.run(
            ["bash", str(RECIPE / "materialize.sh"), "--check-config"],
            env={**self.environment, **paths}, capture_output=True, text=True,
        )

    def test_explicit_paths_are_reported_without_creating_anything(self):
        result = self.check(self.paths)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(f"final_root={self.paths['DOCKPIPE_QEMU_FINAL_ROOT']}\n", result.stdout)
        self.assertIn(f"publish_root={self.paths['DOCKPIPE_QEMU_FINAL_ROOT']}.partial\n", result.stdout)
        self.assertEqual(list(self.root.iterdir()), [])

    def test_missing_unsafe_and_overlapping_paths_fail_before_writes(self):
        for name in self.paths:
            paths = dict(self.paths)
            paths.pop(name)
            self.assertNotEqual(self.check(paths).returncode, 0, name)
        for value in ("", "/", "relative", "/tmp/../toolchain", "/tmp/a,b", "/tmp/a b", "/tmp/a\nb"):
            with self.subTest(value=value):
                paths = dict(self.paths, DOCKPIPE_QEMU_FINAL_ROOT=value)
                self.assertNotEqual(self.check(paths).returncode, 0)
        for name in ("DOCKPIPE_QEMU_SOURCE_DIR", "DOCKPIPE_QEMU_BUILD_ROOT"):
            for suffix in ("", "/child"):
                paths = dict(self.paths, DOCKPIPE_QEMU_FINAL_ROOT=self.paths[name] + suffix)
                self.assertNotEqual(self.check(paths).returncode, 0)
        paths = dict(self.paths, DOCKPIPE_QEMU_BUILD_ROOT=str(RECIPE / "records"))
        self.assertNotEqual(self.check(paths).returncode, 0)
        self.assertEqual(list(self.root.iterdir()), [])

    def test_symlinked_ancestor_is_rejected(self):
        target = self.root / "target"
        target.mkdir()
        link = self.root / "link"
        link.symlink_to(target, target_is_directory=True)
        paths = dict(self.paths, DOCKPIPE_QEMU_FINAL_ROOT=str(link / "toolchain"))
        self.assertNotEqual(self.check(paths).returncode, 0)
        self.assertEqual(list(target.iterdir()), [])

    def test_shipped_configuration_has_no_personal_paths_or_historical_pins(self):
        for directory in (PACKAGE / "manifests", RECIPE):
            for path in directory.rglob("*"):
                if path.is_file():
                    self.assertNotRegex(path.read_text(), r"/(?:home|Users)/[A-Za-z0-9_.-]+/", str(path))
        template = json.loads((PACKAGE / "manifests/linux-provisioning.template.json").read_text())
        self.assertTrue(template["toolchain"]["manifest_sha256"].startswith("REPLACE_"))
        fixture = json.loads((PACKAGE / "manifests/linux-qualification.json").read_text())
        self.assertFalse(fixture["live_execution_approved"])
        self.assertEqual(fixture["qemu"]["binary_sha256"], "a" * 64)


if __name__ == "__main__":
    unittest.main()
