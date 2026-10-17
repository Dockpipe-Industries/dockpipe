#!/usr/bin/env python3
import importlib.util
import hashlib
import io
from pathlib import Path
import signal
import tarfile
import tempfile
import time
import unittest
from unittest.mock import patch
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
            source = {"url": "https://example.invalid/tool", "sha256": hashlib.sha256(b"correct").hexdigest()}
            with patch.object(tools.urllib.request, "urlopen", return_value=io.BytesIO(b"correct")) as download:
                self.assertEqual(tools.verified_input("tool", source, cache).read_bytes(), b"correct")
            download.assert_called_once_with(source["url"], timeout=60)

    def test_checksum_mismatch_retries_only_once_and_leaves_no_bad_cache(self):
        with tempfile.TemporaryDirectory() as temporary:
            cache = Path(temporary)
            source = {"url": "https://example.invalid/tool", "sha256": hashlib.sha256(b"correct").hexdigest()}
            with patch.object(tools.urllib.request, "urlopen", side_effect=[
                io.BytesIO(b"wrong"), io.BytesIO(b"wrong-again"),
            ]) as download:
                with self.assertRaisesRegex(ValueError, "Checksum mismatch after a fresh retry"):
                    tools.verified_input("tool", source, cache)
            self.assertEqual(download.call_count, 2)
            self.assertFalse((cache / "tool").exists())

    def test_bad_first_download_is_replaced_by_verified_retry(self):
        with tempfile.TemporaryDirectory() as temporary:
            cache = Path(temporary)
            source = {"url": "https://example.invalid/tool", "sha256": hashlib.sha256(b"correct").hexdigest()}
            with patch.object(tools.urllib.request, "urlopen", side_effect=[
                io.BytesIO(b"wrong"), io.BytesIO(b"correct"),
            ]) as download:
                self.assertEqual(tools.verified_input("tool", source, cache).read_bytes(), b"correct")
            self.assertEqual(download.call_count, 2)

    def test_oversized_download_is_rejected_before_caching(self):
        with tempfile.TemporaryDirectory() as temporary:
            cache = Path(temporary)
            source = {"url": "https://example.invalid/tool", "sha256": "0" * 64}
            with patch.object(tools, "MAX_TOOL_ARCHIVE_BYTES", 3):
                with patch.object(tools.urllib.request, "urlopen", return_value=io.BytesIO(b"long")):
                    with self.assertRaisesRegex(ValueError, "exceeds capacity"):
                        tools.verified_input("tool", source, cache)
            self.assertFalse((cache / "tool").exists())

    def test_checksum_stream_stops_if_file_grows_after_open(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "tool"
            path.write_bytes(b"a")
            real_fdopen = tools.os.fdopen

            def grow_then_open(descriptor, mode):
                stream = real_fdopen(descriptor, mode)
                path.write_bytes(b"long")
                return stream

            with patch.object(tools, "MAX_TOOL_ARCHIVE_BYTES", 3):
                with patch.object(tools.os, "fdopen", side_effect=grow_then_open):
                    with self.assertRaisesRegex(ValueError, "exceeds capacity"):
                        tools.checksum(path)

    def test_linked_cache_file_and_parent_are_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            cache = root / "cache"
            cache.mkdir()
            outside = root / "outside"
            outside.write_bytes(b"untouched")
            source = {"url": "https://example.invalid/tool", "sha256": hashlib.sha256(b"correct").hexdigest()}

            (cache / "tool").symlink_to(outside)
            with self.assertRaisesRegex(ValueError, "Linked optional tool cache file"):
                tools.verified_input("tool", source, cache)
            (cache / "tool").unlink()

            linked_cache = root / "linked-cache"
            linked_cache.symlink_to(cache, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "Linked optional tool cache parent"):
                tools.verified_input("tool", source, linked_cache)
            self.assertEqual(outside.read_bytes(), b"untouched")

    def test_exclusive_temporary_file_ignores_old_predictable_symlink(self):
        with tempfile.TemporaryDirectory() as temporary:
            cache = Path(temporary)
            outside = cache / "outside"
            outside.write_bytes(b"untouched")
            (cache / "tool.download").symlink_to(outside)
            source = {"url": "https://example.invalid/tool", "sha256": hashlib.sha256(b"correct").hexdigest()}
            with patch.object(tools.urllib.request, "urlopen", return_value=io.BytesIO(b"correct")):
                self.assertEqual(tools.verified_input("tool", source, cache).read_bytes(), b"correct")
            self.assertEqual(outside.read_bytes(), b"untouched")
            self.assertTrue((cache / "tool.download").is_symlink())

    def test_total_deadline_interrupts_blocked_read_and_restores_alarm(self):
        class SlowResponse(io.BytesIO):
            def read1(self, size):
                time.sleep(0.2)
                return super().read1(size)

        with tempfile.TemporaryDirectory() as temporary:
            cache = Path(temporary)
            source = {"url": "https://example.invalid/tool", "sha256": hashlib.sha256(b"correct").hexdigest()}
            previous_handler = signal.getsignal(signal.SIGALRM)
            previous_timer = signal.getitimer(signal.ITIMER_REAL)

            def prior_alarm(_signum, _frame):
                raise AssertionError("Prior alarm should not fire during the fixture")

            try:
                signal.signal(signal.SIGALRM, prior_alarm)
                signal.setitimer(signal.ITIMER_REAL, 1)
                with patch.object(tools.urllib.request, "urlopen", return_value=SlowResponse(b"correct")):
                    with patch.object(tools, "INPUT_TIMEOUT_SECONDS", 0.01):
                        with self.assertRaisesRegex(TimeoutError, "wall time limit"):
                            tools.verified_input("tool", source, cache)
                self.assertIs(signal.getsignal(signal.SIGALRM), prior_alarm)
                self.assertGreater(signal.getitimer(signal.ITIMER_REAL)[0], 0.5)
            finally:
                signal.setitimer(signal.ITIMER_REAL, 0)
                signal.signal(signal.SIGALRM, previous_handler)
                if previous_timer[0]:
                    signal.setitimer(signal.ITIMER_REAL, *previous_timer)
            self.assertFalse((cache / "tool").exists())


if __name__ == "__main__":
    unittest.main()
