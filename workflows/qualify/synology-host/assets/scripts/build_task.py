#!/usr/bin/env python3
"""Build the separate unprivileged DSM Task Scheduler qualification companion."""

import argparse
import gzip
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile


ASSETS = Path(__file__).resolve().parents[1]
UI_ASSETS = ASSETS / "task-ui"
TASK_ASSETS = ASSETS / "task"
PACKAGE = "dockpipe-host-task-qualification"
APPLICATION = "com.dockpipe.HostTaskQualification"
JAVASCRIPT = "Dockpipe.HostTaskQualification.v3.js"
VERSION = "0.1.0-3"
UNIT = "pkguser-" + PACKAGE + ".service"
MAXIMUM_UI_BYTES = 256 * 1024

specification = importlib.util.spec_from_file_location("host_task_base_packaging", ASSETS / "scripts/build.py")
BASE = importlib.util.module_from_spec(specification)
specification.loader.exec_module(BASE)
PACKAGING = BASE.PACKAGING
ROOT = BASE.ROOT


def validate_ui():
    for name in ("config", JAVASCRIPT):
        path = UI_ASSETS / name
        if path.is_symlink() or not path.is_file() or path.stat().st_size > MAXIMUM_UI_BYTES:
            raise ValueError("Expected bounded regular task UI assets")
    configuration = json.loads((UI_ASSETS / "config").read_text())
    if not isinstance(configuration, dict):
        raise ValueError("Expected a task UI component map")
    components = configuration.get(JAVASCRIPT, {})
    if not isinstance(components, dict) or APPLICATION not in components:
        raise ValueError("Task UI application identity does not match the package")


def write_payload(path, binary):
    with path.open("wb") as stream:
        with gzip.GzipFile(filename="", mode="wb", fileobj=stream, mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode="w|", format=tarfile.USTAR_FORMAT) as archive:
                PACKAGING.add_file(archive, "bin/task-host", binary, 0o755)
                for name in ("config", JAVASCRIPT):
                    PACKAGING.add_file(archive, "ui/" + name, UI_ASSETS / name)
                PACKAGING.add_file(archive, "nginx/report.conf", TASK_ASSETS / "report.conf")
                PACKAGING.add_file(archive, "port_conf/native-probe.sc", TASK_ASSETS / "native-probe.sc")
                icons = ROOT / "src/dockpipe-app/resources/icons/hicolor"
                for dimension in (16, 24, 32, 48, 64, 72, 256):
                    source_dimension = 96 if dimension == 72 else dimension
                    PACKAGING.add_file(
                        archive, f"ui/images/icon_{dimension}.png",
                        icons / f"{source_dimension}x{source_dimension}/apps/dockpipe-launcher.png",
                    )
                PACKAGING.add_file(archive, "share/doc/LICENSE", ROOT / "LICENSE")
                PACKAGING.add_file(archive, "share/doc/TASK-QUALIFICATION.md", TASK_ASSETS / "README.md")


def build(architecture, binary, output):
    if architecture not in PACKAGING.ARCHITECTURES:
        raise ValueError("Expected amd64 or arm64")
    PACKAGING.validate_binary(binary, architecture)
    if binary.stat().st_size > BASE.MAXIMUM_BINARY_BYTES:
        raise ValueError("Task qualification binary exceeds 64 MiB")
    validate_ui()
    output.mkdir(parents=True, exist_ok=True)
    label, dsm_architecture, _ = PACKAGING.ARCHITECTURES[architecture]
    destination = output / f"{PACKAGE}_{VERSION}_{label}.spk"
    with tempfile.TemporaryDirectory(prefix=".task-qualification-", dir=output) as temporary:
        temporary = Path(temporary)
        payload = temporary / "package.tgz"
        write_payload(payload, binary)
        fields = {
            "package": PACKAGE,
            "displayname": "Dockpipe host task qualification",
            "version": VERSION,
            "arch": dsm_architecture,
            "os_min_ver": "7.2.1-69057",
            "maintainer": "Dockpipe Industries",
            "description": "Unprivileged native qualification of a fixed DSM-user Task Scheduler probe. No terminal or arbitrary commands.",
            "thirdparty": "yes",
            "dsmuidir": "ui",
            "dsmappname": APPLICATION,
            "dsmapplaunchname": APPLICATION,
            "ctl_stop": "yes",
            "silent_install": "no",
            "silent_upgrade": "yes",
            "silent_uninstall": "no",
            "instuninst_restart_services": "nginx.service",
            "startstop_restart_services": "nginx.service",
            "checksum": BASE.file_digest(payload, "md5"),
        }
        resource = {
            "systemd-user-unit": {},
            "port-config": {"protocol-file": "port_conf/native-probe.sc"},
            "web-config": {"nginx-static-config": {"enable": [{
                "type": "dsm", "relpath": "nginx/report.conf",
            }]}},
        }
        staged = temporary / destination.name
        with tarfile.open(staged, "w", format=tarfile.USTAR_FORMAT) as archive:
            info = "".join(f'{key}="{value}"\n' for key, value in fields.items()).encode()
            PACKAGING.add_bytes(archive, "INFO", info)
            PACKAGING.add_file(archive, "package.tgz", payload)
            PACKAGING.add_bytes(archive, "conf/privilege", b'{"defaults":{"run-as":"package"}}\n')
            PACKAGING.add_bytes(archive, "conf/resource", (json.dumps(resource, indent=2) + "\n").encode())
            PACKAGING.add_file(archive, "conf/systemd/" + UNIT, TASK_ASSETS / UNIT)
            control = (TASK_ASSETS / "package-control.sh").read_bytes()
            for action in ("preinst", "postinst", "preupgrade", "postupgrade", "preuninst", "postuninst", "start-stop-status"):
                PACKAGING.add_bytes(archive, "scripts/" + action, control, 0o755)
            icons = ROOT / "src/dockpipe-app/resources/icons/hicolor"
            for dimension, name in ((64, "PACKAGE_ICON.PNG"), (256, "PACKAGE_ICON_256.PNG")):
                PACKAGING.add_file(archive, name, icons / f"{dimension}x{dimension}/apps/dockpipe-launcher.png")
        staged.replace(destination)
    receipt = {
        "schema": 1,
        "package": PACKAGE,
        "version": VERSION,
        "architecture": architecture,
        "sha256": BASE.file_digest(destination),
        "bytes": destination.stat().st_size,
        "binary_sha256": BASE.file_digest(binary),
        "privilege": "package; no capabilities or setuid executable",
        "upstream_authentication": "Per-install TLS pin verified by nginx and health curl; no bundled key",
        "callback_authorization": "Kernel SO_PEERCRED UID and matching single-use nonce, not socket mode alone",
        "nas_acceptance": "pending",
        "task_creation_execution_deletion": "requires native DSM session acceptance; not performed by producer",
        "terminal_support": False,
    }
    destination.with_suffix(".json").write_text(json.dumps(receipt, indent=2) + "\n")
    return destination


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--architecture", choices=tuple(PACKAGING.ARCHITECTURES), default="amd64")
    parser.add_argument("--output", type=Path, required=True)
    options = parser.parse_args()
    output = options.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".task-build-", dir=output) as temporary:
        binary = Path(temporary) / "task-host"
        environment = dict(os.environ, GOOS="linux", GOARCH=options.architecture, CGO_ENABLED="0")
        subprocess.run(
            ["go", "build", "-trimpath", "-ldflags=-s -w", "-o", str(binary),
             "./workflows/qualify/synology-host/assets/task-native"],
            cwd=ROOT, env=environment, check=True, timeout=300,
        )
        print(build(options.architecture, binary, output))


if __name__ == "__main__":
    main()
