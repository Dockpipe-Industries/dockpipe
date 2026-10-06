"""Exercise staging tap provenance, update safety, and workflow publication gates."""
import base64
import hashlib
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

import yaml


TAP = Path(__file__).resolve().parents[1] / "homebrew/tap"
sys.path.insert(0, str(TAP / "scripts"))
import staging_formula as formula
import sync_staging as sync
sys.path.pop(0)


class HomebrewTests(unittest.TestCase):
    def setUp(self):
        self.candidate = "0.6.0-staging.123.2.aaaaaaaaaaaa"
        self.pointer = {"candidate": self.candidate, "version": "0.6.0", "channel": "staging",
                        "manifest": f"packages/candidates/{self.candidate}/release-manifest.json"}
        self.catalog = {"schema": 1, "version": "0.6.0", "channel": "staging",
                        "candidate": self.candidate, "source_sha": "a" * 40}
        self.run = {"id": 123, "run_attempt": 2, "head_sha": "a" * 40, "head_branch": "staging",
                    "event": "push", "path": ".github/workflows/ci.yml",
                    "repository": {"full_name": sync.SOURCE}, "status": "completed", "conclusion": "success"}
        self.checksums = {}
        for arch in ("arm64", "amd64"):
            self.checksums[f"dockpipe_0.6.0_darwin_{arch}.tar.gz"] = "b" * 64
            self.checksums[f"dockpipe-packages_0.6.0_darwin-{arch}.tar.gz"] = "c" * 64
            self.checksums[f"dockpipe-desktop_0.6.0_darwin_{arch}.zip"] = "d" * 64

    def contents(self):
        return {sync.FORMULA: formula.render(self.candidate, self.checksums),
                sync.CASK: formula.render_cask(self.candidate, self.checksums)}

    def test_cask_requires_verified_native_apps_and_cli_dependency(self):
        rendered = formula.render_cask(self.candidate, self.checksums)
        for arch in ("arm64", "amd64"):
            self.assertIn(f"dockpipe-desktop_0.6.0_darwin_{arch}.zip", rendered)
        self.assertIn('depends_on formula: "dockpipe-industries/dockpipe/dockpipe-staging"', rendered)
        self.assertIn('app "DockPipe.app"', rendered)
        self.checksums.pop("dockpipe-desktop_0.6.0_darwin_arm64.zip")
        with self.assertRaises(ValueError):
            formula.render_cask(self.candidate, self.checksums)

    def test_formula_pins_both_platforms_and_complete_package_resources(self):
        rendered = formula.render(self.candidate, self.checksums)
        self.assertIn(f'version "{self.candidate}"', rendered)
        for filename, checksum in self.checksums.items():
            if filename.startswith("dockpipe-desktop"):
                continue
            self.assertIn(f'{formula.ORIGIN}/packages/candidates/{self.candidate}/{filename}', rendered)
            self.assertIn(f'sha256 "{checksum}"', rendered)
        for kind in ("core", "workflows", "resolvers"):
            self.assertIn(f'packages/{kind}', rendered)
        self.assertNotIn("@CANDIDATE@", rendered)
        self.assertNotIn("@PLATFORMS@", rendered)
        self.assertNotIn('conflicts_with "dockpipe"', rendered)

    def test_rejects_unpinned_inputs_and_ruby_injection(self):
        for candidate in ("0.6.0", '../bad', self.candidate + '"; system("bad")'):
            with self.assertRaises(ValueError):
                formula.render(candidate, self.checksums)
        self.checksums.pop("dockpipe_0.6.0_darwin_arm64.tar.gz")
        with self.assertRaises(ValueError):
            formula.render(self.candidate, self.checksums)
        for text in ("oops", "b" * 64 + "  ../bad", "b" * 64 + "  file\n" + "c" * 64 + "  file"):
            with self.assertRaises(ValueError):
                formula.parse_checksums(text)

    def test_requires_exact_successful_staging_push_provenance(self):
        self.assertTrue(sync.completed_candidate(self.pointer, self.catalog, self.run))
        for key, value in (("id", 999), ("run_attempt", 1), ("head_sha", "b" * 40),
                           ("head_branch", "master"), ("event", "pull_request"),
                           ("repository", {"full_name": "fork/dockpipe"}), ("path", "other.yml")):
            with self.subTest(key=key), self.assertRaises(ValueError):
                sync.completed_candidate(self.pointer, self.catalog, dict(self.run, **{key: value}))
        for status, conclusion in (("in_progress", None), ("completed", "failure"), ("completed", "cancelled")):
            run = dict(self.run, status=status, conclusion=conclusion)
            self.assertFalse(sync.completed_candidate(self.pointer, self.catalog, run))
        for pointer in (dict(self.pointer, channel="stable"), dict(self.pointer, manifest="https://evil.test/x")):
            with self.assertRaises(ValueError):
                sync.completed_candidate(pointer, self.catalog, self.run)
        with self.assertRaises(ValueError):
            sync.completed_candidate(self.pointer, dict(self.catalog, source_sha="b" * 40), self.run)

    def test_downloaded_catalog_must_match_release_checksum(self):
        catalog = json.dumps(self.catalog).encode()
        checksums = dict(self.checksums, **{"release-manifest.json": hashlib.sha256(catalog).hexdigest()})
        checksum_text = "".join(f"{digest}  {name}\n" for name, digest in checksums.items()).encode()
        downloads = [json.dumps(self.pointer).encode(), catalog, checksum_text]
        with patch.object(sync, "download", side_effect=downloads), patch.object(sync, "github", return_value=self.run):
            self.assertEqual(sync.latest_release(), (self.candidate, self.contents()))
        downloads[1] = catalog + b" "
        with patch.object(sync, "download", side_effect=downloads), patch.object(sync, "github", return_value=self.run):
            with self.assertRaisesRegex(ValueError, "checksum mismatch"):
                sync.latest_release()

    def test_forward_updates_and_idempotency(self):
        current = formula.render(self.candidate, self.checksums)
        self.assertFalse(sync.require_forward_update(current, self.candidate, current))
        newer = "0.6.0-staging.124.1.bbbbbbbbbbbb"
        self.assertTrue(sync.require_forward_update(current, newer, formula.render(newer, self.checksums)))
        for candidate in ("0.6.0-staging.122.9.bbbbbbbbbbbb", "0.6.0-staging.123.1.aaaaaaaaaaaa",
                          "0.6.0-staging.123.2.bbbbbbbbbbbb"):
            with self.assertRaises(ValueError):
                sync.require_forward_update(current, candidate, formula.render(candidate, self.checksums))

    def test_publication_rechecks_current_candidate_and_uses_contents_compare_and_swap(self):
        rendered = formula.render(self.candidate, self.checksums)
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            (directory / "candidate.txt").write_text(self.candidate)
            for name, content in self.contents().items():
                (directory / Path(name).name).write_text(content)
            environment = {"GITHUB_REPOSITORY": sync.TAP, "GITHUB_REF": "refs/heads/main"}
            old = formula.render("0.6.0-staging.122.1.bbbbbbbbbbbb", self.checksums)
            existing = {"content": base64.b64encode(old.encode()).decode(), "sha": "old-blob"}
            with patch.dict(os.environ, environment), patch.object(sync, "latest_release", return_value=(self.candidate, self.contents())):
                verified = {"content": base64.b64encode(rendered.encode()).decode()}
                with patch.object(sync, "github", side_effect=[existing, {"content": base64.b64encode(self.contents()[sync.CASK].encode()).decode()}, {"commit": {"sha": "new-commit"}}, verified]) as api:
                    sync.publish(directory)
                    payload = api.call_args_list[2].args[1]
                    self.assertEqual(payload["sha"], "old-blob")
                    self.assertEqual(payload["branch"], "main")
                    self.assertEqual(base64.b64decode(payload["content"]).decode(), rendered)
            with patch.dict(os.environ, environment), patch.object(sync, "latest_release", return_value=None):
                with patch.object(sync, "github") as api:
                    sync.publish(directory)
                    api.assert_not_called()
            with patch.dict(os.environ, environment), patch.object(sync, "latest_release", return_value=(self.candidate, dict(self.contents(), **{sync.FORMULA: rendered + "# modified"}))):
                with self.assertRaises(ValueError):
                    sync.publish(directory)
            with patch.dict(os.environ, {"GITHUB_REPOSITORY": "fork/tap", "GITHUB_REF": "refs/heads/main"}):
                with self.assertRaises(ValueError):
                    sync.publish(directory)

    def test_publication_recovers_an_interrupted_formula_and_cask_pair(self):
        contents = self.contents()
        environment = {"GITHUB_REPOSITORY": sync.TAP, "GITHUB_REF": "refs/heads/main"}
        formula_record = {"content": base64.b64encode(contents[sync.FORMULA].encode()).decode()}
        cask_record = {"content": base64.b64encode(contents[sync.CASK].encode()).decode()}
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            (directory / "candidate.txt").write_text(self.candidate)
            for name, content in contents.items():
                (directory / Path(name).name).write_text(content)
            with patch.dict(os.environ, environment), patch.object(sync, "latest_release", return_value=(self.candidate, contents)):
                responses = [formula_record, None, {"commit": {"sha": "cask-commit"}}, cask_record]
                with patch.object(sync, "github", side_effect=responses) as api:
                    sync.publish(directory)
                    self.assertEqual(api.call_args_list[2].args[0], f"{sync.TAP}/contents/{sync.CASK}")
                    self.assertEqual(base64.b64decode(api.call_args_list[2].args[1]["content"]).decode(), contents[sync.CASK])
                with patch.object(sync, "github", side_effect=[formula_record, cask_record]) as api:
                    sync.publish(directory)
                    self.assertEqual(api.call_count, 2)

    def test_tap_workflow_gates_writes_on_both_native_tests(self):
        workflow = yaml.load((TAP / ".github/workflows/update-staging.yml").read_text(), Loader=yaml.BaseLoader)
        self.assertEqual(workflow["on"]["schedule"], [{"cron": "7,22,37,52 * * * *"}])
        self.assertEqual(workflow["permissions"], {"contents": "read"})
        jobs = workflow["jobs"]
        self.assertEqual(jobs["verify"]["strategy"]["matrix"]["runner"], ["macos-15", "macos-15-intel"])
        self.assertEqual(jobs["publish"]["needs"], ["prepare", "verify"])
        self.assertEqual(jobs["publish"]["permissions"], {"contents": "write"})
        self.assertNotIn("secrets.", yaml.dump(workflow))
        self.assertNotIn("workflow_run", workflow["on"])


if __name__ == "__main__":
    unittest.main()
