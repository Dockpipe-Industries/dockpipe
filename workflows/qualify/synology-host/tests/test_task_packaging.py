"""Check the unprivileged task companion's archive and lifecycle contracts."""

import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import struct
import subprocess
import tarfile
import tempfile
import unittest
from unittest.mock import patch


SCRIPT = Path(__file__).resolve().parents[1] / "assets/scripts/build_task.py"
specification = importlib.util.spec_from_file_location("task_qualification_packaging", SCRIPT)
BUILD = importlib.util.module_from_spec(specification)
specification.loader.exec_module(BUILD)


class TaskPackagingTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.ui = self.root / "task-ui-fixture"
        self.ui.mkdir()
        self.config = {BUILD.JAVASCRIPT: {BUILD.APPLICATION: {"type": "app"}}}
        (self.ui / "config").write_text(json.dumps(self.config))
        (self.ui / BUILD.JAVASCRIPT).write_text("/* Bounded UI packaging fixture; no live scheduler actions. */\n")
        self.ui_patch = patch.object(BUILD, "UI_ASSETS", self.ui)
        self.ui_patch.start()
        self.addCleanup(self.ui_patch.stop)

    def binary(self, architecture="amd64"):
        content = bytearray(120)
        content[:7] = b"\x7fELF\x02\x01\x01"
        struct.pack_into("<HH", content, 16, 2, BUILD.PACKAGING.ARCHITECTURES[architecture][2])
        struct.pack_into("<Q", content, 32, 64)
        struct.pack_into("<HH", content, 54, 56, 1)
        struct.pack_into("<I", content, 64, 1)
        binary = self.root / ("task-host-" + architecture)
        binary.write_bytes(content)
        return binary

    def test_separate_package_has_no_elevated_executables_or_dependencies(self):
        for architecture in ("amd64", "arm64"):
            with self.subTest(architecture=architecture):
                binary = self.binary(architecture)
                destination = BUILD.build(architecture, binary, self.root)
                with tarfile.open(destination) as package:
                    info = package.extractfile("INFO").read().decode()
                    self.assertIn('package="dockpipe-host-task-qualification"', info)
                    self.assertIn('version="0.1.0-3"', info)
                    self.assertIn('dsmappname="com.dockpipe.HostTaskQualification"', info)
                    self.assertNotIn("install_dep_packages", info)
                    self.assertEqual(json.load(package.extractfile("conf/privilege")), {
                        "defaults": {"run-as": "package"},
                    })
                    resource = json.load(package.extractfile("conf/resource"))
                    self.assertEqual(resource["systemd-user-unit"], {})
                    self.assertEqual(resource["web-config"]["nginx-static-config"]["enable"], [
                        {"type": "dsm", "relpath": "nginx/report.conf"},
                    ])
                    unit = package.extractfile("conf/systemd/" + BUILD.UNIT).read().decode()
                    self.assertIn("NoNewPrivileges=true", unit)
                    self.assertIn("/bin/task-host serve", unit)
                    self.assertNotIn("User=root", unit)
                    self.assertNotIn("AmbientCapabilities", unit)
                    payload = package.extractfile("package.tgz").read()
                    self.assertIn(f'checksum="{hashlib.md5(payload, usedforsecurity=False).hexdigest()}"', info)
                    with tarfile.open(fileobj=io.BytesIO(payload), mode="r:gz") as contents:
                        program = contents.getmember("bin/task-host")
                        self.assertEqual(program.mode, 0o755)
                        self.assertTrue(program.isfile())
                        self.assertEqual(contents.extractfile(program).read(), binary.read_bytes())
                        self.assertEqual(contents.extractfile("ui/config").read(), (self.ui / "config").read_bytes())
                        self.assertEqual(contents.extractfile("ui/" + BUILD.JAVASCRIPT).read(), (self.ui / BUILD.JAVASCRIPT).read_bytes())
                        self.assertFalse(any(name.endswith((".pem", ".key")) for name in contents.getnames()))
                        proxy = contents.extractfile("nginx/report.conf").read().decode()
                        self.assertIn("proxy_pass https://127.0.0.1:38142/report;", proxy)
                        self.assertIn("proxy_ssl_verify on;", proxy)
                        self.assertIn("proxy_ssl_name localhost;", proxy)
                        self.assertIn("/dockpipe-host-task-qualification/var/endpoint-public/ca.pem;", proxy)
                        self.assertNotIn("proxy_ssl_verify off", proxy)
                        self.assertIn("access_log off;", proxy)
                    control = package.extractfile("scripts/start-stop-status").read()
                    subprocess.run(["sh", "-n"], input=control, check=True)
                    self.assertIn(b'--cacert "$root/var/endpoint-public/ca.pem"', control)
                    self.assertIn(b"https://127.0.0.1:38142/health", control)
                    self.assertNotIn(b"--insecure", control)

    def lifecycle_fixture(self, action):
        package_root = self.root / "installed-task-package"
        for directory in ("target/bin", "target/ui/images", "var"):
            (package_root / directory).mkdir(parents=True, exist_ok=True)
        marker = self.root / "endpoint-init-called"
        program = package_root / "target/bin/task-host"
        program.write_text('#!/bin/sh\n[ "$1" = endpoint-init ] || exit 99\nprintf initialized > "$TASK_INIT_MARKER"\n')
        program.chmod(0o755)
        control = (BUILD.TASK_ASSETS / "package-control.sh").read_text()
        control = control.replace(
            "root=/var/packages/dockpipe-host-task-qualification",
            "root=" + str(package_root),
        )
        path = self.root / action
        path.write_text(control)
        return package_root, path, dict(os.environ, TASK_INIT_MARKER=str(marker)), marker

    def test_postinstall_initializes_endpoint_and_exposes_only_callback_traversal(self):
        package_root, control, environment, marker = self.lifecycle_fixture("postinst")
        result = subprocess.run(["sh", str(control)], env=environment, capture_output=True, timeout=5)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(marker.exists())
        for name in ("target", "target/bin", "target/ui", "target/ui/images", "var", "var/run"):
            self.assertEqual((package_root / name).stat().st_mode & 0o777, 0o755)

    def test_postinstall_refuses_linked_callback_directory(self):
        package_root, control, environment, marker = self.lifecycle_fixture("postinst")
        outside = self.root / "unrelated-directory"
        outside.mkdir(mode=0o700)
        (package_root / "var/run").symlink_to(outside, target_is_directory=True)
        result = subprocess.run(["sh", str(control)], env=environment, capture_output=True, timeout=5)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(marker.exists())
        self.assertEqual(outside.stat().st_mode & 0o777, 0o700)

    def test_released_hooks_do_not_stop_service_or_claim_task_removal(self):
        commands = self.root / "commands"
        commands.mkdir()
        marker = self.root / "service-control-called"
        service = commands / "synosystemctl"
        service.write_text('#!/bin/sh\nprintf called > "$SERVICE_CONTROL_MARKER"\nexit 7\n')
        service.chmod(0o755)
        for action in ("preuninst", "postuninst", "preupgrade", "start-stop-status"):
            _, control, environment, _ = self.lifecycle_fixture(action)
            environment.update(PATH=str(commands) + ":" + os.environ["PATH"], SERVICE_CONTROL_MARKER=str(marker))
            arguments = ["sh", str(control)]
            if action == "start-stop-status":
                arguments.append("stop")
            result = subprocess.run(arguments, env=environment, capture_output=True, timeout=5)
            if action == "start-stop-status":
                self.assertEqual(result.returncode, 7)
                self.assertTrue(marker.exists())
            else:
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertFalse(marker.exists())

    def test_reproducibility_receipt_and_bad_input_preserve_previous_artifact(self):
        binary = self.binary()
        destination = BUILD.build("amd64", binary, self.root)
        previous = destination.read_bytes()
        BUILD.build("amd64", binary, self.root)
        self.assertEqual(destination.read_bytes(), previous)
        receipt = json.loads(destination.with_suffix(".json").read_text())
        self.assertEqual(receipt["sha256"], hashlib.sha256(previous).hexdigest())
        self.assertEqual(receipt["nas_acceptance"], "pending")
        self.assertFalse(receipt["terminal_support"])
        with self.assertRaises(ValueError):
            BUILD.build("arm64", binary, self.root)
        (self.ui / "config").write_text("[]")
        with self.assertRaises(ValueError):
            BUILD.build("amd64", binary, self.root)
        self.assertEqual(destination.read_bytes(), previous)


if __name__ == "__main__":
    unittest.main()
