"""Offline protocol and installer boundary checks; no model downloads."""
import hashlib
import importlib.util
import io
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1] / "neural"


def module(name):
    spec = importlib.util.spec_from_file_location(name, ROOT / (name + ".py"))
    loaded = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(loaded)
    return loaded


installer = module("install")
worker = module("worker")


class Response(io.BytesIO):
    url = "https://fixture.invalid/model"


class NeuralBoundaryTest(unittest.TestCase):
    def test_animal_forms_require_confident_unambiguous_evidence(self):
        self.assertEqual(worker.animal_form([0.8] + [0.1] * 8), "spider")
        self.assertEqual(worker.animal_form([0.2] * 9), "")
        self.assertEqual(worker.animal_form([0.8, 0.78] + [0.1] * 7), "")
        self.assertEqual(worker.animal_form([float("nan")] * 9), "")
        self.assertEqual(worker.animal_form([0.9]), "")

    def test_checksum_failure_never_replaces_existing_model(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            target = root / "model.onnx"
            target.write_bytes(b"old")
            entry = {"name": target.name, "url": Response.url, "bytes": 3,
                     "sha256": hashlib.sha256(b"new").hexdigest()}
            with patch.object(installer.urllib.request, "urlopen", return_value=Response(b"bad")):
                with self.assertRaises(ValueError):
                    installer.download(entry, root)
            self.assertEqual(target.read_bytes(), b"old")
            self.assertFalse(root.joinpath("model.partial").exists())

    def test_oversized_download_is_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            entry = {"name": "model.onnx", "url": Response.url, "bytes": 3, "sha256": "unused"}
            with patch.object(installer.urllib.request, "urlopen", return_value=Response(b"excessive")):
                with self.assertRaises(ValueError):
                    installer.download(entry, root)
            self.assertEqual(list(root.iterdir()), [])

    def test_verified_model_is_reused_without_network(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            root.joinpath("model.onnx").write_bytes(b"new")
            entry = {"name": "model.onnx", "url": Response.url, "bytes": 3,
                     "sha256": hashlib.sha256(b"new").hexdigest()}
            with patch.object(installer.urllib.request, "urlopen") as network:
                installer.download(entry, root)
                network.assert_not_called()

    def test_page_input_does_not_select_operations_or_paths(self):
        inference = object.__new__(worker.Inference)
        with self.assertRaises(ValueError):
            inference.plan({"title": "x" * 181})
        with self.assertRaises(ValueError):
            inference.plan({"items": [{}] * 13})
        with self.assertRaises(ValueError):
            inference.plan({"items": [{"id": "../"}]})
        with self.assertRaises(ValueError):
            inference.plan({"items": [{"id": "0"}, {"id": "0"}]})

    def test_models_manifest_is_pinned(self):
        import json
        manifest = json.loads(ROOT.joinpath("models.json").read_text())
        self.assertEqual(len(manifest["files"]), 3)
        for entry in manifest["files"]:
            self.assertEqual(Path(entry["name"]).name, entry["name"])
            self.assertRegex(entry["sha256"], r"^[a-f0-9]{64}$")
            self.assertTrue(entry["url"].startswith("https://"))
            self.assertLess(entry["bytes"], 100 * 1024 * 1024)


if __name__ == "__main__":
    unittest.main()
