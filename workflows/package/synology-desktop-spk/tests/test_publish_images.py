"""Ensure registry publication preserves identity and requires anonymous readback."""

import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock


SCRIPT = Path(__file__).resolve().parents[1] / "assets/scripts/publish-images.py"
SPECIFICATION = importlib.util.spec_from_file_location("publish_desktop_images", SCRIPT)
PUBLISHER = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(PUBLISHER)


class ImagePublicationTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.images = self.root / "images"
        self.artifacts = self.root / "artifacts"
        self.images.mkdir()
        self.artifacts.mkdir()
        self.raw = {}
        self.receipts = {}
        self.commands = []
        for architecture, label in (("amd64", "x64"), ("arm64", "arm64")):
            raw = json.dumps({"schemaVersion": 2, "config": {"digest": "sha256:" + ("a" if architecture == "amd64" else "b") * 64}}).encode()
            self.raw[architecture] = raw
            self.receipts[architecture] = {
                "schema": 1, "version": "1.2.3", "architecture": architecture,
                "source_sha": "c" * 40,
                "image": "ghcr.io/example/desktop@sha256:" + hashlib.sha256(raw).hexdigest(),
                "image_id": json.loads(raw)["config"]["digest"],
                "offline_image": "ghcr.io/example/desktop:offline-" + json.loads(raw)["config"]["digest"][7:],
                "archive_sha256": "d" * 64,
                "inputs": {"cli_sha256": hashlib.sha256(b"CLI").hexdigest(),
                           "desktop_sha256": hashlib.sha256(b"GUI").hexdigest()},
            }
            content = json.dumps(self.receipts[architecture]).encode()
            (self.images / (architecture + ".json")).write_bytes(content)
            (self.images / (architecture + ".oci.tar")).write_bytes(b"OCI fixture")
            (self.artifacts / f"dockpipe-desktop_1.2.3_synology_{label}.image.json").write_bytes(content)
            (self.artifacts / f"dockpipe_1.2.3_{architecture}.deb").write_bytes(b"CLI")
            (self.artifacts / f"dockpipe-desktop_1.2.3_{architecture}.deb").write_bytes(b"GUI")
        patcher = mock.patch.dict(os.environ, {"GITHUB_ACTOR": "fixture", "GITHUB_TOKEN": "fixture-secret-only"})
        patcher.start()
        self.addCleanup(patcher.stop)

    def run_command(self, arguments, **options):
        self.commands.append(arguments)
        if arguments[1] == "login":
            self.assertEqual(options["input"], b"fixture-secret-only")
        elif arguments[1] == "inspect":
            if arguments[-1].startswith("oci-archive:"):
                architecture = Path(arguments[-1]).name.split(".")[0]
            else:
                architecture = next(key for key, value in self.receipts.items() if arguments[-1] == "docker://" + value["image"])
            return subprocess.CompletedProcess(arguments, 0, stdout=self.raw[architecture])
        elif arguments[1] == "copy":
            self.assertIn("--preserve-digests", arguments)
            architecture = arguments[-1].split("-")[-1]
            Path(arguments[arguments.index("--digestfile") + 1]).write_text(self.receipts[architecture]["image"].split("@")[1])
        return subprocess.CompletedProcess(arguments, 0)

    def test_both_images_are_published_and_read_anonymously_at_exact_digest(self):
        with mock.patch.object(PUBLISHER, "run", side_effect=self.run_command):
            PUBLISHER.publish(self.images, self.artifacts, "1.2.3", "c" * 40)
        copies = [command for command in self.commands if command[1] == "copy"]
        self.assertEqual(len(copies), 2)
        anonymous = [command for command in self.commands if command[1] == "inspect" and command[-1].startswith("docker://")]
        self.assertEqual(len(anonymous), 2)
        self.assertTrue(all("anonymous.json" in command[command.index("--authfile") + 1] for command in anonymous))
        self.assertTrue(all("fixture-secret-only" not in " ".join(command) for command in self.commands))

    def test_private_registry_readback_stops_publication_with_setup_error(self):
        def private(arguments, **options):
            if arguments[1] == "inspect" and arguments[-1].startswith("docker://"):
                raise subprocess.CalledProcessError(1, arguments)
            return self.run_command(arguments, **options)
        with mock.patch.object(PUBLISHER, "run", side_effect=private), self.assertRaisesRegex(ValueError, "Public"):
            PUBLISHER.publish(self.images, self.artifacts, "1.2.3", "c" * 40)
        self.assertEqual(len([command for command in self.commands if command[1] == "copy"]), 1)

    def test_wrong_source_stops_before_any_image_push(self):
        with mock.patch.object(PUBLISHER, "run", side_effect=self.run_command), self.assertRaisesRegex(ValueError, "different source"):
            PUBLISHER.publish(self.images, self.artifacts, "1.2.3", "e" * 40)
        self.assertFalse(any(command[1] == "copy" for command in self.commands))

    def test_mismatched_oci_manifest_stops_before_any_image_push(self):
        self.raw["amd64"] += b" "
        with mock.patch.object(PUBLISHER, "run", side_effect=self.run_command), self.assertRaisesRegex(ValueError, "does not match"):
            PUBLISHER.publish(self.images, self.artifacts, "1.2.3", "c" * 40)
        self.assertFalse(any(command[1] == "copy" for command in self.commands))


if __name__ == "__main__":
    unittest.main()
