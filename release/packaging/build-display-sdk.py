#!/usr/bin/env python3
"""Qualify and archive the native display interface SDK without desktop dependencies."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import platform
import re
import shutil
import struct
import subprocess


LAUNCHER = Path("src/app/tooling/dockpipe-launcher")
VERSION = "1.0.0"
QT_VERSION = "6.10.3"
MAX_FILES = 64
MAX_BYTES = 2 << 20
INSTALLED_PATHS = {
    "include/dockpipe-launcher/LauncherDisplayPackage.h",
    "include/dockpipe-launcher/LauncherNativeLibraryScope.h",
    "lib/cmake/DockPipeDisplaySDK/DockPipeDisplaySDKConfig.cmake",
    "lib/cmake/DockPipeDisplaySDK/DockPipeDisplaySDKConfigVersion.cmake",
    "lib/cmake/DockPipeDisplaySDK/DockPipeDisplaySDKTargets.cmake",
    "share/DockPipeDisplaySDK/LICENSE",
    "share/DockPipeDisplaySDK/display-sdk-candidate.json",
}
HELPER_PATH = Path(__file__).with_name("build-launcher-sdk.py")
SPEC = importlib.util.spec_from_file_location("launcher_sdk_archive", HELPER_PATH)
ARCHIVE_HELPER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(ARCHIVE_HELPER)


def native_platform(system=None, machine=None):
    system = system or platform.system()
    machine = machine or platform.machine()
    operating_system = {"Linux": "linux", "Darwin": "darwin", "Windows": "windows"}.get(system)
    architecture = {"x86_64": "amd64", "AMD64": "amd64", "aarch64": "arm64", "arm64": "arm64"}.get(machine)
    if not operating_system or not architecture:
        raise ValueError("Unsupported native display SDK platform")
    return operating_system + "-" + architecture


def source_identity(source):
    leaf = source / LAUNCHER / "display-sdk"
    files = [source / "LICENSE", source / LAUNCHER / "src/LauncherDisplayPackage.h",
             source / LAUNCHER / "src/LauncherNativeLibraryScope.h",
             source / "release/packaging/build-display-sdk.py", source / "release/packaging/build-launcher-sdk.py"]
    for directory, directories, filenames in os.walk(leaf, followlinks=False):
        directories[:] = [name for name in directories if name not in {"build", "__pycache__", "CMakeFiles"}]
        if any((Path(directory) / name).is_symlink() for name in directories):
            raise ValueError("Linked SDK source directory rejected")
        files.extend(Path(directory) / name for name in filenames if not name.endswith(".pyc"))
    if leaf.is_symlink() or not leaf.is_dir() or any(path.is_symlink() or not path.is_file() for path in files):
        raise ValueError("Display SDK source requires regular authored files")
    rows = [{"path": path.relative_to(source).as_posix(), "sha256": ARCHIVE_HELPER.digest(path)}
            for path in sorted(files)]
    return hashlib.sha256(json.dumps(rows, sort_keys=True, separators=(",", ":")).encode()).hexdigest()


def installed_inventory(prefix):
    entries = []
    total_bytes = 0
    for directory, directories, filenames in os.walk(prefix, followlinks=False):
        current = Path(directory)
        if any((current / name).is_symlink() for name in directories):
            raise ValueError("Linked SDK payload directory rejected")
        for name in filenames:
            path = current / name
            if path.is_symlink() or not path.is_file():
                raise ValueError("Display SDK requires regular installed files")
            size = path.stat().st_size
            total_bytes += size
            if len(entries) >= MAX_FILES or total_bytes > MAX_BYTES:
                raise ValueError("Display SDK exceeds its 64-file/2-MiB distribution contract")
            entries.append({"path": path.relative_to(prefix).as_posix(), "bytes": size})
    if {entry["path"] for entry in entries} != INSTALLED_PATHS:
        raise ValueError("Display SDK contains missing or unexpected installed files")
    for entry in entries:
        entry["sha256"] = ARCHIVE_HELPER.digest(prefix / entry["path"])
    return sorted(entries, key=lambda entry: entry["path"])


def consumer_platform(path):
    """Read the executable machine type rather than trusting the build host label."""
    with path.open("rb") as stream:
        header = stream.read(64)
        if len(header) != 64:
            raise ValueError("Consumer executable header is truncated")
        if header[:5] == b"\x7fELF\x02" and header[5] in {1, 2}:
            byte_order = "<" if header[5] == 1 else ">"
            machine = struct.unpack_from(byte_order + "H", header, 18)[0]
            architecture = {62: "amd64", 183: "arm64"}.get(machine)
            operating_system = "linux"
        elif header[:4] == b"\xcf\xfa\xed\xfe":
            machine = struct.unpack_from("<I", header, 4)[0]
            architecture = {0x01000007: "amd64", 0x0100000C: "arm64"}.get(machine)
            operating_system = "darwin"
        elif header[:2] == b"MZ" and len(header) == 64:
            offset = struct.unpack_from("<I", header, 60)[0]
            if not 64 <= offset <= 1 << 20:
                raise ValueError("Consumer PE header offset exceeds capacity")
            stream.seek(offset)
            signature = stream.read(6)
            if len(signature) != 6 or signature[:4] != b"PE\x00\x00":
                raise ValueError("Consumer PE signature is invalid")
            architecture = {0x8664: "amd64", 0xAA64: "arm64"}.get(struct.unpack_from("<H", signature, 4)[0])
            operating_system = "windows"
        else:
            raise ValueError("Consumer executable format is unsupported")
    if not architecture:
        raise ValueError("Consumer executable architecture is unsupported")
    return operating_system + "-" + architecture


def build(source, source_sha, qt, work, output, jobs, cmake, ctest, generator):
    source = source.resolve(strict=True)
    qt = qt.resolve(strict=True)
    work = work.resolve()
    output = output.resolve()
    target = native_platform()
    qt_config = qt / "lib/cmake/Qt6/Qt6Config.cmake"
    if not qt_config.is_file():
        raise ValueError("Selected Qt root does not contain Qt6Config.cmake")
    if source_sha is not None and not re.fullmatch(r"[0-9a-f]{40}", source_sha):
        raise ValueError("Frozen source identity requires an exact commit SHA")
    if (work.exists() or output.exists() or work.is_relative_to(source) or output.is_relative_to(source)
            or work.is_relative_to(output) or output.is_relative_to(work)):
        raise ValueError("Use separate fresh work/output directories outside the source tree")
    identity = source_identity(source)
    environment = dict(os.environ)
    environment["PATH"] = str(qt / "bin") + os.pathsep + environment.get("PATH", "")

    def run(arguments, timeout):
        subprocess.run([str(argument) for argument in arguments], env=environment, check=True, timeout=timeout)

    work.mkdir(parents=True)
    output.mkdir(parents=True)
    archive_root = "DockPipeDisplaySDK-" + VERSION + "-" + target
    prefix = work / archive_root
    native_build = work / "sdk"
    configure = [cmake, "-S", source / LAUNCHER / "display-sdk", "-B", native_build,
                 "-DCMAKE_BUILD_TYPE=Release", f"-DCMAKE_PREFIX_PATH={qt}",
                 f"-DQt6_DIR={qt_config.parent}", f"-DCMAKE_INSTALL_PREFIX={prefix}"]
    if generator:
        configure.extend(["-G", generator])
    run(configure, 300)
    run([cmake, "--install", native_build, "--config", "Release", "--component", "DisplaySDK"], 120)
    candidate = json.loads((prefix / "share/DockPipeDisplaySDK/display-sdk-candidate.json").read_text())
    if candidate.get("qt_version") != QT_VERSION or candidate.get("interface") != "com.dockpipe.launcher.DisplayPackage/1.0":
        raise ValueError("Installed display SDK does not match the public Qt/interface contract")
    if (not candidate.get("system") or not candidate.get("processor")
            or native_platform(candidate["system"], candidate["processor"]) != target):
        raise ValueError("Installed display SDK platform differs from the native host")
    shutil.copyfile(source / "LICENSE", prefix / "share/DockPipeDisplaySDK/LICENSE")
    # Moving the prefix before consumer compilation verifies relocated CMake imports.
    relocated = work / "relocated" / archive_root
    relocated.parent.mkdir()
    prefix.rename(relocated)
    consumer = work / "consumer"
    configure = [cmake, "-S", source / LAUNCHER / "display-sdk/tests/consumer", "-B", consumer,
                 "-DCMAKE_BUILD_TYPE=Release", f"-DCMAKE_PREFIX_PATH={relocated};{qt}",
                 f"-DQt6_DIR={qt_config.parent}"]
    if generator:
        configure.extend(["-G", generator])
    run(configure, 300)
    run([cmake, "--build", consumer, "--config", "Release", "--parallel", jobs], 600)
    executable = "display-sdk-consumer.exe" if target.startswith("windows-") else "display-sdk-consumer"
    candidates = [path for path in (consumer / executable, consumer / "Release" / executable) if path.is_file()]
    if len(candidates) != 1 or consumer_platform(candidates[0]) != target:
        raise ValueError("Relocated consumer executable does not match the native SDK platform")
    run([ctest, "--test-dir", consumer, "-C", "Release", "--output-on-failure", "--no-tests=error"], 120)
    if source_identity(source) != identity:
        raise ValueError("Display SDK source changed during qualification")
    entries = installed_inventory(relocated)
    manifest = {"schema": 1, "artifact_class": "sdk", "files": entries}
    manifest_path = output / "sdk-files.json"
    manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n")
    archive = output / (archive_root + ".tar.gz")
    ARCHIVE_HELPER.write_archive(relocated, archive, entries, archive_root)
    metadata = {
        "schema": 1, "product": "DockPipeDisplaySDK", "version": VERSION, "platform": target,
        "qt_version": QT_VERSION, "interface": candidate["interface"],
        "source_repository": "Dockpipe-Industries/dockpipe", "source_sha": source_sha,
        "source_state": "committed-snapshot" if source_sha else "development", "source_digest": identity,
        "compiler": {"id": candidate["compiler_id"], "version": candidate["compiler_version"]},
        "archive": {"name": archive.name, "size_bytes": archive.stat().st_size, "sha256": ARCHIVE_HELPER.digest(archive)},
        "file_manifest": {"name": manifest_path.name, "sha256": ARCHIVE_HELPER.digest(manifest_path)},
        "checks": ["sdk-component-install", "relocated-external-consumer-compile-link", "native-consumer-run"],
    }
    (output / "sdk-release.json").write_text(json.dumps(metadata, indent=2, sort_keys=True) + "\n")
    print(output / "sdk-release.json")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-root", type=Path, required=True)
    identity = parser.add_mutually_exclusive_group(required=True)
    identity.add_argument("--source-sha")
    identity.add_argument("--development-source", action="store_true")
    parser.add_argument("--qt-root", type=Path, required=True)
    parser.add_argument("--work-directory", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--jobs", type=int, default=2)
    parser.add_argument("--cmake", default="cmake")
    parser.add_argument("--ctest", default="ctest")
    parser.add_argument("--generator")
    args = parser.parse_args()
    if args.jobs < 1:
        parser.error("--jobs must be positive")
    try:
        build(args.source_root, args.source_sha, args.qt_root, args.work_directory,
              args.output, args.jobs, args.cmake, args.ctest, args.generator)
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        parser.exit(2, f"Display SDK build failed: {error}\n")


if __name__ == "__main__":
    main()
