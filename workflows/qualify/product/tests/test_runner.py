"""Adversarial evidence, coverage and bounded-process contracts for the runner."""
from pathlib import Path
import copy
import contextlib
import importlib.util
import io
import json
import os
import signal
import subprocess
import sys
import tempfile
import textwrap
import time
import unittest
from unittest.mock import patch


SCRIPTS = Path(__file__).resolve().parents[1] / "assets/scripts"
sys.path.insert(0, str(SCRIPTS))
from contracts import (ContractError, contained_file, digest_file, read_json,
                       validate_plan, validate_receipt, validate_registry, write_json)
from coverage import expected_cases, overall_summary, target_summary
from process import run_driver
import qualify


class QualificationContracts(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.registry = validate_registry(read_json(SCRIPTS.parents[1] / "registry.json"))
        self.target = {"id": "selected", "platform": "linux-portable", "kind": "local",
                       "artifact_sha256": "a" * 64}
        self.case = {"id": "install.cli", "scenario": "happy", "layer": "installed_e2e",
                     "required_assertions": ["version", "execution"]}
        self.request = {"run_id": "test", "plan_sha256": "b" * 64,
                        "registry_sha256": "c" * 64, "target": self.target, "cases": [self.case]}
        proof = self.root / "proof.txt"
        proof.write_text("actual observation fixture\n")
        self.row = {"id": "install.cli", "scenario": "happy", "layer": "installed_e2e",
                    "status": "passed", "assertions": [
                        {"id": value, "status": "passed"} for value in self.case["required_assertions"]],
                    "evidence": [{"path": "proof.txt", "sha256": digest_file(proof)}]}
        self.receipt = {key: self.request[key] for key in ("run_id", "plan_sha256", "registry_sha256", "target")}
        self.receipt.update(schema="dockpipe.product-integration.lane/v1", cases=[self.row])

    def tearDown(self):
        self.temporary.cleanup()

    def test_complete_bound_observation_is_admitted(self):
        self.assertEqual(len(validate_receipt(self.receipt, self.request, self.root)), 1)

    def test_missing_assertion_cannot_pass(self):
        self.row["assertions"].pop()
        with self.assertRaises(ContractError):
            validate_receipt(self.receipt, self.request, self.root)

    def test_changed_evidence_cannot_pass(self):
        (self.root / "proof.txt").write_text("replaced")
        with self.assertRaises(ContractError):
            validate_receipt(self.receipt, self.request, self.root)

    def test_wrong_product_artifact_cannot_pass(self):
        self.receipt["target"] = dict(self.target, artifact_sha256="d" * 64)
        with self.assertRaises(ContractError):
            validate_receipt(self.receipt, self.request, self.root)

    def test_extra_or_duplicate_case_cannot_pass(self):
        self.receipt["cases"].append(copy.deepcopy(self.row))
        with self.assertRaises(ContractError):
            validate_receipt(self.receipt, self.request, self.root)

    def test_symlink_evidence_cannot_pass(self):
        (self.root / "linked.txt").symlink_to(self.root / "proof.txt")
        with self.assertRaises(ContractError):
            contained_file(self.root, "linked.txt")

    def test_traversal_and_noncanonical_evidence_rejected(self):
        for value in ("../proof.txt", "./proof.txt", "/proof.txt", "a/../proof.txt", "a\\proof.txt"):
            with self.subTest(value=value), self.assertRaises(ContractError):
                contained_file(self.root, value)

    def test_repository_source_selector_keeps_provider_with_owning_workflow(self):
        reference = "repo://workflows/build/flatpak/assets/scripts/build.sh"
        self.assertEqual(qualify.selected_source(reference),
                         qualify.REPOSITORY / "workflows/build/flatpak/assets/scripts/build.sh")

    def test_source_selector_cannot_escape_repository(self):
        for reference in ("repo://../outside.py", "repo:///etc/passwd", "host:///etc/passwd"):
            with self.subTest(reference=reference), self.assertRaises(ContractError):
                qualify.selected_source(reference)

    def test_duplicate_json_member_rejected(self):
        path = self.root / "duplicate.json"
        path.write_text('{"status":"failed","status":"passed"}')
        with self.assertRaises(ContractError):
            read_json(path)

    def test_non_json_numeric_literals_rejected(self):
        for literal in ("NaN", "Infinity", "-Infinity"):
            path = self.root / (literal + ".json")
            path.write_text('{"value":' + literal + '}')
            with self.subTest(literal=literal), self.assertRaises(ContractError):
                read_json(path)

    def test_malformed_receipt_shapes_have_controlled_rejection(self):
        malformed = [None, []]
        for key, value in (("assertions", [None]), ("evidence", [None]), ("status", []), ("id", {})):
            receipt = copy.deepcopy(self.receipt)
            receipt["cases"][0][key] = value
            malformed.append(receipt)
        receipt = copy.deepcopy(self.receipt)
        receipt["cases"] = [None]
        malformed.append(receipt)
        for receipt in malformed:
            with self.subTest(receipt=receipt), self.assertRaises(ContractError):
                validate_receipt(receipt, self.request, self.root)

    def test_plan_and_registry_capture_bind_the_validated_bytes(self):
        plan = {"schema": "dockpipe.product-integration.plan/v1", "run_id": "captured",
                "targets": [self.target]}
        plan_path = self.root / "plan.json"
        registry_path = self.root / "registry.json"
        write_json(plan_path, plan)
        write_json(registry_path, self.registry)
        original_plan = plan_path.read_bytes()
        original_registry = registry_path.read_bytes()
        admission = qualify.admit_sources

        def change_source_paths_after_admission(captured_plan, captured_registry):
            admission(captured_plan, captured_registry)
            plan_path.write_text('{"run_id":"mutated-after-admission"}')
            registry_path.write_text('{"schema":"mutated-after-admission"}')

        output = self.root / "captured-output"
        arguments = ["qualify.py", "--registry", str(registry_path), "--plan", str(plan_path), "--output", str(output)]
        with patch.object(sys, "argv", arguments), patch.object(qualify, "admit_sources", change_source_paths_after_admission):
            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                self.assertEqual(qualify.main(), 2)
        self.assertEqual((output / "plan.json").read_bytes(), original_plan)
        self.assertEqual((output / "registry.json").read_bytes(), original_registry)
        report = read_json(output / "result.json")
        self.assertEqual(report["run_id"], "captured")
        self.assertEqual(report["plan_sha256"], digest_file(output / "plan.json"))
        self.assertEqual(report["registry_sha256"], digest_file(output / "registry.json"))
        matrix_path = output / "remote-matrix.json"
        self.assertEqual(report["remote_matrix"], {"path": matrix_path.name, "sha256": digest_file(matrix_path)})
        self.assertEqual(read_json(matrix_path)["counts"], report["overall"]["remote_role_coverage"])

    def test_manual_or_missing_observations_remain_unpassed(self):
        expected = expected_cases(self.registry, self.target)
        summary = target_summary(self.target, expected, {})
        self.assertEqual(summary["passed"], 0)
        self.assertGreater(summary["required"], 0)
        self.assertEqual(summary["coverage"], 0)

    def test_selected_platform_does_not_qualify_all_platforms(self):
        summary = {"required": 1, "passed": 1, "status": "qualified", "target": self.target}
        result = overall_summary(self.registry, [summary])
        self.assertTrue(result["selected_targets_qualified"])
        self.assertFalse(result["full_product_qualified"])
        self.assertIn("windows-amd64-msi", result["unconfigured_platform_profiles"])
        self.assertEqual(result["unconfigured_platform_profiles"][-1], "windows-server-2025-amd64")

    def test_pinned_expansion_requires_cli_upgrade_and_desktop_checks(self):
        platforms = [platform for platform in self.registry["platforms"]
                     if "package-upgrade" in platform["selectors"]]
        self.assertTrue(platforms)

        for platform in platforms:
            with self.subTest(platform=platform["id"]):
                target = dict(self.target, platform=platform["id"])
                cases = {case["id"]: case for case in expected_cases(self.registry, target)
                         if case["scenario"] == "happy" and case["layer"] == "installed_e2e"}
                self.assertIn("host-workflow-executed", cases["install.cli"]["required_assertions"])
                self.assertTrue(cases["install.upgrade"]["critical"])
                self.assertEqual("install.desktop" in cases, "desktop" in platform["baseline"])

    def test_upgrade_profile_without_pinned_release_is_rejected(self):
        registry = copy.deepcopy(self.registry)
        registry["platforms"][-1]["baseline"]["release"] = ""
        with self.assertRaisesRegex(ContractError, "Platform baseline fields"):
            validate_registry(registry)

    def test_container_cannot_substitute_for_workstation_vm(self):
        target = dict(self.target, platform="fedora-workstation-44-amd64", kind="container")
        plan = {"schema": "dockpipe.product-integration.plan/v1", "run_id": "test", "targets": [target]}
        with self.assertRaisesRegex(ContractError, "execution kind differs"):
            validate_plan(plan, self.registry)

    def test_malformed_pin_rejected_before_execution(self):
        plan = {"schema": "dockpipe.product-integration.plan/v1", "run_id": "test",
                "targets": [dict(self.target, artifact_sha256=None)]}
        with self.assertRaises(ContractError):
            validate_plan(plan, self.registry)

    def test_failed_critical_check_prevents_qualification(self):
        expected = [{"id": "critical", "scenario": "happy", "layer": "installed_e2e", "critical": True}]
        observed = {("critical", "happy", "installed_e2e"): dict(expected[0], status="failed")}
        summary = target_summary(self.target, expected, observed)
        self.assertEqual(summary["status"], "failed")

    def test_fast_process_log_overflow_rejected_after_exit(self):
        execution = run_driver([sys.executable, "-c", "import sys;sys.stdout.write('x'*5000000)"],
                               self.root, 5, dict(os.environ))
        self.assertIn("log exceeded", execution["failure"])

    def test_process_deadline_enforced(self):
        execution = run_driver([sys.executable, "-c", "import time;time.sleep(10)"],
                               self.root, 0.1, dict(os.environ))
        self.assertIn("deadline", execution["failure"])

    @unittest.skipUnless(hasattr(os, "waitid"), "POSIX process group supervision")
    def test_deadline_terminates_child_after_driver_exits(self):
        import fcntl

        lock_path = self.root / "child.lock"
        ready_path = self.root / "child.pid"
        child_program = textwrap.dedent("""
            import fcntl
            import os
            from pathlib import Path
            import signal
            import sys
            import time

            signal.signal(signal.SIGTERM, signal.SIG_IGN)
            with open(sys.argv[1], "w") as lock:
                fcntl.flock(lock, fcntl.LOCK_EX)
                ready = Path(sys.argv[2])
                pending = ready.with_suffix(".pending")
                pending.write_text(str(os.getpid()))
                pending.replace(ready)
                time.sleep(30)
        """)
        driver_program = textwrap.dedent("""
            import subprocess
            import sys
            import time

            subprocess.Popen([sys.executable, "-c", *sys.argv[1:]])
            time.sleep(30)
        """)

        try:
            execution = run_driver(
                [sys.executable, "-c", driver_program, child_program, str(lock_path), str(ready_path)],
                self.root, 1, dict(os.environ))
            self.assertIn("deadline", execution["failure"])
            self.assertTrue(ready_path.is_file(), "Child did not make it to the termination fixture")
            with lock_path.open("r") as lock:
                deadline = time.monotonic() + 2
                while True:
                    try:
                        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
                        break
                    except BlockingIOError:
                        if time.monotonic() >= deadline:
                            self.fail("Child remained alive after the driver deadline")
                        time.sleep(0.02)
        finally:
            if ready_path.is_file():
                with lock_path.open("r") as lock:
                    try:
                        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
                    except BlockingIOError:
                        with contextlib.suppress(ProcessLookupError):
                            os.kill(int(ready_path.read_text()), signal.SIGKILL)

    def test_private_result_creation_refuses_overwrite(self):
        path = self.root / "result.json"
        write_json(path, {"status": "failed"})
        with self.assertRaises(FileExistsError):
            write_json(path, {"status": "passed"})
        self.assertEqual(json.loads(path.read_text())["status"], "failed")

    def fixture_lane(self, exit_code):
        """A runner protocol fixture, never registered as product acceptance."""
        script = self.root / "fixture.py"
        script.write_text(textwrap.dedent('''\
            import argparse
            import hashlib
            import json
            from pathlib import Path

            parser = argparse.ArgumentParser()
            parser.add_argument("--request")
            parser.add_argument("--output")
            arguments = parser.parse_args()
            request = json.loads(Path(arguments.request).read_text())
            output = Path(arguments.output)
            output.mkdir()
            evidence = output / "fixture.txt"
            evidence.write_text("runner protocol fixture")
            rows = []
            for case in request["cases"]:
                row = dict(case)
                row.pop("required_assertions")
                row["status"] = "passed"
                row["assertions"] = [{"id": name, "status": "passed"}
                                     for name in case["required_assertions"]]
                row["evidence"] = [{"path": "fixture.txt",
                                    "sha256": hashlib.sha256(evidence.read_bytes()).hexdigest()}]
                rows.append(row)
            receipt = {key: request[key] for key in ("run_id", "plan_sha256", "registry_sha256", "target")}
            receipt.update(schema="dockpipe.product-integration.lane/v1", cases=rows)
            (output / "result.json").write_text(json.dumps(receipt))
            ''') + "raise SystemExit(" + str(exit_code) + ")\n")
        driver = {"id": "fixture", "entrypoint": "fixture.py", "sha256": digest_file(script),
                  "features": ["install.cli"], "scenarios": ["happy"], "layers": ["installed_e2e"]}
        return dict(self.target, drivers=[driver])

    def run_fixture(self, target, resume=False):
        original = qualify.OWNER
        qualify.OWNER = self.root
        try:
            with contextlib.redirect_stderr(io.StringIO()):
                return qualify.run_target(target, self.registry, "b" * 64, "c" * 64,
                                          {"run_id": "test"}, self.root / "run", io.StringIO(), resume)
        finally:
            qualify.OWNER = original

    def test_nonzero_driver_cannot_supply_passing_coverage(self):
        result = self.run_fixture(self.fixture_lane(7))
        self.assertEqual(result["passed"], 0)
        self.assertEqual(result["counts"]["failed"], 1)

    def test_resume_revalidates_without_replaying(self):
        target = self.fixture_lane(0)
        result = self.run_fixture(target)
        self.assertEqual(result["passed"], 1)
        evidence = self.root / "run/selected/fixture/output/fixture.txt"
        previous = evidence.stat().st_mtime_ns
        result = self.run_fixture(target, resume=True)
        self.assertEqual(result["passed"], 1)
        self.assertEqual(previous, evidence.stat().st_mtime_ns)

    def test_incomplete_invocation_is_retained_without_replay(self):
        target = self.fixture_lane(0)
        invocation = self.root / "run/selected/fixture"
        invocation.mkdir(parents=True)
        sentinel = invocation / "uncertain-effect.txt"
        sentinel.write_text("retain exact resource reference")
        result = self.run_fixture(target, resume=True)
        self.assertEqual(result["passed"], 0)
        self.assertFalse((invocation / "output").exists())
        self.assertEqual(sentinel.read_text(), "retain exact resource reference")

    def test_human_packet_does_not_auto_pass(self):
        specification = importlib.util.spec_from_file_location("human_driver", SCRIPTS.parents[1] / "drivers/manual.py")
        module = importlib.util.module_from_spec(specification)
        specification.loader.exec_module(module)
        request = copy.deepcopy(self.request)
        request["cases"][0]["layer"] = "human"
        module.run(request, {}, self.root / "human")
        receipt = read_json(self.root / "human/result.json")
        self.assertEqual(receipt["cases"][0]["status"], "manual_pending")

    @unittest.skipUnless(sys.platform.startswith("linux"), "Linux portable Docker adapter")
    def test_unavailable_docker_prerequisite_is_blocked_without_fixture_effects(self):
        specification = importlib.util.spec_from_file_location("docker_driver", SCRIPTS.parents[1] / "drivers/docker.py")
        module = importlib.util.module_from_spec(specification)
        specification.loader.exec_module(module)
        request = copy.deepcopy(self.request)
        artifact = self.root / "proof.txt"
        request["target"]["artifact_sha256"] = digest_file(artifact)
        request["configuration"] = {"artifact": str(artifact), "image": "sha256:" + "d" * 64,
                                    "endpoint": "unix:///var/run/docker.sock"}
        request["cases"] = [{"id": "workflow.containers", "scenario": "happy", "layer": "installed_e2e",
                             "required_assertions": sorted(module.ASSERTIONS)}]
        output = self.root / "docker-blocked"
        with patch.object(module, "docker_read", side_effect=FileNotFoundError("fixture missing Docker")):
            module.run(request, output)
        receipt = read_json(output / "result.json")
        self.assertEqual(receipt["cases"][0]["status"], "blocked")
        self.assertFalse((output / "install").exists())
        self.assertFalse((output / "workspace").exists())

    @unittest.skipIf(os.name == "nt", "POSIX SIGTERM cancellation contract")
    def test_sigterm_stops_owned_driver_and_records_incomplete_report(self):
        fixture = SCRIPTS.parents[1] / "tests/fixtures/waiting_driver.py"
        target = dict(self.target, drivers=[{
            "id": "waiting", "entrypoint": "tests/fixtures/waiting_driver.py", "sha256": digest_file(fixture),
            "features": ["install.cli"], "scenarios": ["happy"], "layers": ["installed_e2e"]}])
        plan = {"schema": "dockpipe.product-integration.plan/v1", "run_id": "interrupt-fixture", "targets": [target]}
        plan_path = self.root / "plan.json"
        write_json(plan_path, plan)
        output = self.root / "interrupted"
        process = subprocess.Popen([sys.executable, str(SCRIPTS / "qualify.py"), "--plan", str(plan_path),
                                    "--output", str(output)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        try:
            marker = output / "selected/waiting/output/started.json"
            deadline = time.monotonic() + 5
            while not marker.exists() and process.poll() is None and time.monotonic() < deadline:
                time.sleep(0.02)
            self.assertTrue(marker.exists(), "Driver did not reach the cancellation observation point")
            driver_pid = read_json(marker)["pid"]
            process.send_signal(signal.SIGTERM)
            self.assertEqual(process.wait(timeout=5), 130)
            report = read_json(output / "result.json")
            self.assertTrue(report["interrupted"])
            self.assertFalse(report["overall"]["full_product_qualified"])
            self.assertGreater(report["overall"]["required"], 0)
            with self.assertRaises(ProcessLookupError):
                os.kill(driver_pid, 0)
        finally:
            if process.poll() is None:
                process.kill()
                process.wait(timeout=5)


if __name__ == "__main__":
    unittest.main()
