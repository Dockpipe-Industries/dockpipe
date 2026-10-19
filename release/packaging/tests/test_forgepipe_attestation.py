"""Regression gates for the exact public SDK inventory and ELF disclosure boundary."""
import importlib.util
import io
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "attest-forgepipe.py"
SPEC = importlib.util.spec_from_file_location("attest_forgepipe", SCRIPT)
PUBLISHER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PUBLISHER)


@unittest.skipUnless(shutil.which("cc") and shutil.which("readelf") and shutil.which("nm"),
                     "native ELF inspection tools are required")
class ForgePipePublisherTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="forgepipe-public-sdk-test-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        source = self.root / "fixture.c"
        source.write_text("int public_entry(void) { return 7; }\n")
        self.binary = self.root / "fixture.so"
        subprocess.run(["cc", "-shared", "-fPIC", "-g", "-Wl,--build-id",
                        str(source), "-o", str(self.binary)], check=True)
        self.debug_payload = self.binary.read_bytes()
        subprocess.run(["strip", "--strip-unneeded", str(self.binary)], check=True)
        self.runtime_payload = self.binary.read_bytes()

    def archive(self, *, extra=None, missing=None, debug=False, changed_alias=False, link=False):
        path = self.root / "sdk.tar.gz"
        with tarfile.open(path, "w:gz") as archive:
            for name in sorted(PUBLISHER.SDK_FILES):
                if name == missing:
                    continue
                payload = self.runtime_payload if name in PUBLISHER.LIBRARIES else b"Public SDK declaration\n"
                if debug and name in PUBLISHER.LIBRARIES:
                    payload = self.debug_payload
                if changed_alias and name == "lib/libforgepipe-engine.so":
                    payload += b"different alias bytes"
                member = tarfile.TarInfo("ForgePipe-SDK-0.1.0-Linux/" + name)
                member.size = len(payload)
                if link and name == "lib/libforgepipe-engine.so":
                    member.type = tarfile.SYMTYPE
                    member.linkname = "libforgepipe-engine.so.0.1.0"
                    archive.addfile(member)
                else:
                    archive.addfile(member, io.BytesIO(payload))
            if extra:
                member = tarfile.TarInfo("ForgePipe-SDK-0.1.0-Linux/" + extra)
                member.size = 1
                archive.addfile(member, io.BytesIO(b"x"))
        return path

    def verify(self, **options):
        return PUBLISHER.verify_archive(self.archive(**options), self.root, ["public_entry"])

    def test_stripped_approved_sdk_has_an_inspectable_build_id(self):
        receipt = self.verify()
        self.assertTrue(receipt["build_id"])
        self.assertEqual(receipt["exports"], ["public_entry"])

    def test_private_debug_information_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "private symbols or debug"):
            self.verify(debug=True)

    def test_unapproved_source_and_debug_payloads_are_rejected(self):
        for extra in ("src/private.cpp", "debug/engine.debug", "include/implementation.h"):
            with self.subTest(extra=extra), self.assertRaisesRegex(ValueError, "Unapproved SDK"):
                self.verify(extra=extra)

    def test_missing_sdk_file_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "inventory"):
            self.verify(missing="share/forgepipe/LICENSE")

    def test_aliases_must_be_identical_regular_files(self):
        with self.assertRaisesRegex(ValueError, "identical"):
            self.verify(changed_alias=True)
        with self.assertRaisesRegex(ValueError, "Unapproved SDK"):
            self.verify(link=True)

    def test_archive_path_escape_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "archive member"):
            self.verify(extra="../../outside")


if __name__ == "__main__":
    unittest.main()
