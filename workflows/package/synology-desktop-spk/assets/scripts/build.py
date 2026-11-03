#!/usr/bin/env python3
"""Package the native CLI and Container Manager desktop without host installation."""

import argparse
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile


ROOT = Path(__file__).resolve().parents[5]
ASSETS = Path(__file__).resolve().parents[1]
DESKTOP_ASSETS = (
    "Dockerfile", "entrypoint.sh", "compose.yml", ".dockerignore",
    "desktop.py", "session.sh", "openbox.xml",
)
UI_ASSETS = ("config", "Dockpipe.Desktop.js")
PACKAGE_SCRIPTS = (
    "preinst", "postinst", "preupgrade", "postupgrade", "preuninst", "postuninst", "start-stop-status",
)


def load_helper(name, relative_path):
    specification = importlib.util.spec_from_file_location(name, ROOT / relative_path)
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


CLI_PACKAGING = load_helper("synology_cli_packaging", "workflows/package/synology/assets/scripts/build.py")
DESKTOP_PACKAGING = load_helper("synology_desktop_packaging", "workflows/package/synology-desktop/assets/scripts/stage.py")
DEFAULT_WEBTOP_IMAGE = DESKTOP_PACKAGING.DEFAULT_WEBTOP_IMAGE
IMAGE_RECEIPT = load_helper("synology_image_receipt", "workflows/package/synology-desktop-spk/assets/scripts/image_receipt.py")


def render_asset(relative_path, architecture, webtop_image):
    content = (ASSETS / relative_path).read_text()
    return content.replace("@WEBTOP_IMAGE@", webtop_image).replace("@ARCH@", architecture).encode("utf-8")


def write_payload(destination, binary, cli_deb, desktop_deb, architecture, webtop_image, receipt=None, offline_archive=None):
    with destination.open("wb") as stream:
        with gzip.GzipFile(filename="", mode="wb", fileobj=stream, mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode="w|", format=tarfile.USTAR_FORMAT) as archive:
                CLI_PACKAGING.add_file(archive, "bin/dockpipe", binary, 0o755)
                CLI_PACKAGING.add_file(archive, "share/doc/dockpipe/LICENSE", ROOT / "LICENSE")
                CLI_PACKAGING.add_file(archive, "port_conf/dockpipe-desktop.sc", ASSETS / "port_conf/dockpipe-desktop.sc")
                if receipt is None:
                    for name in DESKTOP_ASSETS:
                        mode = 0o755 if name.endswith(".sh") else 0o644
                        content = render_asset(Path("desktop") / name, architecture, webtop_image)
                        CLI_PACKAGING.add_bytes(archive, "desktop/" + name, content, mode)
                    CLI_PACKAGING.add_file(archive, "desktop/packages/cli.deb", cli_deb)
                    CLI_PACKAGING.add_file(archive, "desktop/packages/desktop.deb", desktop_deb)
                else:
                    reference = receipt["offline_image"] if offline_archive else receipt["image"]
                    compose = (ASSETS / "desktop/compose-prebuilt.yml").read_text()
                    compose = compose.replace("@IMAGE@", reference).replace("@ARCH@", architecture)
                    compose = compose.replace("@PULL_POLICY@", "never" if offline_archive else "missing")
                    CLI_PACKAGING.add_bytes(archive, "desktop/compose.yml", compose.encode())
                    CLI_PACKAGING.add_bytes(archive, "desktop/image.json", (json.dumps(receipt, indent=2) + "\n").encode())
                    if offline_archive:
                        CLI_PACKAGING.add_file(archive, "desktop/image.tar.gz", offline_archive)
                for name in UI_ASSETS:
                    ui = render_asset(Path("ui") / name, architecture, webtop_image)
                    CLI_PACKAGING.add_bytes(archive, "ui/" + name, ui)
                CLI_PACKAGING.add_file(archive, "nginx/desktop.conf", ASSETS / "nginx/desktop.conf")
                icons = ROOT / "src/dockpipe-app/resources/icons/hicolor"
                for dimension in (16, 24, 32, 48, 64, 72, 256):
                    source_dimension = 96 if dimension == 72 else dimension
                    source = icons / f"{source_dimension}x{source_dimension}/apps/dockpipe-launcher.png"
                    CLI_PACKAGING.add_file(archive, f"ui/images/icon_{dimension}.png", source)


def payload_checksum(path):
    checksum = hashlib.md5(usedforsecurity=False)
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            checksum.update(block)
    return checksum.hexdigest()


def package_info(version, architecture, checksum, package_revision=1):
    fields = {
        "package": "dockpipe-desktop",
        "displayname": "Dockpipe",
        "version": f"{version}-{package_revision}",
        "arch": CLI_PACKAGING.ARCHITECTURES[architecture][1],
        "os_min_ver": "7.2.1-69057",
        "maintainer": "Dockpipe Industries",
        "maintainer_url": "https://dockpipe.com",
        "description": "Dockpipe desktop and command-line tools. Open Dockpipe from DSM.",
        "install_dep_packages": "ContainerManager>=20.10.23-1437",
        "thirdparty": "yes",
        "dsmuidir": "ui",
        "dsmappname": "com.dockpipe.Desktop",
        "dsmapplaunchname": "com.dockpipe.Desktop",
        "instuninst_restart_services": "nginx.service",
        "startstop_restart_services": "nginx.service",
        "ctl_stop": "yes",
        "checkport": "no",
        "silent_install": "no",
        "silent_upgrade": "yes",
        "silent_uninstall": "no",
        "checksum": checksum,
    }
    return "".join(f'{key}="{value}"\n' for key, value in fields.items()).encode("utf-8")


def build(version, architecture, binary, cli_deb, desktop_deb, output, webtop_image=DEFAULT_WEBTOP_IMAGE,
          package_revision=1, *, image_receipt=None, offline_archive=None):
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", version):
        raise ValueError("Expected a numeric release version")
    if any(int(part) > 2**31 - 1 for part in version.split(".")):
        raise ValueError("Version exceeds DSM numeric field limits")
    if type(package_revision) is not int or not 1 <= package_revision <= 2**31 - 1:
        raise ValueError("Package revision must be a positive DSM build number")
    if architecture not in CLI_PACKAGING.ARCHITECTURES:
        raise ValueError("Expected amd64 or arm64 architecture")
    CLI_PACKAGING.validate_binary(binary, architecture)
    cli_path, desktop_path, package_version = DESKTOP_PACKAGING.validate_inputs(
        cli_deb, desktop_deb, architecture, webtop_image,
    )
    if package_version != version:
        raise ValueError("DEB versions must match the requested release version exactly")

    if offline_archive and image_receipt is None:
        raise ValueError("Offline packaging requires a prebuilt image receipt")
    receipt = None
    if image_receipt is not None:
        receipt = IMAGE_RECEIPT.load_receipt(
            image_receipt, version, architecture, cli_path, desktop_path, offline_archive,
        )

    output.mkdir(parents=True, exist_ok=True)
    label = CLI_PACKAGING.ARCHITECTURES[architecture][0]
    suffix = "_offline" if offline_archive else ""
    if package_revision != 1:
        suffix += f"_r{package_revision}"
    destination = output / f"dockpipe-desktop_{version}_synology_{label}{suffix}.spk"
    with tempfile.TemporaryDirectory(prefix=".synology-desktop-", dir=output) as temporary:
        temporary = Path(temporary)
        payload = temporary / "package.tgz"
        write_payload(payload, binary, cli_path, desktop_path, architecture, webtop_image, receipt, offline_archive)
        resource = {
            "port-config": {"protocol-file": "port_conf/dockpipe-desktop.sc"},
            "web-config": {"nginx-static-config": {"enable": [{
                "type": "dsm", "relpath": "nginx/desktop.conf",
            }]}},
            "docker-project": {
                "projects": [{
                    "name": "dockpipe-desktop",
                    "path": "desktop",
                    # Prebuilt image references change with image content. Let
                    # Compose reconcile them without replacing unchanged containers.
                    "build_params": {
                        "force_recreate": receipt is None,
                        "build": receipt is None,
                        "force_pull": False,
                    },
                }],
            },
        }
        if offline_archive:
            resource["docker-project"]["preload-image"] = "desktop/image.tar.gz"
        privilege = {"defaults": {"run-as": "package"}}
        control = (ASSETS / "scripts/package-control.sh").read_bytes()
        wizard = (ASSETS / "install_uifile").read_text()
        wizard_script = (ASSETS / "install_uifile.sh").read_text().replace("@INSTALL_WIZARD@", wizard)
        package = temporary / destination.name
        with tarfile.open(package, "w", format=tarfile.USTAR_FORMAT) as archive:
            CLI_PACKAGING.add_bytes(archive, "INFO", package_info(version, architecture, payload_checksum(payload), package_revision))
            CLI_PACKAGING.add_file(archive, "package.tgz", payload)
            CLI_PACKAGING.add_bytes(archive, "conf/privilege", (json.dumps(privilege, indent=2) + "\n").encode())
            CLI_PACKAGING.add_bytes(archive, "conf/resource", (json.dumps(resource, indent=2) + "\n").encode())
            CLI_PACKAGING.add_bytes(archive, "WIZARD_UIFILES/install_uifile.sh", wizard_script.encode(), 0o755)
            CLI_PACKAGING.add_file(archive, "WIZARD_UIFILES/uninstall_uifile", ASSETS / "uninstall_uifile")
            for name in PACKAGE_SCRIPTS:
                CLI_PACKAGING.add_bytes(archive, "scripts/" + name, control, 0o755)
            icons = ROOT / "src/dockpipe-app/resources/icons/hicolor"
            CLI_PACKAGING.add_file(archive, "PACKAGE_ICON.PNG", icons / "64x64/apps/dockpipe-launcher.png")
            CLI_PACKAGING.add_file(archive, "PACKAGE_ICON_256.PNG", icons / "256x256/apps/dockpipe-launcher.png")
        package.replace(destination)
    return destination


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("version")
    parser.add_argument("architecture", choices=CLI_PACKAGING.ARCHITECTURES)
    parser.add_argument("--binary", required=True, type=Path)
    parser.add_argument("--cli-deb", required=True, type=Path)
    parser.add_argument("--desktop-deb", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--webtop-image", default=DEFAULT_WEBTOP_IMAGE)
    parser.add_argument("--package-revision", type=int, default=1)
    parser.add_argument("--image-receipt", type=Path)
    parser.add_argument("--offline-archive", type=Path)
    arguments = parser.parse_args()
    try:
        destination = build(
            arguments.version, arguments.architecture, arguments.binary,
            arguments.cli_deb, arguments.desktop_deb, arguments.output, arguments.webtop_image,
            arguments.package_revision, image_receipt=arguments.image_receipt,
            offline_archive=arguments.offline_archive,
        )
    except (ValueError, OSError, subprocess.SubprocessError) as error:
        parser.exit(1, f"Synology desktop packaging failed: {error}\n")
    print("Built " + str(destination))


if __name__ == "__main__":
    main()
