"""Verify combined-package admission, lifecycle resources and reproducible archives."""

import hashlib
import importlib.util
import io
import json
import os
import shlex
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
        (self.assets / "nginx").mkdir()
        (self.assets / "nginx/desktop.conf").write_text((PACKAGING.ASSETS / "nginx/desktop.conf").read_text())
        for name in PACKAGING.UI_ASSETS:
            (self.assets / "ui" / name).write_bytes((PACKAGING.ASSETS / "ui" / name).read_bytes())
        (self.assets / "systemd").mkdir()
        (self.assets / "systemd" / PACKAGING.NATIVE_HOST_UNIT).write_bytes(
            (PACKAGING.ASSETS / "systemd" / PACKAGING.NATIVE_HOST_UNIT).read_bytes())
        (self.assets / "scripts").mkdir()
        (self.assets / "port_conf").mkdir()
        (self.assets / "port_conf/dockpipe-desktop.sc").write_bytes((PACKAGING.ASSETS / "port_conf/dockpipe-desktop.sc").read_bytes())
        for name in PACKAGING.DESKTOP_ASSETS:
            (self.assets / "desktop" / name).write_text(f"fixture {name} @WEBTOP_IMAGE@ @ARCH@\n")
        for name in ("install_uifile", "upgrade_uifile", "host-folders-wizard.js"):
            (self.assets / name).write_bytes((PACKAGING.ASSETS / name).read_bytes())
        (self.assets / "uninstall_uifile").write_text((PACKAGING.ASSETS / "uninstall_uifile").read_text())
        (self.assets / "install_uifile.sh").write_text("#!/bin/sh\ncat <<'JSON'\n@INSTALL_WIZARD@\nJSON\n")
        (self.assets / "scripts/restore-discovery.sh").write_bytes((PACKAGING.ASSETS / "scripts/restore-discovery.sh").read_bytes())
        (self.assets / "scripts/package-control.sh").write_text('#!/bin/sh\ncase "${0##*/}" in\n*) exit 0;;\nesac\n')
        patcher = mock.patch.object(PACKAGING, "ASSETS", self.assets)
        patcher.start()
        self.addCleanup(patcher.stop)
        self.binary = self.native_binary("amd64")
        self.native_host = self.native_binary("amd64", "dockpipe-dsm-host")
        self.cli = self.deb("dockpipe")
        self.desktop = self.deb("dockpipe-desktop")

    def native_binary(self, architecture, name="dockpipe"):
        content = bytearray(120)
        content[:7] = b"\x7fELF\x02\x01\x01"
        machine = PACKAGING.CLI_PACKAGING.ARCHITECTURES[architecture][2]
        struct.pack_into("<HH", content, 16, 2, machine)
        struct.pack_into("<Q", content, 32, 64)
        struct.pack_into("<HH", content, 54, 56, 1)
        struct.pack_into("<I", content, 64, 1)
        destination = self.root / (name + "-" + architecture)
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
            "native_host_binary": self.native_host,
        }
        arguments.update(overrides)
        return PACKAGING.build(**arguments)

    def test_both_architectures_contain_host_cli_desktop_and_supported_resources(self):
        for architecture, filename_architecture, dsm_architecture in (
            ("amd64", "x64", "x86_64"), ("arm64", "arm64", "armv8"),
        ):
            with self.subTest(architecture=architecture):
                binary = self.native_binary(architecture)
                native_host = self.native_binary(architecture, "dockpipe-dsm-host")
                cli = self.deb("dockpipe", architecture=architecture)
                desktop = self.deb("dockpipe-desktop", architecture=architecture)
                package = self.build(architecture=architecture, binary=binary, native_host_binary=native_host,
                                     cli_deb=cli, desktop_deb=desktop)
                self.assertEqual(package.name, f"dockpipe-desktop_1.2.3_synology_{filename_architecture}.spk")
                with tarfile.open(package) as archive:
                    self.assertTrue(all(member.isfile() for member in archive.getmembers()))
                    info = archive.extractfile("INFO").read().decode()
                    for field in (
                        'package="dockpipe-desktop"', 'version="1.2.3-1"', f'arch="{dsm_architecture}"',
                        'os_min_ver="7.2.2-72806"', 'install_dep_packages="ContainerManager>=20.10.23-1437"',
                        'dsmuidir="ui"', 'dsmappname="com.dockpipe.Desktop"', 'dsmapplaunchname="com.dockpipe.Desktop"',
                        'instuninst_restart_services="nginx.service"', 'startstop_restart_services="nginx.service"',
                        'ctl_stop="yes"', 'checkport="no"',
                        'silent_install="no"', 'silent_upgrade="no"',
                    ):
                        self.assertIn(field, info)
                    self.assertEqual(json.load(archive.extractfile("conf/privilege")), {"defaults": {"run-as": "package"}})
                    resource = json.load(archive.extractfile("conf/resource"))
                    self.assertEqual(resource, {"systemd-user-unit": {},
                                               "port-config": {"protocol-file": "port_conf/dockpipe-desktop.sc"},
                                                "web-config": {"nginx-static-config": {"enable": [{"type": "dsm", "relpath": "nginx/desktop.conf"}]}},
                                               "docker-project": {"projects": [{
                        "name": "dockpipe-desktop", "path": "desktop",
                        "build_params": {"force_recreate": True, "build": True, "force_pull": False},
                    }]}})
                    unit = archive.extractfile("conf/systemd/" + PACKAGING.NATIVE_HOST_UNIT).read().decode()
                    self.assertIn("ExecStart=/var/packages/dockpipe-desktop/target/bin/dockpipe-dsm-host serve", unit)
                    self.assertIn("NoNewPrivileges=true", unit)
                    self.assertIn("Restart=no", unit)
                    self.assertIn("UMask=0077", unit)
                    self.assertNotIn("WantedBy", unit)
                    self.assertNotIn("User=root", unit)
                    self.assertIn('silent_uninstall="no"', info)
                    removal = json.load(archive.extractfile("WIZARD_UIFILES/uninstall_uifile"))
                    choice = removal[0]["items"][1]["subitems"][0]
                    self.assertEqual(choice["key"], "wizard_uninstall_data")
                    self.assertEqual(choice["defaultValue"], "keep")
                    self.assertEqual([row[1] for row in choice["store"]["data"]], ["keep", "delete"])
                    wizard = archive.extractfile("WIZARD_UIFILES/install_uifile.sh").read()
                    rendered = subprocess.run(["sh"], input=wizard, capture_output=True, check=True)
                    install_pages = json.loads(rendered.stdout)
                    upgrade_pages = json.load(archive.extractfile("WIZARD_UIFILES/upgrade_uifile"))
                    self.assertEqual(install_pages[:-1], json.loads((self.assets / "install_uifile").read_text()))
                    self.assertEqual(install_pages[-1]["custom_render_name"], upgrade_pages[0]["custom_render_name"])
                    self.assertEqual(install_pages[-1]["custom_render_fn"], (self.assets / "host-folders-wizard.js").read_text())
                    self.assertEqual(upgrade_pages[0]["custom_render_name"], "dockpipe_host_folders")
                    self.assertIn('var restoreMode = "upgrade";', upgrade_pages[0]["custom_render_fn"])
                    self.assertNotIn("@DOCKPIPE_", upgrade_pages[0]["custom_render_fn"])
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
                        expected = {"bin/dockpipe", "bin/dockpipe-dsm-host", "share/doc/dockpipe/LICENSE", "ui/config", "ui/Dockpipe.Desktop.native-v4.js",
                                    "nginx/desktop.conf", "port_conf/dockpipe-desktop.sc",
                                    "desktop/packages/cli.deb", "desktop/packages/desktop.deb"}
                        expected.add("desktop/compose.base.yml")
                        expected.update("desktop/" + name for name in PACKAGING.DESKTOP_ASSETS)
                        expected.update(f"ui/images/icon_{size}.png" for size in (16, 24, 32, 48, 64, 72, 256))
                        self.assertEqual(set(content.getnames()), expected)
                        self.assertEqual(content.extractfile("bin/dockpipe").read(), binary.read_bytes())
                        self.assertEqual(content.getmember("bin/dockpipe").mode, 0o755)
                        self.assertEqual(content.extractfile("bin/dockpipe-dsm-host").read(), native_host.read_bytes())
                        self.assertEqual(content.getmember("bin/dockpipe-dsm-host").mode, 0o755)
                        javascript = content.extractfile("ui/Dockpipe.Desktop.native-v4.js").read()
                        shared = PACKAGING.SHARED_DSM_SCRIPT.read_bytes()
                        entry = (self.assets / "ui/Dockpipe.Desktop.native-v4.js").read_bytes()
                        self.assertEqual(javascript, shared + b"\n" + entry)
                        launcher = json.load(content.extractfile("ui/config"))["Dockpipe.Desktop.native-v4.js"]
                        self.assertEqual(launcher["com.dockpipe.Desktop"]["appWindow"], "com.dockpipe.DesktopWindow")
                        self.assertEqual(content.getmember("desktop/entrypoint.sh").mode, 0o755)
                        self.assertEqual(content.extractfile("desktop/compose.base.yml").read(),
                                         content.extractfile("desktop/compose.yml").read())
                        self.assertEqual(content.extractfile("desktop/packages/cli.deb").read(), cli.read_bytes())
                        self.assertEqual(content.extractfile("desktop/packages/desktop.deb").read(), desktop.read_bytes())
                        self.assertEqual(content.extractfile("share/doc/dockpipe/LICENSE").read(), (ROOT / "LICENSE").read_bytes())
                        dockerfile = content.extractfile("desktop/Dockerfile").read().decode()
                        self.assertIn(PACKAGING.DEFAULT_WEBTOP_IMAGE, dockerfile)
                        self.assertIn(architecture, dockerfile)
                        self.assertNotIn("@ARCH@", dockerfile)
                        self.assertNotIn("@WEBTOP_IMAGE@", dockerfile)

    def test_prebuilt_editions_share_identity_and_use_native_unprivileged_preload(self):
        compose = PACKAGING.ROOT / "workflows/package/synology-desktop-spk/assets/desktop/compose-prebuilt.yml"
        (self.assets / "desktop/compose-prebuilt.yml").write_bytes(compose.read_bytes())
        config = json.dumps({
            "os": "linux", "architecture": "amd64", "rootfs": {"diff_ids": []},
            "config": {"User": "65532:65532", "Labels": {
                "org.opencontainers.image.version": "1.2.3",
                "org.opencontainers.image.revision": "a" * 40,
            }},
        }).encode()
        image_id = "sha256:" + hashlib.sha256(config).hexdigest()
        offline_image = "ghcr.io/example/desktop:offline-" + image_id.removeprefix("sha256:")
        image = self.root / "image.tar.gz"
        with tarfile.open(image, "w:gz") as archive:
            for name, content in (("config.json", config), ("manifest.json", json.dumps([{"Config": "config.json", "Layers": [], "RepoTags": [offline_image]}]).encode())):
                member = tarfile.TarInfo(name)
                member.size = len(content)
                archive.addfile(member, io.BytesIO(content))
        receipt = {
            "schema": 1, "version": "1.2.3", "architecture": "amd64", "source_sha": "a" * 40,
            "image": "ghcr.io/example/desktop@sha256:" + "b" * 64, "image_id": image_id, "offline_image": offline_image,
            "inputs": {"cli_sha256": hashlib.sha256(self.cli.read_bytes()).hexdigest(),
                       "desktop_sha256": hashlib.sha256(self.desktop.read_bytes()).hexdigest()},
            "archive_sha256": hashlib.sha256(image.read_bytes()).hexdigest(),
        }
        receipt_path = self.root / "image.json"
        receipt_path.write_text(json.dumps(receipt))
        for offline in (False, True):
            with self.subTest(offline=offline):
                package = self.build(image_receipt=receipt_path, offline_archive=image if offline else None)
                self.assertEqual("_offline" in package.name, offline)
                with tarfile.open(package) as archive:
                    self.assertEqual(json.loads(archive.extractfile("conf/privilege").read()),
                                     {"defaults": {"run-as": "package"}})
                    worker = json.loads(archive.extractfile("conf/resource").read())["docker-project"]
                    self.assertEqual(worker["projects"][0]["build_params"],
                                     {"build": False, "force_pull": False, "force_recreate": False})
                    self.assertEqual("preload-image" in worker, offline)
                    if offline:
                        self.assertEqual(worker["preload-image"], "desktop/image.tar.gz")
                    with tarfile.open(fileobj=io.BytesIO(archive.extractfile("package.tgz").read()), mode="r:gz") as payload:
                        self.assertNotIn("desktop/Dockerfile", payload.getnames())
                        self.assertNotIn("desktop/packages/desktop.deb", payload.getnames())
                        rendered = payload.extractfile("desktop/compose.yml").read().decode()
                        self.assertEqual(payload.extractfile("desktop/compose.base.yml").read().decode(), rendered)
                        self.assertNotIn("build:", rendered)
                        self.assertIn(offline_image if offline else receipt["image"], rendered)
                        self.assertIn("pull_policy: " + ("never" if offline else "missing"), rendered)
                        self.assertEqual(json.loads(payload.extractfile("desktop/image.json").read()), receipt)
                        if offline:
                            self.assertEqual(payload.extractfile("desktop/image.tar.gz").read(), image.read_bytes())
        image.write_bytes(b"changed archive")
        with self.assertRaisesRegex(ValueError, "SHA-256"):
            self.build(image_receipt=receipt_path, offline_archive=image)

    def test_packaging_repair_revision_upgrades_without_rebuilding_product_debs(self):
        package = self.build(package_revision=2)
        self.assertEqual(package.name, "dockpipe-desktop_1.2.3_synology_x64_r2.spk")
        with tarfile.open(package) as archive:
            self.assertIn('version="1.2.3-2"', archive.extractfile("INFO").read().decode())
        for revision in (0, -1, 2**31, True, "2"):
            with self.subTest(revision=revision), self.assertRaisesRegex(ValueError, "Package revision"):
                self.build(package_revision=revision)

    def test_rendered_wizards_embed_the_same_repeatable_folder_module(self):
        install, upgrade, folder_step = PACKAGING.render_wizards()
        install_pages = json.loads(install)
        upgrade_pages = json.loads(upgrade)
        self.assertEqual(len(upgrade_pages), 1)
        self.assertEqual(install_pages[-1]["custom_render_name"], upgrade_pages[0]["custom_render_name"])
        self.assertEqual(json.loads(folder_step), install_pages[-1])
        self.assertEqual(install_pages[:-1], json.loads((self.assets / "install_uifile").read_text()))
        self.assertEqual(upgrade_pages[0]["custom_render_name"], "dockpipe_host_folders")
        self.assertEqual(install_pages[-1]["custom_render_fn"], (self.assets / "host-folders-wizard.js").read_text())
        self.assertIn('var restoreMode = "upgrade";', upgrade_pages[0]["custom_render_fn"])
        self.assertNotIn("@DOCKPIPE_", upgrade_pages[0]["custom_render_fn"])
        self.assertNotIn("wizard_host_folder_path", folder_step)
        self.assertNotIn("wizard_host_folder_access", folder_step)

    def test_packaged_restore_wizard_includes_folder_step_without_requesting_credentials(self):
        template = ROOT / "workflows/package/synology-desktop-spk/assets/install_uifile.sh"
        (self.assets / "install_uifile.sh").write_bytes(template.read_bytes())
        filesystem = self.root / "filesystem"
        package_home = filesystem / "volume1/@apphome/dockpipe-desktop"
        (package_home / "desktop-settings").mkdir(parents=True)
        output = self.root / "wizard.json"
        with tarfile.open(self.build()) as archive:
            script = archive.extractfile("WIZARD_UIFILES/install_uifile.sh").read()
        self.assertNotIn(b"@HOST_FOLDER_WIZARD@", script)
        self.assertNotIn(b"@RESTORE_DISCOVERY_HELPERS@", script)
        script = script.decode().replace("render_install_wizard /", "render_install_wizard " + shlex.quote(str(filesystem))).encode()
        subprocess.run(["sh"], input=script, check=True, capture_output=True, timeout=5, env={
            **os.environ,
            "SYNOPKG_PKGHOME": "",
            "SYNOPKG_PKGDEST_VOL": "",
            "SYNOPKG_TEMP_LOGFILE": str(output),
        })
        wizard = json.loads(output.read_text())
        self.assertEqual(wizard[0]["step_title"], "Restore Dockpipe")
        self.assertNotIn("wizard_password", json.dumps(wizard))
        self.assertNotIn("wizard_username", json.dumps(wizard))
        self.assertEqual(wizard[1]["custom_render_name"], "dockpipe_host_folders")
        self.assertIn('var restoreMode = "restore";', wizard[1]["custom_render_fn"])
        self.assertIn('volumeList("/volume1")', wizard[1]["custom_render_fn"])

    def test_browser_routes_only_reach_the_pinned_authenticated_native_gateway(self):
        proxy = (self.assets / "nginx/desktop.conf").read_text()
        self.assertIn("location = /webman/3rdparty/dockpipe-desktop/native-host.cgi", proxy)
        self.assertIn("location ^~ /webman/3rdparty/dockpipe-desktop/desktop/", proxy)
        self.assertEqual(proxy.count("proxy_pass https://127.0.0.1:38143;"), 2)
        self.assertNotIn("38139", proxy)
        self.assertNotIn("proxy_ssl_verify off", proxy)
        self.assertEqual(proxy.count("proxy_ssl_verify on;"), 2)
        self.assertEqual(proxy.count("proxy_set_header Authorization \"\";"), 2)
        self.assertEqual(proxy.count("proxy_set_header Cookie $http_cookie;"), 2)
        self.assertEqual(proxy.count("proxy_set_header X-Syno-Token $http_x_syno_token;"), 2)
        for name, value in (("Remote-Addr", "$remote_addr"), ("Server-Addr", "$server_addr"),
                            ("Server-Port", "$server_port"), ("Scheme", "$scheme")):
            self.assertEqual(proxy.count(f"proxy_set_header X-Dockpipe-{name} {value};"), 2)
        self.assertIn("client_max_body_size 8k;", proxy)
        self.assertIn("proxy_read_timeout 5s;", proxy)
        self.assertIn("proxy_read_timeout 3600s;", proxy)
        self.assertIn("proxy_set_header Upgrade $http_upgrade;", proxy)
        self.assertIn("proxy_set_header Connection upgrade;", proxy)
        self.assertIn("/etc/native-host/endpoint-public/ca.pem", proxy)

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

    def test_rejects_invalid_native_gateway_before_publishing_an_artifact(self):
        with self.assertRaisesRegex(ValueError, "architecture"):
            self.build(native_host_binary=self.native_binary("arm64", "wrong-gateway"))
        self.native_host.write_bytes(b"not a native gateway")
        with self.assertRaisesRegex(ValueError, "ELF64"):
            self.build()
        self.native_host = self.native_binary("amd64", "restored-gateway")
        linked = self.root / "linked-gateway"
        linked.symlink_to(self.native_host)
        with self.assertRaisesRegex(ValueError, "regular file"):
            self.build(native_host_binary=linked)
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
