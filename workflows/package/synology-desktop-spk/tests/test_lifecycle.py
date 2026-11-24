"""Exercise DSM lifecycle scripts with private state and bounded local probes."""

import hashlib
import json
import os
from pathlib import Path
import secrets
import shlex
import shutil
import socket
import stat
import subprocess
import sys
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
        self.etc = self.root / "package etc"
        self.target = self.root / "package target"
        self.home.mkdir(mode=0o700)
        self.data.mkdir(mode=0o700)
        self.etc.mkdir(mode=0o700)
        (self.target / "bin").mkdir(parents=True)
        (self.target / "ui/images").mkdir(parents=True)
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
            "SYNOPKG_PKGETC": str(self.etc),
            "SYNOPKG_PKGDEST": str(self.target),
            "SYNOPKG_PKGINST_TEMP_DIR": str(self.root / "installer staging"),
            "FIXTURE_EVENTS": str(self.root / "events"),
            "wizard_username": "Desktop.operator-1",
            "wizard_password": self.password,
            "wizard_password_confirm": self.password,
        }
        self.fake_tools = self.root / "probe tools"
        self.fake_tools.mkdir()
        self.write_tool("curl", "#!" + sys.executable + "\n" + '''import os
from pathlib import Path
import sys
arguments = sys.argv[1:]
native = '38143' in arguments[-1]
if '%{remote_ip}' in arguments:
    key = 'FIXTURE_NATIVE_REMOTE_IP' if native else 'FIXTURE_REMOTE_IP'
    print(os.environ.get(key, ''), end='')
else:
    with open(os.environ['FIXTURE_EVENTS'], 'a') as events:
        events.write('native\\n' if native else 'stream\\n')
    if native:
        assert '--insecure' not in arguments
        assert '--cacert' in arguments and '--resolve' in arguments
        assert arguments[-1] == 'https://localhost:38143/health'
        print(os.environ.get('FIXTURE_NATIVE_HTTP_CODE', '204'), end='')
    else:
        print(os.environ.get('FIXTURE_HTTP_CODE', '401'), end='')
sys.exit(int(os.environ.get('FIXTURE_CURL_EXIT', '0')))
''')
        self.write_tool("synosystemctl", "#!" + sys.executable + "\n" + '''import os
from pathlib import Path
import socket
import sys
assert sys.argv[2] == 'pkguser-dockpipe-desktop-native-host.service'
with open(os.environ['FIXTURE_EVENTS'], 'a') as events:
    events.write(sys.argv[1] + '\\n')
failure = int(os.environ.get('FIXTURE_SERVICE_EXIT', '0'))
if failure:
    sys.exit(failure)
volume = os.environ.get('SYNOPKG_PKGDEST_VOL')
data = Path(volume) / '@appdata/dockpipe-desktop' if volume else Path(os.environ['SYNOPKG_PKGVAR'])
etc = Path(volume) / '@appconf/dockpipe-desktop' if volume else Path(os.environ['SYNOPKG_PKGETC'])
for path in (etc / 'native-host/native-worker.sock', data / 'native-client/client.sock'):
    if sys.argv[1] == 'start' and not os.environ.get('FIXTURE_SKIP_SOCKETS'):
        if not path.exists():
            with socket.socket(socket.AF_UNIX) as current:
                current.bind(str(path))
    elif sys.argv[1] == 'stop' and path.exists():
        path.unlink()
''')
        native = self.target / "bin/dockpipe-dsm-host"
        native.write_text("#!" + sys.executable + "\n" + '''import os
from pathlib import Path
import sys
if sys.argv[1:] in (['folder-prepare'], ['folder-save']):
    if os.environ.get('FIXTURE_FOLDER_FAILURE'):
        sys.exit(1)
    sys.exit(0)
assert sys.argv[1:] == ['endpoint-init']
if os.environ.get('FIXTURE_INIT_FAILURE'):
    sys.exit(1)
volume = os.environ.get('SYNOPKG_PKGDEST_VOL')
data = Path(volume) / '@appdata/dockpipe-desktop' if volume else Path(os.environ['SYNOPKG_PKGVAR'])
etc = Path(volume) / '@appconf/dockpipe-desktop' if volume else Path(os.environ['SYNOPKG_PKGETC'])
endpoint = etc / 'native-host'
client = data / 'native-client'
for path, mode in ((endpoint / 'endpoint-private', 0o700), (endpoint / 'endpoint-public', 0o755), (client, 0o700)):
    path.mkdir(exist_ok=True)
    path.chmod(mode)
for path, content, mode in (
    (endpoint / 'endpoint-private/server-key.pem', 'fixture TLS key', 0o600),
    (endpoint / 'endpoint-public/ca.pem', 'fixture public CA', 0o644),
    (client / 'client.key', 'fixture client key', 0o600),
    (client / 'connection.json', 'fixture client reference', 0o600),
):
    if not path.exists():
        path.write_text(content)
        path.chmod(mode)
''')
        native.chmod(0o755)
        staged_native = Path(self.environment["SYNOPKG_PKGINST_TEMP_DIR"]) / "package/bin/dockpipe-dsm-host"
        staged_native.parent.mkdir(parents=True)
        shutil.copy2(native, staged_native)
        staged_desktop = staged_native.parent.parent / "desktop"
        staged_desktop.mkdir()
        (staged_desktop / "compose.base.yml").write_text("services: {}\n")
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
        for base in (self.home, self.data, self.etc):
            for path in (base, *sorted(base.rglob("*"))):
                metadata = path.lstat()
                checksum = None
                if path.is_file() and not path.is_symlink():
                    checksum = hashlib.sha256(path.read_bytes()).hexdigest()
                records.append((str(path.relative_to(self.root)), stat.S_IMODE(metadata.st_mode), checksum))
        return records

    def make_native_sockets(self):
        for path in (self.etc / "native-host/native-worker.sock", self.data / "native-client/client.sock"):
            with socket.socket(socket.AF_UNIX) as current:
                current.bind(str(path))

    def install(self):
        if os.getuid() == 0 or os.getgid() == 0:
            self.skipTest("Successful installation requires a real unprivileged package identity")
        result = self.run_action("postinst")
        self.assertEqual(result.returncode, 0, "Initial package setup failed")

    def test_preinst_accepts_valid_values_without_writing_package_state(self):
        before = self.snapshot()
        self.assertEqual(self.run_action("preinst").returncode, 0)
        self.assertEqual(self.snapshot(), before)

    def test_preinst_requires_staged_helper_and_propagates_folder_preflight_failure(self):
        for overrides in ({"SYNOPKG_PKGINST_TEMP_DIR": ""}, {"FIXTURE_FOLDER_FAILURE": "1"}):
            with self.subTest(overrides=overrides):
                before = self.snapshot()
                self.assertNotEqual(self.run_action("preinst", overrides=overrides).returncode, 0)
                self.assertEqual(self.snapshot(), before)

    def test_preinst_accepts_payload_root_as_the_dsm_temporary_directory(self):
        payload = Path(self.environment["SYNOPKG_PKGINST_TEMP_DIR"]) / "package"
        result = self.run_action("preinst", overrides={"SYNOPKG_PKGINST_TEMP_DIR": str(payload)})
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_preinst_rejects_ambiguous_payloads_and_reports_only_candidate_checks(self):
        temporary = Path(self.environment["SYNOPKG_PKGINST_TEMP_DIR"])
        (temporary / "bin").mkdir()
        shutil.copy2(self.target / "bin/dockpipe-dsm-host", temporary / "bin/dockpipe-dsm-host")
        (temporary / "desktop").mkdir()
        (temporary / "desktop/compose.base.yml").write_text("services: {}\n")
        result = self.run_action("preinst")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("matching payloads: 2", result.stderr)
        self.assertIn("helper_regular=yes", result.stderr)
        self.assertIn(str(temporary), result.stderr)

    def test_preinst_rejects_missing_and_linked_payload_helpers_with_bounded_diagnostics(self):
        temporary = Path(self.environment["SYNOPKG_PKGINST_TEMP_DIR"])
        helper = temporary / "package/bin/dockpipe-dsm-host"
        helper.unlink()
        for linked in (False, True):
            with self.subTest(linked=linked):
                if linked:
                    helper.symlink_to(self.target / "bin/dockpipe-dsm-host")
                result = self.run_action("preinst")
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("matching payloads: 0", result.stderr)
                self.assertIn("helper_regular=no", result.stderr)
                self.assertIn(str(temporary / "package"), result.stderr)

    def test_preinst_does_not_print_control_characters_from_invalid_staging_paths(self):
        for value in ("/tmp/first\nsecond", "/tmp/first\rsecond", "/tmp/first\x1bsecond"):
            with self.subTest(value=repr(value)):
                result = self.run_action("preinst", overrides={"SYNOPKG_PKGINST_TEMP_DIR": value})
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("invalid path characters", result.stderr)
                self.assertNotIn(value, result.stderr)

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
        self.assertEqual(stat.S_IMODE(self.data.stat().st_mode), 0o700)
        self.assertEqual(stat.S_IMODE(self.etc.stat().st_mode), 0o755)
        endpoint = self.etc / "native-host"
        self.assertEqual(stat.S_IMODE(endpoint.stat().st_mode), 0o755)
        for directory in (endpoint / "endpoint-private", self.data / "native-client"):
            self.assertEqual(stat.S_IMODE(directory.stat().st_mode), 0o700)
        for private in (endpoint / "endpoint-private/server-key.pem", self.data / "native-client/client.key"):
            self.assertEqual(stat.S_IMODE(private.stat().st_mode), 0o600)

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

    def test_upgrade_finds_persistent_settings_after_package_link_removal(self):
        self.install()
        volume = self.root / "DSM volume"
        persistent_home = volume / "@apphome/dockpipe-desktop"
        persistent_home.parent.mkdir(parents=True)
        self.home.rename(persistent_home)
        persistent_data = volume / "@appdata/dockpipe-desktop"
        persistent_data.parent.mkdir(parents=True)
        self.data.rename(persistent_data)
        persistent_etc = volume / "@appconf/dockpipe-desktop"
        persistent_etc.parent.mkdir(parents=True)
        self.etc.rename(persistent_etc)
        before = {p.name: p.read_bytes() for p in (persistent_home / "desktop-settings").iterdir()}
        environment = {
            "SYNOPKG_PKGDEST_VOL": str(volume), "SYNOPKG_PKG_STATUS": "UPGRADE",
            "wizard_username": "", "wizard_password": "", "wizard_password_confirm": "",
        }
        for action in ("preinst", "postinst", "postupgrade"):
            with self.subTest(action=action):
                result = self.run_action(action, overrides=environment)
                self.assertEqual(result.returncode, 0, result.stderr)
        after = {p.name: p.read_bytes() for p in (persistent_home / "desktop-settings").iterdir()}
        self.assertEqual(after, before)
        self.assertFalse(self.home.exists())

    def test_upgrade_missing_saved_state_does_not_request_install_wizard_credentials(self):
        result = self.run_action("preinst", overrides={
            "SYNOPKG_PKG_STATUS": "UPGRADE", "wizard_username": "",
            "wizard_password": "", "wizard_password_confirm": "",
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Saved Dockpipe settings are unavailable during upgrade", result.stderr)
        self.assertNotIn("installation wizard", result.stderr)

    def retained_restore_environment(self):
        self.install()
        volume = self.root / "selected DSM volume"
        old_home, old_data, old_etc = self.home, self.data, self.etc
        for source, storage in ((old_home, "@apphome"), (old_data, "@appdata"), (old_etc, "@appconf")):
            destination = volume / storage / "dockpipe-desktop"
            destination.parent.mkdir(parents=True)
            source.rename(destination)
        self.home = volume / "@apphome/dockpipe-desktop"
        self.data = volume / "@appdata/dockpipe-desktop"
        self.etc = volume / "@appconf/dockpipe-desktop"
        self.settings = self.home / "desktop-settings"
        return {
            "SYNOPKG_PKGDEST_VOL": str(volume), "SYNOPKG_PKG_STATUS": "INSTALL",
            "SYNOPKG_PKGHOME": str(old_home), "SYNOPKG_PKGVAR": str(old_data), "SYNOPKG_PKGETC": str(old_etc),
            "wizard_restore_volume": str(volume), "wizard_restore_intent": "restore",
            "wizard_username": "", "wizard_password": "", "wizard_password_confirm": "",
        }

    def test_explicit_restore_selection_preserves_the_complete_selected_dataset(self):
        environment = self.retained_restore_environment()
        before = self.snapshot()
        for intent in ("restore", "upgrade"):
            with self.subTest(intent=intent):
                result = self.run_action("preinst", overrides={**environment, "wizard_restore_intent": intent})
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(self.snapshot(), before)

    def test_restore_selection_rejects_a_different_native_volume_before_preparation(self):
        environment = self.retained_restore_environment()
        before = self.snapshot()
        result = self.run_action("preinst", overrides={**environment, "wizard_restore_volume": "/volume99"})
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("volume", result.stderr.lower())
        self.assertEqual(self.snapshot(), before)

    def test_repair_uses_native_upgrade_volume_when_browser_has_no_selector(self):
        environment = self.retained_restore_environment()
        before = self.snapshot()
        result = self.run_action("preinst", overrides={
            **environment, "SYNOPKG_PKG_STATUS": "UPGRADE",
            "wizard_restore_intent": "upgrade", "wizard_restore_volume": "",
        })
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.snapshot(), before)

    def test_missing_browser_volume_cannot_retarget_fresh_or_restore_install(self):
        environment = self.retained_restore_environment()
        before = self.snapshot()
        for intent, status in (("upgrade", "INSTALL"), ("restore", "UPGRADE"), ("fresh", "UPGRADE")):
            with self.subTest(intent=intent, status=status):
                result = self.run_action("preinst", overrides={
                    **environment, "SYNOPKG_PKG_STATUS": status,
                    "wizard_restore_intent": intent, "wizard_restore_volume": "",
                })
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(self.snapshot(), before)

    def test_explicit_fresh_selection_cannot_ignore_retained_settings(self):
        environment = self.retained_restore_environment()
        before = self.snapshot()
        result = self.run_action("preinst", overrides={**environment, "wizard_restore_intent": "fresh"})
        self.assertNotEqual(result.returncode, 0)
        self.assertRegex(result.stderr.lower(), "saved|retained|restore|already")
        self.assertEqual(self.snapshot(), before)

    def test_explicit_restore_cannot_fall_back_to_new_credentials(self):
        volume = self.root / "empty selected volume"
        volume.mkdir()
        result = self.run_action("preinst", overrides={
            "SYNOPKG_PKGDEST_VOL": str(volume), "wizard_restore_volume": str(volume),
            "wizard_restore_intent": "restore",
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((volume / "@apphome/dockpipe-desktop/desktop-settings").exists())

    def test_explicit_fresh_selection_accepts_a_confirmed_empty_selected_volume(self):
        volume = self.root / "empty selected volume"
        volume.mkdir()
        result = self.run_action("preinst", overrides={
            "SYNOPKG_PKGDEST_VOL": str(volume), "wizard_restore_volume": str(volume),
            "wizard_restore_intent": "fresh",
        })
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse((volume / "@apphome/dockpipe-desktop/desktop-settings").exists())

    def test_public_install_marker_is_published_only_after_successful_initialization(self):
        marker = self.etc / "install-state"
        failed = self.run_action("postinst", overrides={"FIXTURE_INIT_FAILURE": "1"})
        self.assertNotEqual(failed.returncode, 0)
        self.assertFalse(marker.exists())
        completed = self.run_action("postinst")
        self.assertEqual(completed.returncode, 0, completed.stderr)
        self.assertEqual(marker.read_bytes(), b"dockpipe.saved-settings.v1\n")
        self.assertEqual(stat.S_IMODE(marker.stat().st_mode), 0o644)
        self.assertEqual(marker.stat().st_uid, os.geteuid())

    def test_reinstall_refreshes_stale_package_identity_without_changing_sign_in(self):
        self.install()
        password = (self.settings / "password").read_bytes()
        username = (self.settings / "username").read_bytes()
        (self.settings / "uid").write_text("54321")
        (self.settings / "gid").write_text("54322")
        result = self.run_action("postinst")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.settings / "uid").read_text(), str(os.getuid()))
        self.assertEqual((self.settings / "gid").read_text(), str(os.getgid()))
        self.assertEqual((self.settings / "password").read_bytes(), password)
        self.assertEqual((self.settings / "username").read_bytes(), username)
        self.assertFalse(list(self.home.glob(".desktop-identity.*")))

    def prepare_uninstall_storage(self):
        self.install()
        volume = self.root / "DSM volume"
        home = volume / "@apphome/dockpipe-desktop"
        data = volume / "@appdata/dockpipe-desktop"
        etc = volume / "@appconf/dockpipe-desktop"
        home.parent.mkdir(parents=True)
        data.parent.mkdir(parents=True)
        etc.parent.mkdir(parents=True)
        self.home.rename(home)
        self.data.rename(data)
        self.etc.rename(etc)
        self.home, self.data, self.etc = home, data, etc
        self.settings = home / "desktop-settings"
        self.environment["SYNOPKG_PKGDEST_VOL"] = str(volume)
        (data / "workspace").mkdir()
        (data / "workspace/result").write_text("retained project")

    def test_uninstall_keep_and_upgrade_delete_choice_preserve_all_data(self):
        self.prepare_uninstall_storage()
        before = self.snapshot()
        for status, choice in [("UNINSTALL", "keep"), ("UPGRADE", "delete"), ("", "delete")]:
            result = self.run_action("postuninst", overrides={
                "SYNOPKG_PKG_STATUS": status, "wizard_uninstall_data": choice,
            })
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(self.snapshot(), before)
        self.assertEqual(self.run_action("postuninst", overrides={"SYNOPKG_PKG_STATUS": "UNINSTALL"}).returncode, 0)
        self.assertEqual(self.snapshot(), before)

    def test_explicit_full_uninstall_removes_only_owned_contents_and_keeps_external_symlink_target(self):
        self.prepare_uninstall_storage()
        external = self.root / "unrelated workspace"
        external.mkdir()
        (external / "keep").write_text("unrelated")
        (self.data / "external link").symlink_to(external, target_is_directory=True)
        for attempt in range(2):
            with self.subTest(attempt=attempt):
                result = self.run_action("postuninst", overrides={
                    "SYNOPKG_PKG_STATUS": "UNINSTALL", "wizard_uninstall_data": "delete",
                })
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(list(self.home.iterdir()), [])
                self.assertEqual(list(self.data.iterdir()), [])
                self.assertEqual(list(self.etc.iterdir()), [])
                self.assertEqual((external / "keep").read_text(), "unrelated")

    def test_full_uninstall_retains_data_if_service_port_is_still_active(self):
        self.prepare_uninstall_storage()
        before = self.snapshot()
        result = self.run_action("postuninst", overrides={
            "SYNOPKG_PKG_STATUS": "UNINSTALL", "wizard_uninstall_data": "delete",
            "FIXTURE_REMOTE_IP": "127.0.0.1",
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("still active", result.stderr)
        self.assertEqual(self.snapshot(), before)

    def test_keep_data_uninstall_also_reports_a_port_left_active(self):
        self.prepare_uninstall_storage()
        before = self.snapshot()
        result = self.run_action("postuninst", overrides={
            "SYNOPKG_PKG_STATUS": "UNINSTALL", "wizard_uninstall_data": "keep",
            "FIXTURE_REMOTE_IP": "127.0.0.1",
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("still active", result.stderr)
        self.assertEqual(self.snapshot(), before)

    def test_uninstall_preserves_all_roots_when_native_gateway_is_still_active(self):
        self.prepare_uninstall_storage()
        before = self.snapshot()
        result = self.run_action("postuninst", overrides={
            "SYNOPKG_PKG_STATUS": "UNINSTALL", "wizard_uninstall_data": "delete",
            "FIXTURE_NATIVE_REMOTE_IP": "127.0.0.1",
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("38143 is still active", result.stderr)
        self.assertEqual(self.snapshot(), before)

    def test_full_uninstall_rejects_symbolic_link_root_before_any_data_removal(self):
        self.prepare_uninstall_storage()
        saved = self.data.with_name("retained data")
        self.data.rename(saved)
        self.data.symlink_to(saved, target_is_directory=True)
        credential = (self.settings / "password").read_bytes()
        result = self.run_action("postuninst", overrides={
            "SYNOPKG_PKG_STATUS": "UNINSTALL", "wizard_uninstall_data": "delete",
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.settings / "password").read_bytes(), credential)
        self.assertTrue((saved / "workspace/result").exists())

    def test_full_uninstall_rejects_symbolic_link_configuration_before_any_data_removal(self):
        self.prepare_uninstall_storage()
        saved = self.etc.with_name("retained configuration")
        self.etc.rename(saved)
        self.etc.symlink_to(saved, target_is_directory=True)
        credential = (self.settings / "password").read_bytes()
        result = self.run_action("postuninst", overrides={
            "SYNOPKG_PKG_STATUS": "UNINSTALL", "wizard_uninstall_data": "delete",
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.settings / "password").read_bytes(), credential)
        self.assertTrue((saved / "native-host/endpoint-private/server-key.pem").exists())

    def test_full_uninstall_checks_find_support_before_framework_removal(self):
        self.prepare_uninstall_storage()
        before = self.snapshot()
        self.write_tool("find", "#!/bin/sh\nexit 1\n")
        result = self.run_action("preuninst", overrides={
            "SYNOPKG_PKG_STATUS": "UNINSTALL", "wizard_uninstall_data": "delete",
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("does not support", result.stderr)
        self.assertEqual(self.snapshot(), before)

    def test_invalid_uninstall_choice_is_rejected_before_framework_removal(self):
        result = self.run_action("preuninst", overrides={
            "SYNOPKG_PKG_STATUS": "UNINSTALL", "wizard_uninstall_data": "unexpected",
        })
        self.assertNotEqual(result.returncode, 0)

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
        self.install()
        self.make_native_sockets()
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

    def test_status_requires_pinned_native_health_and_both_real_sockets(self):
        self.install()
        self.make_native_sockets()
        self.assertEqual(self.run_action("start-stop-status", "status").returncode, 0)
        for code in ("200", "401", "503"):
            with self.subTest(code=code):
                result = self.run_action("start-stop-status", "status", {"FIXTURE_NATIVE_HTTP_CODE": code})
                self.assertEqual(result.returncode, 3)
        worker = self.etc / "native-host/native-worker.sock"
        worker.unlink()
        worker.write_text("not a socket")
        self.assertEqual(self.run_action("start-stop-status", "status").returncode, 3)
        worker.unlink()
        worker.symlink_to(self.data / "native-client/client.sock")
        self.assertEqual(self.run_action("start-stop-status", "status").returncode, 3)

    def test_owned_stop_propagates_failure_and_released_hooks_do_not_stop_twice(self):
        self.install()
        events = Path(self.environment["FIXTURE_EVENTS"])
        self.assertNotEqual(self.run_action("start-stop-status", "stop", {"FIXTURE_SERVICE_EXIT": "1"}).returncode, 0)
        self.assertEqual(events.read_text().splitlines(), ["stop"])
        events.unlink()
        for action, argument in (("start-stop-status", "prestop"), ("preuninst", None),
                                 ("preupgrade", None), ("postupgrade", None), ("postuninst", None)):
            self.assertEqual(self.run_action(action, argument).returncode, 0)
        self.assertFalse(events.exists())

    def test_endpoint_initialization_failure_is_not_reported_as_success(self):
        result = self.run_action("postinst", overrides={"FIXTURE_INIT_FAILURE": "1"})
        self.assertNotEqual(result.returncode, 0)

    def test_start_propagates_native_unit_failure_after_stream_readiness(self):
        self.install()
        result = self.run_action("start-stop-status", "start", {"FIXTURE_SERVICE_EXIT": "7"})
        self.assertEqual(result.returncode, 7)
        events = Path(self.environment["FIXTURE_EVENTS"]).read_text().splitlines()
        self.assertEqual(events, ["stream", "start"])

    def test_native_readiness_deadline_expires_without_sleeping_or_false_success(self):
        self.install()
        clock = self.root / "clock state"
        self.write_tool("date", "#!" + sys.executable + "\n" + '''import os
from pathlib import Path
path = Path(os.environ['FIXTURE_CLOCK_STATE'])
count = int(path.read_text()) if path.exists() else 0
path.write_text(str(count + 1))
print([100, 100, 131][min(count, 2)])
''')
        marker = self.root / "sleep invoked"
        self.write_tool("sleep", '#!/bin/sh\n: > "$FIXTURE_SLEEP_MARKER"\nexit 99\n')
        result = self.run_action("start-stop-status", "start", {
            "FIXTURE_SKIP_SOCKETS": "1", "FIXTURE_CLOCK_STATE": str(clock),
            "FIXTURE_SLEEP_MARKER": str(marker),
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("native host service did not become ready", result.stderr)
        self.assertFalse(marker.exists())

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
        events = Path(self.environment["FIXTURE_EVENTS"]).read_text().splitlines()
        self.assertEqual(events, ["stream", "start", "native"])
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
        self.filesystem = self.root / "filesystem"
        (self.filesystem / "volume1").mkdir(parents=True)
        self.output = self.root / "wizard output.json"
        self.script = self.root / "install_uifile.sh"
        template = (ASSETS / "install_uifile.sh").read_text()
        helper = (ASSETS / "scripts/restore-discovery.sh").read_text()
        folder_step = json.loads((ASSETS / "upgrade_uifile").read_text())[0]
        folder_step["custom_render_fn"] = (ASSETS / "host-folders-wizard.js").read_text()
        pages = json.loads((ASSETS / "install_uifile").read_text())
        pages.append(folder_step)
        rendered = template.replace("@RESTORE_DISCOVERY_HELPERS@", helper)
        rendered = rendered.replace("@INSTALL_WIZARD@", json.dumps(pages))
        rendered = rendered.replace("@HOST_FOLDER_WIZARD@", json.dumps(folder_step))
        # The discovery function takes a filesystem root. Production calls /;
        # this unit fixture supplies an isolated tree, without an env backdoor.
        rendered = rendered.replace("render_install_wizard /", "render_install_wizard " + shlex.quote(str(self.filesystem)))
        self.script.write_text(rendered)
        self.script.chmod(0o700)

    def render(self, overrides=None):
        result = subprocess.run([str(self.script)], env={
            **os.environ, "SYNOPKG_PKGHOME": "", "SYNOPKG_PKGDEST_VOL": "",
            "SYNOPKG_TEMP_LOGFILE": str(self.output), **(overrides or {}),
        }, capture_output=True, text=True, timeout=5)
        self.assertEqual(result.returncode, 0, result.stderr)
        return json.loads(self.output.read_text())

    def retained_home(self, volume="volume1", opaque=False, marker=None):
        home = self.filesystem / volume / "@apphome/dockpipe-desktop"
        settings = home / "desktop-settings"
        settings.mkdir(parents=True)
        (settings / "password").write_text("private-password-must-never-be-rendered")
        (settings / "password").chmod(0o000)
        if marker:
            configuration = self.filesystem / volume / "@appconf/dockpipe-desktop"
            if marker == "legacy":
                artifact = configuration / "native-host/endpoint-public/ca.pem"
                content = "PUBLIC CERTIFICATE FIXTURE"
            else:
                artifact = configuration / "install-state"
                content = "dockpipe.saved-settings.v1\n"
            artifact.parent.mkdir(parents=True)
            artifact.write_text(content)
            artifact.chmod(0o644)
        if opaque:
            home.chmod(0o000)
            self.addCleanup(home.chmod, 0o700)
        return home

    def assert_state(self, wizard, mode, retained="", checked="/volume1", unknown=""):
        source = wizard[-1]["custom_render_fn"]
        self.assertIn('var restoreMode = "' + mode + '";', source)
        self.assertIn('var retainedVolumes = volumeList("' + retained + '");', source)
        self.assertIn('var checkedVolumes = volumeList("' + checked + '");', source)
        self.assertIn('var unknownVolumes = volumeList("' + unknown + '");', source)
        self.assertNotIn("@DOCKPIPE_", source)
        self.assertNotIn("private-password-must-never-be-rendered", json.dumps(wizard))

    def test_initial_wizard_contains_masked_credentials_and_dsm_regex_validators(self):
        wizard = self.render()
        fields = {}
        for page in wizard:
            for item in page.get("items", []):
                for field in item.get("subitems", []):
                    fields[field["key"]] = (item, field)
        self.assertEqual(set(fields), {"wizard_username", "wizard_password", "wizard_password_confirm"})
        self.assert_state(wizard, "fresh")
        for key in ("wizard_password", "wizard_password_confirm"):
            item, field = fields[key]
            self.assertEqual(item["type"], "password")
            self.assertFalse(field.get("defaultValue"))
            self.assertFalse(field["validator"]["allowBlank"])
        for key in ("wizard_username", "wizard_password"):
            expression = fields[key][1]["validator"]["regex"]["expr"]
            self.assertTrue(expression.startswith("/^") and expression.endswith("$/"))

    def test_both_hints_absent_and_missing_package_links_still_restore_saved_data(self):
        self.retained_home()
        wizard = self.render()
        self.assertFalse((self.filesystem / "var/packages/dockpipe-desktop").exists())
        self.assertEqual(wizard[0]["step_title"], "Restore Dockpipe")
        self.assertNotIn("wizard_password", json.dumps(wizard))
        self.assertNotIn("wizard_username", json.dumps(wizard))
        self.assert_state(wizard, "restore", retained="/volume1")

    def test_valid_volume_and_home_hints_prefer_the_same_explicit_dataset(self):
        self.retained_home("volume1")
        self.retained_home("volume2")
        wizard = self.render({"SYNOPKG_PKGDEST_VOL": "/volume2", "SYNOPKG_PKGHOME": "/volume2/@apphome/dockpipe-desktop"})
        self.assert_state(wizard, "restore", retained="/volume2", checked="/volume2")

    def test_multiple_retained_volumes_are_reported_without_choosing_the_first(self):
        self.retained_home("volume1")
        self.retained_home("volume2")
        wizard = self.render()
        self.assert_state(wizard, "restore", retained="/volume1,/volume2", checked="/volume1,/volume2")
        self.assertNotIn("wizard_password", json.dumps(wizard))

    @unittest.skipIf(os.geteuid() == 0, "metadata visibility fixture requires an ordinary process")
    def test_public_legacy_marker_restores_even_when_private_home_is_opaque(self):
        self.retained_home(opaque=True, marker="legacy")
        wizard = self.render()
        self.assert_state(wizard, "restore", retained="/volume1")
        self.assertNotIn("wizard_password", json.dumps(wizard))

    @unittest.skipIf(os.geteuid() == 0, "metadata visibility fixture requires an ordinary process")
    def test_public_install_marker_restores_without_reading_private_settings(self):
        self.retained_home(opaque=True, marker="explicit")
        self.assert_state(self.render(), "restore", retained="/volume1")

    @unittest.skipIf(os.geteuid() == 0, "metadata visibility fixture requires an ordinary process")
    def test_opaque_home_without_a_positive_marker_is_unknown_not_fresh(self):
        self.retained_home(opaque=True)
        wizard = self.render()
        self.assert_state(wizard, "unknown", unknown="/volume1")
        self.assertNotIn("wizard_password", json.dumps(wizard))

    def test_conflicting_valid_hints_do_not_guess_a_dataset(self):
        self.retained_home("volume1")
        self.retained_home("volume2")
        wizard = self.render({"SYNOPKG_PKGDEST_VOL": "/volume2", "SYNOPKG_PKGHOME": "/volume1/@apphome/dockpipe-desktop"})
        self.assert_state(wizard, "unknown", checked="")
        self.assertNotIn("wizard_password", json.dumps(wizard))

    def test_inventory_exhaustion_does_not_render_fresh_credentials(self):
        for number in range(2, 66):
            (self.filesystem / ("volume" + str(number))).mkdir()
        wizard = self.render()
        self.assertIn('var restoreMode = "unknown";', wizard[-1]["custom_render_fn"])
        self.assertNotIn("wizard_password", json.dumps(wizard))

    def test_linked_home_is_not_treated_as_fresh_or_a_verified_restore(self):
        outside = self.root / "outside home"
        outside.mkdir()
        parent = self.filesystem / "volume1/@apphome"
        parent.mkdir()
        (parent / "dockpipe-desktop").symlink_to(outside)
        self.assert_state(self.render(), "unknown", unknown="/volume1")

    @unittest.skipIf(os.geteuid() == 0, "metadata visibility fixture requires an ordinary process")
    def test_malformed_or_writable_public_marker_does_not_claim_saved_settings(self):
        self.retained_home(opaque=True, marker="explicit")
        marker = self.filesystem / "volume1/@appconf/dockpipe-desktop/install-state"
        for contents, mode in (("dockpipe.saved-settings.v9\n", 0o644), ("dockpipe.saved-settings.v1\nextra", 0o644), ("dockpipe.saved-settings.v1\n", 0o666)):
            with self.subTest(contents=contents, mode=oct(mode)):
                marker.write_text(contents)
                marker.chmod(mode)
                self.assert_state(self.render(), "unknown", unknown="/volume1")

    def test_canonical_home_hint_does_not_require_access_to_private_contents(self):
        self.retained_home("volume1", opaque=os.geteuid() != 0, marker="legacy")
        self.retained_home("volume2")
        wizard = self.render({"SYNOPKG_PKGHOME": "/volume1/@apphome/dockpipe-desktop"})
        self.assert_state(wizard, "restore", retained="/volume1", checked="/volume1")

    def test_control_characters_in_volume_hints_cannot_enter_runtime_slots(self):
        wizard = self.render({"SYNOPKG_PKGDEST_VOL": "/volume1\nnot-a-volume"})
        self.assert_state(wizard, "fresh")
        self.assertNotIn("not-a-volume", json.dumps(wizard))


if __name__ == "__main__":
    unittest.main()
