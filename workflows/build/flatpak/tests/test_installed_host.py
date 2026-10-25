"""Installed-host qualification must execute real selected fixtures and reject skips."""
import importlib.util
import json
import os
import sys
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


specification = importlib.util.spec_from_file_location(
    "installed_host", Path(__file__).resolve().parents[4] / 'workflows/build/flatpak/assets/scripts/verify-installed-host.py')
installed_host = importlib.util.module_from_spec(specification)
specification.loader.exec_module(installed_host)


class InstalledHostQualificationTests(unittest.TestCase):
    def make_fixture(self, root):
        bundle = root / "candidate.flatpak"
        bundle.write_bytes(b"candidate bundle")
        binaries = root / "qt-build"
        binaries.mkdir()
        for executable, _, _ in installed_host.FIXTURES:
            (binaries / executable).write_bytes(b"test fixture")
        installation = root / "installation"
        installation.mkdir()
        return bundle, binaries, installation

    def test_uses_owned_installation_bus_platform_and_native_slots(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            bundle, binaries, installation = self.make_fixture(root)
            calls = []

            def run(arguments, environment, log, timeout=90):
                calls.append((arguments, environment.copy(), timeout))
                if "nativeFlatpakHostPty" in arguments:
                    return "Totals: 5 passed, 0 failed, 0 skipped"
                if "nativeFlatpakHostIntegration" in arguments:
                    return "Totals: 3 passed, 0 failed, 0 skipped"
                return ""

            with patch.dict(os.environ, {"FLATPAK_USER_DIR": str(installation)}):
                with patch.object(installed_host, "run_command", side_effect=run):
                    result = installed_host.verify(bundle, binaries, root / "proof")
            self.assertEqual(len(calls), 4)
            self.assertEqual(calls[1][0][:3], ["dbus-run-session", "--", "flatpak"])
            self.assertIn("--no-deps", calls[1][0])
            self.assertIn("--bundle", calls[1][0])
            for command, environment, timeout in calls[2:]:
                self.assertEqual(command[:4], ["dbus-run-session", "--", "flatpak", "run"])
                self.assertIn(f"--filesystem={binaries}:ro", command)
                self.assertIn("--env=DOCKPIPE_TEST_FLATPAK_HOST_NATIVE=1", command)
                self.assertNotIn("--devel", command)
                self.assertEqual(environment["FLATPAK_USER_DIR"], str(installation))
                self.assertTrue(Path(environment["HOME"]).is_relative_to(root / "proof"))
                self.assertEqual(timeout, 90)
            self.assertIn("nativeFlatpakHostCleanup", calls[2][0])
            self.assertEqual([item["totals"]["passed"] for item in result["fixtures"]], [5, 3])
            self.assertEqual(json.loads((root / "proof/qualification.json").read_text())["app_id"],
                             installed_host.APP_ID)

    def test_refuses_to_replace_an_existing_app(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            bundle, binaries, installation = self.make_fixture(root)
            (installation / "app" / installed_host.APP_ID).mkdir(parents=True)
            with patch.dict(os.environ, {"FLATPAK_USER_DIR": str(installation)}):
                with patch.object(installed_host, "run_command") as run:
                    with self.assertRaisesRegex(RuntimeError, "Refusing to replace"):
                        installed_host.verify(bundle, binaries, root / "proof")
                    run.assert_not_called()

    def test_native_skips_fail_qualification_before_receipt(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            bundle, binaries, installation = self.make_fixture(root)
            with patch.dict(os.environ, {"FLATPAK_USER_DIR": str(installation)}):
                with patch.object(installed_host, "run_command", side_effect=[
                        "", "", "Totals: 2 passed, 0 failed, 3 skipped"]):
                    with self.assertRaisesRegex(RuntimeError, "zero skips"):
                        installed_host.verify(bundle, binaries, root / "proof")
            self.assertFalse((root / "proof/qualification.json").exists())

    def test_output_capacity_is_enforced_while_reading(self):
        with tempfile.TemporaryDirectory() as temporary:
            log = Path(temporary) / "overflow.log"
            with patch.object(installed_host, "MAX_DIAGNOSTIC_BYTES", 128):
                with self.assertRaisesRegex(RuntimeError, "diagnostic exceeded"):
                    installed_host.run_command([sys.executable, "-c", "print('x' * 10000)"],
                                               os.environ.copy(), log, 5)
            self.assertEqual(log.stat().st_size, 128)

    def test_timeout_preserves_bounded_diagnostics(self):
        with tempfile.TemporaryDirectory() as temporary:
            log = Path(temporary) / "timeout.log"
            with self.assertRaisesRegex(RuntimeError, "timed out.*|fixture-started") as failure:
                installed_host.run_command([sys.executable, "-c",
                    "import time; print('fixture-started', flush=True); time.sleep(30)"],
                    os.environ.copy(), log, 0.1)
            self.assertIn("fixture-started", str(failure.exception))

    def test_failure_missing_summary_and_wrong_selection_cannot_pass(self):
        for text in ("Totals: 4 passed, 1 failed, 0 skipped", "", "Totals: 2 passed, 0 failed, 0 skipped"):
            with self.subTest(text=text):
                with self.assertRaises(RuntimeError):
                    installed_host.validate_fixture(text, 5)


if __name__ == "__main__":
    unittest.main()
