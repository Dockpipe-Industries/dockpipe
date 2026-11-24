"""Admission tests; these do not qualify Windows product behavior."""

import importlib.util
import json
from pathlib import Path
import struct
import tempfile
import unittest
from unittest import mock
import zipfile


SPECIFICATION = importlib.util.spec_from_file_location("windows_driver", Path(__file__).with_name("driver.py"))
DRIVER = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(DRIVER)


def request():
    return {
        "schema": "dockpipe.product-integration.request/v1",
        "run_id": "windows-driver-admission",
        "plan_sha256": "a" * 64,
        "registry_sha256": "b" * 64,
        "target": {
            "id": "windows-fixture", "platform": "windows-amd64-msi", "kind": "vm",
            "artifact_sha256": "c" * 64,
        },
        "configuration": {
            "cli": "C:\\candidate\\dockpipe.exe", "cli_sha256": "d" * 64,
            "artifact": "C:\\candidate\\dockpipe.msi",
        },
        "cases": [{
            "id": "install.cli", "scenario": "happy", "layer": "installed_e2e",
            "required_assertions": list(DRIVER.ASSERTIONS),
        }],
    }


class AdmissionTests(unittest.TestCase):
    def test_rejects_unknown_configuration_and_invalid_timeout(self):
        for values in ({"password": "unaccepted"}, {"timeout_seconds": True}, {"timeout_seconds": 181}):
            with self.subTest(values=values):
                selection = request()
                selection["configuration"].update(values)
                with self.assertRaises(ValueError):
                    DRIVER.validate_request(selection)

    def test_requires_windows_target_and_separate_executable_pin(self):
        selection = request()
        selection["target"]["platform"] = "linux-amd64"
        with self.assertRaises(ValueError):
            DRIVER.validate_request(selection)
        selection = request()
        del selection["configuration"]["cli_sha256"]
        with self.assertRaises(ValueError):
            DRIVER.validate_request(selection)

    @unittest.skipIf(DRIVER.os.name == "nt", "Exercises non-Windows admission only")
    def test_other_host_cannot_execute_or_pass_windows_case(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            selection = root / "request.json"
            selection.write_text(json.dumps(request()), encoding="utf-8")
            output = root / "result"
            with mock.patch("sys.argv", ["driver.py", "--request", str(selection), "--output", str(output)]):
                with mock.patch.object(DRIVER, "qualify_cli") as qualify:
                    DRIVER.main()
            qualify.assert_not_called()
            result = json.loads((output / "result.json").read_text(encoding="utf-8"))
            self.assertEqual(result["cases"][0]["status"], "blocked")
            self.assertEqual(result["cases"][0]["assertions"], [])
            self.assertFalse((output / "workspace").exists())

    def test_artifact_mismatch_refuses_workspace_and_process_creation(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            cli = root / "dockpipe.exe"
            cli.write_bytes(b"selected fixture")
            artifact = root / "dockpipe.msi"
            artifact.write_bytes(b"wrong artifact")
            configuration = {"cli": str(cli), "cli_sha256": DRIVER.digest(cli), "artifact": str(artifact)}
            with mock.patch.object(DRIVER, "run_command") as command:
                with self.assertRaisesRegex(ValueError, "artifact digest"):
                    DRIVER.qualify_cli(configuration, request(), root)
            command.assert_not_called()
            self.assertFalse((root / "workspace").exists())

    def test_wrong_native_or_executable_architecture_is_blocked_before_running_cli(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            artifact = root / "dockpipe.msi"
            artifact.write_bytes(b"pinned fixture MSI")
            cli = root / "dockpipe.exe"
            header = bytearray(64)
            header[:2] = b"MZ"
            struct.pack_into("<I", header, 60, 64)
            cli.write_bytes(header + b"PE\0\0" + struct.pack("<H", 0x8664))
            selection = request()
            selection["target"].update(platform="windows-11-25h2-arm64", artifact_sha256=DRIVER.digest(artifact))
            configuration = {"cli": str(cli), "cli_sha256": DRIVER.digest(cli), "artifact": str(artifact)}

            with mock.patch.object(DRIVER, "run_command") as command:
                for native, message in ((0x8664, "Native Windows architecture"), (0xAA64, "CLI PE architecture")):
                    with self.subTest(native=native), mock.patch.object(DRIVER, "native_windows_machine", return_value=native):
                        with self.assertRaisesRegex(ValueError, message):
                            DRIVER.qualify_cli(configuration, selection, root)
                command.assert_not_called()

    def test_msi_requires_matching_installation_receipt_and_log(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            artifact = root / "dockpipe.msi"
            cli = root / "dockpipe.exe"
            log = root / "msi-install.log"
            log.write_text("Installer fixture", encoding="utf-8")
            selection = request()
            configuration = selection["configuration"]
            with self.assertRaisesRegex(ValueError, "installation receipt"):
                DRIVER.verify_delivery(configuration, selection, artifact, cli)

            receipt_path = root / "msi-install.json"
            configuration["install_receipt"] = str(receipt_path)
            receipt = {
                **{key: selection[key] for key in DRIVER.RUN_BINDINGS},
                "schema": DRIVER.MSI_RECEIPT_SCHEMA,
                "artifact": str(artifact),
                "msi_sha256": selection["target"]["artifact_sha256"],
                "cli": str(cli),
                "installed_cli_sha256": configuration["cli_sha256"],
                "installer_exit_code": 0,
                "installer_log_sha256": DRIVER.digest(log),
            }
            receipt_path.write_text(json.dumps(receipt), encoding="utf-8")
            self.assertEqual(DRIVER.verify_delivery(configuration, selection, artifact, cli),
                             {"msi-install.json": receipt_path.read_bytes(), "msi-install.log": log.read_bytes()})

            for key, value in (("run_id", "unrelated"), ("artifact", str(root / "another.msi")),
                               ("installed_cli_sha256", "e" * 64), ("installer_exit_code", 1),
                               ("installer_log_sha256", "f" * 64)):
                with self.subTest(key=key):
                    receipt_path.write_text(json.dumps({**receipt, key: value}), encoding="utf-8")
                    with self.assertRaises(ValueError):
                        DRIVER.verify_delivery(configuration, selection, artifact, cli)

    def test_portable_artifact_requires_its_own_executable(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            artifact = root / "dockpipe.zip"
            cli = root / "dockpipe.exe"
            cli.write_bytes(b"selected")
            configuration = {"cli_sha256": DRIVER.digest(cli)}
            selection = request()
            with self.assertRaisesRegex(ValueError, "requires an MSI"):
                DRIVER.verify_delivery(configuration, selection, artifact, cli)
            selection["target"]["platform"] = "windows-11-25h2-arm64"
            with zipfile.ZipFile(artifact, "w") as archive:
                archive.writestr("dockpipe.exe", b"unlinked")
            with self.assertRaisesRegex(ValueError, "differs from the executable"):
                DRIVER.verify_delivery(configuration, selection, artifact, cli)

            with zipfile.ZipFile(artifact, "w") as archive:
                archive.write(cli, "dockpipe.exe")
            self.assertEqual(DRIVER.verify_delivery(configuration, selection, artifact, cli), {})


if __name__ == "__main__":
    unittest.main()
