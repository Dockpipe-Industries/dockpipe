#!/usr/bin/env python3
"""Build the separate, capability-explicit DSM identity qualification companion."""

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
specification = importlib.util.spec_from_file_location("host_qualification_packaging", ASSETS / "scripts/build.py")
BASE = importlib.util.module_from_spec(specification)
specification.loader.exec_module(BASE)
PACKAGING = BASE.PACKAGING
ROOT = BASE.ROOT
PACKAGE = "dockpipe-host-identity-qualification"
VERSION = "0.1.0-3"
UNIT = "pkguser-" + PACKAGE + ".service"


def companion_text(path):
    text = path.read_text()
    replacements = {
        "dockpipe-host-qualification": PACKAGE,
        "HostQualification": "HostIdentityQualification",
        "dockpipe.synology_host_qualification.v1": "dockpipe.synology_host_identity_qualification.v1",
        "Dockpipe host qualification": "Dockpipe host identity qualification",
        "38140": "38141",
        "requestTimeoutMilliseconds = 5000": "requestTimeoutMilliseconds = 10000",
    }
    for old, new in replacements.items():
        text = text.replace(old, new)
    return text.encode()


def write_payload(path, binary):
    with path.open("wb") as stream:
        with gzip.GzipFile(filename="", mode="wb", fileobj=stream, mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode="w|", format=tarfile.USTAR_FORMAT) as archive:
                # Ordinary worker/probe roles execute the same immutable sealed
                # memfd, never a mutable package-owned filesystem executable.
                PACKAGING.add_file(archive, "bin/identity-cap-helper", binary, 0o700)
                # Installer-only ordinary role; no secret is bundled in the SPK.
                PACKAGING.add_file(archive, "bin/identity-endpoint-init", binary, 0o700)
                for source, target in (("config", "config"), ("Dockpipe.HostQualification.js", "Dockpipe.HostIdentityQualification.js")):
                    PACKAGING.add_bytes(archive, "ui/" + target, companion_text(ASSETS / "ui" / source))
                PACKAGING.add_file(archive, "nginx/report.conf", ASSETS / "identity/report.conf")
                PACKAGING.add_bytes(archive, "port_conf/native-probe.sc", companion_text(ASSETS / "port_conf/native-probe.sc"))
                icons = ROOT / "src/dockpipe-app/resources/icons/hicolor"
                for dimension in (16, 24, 32, 48, 64, 72, 256):
                    source_dimension = 96 if dimension == 72 else dimension
                    PACKAGING.add_file(archive, f"ui/images/icon_{dimension}.png", icons / f"{source_dimension}x{source_dimension}/apps/dockpipe-launcher.png")
                PACKAGING.add_file(archive, "share/doc/LICENSE", ROOT / "LICENSE")
                PACKAGING.add_file(archive, "share/doc/IDENTITY-QUALIFICATION.md", ASSETS / "identity/README.md")


def build(architecture, binary, output):
    if architecture not in PACKAGING.ARCHITECTURES:
        raise ValueError("Expected amd64 or arm64")
    PACKAGING.validate_binary(binary, architecture)
    if binary.stat().st_size > BASE.MAXIMUM_BINARY_BYTES:
        raise ValueError("Identity qualification binary exceeds 64 MiB")
    output.mkdir(parents=True, exist_ok=True)
    label, dsm_architecture, _ = PACKAGING.ARCHITECTURES[architecture]
    destination = output / f"{PACKAGE}_{VERSION}_{label}.spk"
    with tempfile.TemporaryDirectory(prefix=".identity-qualification-", dir=output) as temporary:
        temporary = Path(temporary)
        payload = temporary / "package.tgz"
        write_payload(payload, binary)
        fields = {
            "package": PACKAGE, "displayname": "Dockpipe host identity qualification",
            "version": VERSION, "arch": dsm_architecture, "os_min_ver": "7.2.1-69057",
            "maintainer": "Dockpipe Industries", "thirdparty": "yes", "dsmuidir": "ui",
            "description": "Explicit CAP_SETUID/CAP_SETGID qualification for one authenticated local administrator identity report. No terminal.",
            "dsmappname": "com.dockpipe.HostIdentityQualification",
            "dsmapplaunchname": "com.dockpipe.HostIdentityQualification",
            "ctl_stop": "yes", "silent_install": "no", "silent_upgrade": "yes", "silent_uninstall": "no",
            "instuninst_restart_services": "nginx.service", "startstop_restart_services": "nginx.service",
            "checksum": BASE.file_digest(payload, "md5"),
        }
        privilege = {
            "defaults": {"run-as": "package"},
            "tool": [{"relpath": "bin/identity-cap-helper", "user": "package", "group": "package",
                      "permission": "0700", "capabilities": "cap_setuid,cap_setgid"}],
        }
        resource = {
            "systemd-user-unit": {},
            "port-config": {"protocol-file": "port_conf/native-probe.sc"},
            "web-config": {"nginx-static-config": {"enable": [{"type": "dsm", "relpath": "nginx/report.conf"}]}},
        }
        staged = temporary / destination.name
        with tarfile.open(staged, "w", format=tarfile.USTAR_FORMAT) as archive:
            PACKAGING.add_bytes(archive, "INFO", "".join(f'{key}="{value}"\n' for key, value in fields.items()).encode())
            PACKAGING.add_file(archive, "package.tgz", payload)
            for name, value in (("privilege", privilege), ("resource", resource)):
                PACKAGING.add_bytes(archive, "conf/" + name, (json.dumps(value, indent=2) + "\n").encode())
            PACKAGING.add_file(archive, "conf/systemd/" + UNIT, ASSETS / "identity" / UNIT)
            control = (ASSETS / "identity/package-control.sh").read_bytes()
            for action in ("preinst", "postinst", "preupgrade", "postupgrade", "preuninst", "postuninst", "start-stop-status"):
                PACKAGING.add_bytes(archive, "scripts/" + action, control, 0o755)
            icons = ROOT / "src/dockpipe-app/resources/icons/hicolor"
            for dimension, name in ((64, "PACKAGE_ICON.PNG"), (256, "PACKAGE_ICON_256.PNG")):
                PACKAGING.add_file(archive, name, icons / f"{dimension}x{dimension}/apps/dockpipe-launcher.png")
        staged.replace(destination)
    receipt = {
        "schema": 1, "package": PACKAGE, "version": VERSION, "architecture": architecture,
        "sha256": BASE.file_digest(destination), "bytes": destination.stat().st_size,
        "binary_sha256": BASE.file_digest(binary), "privilege": "package plus CAP_SETUID,CAP_SETGID daemon only",
        "nas_acceptance": "pending", "terminal_support": False,
        "limitations": ["Local /etc/passwd and /etc/group administrators only", "Sealed memfd execution, ambient capability propagation and DSM tool worker require native acceptance", "No production shell, sudo, or shared GUI grant integration"],
        "executable_authority": "Running daemon inode copied to sealed memfd and inherited as FD3; no mutable worker/probe paths",
        "upstream_authentication": "Per-install TLS certificate pinned by nginx and health curl; no bundled key",
        "supersedes": "0.1.0-2 unauthenticated loopback transport and 0.1.0-1 mutable executable boundary must not be installed",
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
    with tempfile.TemporaryDirectory(prefix=".identity-build-", dir=output) as temporary:
        binary = Path(temporary) / "identity-cap-helper"
        environment = dict(os.environ, GOOS="linux", GOARCH=options.architecture, CGO_ENABLED="0")
        subprocess.run(["go", "build", "-trimpath", "-ldflags=-s -w", "-o", str(binary),
                        "./workflows/qualify/synology-host/assets/identity-native"],
                       cwd=ROOT, env=environment, check=True, timeout=300)
        print(build(options.architecture, binary, output))


if __name__ == "__main__":
    main()
