"""Qualify installed desktop startup and a GUI-launched host workflow on Pop!_OS."""

import argparse
import json
from pathlib import Path
import subprocess
import sys

from PIL import ImageGrab

from debian_installation import CLI, LAUNCHER, admit
from desktop import OwnedDesktop, digest
import ui

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "assets/scripts"))
from contracts import read_json, require, validate_receipt, write_json


ASSERTIONS = {
    "install.desktop": {"installed-cli-version", "installed-gui-identity", "normal-gui-started", "settings-retained"},
    "workflow.host": {"exit-status-propagated", "fixture-output-exact", "cwd-env-output-binding"},
}
WORKFLOW = "installed-host-proof"
MARKER = "dockpipe-installed-gui-host-924C"
CONFIGURATION = """name: installed-host-proof
vault: none
docker_preflight: false
steps:
  - id: proof
    kind: host
    cwd: repo
    vars:
      DOCKPIPE_PROOF_VALUE: dockpipe-installed-gui-host-924C
    run: proof.sh
"""
SCRIPT = """set -eu
test -f dockpipe.config.json
test "$DOCKPIPE_PROOF_VALUE" = dockpipe-installed-gui-host-924C
printf '%s\\n' "$PWD" > observed-cwd.txt
for attempt in $(seq 1 150); do
    if test -f release.txt; then
        printf '%s\\n' "$DOCKPIPE_PROOF_VALUE" | tee workflow-result.txt
        exit 0
    fi
    sleep 0.1
done
exit 23
"""


def capture_workflow(output, xvfb):
    home = output / "home"
    home.mkdir(mode=0o700)
    workspace = output / "chosen-project"
    source = workspace / "workflows" / WORKFLOW
    source.mkdir(parents=True)
    project = workspace / "dockpipe.config.json"
    project.write_text('{"schema":1}\n')
    config = source / "config.yml"
    config.write_text(CONFIGURATION)
    script = source / "proof.sh"
    script.write_text(SCRIPT)

    def environment_setup(environment):
        environment.update(HOME=str(home), PATH="/usr/bin:/bin",
                           DOCKPIPE_GLOBAL_ROOT=str(output / "global"), DOCKPIPE_SYSTEM_ROOT=str(output / "system"))

    desktop = OwnedDesktop(output / "desktop", {
        "xvfb": str(xvfb), "gui": str(LAUNCHER), "prepare_environment": environment_setup,
    })
    screenshots = []
    environment = None

    def event(*arguments):
        return subprocess.check_output(["/usr/bin/xdotool", *arguments], env=environment, timeout=5, text=True)

    def match(name, role, parent=None):
        candidates = parent if parent is not None else (node for node, _ in ui.nodes(atspi.Registry, gui.pid))
        visible = [node for node in candidates
                   if node is not None and node.name == name and node.getRoleName() == role
                   and node.getState().contains(atspi.STATE_SHOWING)]
        return visible[0] if len(visible) == 1 else None

    def click(name, role, parent=None):
        node = ui.wait_for(f"Expected visible {role} {name}", lambda: match(name, role, parent))
        bounds = node.queryComponent().getExtents(atspi.DESKTOP_COORDS)
        require(bounds.width > 0 and bounds.height > 0, "The selected launcher action has no usable bounds")
        event("mousemove", str(bounds.x + bounds.width // 2), str(bounds.y + bounds.height // 2), "click", "1")

    def capture(name):
        screenshot = output / (name + ".png")
        ImageGrab.grab(xdisplay=environment["DISPLAY"]).save(screenshot)
        screenshots.append(screenshot)

    def focus():
        ui.wait_for("Expected installed Dockpipe frame", lambda: match("Dockpipe", "frame"))
        require(Path("/proc", str(gui.pid), "exe").resolve(strict=True) == LAUNCHER.resolve(),
                "Unexpected running launcher executable")
        windows = event("search", "--onlyvisible", "--pid", str(gui.pid)).splitlines()
        require(len(windows) == 1, "Expected one visible launcher window")
        event("windowfocus", windows[0])

    def quit_gui():
        click("File", "menu item")
        click("Quit", "menu item")
        require(gui.wait(timeout=12) == 0, "Installed launcher did not quit normally")

    def visible_label(text):
        return any(node.name == text and node.getState().contains(atspi.STATE_SHOWING)
                   for node, _ in ui.nodes(atspi.Registry, gui.pid))

    def observe():
        result = subprocess.run([str(CLI), "runs", "observe", "--workdir", str(workspace), "--json"],
                                cwd=workspace, env=environment, timeout=5, check=True, capture_output=True)
        snapshot = json.loads(result.stdout)
        require(snapshot["schema"] == "dockpipe.execution_snapshot.v1" and not snapshot["truncated"],
                "Unexpected project run observation")
        candidates = [record for record in snapshot["executions"]
                      if record.get("workflow") == WORKFLOW and record.get("task") == "proof"]
        require(len(candidates) <= 1, "Expected one workflow execution")
        return candidates[0] if candidates else None

    try:
        gui = desktop.start()
        atspi, session = ui.connect(desktop.output / "session-private.json")
        environment = session["environment"]
        focus()
        capture("installed-start")

        click("File", "menu item")
        click("Open project folder…", "menu item")
        ui.wait_for("Expected the project folder picker", lambda: match("Choose", "push button"))
        event("type", "--clearmodifiers", "--delay", "12", str(workspace))
        event("key", "Return")
        capture("folder-picker-result")
        ui.wait_for("Selected project was not shown", lambda: visible_label(workspace.name))
        capture("selected-project")

        click("Workflows", "list item")
        click("Workflows", "push button")
        picker = ui.wait_for("Expected workflow picker", lambda: match("Workflows — Dockpipe", "dialog"))
        ui.wait_for("Generated host workflow not shown", lambda: any(
            "ATSPI_STATE_SHOWING" in item.get("states", [])
            and (WORKFLOW in item["name"] or WORKFLOW in item.get("text", ""))
            for item in ui.tree(atspi.Registry, gui.pid)))
        capture("workflow-picker")
        click("Run", "push button", picker)

        cwd_marker = workspace / "observed-cwd.txt"
        ui.wait_for("Host step did not start in the selected workspace", cwd_marker.is_file)
        running = ui.wait_for("Expected the original running host step", observe, 5)
        require(running["state"] == "running", "Expected the original host step to be running")
        launched_cli = Path("/proc", str(running["parent_pid"]), "exe").resolve(strict=True)
        require(launched_cli == CLI.resolve(), "The GUI did not invoke the installed CLI")
        require(cwd_marker.read_text() == str(workspace) + "\n", "Host step was bound to a different cwd")
        release = workspace / "release.txt"
        release.touch(exist_ok=False)

        def completed():
            record = observe()
            return record if record and record["state"] != "running" else None

        final = ui.wait_for("Installed host step did not produce a terminal result", completed, 8)
        require(final["id"] == running["id"] and final["state"] == "succeeded" and final["exit_code"] == 0,
                "Installed host workflow did not report the original successful exit")
        result = workspace / "workflow-result.txt"
        require(result.read_bytes() == (MARKER + "\n").encode(), "Installed host workflow marker differed")
        capture("workflow-result")
        quit_gui()

        settings = desktop.output / "config/dockpipe/dockpipe-launcher/launcher.json"
        require(read_json(settings)["projectFolder"] == str(workspace), "GUI did not persist the selected workspace")
        with (output / "restart.log").open("xb") as log:
            gui = subprocess.Popen([str(LAUNCHER), "--allow-second-instance"], cwd=session["workspace"],
                                   env=environment, stdout=log, stderr=log, start_new_session=True)
        desktop.processes.append(gui)
        focus()
        ui.wait_for("Restart did not restore the selected workspace", lambda: visible_label(workspace.name))
        require(result.read_bytes() == (MARKER + "\n").encode(), "Generated output did not survive restart")
        capture("restored-project")
        quit_gui()
        observations = output / "workflow-observation.json"
        write_json(observations, {"running": running, "final": final, "installed_cli": str(launched_cli),
                                  "workspace_selected": str(workspace), "workspace_restored": str(workspace)})
    except Exception:
        if environment is not None:
            try:
                capture("failure")
            except OSError as screenshot_error:
                print(f"Could not capture the failure screenshot: {screenshot_error}", file=sys.stderr)
        raise
    finally:
        desktop.stop()

    display = environment["DISPLAY"].removeprefix(":")
    require(all(process.poll() is not None for process in desktop.processes)
            and not Path(f"/tmp/.X11-unix/X{display}").exists(), "Owned launcher display remained live")
    return [project, config, script, cwd_marker, result, observations, *screenshots]


def run(request, output):
    require(bool(request["cases"]), "Select at least one installed desktop case")
    for case in request["cases"]:
        require(case["id"] in ASSERTIONS and (case["scenario"], case["layer"]) == ("happy", "installed_e2e")
                and set(case["required_assertions"]) == ASSERTIONS[case["id"]], "Unexpected installed desktop case")
    xvfb, identity, binaries = admit(request, output)
    configuration = request["configuration"]
    version = subprocess.check_output(["/usr/bin/dpkg-deb", "-f", configuration["cli_artifact"], "Version"],
                                      timeout=10, text=True).strip()
    environment = {"HOME": str(output), "PATH": "/usr/bin:/bin", "LANG": "C.UTF-8"}
    observed = {str(path): subprocess.check_output([str(path), "--version"], env=environment, timeout=10, text=True).strip()
                for path in (CLI, LAUNCHER)}
    require(all(value == version for value in observed.values()), "Installed CLI or launcher version differed from the DEB")
    versions = output / "versions.json"
    write_json(versions, {"package": version, "installed": observed})
    evidence = [identity, versions, *capture_workflow(output, xvfb)]
    require(all(digest(Path(name)) == value for name, value in binaries.items()),
            "Installed files changed during the workflow journey")

    cases = []
    for case in request["cases"]:
        row = {key: case[key] for key in ("id", "scenario", "layer")}
        reason = "Installed CLI and GUI startup, project selection, host workflow and retained workspace"
        row.update(status="passed", reason=reason,
                   assertions=[{"id": name, "status": "passed", "detail": reason} for name in case["required_assertions"]],
                   evidence=[{"path": path.relative_to(output).as_posix(), "sha256": digest(path)} for path in evidence])
        cases.append(row)
    receipt = {key: request[key] for key in ("run_id", "plan_sha256", "registry_sha256", "target")}
    receipt.update(schema="dockpipe.product-integration.lane/v1", cases=cases)
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
