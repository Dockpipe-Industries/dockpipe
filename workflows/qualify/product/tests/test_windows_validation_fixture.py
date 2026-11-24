"""Pure producer closure, integrity and refusal tests; no validator code import."""
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

specification = importlib.util.spec_from_file_location(
    "windows_validation_builder",
    Path(__file__).resolve().parents[1] / "fixtures/windows-validation/build.py",
)
BUILDER = importlib.util.module_from_spec(specification)
specification.loader.exec_module(BUILDER)


class WindowsValidationFixtureTests(unittest.TestCase):
    def test_deterministic_exact_closure_and_manifest_hash(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / "native_arguments.py"
            source.write_bytes(b"raise RuntimeError('must never import during export')\n")
            first = BUILDER.produce(source, root / "one")
            second = BUILDER.produce(source, root / "two")
            data = Path(first["artifact"]).read_bytes()
            self.assertEqual(data, Path(second["artifact"]).read_bytes())
            self.assertEqual(hashlib.sha256(data).hexdigest(), first["artifact_sha256"])
            with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as archive:
                self.assertEqual(archive.getnames(), ["manifest.json", "native_arguments.py"])
                self.assertTrue(all(member.isfile() for member in archive.getmembers()))
                manifest = json.load(archive.extractfile("manifest.json"))
                self.assertEqual(manifest, first["manifest"])
                module = archive.extractfile("native_arguments.py").read()
                self.assertEqual(manifest["files"][0]["sha256"], hashlib.sha256(module).hexdigest())
                self.assertEqual(manifest["files"][0]["bytes"], len(module))
            self.assertFalse(first["code_imported"])
            self.assertFalse(first["live_effects_started"])

    def test_changed_source_refuses_before_output(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / "output"
            with patch.object(BUILDER, "read_source", side_effect=[b"first", b"changed"]):
                with self.assertRaises(ValueError):
                    BUILDER.produce(Path(temporary) / "source", output)
            self.assertFalse(output.exists())

    def test_link_and_oversized_module_refuse(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / "source"
            source.write_bytes(b"x" * (BUILDER.MAXIMUM_MODULE_BYTES + 1))
            with self.assertRaises(ValueError):
                BUILDER.produce(source, root / "output")
            alias = root / "alias"
            alias.symlink_to(source)
            with self.assertRaises(OSError):
                BUILDER.produce(alias, root / "output")
            self.assertFalse((root / "output").exists())


if __name__ == "__main__":
    unittest.main()
