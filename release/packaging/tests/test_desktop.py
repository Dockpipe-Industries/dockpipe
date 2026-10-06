"""Test installed CLI wrappers and installer ownership without modifying the host."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


DESKTOP = Path(__file__).resolve().parents[1] / "desktop"


class DesktopTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="dockpipe desktop ")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)

    def test_app_wrapper_resolves_symlinks_spaces_arguments_and_explicit_override(self):
        contents = self.root / "Applications/DockPipe.app/Contents"
        (contents / "MacOS").mkdir(parents=True)
        (contents / "Helpers").mkdir(parents=True)
        wrapper = contents / "MacOS/dockpipe-cli"
        shutil.copyfile(DESKTOP / "macos/dockpipe", wrapper)
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

    def test_mac_installer_preserves_foreign_commands_and_links(self):
        command = self.root / "usr/local/bin/dockpipe"
        command.parent.mkdir(parents=True)
        script = ["sh", str(DESKTOP / "macos/scripts/preinstall"), "package.pkg", "/", str(self.root)]
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
