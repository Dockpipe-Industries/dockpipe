"""The installed package preset keeps platform, candidate and driver pins together."""

import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


PRODUCT = Path(__file__).resolve().parents[1]
PREPARER = PRODUCT / "assets/scripts/prepare-plan.py"


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


class PreparePackagesTests(unittest.TestCase):
    def prepare(self, root, profile, extra=()):
        baseline = root / "baseline"
        baseline.write_text("previous package", encoding="utf-8")
        candidate = root / "candidate"
        candidate.write_text("current package", encoding="utf-8")
        output = root / "plan.json"
        command = [sys.executable, "-EB", str(PREPARER), "--package-profile", profile,
                   "--package-baseline", str(baseline), "--package-candidate", str(candidate),
                   "--baseline-version", "0.6.39", "--expected-version", "0.6.44",
                   "--run-id", "fixture", "--output", str(output), *extra]
        result = subprocess.run(command, capture_output=True, text=True, timeout=15, check=False)
        return result, output, baseline, candidate

    def test_each_pinned_container_prepares_one_driver_for_both_installed_cases(self):
        registry = json.loads((PRODUCT / "registry.json").read_text())
        profiles = [entry for entry in registry["platforms"]
                    if "image_digest" in entry.get("baseline", {})]
        self.assertEqual(len(profiles), 4)
        for profile in profiles:
            with self.subTest(profile=profile["id"]), tempfile.TemporaryDirectory() as temporary:
                result, output, baseline, candidate = self.prepare(Path(temporary), profile["id"])
                self.assertEqual(result.returncode, 0, result.stderr)
                target, = json.loads(output.read_text())["targets"]
                driver, = target["drivers"]
                self.assertEqual(target["platform"], profile["id"])
                self.assertEqual(target["kind"], "container")
                self.assertEqual(target["artifact_sha256"], digest(candidate))
                self.assertEqual(driver["configuration"]["prior_artifact_sha256"], digest(baseline))
                self.assertEqual(driver["configuration"]["image"], profile["baseline"]["image_digest"])
                self.assertEqual(driver["features"], ["install.cli", "install.upgrade"])
                self.assertEqual(driver["scenarios"], ["happy"])
                self.assertEqual(driver["layers"], ["installed_e2e"])
                self.assertEqual(driver["sha256"], digest(PRODUCT / driver["entrypoint"]))
                for dependency in driver["dependencies"]:
                    self.assertEqual(dependency["sha256"], digest(PRODUCT / dependency["path"]))

    def test_rejects_vm_and_mismatched_or_out_of_order_options_before_write(self):
        for profile, extra in [
            ("rhel-9.6-amd64", []),
            ("rhel-ubi-9.6-amd64", ["--docker-image", "wrong-mode"]),
            ("rhel-ubi-9.6-amd64", ["--expected-version", "0.6.38"]),
        ]:
            with self.subTest(profile=profile, extra=extra), tempfile.TemporaryDirectory() as temporary:
                result, output, _, _ = self.prepare(Path(temporary), profile, extra)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(output.exists())
