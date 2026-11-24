"""Prevent root/disabled privilege policy and changed executable false passes."""
from pathlib import Path
import importlib.util
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch


OWNER = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(OWNER / "assets/scripts"))
from contracts import ContractError, digest_file


specification = importlib.util.spec_from_file_location("docker_policy_driver", OWNER / "drivers/docker.py")
driver = importlib.util.module_from_spec(specification)
specification.loader.exec_module(driver)


class DockerPolicyRegression(unittest.TestCase):
    def setUp(self):
        self.observation = {"network": "none", "readonly_root": True,
                            "memory_bytes": 128 * 1024 * 1024, "cpu_nanoseconds": 1000000000,
                            "pid_limit": 64, "effective_uid": 1000, "kernel_no_new_privileges": 1,
                            "dropped_capabilities": ["ALL"], "security_options": ["no-new-privileges"]}

    def test_enabled_policy_and_kernel_identity_qualify(self):
        self.assertTrue(driver.declared_policy_observed(self.observation))

    def test_root_with_nonroot_group_cannot_qualify(self):
        for user in ("0:1000", "root:1000"):
            observation = dict(self.observation, process_user=user, effective_uid=0)
            with self.subTest(user=user):
                self.assertFalse(driver.declared_policy_observed(observation))

    def test_disabled_or_absent_no_new_privileges_cannot_qualify(self):
        for option in ("no-new-privileges=false", "no-new-privileges:0", "other-option"):
            observation = dict(self.observation, security_options=[option])
            with self.subTest(option=option):
                self.assertFalse(driver.declared_policy_observed(observation))
        self.assertFalse(driver.declared_policy_observed(dict(self.observation, kernel_no_new_privileges=0)))

    def test_missing_kernel_identity_cannot_be_replaced_by_config_user(self):
        observation = dict(self.observation, process_user="1000:1000")
        observation.pop("effective_uid")
        self.assertFalse(driver.declared_policy_observed(observation))

    @unittest.skipUnless(sys.platform.startswith("linux"), "Linux portable Docker adapter")
    def test_replaced_artifact_cannot_reach_workflow_execution(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            artifact = root / "admitted-cli"
            artifact.write_bytes(b"admitted fixture executable")
            image = "sha256:" + "d" * 64
            request = {"target": {"platform": "linux-portable", "kind": "local", "artifact_sha256": digest_file(artifact)},
                       "configuration": {"artifact": str(artifact), "image": image, "endpoint": "unix:///var/run/docker.sock"},
                       "cases": [{"id": "workflow.containers", "scenario": "happy", "layer": "installed_e2e",
                                  "required_assertions": sorted(driver.ASSERTIONS)}]}

            def replace_during_copy(_source, destination):
                destination.write_bytes(b"different unadmitted executable")

            inspected = subprocess.CompletedProcess([], 0, stdout=(image + "\n").encode(), stderr=b"")
            with patch.object(driver, "docker_read", return_value=inspected):
                with patch.object(driver.shutil, "copyfile", side_effect=replace_during_copy):
                    with patch.object(driver, "run_driver", side_effect=AssertionError("changed CLI executed")):
                        with self.assertRaises(ContractError):
                            driver.run(request, root / "output")
            self.assertFalse((root / "output/workspace").exists())


if __name__ == "__main__":
    unittest.main()
