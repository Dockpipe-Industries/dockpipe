"""Check owned native payload selection and fail-closed symbol audits."""
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch


DESKTOP = Path(__file__).resolve().parents[1] / "desktop"
spec = importlib.util.spec_from_file_location("mac_owned_symbols", DESKTOP / "macos/protect-owned-binaries.py")
protector = importlib.util.module_from_spec(spec)
spec.loader.exec_module(protector)


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
                    if "-g" in arguments:
                        return "_changed_export\n" if failure == "abi" and binary in stripped else "_required_export\n"
                    return "_required_export\n_private_function\n" if failure == "local" else "_required_export\n"

                def strip(arguments, **options):
                    self.assertEqual(arguments[:3], ["strip", "-S", "-x"])
                    stripped.add(arguments[-1])

                with patch.object(protector, "run_tool", side_effect=run_tool), \
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

    @unittest.skipUnless(sys.platform == "darwin" and shutil.which("clang"), "Requires native Apple Mach-O tools")
    def test_strips_real_macho_debug_and_local_symbols_without_losing_exports(self):
        source = self.root / "fixture.c"
        source.write_text(
            "__attribute__((noinline)) static int private_impl(int n) { return n + 1; }\n"
            "int exported_api(int n) { return private_impl(n); }\n"
            "int main(void) { return exported_api(1); }\n"
        )
        # Keep the debug object private. Passing -g while linking would make
        # clang create an automatic dSYM bundle beside the public fixture.
        debug_object = self.root / "fixture.o"
        subprocess.run(["clang", "-O0", "-g", "-c", str(source), "-o", str(debug_object)], check=True)
        subprocess.run(["clang", str(debug_object), "-o", str(self.launcher)], check=True)
        subprocess.run(["clang", "-dynamiclib", str(debug_object), "-o", str(self.library)], check=True)
        protector.protect(self.app, self.receipt)
        self.assertEqual(len(json.loads(self.receipt.read_text())["owned_binaries"]), 2)


if __name__ == "__main__":
    unittest.main()
