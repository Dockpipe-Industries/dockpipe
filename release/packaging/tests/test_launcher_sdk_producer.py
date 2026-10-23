"""Deterministic host SDK archives and owned installed payload boundaries."""
import importlib.util
import os
from pathlib import Path
import tarfile
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "build-launcher-sdk.py"
SPEC = importlib.util.spec_from_file_location("launcher_sdk_producer", SCRIPT)
PRODUCER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PRODUCER)


class LauncherSDKProducerTests(unittest.TestCase):
    def test_archive_bytes_ignore_install_mtimes_and_flatten_owned_library_alias(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            prefix = root / "sdk"
            library = prefix / "lib/libhost.so.1.0"
            library.parent.mkdir(parents=True)
            library.write_bytes(b"known library bytes")
            library.chmod(0o755)
            (library.parent / "libhost.so.1").symlink_to(library.name)
            entries = PRODUCER.inventory(prefix)
            first = root / "first.tar.gz"
            second = root / "second.tar.gz"
            PRODUCER.write_archive(prefix, first, entries)
            os.utime(library, (1700000000, 1700000000))
            PRODUCER.write_archive(prefix, second, PRODUCER.inventory(prefix))
            self.assertEqual(first.read_bytes(), second.read_bytes())
            with tarfile.open(first) as archive:
                members = archive.getmembers()
                self.assertEqual(len(members), 2)
                for member in members:
                    self.assertTrue(member.isfile())
                    self.assertEqual(member.mtime, 0)
                    self.assertEqual(member.uid, 0)
                    self.assertEqual(archive.extractfile(member).read(), b"known library bytes")

    def test_unowned_file_or_directory_alias_is_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            prefix = root / "sdk"
            prefix.mkdir()
            external = root / "private"
            external.write_bytes(b"private")
            alias = prefix / "escaped"
            alias.symlink_to(external)
            with self.assertRaisesRegex(ValueError, "escapes"):
                PRODUCER.inventory(prefix)
            alias.unlink()
            alias.symlink_to(root, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "directory aliases"):
                PRODUCER.inventory(prefix)

    def test_distribution_capacity_and_empty_payload_fail(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            with self.assertRaisesRegex(ValueError, "empty"):
                PRODUCER.inventory(root)
            (root / "payload").write_bytes(b"bytes")
            original_limit = PRODUCER.MAX_INSTALLED_BYTES
            self.addCleanup(setattr, PRODUCER, "MAX_INSTALLED_BYTES", original_limit)
            PRODUCER.MAX_INSTALLED_BYTES = 4
            with self.assertRaisesRegex(ValueError, "distribution contract"):
                PRODUCER.inventory(root)

    def test_compiler_identity_requires_unambiguous_cmake_record(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            compiler = root / "CMakeFiles/3.22.1/CMakeCXXCompiler.cmake"
            compiler.parent.mkdir(parents=True)
            compiler.write_text('set(CMAKE_CXX_COMPILER_ID "GNU")\n'
                                'set(CMAKE_CXX_COMPILER_VERSION "11.4.0")\n')
            self.assertEqual(PRODUCER.compiler_identity(root), {"id": "GNU", "version": "11.4.0"})
            compiler.write_text("missing identity")
            with self.assertRaisesRegex(ValueError, "missing"):
                PRODUCER.compiler_identity(root)

    def test_source_identity_tracks_authored_bytes_and_rejects_sibling_imports(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for relative in (
                PRODUCER.LAUNCHER / "CMakeLists.txt", "LICENSE",
                "release/packaging/lib/elf_artifacts.py",
                "release/packaging/desktop/licenses/Qt-LGPL-3.txt",
                "release/packaging/desktop/licenses/Qt-GPL-3.txt",
            ):
                path = root / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(b"authored input")
            before = PRODUCER.source_identity(root)
            cache = root / PRODUCER.LAUNCHER / "build/cache"
            cache.parent.mkdir()
            cache.write_bytes(b"generated")
            self.assertEqual(PRODUCER.source_identity(root), before)
            (root / "LICENSE").write_bytes(b"changed license")
            self.assertNotEqual(PRODUCER.source_identity(root), before)
            (root / PRODUCER.LAUNCHER / "sibling").symlink_to(root, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "directory aliases"):
                PRODUCER.source_identity(root)


if __name__ == "__main__":
    unittest.main()
