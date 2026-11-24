import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch


SPEC = importlib.util.spec_from_file_location(
    "desktop_fixture_builder", Path(__file__).resolve().parents[1] / "fixtures/desktop/build.py")
BUILDER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(BUILDER)


class DesktopFixtureTests(unittest.TestCase):
    def test_exports_identical_bytes_with_exact_manifest_and_regular_members(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / "source"
            source.mkdir()
            for name in BUILDER.MODULE_NAMES:
                (source / name).write_text("# generated fixture module: " + name + "\n")
            first = BUILDER.produce(source, root / "first")
            second = BUILDER.produce(source, root / "second")
            artifact = Path(first["artifact"]).read_bytes()
            self.assertEqual(artifact, Path(second["artifact"]).read_bytes())
            self.assertFalse(first["product_qualified"])
            self.assertFalse(first["desktop_started"])
            with tarfile.open(fileobj=io.BytesIO(artifact), mode="r:gz") as archive:
                self.assertEqual(archive.getnames(), ["manifest.json", "desktop.py", "ui.py"])
                self.assertTrue(all(member.isfile() for member in archive.getmembers()))
                manifest = json.loads(archive.extractfile("manifest.json").read())
                self.assertEqual(manifest, first["manifest"])
                for entry in manifest["files"]:
                    contents = archive.extractfile(entry["path"]).read()
                    self.assertEqual(contents, (source / entry["path"]).read_bytes())
                    self.assertEqual(entry["bytes"], len(contents))
                    self.assertEqual(entry["sha256"], hashlib.sha256(contents).hexdigest())

    def test_source_change_refuses_before_creating_output(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / "output"
            with patch.object(BUILDER, "read_source", side_effect=[b"one", b"two", b"changed"]):
                with self.assertRaisesRegex(ValueError, "changed during export"):
                    BUILDER.produce(Path(temporary) / "source", output)
            self.assertFalse(output.exists())

    @unittest.skipUnless(os.name == "posix", "Linux desktop fixture requires POSIX links")
    def test_linked_and_oversized_source_members_are_refused(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / "source.py"
            source.write_bytes(b"x" * (BUILDER.MAXIMUM_MODULE_BYTES + 1))
            with self.assertRaisesRegex(ValueError, "byte limit"):
                BUILDER.read_source(source)
            alias = root / "alias.py"
            alias.symlink_to(source)
            with self.assertRaisesRegex(ValueError, "regular file"):
                BUILDER.read_source(alias)


if __name__ == "__main__":
    unittest.main()
