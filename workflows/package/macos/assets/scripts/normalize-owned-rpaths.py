#!/usr/bin/env python3
"""Normalize only Dockpipe-owned Mach-O rpaths before protection and signing."""
import argparse
from collections import Counter
import json
import os
from pathlib import Path
import re
import selectors
import signal
import stat
import subprocess
import tempfile
import time


HOST_LIBRARY = re.compile(r"libdockpipe-launcher-ui(?:\.[0-9]+)*\.dylib\Z")
MAX_COMMANDS = 4096
MAX_TOOL_OUTPUT = 8 * 1024 * 1024
MAX_RPATH_BYTES = 4096
MAX_ARGUMENT_BYTES = 64 * 1024
MAX_DIRECTORY_ENTRIES = 4096
EXECUTABLE_RPATH = "@executable_path/../Frameworks"
LIBRARY_RPATH = "@loader_path"


def tool_output(arguments, deadline):
    """Bound both output streams and reap the owned tool on every failure."""
    if sum(len(os.fsencode(value)) + 1 for value in arguments) > MAX_ARGUMENT_BYTES:
        raise ValueError("Owned rpath tool argument budget exceeded")
    timeout = min(30, deadline - time.monotonic())
    if timeout <= 0:
        raise ValueError("Owned rpath normalization deadline exceeded")
    process = subprocess.Popen(arguments, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                               start_new_session=True,
                               env={"PATH": "/usr/bin:/bin:/usr/sbin:/sbin", "LANG": "C", "LC_ALL": "C"})
    output = bytearray()
    diagnostic_bytes = 0
    selector = selectors.DefaultSelector()
    selector.register(process.stdout, selectors.EVENT_READ, "stdout")
    selector.register(process.stderr, selectors.EVENT_READ, "stderr")
    until = time.monotonic() + timeout
    try:
        while selector.get_map():
            remaining = until - time.monotonic()
            if remaining <= 0:
                raise ValueError("Owned rpath tool deadline exceeded")
            for key, _ in selector.select(min(remaining, 0.25)):
                data = os.read(key.fd, 65536)
                if not data:
                    selector.unregister(key.fileobj)
                elif key.data == "stdout":
                    if len(output) + len(data) > MAX_TOOL_OUTPUT:
                        raise ValueError("Owned rpath tool output limit exceeded")
                    output.extend(data)
                else:
                    diagnostic_bytes += len(data)
                    if diagnostic_bytes > 4096:
                        raise ValueError("Owned rpath tool diagnostic limit exceeded")
        remaining = until - time.monotonic()
        if remaining <= 0 or process.wait(timeout=remaining) != 0:
            raise ValueError("Owned rpath tool failed")
        return output.decode("utf-8", errors="strict")
    except subprocess.TimeoutExpired:
        raise ValueError("Owned rpath tool deadline exceeded") from None
    finally:
        selector.close()
        if process.returncode is None:
            # No poll/wait has reaped the leader before this group signal.
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.wait(timeout=3)
        process.stdout.close()
        process.stderr.close()


def parse_rpaths(text):
    """Parse otool's actual load-command records, not a grep of path-like text."""
    if len(text.encode("utf-8")) > MAX_TOOL_OUTPUT or "\0" in text:
        raise ValueError("Invalid owned load-command output bounds")
    lines = text.splitlines()
    if not lines or not lines[0].endswith(":"):
        raise ValueError("Owned load-command image header is missing")
    commands = []
    current = None
    for line in lines[1:]:
        if not line.strip():
            continue
        match = re.fullmatch(r"Load command ([0-9]+)", line.strip())
        if match:
            if int(match.group(1)) != len(commands) or len(commands) >= MAX_COMMANDS:
                raise ValueError("Owned load-command sequence is invalid or unsupported")
            current = []
            commands.append(current)
        elif current is None:
            raise ValueError("Malformed owned load-command record")
        else:
            current.append(line.lstrip())
    if not commands:
        raise ValueError("Owned load commands are missing")
    paths = []
    for command in commands:
        if len(command) < 2 or not re.fullmatch(r"cmd (?:LC_[A-Z0-9_]+|0x[0-9a-fA-F]+)", command[0]):
            raise ValueError("Malformed owned load-command type")
        size = re.fullmatch(r"cmdsize ([0-9]+)", command[1])
        if size is None or int(size.group(1)) < 8 or int(size.group(1)) % 8 != 0:
            raise ValueError("Malformed owned load-command size")
        if command[0] != "cmd LC_RPATH":
            continue
        if len(command) != 3:
            raise ValueError("Malformed owned rpath command fields")
        match = re.fullmatch(r"path (.+) \(offset ([0-9]+)\)", command[2])
        if match is None:
            raise ValueError("Malformed owned rpath value")
        path, offset = match.group(1), int(match.group(2))
        length = len(path.encode("utf-8"))
        if (length > MAX_RPATH_BYTES or any(ord(value) < 32 or ord(value) == 127 for value in path)
                or offset < 12 or offset + length + 1 > int(size.group(1))):
            raise ValueError("Owned rpath offset or value exceeds its command")
        paths.append(path)
    return paths


def contained_image(app, candidate, directory, expected_name=None):
    try:
        image = candidate.resolve(strict=True)
        metadata = image.stat()
    except (OSError, RuntimeError):
        raise ValueError("Owned bundle image link resolution failed") from None
    if (not image.is_relative_to(app) or image.parent != directory or not stat.S_ISREG(metadata.st_mode)
            or metadata.st_nlink != 1 or (expected_name is not None and image.name != expected_name)):
        raise ValueError("Owned bundle image escapes its role or is externally linked")
    return image


def owned_images(app):
    if app.is_symlink():
        raise ValueError("Application root must not be a symbolic link")
    app = app.resolve(strict=True)
    if not app.is_dir():
        raise ValueError("Application root is not a directory")
    frameworks = (app / "Contents/Frameworks").resolve(strict=True)
    executables = (app / "Contents/MacOS").resolve(strict=True)
    if (not frameworks.is_relative_to(app) or not executables.is_relative_to(app)
            or (executables / "../Frameworks").resolve(strict=True) != frameworks):
        raise ValueError("Owned bundle directories escape the application")
    main = contained_image(app, app / "Contents/MacOS/DockPipe", executables, "DockPipe")
    libraries = set()
    for index, candidate in enumerate(frameworks.iterdir()):
        if index >= MAX_DIRECTORY_ENTRIES:
            raise ValueError("Owned Frameworks discovery limit exceeded")
        if not candidate.name.startswith("libdockpipe-launcher-ui"):
            continue
        if not HOST_LIBRARY.fullmatch(candidate.name):
            raise ValueError("Unexpected owned host library name")
        image = contained_image(app, candidate, frameworks)
        if not HOST_LIBRARY.fullmatch(image.name):
            raise ValueError("Owned host library aliases a foreign image")
        libraries.add(image)
    if len(libraries) != 1:
        raise ValueError("Expected one canonical owned host UI image")
    reader = contained_image(app, app / "Contents/MacOS/dockpipe-workspace-reader", executables, "dockpipe-workspace-reader")
    terminal = contained_image(app, app / "Contents/MacOS/dockpipe-terminal-host", executables, "dockpipe-terminal-host")
    return app, [(main, EXECUTABLE_RPATH), (next(iter(libraries)), LIBRARY_RPATH),
                 (reader, EXECUTABLE_RPATH), (terminal, EXECUTABLE_RPATH)]


def deletion_arguments(paths):
    """One delete per distinct path; Apple rejects duplicate delete arguments."""
    if len(paths) > MAX_COMMANDS:
        raise ValueError("Owned duplicate rpath count exceeds the load-command limit")
    arguments = []
    for path in dict.fromkeys(paths):
        arguments.extend(["-delete_rpath", path])
    if sum(len(value.encode("utf-8")) + 1 for value in arguments) > MAX_ARGUMENT_BYTES // 2:
        raise ValueError("Owned rpath mutation argument budget exceeded")
    return arguments


def normalize(app, receipt, run=tool_output):
    app, images = owned_images(app)
    receipt = receipt.absolute()
    if receipt.is_symlink() or not receipt.parent.resolve(strict=True).is_relative_to(app):
        raise ValueError("Owned rpath receipt escapes the application")
    deadline = time.monotonic() + 120
    inspections = [(image, desired, parse_rpaths(run(["/usr/bin/otool", "-l", str(image)], deadline)))
                   for image, desired in images]
    # Validate every mutation's bounded argv before modifying the first image.
    for _, _, paths in inspections:
        deletion_arguments(paths)
    records = []
    for image, desired, original in inspections:
        paths = original
        calls = 0
        if paths != [desired]:
            for _ in range(len(original)):
                if not paths:
                    break
                arguments = deletion_arguments(paths)
                run(["/usr/bin/install_name_tool", *arguments, str(image)], deadline)
                calls += 1
                after = parse_rpaths(run(["/usr/bin/otool", "-l", str(image)], deadline))
                before_counts, after_counts = Counter(paths), Counter(after)
                if len(after) >= len(paths) or any(count > before_counts[path] for path, count in after_counts.items()):
                    raise ValueError("Owned rpath deletion did not make exact bounded progress")
                paths = after
            if paths:
                raise ValueError("Owned duplicate rpath deletion budget exhausted")
            run(["/usr/bin/install_name_tool", "-add_rpath", desired, str(image)], deadline)
            calls += 1
            paths = parse_rpaths(run(["/usr/bin/otool", "-l", str(image)], deadline))
        if paths != [desired]:
            raise ValueError("Owned final bundle-relative rpath verification failed")
        records.append({"path": image.relative_to(app).as_posix(), "previous_count": len(original),
                        "removed_count": len(original) if calls else 0, "mutation_calls": calls, "rpaths": paths})
    value = {"schema_version": 1, "owned_images": records}
    with tempfile.NamedTemporaryFile(mode="w", dir=receipt.parent, prefix=".owned-rpaths-", delete=False) as output:
        temporary = Path(output.name)
        try:
            json.dump(value, output, indent=2)
            output.write("\n")
            output.flush()
            os.fsync(output.fileno())
            os.replace(temporary, receipt)
        finally:
            temporary.unlink(missing_ok=True)
    return value


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("app", type=Path)
    parser.add_argument("receipt", type=Path)
    arguments = parser.parse_args()
    try:
        normalize(arguments.app, arguments.receipt)
    except (ValueError, OSError, UnicodeError, RuntimeError, subprocess.SubprocessError):
        # Tool errors can contain private build paths. The failed unsealed
        # candidate remains available for local inspection; do not leak them.
        raise SystemExit("Owned bundle rpath normalization failed before signing.") from None


if __name__ == "__main__":
    main()
