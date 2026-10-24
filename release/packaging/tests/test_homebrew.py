"""Exercise staging tap provenance, update safety, and workflow publication gates."""
import base64
import hashlib
import json
import os
from pathlib import Path
import sys
import subprocess
import tempfile
import unittest
import urllib.error
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
        self.checksums = {"dockpipe-core-0.6.0.tar.gz": "e" * 64}
        for arch in ("arm64", "amd64"):
            self.checksums[f"dockpipe_0.6.0_darwin_{arch}.tar.gz"] = "b" * 64
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

    def test_formula_pins_both_platforms_and_only_core(self):
        rendered = formula.render(self.candidate, self.checksums)
        self.assertIn(f'version "{self.candidate}"', rendered)
        for filename, checksum in self.checksums.items():
            if filename.startswith("dockpipe-desktop"):
                continue
            self.assertIn(f'{formula.ORIGIN}/packages/candidates/{self.candidate}/{filename}', rendered)
            self.assertIn(f'sha256 "{checksum}"', rendered)
        self.assertIn('packages/core', rendered)
        self.assertNotIn('packages/workflows', rendered)
        self.assertNotIn('packages/resolvers', rendered)
        self.assertNotIn('dockpipe-packages_', rendered)
        self.assertNotIn('@RELEASE_VERSION@', rendered)
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

    def test_runtime_pointer_falls_back_only_when_missing(self):
        catalog = json.dumps(self.catalog).encode()
        checksums = dict(self.checksums, **{"release-manifest.json": hashlib.sha256(catalog).hexdigest()})
        checksum_text = "".join(f"{digest}  {name}\n" for name, digest in checksums.items()).encode()
        missing = urllib.error.HTTPError("https://example.invalid/runtime/latest.json", 404, "missing", {}, None)
        responses = [missing, json.dumps(self.pointer).encode(), catalog, checksum_text]
        with patch.object(sync, "download", side_effect=responses) as download, \
                patch.object(sync, "github", return_value=self.run):
            self.assertEqual(sync.latest_release(), (self.candidate, self.contents()))
            self.assertEqual(download.call_args_list[0].args, ("packages/runtime/latest.json",))
            self.assertEqual(download.call_args_list[1].args, ("packages/latest.json",))
        for response in (b"invalid JSON", urllib.error.HTTPError(
                "https://example.invalid/runtime/latest.json", 503, "unavailable", {}, None)):
            with self.subTest(response=type(response).__name__):
                with patch.object(sync, "download", side_effect=[response]) as download:
                    with self.assertRaises((ValueError, urllib.error.HTTPError)):
                        sync.latest_release()
                    self.assertEqual(download.call_count, 1)

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


class RuntimePointerTransportTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.fixture = self.directory / "responses.json"
        self.calls = self.directory / "calls.jsonl"
        curl = self.directory / "curl"
        curl.write_text('''#!/usr/bin/env python3
import json
import os
from pathlib import Path
import sys

arguments = sys.argv[1:]
url = arguments[-1]
path = url.split(".com/", 1)[1]
with Path(os.environ["CURL_TEST_CALLS"]).open("a") as stream:
    stream.write(json.dumps(path) + "\\n")
response = json.loads(Path(os.environ["CURL_TEST_RESPONSES"]).read_text())[path]
if "transport_exit" in response:
    sys.stdout.write(response.get("status", "000"))
    sys.exit(response["transport_exit"])
status = response.get("status", 200)
Path(arguments[arguments.index("--output") + 1]).write_bytes(response.get("body", "").encode())
sys.stdout.write(str(status))
sys.exit(22 if status >= 400 else 0)
''')
        curl.chmod(0o755)
        environment = {
            "PATH": str(self.directory) + os.pathsep + os.environ["PATH"],
            "CURL_TEST_RESPONSES": str(self.fixture),
            "CURL_TEST_CALLS": str(self.calls),
        }
        self.environment = patch.dict(os.environ, environment)
        self.environment.start()
        self.addCleanup(self.environment.stop)
        self.candidate = "0.6.0-staging.123.2.aaaaaaaaaaaa"
        self.base = f"packages/candidates/{self.candidate}"
        pointer = {
            "candidate": self.candidate,
            "version": "0.6.0",
            "channel": "staging",
            "manifest": f"{self.base}/release-manifest.json",
        }
        catalog = dict(pointer, schema=1, source_sha="a" * 40)
        catalog_bytes = json.dumps(catalog)
        checksums = {
            "release-manifest.json": hashlib.sha256(catalog_bytes.encode()).hexdigest(),
            "dockpipe-core-0.6.0.tar.gz": "b" * 64,
        }
        for architecture in ("amd64", "arm64"):
            checksums[f"dockpipe_0.6.0_darwin_{architecture}.tar.gz"] = "c" * 64
            checksums[f"dockpipe-desktop_0.6.0_darwin_{architecture}.zip"] = "d" * 64
        checksum_body = "".join(f"{digest}  {name}\n" for name, digest in checksums.items())
        self.responses = {
            "packages/runtime/latest.json": {"body": json.dumps(pointer)},
            "packages/latest.json": {"body": "Marketplace metadata is independent"},
            f"{self.base}/release-manifest.json": {"body": catalog_bytes},
            f"{self.base}/SHA256SUMS.txt": {"body": checksum_body},
        }
        self.run = {
            "id": 123,
            "run_attempt": 2,
            "head_sha": "a" * 40,
            "head_branch": "staging",
            "event": "push",
            "path": ".github/workflows/ci.yml",
            "repository": {"full_name": sync.SOURCE},
            "status": "completed",
            "conclusion": "success",
        }

    def release(self):
        self.fixture.write_text(json.dumps(self.responses))
        with patch.object(sync, "github", return_value=self.run):
            return sync.latest_release()

    def requested_paths(self):
        return [json.loads(line) for line in self.calls.read_text().splitlines()]

    def test_runtime_pointer_does_not_read_marketplace_pointer(self):
        candidate, contents = self.release()
        self.assertEqual(candidate, self.candidate)
        self.assertEqual(set(contents), {sync.FORMULA, sync.CASK})
        self.assertNotIn("packages/latest.json", self.requested_paths())

    def test_real_curl_http_404_admits_legacy_fallback(self):
        self.responses["packages/latest.json"] = self.responses["packages/runtime/latest.json"]
        self.responses["packages/runtime/latest.json"] = {"status": 404}
        self.assertEqual(self.release()[0], self.candidate)
        self.assertEqual(self.requested_paths()[:2], [
            "packages/runtime/latest.json", "packages/latest.json",
        ])

    def test_http_errors_do_not_admit_fallback(self):
        for status in (403, 429, 500, 503):
            with self.subTest(status=status):
                self.calls.unlink(missing_ok=True)
                self.responses["packages/runtime/latest.json"] = {"status": status}
                with self.assertRaises(subprocess.CalledProcessError) as failure:
                    self.release()
                self.assertEqual(failure.exception.returncode, 22)
                self.assertEqual(self.requested_paths(), ["packages/runtime/latest.json"])

    def test_transport_failure_does_not_admit_fallback(self):
        self.responses["packages/runtime/latest.json"] = {"transport_exit": 28, "status": "404"}
        with self.assertRaises(subprocess.CalledProcessError) as failure:
            self.release()
        self.assertEqual(failure.exception.returncode, 28)
        self.assertEqual(self.requested_paths(), ["packages/runtime/latest.json"])

    def test_invalid_runtime_json_does_not_admit_fallback(self):
        self.responses["packages/runtime/latest.json"] = {"body": "invalid JSON"}
        with self.assertRaises(ValueError):
            self.release()
        self.assertEqual(self.requested_paths(), ["packages/runtime/latest.json"])

    def test_invalid_runtime_identity_does_not_admit_fallback(self):
        self.responses["packages/runtime/latest.json"] = {"body": '{"candidate":"invalid"}'}
        with self.assertRaises(ValueError):
            self.release()
        self.assertEqual(self.requested_paths(), ["packages/runtime/latest.json"])

    def test_download_preserves_response_body_bytes(self):
        body = "payload\n200\nUnicode: λ\n"
        self.responses["body"] = {"body": body}
        self.fixture.write_text(json.dumps(self.responses))
        self.assertEqual(sync.download("body"), body.encode())

    def test_failed_source_run_cannot_prepare_candidate(self):
        self.run["conclusion"] = "failure"
        self.assertIsNone(self.release())
        self.assertEqual(self.requested_paths(), [
            "packages/runtime/latest.json", f"{self.base}/release-manifest.json",
        ])

    def test_release_checksum_mismatch_fails_closed(self):
        self.responses[f"{self.base}/release-manifest.json"]["body"] += " "
        with self.assertRaisesRegex(ValueError, "checksum mismatch"):
            self.release()


if __name__ == "__main__":
    unittest.main()
