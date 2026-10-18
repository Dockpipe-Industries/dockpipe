#!/usr/bin/env python3
"""Bundle optional native runtime dependencies from a trusted local Qt build."""
import argparse
import json
import os
from pathlib import Path
import re
import shutil
import subprocess


def command(*args):
    return subprocess.check_output(args, text=True).strip()


def dependencies(binary):
    output = command("ldd", str(binary))
    if "not found" in output:
        raise RuntimeError(f"Unresolved dependency in {binary}:\n{output}")
    libraries = {}
    for line in output.splitlines():
        match = re.match(r"\s*(\S+) => (/\S+) ", line)
        if match:
            libraries[match[1]] = Path(match[2])
    return libraries


def bundle(build, destination):
    source = build / "cockpit-package/assets/ui"
    destination.mkdir(parents=True, exist_ok=True)
    ui = destination / "ui"
    shutil.copytree(source, ui)
    library = ui / "libdockpipe-cockpit.so"
    qt = os.environ.get("DOCKPIPE_QMAKE")
    if not qt:
        cache = (build / "CMakeCache.txt").read_text()
        match = re.search(r"^Qt6Core_DIR:PATH=(.+)$", cache, re.MULTILINE)
        if match:
            sdk = Path(match[1]).parents[2]
            for name in ("qmake6", "qmake"):
                candidate = sdk / "bin" / name
                if candidate.is_file():
                    qt = str(candidate)
                    break
    if not qt:
        qt = shutil.which("qmake6")
    if not qt:
        raise RuntimeError("qmake6 is required to locate the matching WebEngine runtime")
    linked = dependencies(source / library.name)
    core = linked.get("libQt6Core.so.6")
    version = command(qt, "-query", "QT_VERSION")
    if not core or not core.resolve().name.endswith(".so." + version):
        raise RuntimeError("qmake and the compiled Cockpit Qt runtime do not match; set DOCKPIPE_QMAKE to the matching SDK tool")
    helper = Path(os.environ.get("QTWEBENGINEPROCESS_PATH",
        str(Path(command(qt, "-query", "QT_INSTALL_LIBEXECS")) / "QtWebEngineProcess")))
    if not helper.is_file():
        raise RuntimeError("Install the matching Qt WebEngine runtime helper, or set QTWEBENGINEPROCESS_PATH explicitly")
    shutil.copy2(helper, ui / "QtWebEngineProcess.bin")
    # Keep the child runtime local without changing the launcher's library search path.
    wrapper = ui / "QtWebEngineProcess"
    wrapper.write_text('''#!/bin/sh
set -eu
runtime_dir="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
export LD_LIBRARY_PATH="$runtime_dir/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
exec "$runtime_dir/ui/QtWebEngineProcess.bin" "$@"
''')
    wrapper.chmod(0o755)
    baseline = dependencies(build / "libdockpipe-launcher-ui.so")
    baseline["libdockpipe-launcher-ui.so.1"] = build / "libdockpipe-launcher-ui.so.1"
    runtime = linked
    runtime.update(dependencies(helper))
    libraries = destination / "lib"
    libraries.mkdir()
    notices = destination / "licenses"
    notices.mkdir()
    bundled = {}
    for name, path in sorted(runtime.items()):
        if name in baseline:
            continue
        shutil.copy2(path.resolve(), libraries / name)
        bundled[name] = str(path.resolve())
        # Debian runtime copyright files include the upstream license/notice inventory.
        owner = subprocess.run(["dpkg-query", "-S", str(path.resolve())], capture_output=True, text=True)
        if owner.returncode == 0:
            package = owner.stdout.split(": ", 1)[0].split(":", 1)[0]
            copyright_file = Path("/usr/share/doc") / package / "copyright"
            if copyright_file.is_file():
                shutil.copy2(copyright_file, notices / f"{package}.copyright")
    helper_notice = helper.parents[3] / "share/doc/libqt6webenginecore6-bin/copyright"
    if helper_notice.is_file():
        shutil.copy2(helper_notice, notices / "libqt6webenginecore6-bin.copyright")
    data = Path(command(qt, "-query", "QT_INSTALL_DATA"))
    translations = Path(command(qt, "-query", "QT_INSTALL_TRANSLATIONS"))
    shutil.copytree(data / "resources", destination / "resources")
    shutil.copytree(translations / "qtwebengine_locales", destination / "locales")
    (destination / "runtime.json").write_text(json.dumps({
        "schema": 1, "interface": "com.dockpipe.launcher.WorkspacePackage/1.0",
        "qt": command(qt, "-query", "QT_VERSION"), "libraries": sorted(bundled),
    }, indent=2) + "\n")
    print(f"Bundled {len(bundled)} optional libraries in {destination}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("build", type=Path)
    parser.add_argument("destination", type=Path)
    arguments = parser.parse_args()
    bundle(arguments.build.resolve(), arguments.destination.resolve())
