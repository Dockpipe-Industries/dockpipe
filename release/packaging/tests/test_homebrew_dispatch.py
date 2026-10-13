"""Verify the privilege boundary for staging-to-tap notifications."""
import copy
import importlib.util
from pathlib import Path
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[3]
SPEC = importlib.util.spec_from_file_location(
    "homebrew_dispatch", ROOT / "release/packaging/homebrew/verify-dispatch.py"
)
DISPATCH = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(DISPATCH)


class HomebrewDispatchTests(unittest.TestCase):
    def setUp(self):
        self.repository = "example/runtime"
        self.run = {
            "repository": {"full_name": self.repository},
            "head_repository": {"full_name": self.repository},
            "path": ".github/workflows/ci.yml",
            "event": "push",
            "head_branch": "staging",
            "status": "completed",
            "conclusion": "success",
        }
        self.jobs = [{
            "name": DISPATCH.PUBLICATION_JOB,
            "status": "completed",
            "conclusion": "success",
        }]

    def test_successful_staging_publication_is_admitted(self):
        self.assertTrue(DISPATCH.published_staging(self.run, self.jobs, self.repository))

    def test_pr_fork_other_branch_and_incomplete_runs_are_rejected(self):
        for field, value in [
            ("event", "pull_request"),
            ("event", "workflow_dispatch"),
            ("head_branch", "master"),
            ("status", "in_progress"),
            ("conclusion", "failure"),
            ("conclusion", "cancelled"),
            ("path", ".github/workflows/release.yml"),
            ("repository", {"full_name": "another/runtime"}),
            ("head_repository", {"full_name": "fork/runtime"}),
        ]:
            with self.subTest(field=field, value=value):
                run = copy.deepcopy(self.run)
                run[field] = value
                self.assertFalse(DISPATCH.published_staging(run, self.jobs, self.repository))

    def test_ci_success_without_successful_publication_is_rejected(self):
        for jobs in [[], self.jobs * 2]:
            self.assertFalse(DISPATCH.published_staging(self.run, jobs, self.repository))
        for conclusion in ["skipped", "failure", "cancelled", None]:
            with self.subTest(conclusion=conclusion):
                jobs = copy.deepcopy(self.jobs)
                jobs[0]["conclusion"] = conclusion
                self.assertFalse(DISPATCH.published_staging(self.run, jobs, self.repository))

    def test_workflow_uses_trusted_code_and_scoped_token(self):
        workflow = yaml.load(
            (ROOT / ".github/workflows/homebrew-staging.yml").read_text(),
            Loader=yaml.BaseLoader,
        )
        self.assertEqual(workflow["on"]["workflow_run"]["types"], ["completed"])
        self.assertEqual(workflow["on"]["workflow_run"]["branches"], ["staging"])
        job = workflow["jobs"]["notify"]
        self.assertIn("vars.HOMEBREW_SYNC_ENABLED == 'true'", job["if"])
        checkout = job["steps"][0]
        self.assertEqual(checkout["with"]["ref"], "${{ github.event.repository.default_branch }}")
        token = next(step for step in job["steps"] if step.get("id") == "tap-token")
        self.assertEqual(token["with"]["permission-actions"], "write")
        self.assertEqual(token["with"]["repositories"], "${{ vars.HOMEBREW_TAP_REPOSITORY }}")
        self.assertEqual(token["if"], "steps.source.outputs.published == 'true'")


if __name__ == "__main__":
    unittest.main()
