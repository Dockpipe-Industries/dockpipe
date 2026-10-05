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
            if name in ("publish", "publish-staging", "devto"):
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

    def test_staging_admission_requires_own_staging_push(self):
        metadata = next(step for step in self.jobs["meta"]["steps"] if step.get("id") == "m")
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / "outputs"
            for event in ("push", "pull_request", "workflow_dispatch"):
                for ref in ("refs/heads/staging", "refs/heads/master", "refs/heads/dev"):
                    for repository in ("Dockpipe-Industries/dockpipe", "fork/dockpipe"):
                        output.write_text("")
                        environment = dict(os.environ, GITHUB_REF=ref, GITHUB_EVENT_NAME=event,
                                           GITHUB_REPOSITORY=repository, GITHUB_SHA="a" * 40,
                                           GITHUB_RUN_ID="1234", GITHUB_RUN_ATTEMPT="2",
                                           GITHUB_OUTPUT=str(output), INPUT_STAGING="true")
                        result = subprocess.run(["bash", "-c", metadata["run"]], cwd=REPOSITORY,
                                                env=environment, text=True, capture_output=True)
                        allowed = event == "push" and ref == "refs/heads/staging" and repository == "Dockpipe-Industries/dockpipe"
                        self.assertEqual(result.returncode == 0, allowed, result.stdout + result.stderr)
                        if allowed:
                            self.assertIn("-staging.1234.2.aaaaaaaaaaaa", output.read_text())
                            self.assertIn("dry_run=false", output.read_text())
                        else:
                            self.assertEqual(output.read_text(), "")

    def test_staging_call_waits_for_tests_and_publishes_only_prereleases(self):
        ci = yaml.load((REPOSITORY / ".github/workflows/ci.yml").read_text(), Loader=yaml.BaseLoader)
        call = ci["jobs"]["staging-release"]
        self.assertEqual(call["needs"], ["test", "test-windows"])
        self.assertEqual(call["uses"], "./.github/workflows/release.yml")
        self.assertEqual(call["with"], {"staging": "true"})
        self.assertIn("github.event_name == 'push'", call["if"])
        self.assertIn("github.ref == 'refs/heads/staging'", call["if"])
        self.assertNotIn("secrets", call)
        job = self.jobs["publish-staging"]
        self.assertEqual(job["needs"], ["meta", "assemble"])
        self.assertEqual(job["environment"], "release-staging")
        self.assertIn("needs.meta.outputs.candidate != ''", job["if"])
        release = next(step for step in job["steps"] if step.get("uses", "").startswith("softprops/"))
        self.assertEqual(release["with"]["prerelease"], "true")
        self.assertEqual(release["with"]["make_latest"], "false")
        self.assertEqual(release["with"]["target_commitish"], "${{ github.sha }}")
        self.assertIn("Require the current staging head", [step.get("name") for step in job["steps"]])

    def test_manual_runs_default_to_dry_run_and_only_master_pushes_trigger(self):
        triggers = self.workflow["on"]
        self.assertEqual(triggers["push"]["branches"], ["master"])
        self.assertEqual(triggers["workflow_dispatch"]["inputs"]["dry_run"]["default"], "true")

    def run_release_notes_gate(self, changed_paths):
        ci = yaml.load((REPOSITORY / ".github/workflows/ci.yml").read_text(), Loader=yaml.BaseLoader)
        gate = next(step for step in ci["jobs"]["test"]["steps"]
                    if step.get("name") == "Release notes + version bump (PRs targeting master only)")
        script = gate["run"].replace("${{ github.event.pull_request.base.sha }}", "base")
        script = script.replace("${{ github.event.pull_request.head.sha }}", "head")
        with tempfile.TemporaryDirectory(prefix="dockpipe-release-notes-gate-") as temporary:
            root = Path(temporary)
            (root / "VERSION").write_text("0.6.0\n")
            notes = root / "release/releasenotes/0.6.0.md"
            notes.parent.mkdir(parents=True)
            notes.write_text("Release notes\n")
            changed = root / "changed-paths.txt"
            changed.write_text("\n".join(changed_paths) + "\n")
            binaries = root / "bin"
            binaries.mkdir()
            git = binaries / "git"
            git.write_text(
                '#!/usr/bin/env bash\n'
                'case "$*" in\n'
                '  "show base:VERSION") printf "0.5.3\\n" ;;\n'
                '  "diff --name-only base head") cat "$TEST_CHANGED_PATHS" ;;\n'
                '  *) exit 2 ;;\n'
                'esac\n'
            )
            git.chmod(0o755)
            environment = dict(os.environ, PATH=str(binaries) + os.pathsep + os.environ["PATH"],
                               TEST_CHANGED_PATHS=str(changed))
            return subprocess.run(["bash", "-c", script], cwd=root, env=environment,
                                  capture_output=True, text=True)

    def test_release_notes_gate_accepts_a_large_change_set(self):
        paths = ["release/releasenotes/0.6.0.md"]
        paths.extend(f"packages/example/workflows/workflow-{number}/assets/script.sh" for number in range(10000))
        result = self.run_release_notes_gate(paths)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("OK: shipping 0.6.0", result.stdout)

    def test_release_notes_gate_requires_the_exact_notes_path(self):
        for paths in (["VERSION"], ["release/releasenotes/0x6x0xmd"]):
            with self.subTest(paths=paths):
                result = self.run_release_notes_gate(paths)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("This PR must modify release/releasenotes/0.6.0.md", result.stdout)


if __name__ == "__main__":
    unittest.main()
