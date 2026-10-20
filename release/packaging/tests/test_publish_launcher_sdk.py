import importlib.util
import json
import hashlib
import tempfile
from types import SimpleNamespace
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location(
    'publisher', Path(__file__).resolve().parents[1] / 'publish-launcher-sdk.py')
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
                "source_repository": publisher.REPOSITORY, "source_sha": source_sha,
                "source_state": "committed-snapshot", "source_digest": "d" * 64,
                "compiler": browser["compiler"], "dependencies": {"spatialweb_browser": browser},
                "abi": {"cxx_standard": 17, "workspace_interface": "com.dockpipe.launcher.WorkspacePackage/2.0"},
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
        publisher.verify_tag({'target': commit}, 'a' * 40)
        publisher.verify_tag({'target': {'__typename': 'Tag', 'target': commit}}, 'a' * 40)
        publisher.verify_tag(None, 'a' * 40, allow_missing=True)
        for reference in [None, {'target': {'__typename': 'Commit', 'oid': 'b' * 40}},
                          {'target': {'__typename': 'Tree', 'oid': 'a' * 40}}]:
            with self.assertRaises(ValueError):
                publisher.verify_tag(reference, 'a' * 40)

    def test_production_and_manual_calls_fail_before_network(self):
        for ref, event in [('refs/heads/master', 'push'), ('refs/heads/dev', 'workflow_dispatch')]:
            environment = {
                'GITHUB_REPOSITORY': publisher.REPOSITORY,
                'GITHUB_REF': ref,
                'GITHUB_EVENT_NAME': event,
                'GITHUB_SHA': 'a' * 40,
            }
            with patch.dict(publisher.os.environ, environment, clear=True), \
                    patch.object(publisher, 'run') as run:
                with self.assertRaises(ValueError):
                    publisher.publish(Path('/not-read'))
                run.assert_not_called()

    def test_remote_mutation_is_rejected(self):
        assets = {'a': {'size': 2, 'digest': 'sha256:aaa'}}
        release = {
            'tag_name': 'candidate',
            'target_commitish': 'a' * 40,
            'prerelease': True,
            'assets': [{'name': 'a', 'size': 2, 'digest': 'sha256:aaa'}],
        }
        publisher.verify_release(release, 'candidate', 'a' * 40, assets)
        release['assets'][0]['digest'] = 'sha256:bbb'
        with self.assertRaises(ValueError):
            publisher.verify_release(release, 'candidate', 'a' * 40, assets)

    def test_draft_lookup_uses_inventory_and_numeric_id_not_tag_endpoint(self):
        draft = {"id": 418113526, "tag_name": "candidate", "draft": True}
        with patch.object(publisher, "run", side_effect=[json.dumps([draft]), json.dumps(draft)]) as command:
            self.assertEqual(publisher.find_release("candidate"), draft)
        endpoints = [call.args[0][-1] for call in command.call_args_list]
        self.assertEqual(endpoints, [f"repos/{publisher.REPOSITORY}/releases?per_page=100&page=1",
                                     f"repos/{publisher.REPOSITORY}/releases/418113526"])

    def test_draft_lookup_is_bounded_and_does_not_treat_api_failure_as_absence(self):
        full_page = [{"id": index + 1, "tag_name": f"other-{index}"} for index in range(100)]
        with patch.object(publisher, "run", return_value=json.dumps(full_page)) as command:
            with self.assertRaisesRegex(ValueError, "exceeds capacity"):
                publisher.find_release("candidate")
            self.assertEqual(command.call_count, 5)
        with patch.object(publisher, "run", side_effect=ValueError("HTTP 403")):
            with self.assertRaisesRegex(ValueError, "HTTP 403"):
                publisher.find_release("candidate")
        with patch.object(publisher, "run", return_value="[]"):
            self.assertIsNone(publisher.find_release("candidate"))

    def test_draft_lookup_rejects_ambiguous_or_changed_identity(self):
        draft = {"id": 123, "tag_name": "candidate", "draft": True}
        for responses in ([json.dumps([draft, draft])],
                          [json.dumps([draft]), json.dumps({**draft, "id": 456})],
                          [json.dumps([draft]), json.dumps({**draft, "tag_name": "other"})]):
            with self.subTest(responses=responses), patch.object(publisher, "run", side_effect=responses):
                with self.assertRaises(ValueError):
                    publisher.find_release("candidate")

    def test_failed_command_reports_status_without_stderr_secrets(self):
        result = SimpleNamespace(returncode=1, stdout="", stderr="private token SECRET; Not Found (HTTP 404)")
        with patch.object(publisher.subprocess, "run", return_value=result):
            with self.assertRaises(ValueError) as raised:
                publisher.run(["gh", "api", "endpoint"])
        self.assertIn("exit 1; HTTP 404", str(raised.exception))
        self.assertNotIn("SECRET", str(raised.exception))

    def test_fresh_and_resumed_drafts_use_id_reads_and_never_duplicate_release(self):
        source = "a" * 40
        environment = {"GITHUB_REPOSITORY": publisher.REPOSITORY, "GITHUB_REF": "refs/heads/dev", "GITHUB_EVENT_NAME": "push", "GITHUB_SHA": source}
        tag = "launcher-sdk-1.1.0-staging." + source
        assets = {"fixture": {"size": 2, "digest": "sha256:aa"}}
        for previously_created in (False, True):
            with self.subTest(previously_created=previously_created):
                exists = previously_created
                release = {"id": 418113526, "tag_name": tag, "target_commitish": source,
                           "prerelease": True, "draft": True, "html_url": "https://github.com/example/candidate",
                           "assets": [{"name": "fixture", **assets["fixture"]}]}
                calls = []

                def execute(arguments):
                    nonlocal exists
                    calls.append(arguments)
                    if arguments[1:3] == ["release", "create"]:
                        self.assertFalse(exists)
                        self.assertIn("--draft", arguments)
                        exists = True
                        return ""
                    if arguments[1:3] == ["release", "edit"]:
                        self.assertTrue(exists)
                        self.assertIn("--draft=false", arguments)
                        release["draft"] = False
                        return ""
                    if "graphql" in arguments:
                        target = None if release["draft"] else {"target": {"__typename": "Commit", "oid": source}}
                        # The tag lookup fails to expose this complete draft, as
                        # in the observed hosted failure. Inventory still finds it.
                        state = {"isPrivate": True, "release": None, "ref": target}
                        return json.dumps({"data": {"repository": state}})
                    endpoint = arguments[-1]
                    if endpoint == f"repos/{publisher.REPOSITORY}/releases?per_page=100&page=1":
                        return json.dumps([release] if exists else [])
                    if endpoint == f"repos/{publisher.REPOSITORY}/releases/418113526":
                        self.assertTrue(exists)
                        return json.dumps(release)
                    self.fail(f"Unexpected candidate operation: {arguments[:3]}")

                with patch.dict(publisher.os.environ, environment, clear=True), \
                     patch.object(publisher, "validate", return_value=assets), \
                     patch.object(publisher, "run", side_effect=execute), patch("builtins.print"):
                    publisher.publish(Path("fixture"))
                creations = [call for call in calls if call[1:3] == ["release", "create"]]
                self.assertEqual(len(creations), 0 if previously_created else 1)
                self.assertEqual(sum(call[1:3] == ["release", "edit"] for call in calls), 1)
                self.assertFalse(any("/releases/tags/" in argument for call in calls for argument in call))
                self.assertFalse(release["draft"])


if __name__ == "__main__":
    unittest.main()
