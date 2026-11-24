"""Challenge the generic package-workflow bridge, without claiming product proof."""
from pathlib import Path
import copy
import sys
import tempfile
import unittest


OWNER = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(OWNER / "drivers"))
sys.path.insert(0, str(OWNER / "assets/scripts"))
import package_workflow
from contracts import ContractError, digest_file


class PackageWorkflowTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.output = Path(self.temporary.name)
        self.operation = self.output / "operation"
        self.operation.mkdir()
        evidence = self.operation / "observation.txt"
        evidence.write_text("Protocol fixture only; not product qualification.\n")
        self.request = {
            "run_id": "bridge-contract", "plan_sha256": "a" * 64, "registry_sha256": "b" * 64,
            "target": {"id": "fixture", "artifact_sha256": "c" * 64},
            "cases": [{"id": "fixture", "scenario": "happy", "layer": "component",
                       "required_assertions": ["observed"]}],
        }
        self.receipt = {
            "schema": "dockpipe.product-integration.lane/v1",
            **{name: self.request[name] for name in ("run_id", "plan_sha256", "registry_sha256", "target")},
            "cases": [{"id": "fixture", "scenario": "happy", "layer": "component", "status": "passed",
                       "assertions": [{"id": "observed", "status": "passed", "detail": "Protocol fixture"}],
                       "evidence": [{"path": "observation.txt", "sha256": digest_file(evidence)}]}],
        }
        self.proof = [{"path": "bridge.json", "sha256": "d" * 64}]

    def tearDown(self):
        self.temporary.cleanup()

    def forward(self, receipt=None, exit_code=0, failure=""):
        return package_workflow.forwarded_cases(self.receipt if receipt is None else receipt,
            self.request, self.operation, {"exit_code": exit_code, "failure": failure}, self.proof)

    def test_nested_evidence_is_prefixed_without_mutating_owner_receipt(self):
        original = copy.deepcopy(self.receipt)
        rows = self.forward()
        self.assertEqual(rows[0]["evidence"][0]["path"], "operation/observation.txt")
        self.assertEqual(rows[0]["evidence"][-1], self.proof[0])
        self.assertEqual(self.receipt, original)

    def test_nonzero_or_deadline_cannot_forward_owner_pass(self):
        for exit_code, failure in ((1, ""), (0, "deadline"), (-15, "")):
            with self.subTest(exit_code=exit_code, failure=failure):
                self.assertEqual(self.forward(exit_code=exit_code, failure=failure)[0]["status"], "failed")

    def test_changed_target_is_rejected(self):
        receipt = copy.deepcopy(self.receipt)
        receipt["target"]["artifact_sha256"] = "e" * 64
        with self.assertRaises(ContractError):
            self.forward(receipt)

    def test_nested_evidence_escape_or_changed_digest_is_rejected(self):
        for path, digest in (("../observation.txt", "d" * 64), ("observation.txt", "e" * 64)):
            with self.subTest(path=path):
                receipt = copy.deepcopy(self.receipt)
                receipt["cases"][0]["evidence"] = [{"path": path, "sha256": digest}]
                with self.assertRaises(ContractError):
                    self.forward(receipt)

    def test_request_hash_substitution_is_one_exact_argument(self):
        digest = "a" * 64
        actual = package_workflow.workflow_arguments("workflows/probe/config.yml",
            ["--execute-reviewed", "--approve-request-sha256", "@request-sha256"], digest)
        self.assertEqual(actual[-1], digest)
        with self.assertRaises(ContractError):
            package_workflow.workflow_arguments("workflows/probe/config.yml", ["prefix@request-sha256"], digest)

    def test_provider_cannot_override_request_output_or_separator(self):
        for argument in ("--request", "--request=/other", "--output", "--output=/other", "--"):
            with self.subTest(argument=argument), self.assertRaises(ContractError):
                package_workflow.workflow_arguments("workflows/probe/config.yml", [argument], "a" * 64)

    def test_archive_member_admission_is_host_independent(self):
        for member in ("/config.yml", "../config.yml", "workflows/../config.yml", "C:/config.yml",
                       "workflows\\config.yml", "workflows//config.yml", "./config.yml"):
            with self.subTest(member=member), self.assertRaises(ContractError):
                package_workflow.workflow_arguments(member, [], "a" * 64)


if __name__ == "__main__":
    unittest.main()
