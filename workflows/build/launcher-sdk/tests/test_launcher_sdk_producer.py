"""Deterministic host SDK archives and owned installed payload boundaries."""
import importlib.util
import json
import shutil
from unittest.mock import patch
import os
from pathlib import Path
import tarfile
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "assets/scripts/build.py"
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
                "workflows/shared/archive.py", "workflows/shared/lib/elf_artifacts.py",
                "workflows/build/desktop/assets/licenses/Qt-LGPL-3.txt",
                "workflows/build/desktop/assets/licenses/Qt-GPL-3.txt",
                "workflows/build/launcher-sdk/assets/scripts/build.py",
                "workflows/dependencies/browser-sdk/assets/scripts/prepare-browser-sdk.py",
                "workflows/dependencies/browser-sdk/assets/scripts/browser-runtime.py",
                "workflows/dependencies/browser-sdk/assets/spatialweb-browser.lock.json",
            ):
                path = root / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(b"authored input")
            before = PRODUCER.source_identity(root)
            cache = root / PRODUCER.LAUNCHER / "build/cache"
            cache.parent.mkdir()
            cache.write_bytes(b"generated")
            self.assertEqual(PRODUCER.source_identity(root), before)
            dependency = root / "workflows/dependencies/browser-sdk/assets/spatialweb-browser.lock.json"
            dependency.write_bytes(b"changed dependency pin")
            self.assertNotEqual(PRODUCER.source_identity(root), before)
            dependency.write_bytes(b"authored input")
            (root / "LICENSE").write_bytes(b"changed license")
            self.assertNotEqual(PRODUCER.source_identity(root), before)
            (root / PRODUCER.LAUNCHER / "sibling").symlink_to(root, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "directory aliases"):
                PRODUCER.source_identity(root)


FIXTURE_SPEC = importlib.util.spec_from_file_location("host_browser_fixtures", Path(__file__).resolve().parents[4] / "workflows/dependencies/browser-sdk/tests/test_prepare_browser_sdk.py")
FIXTURES = importlib.util.module_from_spec(FIXTURE_SPEC)
FIXTURE_SPEC.loader.exec_module(FIXTURES)


class BrowserDependencyBindingTests(unittest.TestCase):
    setUp = FIXTURES.BrowserSDKPreparationTest.setUp
    tearDown = FIXTURES.BrowserSDKPreparationTest.tearDown
    write_lock = FIXTURES.BrowserSDKPreparationTest.write_lock
    make_candidate = FIXTURES.BrowserSDKPreparationTest.make_candidate
    prepare = FIXTURES.BrowserSDKPreparationTest.prepare

    def source_fixture(self):
        self.prepare()
        source = self.root / "source"
        repository = SCRIPT.parents[5]
        for relative in ("workflows/dependencies/browser-sdk/assets/scripts/prepare-browser-sdk.py", "workflows/dependencies/browser-sdk/assets/scripts/browser-runtime.py"):
            destination = source / relative
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(repository / relative, destination)
        lock_path = source / "workflows/dependencies/browser-sdk/assets/spatialweb-browser.lock.json"
        lock_path.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(self.lock_path, lock_path)
        return source

    def test_producer_rejects_injected_browser_file_and_forged_receipt_identity(self):
        source = self.source_fixture()
        dependency = PRODUCER.browser_dependency(source, self.output)
        self.assertEqual(self.lock["platforms"]["linux-amd64"]["compiler"], dependency["compiler"])
        extra = self.output / "lib/injected.so"
        extra.write_bytes(b"unadmitted library")
        with self.assertRaisesRegex(ValueError, "inventory"):
            PRODUCER.browser_dependency(source, self.output)
        extra.unlink()
        receipt_path = self.output / "consumer-receipt.json"
        receipt = json.loads(receipt_path.read_text())
        receipt["identity"]["compiler"]["version"] = "13.3.0"
        receipt_path.write_text(json.dumps(receipt))
        with self.assertRaisesRegex(ValueError, "receipt"):
            PRODUCER.browser_dependency(source, self.output)

    def test_packaging_rejects_compiler_abi_mismatch_before_writing_assets(self):
        browser = {"compiler": {"id": "GNU", "version": "11.4.0"}}
        with patch.object(PRODUCER, "compiler_identity", return_value={"id": "GNU", "version": "13.3.0"}), \
                patch.object(PRODUCER, "inventory") as inventory:
            with self.assertRaisesRegex(ValueError, "Browser SDK ABI"):
                PRODUCER.package_sdk(self.root, self.root, "a" * 40, "b" * 64, self.root, browser)
            inventory.assert_not_called()
        with patch.object(PRODUCER, "compiler_identity", return_value=browser["compiler"]):
            PRODUCER.verify_browser_compiler(self.root, browser)


if __name__ == "__main__":
    unittest.main()
