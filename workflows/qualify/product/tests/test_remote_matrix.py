"""Installed role matrix covers the remote registry and admits exact evidence."""

from copy import deepcopy
from pathlib import Path
import json
import subprocess
import sys
import tempfile
import unittest

OWNER = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(OWNER / "assets/scripts"))
from contracts import ContractError, digest_file, read_json, validate_registry
from remote_matrix import MANUAL_SERVER, OBSERVATION_SCHEMA, remote_matrix
from remote_work_plan import remote_work_plan


class RemoteMatrixTests(unittest.TestCase):
    def setUp(self):
        self.registry = validate_registry(read_json(OWNER / "registry.json"))

    def write_observations(self, path, *observations):
        path.write_text(json.dumps({"schema": OBSERVATION_SCHEMA, "observations": observations}), encoding="utf-8")

    def test_matrix_preserves_endpoint_roles_and_manual_server_order(self):
        matrix = remote_matrix(self.registry)
        self.assertEqual(matrix["counts"]["passed"], 0)
        self.assertEqual(matrix["counts"]["failed"], 0)
        seen_server = False
        for lane in matrix["lanes"]:
            server = MANUAL_SERVER in lane["participants"].values()
            if server:
                seen_server = True
                self.assertEqual(lane["status"], "blocked")
                self.assertIn("Windows Server requires", lane["reason"])
            elif seen_server:
                self.fail("A non-Server combination followed Windows Server")
        self.assertTrue(seen_server)
        self.assertTrue(all(row.get("reason") for row in matrix["lanes"] if row["status"] == "blocked"))
        self.assertTrue(any(row["feature"] == "remote.rdp" for row in matrix["unsupported"]))
        self.assertTrue(any(row["role"] == "worker" and row["platform"] == "windows-amd64-msi"
                            and row["status"] == "unrun" for row in matrix["roles"]))
        self.assertTrue(any(row["role"] == "desktop-sender" and row["platform"] == "linux-amd64-deb-ubuntu24.04"
                            for row in matrix["unsupported"]))

        for windows in ("windows-amd64-msi", "windows-11-25h2-arm64"):
            enrollment = next(row for row in matrix["lanes"] if row["feature"] == "remote.enrollment"
                              and row["participants"] == {"background-worker": windows})
            self.assertEqual(enrollment["status"], "unrun")
            self.assertTrue(any(row["role"] == "broker" and row["platform"] == windows
                                for row in matrix["unsupported"]))

    def test_background_worker_requires_platform_supervision_evidence(self):
        registry = deepcopy(self.registry)
        requirements = registry["remote_qualification"]["features"]["remote.enrollment"]
        checks = set(requirements["service_assertions"]["background-worker"])
        lanes = [lane for lane in remote_matrix(registry)["lanes"] if lane["feature"] == "remote.enrollment"]
        background = [lane for lane in lanes if "background-worker" in lane["participants"]]
        cli_platforms = {platform["id"] for platform in registry["platforms"] if "all-cli" in platform["selectors"]}
        self.assertEqual({lane["participants"]["background-worker"] for lane in background}, cli_platforms)
        for lane in lanes:
            required = set(lane["required_assertions"])
            if "background-worker" in lane["participants"]:
                self.assertLessEqual(checks, required)
            else:
                self.assertTrue(checks.isdisjoint(required))

        requirements["service_assertions"]["operator"] = ["unexpected-service-check"]
        with self.assertRaises(ContractError):
            remote_matrix(registry)

    def test_browser_and_desktop_require_exact_roles_and_assertions(self):
        rows = remote_matrix(self.registry)["lanes"]
        local = next(row for row in rows if row["feature"] == "browser.local")
        for assertion in ("address-draft-preserved-during-tab-switch",
                          "extension-load-enable-popup-disable-remove",
                          "devtools-open-inspect-close",
                          "site-permission-denied-and-remembered"):
            self.assertIn(assertion, local["required_assertions"])

        browser = next(row for row in rows if row["feature"] == "remote.browser")
        self.assertEqual(set(browser["participants"]), {"worker", "browser-viewer"})
        self.assertIn("interactive-test-app", browser["required_assertions"])
        self.assertIn("remote-session-reconnected", browser["required_assertions"])

        clipboard = next(row for row in rows if row["feature"] == "remote.desktop.clipboard")
        self.assertEqual(set(clipboard["participants"]), {"desktop-sender", "desktop-viewer"})
        for kind in ("text", "image", "file"):
            for direction in ("viewer-to-sender", "sender-to-viewer"):
                self.assertIn(f"{kind}-{direction}", clipboard["required_assertions"])
        displays = next(row for row in rows if row["feature"] == "remote.desktop.viewer")
        self.assertIn("frame-from-every-attached-monitor", displays["required_assertions"])
        self.assertIn("mixed-dpi-alignment", displays["required_assertions"])

    def test_work_plan_groups_exact_roles_and_counts_shared_blockers(self):
        matrix = remote_matrix(self.registry)
        plan = remote_work_plan(self.registry, matrix)

        self.assertEqual(plan["counts"], matrix["counts"])
        self.assertEqual(plan["platforms"][-1]["id"], MANUAL_SERVER)
        checks = [(row["feature"], row["participants"], row["status"])
                  for batch in plan["batches"] for row in batch["checks"]]
        expected = [(lane["feature"], lane["participants"], lane["status"])
                    for lane in matrix["lanes"] if lane["status"] in {"failed", "unrun"}]
        self.assertEqual(sorted(checks, key=str), sorted(expected, key=str))
        self.assertTrue(all(batch["checks"] and MANUAL_SERVER not in batch["platforms"]
                            for batch in plan["batches"]))
        self.assertTrue(any({"remote.files", "remote.git", "remote.workflow"} <=
                            {row["feature"] for row in batch["checks"]} for batch in plan["batches"]))

        windows = next(row for row in plan["blockers"] if "Native Windows" in row["reason"])
        server = next(row for row in plan["blockers"] if "Windows Server requires" in row["reason"])
        self.assertEqual(windows["affected_lanes"], sum("Native Windows" in lane.get("reason", "")
                                                       for lane in matrix["lanes"]))
        self.assertEqual(server["affected_lanes"], sum("Windows Server requires" in lane.get("reason", "")
                                                      for lane in matrix["lanes"]))
        self.assertTrue(any("Native Windows" in lane.get("reason", "")
                            and "Windows Server requires" in lane["reason"] for lane in matrix["lanes"]))
        self.assertIn({"check": "manual-windows-server-vm", "platform": MANUAL_SERVER,
                       "affected_lanes": server["affected_lanes"]}, plan["hands_on"])
        self.assertEqual(plan["hands_on"][-1]["platform"], MANUAL_SERVER)
        self.assertTrue(any(row["check"] == "hardware-key" for row in plan["hands_on"]))
        self.assertTrue(any(row["check"] == "mixed-dpi-displays" for row in plan["hands_on"]))

    def test_pinned_wayland_desktop_keeps_local_browser_and_worker_roles(self):
        arch = "arch-linux-2026.11.01-amd64"
        matrix = remote_matrix(self.registry)
        roles = {row["role"]: row["status"] for row in matrix["roles"] if row["platform"] == arch}

        self.assertEqual(roles["local-browser"], "unrun")
        self.assertEqual(roles["worker"], "unrun")
        for role in ("browser-viewer", "desktop-viewer"):
            self.assertEqual(roles[role], "unsupported")
            self.assertFalse(any(lane["participants"].get(role) == arch for lane in matrix["lanes"]))

        profile = next(platform for platform in self.registry["platforms"] if platform["id"] == arch)
        profile["baseline"]["display_backend"] = "x11"
        x11_matrix = remote_matrix(validate_registry(self.registry))
        self.assertTrue(any(lane["participants"].get("browser-viewer") == arch for lane in x11_matrix["lanes"]))

        profile["baseline"]["display_backend"] = "auto"
        with self.assertRaises(ContractError):
            validate_registry(self.registry)

    def test_pass_is_bound_to_installed_role_pair_and_hashed_evidence(self):
        matrix = remote_matrix(self.registry)
        lane = next(row for row in matrix["lanes"] if row["feature"] == "remote.browser")
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            proof = root / "fixture.json"
            proof.write_text("installed Browser fixture data", encoding="utf-8")
            observation = {
                "feature": lane["feature"],
                "participants": lane["participants"],
                "status": "passed",
                "layer": "installed_e2e",
                "passed_assertions": lane["required_assertions"],
                "artifact_sha256": {role: "a" * 64 for role in lane["participants"]},
                "evidence": [{"path": proof.name, "sha256": digest_file(proof)}],
            }
            receipt = root / "observations.json"
            self.write_observations(receipt, observation)
            admitted = remote_matrix(self.registry, [receipt])
            self.assertEqual(admitted["counts"]["passed"], 1)
            partial = deepcopy(observation)
            partial["passed_assertions"].pop()
            self.write_observations(receipt, partial)
            with self.assertRaises(ContractError):
                remote_matrix(self.registry, [receipt])
            partial["status"] = "unrun"
            self.write_observations(receipt, partial)
            retained = remote_matrix(self.registry, [receipt])
            self.assertEqual(retained["counts"]["passed"], 0)
            proof.write_text("mismatched evidence", encoding="utf-8")
            self.write_observations(receipt, observation)
            with self.assertRaises(ContractError):
                remote_matrix(self.registry, [receipt])

    def test_cli_keeps_independent_evidence_roots_and_rejects_duplicate_lanes(self):
        lanes = [lane for lane in remote_matrix(self.registry)["lanes"] if lane["feature"] == "browser.local"]
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            paths = []
            for label, lane in zip(("first", "second"), lanes):
                owner = root / label
                owner.mkdir()
                proof = owner / "proof.txt"
                proof.write_text(label, encoding="utf-8")
                observation = {
                    "feature": lane["feature"], "participants": lane["participants"],
                    "status": "passed", "layer": "installed_e2e", "passed_assertions": lane["required_assertions"],
                    "artifact_sha256": {"local-browser": "a" * 64},
                    "evidence": [{"path": proof.name, "sha256": digest_file(proof)}],
                }
                receipt = owner / "observations.json"
                self.write_observations(receipt, observation)
                paths.append(receipt)

            command = [sys.executable, str(OWNER / "assets/scripts/qualify.py"), "--remote-matrix"]
            for receipt in paths:
                command.extend(["--remote-observations", str(receipt)])
            run = subprocess.run(command, capture_output=True, check=True, timeout=10)
            matrix = json.loads(run.stdout)
            self.assertEqual(matrix["counts"]["passed"], 2)
            self.assertEqual({lane["observation_sha256"] for lane in matrix["lanes"] if lane["status"] == "passed"},
                             {digest_file(path) for path in paths})
            self.assertNotIn(str(root).encode(), run.stdout)
            with self.assertRaisesRegex(ContractError, "Duplicate remote feature"):
                remote_matrix(self.registry, [paths[0], paths[0]])

    def test_new_remote_features_require_role_inventory(self):
        self.registry["remote_qualification"]["features"].pop("remote.files")
        with self.assertRaises(ContractError):
            remote_matrix(self.registry)

    def test_blocked_lane_evidence_is_verified_without_installed_endpoints(self):
        lane = next(row for row in remote_matrix(self.registry)["lanes"]
                    if row["feature"] == "remote.browser" and row["status"] == "unrun")
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            evidence = root / "catalog.json"
            evidence.write_text("[]\n", encoding="utf-8")
            receipt = root / "observations.json"
            self.write_observations(receipt, {
                "feature": lane["feature"], "participants": lane["participants"],
                "status": "blocked", "reason": "No live worker route in the broker catalog.",
                "evidence": [{"path": evidence.name, "sha256": digest_file(evidence)}],
            })
            verified = remote_matrix(self.registry, [receipt])
            self.assertEqual(verified["counts"]["blocked"], remote_matrix(self.registry)["counts"]["blocked"] + 1)
            evidence.write_text('{"changed": true}\n', encoding="utf-8")
            with self.assertRaisesRegex(ContractError, "Remote evidence SHA-256 mismatch"):
                remote_matrix(self.registry, [receipt])
