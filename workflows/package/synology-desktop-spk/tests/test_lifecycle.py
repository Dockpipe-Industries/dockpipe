"""Exercise DSM lifecycle scripts with private state and bounded local probes."""

import hashlib
import json
import os
from pathlib import Path
import secrets
import shutil
import stat
import subprocess
import tempfile
import unittest


ASSETS = Path(__file__).resolve().parents[1] / "assets"
CONTROL_ACTIONS = (
    "preinst", "postinst", "preupgrade", "postupgrade", "preuninst", "postuninst", "start-stop-status",
)


class PackageLifecycleTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="dockpipe lifecycle spaces ")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.home = self.root / "package home"
        self.data = self.root / "package var"
        self.home.mkdir(mode=0o700)
        self.data.mkdir(mode=0o700)
        self.settings = self.home / "desktop-settings"
        self.scripts = self.root / "package scripts"
        self.scripts.mkdir()
        for action in CONTROL_ACTIONS:
            destination = self.scripts / action
            shutil.copyfile(ASSETS / "scripts/package-control.sh", destination)
            destination.chmod(0o700)
        self.password = secrets.token_urlsafe(24) + "'\"$`\\; #"
        self.environment = {
            **os.environ,
            "SYNOPKG_PKGHOME": str(self.home),
            "SYNOPKG_PKGVAR": str(self.data),
            "wizard_username": "Desktop.operator-1",
            "wizard_password": self.password,
            "wizard_password_confirm": self.password,
        }
        self.fake_tools = self.root / "probe tools"
        self.fake_tools.mkdir()
        self.write_tool("curl", '#!/bin/sh\nprintf "%s" "${FIXTURE_HTTP_CODE:-401}"\nexit "${FIXTURE_CURL_EXIT:-0}"\n')
        self.environment["PATH"] = str(self.fake_tools) + os.pathsep + os.environ["PATH"]

    def write_tool(self, name, content):
        destination = self.fake_tools / name
        destination.write_text(content)
        destination.chmod(0o700)

    def run_action(self, action, argument=None, overrides=None):
        environment = {**self.environment, **(overrides or {})}
        command = [str(self.scripts / action)]
        if argument is not None:
            command.append(argument)
        result = subprocess.run(command, env=environment, capture_output=True, text=True, timeout=5)
        diagnostic = result.stdout + result.stderr
        for key in ("wizard_password", "wizard_password_confirm"):
            value = environment.get(key, "")
            if value:
                self.assertFalse(value in diagnostic, "Lifecycle script exposed a supplied credential")
        return result

    def snapshot(self):
        records = []
        for base in (self.home, self.data):
            for path in (base, *sorted(base.rglob("*"))):
                metadata = path.lstat()
                checksum = None
                if path.is_file() and not path.is_symlink():
                    checksum = hashlib.sha256(path.read_bytes()).hexdigest()
                records.append((str(path.relative_to(self.root)), stat.S_IMODE(metadata.st_mode), checksum))
        return records

    def install(self):
        if os.getuid() == 0 or os.getgid() == 0:
            self.skipTest("Successful installation requires a real unprivileged package identity")
        result = self.run_action("postinst")
        self.assertEqual(result.returncode, 0, "Initial package setup failed")

    def test_preinst_accepts_valid_values_without_writing_package_state(self):
        before = self.snapshot()
        self.assertEqual(self.run_action("preinst").returncode, 0)
        self.assertEqual(self.snapshot(), before)

    def test_preinst_rejects_invalid_usernames_without_side_effects(self):
        usernames = {
            "empty": "", "numeric prefix": "1operator", "too long": "a" * 33,
            "space": "desktop operator", "internal newline": "operator\nsecond",
            "trailing newline": "operator\n", "shell metacharacter": "operator;touch injected",
            "command substitution": "$(touch injected)", "unicode": "opérator",
        }
        before = self.snapshot()
        for label, username in usernames.items():
            with self.subTest(case=label):
                self.assertNotEqual(self.run_action("preinst", overrides={"wizard_username": username}).returncode, 0)
                self.assertEqual(self.snapshot(), before)
        self.assertFalse((self.root / "injected").exists())

    def test_preinst_enforces_password_length_ascii_and_line_boundaries(self):
        before = self.snapshot()
        for length, expected_success in ((15, False), (16, True), (128, True), (129, False)):
            password = secrets.token_hex((length + 1) // 2)[:length]
            with self.subTest(length=length):
                result = self.run_action("preinst", overrides={
                    "wizard_password": password, "wizard_password_confirm": password,
                })
                self.assertEqual(result.returncode == 0, expected_success)
        for label, password in (
            ("newline", self.password + "\n"), ("internal newline", self.password + "\n" + secrets.token_hex(4)),
            ("carriage return", self.password + "\r"), ("tab", self.password + "\t"),
            ("unicode", self.password + "é"), ("empty", ""),
        ):
            with self.subTest(case=label):
                result = self.run_action("preinst", overrides={
                    "wizard_password": password, "wizard_password_confirm": password,
                })
                self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.snapshot(), before)

    def test_preinst_requires_matching_confirmation(self):
        result = self.run_action("preinst", overrides={"wizard_password_confirm": secrets.token_urlsafe(32)})
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(self.settings.exists())

    def test_postinst_creates_private_complete_settings_for_actual_package_identity(self):
        self.install()
        self.assertEqual(stat.S_IMODE(self.settings.stat().st_mode), 0o700)
        self.assertEqual({path.name for path in self.settings.iterdir()}, {"username", "password", "uid", "gid"})
        for path in self.settings.iterdir():
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
            self.assertFalse(path.is_symlink())
        self.assertTrue((self.settings / "username").read_text() == self.environment["wizard_username"])
        self.assertTrue((self.settings / "password").read_text() == self.password, "Stored password differs from wizard input")
        self.assertEqual(int((self.settings / "uid").read_text()), os.getuid())
        self.assertEqual(int((self.settings / "gid").read_text()), os.getgid())
        self.assertNotEqual(os.getuid(), 0)
        self.assertNotEqual(os.getgid(), 0)
        self.assertFalse(list(self.home.glob(".desktop-settings.*")))

    def test_reinstall_and_upgrade_preserve_credentials_and_workspace(self):
        self.install()
        (self.data / "project evidence").write_bytes(secrets.token_bytes(32))
        before = self.snapshot()
        replacement = secrets.token_urlsafe(32)
        for action in ("preinst", "postinst", "preupgrade", "postupgrade"):
            with self.subTest(action=action):
                result = self.run_action(action, overrides={
                    "wizard_username": "Replacement", "wizard_password": replacement,
                    "wizard_password_confirm": replacement,
                })
                self.assertEqual(result.returncode, 0)
                self.assertEqual(self.snapshot(), before)

    def test_symlinked_settings_directory_and_files_fail_closed(self):
        self.install()
        saved = self.home / "saved settings"
        self.settings.rename(saved)
        self.settings.symlink_to(saved, target_is_directory=True)
        for action, argument in (("preinst", None), ("postinst", None), ("start-stop-status", "prestart")):
            with self.subTest(kind="directory", action=action):
                self.assertNotEqual(self.run_action(action, argument).returncode, 0)
        self.settings.unlink()
        saved.rename(self.settings)
        for name in ("username", "password", "uid", "gid"):
            path = self.settings / name
            saved_file = self.home / ("saved " + name)
            path.rename(saved_file)
            path.symlink_to(saved_file)
            with self.subTest(kind="file", name=name):
                self.assertNotEqual(self.run_action("preinst").returncode, 0)
                self.assertNotEqual(self.run_action("start-stop-status", "prestart").returncode, 0)
            path.unlink()
            saved_file.rename(path)

    def test_stop_and_uninstall_leave_settings_and_user_data_intact(self):
        self.install()
        (self.data / "workspace").mkdir()
        (self.data / "workspace/project").write_bytes(secrets.token_bytes(32))
        before = self.snapshot()
        for action, argument in (
            ("start-stop-status", "prestop"), ("start-stop-status", "stop"),
            ("preuninst", None), ("postuninst", None),
        ):
            with self.subTest(action=action, argument=argument):
                self.assertEqual(self.run_action(action, argument).returncode, 0)
                self.assertEqual(self.snapshot(), before)

    def test_status_accepts_only_an_available_authentication_gate(self):
        for label, status, curl_exit, expected in (
            ("authentication required", "401", "0", 0), ("unauthenticated content", "200", "0", 3),
            ("server unavailable", "503", "0", 3), ("connection failure", "000", "7", 3),
            ("probe timeout", "000", "28", 3),
        ):
            with self.subTest(case=label):
                result = self.run_action("start-stop-status", "status", {
                    "FIXTURE_HTTP_CODE": status, "FIXTURE_CURL_EXIT": curl_exit,
                })
                self.assertEqual(result.returncode, expected)

    def test_prestart_rejects_missing_invalid_and_root_identities(self):
        self.assertNotEqual(self.run_action("start-stop-status", "prestart").returncode, 0)
        self.install()
        self.assertEqual(self.run_action("start-stop-status", "prestart").returncode, 0)
        for name in ("uid", "gid"):
            path = self.settings / name
            original = path.read_bytes()
            for value in (b"0", b"not-numeric", b""):
                path.write_bytes(value)
                with self.subTest(identity=name, kind="invalid"):
                    self.assertNotEqual(self.run_action("start-stop-status", "prestart").returncode, 0)
            path.write_bytes(original)

    def test_start_succeeds_at_auth_gate_and_exhausts_deadline_without_real_wait(self):
        self.install()
        self.assertEqual(self.run_action("start-stop-status", "start").returncode, 0)
        clock_state = self.root / "clock state"
        self.write_tool("date", '''#!/bin/sh
if [ -f "$FIXTURE_CLOCK_STATE" ]; then
    printf '221\n'
else
    : > "$FIXTURE_CLOCK_STATE"
    printf '100\n'
fi
''')
        sleep_marker = self.root / "sleep invoked"
        self.write_tool("sleep", '#!/bin/sh\n: > "$FIXTURE_SLEEP_MARKER"\nexit 99\n')
        result = self.run_action("start-stop-status", "start", {
            "FIXTURE_HTTP_CODE": "200", "FIXTURE_CLOCK_STATE": str(clock_state),
            "FIXTURE_SLEEP_MARKER": str(sleep_marker),
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("did not become ready", result.stderr)
        self.assertFalse(sleep_marker.exists(), "Startup slept after its deadline expired")


class InstallWizardTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="dockpipe wizard spaces ")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.home = self.root / "package home"
        self.home.mkdir(mode=0o700)
        self.output = self.root / "wizard output.json"
        self.script = self.root / "install_uifile.sh"
        template = (ASSETS / "install_uifile.sh").read_text()
        wizard = (ASSETS / "install_uifile").read_text()
        self.script.write_text(template.replace("@INSTALL_WIZARD@", wizard))
        self.script.chmod(0o700)

    def render(self):
        result = subprocess.run([str(self.script)], env={
            **os.environ, "SYNOPKG_PKGHOME": str(self.home), "SYNOPKG_TEMP_LOGFILE": str(self.output),
        }, capture_output=True, text=True, timeout=5)
        self.assertEqual(result.returncode, 0)
        return json.loads(self.output.read_text())

    def test_initial_wizard_contains_masked_credentials_and_dsm_regex_validators(self):
        wizard = self.render()
        fields = {}
        for page in wizard:
            for item in page.get("items", []):
                for field in item.get("subitems", []):
                    fields[field["key"]] = (item, field)
        self.assertEqual(set(fields), {"wizard_username", "wizard_password", "wizard_password_confirm"})
        for key in ("wizard_password", "wizard_password_confirm"):
            item, field = fields[key]
            self.assertEqual(item["type"], "password")
            self.assertFalse(field.get("defaultValue"))
            self.assertFalse(field["validator"]["allowBlank"])
        for key in ("wizard_username", "wizard_password"):
            expression = fields[key][1]["validator"]["regex"]["expr"]
            self.assertTrue(expression.startswith("/^") and expression.endswith("$/"))

    def test_existing_settings_render_restore_message_without_credential_fields(self):
        (self.home / "desktop-settings").mkdir(mode=0o700)
        wizard = self.render()
        encoded = json.dumps(wizard)
        self.assertIn("retained", encoded)
        self.assertNotIn("wizard_password", encoded)
        self.assertNotIn("wizard_username", encoded)
        self.assertTrue(all("subitems" not in item for page in wizard for item in page.get("items", [])))


if __name__ == "__main__":
    unittest.main()
