"""Check owned native payload selection and fail-closed symbol audits."""
import importlib.util
import json
from pathlib import Path
import shutil
import struct
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch


DESKTOP = Path(__file__).resolve().parents[1] / "desktop"
spec = importlib.util.spec_from_file_location("mac_owned_symbols", DESKTOP / "macos/protect-owned-binaries.py")
protector = importlib.util.module_from_spec(spec)
spec.loader.exec_module(protector)


def macho_marker_fixture(name="radr://5614542", symbol_type=0x3C, section=0,
                         description=0, value=0x05614542, index=0, indirect_count=1):
    """A bounded Mach-O header, load commands, nlist and indirect table."""
    command_bytes = 104
    symbol_offset = 32 + command_bytes
    symbol_count = index + 1
    string_offset = symbol_offset + symbol_count * 16
    strings = b"\0" + name.encode("ascii") + b"\0"
    indirect_offset = string_offset + len(strings)
    dynamic_table = [0] * 18
    dynamic_table[0:2] = [0, symbol_count]
    dynamic_table[12:14] = [indirect_offset, indirect_count]
    header = struct.pack("<8I", 0xFEEDFACF, 0x0100000C, 0, 2, 2, command_bytes, 4, 0)
    commands = struct.pack("<6I", 2, 24, symbol_offset, symbol_count, string_offset, len(strings))
    commands += struct.pack("<20I", 0xB, 80, *dynamic_table)
    symbols = b"\0" * (index * 16)
    symbols += struct.pack("<IBBHQ", 1, symbol_type, section, description, value)
    return header + commands + symbols + strings + b"\0" * (indirect_count * 4)


class NativeSymbolBoundaryTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="dockpipe native symbols ")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()
        self.app = self.root / "DockPipe.app"
        self.launcher = self.app / "Contents/MacOS/DockPipe"
        self.library = self.app / "Contents/Frameworks/libdockpipe-launcher-ui.1.dylib"
        for binary in (self.launcher, self.library):
            binary.parent.mkdir(parents=True, exist_ok=True)
            binary.write_bytes(b"native binary fixture")
        self.receipt = self.root / "audit.json"

    def test_selects_one_owned_library_through_version_symlinks(self):
        alias = self.library.with_name("libdockpipe-launcher-ui.dylib")
        alias.symlink_to(self.library.name)
        third_party = self.library.with_name("libQt6Core.dylib")
        third_party.write_bytes(b"third party")
        self.assertEqual(protector.owned_binaries(self.app), [self.launcher, self.library])
        self.library.with_name("libdockpipe-launcher-ui.2.dylib").write_bytes(b"ambiguous version")
        with self.assertRaisesRegex(ValueError, "exactly one"):
            protector.owned_binaries(self.app)

    def test_rejects_missing_and_escaping_owned_library(self):
        self.library.unlink()
        with self.assertRaisesRegex(ValueError, "exactly one"):
            protector.owned_binaries(self.app)
        external = self.root / "outside.dylib"
        external.write_bytes(b"external")
        self.library.symlink_to(external)
        with self.assertRaisesRegex(ValueError, "escapes"):
            protector.owned_binaries(self.app)

    def test_symbol_and_debug_audit_preserves_exported_abi(self):
        for failure in (None, "debug", "local", "abi"):
            with self.subTest(failure=failure):
                self.receipt.unlink(missing_ok=True)
                stripped = set()

                def run_tool(*arguments):
                    binary = arguments[-1]
                    if arguments[:2] == ("otool", "-hv"):
                        return "MH_MAGIC_64\n"
                    if arguments[:2] == ("otool", "-l"):
                        if failure == "debug":
                            return "segname __DWARF\nsectname __debug_info\n"
                        return "segname __TEXT\nsectname __text\n"
                    if "-m" in arguments:
                        return "0000000100000010 (__TEXT,__text) non-external _private_function\n"
                    if "-g" in arguments:
                        return "_changed_export\n" if failure == "abi" and binary in stripped else "_required_export\n"
                    return "_required_export\n_private_function\n" if failure == "local" else "_required_export\n"

                def strip(arguments, **options):
                    self.assertEqual(arguments[:3], ["strip", "-S", "-x"])
                    stripped.add(arguments[-1])

                with patch.object(protector, "run_tool", side_effect=run_tool), \
                        patch.object(protector, "required_linker_markers", return_value=0), \
                        patch.object(protector.subprocess, "run", side_effect=strip):
                    if failure:
                        with self.assertRaises(ValueError):
                            protector.protect(self.app, self.receipt)
                        self.assertFalse(self.receipt.exists())
                    else:
                        protector.protect(self.app, self.receipt)
                        audit = json.loads(self.receipt.read_text())
                        self.assertEqual(len(audit["owned_binaries"]), 2)
                        self.assertTrue(all(item["exported_symbols"] == 1 for item in audit["owned_binaries"]))
                        self.assertTrue(all(item["private_defined_symbols"] == 0 for item in audit["owned_binaries"]))

    def test_private_symbol_diagnostics_are_typed_and_bounded(self):
        symbols = protector.Counter({"_private_%02d" % index: 1 for index in range(20)})
        listing = "\n".join(
            "0000000100000010 (__TEXT,__text) non-external " + name for name in symbols
        )
        with patch.object(protector, "run_tool", return_value=listing):
            diagnostic = json.loads(protector.describe_private_symbols(self.launcher, symbols))
        self.assertEqual(diagnostic["count"], 20)
        self.assertEqual(len(diagnostic["samples"]), 8)
        self.assertTrue(all("non-external" in item["type"] for item in diagnostic["samples"]))
        self.assertEqual(diagnostic["samples"][0]["name"], "_private_00")
        self.assertNotIn("_private_19", json.dumps(diagnostic))

    def test_only_exact_typed_linker_marker_at_index_zero_is_classified(self):
        binary = self.root / "typed-marker"
        binary.write_bytes(macho_marker_fixture())
        self.assertEqual(protector.required_linker_markers(binary), 1)
        for changed in (
            {"name": "private_function"},
            {"symbol_type": 0x0E},
            {"section": 1},
            {"description": 1},
            {"value": 1},
            {"index": 1},
            {"indirect_count": 0},
        ):
            with self.subTest(changed=changed):
                binary.write_bytes(macho_marker_fixture(**changed))
                self.assertEqual(protector.required_linker_markers(binary), 0)

    def test_required_marker_does_not_excuse_another_private_symbol(self):
        self.launcher.write_bytes(macho_marker_fixture())

        def run_tool(*arguments):
            if arguments[:2] == ("otool", "-l"):
                return "segname __TEXT\nsectname __text\n"
            if "-g" in arguments:
                return "_required_export\n"
            if "-m" in arguments:
                return "0000000100000010 (__TEXT,__text) non-external _private_function\n"
            return "_required_export\nradr://5614542\n_private_function\n"

        with patch.object(protector, "run_tool", side_effect=run_tool), \
                patch.object(protector.subprocess, "run"):
            with self.assertRaisesRegex(ValueError, "_private_function"):
                protector.protect(self.app, self.receipt)
        self.assertFalse(self.receipt.exists())

    def test_marker_parser_rejects_unknown_images_and_invalid_table_bounds(self):
        binary = self.root / "invalid-marker"
        valid = macho_marker_fixture()
        variants = [b"truncated", b"\xca\xfe\xba\xbe" + valid[4:]]
        unknown_cpu = bytearray(valid)
        struct.pack_into("<I", unknown_cpu, 4, 0)
        variants.append(unknown_cpu)
        invalid_command = bytearray(valid)
        struct.pack_into("<I", invalid_command, 36, 0xFFFFFFFF)
        variants.append(invalid_command)
        invalid_symbols = bytearray(valid)
        struct.pack_into("<I", invalid_symbols, 44, 0xFFFFFFFF)
        variants.append(invalid_symbols)
        invalid_indirect = bytearray(valid)
        struct.pack_into("<I", invalid_indirect, 56 + 8 + 13 * 4, 0xFFFFFFFF)
        variants.append(invalid_indirect)
        for invalid in variants:
            with self.subTest(header=bytes(invalid[:8])):
                binary.write_bytes(invalid)
                with self.assertRaises(ValueError):
                    protector.required_linker_markers(binary)

    @unittest.skipUnless(sys.platform == "darwin" and shutil.which("clang"), "Requires native Apple Mach-O tools")
    def test_strips_real_macho_debug_and_local_symbols_without_losing_exports(self):
        source = self.root / "fixture.c"
        source.write_text(
            "#include <stdio.h>\n"
            "__attribute__((noinline)) static int private_impl(int n) { return n + 1; }\n"
            "int exported_api(int n) { return private_impl(n); }\n"
            'int main(void) { puts("native symbol fixture"); return exported_api(1); }\n'
        )
        # Keep the debug object private. Passing -g while linking would make
        # clang create an automatic dSYM bundle beside the public fixture.
        debug_object = self.root / "fixture.o"
        subprocess.run(["clang", "-O0", "-g", "-c", str(source), "-o", str(debug_object)], check=True)
        subprocess.run(["clang", str(debug_object), "-o", str(self.launcher)], check=True)
        subprocess.run(["clang", "-dynamiclib", str(debug_object), "-o", str(self.library)], check=True)
        protector.protect(self.app, self.receipt)
        records = json.loads(self.receipt.read_text())["owned_binaries"]
        self.assertEqual(len(records), 2)
        self.assertTrue(all(item["required_linker_markers"] == 1 for item in records))
        self.assertTrue(all(item["private_defined_symbols"] == 0 for item in records))


if __name__ == "__main__":
    unittest.main()
