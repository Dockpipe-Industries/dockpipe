#!/usr/bin/env python3
"""Exercise release admission and deployment isolation from the authored workflow."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import yaml


REPOSITORY = Path(__file__).resolve().parents[3]
WORKFLOW = REPOSITORY / ".github/workflows/release.yml"


class ReleaseWorkflowTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workflow = yaml.load(WORKFLOW.read_text(), Loader=yaml.BaseLoader)
        cls.jobs = cls.workflow["jobs"]

    def condition(self, expression, ref, dry_run):
        """Evaluate the simple job guards with supplied GitHub context values."""
        expression = expression.removeprefix("${{").removesuffix("}}")
        context = {
            "github.ref": ref,
            "needs.meta.outputs.dry_run": dry_run,
            "vars.DEVTO_PUBLISH": "true",
            "vars.DEVTO_ARTICLE_ID": "test-article",
            "vars.DEVTO_ONE_TIME_POST": "false",
        }
        for name, value in context.items():
            expression = expression.replace(name, repr(value))
        expression = expression.replace("&&", " and ").replace("||", " or ")
        return eval(expression.strip(), {"__builtins__": {}}, {})

    def test_metadata_admits_dry_runs_and_rejects_non_master_publication(self):
        metadata = next(step for step in self.jobs["meta"]["steps"] if step.get("id") == "m")
        refs = [
            "refs/heads/js/pipelang", "refs/heads/js/dev", "refs/heads/dev",
            "refs/heads/staging", "refs/heads/master", "refs/tags/master",
        ]
        with tempfile.TemporaryDirectory(prefix="dockpipe-workflow-test-") as temporary:
            output = Path(temporary) / "outputs"
            for ref in refs:
                for event, requested_dry_run in [
                    ("workflow_dispatch", "true"), ("workflow_dispatch", "false"),
                    ("workflow_dispatch", ""), ("push", ""),
                ]:
                    with self.subTest(ref=ref, event=event, dry_run=requested_dry_run):
                        output.write_text("")
                        dry_run = event == "workflow_dispatch" and requested_dry_run != "false"
                        allowed = dry_run or ref == "refs/heads/master"
                        environment = dict(os.environ, GITHUB_REF=ref, GITHUB_EVENT_NAME=event,
                                           GITHUB_OUTPUT=str(output), INPUT_VERSION="",
                                           INPUT_DRY_RUN=requested_dry_run, INPUT_BUILD_MSI="true")
                        result = subprocess.run(["bash", "-c", metadata["run"]], cwd=REPOSITORY,
                                                env=environment, text=True, capture_output=True)
                        self.assertEqual(result.returncode == 0, allowed, result.stdout + result.stderr)
                        if allowed:
                            self.assertIn(f"dry_run={str(dry_run).lower()}\n", output.read_text())
                        else:
                            self.assertIn("Production releases require master", result.stdout)
                            self.assertEqual(output.read_text(), "")

    def test_deployment_jobs_only_admit_explicit_production_on_master(self):
        for name in ("publish", "devto"):
            job = self.jobs[name]
            self.assertEqual(job["environment"], "release")
            for ref in ("refs/heads/js/pipelang", "refs/heads/js/dev", "refs/heads/dev",
                        "refs/heads/staging", "refs/heads/master", "refs/tags/master"):
                for dry_run in ("true", "false", ""):
                    with self.subTest(job=name, ref=ref, dry_run=dry_run):
                        self.assertEqual(self.condition(job["if"], ref, dry_run),
                                         ref == "refs/heads/master" and dry_run == "false")

    def test_artifact_verification_has_no_deployment_or_production_credentials(self):
        for name, job in self.jobs.items():
            if name in ("publish", "devto"):
                continue
            self.assertNotIn("environment", job, name)
            self.assertNotIn("secrets.", yaml.dump(job), name)
            self.assertNotIn("write", job.get("permissions", {}).values(), name)

        assembly = self.jobs["assemble"]
        self.assertEqual(assembly["needs"], ["meta", "build-unix", "build-msi"])
        self.assertEqual(self.jobs["publish"]["needs"], ["meta", "assemble"])
        self.assertEqual(self.jobs["devto"]["needs"], ["meta", "publish"])
        self.assertIn("needs.build-unix.result == 'success'", assembly["if"])
        self.assertIn("needs.build-msi.result == 'success'", assembly["if"])
        self.assertIn("needs.build-msi.result == 'skipped'", assembly["if"])

        dry_run_upload = next(step for step in assembly["steps"]
                              if step.get("name") == "Upload dry-run artifacts")
        self.assertTrue(self.condition(dry_run_upload["if"], "refs/heads/js/pipelang", "true"))
        self.assertFalse(self.condition(dry_run_upload["if"], "refs/heads/master", "false"))
        self.assertIn("release/apt/", dry_run_upload["with"]["path"])
        prepared = next(step for step in assembly["steps"]
                        if step.get("name") == "Upload prepared production artifacts")
        download = next(step for step in self.jobs["publish"]["steps"]
                        if step.get("uses", "").startswith("actions/download-artifact@"))
        self.assertEqual(prepared["with"]["name"], download["with"]["name"])
        self.assertEqual(prepared["with"]["path"].rstrip("/"), download["with"]["path"])

    def test_manual_runs_default_to_dry_run_and_only_master_pushes_trigger(self):
        triggers = self.workflow["on"]
        self.assertEqual(triggers["push"]["branches"], ["master"])
        self.assertEqual(triggers["workflow_dispatch"]["inputs"]["dry_run"]["default"], "true")


if __name__ == "__main__":
    unittest.main()
