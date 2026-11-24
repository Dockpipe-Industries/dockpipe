"""Exercise the installed Pop!_OS Terminal on an owned X11 display."""

import argparse
from pathlib import Path
import subprocess
import sys

from PIL import ImageGrab

from debian_installation import CLI, LAUNCHER, admit
from desktop import OwnedDesktop, digest
import ui

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "assets/scripts"))
from contracts import read_json, require, validate_receipt, write_json


ASSERTIONS = {"selected-shell-started", "input-output-roundtrip", "owned-shell-tree-closed"}
EXPECTED_OUTPUT = "dp-terminal-ROUNDTRIP-7F92"
COMMAND = ("printf '%s\\n' \"$BASHPID\" > shell.pid; "
           "/usr/bin/sleep 35 & printf '%s\\n' \"$!\" > child.pid; "
           "printf '\\033[2J\\033[H%s%s\\n' dp-terminal- ROUNDTRIP-7F92; wait")
CLIPBOARD_READER = """import gi
gi.require_version('Gdk', '3.0')
gi.require_version('Gtk', '3.0')
from gi.repository import Gdk, Gtk
text = Gtk.Clipboard.get(Gdk.SELECTION_CLIPBOARD).wait_for_text()
print(text or '', end='')
"""


def exercise(output, xvfb):
    home = output / "home"
    home.mkdir(mode=0o700)

    def environment_setup(environment):
        environment.update(HOME=str(home), DOCKPIPE_BIN=str(CLI), PATH="/usr/bin:/bin",
                           DOCKPIPE_GLOBAL_ROOT=str(output / "global"), DOCKPIPE_SYSTEM_ROOT=str(output / "system"))

    desktop = OwnedDesktop(output / "desktop", {
        "xvfb": str(xvfb), "gui": str(LAUNCHER), "prepare_environment": environment_setup,
    })

    def event(*arguments):
        return subprocess.check_output(["/usr/bin/xdotool", *arguments], env=environment, timeout=5, text=True)

    def match(name, role):
        candidates = [node for node, _ in ui.nodes(atspi.Registry, gui.pid)
                      if node.name == name and node.getRoleName() == role
                      and node.getState().contains(atspi.STATE_SHOWING)]
        return candidates[0] if len(candidates) == 1 else None

    def bounds(name, role):
        node = ui.wait_for("Expected visible terminal control: " + name, lambda: match(name, role))
        area = node.queryComponent().getExtents(atspi.DESKTOP_COORDS)
        require(area.width > 0 and area.height > 0, "The selected Terminal control has no usable bounds")
        return area

    def click(name, role):
        area = bounds(name, role)
        event("mousemove", str(area.x + area.width // 2), str(area.y + area.height // 2), "click", "1")

    def capture(name):
        path = output / (name + ".png")
        ImageGrab.grab(xdisplay=environment["DISPLAY"]).save(path)
        return path

    def process_reference(path):
        pid = int(path.read_text())
        started = desktop.process_starttime(pid)
        require(started is not None, "The recorded Terminal process was not live")
        executable = Path("/proc", str(pid), "exe").resolve(strict=True)
        status = Path("/proc", str(pid), "status").read_text().splitlines()
        parent, = [int(line.split()[1]) for line in status if line.startswith("PPid:")]
        return {"pid": pid, "start_time": started, "parent": parent, "executable": str(executable)}

    def no_longer_live(reference):
        return desktop.process_starttime(reference["pid"]) != reference["start_time"]

    try:
        gui = desktop.start()
        atspi, session = ui.connect(desktop.output / "session-private.json")
        environment = session["environment"]
        workspace = Path(session["workspace"])
        ui.wait_for("Expected installed Dockpipe frame", lambda: match("Dockpipe", "frame"))
        require(Path("/proc", str(gui.pid), "exe").resolve(strict=True) == LAUNCHER.resolve(),
                "Unexpected running launcher executable")
        windows = event("search", "--onlyvisible", "--pid", str(gui.pid)).splitlines()
        require(len(windows) == 1, "Expected one visible launcher window")
        event("windowfocus", windows[0])
        click("View", "menu item")
        click("Terminal", "menu item")
        click("Default shell", "combo box")
        click("bash", "list item")
        click("New terminal", "push button")
        click("DockPipe Terminal", "document web")
        event("type", "--clearmodifiers", "--delay", "15", COMMAND)
        event("key", "Return")

        shell_path = workspace / "shell.pid"
        child_path = workspace / "child.pid"
        ui.wait_for("Terminal input did not reach the Bash shell",
                    lambda: all(path.is_file() and path.read_text().strip().isdigit() for path in (shell_path, child_path)))
        shell = process_reference(shell_path)
        child = process_reference(child_path)
        require(shell["executable"] == "/usr/bin/bash" and child["executable"] == "/usr/bin/sleep"
                and child["parent"] == shell["pid"] and shell["parent"] == gui.pid,
                "Unexpected Terminal process tree")
        require(Path("/proc", str(shell["pid"]), "cwd").resolve(strict=True) == workspace,
                "Bash did not run in the generated workspace")

        area = bounds("DockPipe Terminal", "document web")
        require(area.width > 325 and area.height > 14, "Terminal document is too small to select its first output line")
        first_row = str(area.y + 7)

        def copy_output():
            event("mousemove", str(area.x + 3), first_row, "mousedown", "1",
                  "mousemove", str(area.x + 320), first_row, "mouseup", "1")
            event("key", "ctrl+shift+c")
            text = subprocess.check_output(["/usr/bin/python3", "-c", CLIPBOARD_READER],
                                           env=environment, timeout=5, text=True)
            selected = text.strip()
            return selected if selected == EXPECTED_OUTPUT else None

        selected = ui.wait_for("Private Terminal selection did not contain the exact rendered output", copy_output, 4)
        rendered = output / "rendered-output.txt"
        rendered.write_text(selected + "\n")
        screenshot = capture("roundtrip")
        require(not no_longer_live(shell) and not no_longer_live(child), "Fixture exited before Terminal Close")
        click("Close", "push button")
        ui.wait_for("Terminal Close left an original process alive",
                    lambda: no_longer_live(shell) and no_longer_live(child), 4)
        observations = output / "process-closure.json"
        boot_id = Path("/proc/sys/kernel/random/boot_id").read_text().strip()
        write_json(observations, {"launcher_pid": gui.pid, "shell": shell, "child": child, "boot_id": boot_id,
                                  "both_live_before_close": True, "both_closed_after_close": True})
        closed = capture("closed")
        click("File", "menu item")
        click("Quit", "menu item")
        require(gui.wait(timeout=12) == 0, "Installed launcher did not quit normally")
    finally:
        desktop.stop()
    display = environment["DISPLAY"].removeprefix(":")
    require(all(process.poll() is not None for process in desktop.processes)
            and not Path(f"/tmp/.X11-unix/X{display}").exists(), "Owned Terminal display remained live")
    return [rendered, screenshot, closed, observations, shell_path, child_path]


def run(request, output):
    case, = request["cases"]
    require((case["id"], case["scenario"], case["layer"]) == ("terminal.local", "happy", "installed_e2e")
            and set(case["required_assertions"]) == ASSERTIONS, "Unexpected installed Terminal case")
    xvfb, identity_path, binaries = admit(request, output)
    evidence = exercise(output, xvfb)
    require(all(digest(Path(name)) == expected for name, expected in binaries.items()),
            "Installed files changed during the Terminal journey")
    reason = "Selected Bash returned exact rendered input/output and its original process tree closed"
    row = {key: case[key] for key in ("id", "scenario", "layer")}
    row.update(status="passed", reason=reason,
               assertions=[{"id": name, "status": "passed", "detail": reason} for name in case["required_assertions"]],
               evidence=[{"path": path.relative_to(output).as_posix(), "sha256": digest(path)}
                         for path in [identity_path, *evidence]])
    receipt = {key: request[key] for key in ("run_id", "plan_sha256", "registry_sha256", "target")}
    receipt.update(schema="dockpipe.product-integration.lane/v1", cases=[row])
    validate_receipt(receipt, request, output)
    write_json(output / "result.json", receipt)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--request", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    arguments = parser.parse_args()
    require(arguments.output.is_absolute() and not arguments.output.exists(), "Select fresh absolute output")
    request = read_json(arguments.request)
    require(request.get("schema") == "dockpipe.product-integration.request/v1", "Unknown qualification request schema")
    run(request, arguments.output)


if __name__ == "__main__":
    main()
