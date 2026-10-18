"""Audit and split ELF SDK binaries without changing executable section bytes."""
import hashlib
from pathlib import Path
import re
import subprocess


def run(arguments):
    result = subprocess.run(arguments, capture_output=True, text=True, timeout=60)
    if result.returncode:
        raise ValueError(f"{Path(arguments[0]).name} failed (exit {result.returncode})")
    return result.stdout


def sections(binary):
    output = run(["readelf", "--wide", "--sections", str(binary)])
    result = {}
    pattern = re.compile(
        r"^\s*\[\s*\d+\]\s+(\S+)\s+(\S+)\s+([0-9a-fA-F]+)\s+"
        r"([0-9a-fA-F]+)\s+([0-9a-fA-F]+)\s+[0-9a-fA-F]+\s+(\S*)"
    )
    with Path(binary).open("rb") as stream:
        for line in output.splitlines():
            match = pattern.match(line)
            if not match:
                continue
            name, kind, address, offset, size, flags = match.groups()
            digest = None
            if "A" in flags and kind != "NOBITS":
                stream.seek(int(offset, 16))
                remaining = int(size, 16)
                checksum = hashlib.sha256()
                while remaining:
                    block = stream.read(min(remaining, 1024 * 1024))
                    if not block:
                        raise ValueError("Truncated allocated ELF section")
                    checksum.update(block)
                    remaining -= len(block)
                digest = checksum.hexdigest()
            result[name] = {"type": kind, "address": int(address, 16), "size": int(size, 16), "flags": flags, "sha256": digest}
    if ".text" not in result:
        raise ValueError("Expected an ELF binary with a text section")
    return result


def exports(binary):
    output = run(["nm", "--dynamic", "--defined-only", "--format=posix", str(binary)])
    return sorted(line.split()[0] for line in output.splitlines() if line.strip())


def build_id(binary):
    output = run(["readelf", "--notes", str(binary)])
    identities = re.findall(r"Build ID: ([0-9a-f]+)", output)
    if len(identities) != 1:
        raise ValueError("Expected exactly one ELF build ID")
    return identities[0]


def audit(binary, expected_exports):
    actual = exports(binary)
    if actual != sorted(expected_exports) or len(expected_exports) != len(set(expected_exports)):
        raise ValueError("Dynamic exports do not match the exact public API allowlist")
    return {"build_id": build_id(binary), "exports": actual, "sections": sections(binary)}


def split(binary, runtime, debug, expected_exports):
    binary = Path(binary)
    runtime = Path(runtime)
    debug = Path(debug)
    before = audit(binary, expected_exports)
    if not any(name.startswith((".debug_info", ".zdebug_info")) for name in before["sections"]):
        raise ValueError("Build with debug information first; no private diagnostics to retain")
    if runtime.exists() or debug.exists() or runtime == debug:
        raise ValueError("Runtime and private debug outputs must be distinct new files")
    runtime.parent.mkdir(parents=True, exist_ok=True)
    debug.parent.mkdir(parents=True, exist_ok=True)
    run(["objcopy", "--only-keep-debug", str(binary), str(debug)])
    if build_id(debug) != before["build_id"]:
        raise ValueError("Private debug build ID mismatch")
    run(["objcopy", "--strip-debug", "--strip-unneeded", str(binary), str(runtime)])
    run(["objcopy", "--add-gnu-debuglink=" + str(debug), str(runtime)])
    after = audit(runtime, expected_exports)
    if after["build_id"] != before["build_id"]:
        raise ValueError("Runtime build ID changed")
    allocated_before = {name: value for name, value in before["sections"].items() if "A" in value["flags"]}
    allocated_after = {name: value for name, value in after["sections"].items() if "A" in value["flags"]}
    if allocated_before != allocated_after:
        raise ValueError("Stripping changed allocated ELF sections")
    if any(name.startswith((".debug", ".zdebug")) or name == ".symtab" for name in after["sections"]):
        raise ValueError("Private symbol/debug sections remain in runtime")
    return {"build_id": before["build_id"], "exports": before["exports"], "allocated_sections_unchanged": True}
