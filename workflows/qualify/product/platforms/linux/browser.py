"""Exercise the installed Pop!_OS Dockpipe Browser on an owned X11 display."""

import argparse
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
import socket
import subprocess
import sys
from threading import Thread
import time
from urllib.parse import urlparse

from PIL import ImageGrab

from debian_installation import CLI, LAUNCHER, admit
from desktop import OwnedDesktop, digest
import ui

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "assets/scripts"))
from contracts import read_json, require, validate_receipt, write_json


ASSERTIONS = {"local-page-loaded", "tabs-and-downloads", "profile-restart"}
PAYLOAD = b"Dockpipe generated browser download\n"


def fixture_server(requests):
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *_arguments):
            pass

        def do_GET(self):
            route = urlparse(self.path)
            requests.append(self.path)
            if route.path == "/attachment":
                body = PAYLOAD
                self.send_response(200)
                self.send_header("Content-Type", "application/octet-stream")
                self.send_header("Content-Disposition", 'attachment; filename="dockpipe-fixture.txt"')
            elif route.path in ("/first", "/second"):
                title = "First fixture tab" if route.path == "/first" else "Second fixture tab"
                body = (
                    f"<!doctype html><title>{title}</title><h1>{title}</h1>"
                    '<a href="/attachment">Download Dockpipe fixture</a>'
                    "<button onclick=\"localStorage.setItem('dockpipe-fixture', 'retained');"
                    "fetch('/storage-seeded')\">Seed temporary storage</button>"
                    '<script>fetch("/storage-" + (localStorage.getItem("dockpipe-fixture") || "empty"));</script>'
                ).encode()
                self.send_response(200)
                self.send_header("Content-Type", "text/html; charset=utf-8")
            else:
                body = b""
                self.send_response(204)
            self.send_header("Content-Length", str(len(body)))
            self.send_header("Cache-Control", "no-store")
            self.end_headers()
            self.wfile.write(body)

    return HTTPServer(("127.0.0.1", 0), Handler)


def exercise(output, xvfb):
    home = output / "home"
    home.mkdir(mode=0o700)
    (home / "Downloads").mkdir()
    requests = []

    def environment_setup(environment):
        environment.update(HOME=str(home), DOCKPIPE_BIN=str(CLI), PATH="/usr/bin:/bin",
                           DOCKPIPE_GLOBAL_ROOT=str(output / "global"), DOCKPIPE_SYSTEM_ROOT=str(output / "system"))

    desktop = OwnedDesktop(output / "desktop", {
        "xvfb": str(xvfb), "gui": str(LAUNCHER), "prepare_environment": environment_setup,
    })
    server = fixture_server(requests)
    server_thread = Thread(target=server.serve_forever, daemon=True)
    server_thread.start()
    origin = f"http://127.0.0.1:{server.server_port}"
    screenshots = []

    def key(*arguments):
        return subprocess.check_output(["/usr/bin/xdotool", *arguments], env=environment, timeout=5, text=True)

    def match(name, role, parent=None):
        candidates = list(parent) if parent is not None else (node for node, _ in ui.nodes(atspi.Registry, gui.pid))
        visible = [node for node in candidates if node.name == name and node.getRoleName() == role
                   and node.getState().contains(atspi.STATE_SHOWING)]
        return visible[0] if len(visible) == 1 else None

    def click(name, role, parent=None):
        node = ui.wait_for(f"Expected visible {role} {name}", lambda: match(name, role, parent))
        bounds = node.queryComponent().getExtents(atspi.DESKTOP_COORDS)
        require(bounds.width > 0 and bounds.height > 0, "The selected Browser control has no usable bounds")
        key("mousemove", str(bounds.x + bounds.width // 2), str(bounds.y + bounds.height // 2), "click", "1")

    def open_browser():
        ui.wait_for("Expected original Dockpipe frame", lambda: match("Dockpipe", "frame"))
        windows = key("search", "--onlyvisible", "--pid", str(gui.pid)).splitlines()
        require(len(windows) == 1, "Expected one visible launcher window")
        key("windowfocus", windows[0])
        click("View", "menu item")
        click("Browser", "menu item")

    def navigate(path, title, phase):
        click("Website address or search", "text")
        key("key", "ctrl+a")
        key("type", origin + path)
        key("key", "Return")
        ui.wait_for("Browser did not request " + path, lambda: path in requests)
        page = ui.wait_for("Expected rendered page " + title, lambda: match(title, "document web"))
        screenshot = output / (phase + ".png")
        ImageGrab.grab(xdisplay=environment["DISPLAY"]).save(screenshot)
        screenshots.append(screenshot)
        return page

    def quit_gui():
        click("File", "menu item")
        click("Quit", "menu item")
        require(gui.wait(timeout=12) == 0, "Installed launcher did not quit normally")

    try:
        gui = desktop.start()
        atspi, session = ui.connect(desktop.output / "session-private.json")
        environment = session["environment"]
        open_browser()
        navigate("/first", "First fixture tab", "first-tab")
        click("New tab (Ctrl+T)", "push button")
        navigate("/second", "Second fixture tab", "second-tab")
        click("First fixture tab", "page tab")
        ui.wait_for("First tab did not retain its document", lambda: match("First fixture tab", "document web"))
        click("Second fixture tab", "page tab")
        page = ui.wait_for("Second tab did not retain its document", lambda: match("Second fixture tab", "document web"))
        tabs = output / "tab-observation.json"
        write_json(tabs, {"selected_documents": ["First fixture tab", "Second fixture tab"]})
        click("Seed temporary storage", "push button", page)
        ui.wait_for("Browser did not seed temporary storage", lambda: "/storage-seeded" in requests)
        click("Download Dockpipe fixture", "link", page)
        ui.wait_for("Browser did not request the attachment", lambda: "/attachment" in requests)
        download = home / "Downloads/dockpipe-fixture.txt"
        time.sleep(1)
        if not download.is_file():
            key("key", "Return")
        ui.wait_for("Expected exact downloaded bytes", lambda: download.is_file() and download.read_bytes() == PAYLOAD)
        retained_before = requests.count("/storage-retained")
        navigate("/second?phase=reload", "Second fixture tab", "before-restart")
        ui.wait_for("Temporary storage did not survive a page reload", lambda: requests.count("/storage-retained") > retained_before)
        quit_gui()

        previous_empty = requests.count("/storage-empty")
        previous_retained = requests.count("/storage-retained")
        with (output / "restart.log").open("xb") as log:
            gui = subprocess.Popen([str(LAUNCHER), "--allow-second-instance"], cwd=session["workspace"],
                                   env=environment, stdout=log, stderr=log, start_new_session=True)
        desktop.processes.append(gui)
        open_browser()
        navigate("/second?phase=restart", "Second fixture tab", "after-restart")
        ui.wait_for("Restart did not use fresh temporary storage", lambda: requests.count("/storage-empty") > previous_empty)
        require(requests.count("/storage-retained") == previous_retained, "Restart reused the prior temporary storage")
        quit_gui()
    finally:
        try:
            desktop.stop()
        finally:
            server.shutdown()
            server_thread.join(timeout=5)
            server.server_close()
            write_json(output / "requests.json", requests)

    display = desktop.environment["DISPLAY"].removeprefix(":")
    require(all(process.poll() is not None for process in desktop.processes)
            and not Path(f"/tmp/.X11-unix/X{display}").exists() and not server_thread.is_alive(),
            "Owned Browser fixture or display remained live")
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as client:
        client.settimeout(1)
        require(client.connect_ex(("127.0.0.1", server.server_port)) != 0, "Owned HTTP listener remained live")
    return [output / "requests.json", tabs, download, *screenshots]


def run(request, output):
    case, = request["cases"]
    require((case["id"], case["scenario"], case["layer"]) == ("browser.local", "happy", "installed_e2e")
            and set(case["required_assertions"]) == ASSERTIONS, "Unexpected installed Browser case")
    xvfb, identity_path, binaries = admit(request, output)
    evidence = exercise(output, xvfb)
    require(all(digest(Path(name)) == expected for name, expected in binaries.items()),
            "Installed files changed during the Browser journey")

    row = {key: case[key] for key in ("id", "scenario", "layer")}
    row.update(status="passed", reason="Installed Browser rendered live pages, downloaded exact bytes and restarted its profile",
               assertions=[{"id": name, "status": "passed", "detail": "See loopback requests, download and captured Browser frames"}
                           for name in case["required_assertions"]],
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
