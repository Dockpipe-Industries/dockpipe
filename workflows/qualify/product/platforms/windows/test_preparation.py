"""Preparation guards only; no GUI, installer, or guest process is started."""

import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from types import SimpleNamespace
from unittest.mock import patch


def load(name):
    specification = importlib.util.spec_from_file_location(name, Path(__file__).with_name(name + ".py"))
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


PAYLOAD = load("payload")
LIFECYCLE = load("lifecycle")


class PreparationTests(unittest.TestCase):
    def test_manual_setup_rejects_clipped_or_footer_overlapping_controls(self):
        dialog = SimpleNamespace(x=200, y=150, width=760, height=620)
        visible = SimpleNamespace(x=420, y=490, width=76, height=38)
        clipped = SimpleNamespace(x=420, y=1020, width=76, height=38)
        near_footer = SimpleNamespace(x=420, y=650, width=76, height=38)
        self.assertTrue(LIFECYCLE.manual_setup_extent_admitted(visible, dialog, 711, 80))
        self.assertFalse(LIFECYCLE.manual_setup_extent_admitted(clipped, dialog, 711, 80))
        self.assertFalse(LIFECYCLE.manual_setup_extent_admitted(near_footer, dialog, 711, 80))
        # An open option has a smaller margin, but cannot cover Launch itself.
        option = SimpleNamespace(x=420, y=690, width=76, height=30)
        self.assertFalse(LIFECYCLE.manual_setup_extent_admitted(option, dialog, 711))

    def test_payload_mismatch_refuses_output_creation(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            msi = root / "selected.msi"
            msi.write_bytes(b"wrong artifact")
            target_hash = "a" * 64
            request = {"schema": "dockpipe.product-integration.request/v1",
                       "target": {"artifact_sha256": target_hash}}
            artifacts = {"msi": {"path": str(msi), "sha256": target_hash}}
            with self.assertRaisesRegex(ValueError, "artifact digest mismatch"):
                PAYLOAD.prepare(root / "payload", request, artifacts, "C:\\DockpipeQualification\\run-01")
            self.assertFalse((root / "payload").exists())

    def test_installer_refuses_existing_disk_before_other_prerequisites(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            disk = root / "protected.qcow2"
            disk.write_bytes(b"preserve")
            guest = {name: str(root / name) for name in (
                "installer", "firmware_code", "firmware_vars_template", "payload")}
            guest["disk"] = str(disk)
            with self.assertRaisesRegex(ValueError, "existing target disk"):
                LIFECYCLE.validate_guest(guest, True)
            self.assertEqual(disk.read_bytes(), b"preserve")

    def test_launch_requires_explicit_slot_before_any_gui_action(self):
        desktop = object.__new__(LIFECYCLE.WindowsDesktop)
        with self.assertRaisesRegex(RuntimeError, "slot grant"):
            desktop.launch()

    def test_launch_requires_successful_form_readback(self):
        desktop = object.__new__(LIFECYCLE.WindowsDesktop)
        desktop.form_configured = False
        with self.assertRaisesRegex(RuntimeError, "configured and read back"):
            desktop.launch(slot_granted=True)

    def test_restart_cannot_create_another_profile_identity(self):
        with self.assertRaisesRegex(ValueError, "existing owned GUI workspace/profile"):
            LIFECYCLE.WindowsDesktop("unused", {}, {}, install=False)

    def test_disk_restart_omits_bootstrap_and_install_media(self):
        guest = {"disk": "/task/disk", "payload": "/task/payload",
                 "firmware_code": "/system/code", "firmware_vars_template": "/system/vars"}
        with patch.object(LIFECYCLE, "validate_guest"):
            environment = LIFECYCLE.guest_environment(guest, False)
        self.assertEqual(environment["DOCKPIPE_VM_GUEST_PREPARATION"], "provided-only")
        self.assertEqual(environment["DOCKPIPE_VM_BOOT_SOURCE"], "image")
        for name in ("CDROM", "WINDOWS_UNATTEND_DIR", "BOOTSTRAP_PATH", "BOOTSTRAP_ISO"):
            self.assertNotIn("DOCKPIPE_VM_" + name, environment)

    def test_resume_refuses_uninstalled_and_original_guest(self):
        identity = {"schema": "dockpipe.product-integration.windows-owned-identity/v1",
                    "original_guest_used": True}
        with self.assertRaisesRegex(ValueError, "original or unclassified"):
            LIFECYCLE.validate_resume_identity(identity, {}, {})
        identity["original_guest_used"] = False
        with self.assertRaisesRegex(ValueError, "observed guest installation"):
            LIFECYCLE.validate_resume_identity(identity, {}, {})

    def test_existing_ssh_selection_uses_only_same_guest_file_references(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            key = root / "existing key"
            key.write_bytes(b"not key material: the form selector must not parse contents")
            reference = root / "private-reference.json"
            reference.write_text(json.dumps({"expected_machine_uuid": "owned-uuid", "user": "fixture-user",
                                             "private_key_reference": str(key)}))
            self.assertEqual(LIFECYCLE.read_existing_ssh_reference(reference, {"machine_uuid": "owned-uuid"}),
                             {"user": "fixture-user", "key_file": str(key)})
            with self.assertRaisesRegex(ValueError, "another captured guest"):
                LIFECYCLE.read_existing_ssh_reference(reference, {"machine_uuid": "another-uuid"})

    def test_resume_requires_same_disk_firmware_tpm_uuid_and_stopped_processes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            state = root / "state"
            state.mkdir()
            disk = root / "disk.qcow2"
            disk.write_bytes(b"owned disk")
            firmware = state / "vars.fd"
            firmware.write_bytes(b"owned firmware")
            tpm = state / "tpm"
            tpm.mkdir()
            uuid_file = state / "guest.uuid"
            uuid_file.write_text("owned-machine-uuid\n")
            binary = root / "bin/qemu-system-x86_64"
            binary.parent.mkdir()
            binary.write_bytes(b"selected qemu")
            identity = {
                "schema": "dockpipe.product-integration.windows-owned-identity/v1",
                "original_guest_used": False, "guest_installed_claimed": True,
                "workdir": str(root), "state_root": str(state),
                "disk": str(disk), "disk_inode": disk.stat().st_ino,
                "firmware_vars": str(firmware), "firmware_vars_inode": firmware.stat().st_ino,
                "tpm_directory": str(tpm), "tpm_directory_inode": tpm.stat().st_ino,
                "uuid_file": str(uuid_file), "machine_uuid": "owned-machine-uuid",
                "pid": 2147483646, "tpm_pid": 2147483645,
                "qemu_sha256": LIFECYCLE.DESKTOP.digest(binary),
            }
            selections = {"workspace_path": str(root), "qemu_prefix": str(root)}
            guest = {"disk": str(disk)}
            self.assertEqual(LIFECYCLE.validate_resume_identity(identity, guest, selections), state)
            identity["firmware_vars_inode"] += 1
            with self.assertRaisesRegex(ValueError, "firmware_vars"):
                LIFECYCLE.validate_resume_identity(identity, guest, selections)
            identity["firmware_vars_inode"] = firmware.stat().st_ino
            uuid_file.write_text("changed-uuid\n")
            with self.assertRaisesRegex(ValueError, "machine UUID"):
                LIFECYCLE.validate_resume_identity(identity, guest, selections)


if __name__ == "__main__":
    unittest.main()
