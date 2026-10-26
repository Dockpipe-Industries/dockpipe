"""Publication verifies the exact qualified bytes before contacting Marketplace."""
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock
import zipfile

import contract


class ContractTests(unittest.TestCase):
    def fixture(self, root):
        extension = root / "source"
        extension.mkdir()
        document = {"publisher": "dockpipe", "name": "example", "version": "1.2.3", "files": ["extension.js", "package.json", "README.md", "LICENSE"]}
        (extension / "package.json").write_text(json.dumps(document))
        (extension / "extension.js").write_text("module.exports = {};\n")
        (extension / "README.md").write_text("Example extension\n")
        (extension / "LICENSE").write_text("License text\n")
        output = root / "artifact"
        output.mkdir()
        artifact = output / "example-1.2.3.vsix"
        with zipfile.ZipFile(artifact, "w") as archive:
            for name in ("package.json", "extension.js"):
                archive.writestr("extension/" + name, (extension / name).read_bytes())
            archive.writestr("extension/readme.md", (extension / "README.md").read_bytes())
            archive.writestr("extension/LICENSE.txt", (extension / "LICENSE").read_bytes())
            archive.writestr("extension.vsixmanifest", '<PackageManifest><Metadata><Identity Id="example" Publisher="dockpipe" Version="1.2.3" /></Metadata></PackageManifest>')
        return extension, output, artifact

    def receipt(self, output, artifact):
        document = {"schema": 1, "identity": "dockpipe.example", "version": "1.2.3", "filename": artifact.name,
                    "sha256": contract.digest(artifact), "bytes": artifact.stat().st_size,
                    "source_inventory": contract.inventory(), "qualification": "tests-and-vsce-package-passed"}
        (output / "extension.json").write_text(json.dumps(document))
        return document["sha256"]

    def test_wrong_bytes_or_changed_source_fail_qualification(self):
        with tempfile.TemporaryDirectory() as directory:
            extension, output, artifact = self.fixture(Path(directory))
            with mock.patch.object(contract, "EXTENSION", extension):
                checksum = self.receipt(output, artifact)
                self.assertEqual(contract.qualify_artifact(output, "1.2.3", checksum), artifact)
                with self.assertRaisesRegex(ValueError, "reviewed artifact"):
                    contract.qualify_artifact(output, "1.2.3", "0" * 64)
                (extension / "extension.js").write_text("changed source")
                with self.assertRaises(ValueError):
                    contract.qualify_artifact(output, "1.2.3", checksum)

    def test_vsix_identity_is_checked_in_package_and_vsix_manifest(self):
        with tempfile.TemporaryDirectory() as directory:
            extension, output, artifact = self.fixture(Path(directory))
            with zipfile.ZipFile(artifact) as archive:
                payload = {name: archive.read(name) for name in archive.namelist()}
            payload["extension.vsixmanifest"] = b'<PackageManifest><Identity Id="other" Publisher="dockpipe" Version="1.2.3" /></PackageManifest>'
            with zipfile.ZipFile(artifact, "w") as archive:
                for name, data in payload.items():
                    archive.writestr(name, data)
            with mock.patch.object(contract, "EXTENSION", extension):
                with self.assertRaisesRegex(ValueError, "VSIX manifest"):
                    contract.inspect_vsix(artifact, "1.2.3")

    def test_publish_uses_the_same_qualified_vsix(self):
        path = contract.ROOT / "workflows/editor.publish/assets/scripts/publish.py"
        spec = importlib.util.spec_from_file_location("editor_publisher", path)
        publisher = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(publisher)
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            artifact = root / "reviewed.vsix"
            artifact.write_bytes(b"reviewed")
            arguments = ["--artifact-dir", str(root), "--version", "1.2.3", "--sha256", "a" * 64]
            with mock.patch.dict(os.environ, {"DOCKPIPE_ARGS_JSON": json.dumps(arguments), "VSCE_PAT": "test-reference"}), \
                    mock.patch.object(contract, "qualify_artifact", return_value=artifact), \
                    mock.patch.object(contract, "clean_environment", return_value={}), \
                    mock.patch.object(contract, "scope", return_value=root), \
                    mock.patch.object(contract, "stage", return_value=root), \
                    mock.patch.object(contract, "install_tools", return_value=("node", "locked-vsce")), \
                    mock.patch.object(contract, "run") as child:
                publisher.main()
            self.assertEqual(child.call_args.args[0], ["node", "locked-vsce", "publish", "--packagePath", artifact])
            self.assertEqual(child.call_args.args[2]["VSCE_PAT"], "test-reference")


if __name__ == "__main__":
    unittest.main()
