#!/usr/bin/env python3
import importlib.util
import io
from pathlib import Path
import tarfile
import tempfile
import unittest
import zipfile

spec = importlib.util.spec_from_file_location("package_tools", Path(__file__).with_name("prepare-package-tools.py"))
tools = importlib.util.module_from_spec(spec)
spec.loader.exec_module(tools)


class DependencyArchiveTests(unittest.TestCase):
    def test_tar_rejects_escape_and_never_materializes_links(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            archive_path = root / "input.tar"
            destination = root / "output"
            destination.mkdir()
            with tarfile.open(archive_path, "w") as archive:
                link = tarfile.TarInfo("vendor/bin/npm")
                link.type = tarfile.SYMTYPE
                link.linkname = "../lib/npm.js"
                archive.addfile(link)
                regular = tarfile.TarInfo("vendor/lib/npm.js")
                regular.size = 4
                archive.addfile(regular, io.BytesIO(b"tool"))
            tools.unpack_regular_files(archive_path, destination, "tar")
            self.assertFalse((destination / "vendor/bin/npm").exists())
            self.assertEqual((destination / "vendor/lib/npm.js").read_text(), "tool")
            with tarfile.open(archive_path, "w") as archive:
                escape = tarfile.TarInfo("../escaped")
                escape.size = 4
                archive.addfile(escape, io.BytesIO(b"oops"))
            with self.assertRaises(ValueError):
                tools.unpack_regular_files(archive_path, destination, "tar")
            self.assertFalse((root / "escaped").exists())

    def test_zip_rejects_escape(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            archive_path = root / "input.zip"
            with zipfile.ZipFile(archive_path, "w") as archive:
                archive.writestr("../escaped", "oops")
            with self.assertRaises(ValueError):
                tools.unpack_regular_files(archive_path, root / "output", "zip")
            self.assertFalse((root / "escaped").exists())

    def test_corrupt_cached_input_is_not_used(self):
        with tempfile.TemporaryDirectory() as temporary:
            cache = Path(temporary)
            (cache / "tool").write_bytes(b"tampered")
            with self.assertRaises(ValueError):
                tools.verified_input("tool", {"sha256": "0" * 64}, cache)


if __name__ == "__main__":
    unittest.main()
