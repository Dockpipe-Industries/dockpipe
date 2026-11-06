"""Check the companion's explicit capability and separate-package boundaries."""

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


SCRIPT = Path(__file__).resolve().parents[1] / "assets/scripts/build_identity.py"
specification = importlib.util.spec_from_file_location("identity_qualification_packaging", SCRIPT)
BUILD = importlib.util.module_from_spec(specification)
specification.loader.exec_module(BUILD)


class IdentityPackagingTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        content = bytearray(120)
        content[:7] = b"\x7fELF\x02\x01\x01"
        struct.pack_into("<HH", content, 16, 2, BUILD.PACKAGING.ARCHITECTURES["amd64"][2])
        struct.pack_into("<Q", content, 32, 64)
        struct.pack_into("<HH", content, 54, 56, 1)
        struct.pack_into("<I", content, 64, 1)
        self.binary = self.root / "identity-cap-helper"
        self.binary.write_bytes(content)

    def test_only_daemon_receives_the_explicit_capability_pair(self):
        destination = BUILD.build("amd64", self.binary, self.root)
        with tarfile.open(destination) as package:
            info = package.extractfile("INFO").read().decode()
            self.assertIn('package="dockpipe-host-identity-qualification"', info)
            self.assertIn('version="0.1.0-3"', info)
            self.assertNotIn("install_dep_packages", info)
            self.assertEqual(json.load(package.extractfile("conf/privilege")), {
                "defaults": {"run-as": "package"},
                "tool": [{"relpath": "bin/identity-cap-helper", "user": "package", "group": "package",
                          "permission": "0700", "capabilities": "cap_setuid,cap_setgid"}],
            })
            unit = package.extractfile("conf/systemd/" + BUILD.UNIT).read().decode()
            self.assertIn("NoNewPrivileges=false", unit)
            self.assertNotIn("User=root", unit)
            self.assertNotIn("CAP_KILL", unit)
            control = package.extractfile("scripts/start-stop-status").read()
            subprocess.run(["sh", "-n"], input=control, check=True)
            self.assertIn(b"https://127.0.0.1:38141/health", control)
            self.assertIn(b'--cacert "$root/var/endpoint-public/ca.pem"', control)
            self.assertNotIn(b"--insecure", control)
            self.assertIn(b'"$root/target/bin/identity-endpoint-init"', control)
            payload = package.extractfile("package.tgz").read()
            self.assertIn(f'checksum="{hashlib.md5(payload, usedforsecurity=False).hexdigest()}"', info)
            with tarfile.open(fileobj=io.BytesIO(payload), mode="r:gz") as contents:
                member = contents.getmember("bin/identity-cap-helper")
                self.assertTrue(member.isfile())
                self.assertFalse(member.islnk())
                self.assertEqual(member.mode, 0o700)
                self.assertEqual(contents.extractfile(member).read(), self.binary.read_bytes())
                self.assertNotIn("bin/identity-worker", contents.getnames())
                self.assertNotIn("bin/identity-probe", contents.getnames())
                initializer = contents.getmember("bin/identity-endpoint-init")
                self.assertEqual(initializer.mode, 0o700)
                self.assertEqual(contents.extractfile(initializer).read(), self.binary.read_bytes())
                self.assertFalse(any(name.endswith((".pem", ".key")) for name in contents.getnames()))
                javascript = contents.extractfile("ui/Dockpipe.HostIdentityQualification.js").read().decode()
                self.assertIn("dockpipe-host-identity-qualification/host-auth.cgi", javascript)
                self.assertIn("dockpipe.synology_host_identity_qualification.v1", javascript)
                self.assertIn('"X-Syno-Token": sessionToken', javascript)
                self.assertNotIn("dockpipe-host-qualification/", javascript)
                proxy = contents.extractfile("nginx/report.conf").read().decode()
                self.assertIn("proxy_pass https://127.0.0.1:38141/report;", proxy)
                self.assertIn("proxy_ssl_verify on;", proxy)
                self.assertIn("proxy_ssl_name localhost;", proxy)
                self.assertIn("proxy_ssl_trusted_certificate /var/packages/dockpipe-host-identity-qualification/var/endpoint-public/ca.pem;", proxy)
                self.assertIn("access_log off;", proxy)

    def test_reproducibility_receipt_and_failed_admission_preserve_artifact(self):
        destination = BUILD.build("amd64", self.binary, self.root)
        original = destination.read_bytes()
        BUILD.build("amd64", self.binary, self.root)
        self.assertEqual(original, destination.read_bytes())
        receipt = json.loads(destination.with_suffix(".json").read_text())
        self.assertEqual(receipt["sha256"], hashlib.sha256(original).hexdigest())
        self.assertEqual(receipt["nas_acceptance"], "pending")
        self.assertFalse(receipt["terminal_support"])
        with self.assertRaises(ValueError):
            BUILD.build("arm64", self.binary, self.root)
        self.assertEqual(original, destination.read_bytes())


if __name__ == "__main__":
    unittest.main()
