#!/usr/bin/env python3
"""Build a separate, read-only DSM host authentication/service probe."""

import argparse
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile


ROOT = Path(__file__).resolve().parents[5]
ASSETS = Path(__file__).resolve().parents[1]
PACKAGE = "dockpipe-host-qualification"
VERSION = "0.1.0-5"
specification = importlib.util.spec_from_file_location(
    "synology_cli_packaging", ROOT / "workflows/package/synology/assets/scripts/build.py",
)
PACKAGING = importlib.util.module_from_spec(specification)
specification.loader.exec_module(PACKAGING)
MAXIMUM_BINARY_BYTES = 64 * 1024 * 1024


def file_digest(path, algorithm="sha256"):
    result = hashlib.new(algorithm, usedforsecurity=algorithm != "md5")
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            result.update(block)
    return result.hexdigest()


def write_payload(path, binary):
    with path.open("wb") as stream:
        with gzip.GzipFile(filename="", mode="wb", fileobj=stream, mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode="w|", format=tarfile.USTAR_FORMAT) as archive:
                PACKAGING.add_file(archive, "bin/host-probe", binary, 0o755)
                PACKAGING.add_file(archive, "ui/host-auth.cgi", binary, 0o755)
                for name in ("config", "Dockpipe.HostQualification.js"):
                    PACKAGING.add_file(archive, "ui/" + name, ASSETS / "ui" / name)
                PACKAGING.add_file(archive, "nginx/report.conf", ASSETS / "nginx/report.conf")
                PACKAGING.add_file(archive, "port_conf/native-probe.sc", ASSETS / "port_conf/native-probe.sc")
                icons = ROOT / "src/dockpipe-app/resources/icons/hicolor"
                for dimension in (16, 24, 32, 48, 64, 72, 256):
                    source_dimension = 96 if dimension == 72 else dimension
                    PACKAGING.add_file(
                        archive, f"ui/images/icon_{dimension}.png",
                        icons / f"{source_dimension}x{source_dimension}/apps/dockpipe-launcher.png",
                    )
                PACKAGING.add_file(archive, "share/doc/LICENSE", ROOT / "LICENSE")


def build(architecture, binary, output):
    if architecture not in PACKAGING.ARCHITECTURES:
        raise ValueError("Expected amd64 or arm64")
    PACKAGING.validate_binary(binary, architecture)
    if binary.stat().st_size > MAXIMUM_BINARY_BYTES:
        raise ValueError("Native qualification binary exceeds 64 MiB")
    output.mkdir(parents=True, exist_ok=True)
    label, dsm_architecture, _ = PACKAGING.ARCHITECTURES[architecture]
    destination = output / f"{PACKAGE}_{VERSION}_{label}.spk"
    with tempfile.TemporaryDirectory(prefix=".native-qualification-", dir=output) as temporary:
        temporary = Path(temporary)
        payload = temporary / "package.tgz"
        write_payload(payload, binary)
        fields = {
            "package": PACKAGE,
            "displayname": "Dockpipe host qualification",
            "version": VERSION,
            "arch": dsm_architecture,
            "os_min_ver": "7.2.1-69057",
            "maintainer": "Dockpipe Industries",
            "description": "Read-only DSM login and native service qualification. No terminal or Docker controls.",
            "thirdparty": "yes",
            "dsmuidir": "ui",
            "dsmappname": "com.dockpipe.HostQualification",
            "dsmapplaunchname": "com.dockpipe.HostQualification",
            "ctl_stop": "yes",
            "silent_install": "no",
            "silent_upgrade": "yes",
            "silent_uninstall": "no",
            "instuninst_restart_services": "nginx.service",
            "startstop_restart_services": "nginx.service",
            "checksum": file_digest(payload, "md5"),
        }
        info = "".join(f'{key}="{value}"\n' for key, value in fields.items()).encode()
        staged = temporary / destination.name
        with tarfile.open(staged, "w", format=tarfile.USTAR_FORMAT) as archive:
            PACKAGING.add_bytes(archive, "INFO", info)
            PACKAGING.add_file(archive, "package.tgz", payload)
            PACKAGING.add_bytes(archive, "conf/privilege", b'{"defaults":{"run-as":"package"}}\n')
            resource = {
                "systemd-user-unit": {},
                "port-config": {"protocol-file": "port_conf/native-probe.sc"},
                "web-config": {"nginx-static-config": {"enable": [{
                    "type": "dsm", "relpath": "nginx/report.conf",
                }]}},
            }
            PACKAGING.add_bytes(archive, "conf/resource", (json.dumps(resource, indent=2) + "\n").encode())
            unit_name = "pkguser-dockpipe-host-qualification.service"
            PACKAGING.add_file(archive, "conf/systemd/" + unit_name, ASSETS / "systemd" / unit_name)
            control = (ASSETS / "scripts/package-control.sh").read_bytes()
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
        "sha256": file_digest(destination),
        "bytes": destination.stat().st_size,
        "binary_sha256": file_digest(binary),
        "privilege": "package",
        "purpose": "Read-only native authentication and service identity qualification; not a Dockpipe upgrade.",
        "nas_acceptance": "pending",
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
    with tempfile.TemporaryDirectory(prefix=".host-probe-build-", dir=output) as temporary:
        binary = Path(temporary) / "host-probe"
        environment = dict(os.environ, GOOS="linux", GOARCH=options.architecture, CGO_ENABLED="0")
        subprocess.run(
            ["go", "build", "-trimpath", "-ldflags=-s -w", "-o", str(binary),
             "./workflows/qualify/synology-host/assets/native"],
            cwd=ROOT, env=environment, check=True, timeout=300,
        )
        print(build(options.architecture, binary, output))


if __name__ == "__main__":
    main()
