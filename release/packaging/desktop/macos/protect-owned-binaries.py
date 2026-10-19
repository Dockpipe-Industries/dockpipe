#!/usr/bin/env python3
"""Strip and verify only Dockpipe's native Mach-O payload before signing."""
import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path
import re
import struct
import subprocess


HOST_LIBRARY = re.compile(r"libdockpipe-launcher-ui(?:\.[0-9]+)*\.dylib\Z")


LINKER_MARKER = "radr://5614542"
LC_SYMTAB = 0x2
LC_DYSYMTAB = 0xB
N_OPT = 0x3C
LINKER_MARKER_VALUE = 0x05614542


def required_linker_markers(binary):
    """Recognize only Apple's exact strip-inserted dyld index-zero marker.

    Apple's cctools misc/strip.c inserts this N_OPT compatibility sentinel
    when removing the last local symbol from an image with indirect symbols.
    It prevents an indirect symbol from occupying dyld's reserved index zero.
    This is a typed format exception, never a general symbol-name allowlist.
    """
    with binary.open("rb") as stream:
        file_size = binary.stat().st_size
        header = stream.read(32)
        if len(header) != 32:
            raise ValueError("Truncated owned Mach-O header")
        magic, cpu, _, file_type, command_count, command_bytes, _, _ = struct.unpack("<8I", header)
        if magic != 0xFEEDFACF or cpu not in {0x01000007, 0x0100000C} or file_type not in {2, 6}:
            raise ValueError("Owned symbol audit requires a single 64-bit little-endian macOS image")
        if command_count > 4096 or command_bytes > 4 * 1024 * 1024 or 32 + command_bytes > file_size:
            raise ValueError("Invalid owned Mach-O load-command bounds")
        commands = stream.read(command_bytes)
        symbol_table = None
        dynamic_table = None
        offset = 0
        for _ in range(command_count):
            if offset + 8 > len(commands):
                raise ValueError("Truncated owned Mach-O load command")
            kind, size = struct.unpack_from("<2I", commands, offset)
            if size < 8 or size % 8 != 0 or offset + size > len(commands):
                raise ValueError("Invalid owned Mach-O load-command size")
            if kind == LC_SYMTAB:
                if size != 24 or symbol_table is not None:
                    raise ValueError("Invalid or duplicate owned Mach-O symbol table")
                symbol_table = struct.unpack_from("<4I", commands, offset + 8)
            elif kind == LC_DYSYMTAB:
                if size != 80 or dynamic_table is not None:
                    raise ValueError("Invalid or duplicate owned Mach-O dynamic symbol table")
                dynamic_table = struct.unpack_from("<18I", commands, offset + 8)
            offset += size
        if offset != command_bytes or symbol_table is None or dynamic_table is None:
            raise ValueError("Incomplete owned Mach-O symbol metadata")
        symbol_offset, symbol_count, string_offset, string_bytes = symbol_table
        local_index, local_count = dynamic_table[:2]
        # LC_DYSYMTAB fields 12/13 are indirectsymoff/nindirectsyms.
        indirect_offset, indirect_count = dynamic_table[12:14]
        if (symbol_count > 5_000_000 or string_bytes > 64 * 1024 * 1024
                or symbol_offset + symbol_count * 16 > file_size
                or string_offset + string_bytes > file_size
                or local_index + local_count > symbol_count
                or indirect_offset + indirect_count * 4 > file_size):
            raise ValueError("Invalid owned Mach-O symbol-table bounds")
        if symbol_count == 0 or local_index != 0 or local_count == 0 or indirect_count == 0:
            return 0
        stream.seek(symbol_offset)
        string_index, symbol_type, section, description, value = struct.unpack("<IBBHQ", stream.read(16))
        if (symbol_type, section, description, value) != (N_OPT, 0, 0, LINKER_MARKER_VALUE):
            return 0
        expected_name = LINKER_MARKER.encode("ascii") + b"\0"
        if string_index + len(expected_name) > string_bytes:
            return 0
        stream.seek(string_offset + string_index)
        return int(stream.read(len(expected_name)) == expected_name)


def run_tool(*arguments):
    return subprocess.check_output(arguments, text=True, stderr=subprocess.PIPE, timeout=60)


def defined_symbols(binary, external_only):
    arguments = ["nm", "-U", "-j"]
    arguments.append("-g" if external_only else "-a")
    arguments.append(str(binary))
    return Counter(line.strip() for line in run_tool(*arguments).splitlines() if line.strip())


def describe_private_symbols(binary, symbols):
    # Keep hosted failure logs useful without dumping a complete symbol table.
    maximum_samples = 8
    maximum_field_length = 200
    selected = sorted(symbols)[:maximum_samples]
    descriptions = {}
    for line in run_tool("nm", "-m", "-a", "-U", str(binary)).splitlines():
        fields = line.rsplit(None, 1)
        if len(fields) == 2 and fields[1] in selected:
            descriptions[fields[1]] = fields[0].strip()[:maximum_field_length]
    samples = [{
        "name": name[:maximum_field_length],
        "type": descriptions.get(name, "No matching nm type description"),
    } for name in selected]
    return json.dumps({"count": sum(symbols.values()), "samples": samples}, separators=(",", ":"))


def owned_binaries(app):
    app = app.resolve(strict=True)
    launcher = app / "Contents/MacOS/DockPipe"
    if not launcher.is_file() or not launcher.resolve().is_relative_to(app):
        raise ValueError("The application is missing its owned launcher binary")
    libraries = set()
    for candidate in (app / "Contents/Frameworks").rglob("libdockpipe-launcher-ui*.dylib"):
        if not HOST_LIBRARY.fullmatch(candidate.name):
            raise ValueError("Unexpected owned host library name: " + candidate.name)
        binary = candidate.resolve(strict=True)
        if not binary.is_file() or not binary.is_relative_to(app):
            raise ValueError("The owned host library escapes the application bundle")
        libraries.add(binary)
    if len(libraries) != 1:
        raise ValueError("Expected exactly one owned host UI binary, found " + str(len(libraries)))
    return [launcher.resolve(), next(iter(libraries))]


def protect(app, receipt):
    app = app.resolve(strict=True)
    records = []
    for binary in owned_binaries(app):
        run_tool("otool", "-hv", str(binary))
        exported_before = defined_symbols(binary, external_only=True)
        if not exported_before:
            raise ValueError("The owned binary has no defined external ABI: " + binary.name)
        subprocess.run(["strip", "-S", "-x", str(binary)], check=True, timeout=60)
        load_commands = run_tool("otool", "-l", str(binary))
        for line in load_commands.splitlines():
            fields = line.split()
            if len(fields) >= 2 and fields[0] in {"segname", "sectname"}:
                name = fields[1].lower()
                if "dwarf" in name or name.startswith("__debug") or name.startswith("__zdebug"):
                    raise ValueError("Private debug section remains in " + binary.name + ": " + fields[1])
        exported_after = defined_symbols(binary, external_only=True)
        if exported_after != exported_before:
            raise ValueError("Stripping changed the exported ABI of " + binary.name)
        linker_markers = required_linker_markers(binary)
        local_symbols = defined_symbols(binary, external_only=False) - exported_after
        local_symbols -= Counter({LINKER_MARKER: linker_markers})
        if local_symbols:
            detail = describe_private_symbols(binary, local_symbols)
            raise ValueError("Private defined symbols remain in " + binary.name + ": " + detail)
        records.append({
            "path": binary.relative_to(app).as_posix(),
            "pre_sign_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
            "exported_symbols": sum(exported_after.values()),
            "required_linker_markers": linker_markers,
            "private_defined_symbols": 0,
            "debug_sections": [],
        })
    receipt.write_text(json.dumps({"schema_version": 1, "owned_binaries": records}, indent=2) + "\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("app", type=Path)
    parser.add_argument("receipt", type=Path)
    arguments = parser.parse_args()
    protect(arguments.app, arguments.receipt)


if __name__ == "__main__":
    main()
