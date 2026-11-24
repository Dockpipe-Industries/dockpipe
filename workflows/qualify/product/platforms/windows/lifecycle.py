"""Windows guest GUI orchestration using the shared owned desktop and public VM forms.

This module does not attest lifecycle success. Callers retain live UI, native
surface, guest identity and cleanup evidence for the shared lane receipt.
"""

import importlib.util
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time


def load_module(name, path):
    specification = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


def load_shared(name):
    return load_module(f"product_shared_{name}", Path(__file__).parents[1] / "linux" / f"{name}.py")


DESKTOP = load_shared("desktop")
UI = load_shared("ui")
NATIVE_ARGUMENTS = load_module("product_windows_native_arguments", Path(__file__).with_name("native_arguments.py"))


def resource_admission(directory):
    memory = dict(line.split(":", 1) for line in Path("/proc/meminfo").read_text().splitlines())
    available = int(memory["MemAvailable"].split()[0]) * 1024
    filesystem = os.statvfs(directory)
    free = filesystem.f_bavail * filesystem.f_frsize
    if available < 8 << 30 or free < 32 << 30:
        raise RuntimeError("Windows guest requires 8 GiB available RAM and 32 GiB free disk; no tuning performed.")
    return {"available_memory_bytes": available, "free_disk_bytes": free,
            "guest_memory_mib": 6144, "guest_vcpus": 4}


def qemu_admission(prefix):
    binary = Path(prefix) / "bin/qemu-system-x86_64"
    requirements = [(["-display", "help"], "dbus"), (["-netdev", "help"], "user"),
                    (["-drive", "format=help"], "vvfat")]
    observations = []
    for arguments, capability in requirements:
        process = subprocess.run([str(binary), *arguments], capture_output=True, text=True, timeout=10)
        output = process.stdout + process.stderr
        if process.returncode or capability not in output.replace(":", " ").split():
            raise RuntimeError(f"Selected QEMU lacks required Windows GUI installer capability: {capability}")
        observations.append({"arguments": arguments, "capability": capability, "exit_code": process.returncode})
    return observations


def validate_guest(guest, install):
    for name in ("disk", "installer", "firmware_code", "firmware_vars_template", "payload"):
        path = Path(guest[name])
        if not path.is_absolute() or path.is_symlink():
            raise ValueError(f"{name} must be an explicit absolute ordinary path")
    disk = Path(guest["disk"])
    if install and disk.exists():
        raise ValueError("Fresh Windows installation refuses an existing target disk")
    if not install and not disk.is_file():
        raise ValueError("Disk-only restart requires the owned installed disk")
    for name in ("installer", "firmware_code", "firmware_vars_template"):
        if not Path(guest[name]).is_file():
            raise ValueError(f"Missing {name}")
    if not Path(guest["payload"]).is_dir():
        raise ValueError("Missing prepared qualification payload")
    if DESKTOP.digest(guest["installer"]) != guest["installer_sha256"]:
        raise ValueError("Windows installation media digest mismatch")
    if not Path(guest["firmware_vars_template"]).as_posix().startswith("/usr/share/"):
        raise ValueError("Select a system firmware vars template so the owning package copies it into new guest state")


def guest_environment(guest, install):
    validate_guest(guest, install)
    environment = {
        "DOCKPIPE_VM_MEMORY": "6144",
        "DOCKPIPE_VM_CPUS": "4",
        "DOCKPIPE_VM_ACCEL": "kvm",
        "DOCKPIPE_VM_GUEST_PREPARATION": "automatic",
        "DOCKPIPE_VM_TPM": "required",
        "DOCKPIPE_VM_SECURE_BOOT": "required",
        "DOCKPIPE_VM_DISK": guest["disk"],
        "DOCKPIPE_VM_PERSISTENCE": "persistent",
        "DOCKPIPE_VM_DISK_BUS": "sata",
        "DOCKPIPE_VM_NET_DEVICE": "e1000e",
        "DOCKPIPE_VM_FIRMWARE_CODE": guest["firmware_code"],
        "DOCKPIPE_VM_FIRMWARE_VARS": guest["firmware_vars_template"],
        "DOCKPIPE_VM_KEEPALIVE": "true",
        "DOCKPIPE_VM_KEEPALIVE_SECONDS": "3600",
    }
    if install:
        environment.update(
            DOCKPIPE_VM_BOOT_SOURCE="installer-iso", DOCKPIPE_VM_CDROM=guest["installer"],
            DOCKPIPE_VM_DISK_SIZE="96G", DOCKPIPE_VM_WINDOWS_INSTALL_MODE="unattended",
            DOCKPIPE_VM_WINDOWS_IMAGE_INDEX="1",
            DOCKPIPE_VM_BOOTSTRAP_PATH=guest["payload"],
        )
    else:
        environment["DOCKPIPE_VM_BOOT_SOURCE"] = "image"
        environment["DOCKPIPE_VM_GUEST_PREPARATION"] = "provided-only"
    return environment


def validate_resume_identity(identity, guest, selections):
    """Admit only the stopped task guest whose durable identity was captured live."""
    if identity.get("schema") != "dockpipe.product-integration.windows-owned-identity/v1":
        raise ValueError("Restart requires an explicit captured Windows guest identity")
    if identity.get("original_guest_used") is not False:
        raise ValueError("Restart refuses an original or unclassified guest")
    if identity.get("guest_installed_claimed") is not True:
        raise ValueError("Restart requires observed guest installation before media detachment")
    if selections.get("workspace_path") != identity.get("workdir"):
        raise ValueError("Restart must retain the captured workspace")
    state = Path(identity["state_root"])
    if not state.is_absolute() or state.is_symlink() or not state.is_dir():
        raise ValueError("Restart requires the captured ordinary state directory")
    if str(Path(guest["disk"])) != identity["disk"]:
        raise ValueError("Restart disk differs from the captured guest")
    for name, inode_name in (("disk", "disk_inode"), ("firmware_vars", "firmware_vars_inode"),
                             ("tpm_directory", "tpm_directory_inode")):
        path = Path(identity[name])
        if path.is_symlink() or path.stat().st_ino != identity[inode_name]:
            raise ValueError(f"Restart identity changed: {name}")
        if name != "disk" and not path.is_relative_to(state):
            raise ValueError("Restart firmware and TPM must belong to the captured state")
    uuid_path = Path(identity["uuid_file"])
    if not uuid_path.is_relative_to(state) or uuid_path.is_symlink():
        raise ValueError("Restart UUID file must belong to the captured state")
    if uuid_path.read_text().strip() != identity["machine_uuid"]:
        raise ValueError("Restart machine UUID differs from the captured guest")
    for name in ("pid", "tpm_pid"):
        pid = identity[name]
        if not isinstance(pid, int) or pid <= 1 or Path("/proc", str(pid)).exists():
            raise ValueError("Restart requires independent absence of the previous guest processes")
    binary = Path(selections["qemu_prefix"]) / "bin/qemu-system-x86_64"
    if DESKTOP.digest(binary) != identity["qemu_sha256"]:
        raise ValueError("Restart QEMU artifact differs from the captured guest")
    return state


def read_existing_ssh_reference(path, identity):
    reference = Path(path)
    if not reference.is_absolute() or reference.is_symlink() or not reference.is_file():
        raise ValueError("Existing guest SSH selection requires an explicit private reference file")
    selection = json.loads(reference.read_text())
    if selection.get("expected_machine_uuid") != identity["machine_uuid"]:
        raise ValueError("Existing SSH references belong to another captured guest")
    user = selection.get("user")
    if not isinstance(user, str) or not 1 <= len(user) <= 256 or any(char in user for char in "\r\n\0"):
        raise ValueError("Existing guest SSH user reference is invalid")
    key = Path(selection["private_key_reference"])
    if not key.is_absolute() or key.is_symlink() or not key.is_file():
        raise ValueError("Existing guest SSH key must be an ordinary absolute file reference")
    # Admission and authentication belong to the package. Never load key bytes
    # into qualification configuration, logs, or accessible form snapshots.
    return {"user": user, "key_file": str(key)}


def manual_setup_extent_admitted(extent, dialog, footer_y, bottom_margin=10):
    """Reject clipped controls and choices that could overlap the Launch footer."""
    return (extent.width > 0 and extent.height > 0
            and extent.x >= dialog.x + 10
            and extent.x + extent.width <= dialog.x + dialog.width - 10
            and extent.y >= dialog.y + 100
            and extent.y + extent.height <= footer_y - bottom_margin)


class WindowsDesktop(DESKTOP.OwnedDesktop):
    def __init__(self, output, selections, guest, install=True, resume_identity=None):
        if not install and resume_identity is None:
            raise ValueError("Restart must reuse the existing owned GUI workspace/profile through captured identity")
        if install and resume_identity is not None:
            raise ValueError("Fresh installation cannot adopt an existing guest identity")
        self.guest = dict(guest)
        self.install = install
        self.form_configured = False
        self.action_processes = []
        configured = dict(selections)
        configured["guest_environment"] = guest_environment(guest, install)
        self.resume_identity = dict(resume_identity) if resume_identity is not None else None
        self.existing_ssh = None
        if self.resume_identity is not None:
            state = validate_resume_identity(self.resume_identity, guest, selections)
            if guest.get("ssh_transport_reference"):
                self.existing_ssh = read_existing_ssh_reference(guest["ssh_transport_reference"], self.resume_identity)
            if "prepare_environment" in selections:
                raise ValueError("Restart owns its environment preparation hook")

            def prepare_resume(environment):
                validate_resume_identity(self.resume_identity, self.guest, self.selections)
                for name in ("CDROM", "ISO", "VIRTIO_ISO", "WINDOWS_UNATTEND_DIR", "WINDOWS_UNATTEND_XML", "BOOTSTRAP_PATH",
                             "BOOTSTRAP_ISO", "GUEST_TOOLS_ISO", "WINDOWS_INSTALL_MODE",
                             "WINDOWS_IMAGE_INDEX"):
                    environment.pop("DOCKPIPE_VM_" + name, None)
                environment["XDG_STATE_HOME"] = str(state)

            configured["prepare_environment"] = prepare_resume
        super().__init__(output, configured)

    def start(self):
        capabilities = qemu_admission(self.selections["qemu_prefix"])
        resources = resource_admission(self.output.parent)
        process = super().start()
        (self.output / "windows-qemu-capabilities.json").write_text(json.dumps(capabilities, indent=2) + "\n")
        (self.output / "windows-resources.json").write_text(json.dumps(resources, indent=2) + "\n")
        dependencies = [Path(__file__), Path(__file__).with_name("ui_action.py"),
                        Path(__file__).with_name("native_arguments.py")]
        dependencies.extend(Path(__file__).parents[1] / "linux" / name for name in ("desktop.py", "ui.py"))
        dependencies.extend(Path(self.selections[name]) for name in ("cli", "gui"))
        dependencies.extend(Path(self.selections["gui"]).parent / name
                            for name in ("libdockpipe-launcher-ui.so.1.0", "dockpipe-terminal-host"))
        dependencies.extend(Path(self.selections["qemu_prefix"]) / "bin" / name
                            for name in ("qemu-system-x86_64", "qemu-img"))
        (self.output / "windows-dependencies.json").write_text(json.dumps([
            {"path": str(path), "sha256": DESKTOP.digest(path)} for path in dependencies
        ], indent=2) + "\n")
        return process

    def connect_ui(self):
        registry, session = UI.connect(self.output / "session-private.json")
        return registry, session["gui_pid"]

    def open_windows_form(self):
        self.form_configured = False
        registry, pid = self.connect_ui()
        UI.action(UI.select(registry.Registry, pid, "VM", "list item"), "Toggle")
        # Modal AT-SPI actions block the caller until the dialog closes.
        helper = Path(__file__).with_name("ui_action.py")
        log_path = self.output / f"windows-action-{len(self.action_processes)}.log"
        with log_path.open("xb") as log:
            process = subprocess.Popen(
                ["/usr/bin/python3", str(helper), "--session", str(self.output / "session-private.json"),
                 "--name", "Start Windows VM", "--role", "menu item", "--action", "Press"],
                stdout=log, stderr=log,
            )
        self.action_processes.append(process)
        self.wait_dialog()

    def wait_dialog(self):
        registry, pid = self.connect_ui()
        deadline = time.monotonic() + 10
        while time.monotonic() < deadline:
            matches = [node for node, _ in UI.nodes(registry.Registry, pid)
                       if node.getRoleName() == "dialog" and "Windows virtual machine" in node.name]
            if len(matches) == 1:
                return matches[0]
            time.sleep(0.1)
        raise RuntimeError("Expected one owned Windows workflow settings dialog")

    def capture_tree(self, name):
        registry, pid = self.connect_ui()
        (self.output / name).write_text(json.dumps(UI.tree(registry.Registry, pid), indent=2) + "\n")

    def dialog_nodes(self):
        queue = [(self.wait_dialog(), 0)]
        visited = 0
        while queue and visited < 2000:
            node, depth = queue.pop(0)
            visited += 1
            yield node
            if depth < 18:
                queue.extend((child, depth + 1) for child in node)
        if queue:
            raise RuntimeError("Windows form accessibility traversal exceeded its bound")

    def form_control(self, name, role):
        matches = [node for node in self.dialog_nodes() if node.name == name and node.getRoleName() == role]
        if len(matches) != 1:
            raise RuntimeError(f"Expected one Windows form {role}: {name}; found {len(matches)}")
        return matches[0]

    def set_text(self, name, value):
        node = self.form_control(name, "text")
        if not node.queryEditableText().setTextContents(value):
            raise RuntimeError(f"Windows form rejected field: {name}")
        if node.queryText().getText(0, -1) != value:
            raise RuntimeError(f"Windows form field read-back differs: {name}")

    def select_page(self, name):
        UI.action(self.form_control(name, "page tab"), "Press")

    def select_source(self, install):
        registry, pid = self.connect_ui()
        labels = {"Boot existing disk image", "Install from ISO"}
        matches = [node for node in self.dialog_nodes()
                   if node.getRoleName() == "combo box" and node.name in labels]
        if len(matches) != 1:
            raise RuntimeError("Expected one Windows boot source combo box")
        desired = "Install from ISO" if install else "Boot existing disk image"
        if matches[0].name == desired:
            return
        extent = matches[0].queryComponent().getExtents(registry.DESKTOP_COORDS)
        title = self.wait_dialog().name
        candidates = subprocess.run(
            ["xdotool", "search", "--all", "--pid", str(pid), "--name", "^Workflow Settings"],
            env=self.environment, capture_output=True, text=True, timeout=5, check=True,
        ).stdout.splitlines()
        windows = []
        for candidate in candidates:
            observed = subprocess.run(["xdotool", "getwindowname", candidate], env=self.environment,
                                      capture_output=True, text=True, timeout=5, check=True).stdout.strip()
            if observed == title:
                windows.append(candidate)
        if len(windows) != 1:
            raise RuntimeError("Expected one owned Windows workflow X11 dialog")
        subprocess.run(["xdotool", "windowfocus", "--sync", windows[0]],
                       env=self.environment, timeout=5, check=True)
        keys = ["Home", "Down", "Return"] if install else ["Home", "Return"]
        subprocess.run(["xdotool", "mousemove", str(extent.x + extent.width // 2),
                        str(extent.y + extent.height // 2), "click", "1", "key", *keys],
                       env=self.environment, timeout=5, check=True)
        self.form_control(desired, "combo box")

    def set_media_fields(self, values):
        registry, _ = self.connect_ui()
        nodes = [node for node in self.dialog_nodes()
                 if node.getRoleName() == "text" and not node.name
                 and node.getState().contains(registry.STATE_SHOWING)]
        # The current generic path inference renders Disk Size as an unnamed
        # path editor too. The live published Windows form declares installer,
        # optional VirtIO, destination, size in that order; require all four.
        if len(nodes) != len(values):
            raise RuntimeError("Windows media controls differ from the pinned workflow field order")
        for node, value in zip(nodes, values):
            if not node.queryEditableText().setTextContents(value):
                raise RuntimeError("Windows media field rejected the selected path")
            if node.queryText().getText(0, -1) != value:
                raise RuntimeError("Windows media path read-back differs")

    def set_ssh_key_file(self, value):
        registry, _ = self.connect_ui()
        fields = [node for node in self.dialog_nodes()
                  if node.getRoleName() in ("text", "password text")
                  and node.name == "SSH Key File"]
        if not fields:
            fields = [node for node in self.dialog_nodes()
                      if node.getRoleName() in ("text", "password text") and not node.name
                      and node.getState().contains(registry.STATE_SHOWING)]
        if len(fields) != 1:
            raise RuntimeError("Expected the one declared Connectivity SSH key file picker")
        field = fields[0]
        if not field.queryEditableText().setTextContents(value):
            raise RuntimeError("SSH key file picker rejected its existing file reference")
        observed = field.queryText().getText(0, -1)
        protected = (field.getRoleName() == "password text" and len(observed) == len(value)
                     and bool(observed) and set(observed) <= {"*", "\u2022", "\u25cf"})
        if observed != value and not protected:
            raise RuntimeError("SSH key reference read-back differs")
        # This generic file control masks KEY-named values. Require an exact
        # package session reference comparison after Launch as well; UI length
        # and mask checks alone do not attest the emitted transport selection.
        (self.output / "windows-ssh-reference-ui.json").write_text(json.dumps({
            "field": "SSH Key File", "protected_display": protected,
            "reference_length": len(value),
            "reference_path_sha256": hashlib.sha256(value.encode()).hexdigest(),
            "key_contents_read": False, "package_reference_verification_required": True,
        }, indent=2) + "\n")

    def configure_form(self, install=True):
        if not install and not Path(self.guest["disk"]).is_file():
            raise ValueError("Restart requires the owned newly installed disk")
        self.select_source(True)
        self.select_page("Install Media")
        self.set_media_fields([self.guest["installer"] if install else "", "", self.guest["disk"], "96G"])
        if not install:
            self.select_source(False)
            self.select_page("Existing Image")
            self.set_media_fields([self.guest["disk"]])
            if self.existing_ssh is not None:
                self.set_text("SSH User", self.existing_ssh["user"])
        self.select_page("Machine")
        self.set_media_fields(["sata", self.guest["firmware_code"], self.guest["firmware_vars_template"], ""])
        for name, value in (("Persistence", "persistent"), ("CPUs", "4"), ("Memory", "6144")):
            self.set_text(name, value)
        self.select_page("Connectivity")
        if self.existing_ssh is not None:
            self.set_ssh_key_file(self.existing_ssh["key_file"])
        for name, value in (("Network Adapter", "e1000e"), ("TPM", "required"), ("Secure Boot", "required")):
            self.set_text(name, value)
        self.install = install
        self.form_configured = True
        self.capture_tree("windows-form-configured.json")

    def select_manual_setup(self):
        """Select the package's visible manual mode without accepting the dialog."""
        registry, _ = self.connect_ui()
        self.select_page("Advanced")
        dialog = self.wait_dialog()
        bounds = dialog.queryComponent().getExtents(registry.DESKTOP_COORDS)
        footer = self.form_control("Launch", "push button").queryComponent().getExtents(registry.DESKTOP_COORDS)

        for attempt in range(21):
            candidates = [node for node in self.dialog_nodes()
                          if node.getRoleName() == "combo box"
                          and node.name not in ("Boot existing disk image", "Install from ISO")]
            if len(candidates) != 1:
                raise RuntimeError("Expected the one declared Manual setup choice on Advanced")
            control = candidates[0]
            extent = control.queryComponent().getExtents(registry.DESKTOP_COORDS)
            if manual_setup_extent_admitted(extent, bounds, footer.y, bottom_margin=80):
                break
            if attempt == 20:
                raise RuntimeError("Manual setup did not enter the admitted settings viewport")
            # Qt reports clipped controls as SHOWING and does not implement
            # grabFocus here. Scroll the normal dialog; never click a clipped
            # field or send Return, which could accept its default Launch.
            subprocess.run(["xdotool", "mousemove", str(bounds.x + bounds.width - 35),
                            str(footer.y - 40), "click", "5"],
                           env=self.environment, timeout=5, check=True)
            time.sleep(0.1)
        if control.name != "true":
            subprocess.run(["xdotool", "mousemove", str(extent.x + extent.width // 2),
                            str(extent.y + extent.height // 2), "click", "1"],
                           env=self.environment, timeout=5, check=True)
            choice = self.form_control("true", "list item")
            option = choice.queryComponent().getExtents(registry.DESKTOP_COORDS)
            if not manual_setup_extent_admitted(option, bounds, footer.y):
                raise RuntimeError("Manual setup true option is outside the admitted viewport")
            subprocess.run(["xdotool", "mousemove", str(option.x + option.width // 2),
                            str(option.y + option.height // 2), "click", "1"],
                           env=self.environment, timeout=5, check=True)
        self.form_control("true", "combo box")
        self.capture_tree("windows-manual-setup-selected.json")

    def launch(self, slot_granted=False):
        if not slot_granted:
            raise RuntimeError("Explicit serialized Windows guest slot grant required before GUI Launch")
        if not self.form_configured:
            raise RuntimeError("Windows form must be configured and read back before GUI Launch")
        validate_guest(self.guest, self.install)
        if self.resume_identity is not None:
            validate_resume_identity(self.resume_identity, self.guest, self.selections)
        resources = resource_admission(self.output)
        (self.output / "windows-before-launch-resources.json").write_text(json.dumps(resources, indent=2) + "\n")
        self.form_configured = False
        UI.action(self.form_control("Launch", "push button"), "Press")

    def resource_row(self, title):
        registry, pid = self.connect_ui()
        matches = []
        for node, _ in UI.nodes(registry.Registry, pid):
            if node.getRoleName() != "table":
                continue
            table = node.queryTable()
            if table.nRows > 256 or table.nColumns < 2 or table.nColumns > 16:
                continue
            for row in range(table.nRows):
                if table.getAccessibleAt(row, 0).name == title:
                    matches.append((table, row))
        if len(matches) != 1:
            raise RuntimeError("Expected one exact owned VM resource row from the fresh public snapshot")
        return matches[0]

    def request_shutdown(self, run_id, title):
        if not run_id or "(" + run_id + ")" not in title:
            raise ValueError("Shutdown requires a provisioned public resource title containing the exact owned run ID")
        registry, pid = self.connect_ui()
        UI.action(UI.select(registry.Registry, pid, "VM", "list item"), "Toggle")
        table, row = self.resource_row(title)
        cell = table.getAccessibleAt(row, 0)
        extent = cell.queryComponent().getExtents(registry.DESKTOP_COORDS)
        subprocess.run(["xdotool", "mousemove", str(extent.x + extent.width // 2),
                        str(extent.y + extent.height // 2), "click", "1"],
                       env=self.environment, timeout=5, check=True)
        if not cell.getState().contains(registry.STATE_SELECTED):
            raise RuntimeError("Owned VM resource selection did not read back as selected")
        action = UI.select(registry.Registry, pid, "Shut down guest", "menu item")
        if not action.getState().contains(registry.STATE_ENABLED):
            raise RuntimeError("Public guest shutdown is unavailable; wait for authenticated Guest Tools readiness")
        UI.action(action, "Press")
        (self.output / "windows-shutdown-request.json").write_text(json.dumps({
            "run_id": run_id, "resource_title": title, "action": "shutdown", "status": "requested-only",
        }, indent=2) + "\n")

    def wait_stopped(self, title, timeout=120):
        if not 1 <= timeout <= 180:
            raise ValueError("Stopped-state observation timeout must be from 1 to 180 seconds")
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            table, row = self.resource_row(title)
            state = table.getAccessibleAt(row, 1).name
            if state == "stopped":
                (self.output / "windows-ui-stopped.json").write_text(json.dumps({
                    "resource_title": title, "state": state,
                    "limits": "Public UI observation only; verify exact native process/display cleanup separately.",
                }, indent=2) + "\n")
                return
            time.sleep(0.5)
        raise RuntimeError("Owned Windows resource did not report stopped within the observation bound")

    def stop(self):
        # Guest shutdown/cancellation is a separate observed public UI action.
        # Closing this owned test GUI alone is not guest cleanup evidence.
        for process in self.action_processes:
            if process.poll() is None:
                process.terminate()
                process.wait(timeout=5)
        super().stop()
