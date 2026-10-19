"""Audit actual data-only runtime ELF files without relaxing the SDK contract."""
import importlib.util
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "assets/scripts/protect-runtime.py"
SPEC = importlib.util.spec_from_file_location("cockpit_runtime", SCRIPT)
RUNTIME = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RUNTIME)


@unittest.skipUnless(sys.platform.startswith("linux"), "ELF runtime protection is Linux-specific")
class RuntimeProtectionTests(unittest.TestCase):
    def test_data_only_dso_is_accepted_only_after_private_sections_are_removed(self):
        # These are required by the Linux native build, not optional substitutes
        # for an audit. Missing tools fail the native host's contract explicitly.
        for tool in ("as", "ld", "strip", "readelf", "objcopy"):
            self.assertIsNotNone(shutil.which(tool), f"Native ELF audit tool is required: {tool}")
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / "tables.s"
            source.write_text('.section .rodata\n.global fixture_data\n'
                              'fixture_data:\n.byte 1,2,3,4\n'
                              '.section .note.GNU-stack,""\n')
            object_path = root / "tables.o"
            binary = root / "libtables.so"
            subprocess.run(["as", "-o", str(object_path), str(source)], check=True, timeout=30)
            subprocess.run(["ld", "-shared", "--build-id", "-o", str(binary), str(object_path)],
                           check=True, timeout=30)
            sections = subprocess.check_output(["readelf", "--wide", "--sections", str(binary)], text=True, timeout=30)
            self.assertNotIn(".text", sections)
            with self.assertRaisesRegex(ValueError, "Private symbol sections"):
                RUNTIME.verify_public_sections(binary)
            subprocess.run(["strip", "--strip-unneeded", str(binary)], check=True, timeout=30)
            RUNTIME.verify_public_sections(binary)
            sdk_auditor = RUNTIME.tools(SCRIPT.parents[4])
            with self.assertRaisesRegex(ValueError, "text section"):
                sdk_auditor.sections(binary)
            payload = root / "private-data"
            payload.write_bytes(b"private diagnostics")
            for index, name in enumerate((".debug_info", ".zdebug_info", ".gnu_debugdata", ".gdb_index")):
                changed = root / f"private-{index}.so"
                subprocess.run(["objcopy", "--add-section", f"{name}={payload}", str(binary), str(changed)],
                               check=True, timeout=30)
                with self.subTest(section=name), self.assertRaisesRegex(ValueError, "Private symbol sections"):
                    RUNTIME.verify_public_sections(changed)


if __name__ == "__main__":
    unittest.main()
