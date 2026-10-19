#!/usr/bin/env python3
"""Check the installed modern Linux Qt closure without build-host search paths."""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys


def check(runtime):
    runtime = runtime.resolve()
    manifest = json.loads((runtime / "runtime.json").read_text())
    if manifest["qt"] != "6.10.3" or manifest["platform"] != "linux-amd64":
        raise RuntimeError("Unexpected deployed Qt runtime identity")
    environment = dict(os.environ)
    for name in ("LD_LIBRARY_PATH", "QT_PLUGIN_PATH", "QT_QPA_PLATFORM_PLUGIN_PATH",
                 "QTWEBENGINEPROCESS_PATH", "QTWEBENGINE_RESOURCES_PATH", "QTWEBENGINE_LOCALES_PATH"):
        environment.pop(name, None)
    for relative, digest in manifest["files"].items():
        binary = runtime / relative
        if not binary.resolve().is_relative_to(runtime):
            raise RuntimeError(f"Runtime manifest escapes its install root: {relative}")
        if hashlib.sha256(binary.read_bytes()).hexdigest() != digest:
            raise RuntimeError(f"Runtime file digest mismatch: {relative}")
        output = subprocess.check_output(["ldd", str(binary)], env=environment, text=True, timeout=30)
        if "not found" in output:
            raise RuntimeError(f"Incomplete deployed dependencies: {relative}\n{output}")
        for library, path in re.findall(r"(libQt6\S+) => (/\S+)", output):
            if not Path(path).resolve().is_relative_to(runtime):
                raise RuntimeError(f"{library} escaped the bundled runtime: {path}")
        sections = subprocess.check_output(["readelf", "-S", "--wide", str(binary)], text=True, timeout=30)
        if re.search(r"\.(?:z?debug)[_.]", sections):
            raise RuntimeError(f"Debug sections shipped in {relative}")
    for relative in ("libexec/QtWebEngineProcess", "resources/qtwebengine_resources.pak",
                     "translations/qtwebengine_locales/en-US.pak", "plugins/platforms/libqxcb.so"):
        if not (runtime / relative).is_file():
            raise RuntimeError(f"Missing deployed runtime file: {relative}")
    print(f"PASS: Qt 6.10.3 runtime closure ({len(manifest['files'])} ELF files), no public debug sections")


if __name__ == "__main__":
    check(Path(sys.argv[1]))
