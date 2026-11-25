"""Exercise deployed Qt and the launcher's CLI lookup outside a source checkout."""
import argparse
import base64
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import time


def check_managed_terminal(cli, home, environment, timeout=30):
    """Require real shell I/O and verified closure through the installed CLI."""
    cli = Path(cli).resolve(strict=True)
    home = Path(home).resolve(strict=True)
    if not 0 < timeout <= 30:
        raise ValueError("Managed terminal smoke timeout must be at most 30 seconds")
    selected = {"HOME": str(home), "USERPROFILE": str(home), "TMPDIR": str(home),
                "TMP": str(home), "TEMP": str(home), "TERM": "xterm-256color",
                "COLORTERM": "truecolor", "DOCKPIPE_GLOBAL_ROOT": str(home / "global"),
                "XDG_CONFIG_HOME": str(home / "config"), "XDG_DATA_HOME": str(home / "data"),
                "XDG_CACHE_HOME": str(home / "cache"), "XDG_STATE_HOME": str(home / "state")}
    if os.name == "nt":
        windows = Path(environment.get("SYSTEMROOT") or environment["SystemRoot"]).resolve(strict=True)
        shell = windows / "System32" / "cmd.exe"
        selected.update(SystemRoot=str(windows), WINDIR=str(windows),
                        PATH=os.pathsep.join([str(windows / "System32"), str(windows)]),
                        APPDATA=str(home / "roaming"), LOCALAPPDATA=str(home / "local"),
                        DOCKPIPE_SMOKE_MARKER="TERMINAL_SMOKE")
        arguments = ["/D", "/Q"]
        command = b"echo __DOCKPIPE_DESKTOP_%DOCKPIPE_SMOKE_MARKER%__\r\n"
    else:
        shell = Path("/bin/sh").resolve(strict=True)
        selected.update(PATH="/usr/bin:/bin:/usr/sbin:/sbin", PS1="")
        arguments = ["-i"]
        command = b"printf '__DOCKPIPE_DESKTOP_%s__\\n' TERMINAL_SMOKE\n"
    marker = b"__DOCKPIPE_DESKTOP_TERMINAL_SMOKE__"
    profile = {"id": "installed-desktop-smoke", "executable": str(shell), "args": arguments,
               "env": [f"{key}={value}" for key, value in sorted(selected.items())],
               "dir": str(home), "route": "native", "environment_mode": "complete"}
    schema = "dockpipe.local-terminal/v1"
    deadline = time.monotonic() + timeout
    cleanup_budget = min(5, timeout / 2)
    process = subprocess.Popen([str(cli), "terminal", "--stdio"], cwd=home, env=dict(selected),
                               stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                               stderr=subprocess.DEVNULL, bufsize=0)
    failures = []

    def exchange():
        def send(kind, identity, **fields):
            frame = json.dumps(dict(schema=schema, type=kind, id=identity, **fields)).encode() + b"\n"
            if process.stdin.write(frame) != len(frame):
                raise RuntimeError("Installed managed terminal did not accept its private command")

        try:
            send("start", 1, profile=profile, columns=80, rows=24)
            ready = False
            acknowledged = False
            observed = False
            resized = False
            closing = False
            tail = b""
            total = 0
            for _ in range(512):
                line = process.stdout.readline(96 * 1024 + 1)
                total += len(line)
                if not line or len(line) > 96 * 1024 or total > 512 * 1024:
                    raise RuntimeError("Installed managed terminal output ended or exceeded its smoke budget")
                event = json.loads(line)
                if not isinstance(event, dict) or event.get("schema") != schema:
                    raise RuntimeError("Installed managed terminal returned an invalid event")
                kind = event.get("type")
                if kind == "ready" and not ready and event.get("id") == 1 and event.get("session_id"):
                    ready = True
                    send("input", 2, data=base64.b64encode(command).decode("ascii"))
                elif kind == "output" and ready:
                    data = base64.b64decode(event.get("data", ""), validate=True)
                    if not data or len(data) > 32 * 1024 or event.get("truncated", False):
                        raise RuntimeError("Installed managed terminal output was incomplete")
                    tail = tail + data
                    observed = observed or marker in tail
                    tail = tail[-len(marker):]
                elif kind == "ack" and ready and event.get("id") == 2 and not acknowledged:
                    if event.get("bytes") != len(command):
                        raise RuntimeError("Installed managed terminal input acknowledgement changed")
                    acknowledged = True
                elif kind == "ack" and resized and event.get("id") == 3 and not closing:
                    send("close", 4)
                    closing = True
                elif kind == "closed":
                    if event.get("cleanup") != "complete":
                        raise RuntimeError("Installed managed terminal cleanup could not be verified")
                    if not closing or not observed or not acknowledged:
                        raise RuntimeError("Installed managed terminal closed before real shell I/O completed")
                    return
                else:
                    raise RuntimeError("Installed managed terminal failed its protocol smoke check")
                if observed and acknowledged and not resized:
                    send("resize", 3, columns=113, rows=37)
                    resized = True
            raise RuntimeError("Installed managed terminal exceeded its event budget")
        except Exception as error:
            # Frames contain private launch state and input. Never include them,
            # child output or serializer exception text in smoke diagnostics.
            if isinstance(error, RuntimeError):
                failures.append(error)
            else:
                failures.append(RuntimeError("Installed managed terminal protocol exchange failed"))

    worker = threading.Thread(target=exchange, name="desktop-terminal-smoke", daemon=True)
    worker.start()
    problem = None
    try:
        worker.join(max(0, deadline - time.monotonic() - cleanup_budget))
        if worker.is_alive():
            problem = "Installed managed terminal smoke timed out; cleanup is unverified"
        elif failures:
            problem = str(failures[0])
        else:
            try:
                process.wait(timeout=max(0, deadline - time.monotonic() - cleanup_budget))
                if process.returncode:
                    problem = "Installed managed terminal CLI exited unsuccessfully"
            except subprocess.TimeoutExpired:
                problem = "Installed managed terminal CLI did not exit after verified session closure"
    finally:
        if process.poll() is None:
            try:
                process.terminate()
            except OSError:
                if process.poll() is None:
                    problem = "Installed managed terminal process could not be stopped"
            try:
                process.wait(timeout=max(0, (deadline - time.monotonic()) / 2))
            except subprocess.TimeoutExpired:
                try:
                    process.kill()
                except OSError:
                    if process.poll() is None:
                        problem = "Installed managed terminal process cleanup could not be verified"
                try:
                    process.wait(timeout=max(0, deadline - time.monotonic()))
                except subprocess.TimeoutExpired:
                    problem = "Installed managed terminal process cleanup could not be verified"
        worker.join(max(0, deadline - time.monotonic()))
        if worker.is_alive():
            problem = "Installed managed terminal I/O cleanup could not be verified"
        else:
            process.stdin.close()
            process.stdout.close()
    if problem:
        raise RuntimeError(problem)
    print("PASS: installed CLI managed terminal input/output, resize and cleanup")


def check_package_inventory(cli, home, environment, system_root=None):
    inventory_environment = dict(environment)
    if system_root is not None:
        inventory_environment["DOCKPIPE_SYSTEM_ROOT"] = str(Path(system_root).resolve())
    inventory = json.loads(subprocess.check_output(
        [str(cli), "package", "list", "--format", "json", "--workdir", str(home)],
        cwd=home, env=inventory_environment, text=True, timeout=45))
    if inventory["warnings"] or inventory["packages"]:
        raise RuntimeError(f"Standalone installation must contain no package payloads: {inventory}")
    print("PASS: standalone installation has an empty optional package inventory")


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
        reader = launcher.parent / ("dockpipe-workspace-reader.exe" if os.name == "nt" else "dockpipe-workspace-reader")
        if not reader.is_file():
            raise RuntimeError("Installed workspace reader is missing beside the launcher")
        terminal_directory = launcher.parent.parent / "Helpers" if sys.platform == "darwin" else launcher.parent
        terminal_host = terminal_directory / ("dockpipe-terminal-host.exe" if os.name == "nt" else "dockpipe-terminal-host")
        if not terminal_host.is_file():
            raise RuntimeError("Installed terminal host is missing from the desktop installation")
        # Invalid arguments exercise dependency loading without opening a shell.
        terminal_probe = subprocess.run([str(terminal_host)], cwd=home, env=environment,
                                        capture_output=True, timeout=15)
        if terminal_probe.returncode != 64:
            raise RuntimeError("Installed terminal host failed its no-session startup probe")
        page = subprocess.run([str(reader)], input=json.dumps({"root": temporary, "directory": temporary, "limit": 16}),
                              cwd=home, env=environment, check=True, text=True, stdout=subprocess.PIPE, timeout=30)
        response = json.loads(page.stdout)
        if "error" in response or not isinstance(response.get("entries"), list):
            raise RuntimeError("Installed workspace reader could not read its isolated fixture")
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
        check_managed_terminal(selected_cli, home, environment)
        # Inventory the explicit extracted installer path while keeping the
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
