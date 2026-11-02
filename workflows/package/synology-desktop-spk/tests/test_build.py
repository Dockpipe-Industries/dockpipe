"""Verify combined-package admission, lifecycle resources and reproducible archives."""

import hashlib
import importlib.util
import io
import json
from pathlib import Path
import shutil
import struct
import subprocess
import tarfile
import tempfile
import unittest
from unittest import mock


ROOT = Path(__file__).resolve().parents[4]
SCRIPT = ROOT / "workflows/package/synology-desktop-spk/assets/scripts/build.py"
SPECIFICATION = importlib.util.spec_from_file_location("synology_desktop_spk_build", SCRIPT)
PACKAGING = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(PACKAGING)


@unittest.skipUnless(shutil.which("dpkg-deb"), "dpkg-deb is required for real package fixtures")
class DesktopSpkTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="combined spk fixture ")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.output = self.root / "output"
        self.assets = self.root / "assets"
        (self.assets / "desktop").mkdir(parents=True)
        (self.assets / "ui").mkdir()
        (self.assets / "scripts").mkdir()
        (self.assets / "port_conf").mkdir()
        (self.assets / "port_conf/dockpipe-desktop.sc").write_text('[dockpipe_desktop]\nport_forward="no"\ndst.ports="38139/tcp"\n')
        for name in PACKAGING.DESKTOP_ASSETS:
            (self.assets / "desktop" / name).write_text(f"fixture {name} @WEBTOP_IMAGE@ @ARCH@\n")
        (self.assets / "ui/config").write_text('{".url":{"com.dockpipe.Desktop":{"type":"url"}}}\n')
        (self.assets / "install_uifile").write_text('[{"headline":"Desktop credentials"}]\n')
        (self.assets / "install_uifile.sh").write_text("#!/bin/sh\ncat <<'JSON'\n@INSTALL_WIZARD@\nJSON\n")
        (self.assets / "scripts/package-control.sh").write_text('#!/bin/sh\ncase "${0##*/}" in\n*) exit 0;;\nesac\n')
        patcher = mock.patch.object(PACKAGING, "ASSETS", self.assets)
        patcher.start()
        self.addCleanup(patcher.stop)
        self.binary = self.native_binary("amd64")
        self.cli = self.deb("dockpipe")
        self.desktop = self.deb("dockpipe-desktop")

    def native_binary(self, architecture):
        content = bytearray(120)
        content[:7] = b"\x7fELF\x02\x01\x01"
        machine = PACKAGING.CLI_PACKAGING.ARCHITECTURES[architecture][2]
        struct.pack_into("<HH", content, 16, 2, machine)
        struct.pack_into("<Q", content, 32, 64)
        struct.pack_into("<HH", content, 54, 56, 1)
        struct.pack_into("<I", content, 64, 1)
        destination = self.root / ("dockpipe-" + architecture)
        destination.write_bytes(content)
        return destination

    def deb(self, name, version="1.2.3", architecture="amd64"):
        source = self.root / f"source-{name}-{version}-{architecture}"
        control = source / "DEBIAN"
        control.mkdir(parents=True, exist_ok=True)
        (control / "control").write_text(
            f"Package: {name}\nVersion: {version}\nArchitecture: {architecture}\n"
            "Maintainer: Fixture <fixture@example.invalid>\nDescription: Package fixture\n"
        )
        destination = self.root / f"{name}-{version}-{architecture}.deb"
        subprocess.run(
            ["dpkg-deb", "--root-owner-group", "--build", str(source), str(destination)],
            check=True, capture_output=True, timeout=30,
        )
        return destination

    def build(self, **overrides):
        arguments = {
            "version": "1.2.3", "architecture": "amd64", "binary": self.binary,
            "cli_deb": self.cli, "desktop_deb": self.desktop, "output": self.output,
        }
        arguments.update(overrides)
        return PACKAGING.build(**arguments)

    def test_both_architectures_contain_host_cli_desktop_and_supported_resources(self):
        for architecture, filename_architecture, dsm_architecture in (
            ("amd64", "x64", "x86_64"), ("arm64", "arm64", "armv8"),
        ):
            with self.subTest(architecture=architecture):
                binary = self.native_binary(architecture)
                cli = self.deb("dockpipe", architecture=architecture)
                desktop = self.deb("dockpipe-desktop", architecture=architecture)
                package = self.build(architecture=architecture, binary=binary, cli_deb=cli, desktop_deb=desktop)
                self.assertEqual(package.name, f"dockpipe-desktop_1.2.3_synology_{filename_architecture}.spk")
                with tarfile.open(package) as archive:
                    self.assertTrue(all(member.isfile() for member in archive.getmembers()))
                    info = archive.extractfile("INFO").read().decode()
                    for field in (
                        'package="dockpipe-desktop"', 'version="1.2.3-1"', f'arch="{dsm_architecture}"',
                        'os_min_ver="7.2.1-69057"', 'install_dep_packages="ContainerManager>=20.10.23-1437"',
                        'dsmuidir="ui"', 'dsmappname="com.dockpipe.Desktop"', 'adminprotocol="https"',
                        'adminport="38139"', 'adminurl="/"', 'ctl_stop="yes"', 'checkport="yes"',
                        'silent_install="no"', 'silent_upgrade="yes"',
                    ):
                        self.assertIn(field, info)
                    self.assertEqual(json.load(archive.extractfile("conf/privilege")), {"defaults": {"run-as": "package"}})
                    resource = json.load(archive.extractfile("conf/resource"))
                    self.assertEqual(resource, {"port-config": {"protocol-file": "port_conf/dockpipe-desktop.sc"},
                                               "docker-project": {"projects": [{
                        "name": "dockpipe-desktop", "path": "desktop",
                        "build_params": {"force_recreate": True, "build": True},
                    }]}})
                    wizard = archive.extractfile("WIZARD_UIFILES/install_uifile.sh").read()
                    rendered = subprocess.run(["sh"], input=wizard, capture_output=True, check=True)
                    self.assertEqual(json.loads(rendered.stdout), [{"headline": "Desktop credentials"}])
                    control = (self.assets / "scripts/package-control.sh").read_bytes()
                    for script in PACKAGING.PACKAGE_SCRIPTS:
                        member = archive.getmember("scripts/" + script)
                        self.assertEqual(member.mode, 0o755)
                        self.assertEqual(archive.extractfile(member).read(), control)
                    for name, dimension in (("PACKAGE_ICON.PNG", 64), ("PACKAGE_ICON_256.PNG", 256)):
                        icon = archive.extractfile(name).read()
                        self.assertEqual(struct.unpack(">II", icon[16:24]), (dimension, dimension))
                    payload = archive.extractfile("package.tgz").read()
                    checksum = hashlib.md5(payload, usedforsecurity=False).hexdigest()
                    self.assertIn(f'checksum="{checksum}"', info)
                    with tarfile.open(fileobj=io.BytesIO(payload), mode="r:gz") as content:
                        expected = {"bin/dockpipe", "share/doc/dockpipe/LICENSE", "ui/config", "port_conf/dockpipe-desktop.sc",
                                    "desktop/packages/cli.deb", "desktop/packages/desktop.deb"}
                        expected.update("desktop/" + name for name in PACKAGING.DESKTOP_ASSETS)
                        expected.update(f"ui/images/icon_{size}.png" for size in (16, 24, 32, 48, 64, 72, 256))
                        self.assertEqual(set(content.getnames()), expected)
                        self.assertEqual(content.extractfile("bin/dockpipe").read(), binary.read_bytes())
                        self.assertEqual(content.getmember("bin/dockpipe").mode, 0o755)
                        self.assertEqual(content.getmember("desktop/entrypoint.sh").mode, 0o755)
                        self.assertEqual(content.extractfile("desktop/packages/cli.deb").read(), cli.read_bytes())
                        self.assertEqual(content.extractfile("desktop/packages/desktop.deb").read(), desktop.read_bytes())
                        self.assertEqual(content.extractfile("share/doc/dockpipe/LICENSE").read(), (ROOT / "LICENSE").read_bytes())
                        dockerfile = content.extractfile("desktop/Dockerfile").read().decode()
                        self.assertIn(PACKAGING.DEFAULT_WEBTOP_IMAGE, dockerfile)
                        self.assertIn(architecture, dockerfile)
                        self.assertNotIn("@ARCH@", dockerfile)
                        self.assertNotIn("@WEBTOP_IMAGE@", dockerfile)

    def test_archive_bytes_and_metadata_are_reproducible(self):
        destination = self.build()
        original = destination.read_bytes()
        self.binary.touch()
        self.binary.chmod(0o700)
        self.assertEqual(self.build().read_bytes(), original)
        with tarfile.open(destination) as archive:
            for member in archive.getmembers():
                self.assertEqual((member.uid, member.gid, member.mtime), (0, 0, 0))

    def test_rejects_wrong_native_architecture_header_and_symlink(self):
        with self.assertRaisesRegex(ValueError, "architecture"):
            self.build(binary=self.native_binary("arm64"))
        self.binary.write_bytes(b"not ELF")
        with self.assertRaisesRegex(ValueError, "ELF64"):
            self.build()
        self.binary = self.native_binary("amd64")
        link = self.root / "symlink"
        link.symlink_to(self.binary)
        with self.assertRaisesRegex(ValueError, "regular file"):
            self.build(binary=link)
        self.assertFalse(self.output.exists())

    def test_rejects_nonrelease_version_and_unknown_architecture(self):
        for version in ("../1.2.3", "1.2.3-staging.1", "1.2.3\n", "1.2.2147483648"):
            with self.subTest(version=version), self.assertRaisesRegex(ValueError, "[Vv]ersion"):
                self.build(version=version)
        with self.assertRaisesRegex(ValueError, "architecture"):
            self.build(architecture="armv7")
        self.assertFalse(self.output.exists())

    def test_rejects_wrong_deb_package_version_architecture_and_unpinned_image(self):
        cases = (
            {"cli_deb": self.deb("other")},
            {"desktop_deb": self.deb("dockpipe-desktop", version="1.2.4")},
            {"cli_deb": self.deb("dockpipe", architecture="arm64")},
            {"version": "1.2.4"},
            {"webtop_image": "lscr.io/linuxserver/webtop:ubuntu-xfce"},
        )
        for arguments in cases:
            with self.subTest(arguments=arguments), self.assertRaises(ValueError):
                self.build(**arguments)
        self.assertFalse(self.output.exists())

    def test_metadata_timeout_does_not_create_output(self):
        with mock.patch.object(PACKAGING.DESKTOP_PACKAGING.subprocess, "run", side_effect=subprocess.TimeoutExpired("dpkg-deb", 30)):
            with self.assertRaises(subprocess.TimeoutExpired):
                self.build()
        self.assertFalse(self.output.exists())

    def test_archive_failure_preserves_previous_package_and_cleans_temporary_files(self):
        destination = self.build()
        original = destination.read_bytes()
        with mock.patch.object(PACKAGING.CLI_PACKAGING, "add_file", side_effect=OSError("fixture write failure")):
            with self.assertRaisesRegex(OSError, "fixture write failure"):
                self.build()
        self.assertEqual(destination.read_bytes(), original)
        self.assertEqual(list(self.output.iterdir()), [destination])

    def test_missing_asset_preserves_previous_package(self):
        destination = self.build()
        original = destination.read_bytes()
        (self.assets / "desktop/entrypoint.sh").unlink()
        with self.assertRaises(FileNotFoundError):
            self.build()
        self.assertEqual(destination.read_bytes(), original)
        self.assertEqual(list(self.output.iterdir()), [destination])


if __name__ == "__main__":
    unittest.main()
