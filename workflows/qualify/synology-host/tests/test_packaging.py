"""Check isolation and admission of the native DSM qualification package."""

import hashlib
import importlib.util
import io
import json
from pathlib import Path
import struct
import subprocess
import tarfile
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "assets/scripts/build.py"
specification = importlib.util.spec_from_file_location("synology_host_qualification", SCRIPT)
BUILD = importlib.util.module_from_spec(specification)
specification.loader.exec_module(BUILD)


class QualificationPackagingTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)

    def binary(self, architecture):
        content = bytearray(120)
        content[:7] = b"\x7fELF\x02\x01\x01"
        struct.pack_into("<HH", content, 16, 2, BUILD.PACKAGING.ARCHITECTURES[architecture][2])
        struct.pack_into("<Q", content, 32, 64)
        struct.pack_into("<HH", content, 54, 56, 1)
        struct.pack_into("<I", content, 64, 1)
        binary = self.root / ("probe-" + architecture)
        binary.write_bytes(content)
        return binary

    def test_separate_package_is_unprivileged_and_contains_matching_native_cgi(self):
        for architecture in ("amd64", "arm64"):
            with self.subTest(architecture=architecture):
                binary = self.binary(architecture)
                destination = BUILD.build(architecture, binary, self.root)
                with tarfile.open(destination) as package:
                    self.assertTrue(all(member.isfile() for member in package))
                    info = package.extractfile("INFO").read().decode()
                    self.assertIn('package="dockpipe-host-qualification"', info)
                    self.assertNotIn("install_dep_packages=", info)
                    self.assertEqual(json.load(package.extractfile("conf/privilege")),
                                     {"defaults": {"run-as": "package"}})
                    resource = json.load(package.extractfile("conf/resource"))
                    self.assertEqual(resource["systemd-user-unit"], {})
                    self.assertEqual(resource["web-config"]["nginx-static-config"]["enable"],
                                     [{"type": "dsm", "relpath": "nginx/report.conf"}])
                    unit = package.extractfile("conf/systemd/pkguser-dockpipe-host-qualification.service").read().decode()
                    self.assertIn("NoNewPrivileges=true", unit)
                    self.assertNotIn("User=root", unit)
                    payload = package.extractfile("package.tgz").read()
                    expected = hashlib.md5(payload, usedforsecurity=False).hexdigest()
                    self.assertIn(f'checksum="{expected}"', info)
                    with tarfile.open(fileobj=io.BytesIO(payload), mode="r:gz") as contents:
                        self.assertEqual(contents.extractfile("bin/host-probe").read(), binary.read_bytes())
                        self.assertEqual(contents.extractfile("ui/host-auth.cgi").read(), binary.read_bytes())
                        javascript = contents.extractfile("ui/Dockpipe.HostQualification.js").read().decode()
                        self.assertIn("dockpipe-host-qualification/host-auth.cgi", javascript)
                        self.assertNotIn("password", javascript.lower())
                        proxy = contents.extractfile("nginx/report.conf").read().decode()
                        self.assertIn("location = /webman/3rdparty/dockpipe-host-qualification/host-auth.cgi", proxy)
                        self.assertIn("proxy_pass http://127.0.0.1:38140/report;", proxy)
                        self.assertIn("proxy_set_header Cookie $http_cookie;", proxy)
                        self.assertIn("proxy_set_header X-Dockpipe-Remote-Addr $remote_addr;", proxy)
                    subprocess.run(["sh", "-n"], input=package.extractfile("scripts/start-stop-status").read(), check=True)

    def test_bad_architecture_and_linked_binary_preserve_previous_artifact(self):
        binary = self.binary("amd64")
        destination = BUILD.build("amd64", binary, self.root)
        previous = destination.read_bytes()
        with self.assertRaises(ValueError):
            BUILD.build("arm64", binary, self.root)
        link = self.root / "linked-binary"
        link.symlink_to(binary)
        with self.assertRaises(ValueError):
            BUILD.build("amd64", link, self.root)
        self.assertEqual(previous, destination.read_bytes())

    def test_archive_is_reproducible_and_receipt_matches_artifact(self):
        binary = self.binary("amd64")
        destination = BUILD.build("amd64", binary, self.root)
        previous = destination.read_bytes()
        binary.touch()
        binary.chmod(0o700)
        BUILD.build("amd64", binary, self.root)
        self.assertEqual(previous, destination.read_bytes())
        receipt = json.loads(destination.with_suffix(".json").read_text())
        self.assertEqual(hashlib.sha256(previous).hexdigest(), receipt["sha256"])
        self.assertEqual("pending", receipt["nas_acceptance"])


if __name__ == "__main__":
    unittest.main()
