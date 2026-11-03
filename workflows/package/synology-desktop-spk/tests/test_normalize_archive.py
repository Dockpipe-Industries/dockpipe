"""Normalize Skopeo's legacy aliases while preserving immutable image bytes."""

import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest import mock


SCRIPT = Path(__file__).resolve().parents[1] / "assets/scripts/normalize_archive.py"
SPECIFICATION = importlib.util.spec_from_file_location("normalize_image_archive", SCRIPT)
NORMALIZE = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(NORMALIZE)


class NormalizeArchiveTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.source = self.root / "source.tar"
        self.destination = self.root / "image.tar.gz"
        self.configuration = b'{ "architecture": "amd64", "os": "linux" }\n'
        self.manifest = [{"Config": "config.json", "RepoTags": ["example:release"],
                          "Layers": ["actual/layer.tar"]}]
        self.write_source()

    def write_source(self, extra_members=(), omit_layer=False):
        members = [("config.json", self.configuration),
                   ("manifest.json", json.dumps(self.manifest).encode()),
                   ("repositories", b"legacy metadata"),
                   ("legacy/json", b"legacy metadata"), ("legacy/VERSION", b"1.0")]
        if not omit_layer:
            members.append(("actual/layer.tar", b"real layer bytes"))
        members.extend(extra_members)
        with tarfile.open(self.source, "w") as archive:
            for name, content in members:
                member = tarfile.TarInfo(name)
                if content is None:
                    member.type = tarfile.SYMTYPE
                    member.linkname = "../actual/layer.tar"
                    archive.addfile(member)
                else:
                    member.size = len(content)
                    archive.addfile(member, io.BytesIO(content))

    def normalize(self):
        NORMALIZE.write_loadable_archive(self.source, self.destination)

    def test_unreferenced_legacy_symlink_is_dropped_without_following(self):
        self.write_source(extra_members=[("legacy/layer.tar", None)])
        self.normalize()
        with tarfile.open(self.destination, "r:gz") as archive:
            self.assertEqual(archive.getnames(), ["manifest.json", "config.json", "actual/layer.tar"])
            self.assertTrue(all(member.isfile() for member in archive))
            self.assertEqual(archive.extractfile("config.json").read(), self.configuration)
            self.assertEqual(json.load(archive.extractfile("manifest.json")), self.manifest)
            self.assertEqual(archive.extractfile("actual/layer.tar").read(), b"real layer bytes")
        # The unchanged strict final admission reader accepts the normalized structure.
        members, metadata = NORMALIZE.ADMISSION.archive_metadata(self.destination)
        self.assertEqual(len(members), 3)
        self.assertEqual(metadata["config.json"], self.configuration)

    def test_referenced_symlink_is_rejected_without_following(self):
        self.manifest[0]["Layers"] = ["legacy/layer.tar"]
        self.write_source(extra_members=[("legacy/layer.tar", None)])
        with self.assertRaisesRegex(ValueError, "nonregular"):
            self.normalize()
        self.assertFalse(self.destination.exists())

    def test_rejects_missing_and_unsafe_references(self):
        for reference in ("missing.tar", "../escape", "/absolute", None):
            with self.subTest(reference=reference):
                self.manifest[0]["Layers"] = [reference]
                self.write_source()
                with self.assertRaises(ValueError):
                    self.normalize()
                self.assertFalse(self.destination.exists())

    def test_rejects_multiple_images_and_invalid_tags(self):
        for manifest in ([{}, {}], [{"Config": "config.json", "Layers": [], "RepoTags": "tag"}]):
            with self.subTest(manifest=manifest):
                self.manifest = manifest
                self.write_source()
                with self.assertRaises(ValueError):
                    self.normalize()

    def test_rejects_duplicate_or_unsafe_unreferenced_member_names(self):
        for name in ("config.json", "actual/../escape", "/absolute"):
            with self.subTest(name=name):
                self.write_source(extra_members=[(name, b"extra")])
                with self.assertRaises(ValueError):
                    self.normalize()

    def test_deterministic_output_and_existing_destination_preservation(self):
        self.normalize()
        first_bytes = self.destination.read_bytes()
        another = self.root / "another.tar.gz"
        NORMALIZE.write_loadable_archive(self.source, another)
        self.assertEqual(first_bytes, another.read_bytes())
        with self.assertRaises(FileExistsError):
            self.normalize()
        self.assertEqual(first_bytes, self.destination.read_bytes())

    def test_bounds_fail_with_small_fixtures(self):
        for setting, limit in (("MAX_ARCHIVE_BYTES", self.source.stat().st_size - 1),
                               ("MAX_MEMBERS", 2), ("MAX_METADATA_BYTES", 8)):
            with self.subTest(setting=setting), mock.patch.object(NORMALIZE, setting, limit):
                with self.assertRaisesRegex(ValueError, "limit"):
                    self.normalize()
                self.assertFalse(self.destination.exists())

    def test_extension_bound_is_checked_before_tarfile_allocation(self):
        self.write_source(extra_members=[("legacy/" + "x" * 120, b"ignored")])
        with mock.patch.object(NORMALIZE, "MAX_EXTENSION_BYTES", 8):
            with self.assertRaisesRegex(ValueError, "extension exceeds metadata limit"):
                self.normalize()
        self.assertFalse(self.destination.exists())

    def test_partial_destination_is_removed_on_stream_failure(self):
        with mock.patch.object(NORMALIZE, "add_regular_file", side_effect=OSError("fixture failure")):
            with self.assertRaisesRegex(OSError, "fixture failure"):
                self.normalize()
        self.assertFalse(self.destination.exists())

    def test_optional_content_tag_rewrites_only_manifest_repository_tags(self):
        tag = "ghcr.io/example/desktop:offline-" + "a" * 64
        NORMALIZE.write_loadable_archive(self.source, self.destination, repo_tag=tag)
        with tarfile.open(self.destination, "r:gz") as archive:
            manifest = json.load(archive.extractfile("manifest.json"))
            self.assertEqual(manifest[0]["RepoTags"], [tag])
            self.assertEqual(manifest[0]["Config"], self.manifest[0]["Config"])
            self.assertEqual(manifest[0]["Layers"], self.manifest[0]["Layers"])
            self.assertEqual(archive.extractfile("config.json").read(), self.configuration)
            self.assertEqual(archive.extractfile("actual/layer.tar").read(), b"real layer bytes")

    def test_optional_repository_tag_rejects_noncanonical_values(self):
        for tag in ("ghcr.io/example/desktop:latest", "docker.io/example/desktop:offline-" + "a" * 64,
                    "ghcr.io/example/desktop:offline-short", 42, "ghcr.io/example/desktop:offline-" + "A" * 64):
            with self.subTest(tag=tag):
                with self.assertRaisesRegex(ValueError, "canonical GHCR"):
                    NORMALIZE.write_loadable_archive(self.source, self.destination, repo_tag=tag)
                self.assertFalse(self.destination.exists())

    def test_null_repository_tags_are_preserved(self):
        self.manifest[0]["RepoTags"] = None
        self.write_source()
        self.normalize()
        with tarfile.open(self.destination, "r:gz") as archive:
            manifest = json.load(archive.extractfile("manifest.json"))
            self.assertIsNone(manifest[0]["RepoTags"])

    def test_referenced_configuration_link_is_rejected(self):
        self.manifest[0]["Config"] = "legacy/config.json"
        self.write_source(extra_members=[("legacy/config.json", None)])
        with self.assertRaisesRegex(ValueError, "nonregular"):
            self.normalize()
        self.assertFalse(self.destination.exists())

    def test_blob_config_path_is_preserved(self):
        self.manifest[0]["Config"] = "blobs/sha256/" + "a" * 64
        self.write_source(extra_members=[(self.manifest[0]["Config"], self.configuration)])
        self.normalize()
        with tarfile.open(self.destination, "r:gz") as archive:
            self.assertEqual(archive.extractfile(self.manifest[0]["Config"]).read(), self.configuration)


if __name__ == "__main__":
    unittest.main()
