"""Test installed CLI wrappers and installer ownership without modifying the host."""
import json
import importlib.util
from unittest.mock import patch
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest




spec = importlib.util.spec_from_file_location("desktop_smoke", Path(__file__).resolve().parents[4] / 'workflows/build/desktop/assets/scripts/smoke.py')
smoke = importlib.util.module_from_spec(spec)
spec.loader.exec_module(smoke)


class DesktopTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="dockpipe desktop ")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)

    def smoke_container_calls(self, architecture):
        tools = self.root / "tools"
        tools.mkdir()
        log = self.root / "docker-calls.jsonl"
        uname = tools / "uname"
        uname.write_text("#!/bin/sh\ncase \"$1\" in -m) echo " + architecture + ";; -s) echo Linux;; *) exit 1;; esac\n")
        uname.chmod(0o755)
        docker = tools / "docker"
        docker.write_text(
            "#!" + sys.executable + "\nimport json, os, sys\n"
            "with open(os.environ['DOCKPIPE_SMOKE_CALL_LOG'], 'a') as stream:\n"
            "    stream.write(json.dumps(sys.argv[1:]) + '\\n')\n")
        docker.chmod(0o755)
        artifacts = self.root / "artifacts"
        artifacts.mkdir()
        environment = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ["PATH"],
                           DOCKPIPE_SMOKE_CALL_LOG=str(log), TMPDIR=str(self.root))
        result = subprocess.run(["bash", str(Path(__file__).resolve().parents[4] / 'workflows/build/desktop/assets/scripts/smoke-linux.sh'), "0.6.31", str(artifacts)],
                                env=environment, capture_output=True, text=True, timeout=15)
        calls = [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []
        return result, calls

    def test_linux_smoke_amd64_runs_all_declared_deb_consumers(self):
        result, calls = self.smoke_container_calls("x86_64")
        self.assertEqual(result.returncode, 0, result.stderr)
        images = [argument for call in calls for argument in call
                  if argument.startswith(("ubuntu:", "debian:"))]
        self.assertEqual(images, ["ubuntu:22.04", "ubuntu:24.04", "debian:12", "debian:13"])
        self.assertEqual(len(calls), 5)
        self.assertIn("dockpipe-desktop-builder:ubuntu22.04-qt6.10.3", calls[0])
        self.assertIn("gcc_64", calls[0])

    def test_linux_smoke_arm64_preserves_ubuntu24_baseline_only(self):
        result, calls = self.smoke_container_calls("aarch64")
        self.assertEqual(result.returncode, 0, result.stderr)
        images = [argument for call in calls for argument in call
                  if argument.startswith(("ubuntu:", "debian:"))]
        self.assertEqual(images, ["ubuntu:24.04"])
        self.assertEqual(len(calls), 2)
        self.assertIn("dockpipe-desktop-builder:ubuntu24.04-qt6.10.3", calls[0])
        self.assertIn("gcc_arm64", calls[0])

    def test_linux_smoke_rejects_unknown_architecture_before_docker(self):
        result, calls = self.smoke_container_calls("riscv64")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(calls, [])

    def test_app_wrapper_resolves_symlinks_spaces_arguments_and_explicit_override(self):
        contents = self.root / "Applications/DockPipe.app/Contents"
        (contents / "MacOS").mkdir(parents=True)
        (contents / "Helpers").mkdir(parents=True)
        wrapper = contents / "MacOS/dockpipe-cli"
        shutil.copyfile(Path(__file__).resolve().parents[4] / 'workflows/package/macos/assets/dockpipe', wrapper)
        wrapper.chmod(0o755)
        binary = contents / "Helpers/dockpipe"
        binary.write_text(
            '#!/usr/bin/env python3\nimport json, os, sys\n'
            'print(json.dumps([os.environ["DOCKPIPE_SYSTEM_ROOT"], sys.argv[1:]]))\n'
        )
        binary.chmod(0o755)
        link = self.root / "command"
        link.symlink_to(wrapper)
        relative = self.root / "relative-command"
        relative.symlink_to("command")
        environment = dict(os.environ)
        environment.pop("DOCKPIPE_SYSTEM_ROOT", None)
        for command in (wrapper, link, relative):
            result = subprocess.check_output([str(command), "argument with spaces", "--version"], env=environment)
            self.assertEqual(json.loads(result), [str(contents / "Resources/share/dockpipe"),
                                                 ["argument with spaces", "--version"]])
        result = subprocess.check_output([str(link)], env=dict(environment, DOCKPIPE_SYSTEM_ROOT="/custom store"))
        self.assertEqual(json.loads(result), ["/custom store", []])

    def test_inventory_checks_explicit_installer_payload_without_changing_launcher_environment(self):
        environment = {"DOCKPIPE_GLOBAL_ROOT": str(self.root / "isolated-user")}
        payload = self.root / "extracted-install/usr/share/dockpipe"
        standalone = {"packages": [], "warnings": []}
        with patch.object(smoke.subprocess, "check_output", return_value=json.dumps(standalone)) as command:
            smoke.check_package_inventory("cli", self.root, environment, payload)
            selected_environment = command.call_args.kwargs["env"]
            self.assertEqual(selected_environment["DOCKPIPE_SYSTEM_ROOT"], str(payload.resolve()))
            self.assertEqual(selected_environment["DOCKPIPE_GLOBAL_ROOT"], environment["DOCKPIPE_GLOBAL_ROOT"])
        self.assertNotIn("DOCKPIPE_SYSTEM_ROOT", environment)
        for inventory in ({"packages": [{"kind": "core"}], "warnings": []},
                          {"packages": [{"kind": "core"}, {"kind": "workflow"}], "warnings": []},
                          {"packages": [{"kind": "core"}], "warnings": ["unreadable archive"]}):
            with patch.object(smoke.subprocess, "check_output", return_value=json.dumps(inventory)):
                with self.assertRaises(RuntimeError):
                    smoke.check_package_inventory("cli", self.root, environment, payload)
        with patch.object(smoke.subprocess, "check_output", return_value=json.dumps(standalone)) as command:
            smoke.check_package_inventory("mac-wrapper", self.root, environment)
            self.assertNotIn("DOCKPIPE_SYSTEM_ROOT", command.call_args.kwargs["env"])

    def test_mac_installer_preserves_foreign_commands_and_links(self):
        command = self.root / "usr/local/bin/dockpipe"
        command.parent.mkdir(parents=True)
        script = ["sh", str(Path(__file__).resolve().parents[4] / 'workflows/package/macos/assets/scripts/preinstall'), "package.pkg", "/", str(self.root)]
        subprocess.run(script, check=True, capture_output=True)
        command.write_text("user command")
        result = subprocess.run(script, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(command.read_text(), "user command")
        command.unlink()
        command.symlink_to("/missing/foreign-command")
        self.assertNotEqual(subprocess.run(script, capture_output=True).returncode, 0)
        self.assertEqual(os.readlink(command), "/missing/foreign-command")
        command.unlink()
        command.symlink_to("/Applications/DockPipe.app/Contents/MacOS/dockpipe-cli")
        subprocess.run(script, check=True, capture_output=True)
        homebrew = self.root / "opt/homebrew/bin/dockpipe"
        homebrew.parent.mkdir(parents=True)
        homebrew.symlink_to("/missing/homebrew-cellar")
        self.assertNotEqual(subprocess.run(script, capture_output=True).returncode, 0)
        self.assertEqual(os.readlink(homebrew), "/missing/homebrew-cellar")


if __name__ == "__main__":
    unittest.main()
