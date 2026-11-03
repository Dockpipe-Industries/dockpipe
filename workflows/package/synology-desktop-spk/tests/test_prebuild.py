"""Check prebuilt identity and release publication boundaries without a registry."""

import hashlib
import importlib.util
import json
from pathlib import Path
import unittest
import yaml


ROOT = Path(__file__).resolve().parents[4]
SCRIPT = ROOT / "workflows/package/synology-desktop-spk/assets/scripts/prebuild.py"
SPECIFICATION = importlib.util.spec_from_file_location("synology_prebuild", SCRIPT)
PREBUILD = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(PREBUILD)


class PrebuiltImageTests(unittest.TestCase):
    def test_embedded_container_probe_is_valid_python(self):
        compile(PREBUILD.QUALIFIER.PROBE, "desktop-startup-probe", "exec")

    def test_manifest_digest_is_over_exact_registry_manifest_bytes(self):
        config = "sha256:" + "a" * 64
        raw = json.dumps({"schemaVersion": 2, "config": {"digest": config}}).encode()
        digest, image_id = PREBUILD.manifest_identity(raw)
        self.assertEqual(digest, "sha256:" + hashlib.sha256(raw).hexdigest())
        self.assertEqual(image_id, config)

    def test_manifest_rejects_indexes_invalid_config_and_oversized_metadata(self):
        for raw in (b'[]', b'{"manifests":[]}', b'{"config":{"digest":"latest"}}', b' ' * (4 * 1024**2 + 1)):
            with self.subTest(raw=raw[:60]), self.assertRaises(ValueError):
                PREBUILD.manifest_identity(raw)

    def test_registry_writes_belong_to_publication_jobs_and_dry_runs_keep_artifacts(self):
        workflow = yaml.safe_load((ROOT / ".github/workflows/release.yml").read_text())
        jobs = workflow["jobs"]
        for name, job in jobs.items():
            if job.get("permissions", {}).get("packages") == "write":
                self.assertIn(name, {"publish", "publish-staging"})
        build = jobs["build-unix"]
        upload = next(step for step in build["steps"] if step.get("name") == "Retain verified Synology images for publication")
        self.assertTrue(upload["with"]["name"].startswith("synology-image-"))
        for name in ("publish", "publish-staging"):
            steps = jobs[name]["steps"]
            names = [step.get("name") for step in steps]
            self.assertLess(names.index("Publish exact desktop images and verify public access"), names.index("Create GitHub release"))
            self.assertIn("needs.meta.outputs.dry_run == 'false'", jobs[name]["if"])
        self.assertNotIn("packages", build["permissions"])


if __name__ == "__main__":
    unittest.main()
