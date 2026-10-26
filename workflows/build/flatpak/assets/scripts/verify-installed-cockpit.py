#!/usr/bin/env python3
"""Qualify the locked Cockpit binary in the installed Flatpak host and Platform."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess


APP_ID = "com.dockpipe.Dockpipe"
GL_REF = "runtime/org.freedesktop.Platform.GL.default/x86_64/25.08"
GL_COMMIT = "fe659a591fe882f573584ae62cfb55b92065e4ff7a972833271438bddfa6669e"


def module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


HOST = module("cockpit_installed_command", Path(__file__).resolve().parents[5] / 'workflows/build/flatpak/assets/scripts/verify-installed-host.py')
COCKPIT = module("cockpit_pinned_consumer", Path(__file__).resolve().parents[5] / 'workflows/qualify/cockpit/assets/scripts/prepare-cockpit.py')
SDK = module("cockpit_flatpak_browser", Path(__file__).resolve().parents[5] / 'workflows/build/flatpak/assets/scripts/prepare-flatpak-browser-sdk.py')
BROWSER = module("cockpit_browser_readback", Path(__file__).resolve().parents[5] / 'workflows/dependencies/browser-sdk/assets/scripts/browser-runtime.py')


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def providers(text, assets):
    """Require the actual host distribution providers, not native bundled Qt."""
    paths = {line.split()[-1] for line in text.splitlines() if len(line.split()) >= 6}
    expected = {
        "qt_core": "/usr/lib/x86_64-linux-gnu/libQt6Core.so.6.10.3",
        "qt_webengine_core": "/app/lib/x86_64-linux-gnu/libQt6WebEngineCore.so.6.10.2",
        "qt_webengine_widgets": "/app/lib/x86_64-linux-gnu/libQt6WebEngineWidgets.so.6.10.2",
        "browser": "/app/lib/libSpatialWebBrowser.so.0",
        "host_ui": "/app/libexec/dockpipe/libdockpipe-launcher-ui.so.1.0",
        "cockpit": str(assets / "ui/libdockpipe-cockpit.so"),
    }
    for name, path in expected.items():
        basename = Path(path).name.split(".so", 1)[0] + ".so"
        selected = {value for value in paths if Path(value).name.startswith(basename)}
        if selected != {path}:
            raise ValueError(f"Installed Cockpit selected unexpected {name} providers: {sorted(selected)}")
    return expected


def rendering_result(text):
    product = re.search(r"phase=product movement_frames=(\d+).*compositor_failures=(\d+)", text)
    completed = re.search(r"phase=synthetic-software-GL completed_frames=(\d+)", text)
    recovery = re.search(r"phase=synthetic-software-GL recovery_completed_frames=(\d+)", text)
    if (not product or int(product[1]) < 40 or int(product[2]) != 0
            or not completed or int(completed[1]) < 2 or not recovery or int(recovery[1]) < 1):
        raise ValueError("Installed Cockpit lacks required product and software GL/recovery rendering proof")
    return {"product_movement_frames": int(product[1]), "compositor_failures": int(product[2]),
            "software_gl_completed_frames": int(completed[1]), "software_gl_recovery_frames": int(recovery[1])}


def verify(lock_path, prepared, browser_sdk, app_build, test_build, output):
    installation = Path(os.environ.get("FLATPAK_USER_DIR", ""))
    if not installation.is_absolute() or installation.is_symlink() or not installation.is_dir():
        raise ValueError("FLATPAK_USER_DIR must select an existing task-owned installation")
    if output.exists() or output.is_symlink():
        raise ValueError("Use a fresh qualification output directory")
    lock = COCKPIT.load_lock(lock_path)
    COCKPIT.verify_store(prepared / "store", lock)
    browser_lock_path = Path(__file__).resolve().parents[5] / 'workflows/build/flatpak/assets/spatialweb-flatpak-browser.lock.json'
    browser_lock, _ = SDK.load_lock(browser_lock_path, SDK.PLATFORM)
    SDK.verify_refs(browser_lock)
    gl_commit = subprocess.check_output(
        ["flatpak", "info", "--user", "--show-commit", GL_REF], text=True).strip()
    if gl_commit != GL_COMMIT:
        raise ValueError("Software GL qualification input differs from its exact admitted commit")
    browser_receipt, entries = BROWSER.verify(browser_sdk, browser_lock_path, SDK.PLATFORM, SDK)
    for name in ("cockpit-package-rendering-test", "libcockpit-rendering-viewport-fixture.so"):
        if not (test_build / name).is_file():
            raise ValueError(f"Missing inherited rendering fixture: {name}")
    output.mkdir(parents=True)
    # Re-extract the verified immutable archive; a previously extracted tree is
    # not evidence that its current bytes still match the committed package pin.
    assets = COCKPIT.unpack_runtime(
        prepared / "store" / lock["artifact"]["url"].rsplit("/", 1)[-1], output / "package")
    environment = os.environ.copy()
    for key in ("HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"):
        directory = output / key.lower()
        directory.mkdir()
        environment[key] = str(directory)
    location = Path(subprocess.check_output(
        ["flatpak", "info", "--user", "--show-location", APP_ID], text=True, env=environment).strip())
    if not location.is_relative_to(installation):
        raise ValueError("Installed app is outside the selected task-owned installation")
    installed = location / "files"
    browser_binary = "lib/libSpatialWebBrowser.so.0"
    if browser_binary not in entries or digest(installed / browser_binary) != entries[browser_binary]["sha256"]:
        raise ValueError("Installed app Browser differs from its attested SDK inventory")
    host_ui = Path("libexec/dockpipe/libdockpipe-launcher-ui.so.1.0")
    if digest(installed / host_ui) != digest(app_build / "files" / host_ui):
        raise ValueError("Installed Host UI differs from the tested staged app")
    command = ["dbus-run-session", "--", "xvfb-run", "-a", "-s", "-screen 0 1440x900x24",
               "flatpak", "run", "--user", "--branch=staging", "--unshare=network",
               "--no-a11y-bus", "--no-documents-portal", "--nofilesystem=host", "--nofilesystem=home",
               "--filesystem=" + str(test_build) + ":ro", "--filesystem=" + str(output),
               "--env=QT_QPA_PLATFORM=xcb", "--env=QT_QPA_PLATFORMTHEME=generic",
               "--env=QT_IM_MODULE=compose", "--env=LIBGL_ALWAYS_SOFTWARE=1", "--env=SDL_AUDIODRIVER=dummy",
               "--env=DOCKPIPE_WORKSPACE_PACKAGE=" + str(assets),
               "--env=DOCKPIPE_WORKSPACE_READER=/app/libexec/dockpipe/dockpipe-workspace-reader",
               "--env=DOCKPIPE_COCKPIT_READER=/app/libexec/dockpipe/dockpipe-workspace-reader",
               "--env=DOCKPIPE_PACKAGE_RENDER_MAPS=" + str(output / "loaded-maps.txt"),
               "--env=DOCKPIPE_PACKAGE_RENDER_CAPTURE=" + str(output / "render.png"),
               "--command=" + str(test_build / "cockpit-package-rendering-test"), APP_ID]
    text = HOST.run_command(command, environment, output / "rendering.log", timeout=120)
    frames = rendering_result(text)
    selected = providers((output / "loaded-maps.txt").read_text(), assets)
    runtime_location = Path(subprocess.check_output(
        ["flatpak", "info", "--user", "--show-location", browser_lock["flatpak"]["runtime"]["ref"]],
        text=True, env=environment).strip())
    provider_hashes = {}
    for name, path in selected.items():
        if path.startswith("/app/"):
            source = installed / Path(path).relative_to("/app")
        elif path.startswith("/usr/"):
            source = runtime_location / "files" / Path(path).relative_to("/usr")
        else:
            source = Path(path)
        provider_hashes[name] = digest(source)
    receipt = {"schema": 1, "scope": "Installed Platform host with exact locked Cockpit; in-process distribution ABI compatibility",
               "cockpit_lock_sha256": digest(lock_path), "cockpit": lock,
               "browser_receipt": browser_receipt, "flatpak": browser_lock["flatpak"],
               "qualification_gl": {"ref": GL_REF, "commit": gl_commit},
               "installed_app_commit": subprocess.check_output(
                   ["flatpak", "info", "--user", "--show-commit", APP_ID], text=True, env=environment).strip(),
               "providers": selected, "provider_sha256": provider_hashes, "rendering": frames, "physical_gpu_qualified": False,
               "chromium_sandbox_disabled": False,
               "hashes": {"host_ui": digest(installed / host_ui), "browser": digest(installed / browser_binary),
                          "cockpit": digest(assets / "ui/libdockpipe-cockpit.so"),
                          "fixture": digest(test_build / "cockpit-package-rendering-test"),
                          "mappings": digest(output / "loaded-maps.txt"), "screenshot": digest(output / "render.png")}}
    (output / "qualification.json").write_text(json.dumps(receipt, indent=2) + "\n")
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("lock", "prepared", "browser-sdk", "app-build-dir", "test-build-dir", "output"):
        parser.add_argument("--" + name, type=Path, required=True)
    args = parser.parse_args()
    try:
        receipt = verify(args.lock, args.prepared, args.browser_sdk, args.app_build_dir,
                         args.test_build_dir, args.output.absolute())
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        parser.exit(1, f"Installed Flatpak Cockpit qualification failed: {error}\n")
    print(json.dumps(receipt, indent=2))


if __name__ == "__main__":
    main()
