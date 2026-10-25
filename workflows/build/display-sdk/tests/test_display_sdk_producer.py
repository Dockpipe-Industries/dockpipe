"""Display SDK archives retain interface provenance without importing native runtimes."""
import importlib.util
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest import mock


SCRIPT = Path(__file__).resolve().parents[1] / "assets/scripts/build.py"
SPEC = importlib.util.spec_from_file_location("display_sdk_producer", SCRIPT)
PRODUCER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PRODUCER)


class DisplaySDKProducerTests(unittest.TestCase):
    def test_native_architectures_and_unknown_platform(self):
        self.assertEqual(PRODUCER.native_platform("Windows", "AMD64"), "windows-amd64")
        self.assertEqual(PRODUCER.native_platform("Darwin", "arm64"), "darwin-arm64")
        self.assertEqual(PRODUCER.native_platform("Linux", "aarch64"), "linux-arm64")
        with self.assertRaisesRegex(ValueError, "Unsupported"):
            PRODUCER.native_platform("unknown", "x86_64")

    def test_header_payload_archive_is_relocatable_and_deterministic(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            prefix = root / "prefix"
            for relative in PRODUCER.INSTALLED_PATHS:
                path = prefix / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("public interface\n")
            entries = PRODUCER.installed_inventory(prefix)
            first = root / "first.tar.gz"
            second = root / "second.tar.gz"
            PRODUCER.ARCHIVE_HELPER.write_archive(prefix, first, entries, "DisplaySDK")
            PRODUCER.ARCHIVE_HELPER.write_archive(prefix, second, entries, "DisplaySDK")
            self.assertEqual(first.read_bytes(), second.read_bytes())
            with tarfile.open(first) as archive:
                self.assertEqual(archive.getnames(), ["DisplaySDK/" + path for path in sorted(PRODUCER.INSTALLED_PATHS)])

    def test_runtime_and_payload_capacity_fail_closed(self):
        with tempfile.TemporaryDirectory() as temporary:
            prefix = Path(temporary)
            payload = prefix / "libunexpected.so.1"
            payload.write_bytes(b"runtime")
            with self.assertRaisesRegex(ValueError, "unexpected installed"):
                PRODUCER.installed_inventory(prefix)
            payload.unlink()
            (prefix / "oversized.h").write_bytes(b"x" * (PRODUCER.MAX_BYTES + 1))
            with mock.patch.object(PRODUCER.ARCHIVE_HELPER, "digest") as checksum:
                with self.assertRaisesRegex(ValueError, "distribution contract"):
                    PRODUCER.installed_inventory(prefix)
                checksum.assert_not_called()

    def test_actual_machine_header_distinguishes_rosetta_consumer(self):
        with tempfile.TemporaryDirectory() as temporary:
            executable = Path(temporary) / "consumer"
            executable.write_bytes(b"\xcf\xfa\xed\xfe\x07\x00\x00\x01" + bytes(56))
            self.assertEqual(PRODUCER.consumer_platform(executable), "darwin-amd64")
            executable.write_bytes(b"\xcf\xfa\xed\xfe\x0c\x00\x00\x01" + bytes(56))
            self.assertEqual(PRODUCER.consumer_platform(executable), "darwin-arm64")
            executable.write_bytes(b"\xcf\xfa")
            with self.assertRaisesRegex(ValueError, "truncated"):
                PRODUCER.consumer_platform(executable)

    def test_empty_explicit_qt_root_cannot_fall_back_to_ambient_kit(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            with mock.patch.object(PRODUCER.subprocess, "run") as command:
                with self.assertRaisesRegex(ValueError, "Selected Qt root"):
                    PRODUCER.build(root, None, root, root / "work", root / "output", 2, "cmake", "ctest", None)
                command.assert_not_called()

    def test_source_digest_binds_interface_and_consumer(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for relative in (
                "LICENSE", "workflows/build/display-sdk/assets/scripts/build.py", "workflows/shared/archive.py",
                PRODUCER.LAUNCHER / "src/LauncherDisplayPackage.h",
                PRODUCER.LAUNCHER / "src/LauncherNativeLibraryScope.h",
                PRODUCER.LAUNCHER / "display-sdk/CMakeLists.txt",
                PRODUCER.LAUNCHER / "display-sdk/tests/consumer/main.cpp",
            ):
                path = root / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("authored bytes")
            before = PRODUCER.source_identity(root)
            consumer = root / PRODUCER.LAUNCHER / "display-sdk/tests/consumer/main.cpp"
            consumer.write_text("changed consumer")
            self.assertNotEqual(PRODUCER.source_identity(root), before)
            consumer.unlink()
            consumer.symlink_to(root / "LICENSE")
            with self.assertRaisesRegex(ValueError, "regular authored"):
                PRODUCER.source_identity(root)


if __name__ == "__main__":
    unittest.main()
