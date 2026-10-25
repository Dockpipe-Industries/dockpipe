#!/usr/bin/env python3
"""Exercise the adapted Cockpit catalog through real installed Marketplace paths."""
import argparse
import functools
import hashlib
import http.server
import importlib.util
import json
import os
from pathlib import Path
import ssl
import subprocess
import threading


APP_ID = "com.dockpipe.Dockpipe"


def module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


HOST = module("marketplace_bounded_command", Path(__file__).resolve().parents[5] / 'workflows/build/flatpak/assets/scripts/verify-installed-host.py')
COMPATIBILITY = module("marketplace_provider_check", Path(__file__).resolve().parents[5] / 'workflows/build/flatpak/assets/scripts/verify-installed-cockpit.py')


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def verify(catalog, test_build, lock_path, output):
    installation = Path(os.environ.get("FLATPAK_USER_DIR", ""))
    if not installation.is_absolute() or installation.is_symlink() or not installation.is_dir():
        raise ValueError("Select an existing task-owned FLATPAK_USER_DIR")
    if output.exists() or output.is_symlink():
        raise ValueError("Use a fresh Marketplace fixture output directory")
    lock = COMPATIBILITY.COCKPIT.load_lock(lock_path)
    manifest = COMPATIBILITY.COCKPIT.read_json(catalog / "packages-store-manifest.json")
    expected_platform = "linux-amd64-flatpak-org.kde.Platform-6.10"
    entries = [entry for entry in manifest["packages"].get("resolvers", []) if entry["name"] == "cockpit"]
    if manifest["platform"] != expected_platform or len(entries) != 1 or entries[0]["version"] != lock["version"]:
        raise ValueError("The adapted catalog must contain the exact newly pinned Cockpit for Flatpak")
    for name in ("workspace-package-test", "cockpit-package-rendering-test"):
        if not (test_build / name).is_file():
            raise ValueError(f"Missing existing UI qualification fixture: {name}")
    output.mkdir(parents=True)
    environment = os.environ.copy()
    directories = {}
    for variable in ("HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"):
        path = output / variable.lower()
        path.mkdir()
        directories[variable] = path
        environment[variable] = str(path)
    project = output / "project"
    project.mkdir()
    global_root = output / "global"
    global_root.mkdir()
    saved_fixture = output / "saved-project-state.json"
    saved_fixture.write_text('{"fixture":"preserve after disable and uninstall"}\n')
    preserved_hash = digest(saved_fixture)
    certificate = output / "fixture-ca.pem"
    private_key = output / "fixture-key.pem"
    subprocess.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
                    "-subj", "/CN=localhost", "-addext", "subjectAltName=DNS:localhost,IP:127.0.0.1",
                    "-keyout", str(private_key), "-out", str(certificate)], check=True,
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=30)
    requests = []
    allowed = {"/" + path.name for path in catalog.iterdir() if path.is_file() and not path.is_symlink()}

    class Handler(http.server.SimpleHTTPRequestHandler):
        def do_GET(self):
            if self.path not in allowed or len(requests) >= 512:
                self.send_error(404)
                return
            requests.append(self.path)
            super().do_GET()

        def log_message(self, format, *arguments):
            pass

    server = http.server.ThreadingHTTPServer(
        ("127.0.0.1", 0), functools.partial(Handler, directory=str(catalog)))
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.load_cert_chain(certificate, private_key)
    server.socket = context.wrap_socket(server.socket, server_side=True)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    remote = f"https://127.0.0.1:{server.server_port}/packages-store-manifest.json"

    def run(arguments, name, *, graphical=False, extra_environment=None):
        command = ["dbus-run-session", "--"]
        if graphical:
            command += ["xvfb-run", "-a", "-s", "-screen 0 1440x900x24"]
        command += ["flatpak", "run", "--user", "--branch=staging", "--share=network", "--cwd=" + str(project),
                    "--no-a11y-bus", "--no-documents-portal", "--nofilesystem=host", "--nofilesystem=home",
                    "--filesystem=" + str(output), "--filesystem=" + str(test_build) + ":ro",
                    "--unset-env=DOCKPIPE_WORKSPACE_PACKAGE", "--unset-env=DOCKPIPE_PACKAGES_ROOT",
                    "--env=DOCKPIPE_GLOBAL_ROOT=" + str(global_root), "--env=DOCKPIPE_BIN=/app/bin/dockpipe",
                    "--env=SSL_CERT_FILE=" + str(certificate), "--env=QT_QPA_PLATFORM=xcb",
                    "--env=QT_QPA_PLATFORMTHEME=generic", "--env=QT_IM_MODULE=compose",
                    "--env=LIBGL_ALWAYS_SOFTWARE=1", "--env=SDL_AUDIODRIVER=dummy"]
        for variable, path in directories.items():
            command.append("--env=" + variable + "=" + str(path))
        for variable, value in (extra_environment or {}).items():
            command.append("--env=" + variable + "=" + value)
        stdout = None
        if arguments[0] == "/app/bin/dockpipe":
            stdout = output / (name + ".stdout")
            command.append("--env=DOCKPIPE_FIXTURE_STDOUT=" + str(stdout))
            arguments = ["/usr/bin/bash", "-c",
                         'exec /app/bin/dockpipe "$@" > "$DOCKPIPE_FIXTURE_STDOUT"',
                         "dockpipe-fixture", *arguments[1:]]
        command += ["--command=" + arguments[0], APP_ID, *arguments[1:]]
        diagnostics = HOST.run_command(command, environment, output / (name + ".log"), timeout=180)
        if stdout is None:
            return diagnostics
        if stdout.is_symlink() or not stdout.is_file() or stdout.stat().st_size > HOST.MAX_DIAGNOSTIC_BYTES:
            raise ValueError("CLI stdout must be a bounded regular fixture file")
        return stdout.read_text()

    def inventory(name):
        return json.loads(run(["/app/bin/dockpipe", "package", "list", "--format", "json"], name))

    try:
        discovery = json.loads(run(["/app/bin/dockpipe", "package", "catalog", "--remote", remote], "catalog"))
        selected = [entry for entry in discovery["packages"] if entry["kind"] == "resolver" and entry["name"] == "cockpit"]
        if discovery["platform"] != expected_platform or len(selected) != 1 or selected[0]["version"] != lock["version"]:
            raise ValueError("Real Marketplace discovery did not select the exact adapted Cockpit")
        before = inventory("before-install")
        if any(entry["name"] == "cockpit" for entry in before["packages"]):
            raise ValueError("The fixture must begin without a user-installed Cockpit")
        run([str(test_build / "workspace-package-test")], "enable-install", graphical=True,
            extra_environment={"DOCKPIPE_PACKAGE_TEST_REMOTE": remote, "DOCKPIPE_PACKAGE_TEST_ACTION": "enable"})
        installed = [entry for entry in inventory("after-install")["packages"]
                     if entry["kind"] == "resolver" and entry["name"] == "cockpit" and entry.get("removable") is True]
        if len(installed) != 1 or installed[0]["version"] != lock["version"]:
            raise ValueError("Settings did not install the exact removable Marketplace archive")
        archive = Path(installed[0]["path"])
        if not archive.is_relative_to(output) or archive.is_symlink() or digest(archive) != selected[0]["sha256"]:
            raise ValueError("Installed archive differs from the selected adapted catalog checksum or fixture boundary")
        assets = Path(run(["/app/bin/dockpipe", "scope", "resolver", "cockpit", "assets-dir"], "resolve-installed").strip())
        if not assets.is_relative_to(output):
            raise ValueError("Resolver selected package assets outside the isolated fixture state")
        # Flatpak derives XDG config beneath the selected HOME for its app ID.
        config_root = directories["HOME"] / ".var/app" / APP_ID / "config"
        settings_files = list(config_root.rglob("launcher.json"))
        if len(settings_files) != 1:
            raise ValueError("Expected one fixture-owned persisted launcher settings file")
        settings_file = settings_files[0]
        settings = json.loads(settings_file.read_text())
        if settings.get("cockpit", {}).get("enabled") is not True:
            raise ValueError("Enable did not persist the real Cockpit setting")
        settings["fixture_preserved"] = "keep through disable and package removal"
        settings_file.write_text(json.dumps(settings))
        rendering_log = run([str(test_build / "cockpit-package-rendering-test")], "installed-render", graphical=True,
                            extra_environment={"DOCKPIPE_PACKAGE_RENDER_INSTALLED": "1",
                                               "DOCKPIPE_PACKAGE_RENDER_MAPS": str(output / "loaded-maps.txt"),
                                               "DOCKPIPE_PACKAGE_RENDER_CAPTURE": str(output / "render.png")})
        frames = COMPATIBILITY.rendering_result(rendering_log)
        providers = COMPATIBILITY.providers((output / "loaded-maps.txt").read_text(), assets)
        run([str(test_build / "workspace-package-test")], "disable", graphical=True,
            extra_environment={"DOCKPIPE_PACKAGE_TEST_REMOTE": remote, "DOCKPIPE_PACKAGE_TEST_ACTION": "disable"})
        disabled = json.loads(settings_file.read_text())
        if disabled.get("cockpit", {}).get("enabled") is not False or disabled.get("fixture_preserved") != settings["fixture_preserved"]:
            raise ValueError("Disable failed to persist or changed unrelated fixture preferences")
        if not archive.is_file() or digest(archive) != selected[0]["sha256"]:
            raise ValueError("Disabling Cockpit changed or removed its installed archive")
        settings_hash = digest(settings_file)
        run(["/app/bin/dockpipe", "package", "uninstall", "--path", str(archive)], "uninstall")
        if archive.exists() or any(entry["name"] == "cockpit" for entry in inventory("after-uninstall")["packages"]):
            raise ValueError("Package uninstall did not remove the selected optional Cockpit archive")
        if digest(saved_fixture) != preserved_hash or digest(settings_file) != settings_hash:
            raise ValueError("Package uninstall changed fixture saves or persisted disabled settings")
        receipt = {"schema": 1, "platform": expected_platform, "cockpit_version": lock["version"],
                   "cockpit_lock_sha256": digest(lock_path), "adapted_archive_sha256": selected[0]["sha256"],
                   "phases": ["real HTTPS catalog", "Settings enable and checksum-pinned install", "normal CLI resolution",
                              "installed rendering", "Settings disable", "CLI uninstall", "save/settings preservation"],
                   "providers": providers, "rendering": frames, "requests": requests,
                   "workspace_package_override": False, "chromium_sandbox_disabled": False,
                   "fixture_saves_sha256": preserved_hash, "preserved_disabled_settings_sha256": settings_hash,
                   "fixture_ca_sha256": digest(certificate), "physical_gpu_qualified": False}
        (output / "qualification.json").write_text(json.dumps(receipt, indent=2) + "\n")
        return receipt
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("catalog", "test-build-dir", "cockpit-lock", "output"):
        parser.add_argument("--" + name, type=Path, required=True)
    args = parser.parse_args()
    try:
        receipt = verify(args.catalog.resolve(strict=True), args.test_build_dir.resolve(strict=True),
                         args.cockpit_lock, args.output.absolute())
    except (OSError, ValueError, KeyError, subprocess.SubprocessError) as error:
        parser.exit(1, f"Marketplace Cockpit qualification failed: {error}\n")
    print(json.dumps(receipt, indent=2))


if __name__ == "__main__":
    main()
