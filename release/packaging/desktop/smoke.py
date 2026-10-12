"""Exercise deployed Qt and the launcher's CLI lookup outside a source checkout."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile


def check_package_inventory(cli, home, environment, system_root=None):
    inventory_environment = dict(environment)
    if system_root is not None:
        inventory_environment["DOCKPIPE_SYSTEM_ROOT"] = str(Path(system_root).resolve())
    inventory = json.loads(subprocess.check_output(
        [str(cli), "package", "list", "--format", "json", "--workdir", str(home)],
        cwd=home, env=inventory_environment, text=True, timeout=45))
    if inventory["warnings"] or not inventory["packages"]:
        raise RuntimeError(f"Incomplete installer package inventory: {inventory}")
    if any(package["kind"] != "core" for package in inventory["packages"]):
        raise RuntimeError(f"Installer bundled optional packages: {inventory}")
    print("PASS: clean installation contains core only")


def check(launcher, expected_cli, inventory_system_root=None):
    launcher = Path(launcher).resolve()
    with tempfile.TemporaryDirectory(prefix="dockpipe-desktop-smoke-") as temporary:
        home = Path(temporary)
        environment = dict(os.environ, HOME=temporary, XDG_CONFIG_HOME=str(home / "config"),
                           XDG_DATA_HOME=str(home / "data"), XDG_CACHE_HOME=str(home / "cache"),
                           XDG_STATE_HOME=str(home / "state"), DOCKPIPE_GLOBAL_ROOT=str(home / "global"),
                           QT_DEBUG_PLUGINS="1",
                           QT_QPA_PLATFORM={"darwin": "cocoa", "win32": "windows"}.get(sys.platform, "offscreen"))
        if os.name == "nt":
            environment.update(APPDATA=str(home / "roaming"), LOCALAPPDATA=str(home / "local"))
            windows = Path(os.environ["SystemRoot"])
            environment["PATH"] = os.pathsep.join([str(windows / "System32"), str(windows)])
        else:
            environment["PATH"] = "/usr/bin:/bin:/usr/sbin:/sbin"
        for name in ("DOCKPIPE_BIN", "DOCKPIPE_SYSTEM_ROOT", "DOCKPIPE_REPO_ROOT", "QT_PLUGIN_PATH",
                     "QML2_IMPORT_PATH", "QT_QPA_PLATFORM_PLUGIN_PATH", "DYLD_FRAMEWORK_PATH",
                     "DYLD_LIBRARY_PATH", "LD_LIBRARY_PATH"):
            environment.pop(name, None)
        try:
            result = subprocess.run([str(launcher), "--check-installation"], cwd=home, env=environment,
                                    capture_output=True, text=True, timeout=45)
        except subprocess.TimeoutExpired as error:
            diagnostics = error.stderr or b""
            if isinstance(diagnostics, bytes):
                diagnostics = diagnostics.decode(errors="replace")
            raise RuntimeError("Desktop installation diagnostic timed out:\n" + diagnostics) from error
        selected_cli = next((line.removeprefix("CLI: ") for line in result.stdout.splitlines()
                             if line.startswith("CLI: ")), "")
        # Qt prints forward slashes on Windows; compare filesystem paths rather
        # than their platform-specific display spelling.
        if result.returncode or not selected_cli or Path(selected_cli).resolve() != Path(expected_cli).resolve():
            raise RuntimeError(result.stdout + result.stderr)
        launcher_version = subprocess.check_output(
            [str(launcher), "--version"], cwd=home, env=environment, text=True, timeout=30).strip()
        cli_version = subprocess.check_output(
            [selected_cli, "--version"], cwd=home, env=environment, text=True, timeout=30).strip()
        if launcher_version != cli_version:
            raise RuntimeError(f"Launcher/CLI version mismatch: {launcher_version} != {cli_version}")
        print(result.stdout)
        # Linux smoke uses an extracted DEB; MSI stores core under the original
        # user's install directory. Inventory those payloads while keeping the
        # launcher's isolated environment and default CLI lookup unchanged.
        check_package_inventory(selected_cli, home, environment, inventory_system_root)
        with (home / "launcher.log").open("w+") as log:
            process = subprocess.Popen([str(launcher), "--allow-second-instance"], cwd=home,
                                       env=environment, stdout=log, stderr=log)
            try:
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    print("PASS: deployed Qt window remains running")
                else:
                    log.seek(0)
                    raise RuntimeError(f"Launcher exited early ({process.returncode}): {log.read()}")
            finally:
                process.terminate()
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("launcher")
    parser.add_argument("expected_cli")
    parser.add_argument("--inventory-system-root", type=Path)
    args = parser.parse_args()
    check(args.launcher, args.expected_cli, args.inventory_system_root)
