"""Missing-policy qualification rejects transport failures and container-side effects."""
import copy
import contextlib
import io
import json
from pathlib import Path
import socket
import sys
import tempfile
import unittest
from unittest import mock


OWNER = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(OWNER / "drivers"))
import policy_failure


class PolicyFailureTests(unittest.TestCase):
    def test_normal_actionable_refusal_is_distinct_from_wrong_failure(self):
        with tempfile.TemporaryDirectory() as temporary:
            stderr = Path(temporary) / "stderr"
            process = {"exit_code": 1, "failure": ""}
            stderr.write_text("bounded runtime log\n" + policy_failure.WORKFLOW_DIAGNOSTIC + "\n")
            self.assertTrue(policy_failure.expected_refusal(process, stderr, policy_failure.WORKFLOW_DIAGNOSTIC))
            for message in ["wrapper denied", policy_failure.STEP_DIAGNOSTIC,
                            policy_failure.WORKFLOW_DIAGNOSTIC + "\nsubsequent unrelated failure"]:
                with self.subTest(message=message):
                    stderr.write_text(message)
                    self.assertFalse(policy_failure.expected_refusal(process, stderr, policy_failure.WORKFLOW_DIAGNOSTIC))

    def test_timeout_crash_success_missing_or_boolean_exit_never_qualifies(self):
        with tempfile.TemporaryDirectory() as temporary:
            stderr = Path(temporary) / "stderr"
            stderr.write_text(policy_failure.STEP_DIAGNOSTIC)
            for process in [{"exit_code": 1, "failure": "timeout"}, {"exit_code": -11, "failure": ""},
                            {"exit_code": 0, "failure": ""}, {"exit_code": True, "failure": ""},
                            {"exit_code": 1}, {}, None]:
                with self.subTest(process=process):
                    self.assertFalse(policy_failure.expected_refusal(process, stderr, policy_failure.STEP_DIAGNOSTIC))

    def test_oversized_or_non_utf8_logs_never_qualify(self):
        with tempfile.TemporaryDirectory() as temporary:
            stderr = Path(temporary) / "stderr"
            process = {"exit_code": 1, "failure": ""}
            stderr.write_bytes(b"x" * policy_failure.MAX_LOG_BYTES + policy_failure.WORKFLOW_DIAGNOSTIC.encode())
            self.assertFalse(policy_failure.expected_refusal(process, stderr, policy_failure.WORKFLOW_DIAGNOSTIC))
            stderr.write_bytes(b"\xff\n" + policy_failure.WORKFLOW_DIAGNOSTIC.encode())
            self.assertFalse(policy_failure.expected_refusal(process, stderr, policy_failure.WORKFLOW_DIAGNOSTIC))

    def test_each_fixture_declares_actual_container_security(self):
        workflow = policy_failure.fixture_source("workflow")
        step = policy_failure.fixture_source("step")
        self.assertIn("runtime: dockerimage\n", workflow)
        self.assertIn("\nsecurity:\n  network:\n    mode: offline\n", workflow)
        self.assertIn("\n    security:\n      network:\n        mode: offline\n", step)
        self.assertIn("id: guarded-container\n    runtime: dockerimage", step)
        self.assertNotIn("kind: host", step)
        self.assertIn("effect-marker", step)
        with self.assertRaises(ValueError):
            policy_failure.fixture_source("unknown")

    @staticmethod
    def accepted_observations():
        variants = [{"expected_refusal": True, "protected_fixture_unchanged": True} for _ in range(4)]
        endpoint = {"connections": 0, "listener_error": "", "listener_stopped": True,
                    "socket_removed": True, "directory_removed": True}
        return variants, endpoint

    def test_all_four_refusals_and_clean_endpoint_are_required(self):
        variants, endpoint = self.accepted_observations()
        self.assertTrue(all(policy_failure.observed_assertions(variants, endpoint, False, True).values()))
        self.assertFalse(policy_failure.observed_assertions(variants[:3], endpoint, False, True)["expected-failure-observed"])
        variants[2]["expected_refusal"] = False
        self.assertFalse(policy_failure.observed_assertions(variants, endpoint, False, True)["actionable-result"])

    def test_docker_invocation_or_connection_cannot_pass_without_container_effect(self):
        variants, endpoint = self.accepted_observations()
        self.assertFalse(policy_failure.observed_assertions(variants, endpoint, True, True)["no-unowned-effects"])
        for count in [1, 32, None, False, "0"]:
            with self.subTest(count=count):
                facts = dict(endpoint, connections=count)
                self.assertFalse(policy_failure.observed_assertions(variants, facts, False, True)["no-unowned-effects"])

    def test_source_artifact_or_endpoint_cleanup_failure_cannot_pass(self):
        variants, endpoint = self.accepted_observations()
        changed = copy.deepcopy(variants)
        changed[0]["protected_fixture_unchanged"] = False
        self.assertFalse(policy_failure.observed_assertions(changed, endpoint, False, True)["no-unowned-effects"])
        self.assertFalse(policy_failure.observed_assertions(variants, endpoint, False, False)["no-unowned-effects"])
        for fields in [{"listener_error": "counter failed"}, {"listener_stopped": False},
                       {"socket_removed": False}, {"directory_removed": False}]:
            with self.subTest(fields=fields):
                self.assertFalse(policy_failure.observed_assertions(variants, dict(endpoint, **fields), False, True)["no-unowned-effects"])

    def test_protected_inventory_detects_new_effects_content_changes_and_links(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / "config.yml"
            source.write_text(policy_failure.fixture_source("step"))
            before = policy_failure.protected_inventory(root)
            source.write_text("changed")
            self.assertNotEqual(before, policy_failure.protected_inventory(root))
            source.write_text(policy_failure.fixture_source("step"))
            marker = root / "effect-marker"
            marker.write_text("unexpected")
            self.assertNotEqual(before, policy_failure.protected_inventory(root))
            marker.unlink()
            (root / "alias").symlink_to(source)
            with self.assertRaisesRegex(ValueError, "special file"):
                policy_failure.protected_inventory(root)

    def test_generated_cache_is_recordable_without_allowing_authored_or_adjacent_changes(self):
        before = {"workspace": "directory", "workspace/config.yml": "a" * 64,
                  "outside-canary.txt": "b" * 64}
        after = dict(before, **{"workspace/bin": "directory", "workspace/bin/cache": "c" * 64})
        self.assertTrue(policy_failure.authored_fixture_unchanged(before, after))
        for fields in [{"workspace/config.yml": "c" * 64}, {"outside-canary.txt": "c" * 64},
                       {"workspace/effect-marker": "c" * 64}, {"workspace/binary": "c" * 64},
                       {"escaped-effect": "c" * 64}]:
            with self.subTest(fields=fields):
                self.assertFalse(policy_failure.authored_fixture_unchanged(before, dict(after, **fields)))
        del after["workspace/config.yml"]
        self.assertFalse(policy_failure.authored_fixture_unchanged(before, after))

    def test_private_endpoint_counter_and_exact_cleanup(self):
        try:
            endpoint = policy_failure.FakeDockerEndpoint()
        except PermissionError:
            self.skipTest("Private AF_UNIX fixture requires a reviewed host socket operation in this sandbox")
        try:
            self.assertEqual(endpoint.directory.stat().st_mode & 0o777, 0o700)
            self.assertEqual(endpoint.path.stat().st_mode & 0o777, 0o600)
            with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as client:
                client.connect(str(endpoint.path))
        finally:
            facts = endpoint.close()
        self.assertEqual(facts["connections"], 1)
        self.assertTrue(policy_failure.endpoint_clean(facts))

    def test_denied_socket_factory_removes_its_exact_private_directory(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary) / "owned-endpoint"
            directory.mkdir(mode=0o700)
            with mock.patch.object(policy_failure.tempfile, "mkdtemp", return_value=str(directory)), \
                    mock.patch.object(policy_failure.socket, "socket", side_effect=PermissionError("denied")):
                with self.assertRaises(policy_failure.SocketPrerequisiteUnavailable):
                    policy_failure.FakeDockerEndpoint()
            self.assertFalse(directory.exists())

    def test_socket_denial_emits_bound_blocked_receipt_before_any_cli_command(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            artifact = root / "pinned-fixture-cli"
            artifact.write_bytes(b"fixture not executed")
            request = {"run_id": "socket-denied", "plan_sha256": "1" * 64, "registry_sha256": "2" * 64,
                       "target": {"id": "local", "platform": "linux-portable", "kind": "local",
                                  "artifact_sha256": policy_failure.digest_file(artifact)},
                       "configuration": {"artifact": str(artifact)},
                       "cases": [{"id": "workflow.containers", "scenario": "failure", "layer": "component",
                                  "required_assertions": sorted(policy_failure.ASSERTIONS)}]}
            for stage in ("self-test", "candidate"):
                with self.subTest(stage=stage):
                    output = root / stage
                    warning = io.StringIO()
                    self_test = (mock.Mock(side_effect=policy_failure.SocketPrerequisiteUnavailable("denied"))
                                 if stage == "self-test" else mock.Mock(return_value={"counter_tested": True}))
                    with mock.patch.object(policy_failure, "endpoint_self_test", self_test), \
                            mock.patch.object(policy_failure.socket, "socket", side_effect=PermissionError("denied")), \
                            mock.patch.object(policy_failure, "run_driver") as execute, \
                            contextlib.redirect_stderr(warning):
                        policy_failure.run(request, output)
                    execute.assert_not_called()
                    result = json.loads((output / "result.json").read_text())
                    self.assertEqual(result["target"], request["target"])
                    self.assertEqual(result["plan_sha256"], request["plan_sha256"])
                    self.assertEqual(result["registry_sha256"], request["registry_sha256"])
                    self.assertEqual(result["cases"][0]["status"], "blocked")
                    self.assertEqual(result["cases"][0]["assertions"], [])
                    self.assertEqual(result["observed"]["docker_commands_started"], 0)
                    self.assertIn("WARNING:", warning.getvalue())
                    self.assertIn("Private AF_UNIX", result["cases"][0]["reason"])

    def test_unexpected_listener_fault_is_not_downgraded_to_blocked(self):
        with mock.patch.object(policy_failure, "FakeDockerEndpoint", side_effect=RuntimeError("internal fault")):
            with self.assertRaisesRegex(RuntimeError, "internal fault"):
                policy_failure.endpoint_self_test()


if __name__ == "__main__":
    unittest.main()
