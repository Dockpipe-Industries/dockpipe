#!/usr/bin/env python3
"""Qualify real host PTYs through an installed candidate and owned D-Bus sessions."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import selectors
import signal
import subprocess
import time

APP_ID = "com.dockpipe.Dockpipe"
RUNTIME = "org.kde.Platform//6.10"
# Fixed Qt test selections include setup/cleanup and both cleanup data rows.
FIXTURES = (
    ("terminal-host-launch-test", ("nativeFlatpakHostPty", "nativeFlatpakHostCleanup"), 5),
    ("terminal-session-test", ("nativeFlatpakHostIntegration",), 3),
)
MAX_DIAGNOSTIC_BYTES = 1024 * 1024


def terminate_owned(process):
    try:
        os.killpg(process.pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        process.wait(timeout=5)


def run_command(arguments, environment, log, timeout=90):
    """Bound retained output and runtime; only terminate our own process group."""
    deadline = time.monotonic() + timeout
    failure = ""
    written = 0
    process = subprocess.Popen(arguments, env=environment, stdout=subprocess.PIPE,
                               stderr=subprocess.STDOUT, start_new_session=True)
    try:
        with log.open("wb") as output, selectors.DefaultSelector() as reader:
            reader.register(process.stdout, selectors.EVENT_READ)
            while reader.get_map():
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    failure = "timed out"
                    break
                for key, _ in reader.select(timeout=min(remaining, 1)):
                    chunk = os.read(key.fd, 65536)
                    if not chunk:
                        reader.unregister(key.fileobj)
                        continue
                    available = MAX_DIAGNOSTIC_BYTES - written
                    output.write(chunk[:available])
                    written += min(len(chunk), available)
                    if len(chunk) > available:
                        failure = "diagnostic exceeded 1 MiB"
                        break
                if failure:
                    break
        if failure:
            terminate_owned(process)
        else:
            try:
                code = process.wait(timeout=max(0, deadline - time.monotonic()))
                if code:
                    failure = f"exited {code}"
            except subprocess.TimeoutExpired:
                failure = "timed out"
                terminate_owned(process)
    finally:
        process.stdout.close()
        if process.poll() is None:
            terminate_owned(process)
    text = log.read_text(errors="replace")
    if failure:
        raise RuntimeError(f"Installed-host command {failure}; see {log}\n{text[-8192:]}")
    return text


def validate_fixture(text, expected_passes):
    match = re.search(r"Totals:\s+(\d+) passed, (\d+) failed, (\d+) skipped", text)
    if not match or tuple(map(int, match.groups())) != (expected_passes, 0, 0):
        raise RuntimeError("Installed-host fixture must pass every selected case with zero skips\n" + text[-8192:])
    return dict(zip(("passed", "failed", "skipped"), map(int, match.groups())))


def verify(bundle, build_directory, output):
    bundle = Path(bundle).resolve(strict=True)
    build_directory = Path(build_directory).resolve(strict=True)
    output = Path(output).absolute()
    installation_value = os.environ.get("FLATPAK_USER_DIR")
    if not installation_value:
        raise RuntimeError("FLATPAK_USER_DIR must select the CI-owned installation")
    installation = Path(installation_value)
    if not installation.is_absolute() or installation.is_symlink() or not installation.is_dir():
        raise RuntimeError("FLATPAK_USER_DIR must be an existing absolute installation directory")
    candidate = installation / "app" / APP_ID
    if candidate.exists() or candidate.is_symlink():
        raise RuntimeError("Refusing to replace an existing Dockpipe app installation")
    if not bundle.is_file() or bundle.suffix != ".flatpak":
        raise RuntimeError("Expected the freshly built Flatpak bundle")
    for executable, _, _ in FIXTURES:
        if not (build_directory / executable).is_file():
            raise RuntimeError(f"Missing Flatpak SDK test executable: {executable}")
    output.mkdir(parents=True, exist_ok=False)
    environment = os.environ.copy()
    # Flatpak's writable app data also depends on HOME, independently of its
    # installation directory. Keep both kinds of state inside this qualification.
    for variable, name in (("HOME", "home"), ("XDG_CONFIG_HOME", "config"),
                           ("XDG_DATA_HOME", "data"), ("XDG_STATE_HOME", "state"),
                           ("XDG_CACHE_HOME", "cache")):
        directory = output / name
        directory.mkdir()
        environment[variable] = str(directory)
    run_command(["flatpak", "info", RUNTIME], environment, output / "runtime.log", 30)
    run_command(["dbus-run-session", "--", "flatpak", "install", "--user", "--noninteractive",
                 "--assumeyes", "--no-deps", "--no-related", "--bundle", str(bundle)],
                environment, output / "install.log", 120)
    fixtures = []
    for executable, selections, passes in FIXTURES:
        command = ["dbus-run-session", "--", "flatpak", "run", "--user", "--branch=staging",
                   "--unshare=network", "--nofilesystem=home", "--nofilesystem=host",
                   f"--filesystem={build_directory}:ro", "--env=QT_QPA_PLATFORM=offscreen",
                   "--env=DOCKPIPE_TEST_FLATPAK_HOST_NATIVE=1",
                   f"--command={build_directory / executable}", APP_ID, *selections]
        log = output / f"{executable}.log"
        text = run_command(command, environment, log)
        fixtures.append({"executable": executable, "functions": selections,
                         "totals": validate_fixture(text, passes), "log": log.name})
    digest = hashlib.sha256()
    with bundle.open("rb") as archive:
        for chunk in iter(lambda: archive.read(1024 * 1024), b""):
            digest.update(chunk)
    receipt = {"app_id": APP_ID, "runtime": RUNTIME, "installation": str(installation),
               "bundle_sha256": digest.hexdigest(),
               "scope": "Installed Platform sandbox, real host service and PTYs; no SDK substitution",
               "fixtures": fixtures}
    (output / "qualification.json").write_text(json.dumps(receipt, indent=2) + "\n")
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bundle", required=True)
    parser.add_argument("--build-dir", required=True)
    parser.add_argument("--out", required=True)
    arguments = parser.parse_args()
    result = verify(arguments.bundle, arguments.build_dir, arguments.out)
    passed = sum(fixture["totals"]["passed"] for fixture in result["fixtures"])
    print(f"PASS: installed Platform host PTY, job cleanup and public session: {passed} passed, zero skips")


if __name__ == "__main__":
    main()
