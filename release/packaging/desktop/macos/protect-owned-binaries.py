#!/usr/bin/env python3
"""Strip and verify only Dockpipe's native Mach-O payload before signing."""
import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path
import re
import subprocess


HOST_LIBRARY = re.compile(r"libdockpipe-launcher-ui(?:\.[0-9]+)*\.dylib\Z")


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
        local_symbols = defined_symbols(binary, external_only=False) - exported_after
        if local_symbols:
            detail = describe_private_symbols(binary, local_symbols)
            raise ValueError("Private defined symbols remain in " + binary.name + ": " + detail)
        records.append({
            "path": binary.relative_to(app).as_posix(),
            "pre_sign_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
            "exported_symbols": sum(exported_after.values()),
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
