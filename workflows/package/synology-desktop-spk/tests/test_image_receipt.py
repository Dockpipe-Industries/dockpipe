"""Exercise image identity and hostile archive admission without a Docker daemon."""

import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest import mock


SCRIPT = Path(__file__).resolve().parents[1] / "assets/scripts/image_receipt.py"
SPECIFICATION = importlib.util.spec_from_file_location("image_receipt", SCRIPT)
RECEIPTS = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(RECEIPTS)


class ImageReceiptTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.cli = self.root / "cli.deb"
        self.desktop = self.root / "desktop.deb"
        self.cli.write_bytes(b"CLI fixture")
        self.desktop.write_bytes(b"desktop fixture")
        self.archive = self.root / "image.tar.gz"
        self.receipt_path = self.root / "image.json"
        self.config = {
            "os": "linux", "architecture": "amd64",
            "config": {"User": "65532:65532", "Labels": {
                "org.opencontainers.image.version": "1.2.3",
                "org.opencontainers.image.revision": "a" * 40,
            }},
            "rootfs": {"type": "layers", "diff_ids": ["sha256:" + "b" * 64]},
        }
        self.receipt = {
            "schema": 1, "version": "1.2.3", "architecture": "amd64",
            "source_sha": "a" * 40,
            "image": "ghcr.io/example/desktop@sha256:" + "c" * 64,
            "inputs": {
                "cli_sha256": RECEIPTS.file_sha256(self.cli),
                "desktop_sha256": RECEIPTS.file_sha256(self.desktop),
            },
        }
        self.write_archive()

    def write_archive(self, manifest=None, extra_members=(), config_name="config.json", include_layer=True):
        config_bytes = json.dumps(self.config).encode()
        self.receipt["image_id"] = "sha256:" + hashlib.sha256(config_bytes).hexdigest()
        self.receipt["offline_image"] = (
            self.receipt["image"].split("@", 1)[0] + ":offline-" + self.receipt["image_id"][7:]
        )
        if manifest is None:
            manifest = [{"Config": config_name, "RepoTags": [self.receipt["offline_image"]],
                         "Layers": ["layer/layer.tar"]}]
        members = [("manifest.json", json.dumps(manifest).encode()), (config_name, config_bytes)]
        if include_layer:
            members.append(("layer/layer.tar", b"layer fixture"))
        members.extend(extra_members)
        with tarfile.open(self.archive, "w:gz") as archive:
            for name, content in members:
                information = tarfile.TarInfo(name)
                if content is None:
                    information.type = tarfile.SYMTYPE
                    information.linkname = "config.json"
                    archive.addfile(information)
                else:
                    information.size = len(content)
                    archive.addfile(information, io.BytesIO(content))
        self.receipt["archive_sha256"] = RECEIPTS.file_sha256(self.archive)
        self.save_receipt()

    def save_receipt(self):
        self.receipt_path.write_text(json.dumps(self.receipt))

    def admit(self, offline=True):
        return RECEIPTS.load_receipt(
            self.receipt_path, "1.2.3", "amd64", self.cli, self.desktop,
            self.archive if offline else None,
        )

    def test_standard_and_offline_share_identity_without_repo_digests(self):
        self.assertEqual(self.admit(), self.admit(offline=False))
        self.archive.unlink()
        self.assertEqual(self.admit(offline=False)["image_id"], self.receipt["image_id"])

    def test_accepts_config_blob_path_used_by_new_docker_save(self):
        self.write_archive(config_name="blobs/sha256/" + "d" * 64)
        self.admit()

    def test_rejects_receipt_identity_mismatches(self):
        for field, value in (("schema", True), ("version", "2"), ("architecture", "arm64"),
                             ("source_sha", "bad"), ("image", "ghcr.io/example/desktop:latest"),
                             ("image_id", "sha256:bad"), ("archive_sha256", "bad")):
            with self.subTest(field=field):
                original = self.receipt[field]
                self.receipt[field] = value
                self.save_receipt()
                with self.assertRaises(ValueError):
                    self.admit(offline=False)
                self.receipt[field] = original

    def test_offline_content_tag_is_mandatory_and_derived_from_repository_and_config(self):
        expected = self.receipt["offline_image"]
        for value in (None, "ghcr.io/example/desktop:latest",
                      "ghcr.io/other/desktop:offline-" + self.receipt["image_id"][7:],
                      "ghcr.io/example/desktop:offline-" + "0" * 64):
            with self.subTest(value=value):
                if value is None:
                    self.receipt.pop("offline_image", None)
                else:
                    self.receipt["offline_image"] = value
                self.save_receipt()
                with self.assertRaisesRegex(ValueError, "offline_image"):
                    self.admit(offline=False)
        self.receipt["offline_image"] = expected

    def test_archive_requires_exactly_one_matching_offline_content_tag(self):
        expected = self.receipt["offline_image"]
        for tags in (None, [], ["ghcr.io/example/desktop:latest"],
                     [expected, "ghcr.io/example/desktop:extra"]):
            with self.subTest(tags=tags):
                self.write_archive(manifest=[{
                    "Config": "config.json", "Layers": ["layer/layer.tar"], "RepoTags": tags,
                }])
                with self.assertRaisesRegex(ValueError, "archive tags"):
                    self.admit()
        self.write_archive(manifest=[{"Config": "config.json", "Layers": ["layer/layer.tar"]}])
        with self.assertRaisesRegex(ValueError, "archive tags"):
            self.admit()

    def test_rejects_changed_input_packages(self):
        for package in (self.cli, self.desktop):
            with self.subTest(package=package.name):
                original = package.read_bytes()
                package.write_bytes(b"different package")
                with self.assertRaisesRegex(ValueError, "does not match package"):
                    self.admit()
                package.write_bytes(original)

    def test_rejects_changed_archive_and_configuration(self):
        self.archive.write_bytes(self.archive.read_bytes() + b"changed")
        with self.assertRaisesRegex(ValueError, "archive SHA-256"):
            self.admit()
        self.write_archive(manifest=[{
            "Config": "config.json", "Layers": ["layer/layer.tar"],
            "RepoTags": ["ghcr.io/example/desktop:offline-" + "0" * 64],
        }])
        self.receipt["image_id"] = "sha256:" + "0" * 64
        self.receipt["offline_image"] = "ghcr.io/example/desktop:offline-" + "0" * 64
        self.save_receipt()
        with self.assertRaisesRegex(ValueError, "configuration digest"):
            self.admit()

    def test_rejects_cross_identity_configuration(self):
        mutations = (
            lambda: self.config.update(os="windows"),
            lambda: self.config.update(architecture="arm64"),
            lambda: self.config["config"].update(User="0:0"),
            lambda: self.config["config"]["Labels"].update({"org.opencontainers.image.version": "2"}),
            lambda: self.config["config"]["Labels"].update({"org.opencontainers.image.revision": "f" * 40}),
            lambda: self.config["rootfs"].update(diff_ids=[]),
        )
        original = json.dumps(self.config)
        for index, mutation in enumerate(mutations):
            with self.subTest(mutation=index):
                self.config = json.loads(original)
                mutation()
                self.write_archive()
                with self.assertRaises(ValueError):
                    self.admit()

    def test_rejects_missing_layers_and_multiple_images(self):
        self.write_archive(include_layer=False)
        with self.assertRaisesRegex(ValueError, "layer is missing"):
            self.admit()
        self.write_archive(manifest=[{}, {}])
        with self.assertRaisesRegex(ValueError, "exactly one"):
            self.admit()

    def test_rejects_unsafe_duplicate_and_link_members(self):
        for name, content in (("../escape", b"bad"), ("/absolute", b"bad"),
                              ("manifest.json", b"duplicate"), ("link", None)):
            with self.subTest(name=name):
                self.write_archive(extra_members=[(name, content)])
                with self.assertRaises(ValueError):
                    self.admit()

    def test_resource_limits_fail_with_small_fixtures(self):
        for setting, limit in (("MAX_MEMBERS", 2), ("MAX_METADATA_BYTES", 8),
                               ("MAX_ARCHIVE_BYTES", self.archive.stat().st_size + 1),
                               ("MAX_RECEIPT_BYTES", 8)):
            with self.subTest(setting=setting), mock.patch.object(RECEIPTS, setting, limit):
                with self.assertRaisesRegex(ValueError, "limit"):
                    self.admit()

    def test_extension_metadata_is_bounded_before_tarfile_processes_it(self):
        self.write_archive(extra_members=[("long/" + "x" * 120, b"small fixture")])
        self.admit()
        with mock.patch.object(RECEIPTS, "MAX_EXTENSION_BYTES", 8):
            with self.assertRaisesRegex(ValueError, "extension exceeds metadata limit"):
                self.admit()

    def test_rejects_malformed_json_and_archive(self):
        self.receipt_path.write_text('{"schema": 1, "schema": 1}')
        with self.assertRaisesRegex(ValueError, "duplicate JSON"):
            self.admit()
        self.archive.write_bytes(b"not gzip")
        self.receipt["archive_sha256"] = RECEIPTS.file_sha256(self.archive)
        self.save_receipt()
        with self.assertRaisesRegex(ValueError, "invalid compressed"):
            self.admit()


if __name__ == "__main__":
    unittest.main()
