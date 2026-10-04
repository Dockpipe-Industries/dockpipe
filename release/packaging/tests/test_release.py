#!/usr/bin/env python3
"""Release integrity, publication ordering, and actual APT signature/index checks."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

PACKAGING = Path(__file__).resolve().parents[1]


def load_module(name):
    spec = importlib.util.spec_from_file_location(name, PACKAGING / f"{name}.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


artifacts_module = load_module("release-artifacts")
publisher = load_module("publish-r2")


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="dockpipe-release-test-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)

    def store(self, directory):
        directory.mkdir(parents=True)
        entries = []
        for name in ("core", "workflow", "resolver"):
            tarball = f"{name}.tar.gz"
            (directory / tarball).write_bytes(name.encode())
            entries.append({"tarball": tarball, "sha256": hashlib.sha256(name.encode()).hexdigest()})
        (directory / "packages-store-manifest.json").write_text(json.dumps({"packages": {
            "core": entries[0], "workflows": [entries[1]], "resolvers": [entries[2]],
        }}))

    def test_embedded_inputs_follow_working_tree_removals(self):
        inputs = load_module("embedded-inputs")
        package = self.root / "packages/example"
        package.mkdir(parents=True)
        (package / "current.txt").write_text("current input")
        tracked = b"packages/example/current.txt\0packages/example/removed.txt\0"
        deleted = b"packages/example/removed.txt\0"
        with patch.object(inputs.subprocess, "check_output", side_effect=[tracked, deleted]):
            self.assertEqual(inputs.authored(self.root), ["packages/example/current.txt"])
        # Missing inputs must still fail when Git did not identify them as deletions.
        with patch.object(inputs.subprocess, "check_output", side_effect=[tracked, b""]):
            with self.assertRaises(ValueError):
                inputs.authored(self.root)

    def test_store_rejects_corruption_and_missing_categories(self):
        self.store(self.root / "store")
        self.assertEqual(artifacts_module.verify_store(self.root / "store"), 3)
        (self.root / "store/core.tar.gz").write_bytes(b"corrupt")
        with self.assertRaises(ValueError):
            artifacts_module.verify_store(self.root / "store")
        (self.root / "store/packages-store-manifest.json").write_text('{"packages": {"core": {}}}')
        with self.assertRaises(ValueError):
            artifacts_module.verify_store(self.root / "store")

    def test_catalog_requires_every_platform(self):
        with self.assertRaises(FileNotFoundError):
            artifacts_module.prepare(self.root, "0.6.0")

    def test_publish_commits_signed_index_after_payloads(self):
        artifacts = self.root / "artifacts"
        artifacts.mkdir()
        (artifacts / "release-manifest.json").write_text('{}')
        (artifacts / "dockpipe.tar.gz").write_bytes(b"payload")
        apt = self.root / "apt"
        (apt / "dists/stable").mkdir(parents=True)
        for name in ("InRelease", "Release", "Release.gpg", "Packages"):
            (apt / "dists/stable" / name).write_bytes(b"metadata")
        calls = []
        environment = {"DOCKPIPE_RELEASE_BUCKET": "test-packages", "R2_ENDPOINT_URL": "https://example.invalid", "R2_PREFIX": "packages"}
        with patch.dict(os.environ, environment), patch.object(publisher, "upload", side_effect=lambda *args: calls.append(args)), patch.object(publisher, "require_unpublished"):
            publisher.publish(artifacts, apt, "0.6.0", False)
        keys = [call[1] for call in calls]
        self.assertEqual(keys[-1], "packages/latest.json")
        self.assertEqual(keys[-2], "packages/releases/0.6.0/release-manifest.json")
        self.assertEqual(keys[-3], "apt/dists/stable/InRelease")
        self.assertLess(keys.index("apt/dists/stable/Packages"), keys.index("apt/dists/stable/Release"))
        (apt / "dists/stable/InRelease").unlink()
        with patch.dict(os.environ, environment), patch.object(publisher, "upload") as upload:
            with self.assertRaises(ValueError):
                publisher.publish(artifacts, apt, "0.6.0", False)
            upload.assert_not_called()

    def test_publisher_refuses_existing_versions_and_authentication_failures(self):
        for returncode, stderr in ((0, ""), (1, "AccessDenied")):
            response = subprocess.CompletedProcess([], returncode, stdout="", stderr=stderr)
            with patch.object(publisher.subprocess, "run", return_value=response):
                with self.assertRaises(ValueError):
                    publisher.require_unpublished("bucket", "https://example.invalid", "manifest.json")
        missing = subprocess.CompletedProcess([], 1, stdout="", stderr="An error occurred (404) when calling HeadObject")
        with patch.object(publisher.subprocess, "run", return_value=missing):
            publisher.require_unpublished("bucket", "https://example.invalid", "manifest.json")

    def test_direct_installer_checks_hashes_before_installing(self):
        import io
        import tarfile

        downloads = self.root / "downloads"
        downloads.mkdir()
        tools = self.root / "tools"
        tools.mkdir()
        fetcher = tools / "curl"
        fetcher.write_text("#!/usr/bin/env python3\nimport os, pathlib, sys\n"
                           "sys.stdout.buffer.write((pathlib.Path(os.environ['TEST_DOWNLOADS']) / sys.argv[-1].split('/')[-1]).read_bytes())\n")
        fetcher.chmod(0o755)
        uname = tools / "uname"
        uname.write_text("#!/bin/sh\nif [ \"$1\" = -m ]; then echo arm64; else echo \"${TEST_OS:-Darwin}\"; fi\n")
        uname.chmod(0o755)
        archive = downloads / "dockpipe_0.6.0_darwin_arm64.tar.gz"
        with tarfile.open(archive, "w:gz") as output:
            payload = b"#!/bin/sh\necho dockpipe-test\n"
            member = tarfile.TarInfo("dockpipe")
            member.size = len(payload)
            member.mode = 0o755
            output.addfile(member, io.BytesIO(payload))
        core = downloads / "dockpipe-core-0.6.0.tar.gz"
        core.write_bytes(b"test-core")
        linux_archive = downloads / "dockpipe_0.6.0_linux_arm64.tar.gz"
        shutil.copyfile(archive, linux_archive)
        checksums = "".join(f"{artifacts_module.digest(path)}  {path.name}\n" for path in (archive, linux_archive, core))
        (downloads / "SHA256SUMS.txt").write_text(checksums)
        env = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ["PATH"],
                   TEST_DOWNLOADS=str(downloads), DOCKPIPE_VERSION="0.6.0", DOCKPIPE_INSTALL_MODE="portable",
                   DOCKPIPE_INSTALL_DIR=str(self.root / "installed"), DOCKPIPE_GLOBAL_ROOT=str(self.root / "data"))
        command = ["sh", str(PACKAGING / "linux/install.sh")]
        original = archive.read_bytes()
        archive.write_bytes(b"corrupted-download")
        result = subprocess.run(command, env=env, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Checksum mismatch", result.stderr)
        self.assertFalse((self.root / "installed/dockpipe").exists())
        archive.write_bytes(original)
        subprocess.run(command, env=env, check=True, capture_output=True)
        self.assertTrue((self.root / "installed/dockpipe").is_file())
        self.assertEqual((self.root / "data/packages/core" / core.name).read_bytes(), b"test-core")
        subprocess.run(command, env=dict(env, TEST_OS="Linux"), check=True, capture_output=True)

    def test_release_setup_copies_only_scoped_values_and_defaults_to_checks(self):
        source = PACKAGING.parents[1] / "workflows/package/package-release-setup/assets/scripts/configure-release.py"
        spec = importlib.util.spec_from_file_location("release_setup", source)
        setup = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(setup)
        values = {"AWS_ACCESS_KEY_ID": "test-id", "AWS_SECRET_ACCESS_KEY": "test-key", "APT_SIGNING_KEY": "test-signing-key\nsecond-line",
                  "R2_ENDPOINT_URL": "https://example.invalid", "DOCKPIPE_RELEASE_BUCKET": "dockpipe", "R2_PREFIX": "packages",
                  "CLOUDFLARE_API_TOKEN": "must-not-copy", "R2_STATE_SECRET_ACCESS_KEY": "must-not-copy"}
        with patch.dict(os.environ, values), patch.object(setup, "signing_fingerprint", return_value="A" * 40), patch.object(setup, "run") as run:
            setup.configure(False)
            run.assert_not_called()
            setup.configure(True)
            secrets = [call for call in run.call_args_list if call.args[0][1] == "secret"]
            self.assertEqual({call.args[0][3] for call in secrets}, set(setup.SECRETS))
            for call in secrets:
                self.assertEqual(call.kwargs["value"], values[call.args[0][3]])
                self.assertNotIn(call.kwargs["value"], call.args[0])
            self.assertEqual(len(run.call_args_list), 7)

    def test_release_setup_repairs_only_valid_flattened_armor(self):
        source = PACKAGING.parents[1] / "workflows/package/package-release-setup/assets/scripts/configure-release.py"
        spec = importlib.util.spec_from_file_location("release_setup", source)
        setup = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(setup)
        armor = "-----BEGIN PGP PRIVATE KEY BLOCK-----\n\nYWJjZA==\n=YWJj\n-----END PGP PRIVATE KEY BLOCK-----\n"
        self.assertEqual(setup.normalize_signing_key(armor.replace("\n", " ")), armor)
        self.assertEqual(setup.normalize_signing_key(armor), armor.strip())
        for invalid in ("filename.asc", armor.replace("\n", " ").replace("YWJjZA==", "bad!")):
            with self.assertRaises(ValueError):
                setup.normalize_signing_key(invalid)

    @unittest.skipUnless(all(shutil.which(tool) for tool in ("gpg", "gpgv", "apt-ftparchive", "dpkg-deb")), "APT tools required")
    def test_signed_apt_repository_is_readable_and_by_hash(self):
        gpg_home = self.root / "gnupg"
        gpg_home.mkdir(mode=0o700)
        env = dict(os.environ, GNUPGHOME=str(gpg_home))
        subprocess.run(["gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase", "",
                        "--quick-generate-key", "DockPipe test only", "rsa2048", "sign", "1d"],
                       env=env, check=True, capture_output=True)
        listing = subprocess.check_output(["gpg", "--with-colons", "--list-secret-keys"], env=env, text=True)
        fingerprint = next(line.split(":")[9] for line in listing.splitlines() if line.startswith("fpr:"))
        env["APT_SIGNING_FINGERPRINT"] = fingerprint
        artifacts = self.root / "artifacts"
        artifacts.mkdir()
        for arch in ("amd64", "arm64"):
            package = self.root / arch
            (package / "DEBIAN").mkdir(parents=True)
            (package / "DEBIAN/control").write_text(
                f"Package: dockpipe\nVersion: 0.6.0\nArchitecture: {arch}\n"
                "Maintainer: Test <test@example.invalid>\nDescription: Test only\n"
            )
            subprocess.run(["dpkg-deb", "--build", str(package), str(artifacts / f"dockpipe_0.6.0_{arch}.deb")],
                           check=True, capture_output=True)
        apt = self.root / "apt"
        subprocess.run(["bash", str(PACKAGING / "build-apt.sh"), str(artifacts), str(apt)], env=env, check=True, capture_output=True)
        for arch in ("amd64", "arm64"):
            index = apt / "dists/stable/main" / f"binary-{arch}"
            packages = (index / "Packages").read_bytes()
            self.assertIn(f"Architecture: {arch}".encode(), packages)
            self.assertEqual((index / "by-hash/SHA256" / hashlib.sha256(packages).hexdigest()).read_bytes(), packages)
        subprocess.run(["gpgv", "--keyring", str(apt / "dockpipe-archive-keyring.gpg"), str(apt / "dists/stable/InRelease")],
                       check=True, capture_output=True)
        # Exercise APT's real trust and index reader without changing system sources or installing.
        lists = self.root / "lists"
        (lists / "partial").mkdir(parents=True)
        source = self.root / "dockpipe.list"
        source.write_text(f"deb [signed-by={apt}/dockpipe-archive-keyring.gpg] file:{apt} stable main\n")
        subprocess.run(["apt-get", "-o", f"Dir::Etc::sourcelist={source}", "-o", "Dir::Etc::sourceparts=-",
                        "-o", f"Dir::State::lists={lists}", "-o", "APT::Get::List-Cleanup=0", "update"],
                       check=True, capture_output=True)


if __name__ == "__main__":
    unittest.main()
