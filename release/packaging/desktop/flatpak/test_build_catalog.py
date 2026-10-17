"""Compiled Flatpak catalog archive integrity boundaries."""

import hashlib
import importlib.util
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


DIRECTORY = Path(__file__).resolve().parent


def load_module(name, filename):
    specification = importlib.util.spec_from_file_location(name, DIRECTORY / filename)
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


catalog = load_module("build_catalog", "build-catalog.py")
tools = load_module("prepare_package_tools", "prepare-package-tools.py")


class CompiledArchiveChecksumTests(unittest.TestCase):
    def test_compiled_archive_has_string_digest_with_separate_capacity(self):
        with tempfile.TemporaryDirectory() as temporary:
            archive = Path(temporary) / "compiled.tar.gz"
            archive.write_bytes(b"catalog archive")

            with patch.object(tools, "MAX_TOOL_ARCHIVE_BYTES", 3):
                with self.assertRaisesRegex(ValueError, "Optional tool archive exceeds capacity"):
                    tools.checksum(archive)
                with patch.object(catalog, "MAX_COMPILED_ARCHIVE_BYTES", 32):
                    digest = catalog.compiled_archive_checksum(archive)

            self.assertIsInstance(digest, str)
            self.assertEqual(digest, hashlib.sha256(b"catalog archive").hexdigest())

    def test_compiled_archive_rejects_growth_after_open(self):
        with tempfile.TemporaryDirectory() as temporary:
            archive = Path(temporary) / "compiled.tar.gz"
            archive.write_bytes(b"abc")

            original_fdopen = catalog.os.fdopen

            class GrowingArchive:
                def __init__(self, opened):
                    self.opened = opened
                    self.first_read = True

                def __enter__(self):
                    return self

                def __exit__(self, exception_type, exception, traceback):
                    self.opened.close()

                def fileno(self):
                    return self.opened.fileno()

                def read(self, size):
                    chunk = self.opened.read(size)
                    if self.first_read:
                        self.first_read = False
                        with archive.open("ab") as writer:
                            writer.write(b"d")
                    return chunk

            def open_growing_archive(descriptor, mode):
                return GrowingArchive(original_fdopen(descriptor, mode))

            with patch.object(catalog, "MAX_COMPILED_ARCHIVE_BYTES", 3):
                with patch.object(catalog.os, "fdopen", side_effect=open_growing_archive):
                    with self.assertRaisesRegex(ValueError, "Compiled package archive exceeds capacity"):
                        catalog.compiled_archive_checksum(archive)

    def test_compiled_archive_rejects_symlink(self):
        with tempfile.TemporaryDirectory() as temporary:
            archive = Path(temporary) / "compiled.tar.gz"
            archive.write_bytes(b"archive")
            linked = Path(temporary) / "linked.tar.gz"
            linked.symlink_to(archive)

            with self.assertRaises(OSError):
                catalog.compiled_archive_checksum(linked)


if __name__ == "__main__":
    unittest.main()
