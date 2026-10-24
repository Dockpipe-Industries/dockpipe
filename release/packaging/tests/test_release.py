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

    def store(self, directory, core_only=False):
        directory.mkdir(parents=True)
        entries = []
        package_names = ("core",) if core_only else ("core", "workflow", "resolver")
        for name in package_names:
            tarball = f"{name}.tar.gz"
            (directory / tarball).write_bytes(name.encode())
            entries.append({"tarball": tarball, "sha256": hashlib.sha256(name.encode()).hexdigest()})
        (directory / "packages-store-manifest.json").write_text(json.dumps({"packages": {
            "core": entries[0],
            "workflows": [] if core_only else [entries[1]],
            "resolvers": [] if core_only else [entries[2]],
        }}))

    def test_embedded_inputs_follow_working_tree_removals(self):
        inputs = load_module("embedded-inputs")
        workflow = self.root / "workflows/example"
        workflow.mkdir(parents=True)
        (workflow / "current.txt").write_text("current input")
        tracked = b"workflows/example/current.txt\0workflows/example/removed.txt\0"
        deleted = b"workflows/example/removed.txt\0"
        with patch.object(inputs.subprocess, "check_output", side_effect=[tracked, deleted]):
            self.assertEqual(inputs.authored(self.root), ["workflows/example/current.txt"])
        # Missing inputs must still fail when Git did not identify them as deletions.
        with patch.object(inputs.subprocess, "check_output", side_effect=[tracked, b""]):
            with self.assertRaises(ValueError):
                inputs.authored(self.root)

    def test_embedded_inputs_exclude_all_package_sources(self):
        inputs = load_module("embedded-inputs")
        package_names = ("example", "cockpit", "native-sdk", "pipelang", "future-package")
        tracked = []
        for name in package_names:
            relative = f"packages/{name}/asset.txt"
            path = self.root / relative
            path.parent.mkdir(parents=True)
            path.write_text("external package input")
            tracked.append(relative)
        encoded = ("\0".join(tracked) + "\0").encode()
        with patch.object(inputs.subprocess, "check_output", side_effect=[encoded, b""]):
            self.assertEqual(inputs.authored(self.root), [])

    def test_store_rejects_corruption_and_missing_categories(self):
        self.store(self.root / "store")
        self.assertEqual(artifacts_module.verify_store(self.root / "store"), 3)
        (self.root / "store/core.tar.gz").write_bytes(b"corrupt")
        with self.assertRaises(ValueError):
            artifacts_module.verify_store(self.root / "store")
        (self.root / "store/packages-store-manifest.json").write_text('{"packages": {"core": {}}}')
        with self.assertRaises(ValueError):
            artifacts_module.verify_store(self.root / "store")

    def test_installer_stages_only_verified_core_and_preserves_full_store(self):
        store = self.root / "store"
        self.store(store)
        core_name = "dockpipe-core-0.6.0.tar.gz"
        (store / "core.tar.gz").rename(store / core_name)
        manifest = json.loads((store / "packages-store-manifest.json").read_text())
        manifest["packages"]["core"]["tarball"] = core_name
        (store / "packages-store-manifest.json").write_text(json.dumps(manifest))
        before = {path.name: path.read_bytes() for path in store.iterdir()}
        destination = self.root / "app/Contents/Resources/share/dockpipe"
        artifacts_module.stage_core(store, destination)
        files = [path.relative_to(destination).as_posix() for path in destination.rglob("*") if path.is_file()]
        self.assertEqual(files, [f"packages/core/{core_name}"])
        self.assertEqual((destination / files[0]).read_bytes(), b"core")
        self.assertEqual(before, {path.name: path.read_bytes() for path in store.iterdir()})
        self.assertEqual(artifacts_module.verify_store(store), 3)
        # A reused output must fail rather than retaining old optional packages.
        with self.assertRaises(FileExistsError):
            artifacts_module.stage_core(store, destination)
        (store / core_name).write_bytes(b"corrupt")
        with self.assertRaisesRegex(ValueError, "checksum mismatch"):
            artifacts_module.stage_core(store, self.root / "bad-payload")
        self.assertFalse((self.root / "bad-payload").exists())
        manifest["packages"]["core"]["tarball"] = "../core.tar.gz"
        (store / "packages-store-manifest.json").write_text(json.dumps(manifest))
        with self.assertRaisesRegex(ValueError, "Invalid core filename"):
            artifacts_module.stage_core(store, self.root / "bad-payload")

    def test_runtime_store_accepts_core_and_rejects_optional_or_corrupt_inputs(self):
        store = self.root / "runtime-store"
        self.store(store, core_only=True)
        self.assertEqual(artifacts_module.verify_store(store, core_only=True), 1)
        stray = store / "undeclared-optional.tar.gz"
        stray.write_bytes(b"optional payload")
        with self.assertRaisesRegex(ValueError, "Unexpected runtime store payload"):
            artifacts_module.verify_store(store, core_only=True)
        stray.unlink()
        manifest_path = store / "packages-store-manifest.json"
        manifest = json.loads(manifest_path.read_text())
        manifest["packages"]["workflows"] = [{"tarball": "unavailable-optional.tar.gz"}]
        manifest_path.write_text(json.dumps(manifest))
        with self.assertRaisesRegex(ValueError, "contains optional packages"):
            artifacts_module.verify_store(store, core_only=True)
        manifest["packages"]["workflows"] = []
        manifest_path.write_text(json.dumps(manifest))
        (store / "core.tar.gz").write_bytes(b"corrupt")
        with self.assertRaisesRegex(ValueError, "checksum mismatch"):
            artifacts_module.verify_store(store, core_only=True)

    def test_catalog_requires_every_platform(self):
        with self.assertRaises(FileNotFoundError):
            artifacts_module.prepare(self.root, "0.6.0")

    def test_flatpak_catalog_requires_matching_runtime_and_bundle(self):
        platform = artifacts_module.FLATPAK_PLATFORM
        store = self.root / "stores" / platform
        self.store(store, core_only=True)
        manifest_path = store / "packages-store-manifest.json"
        manifest = json.loads(manifest_path.read_text())
        bundle = "dockpipe-desktop_0.6.2_linux_amd64.flatpak"
        with patch.object(artifacts_module, "PLATFORMS", ()):
            with self.assertRaisesRegex(ValueError, "tested desktop bundle"):
                artifacts_module.prepare(self.root, "0.6.2")
            (self.root / bundle).write_bytes(b"qualified bundle fixture")
            manifest["platform"] = "linux-amd64"
            manifest_path.write_text(json.dumps(manifest))
            with self.assertRaisesRegex(ValueError, "incorrect runtime target"):
                artifacts_module.prepare(self.root, "0.6.2")
            manifest["platform"] = platform
            manifest_path.write_text(json.dumps(manifest))
            artifacts_module.prepare(self.root, "0.6.2")
        catalog = json.loads((self.root / "release-manifest.json").read_text())
        self.assertEqual(catalog["stores"][platform]["count"], 1)
        self.assertEqual(catalog["downloads"][platform], {"cli": bundle, "desktop": bundle})
        self.assertIn(bundle, (self.root / "SHA256SUMS.txt").read_text())

    def test_candidate_catalog_records_provenance_before_checksums(self):
        version = "0.6.0"
        candidate = version + "-staging.123.2." + "a" * 12
        for platform in artifacts_module.PLATFORMS:
            self.store(self.root / "stores" / platform, core_only=True)
            suffix = "zip" if platform.startswith("windows") else "tar.gz"
            names = [f"dockpipe_{version}_{platform.replace('-', '_')}.{suffix}",
                     f"dockpipe-packages_{version}_{platform}.tar.gz"]
            if platform.startswith("linux"):
                arch = platform.split("-")[1]
                names += [f"dockpipe_{version}_{arch}.deb", f"dockpipe-desktop_{version}_{arch}.deb"]
                names += [f"dockpipe_{version}_linux_{arch}.{ext}" for ext in ("rpm", "apk", "pkg.tar.zst")]
            elif platform.startswith("darwin"):
                names += [f"dockpipe-desktop_{version}_{platform.replace('-', '_')}.{extension}"
                          for extension in ("dmg", "zip")]
            for name in names:
                (self.root / name).write_bytes(b"fixture")
        artifacts_module.prepare(self.root, version, candidate, "a" * 40)
        catalog = json.loads((self.root / "release-manifest.json").read_text())
        self.assertEqual(catalog["candidate"], candidate)
        self.assertEqual(catalog["source_sha"], "a" * 40)
        self.assertEqual(catalog["channel"], "staging")
        self.assertEqual(catalog["artifact_class"], "runtime")
        self.assertTrue(all(store["count"] == 1 for store in catalog["stores"].values()))
        expected = artifacts_module.digest(self.root / "release-manifest.json")
        self.assertIn(expected + "  release-manifest.json", (self.root / "SHA256SUMS.txt").read_text())
        with self.assertRaises(ValueError):
            artifacts_module.prepare(self.root, version, candidate, "b" * 40)
        self.assertEqual(catalog["downloads"]["darwin-arm64"]["desktop"], "dockpipe-desktop_0.6.0_darwin_arm64.dmg")
        (self.root / "dockpipe-desktop_0.6.0_darwin_arm64.dmg").unlink()
        with self.assertRaisesRegex(ValueError, "Missing release artifact"):
            artifacts_module.prepare(self.root, version, candidate, "a" * 40)

    def test_publish_commits_signed_index_after_payloads(self):
        artifacts = self.root / "artifacts"
        artifacts.mkdir()
        (artifacts / "release-manifest.json").write_text('{"artifact_class": "runtime"}')
        (artifacts / "dockpipe.tar.gz").write_bytes(b"payload")
        apt = self.root / "apt"
        (apt / "dists/stable").mkdir(parents=True)
        for name in ("InRelease", "Release", "Release.gpg", "Packages"):
            (apt / "dists/stable" / name).write_bytes(b"metadata")
        calls = []
        environment = {"DOCKPIPE_RELEASE_CHANNEL": "stable", "DOCKPIPE_RELEASE_BUCKET": "test-packages", "R2_ENDPOINT_URL": "https://example.invalid", "R2_PREFIX": "packages"}
        with patch.dict(os.environ, environment), patch.object(publisher, "upload", side_effect=lambda *args: calls.append(args)), patch.object(publisher, "require_forward_version"), patch.object(publisher, "require_unpublished"):
            publisher.publish(artifacts, apt, "0.6.0", False)
        keys = [call[1] for call in calls]
        self.assertEqual(keys[-1], "packages/runtime/latest.json")
        self.assertNotIn("packages/latest.json", keys)
        self.assertEqual(keys[-2], "packages/releases/0.6.0/release-manifest.json")
        self.assertEqual(keys[-3], "apt/dists/stable/InRelease")
        self.assertLess(keys.index("apt/dists/stable/Packages"), keys.index("apt/dists/stable/Release"))
        (apt / "dists/stable/InRelease").unlink()
        with patch.dict(os.environ, environment), patch.object(publisher, "upload") as upload:
            with self.assertRaises(ValueError):
                publisher.publish(artifacts, apt, "0.6.0", False)
            upload.assert_not_called()

    def test_staging_publication_is_isolated_and_commits_pointer_last(self):
        artifacts = self.root / "artifacts"
        artifacts.mkdir()
        candidate = "0.6.0-staging.123.1." + "a" * 12
        catalog = {"artifact_class": "runtime", "version": "0.6.0", "channel": "staging",
                   "candidate": candidate, "source_sha": "a" * 40}
        (artifacts / "release-manifest.json").write_text(json.dumps(catalog))
        (artifacts / "payload.tar.gz").write_bytes(b"candidate")
        apt = self.root / "apt"
        (apt / "dists/staging").mkdir(parents=True)
        (apt / "dists/staging/InRelease").write_bytes(b"signed")
        (apt / "pool").mkdir()
        (apt / "pool/candidate.deb").write_bytes(b"deb")
        environment = {"DOCKPIPE_RELEASE_CHANNEL": "staging", "DOCKPIPE_RELEASE_BUCKET": "example-staging", "R2_ENDPOINT_URL": "https://example.invalid", "R2_PREFIX": "packages"}
        with patch.dict(os.environ, environment), patch.object(publisher, "upload") as upload, patch.object(publisher, "require_forward_version"), patch.object(publisher, "require_unpublished") as guard:
            publisher.publish(artifacts, apt, "0.6.0", False, candidate)
            prefix = f"packages/candidates/{candidate}"
            keys = [call.args[1] for call in upload.call_args_list]
            self.assertTrue(all(key.startswith((prefix + "/", "apt/")) for key in keys[:-1]))
            self.assertEqual(keys[-2:], ["apt/dists/staging/InRelease", "packages/runtime/latest.json"])
            self.assertNotIn("packages/latest.json", keys)
            self.assertLess(keys.index(prefix + "/release-manifest.json"), keys.index("apt/pool/candidate.deb"))
            self.assertLess(keys.index("apt/pool/candidate.deb"), keys.index("apt/dists/staging/InRelease"))
            self.assertIn(prefix + "/apt/pool/candidate.deb", keys)
            self.assertEqual(guard.call_args.args[2], prefix + "/release-manifest.json")
            pointer = json.loads((self.root / "latest.json").read_text())
            self.assertEqual(pointer["candidate"], candidate)
            upload.reset_mock()
            guard.side_effect = ValueError("already published")
            with self.assertRaises(ValueError):
                publisher.publish(artifacts, apt, "0.6.0", False, candidate)
            upload.assert_not_called()
        for channel, identity in (("stable", candidate), ("staging", ""), ("staging", "../escape"), ("", candidate)):
            with patch.dict(os.environ, dict(environment, DOCKPIPE_RELEASE_CHANNEL=channel)), patch.object(publisher, "upload") as upload:
                with self.assertRaises(ValueError):
                    publisher.publish(artifacts, apt, "0.6.0", False, identity)
                upload.assert_not_called()
        catalog["source_sha"] = "b" * 40
        (artifacts / "release-manifest.json").write_text(json.dumps(catalog))
        with patch.dict(os.environ, environment), patch.object(publisher, "upload") as upload:
            with self.assertRaises(ValueError):
                publisher.publish(artifacts, apt, "0.6.0", False, candidate)
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

    def test_publisher_requires_a_strictly_newer_native_version(self):
        for old in ("0.6.2", "0.6.10", "0.7.0", "invalid"):
            response = subprocess.CompletedProcess([], 0, stdout=json.dumps({"version": old}))
            with self.subTest(old=old), patch.object(publisher.subprocess, "run", return_value=response):
                with self.assertRaises(ValueError):
                    publisher.require_forward_version("bucket", "https://example.invalid", "packages/latest.json", "0.6.2")
        response = subprocess.CompletedProcess([], 0, stdout='{"version":"0.6.9"}')
        with patch.object(publisher.subprocess, "run", return_value=response):
            publisher.require_forward_version("bucket", "https://example.invalid", "packages/latest.json", "0.6.10")
        for error in ("AccessDenied", "connection failed"):
            response = subprocess.CompletedProcess([], 1, stdout="", stderr=error)
            with patch.object(publisher.subprocess, "run", return_value=response), self.assertRaises(ValueError):
                publisher.require_forward_version("bucket", "https://example.invalid", "packages/latest.json", "0.6.2")

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

    @unittest.skipUnless(all(shutil.which(tool) for tool in ("gpg", "gpgv", "apt-ftparchive", "dpkg-deb")), "APT tools required")
    def test_signed_apt_repository_is_readable_and_by_hash(self):
        gpg_home = self.root / "gnupg"
        gpg_home.mkdir(mode=0o700)
        env = dict(os.environ, GNUPGHOME=str(gpg_home))
        self.addCleanup(subprocess.run, ["gpgconf", "--kill", "gpg-agent"],
                        env=env, capture_output=True)
        subprocess.run(["gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase", "",
                        "--quick-generate-key", "Dockpipe test only", "rsa2048", "sign", "1d"],
                       env=env, check=True, capture_output=True)
        listing = subprocess.check_output(["gpg", "--with-colons", "--list-secret-keys"], env=env, text=True)
        fingerprint = next(line.split(":")[9] for line in listing.splitlines() if line.startswith("fpr:"))
        env["APT_SIGNING_FINGERPRINT"] = fingerprint
        artifacts = self.root / "artifacts"
        artifacts.mkdir()
        for arch in ("amd64", "arm64"):
            for name in ("dockpipe", "dockpipe-desktop"):
                package = self.root / arch / name
                (package / "DEBIAN").mkdir(parents=True)
                (package / "DEBIAN/control").write_text(
                    f"Package: {name}\nVersion: 0.6.0\nArchitecture: {arch}\n"
                    "Maintainer: Test <test@example.invalid>\nDescription: Test only\n"
                )
                subprocess.run(["dpkg-deb", "--build", str(package), str(artifacts / f"{name}_0.6.0_{arch}.deb")],
                               check=True, capture_output=True)
        apt = self.root / "apt"
        subprocess.run(["bash", str(PACKAGING / "build-apt.sh"), str(artifacts), str(apt)], env=env, check=True, capture_output=True)
        for arch in ("amd64", "arm64"):
            index = apt / "dists/stable/main" / f"binary-{arch}"
            packages = (index / "Packages").read_bytes()
            self.assertIn(f"Architecture: {arch}".encode(), packages)
            self.assertIn(b"Package: dockpipe-desktop", packages)
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

        staged = self.root / "staging-apt"
        subprocess.run(["bash", str(PACKAGING / "build-apt.sh"), str(artifacts), str(staged)],
                       env=dict(env, APT_SUITE="staging"), check=True, capture_output=True)
        subprocess.run(["gpgv", "--keyring", str(staged / "dockpipe-archive-keyring.gpg"),
                        str(staged / "dists/staging/InRelease")], check=True, capture_output=True)
        self.assertIn("Suite: staging", (staged / "dists/staging/Release").read_text())
        source.write_text(f"deb [signed-by={staged}/dockpipe-archive-keyring.gpg] file:{staged} staging main\n")
        subprocess.run(["apt-get", "-o", f"Dir::Etc::sourcelist={source}", "-o", "Dir::Etc::sourceparts=-",
                        "-o", f"Dir::State::lists={lists}", "-o", "APT::Get::List-Cleanup=0", "update"],
                       check=True, capture_output=True)

        # Simulate installed old packages in a private dpkg status file. Advance the
        # same signed repository URL and ask APT itself to resolve the paired upgrade.
        status = self.root / "installed-status"
        status.write_text("\n".join(
            f"Package: {name}\nStatus: install ok installed\nArchitecture: amd64\n"
            "Version: 0.6.0\nMaintainer: Test <test@example.invalid>\nDescription: Test only\n"
            for name in ("dockpipe", "dockpipe-desktop")))
        for arch in ("amd64", "arm64"):
            for name in ("dockpipe", "dockpipe-desktop"):
                package = self.root / arch / name
                control = package / "DEBIAN/control"
                body = control.read_text().replace("Version: 0.6.0", "Version: 0.6.1")
                if name == "dockpipe-desktop":
                    body += "Depends: dockpipe (= 0.6.1)\n"
                control.write_text(body)
                (artifacts / f"{name}_0.6.0_{arch}.deb").unlink()
                subprocess.run(["dpkg-deb", "--build", str(package), str(artifacts / f"{name}_0.6.1_{arch}.deb")],
                               check=True, capture_output=True)
        next_apt = self.root / "next-apt"
        subprocess.run(["bash", str(PACKAGING / "build-apt.sh"), str(artifacts), str(next_apt)],
                       env=dict(env, APT_SUITE="staging"), check=True, capture_output=True)
        shutil.copytree(next_apt, staged, dirs_exist_ok=True)
        apt_options = ["-o", f"Dir::Etc::sourcelist={source}", "-o", "Dir::Etc::sourceparts=-",
                       "-o", f"Dir::State::lists={lists}", "-o", f"Dir::State::status={status}",
                       "-o", "APT::Architecture=amd64", "-o", "Debug::NoLocking=1"]
        subprocess.run(["apt-get", *apt_options, "update"], check=True, capture_output=True)
        upgrade = subprocess.check_output(["apt-get", *apt_options, "--simulate", "upgrade"], text=True)
        self.assertIn("Inst dockpipe [0.6.0] (0.6.1", upgrade)
        self.assertIn("Inst dockpipe-desktop [0.6.0] (0.6.1", upgrade)
        self.assertIn("2 upgraded, 0 newly installed, 0 to remove", upgrade)



if __name__ == "__main__":
    unittest.main()
