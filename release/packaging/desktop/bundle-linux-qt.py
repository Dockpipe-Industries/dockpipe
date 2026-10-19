#!/usr/bin/env python3
"""Deploy the pinned native amd64 Qt runtime from a trusted Release build."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess


QT_VERSION = "6.10.3"
PLUGIN_GROUPS = (
    "imageformats", "iconengines", "networkinformation", "tls", "platforminputcontexts",
    "platformthemes", "printsupport", "xcbglintegrations", "wayland-decoration-client",
    "wayland-graphics-integration-client", "wayland-shell-integration",
)
PLATFORMS = ("libqxcb.so", "libqwayland.so", "libqoffscreen.so", "libqminimal.so")
# Ubuntu 24.04 removed this ABI, while the pinned Qt TIFF plugin still needs it.
# Other dependencies remain distribution-owned and are resolved by shlibdeps.
COMPATIBILITY_LIBRARIES = {"libtiff.so.5": "libtiff5"}


def command(*arguments):
    return subprocess.check_output(arguments, text=True, timeout=120).strip()


def dependencies(binary):
    output = command("ldd", str(binary))
    if "not found" in output:
        raise RuntimeError(f"Unresolved runtime dependency in {binary}:\n{output}")
    result = {}
    for line in output.splitlines():
        match = re.match(r"\s*(\S+) => (/\S+) ", line)
        if match:
            result[match[1]] = Path(match[2]).resolve()
    return result


def soname(binary):
    match = re.search(r"\(SONAME\).*\[(.+)\]", command("readelf", "-d", str(binary)))
    return match[1] if match else None


def shlibs_entry(name, version):
    match = re.fullmatch(r"([A-Za-z0-9_+.-]+)\.so\.([A-Za-z0-9_.-]+)", name)
    if not match:
        raise RuntimeError(f"Unsupported bundled SONAME: {name}")
    return f"{match[1]} {match[2]} dockpipe-desktop (= {version})"


def check_baseline(binary):
    output = command("readelf", "--version-info", str(binary))
    versions = {tuple(map(int, item.split("."))) for item in re.findall(r"GLIBC_([0-9.]+)", output)}
    if versions and max(versions) > (2, 35):
        raise RuntimeError(f"{binary.name} requires glibc newer than Ubuntu 22.04")


def deploy(launcher, qt, stage, private_debug, version):
    qt = qt.resolve()
    stage = stage.resolve()
    private_debug = private_debug.resolve()
    if private_debug.is_relative_to(stage):
        raise RuntimeError("Private debug output must be outside the public package tree")
    if command(str(qt / "bin/qmake"), "-query", "QT_VERSION") != QT_VERSION:
        raise RuntimeError(f"The native SDK runtime requires Qt {QT_VERSION}")
    linked = dependencies(launcher)
    core = linked.get("libQt6Core.so.6")
    if not core or not core.is_relative_to(qt) or core.name != f"libQt6Core.so.{QT_VERSION}":
        raise RuntimeError("The launcher was not built against the selected Qt SDK")

    runtime = stage / "usr/lib/dockpipe"
    notices = stage / "usr/share/doc/dockpipe-desktop/qt"
    runtime.mkdir(parents=True, exist_ok=True)
    notices.mkdir(parents=True, exist_ok=True)
    private_debug.mkdir(parents=True, exist_ok=True)
    binaries = []
    libraries = {}

    def copy_binary(source, target, owned=False):
        source = source.resolve()
        check_baseline(source)
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source, target)
        binaries.append((target, owned))
        for name, path in dependencies(source).items():
            if name.startswith("libQt6") and not path.is_relative_to(qt):
                raise RuntimeError(f"Mixed Qt runtime: {name} resolves outside the selected SDK")
            if (path.is_relative_to(qt) or name == "libdockpipe-launcher-ui.so.1"
                    or name in COMPATIBILITY_LIBRARIES):
                if name in libraries and libraries[name] != path:
                    raise RuntimeError(f"Conflicting runtime dependency: {name}")
                libraries[name] = path

    copy_binary(launcher, runtime / "bin/dockpipe-launcher", owned=True)
    copy_binary(qt / "libexec/QtWebEngineProcess", runtime / "libexec/QtWebEngineProcess")
    for name in PLATFORMS:
        copy_binary(qt / "plugins/platforms" / name, runtime / "plugins/platforms" / name)
    for group in PLUGIN_GROUPS:
        for plugin in sorted((qt / "plugins" / group).glob("*.so")):
            copy_binary(plugin, runtime / "plugins" / group / plugin.name)
    copied = set()
    while set(libraries) - copied:
        for name in sorted(set(libraries) - copied):
            copy_binary(libraries[name], runtime / "lib" / name,
                        owned=name == "libdockpipe-launcher-ui.so.1")
            copied.add(name)

    for name, package in COMPATIBILITY_LIBRARIES.items():
        if name not in copied:
            continue
        package_notices = notices / package
        package_notices.mkdir()
        shutil.copy2(Path("/usr/share/doc") / package / "copyright", package_notices / "copyright")
        identity = command("dpkg-query", "-W", "-f=${source:Package} ${source:Version}", package)
        (package_notices / "README").write_text(
            f"Ubuntu 22.04 compatibility runtime: {name}\nSource package: {identity}\n"
            "Source and packaging: https://launchpad.net/ubuntu/+source/tiff\n"
            "Bundled to preserve the Qt TIFF image plugin on Ubuntu 22.04 and 24.04.\n")

    # Keep only runtime data. SDK headers, CMake modules, examples and sources
    # never enter the public payload; upstream SPDX notices retain provenance.
    resources = qt / "resources"
    for name in ("qtwebengine_resources.pak", "qtwebengine_resources_100p.pak",
                 "qtwebengine_resources_200p.pak", "icudtl.dat", "v8_context_snapshot.bin"):
        if not (resources / name).is_file():
            raise RuntimeError(f"Missing WebEngine runtime resource: {name}")
    shutil.copytree(resources, runtime / "resources")
    locales = qt / "translations/qtwebengine_locales"
    if not (locales / "en-US.pak").is_file():
        raise RuntimeError("Missing WebEngine English locale")
    shutil.copytree(locales, runtime / "translations/qtwebengine_locales")
    for translation in sorted((qt / "translations").glob("qtbase_*.qm")):
        shutil.copy2(translation, runtime / "translations" / translation.name)
    sboms = sorted((qt / "sbom").glob("*.spdx.json"))
    if not sboms:
        raise RuntimeError("Qt SDK SPDX license inventory is required")
    for inventory in sboms:
        shutil.copy2(inventory, notices / inventory.name)
    for name in ("Qt-LGPL-3.txt", "Qt-GPL-3.txt"):
        shutil.copy2(Path(__file__).parent / "licenses" / name, notices / name)
    (notices / "README").write_text(
        f"Dockpipe dynamically links unmodified Qt {QT_VERSION}, including Qt WebEngine.\n"
        "Copyright The Qt Company Ltd. and other contributors. Qt is used under LGPLv3.\n"
        "The accompanying SPDX inventories identify third-party components and licenses.\n"
        "Chromium license notices are also available at chrome://credits in the browser.\n"
        f"Corresponding upstream source: https://download.qt.io/archive/qt/6.10/{QT_VERSION}/submodules/\n"
        "Compatible replacement libraries and debugging modifications are permitted.\n"
        "Dockpipe source: https://github.com/Dockpipe-Industries/dockpipe\n")
    configuration = "[Paths]\nPrefix=..\nLibraries=lib\nPlugins=plugins\nLibraryExecutables=libexec\nData=.\nTranslations=translations\n"
    for directory in ("bin", "libexec"):
        (runtime / directory / "qt.conf").write_text(configuration)
    (runtime / "bin/dockpipe-syntax.rcc").symlink_to("../../../share/dockpipe/launcher/dockpipe-syntax.rcc")
    # Preserve the launcher's adjacent CLI lookup after relocating its executable.
    (runtime / "bin/dockpipe").symlink_to("../../../bin/dockpipe")
    (stage / "usr/bin").mkdir(parents=True, exist_ok=True)
    (stage / "usr/bin/dockpipe-launcher").symlink_to("../lib/dockpipe/bin/dockpipe-launcher")

    shlibs = []
    for binary, owned in binaries:
        if owned:
            # Preserve our only diagnostic copy before removing public symbols.
            debug_file = private_debug / (binary.name + ".debug")
            command("objcopy", "--only-keep-debug", str(binary), str(debug_file))
        command("strip", "--strip-unneeded", str(binary))
        relative = os.path.relpath(runtime / "lib", binary.parent)
        command("patchelf", "--force-rpath", "--set-rpath", f"$ORIGIN/{relative}", str(binary))
        if owned:
            command("objcopy", f"--add-gnu-debuglink={debug_file}", str(binary))
        if re.search(r"\.(?:z?debug)[_.]", command("readelf", "-S", "--wide", str(binary))):
            raise RuntimeError(f"Debug sections remain in public binary: {binary.name}")
        name = soname(binary)
        if name:
            shlibs.append(shlibs_entry(name, version))
    (stage / "DEBIAN/shlibs").write_text("\n".join(sorted(set(shlibs))) + "\n")
    manifest = {
        "schema": 1, "qt": QT_VERSION, "platform": "linux-amd64", "glibc_max": "2.35",
        "files": {str(path.relative_to(runtime)): hashlib.sha256(path.read_bytes()).hexdigest()
                  for path, _ in binaries},
    }
    (runtime / "runtime.json").write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n")
    return [path for path, _ in binaries]


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--launcher", type=Path, required=True)
    parser.add_argument("--qt", type=Path, required=True)
    parser.add_argument("--stage", type=Path, required=True)
    parser.add_argument("--private-debug", type=Path, required=True)
    parser.add_argument("--version", required=True)
    arguments = parser.parse_args()
    for binary in deploy(arguments.launcher.resolve(), arguments.qt, arguments.stage,
                         arguments.private_debug, arguments.version):
        print(binary)
