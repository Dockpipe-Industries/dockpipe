import importlib.util
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import unittest
from unittest import mock


SCRIPT = Path(__file__).resolve().parents[1] / "assets/scripts/stage.py"
SPEC = importlib.util.spec_from_file_location("synology_desktop_stage", SCRIPT)
stage = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(stage)


@unittest.skipUnless(shutil.which("dpkg-deb"), "dpkg-deb is required for real package fixtures")
class StageTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.output = self.root / "output"
        self.cli = self.package("dockpipe")
        self.desktop = self.package("dockpipe-desktop")

    def package(self, name, version="1.2.3", architecture="amd64", filename=None):
        source = self.root / f"source-{name}-{version}-{architecture}"
        control = source / "DEBIAN"
        control.mkdir(parents=True, exist_ok=True)
        (control / "control").write_text(
            f"Package: {name}\nVersion: {version}\nArchitecture: {architecture}\n"
            "Maintainer: Fixture <fixture@example.invalid>\nDescription: Test fixture\n"
        )
        destination = self.root / (filename or f"{name}-{version}-{architecture}.deb")
        subprocess.run(
            ["dpkg-deb", "--root-owner-group", "--build", str(source), str(destination)],
            check=True,
            capture_output=True,
            timeout=30,
        )
        return destination

    def bundle(self, **overrides):
        arguments = {
            "cli": self.cli,
            "desktop": self.desktop,
            "architecture": "amd64",
            "webtop_image": stage.DEFAULT_WEBTOP_IMAGE,
            "output_directory": self.output,
        }
        arguments.update(overrides)
        return stage.stage_bundle(**arguments)

    def test_archive_is_reproducible_and_contains_only_expected_files(self):
        bundle = self.bundle()
        first_bytes = bundle.read_bytes()
        self.assertEqual(bundle.name, "dockpipe-desktop_1.2.3_synology_amd64.tar.gz")
        self.assertEqual(self.bundle().read_bytes(), first_bytes)
        with tarfile.open(bundle) as archive:
            prefix = "dockpipe-desktop_1.2.3_synology_amd64/"
            expected = {*stage.TEXT_ASSETS, "packages/cli.deb", "packages/desktop.deb"}
            self.assertEqual(set(archive.getnames()), {prefix + member for member in expected})
            for member in archive.getmembers():
                self.assertEqual((member.uid, member.gid, member.mtime, member.mode), (0, 0, 0, 0o644))
                self.assertTrue(member.isfile())
            self.assertEqual(archive.extractfile(prefix + "packages/cli.deb").read(), self.cli.read_bytes())
            compose = archive.extractfile(prefix + "compose.yml").read().decode()
            self.assertIn(stage.DEFAULT_WEBTOP_IMAGE, compose)
            self.assertIn("platform: linux/amd64", compose)
            self.assertIn("${CUSTOM_USER:?", compose)
            self.assertIn("${PASSWORD:?", compose)
            self.assertIn("${BIND_ADDRESS:-127.0.0.1}", compose)
            self.assertIn("shm_size: 1gb", compose)
            self.assertNotIn("privileged:", compose)
            self.assertNotIn("docker.sock", compose)
            dockerfile = archive.extractfile(prefix + "Dockerfile").read().decode()
            self.assertIn("apt-get install -y --no-install-recommends /tmp/dockpipe/cli.deb /tmp/dockpipe/desktop.deb", dockerfile)
            self.assertNotIn("ENTRYPOINT", dockerfile)

    def test_arm64_is_explicit_in_archive_and_compose(self):
        cli = self.package("dockpipe", architecture="arm64")
        desktop = self.package("dockpipe-desktop", architecture="arm64")
        bundle = self.bundle(cli=cli, desktop=desktop, architecture="arm64")
        self.assertIn("synology_arm64.tar.gz", bundle.name)
        with tarfile.open(bundle) as archive:
            compose = next(member for member in archive.getmembers() if member.name.endswith("/compose.yml"))
            self.assertIn(b"platform: linux/arm64", archive.extractfile(compose).read())

    def test_rejects_unpinned_or_injected_base(self):
        for image in ("lscr.io/linuxserver/webtop:ubuntu-xfce", "", stage.DEFAULT_WEBTOP_IMAGE + "\nRUN false", "alpine@sha256:" + "a" * 64):
            with self.subTest(image=image), self.assertRaisesRegex(ValueError, "pinned"):
                self.bundle(webtop_image=image)
        self.assertFalse(self.output.exists())

    def test_rejects_wrong_package_name(self):
        wrong = self.package("other-package")
        with self.assertRaisesRegex(ValueError, "Expected DEB package dockpipe"):
            self.bundle(cli=wrong)
        self.assertFalse(self.output.exists())

    def test_rejects_mixed_versions(self):
        desktop = self.package("dockpipe-desktop", version="1.2.4")
        with self.assertRaisesRegex(ValueError, "versions must match"):
            self.bundle(desktop=desktop)
        self.assertFalse(self.output.exists())

    def test_rejects_mixed_architectures(self):
        desktop = self.package("dockpipe-desktop", architecture="arm64")
        with self.assertRaisesRegex(ValueError, "Expected amd64 DEB"):
            self.bundle(desktop=desktop)

    def test_arbitrary_input_filename_is_not_interpreted(self):
        unusual = self.package("dockpipe", filename="$(touch compromised).deb")
        self.assertTrue(self.bundle(cli=unusual).is_file())
        self.assertFalse((self.root / "compromised").exists())

    def test_missing_and_invalid_debs_fail_without_output(self):
        with self.assertRaises(FileNotFoundError):
            self.bundle(cli=self.root / "missing.deb")
        invalid = self.root / "invalid.deb"
        invalid.write_text("not a package")
        with self.assertRaises(subprocess.CalledProcessError):
            self.bundle(cli=invalid)
        self.assertFalse(self.output.exists())

    def test_metadata_timeout_propagates_without_output(self):
        with mock.patch.object(stage.subprocess, "run", side_effect=subprocess.TimeoutExpired("dpkg-deb", 30)):
            with self.assertRaises(subprocess.TimeoutExpired):
                self.bundle()
        self.assertFalse(self.output.exists())

    def test_failed_write_preserves_existing_release_and_removes_temporary_file(self):
        bundle = self.bundle()
        original = bundle.read_bytes()
        with mock.patch.object(stage, "add_member", side_effect=OSError("fixture write failure")):
            with self.assertRaisesRegex(OSError, "fixture write failure"):
                self.bundle()
        self.assertEqual(bundle.read_bytes(), original)
        self.assertEqual(list(self.output.iterdir()), [bundle])


if __name__ == "__main__":
    unittest.main()
