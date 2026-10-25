import importlib.util
import json
import hashlib
import tempfile
from types import SimpleNamespace
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location(
    'publisher', Path(__file__).resolve().parents[1] / 'assets/scripts/publish.py')
publisher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(publisher)


class CandidateBoundaryTests(unittest.TestCase):
    def test_candidate_validation_binds_browser_pin_compiler_interface_and_cleanup(self):
        source_sha = "a" * 40
        browser = {"product": "SpatialWebBrowserSDK", "version": "0.1.0",
                   "source_sha": "b" * 40, "compiler": {"id": "GNU", "version": "11.4.0"},
                   "artifact": {"name": "Browser.tar.gz", "sha256": "c" * 64, "size_bytes": 4}}
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            archive_bytes = b"qualified archive fixture"
            inventory_bytes = b'{"schema":1,"artifact_class":"sdk","files":[]}'
            (directory / publisher.ARCHIVE).write_bytes(archive_bytes)
            (directory / "sdk-files.json").write_bytes(inventory_bytes)
            metadata = {
                "schema": 1, "product": "DockPipeLauncherSDK", "version": "1.1.0",
                "platform": "linux-amd64", "qt_version": "6.10.3",
                "source_repository": publisher.SOURCE_REPOSITORY, "source_sha": source_sha,
                "source_state": "committed-snapshot", "source_digest": "d" * 64,
                "compiler": browser["compiler"], "dependencies": {"spatialweb_browser": browser},
                "abi": {"cxx_standard": 17, "workspace_interface": "com.dockpipe.launcher.WorkspacePackage/2.0",
                        "display_interface": "com.dockpipe.launcher.DisplayPackage/1.0"},
                "archive": {"name": publisher.ARCHIVE, "size_bytes": len(archive_bytes),
                            "sha256": hashlib.sha256(archive_bytes).hexdigest()},
                "file_manifest": {"name": "sdk-files.json", "sha256": hashlib.sha256(inventory_bytes).hexdigest()},
                "checks": ["sdk-component-install", "shared-runtime-symbol-cleanup", "relocated-external-consumer-compile-link"],
            }
            path = directory / "sdk-release.json"
            path.write_text(json.dumps(metadata))
            with patch.object(publisher, "browser_dependency", return_value=browser):
                self.assertEqual(set(publisher.ASSETS), set(publisher.validate(directory, source_sha)))
                for key, value in (
                    ("source_repository", "ExampleOperator/release-candidates"),
                    ("dependencies", {}),
                    ("dependencies", {"spatialweb_browser": {**browser, "source_sha": "e" * 40}}),
                    ("compiler", {"id": "GNU", "version": "13.3.0"}),
                    ("abi", {"cxx_standard": 17, "workspace_interface": "com.dockpipe.launcher.WorkspacePackage/1.0"}),
                    ("source_digest", "development"),
                    ("checks", ["sdk-component-install", "relocated-external-consumer-compile-link"]),
                ):
                    with self.subTest(key=key, value=value):
                        path.write_text(json.dumps({**metadata, key: value}))
                        with self.assertRaises(ValueError):
                            publisher.validate(directory, source_sha)
                path.write_text(json.dumps(metadata))
                (directory / publisher.ARCHIVE).write_bytes(b"changed archive fixture")
                with self.assertRaisesRegex(ValueError, "archive differs"):
                    publisher.validate(directory, source_sha)

    def test_existing_tag_must_resolve_to_the_qualified_commit(self):
        commit = {'__typename': 'Commit', 'oid': 'a' * 40}
        publisher.SHARED.verify_tag({'target': commit}, 'a' * 40)
        publisher.SHARED.verify_tag({'target': {'__typename': 'Tag', 'target': commit}}, 'a' * 40)
        publisher.SHARED.verify_tag(None, 'a' * 40, allow_missing=True)
        for reference in [None, {'target': {'__typename': 'Commit', 'oid': 'b' * 40}},
                          {'target': {'__typename': 'Tree', 'oid': 'a' * 40}}]:
            with self.assertRaises(ValueError):
                publisher.SHARED.verify_tag(reference, 'a' * 40)

    def test_production_and_manual_calls_fail_before_network(self):
        for ref, event in [('refs/heads/master', 'push'), ('refs/heads/dev', 'workflow_dispatch')]:
            environment = {
                'GITHUB_REPOSITORY': publisher.SOURCE_REPOSITORY,
                'GITHUB_REF': ref,
                'GITHUB_EVENT_NAME': event,
                'GITHUB_SHA': 'a' * 40,
            }
            with patch.dict(publisher.os.environ, environment, clear=True), \
                    patch.object(publisher.SHARED, 'run') as run:
                with self.assertRaises(ValueError):
                    publisher.publish(Path('/not-read'))
                run.assert_not_called()

    def test_remote_mutation_is_rejected(self):
        assets = {'a': {'size': 2, 'digest': 'sha256:aaa'}}
        release = {
            'id': 1, 'draft': False, 'tag_name': 'candidate',
            'target_commitish': 'a' * 40,
            'prerelease': True,
            'assets': [{'name': 'a', 'size': 2, 'digest': 'sha256:aaa', 'state': 'uploaded'}],
        }
        publisher.SHARED.verify_release(release, 'candidate', 'a' * 40, assets)
        release['assets'][0]['digest'] = 'sha256:bbb'
        with self.assertRaises(ValueError):
            publisher.SHARED.verify_release(release, 'candidate', 'a' * 40, assets)

    def test_draft_lookup_projects_only_selected_metadata_and_reads_exact_id(self):
        draft = {"id": 418113526, "tag_name": "candidate", "draft": True}
        selected = {"count": 100, "matches": [{"id": draft["id"], "tag_name": "candidate"}]}
        with patch.object(publisher.SHARED, "run", side_effect=[json.dumps(selected), json.dumps(draft)]) as command:
            self.assertEqual(publisher.SHARED.find_release(publisher.SOURCE_REPOSITORY, "candidate"), draft)
        operations = [call.args[0] for call in command.call_args_list]
        self.assertTrue(all("--jq" in arguments for arguments in operations))
        self.assertIn(f"repos/{publisher.SOURCE_REPOSITORY}/releases/418113526", operations[1])

    def test_draft_lookup_is_bounded_and_does_not_treat_api_failure_as_absence(self):
        with patch.object(publisher.SHARED, "run", return_value='{"count":100,"matches":[]}') as command:
            with self.assertRaisesRegex(ValueError, "exceeds capacity"):
                publisher.SHARED.find_release(publisher.SOURCE_REPOSITORY, "candidate")
            self.assertEqual(command.call_count, 5)
        with patch.object(publisher.SHARED, "run", side_effect=ValueError("HTTP 403")):
            with self.assertRaisesRegex(ValueError, "HTTP 403"):
                publisher.SHARED.find_release(publisher.SOURCE_REPOSITORY, "candidate")
        with patch.object(publisher.SHARED, "run", return_value='{"count":0,"matches":[]}'):
            self.assertIsNone(publisher.SHARED.find_release(publisher.SOURCE_REPOSITORY, "candidate"))

    def test_draft_lookup_rejects_ambiguous_or_changed_identity(self):
        draft = {"id": 123, "tag_name": "candidate", "draft": True}
        selected = {"count": 2, "matches": [{"id": 123, "tag_name": "candidate"}]}
        for responses in (
                [json.dumps({"count": 2, "matches": [draft, draft]})],
                [json.dumps(selected), json.dumps({**draft, "id": 456})],
                [json.dumps(selected), json.dumps({**draft, "tag_name": "other"})]):
            with self.subTest(responses=responses), patch.object(publisher.SHARED, "run", side_effect=responses):
                with self.assertRaises(ValueError):
                    publisher.SHARED.find_release(publisher.SOURCE_REPOSITORY, "candidate")

    def test_failed_command_reports_status_without_stderr_secrets(self):
        result = SimpleNamespace(returncode=1, stdout="", stderr="private token SECRET; Not Found (HTTP 404)")
        with patch.object(publisher.SHARED.subprocess, "run", return_value=result):
            with self.assertRaises(ValueError) as raised:
                publisher.SHARED.run(["gh", "api", "endpoint"])
        self.assertIn("exit 1; HTTP 404", str(raised.exception))
        self.assertNotIn("SECRET", str(raised.exception))



if __name__ == "__main__":
    unittest.main()
