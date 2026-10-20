#!/usr/bin/env python3
"""Build a relocatable launcher host SDK from an explicitly supplied source tree."""
import argparse
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import tarfile
import tempfile


VERSION = "1.0.0"
QT_VERSION = "6.10.3"
ARCHIVE_ROOT = f"DockPipeLauncherSDK-{VERSION}-Linux-amd64"
MAX_FILES = 4096
MAX_INSTALLED_BYTES = 512 << 20
LAUNCHER = Path("src/app/tooling/dockpipe-launcher")


def digest(path):
    checksum = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1 << 20), b""):
            checksum.update(block)
    return checksum.hexdigest()


def inventory(root):
    """Flatten only owned install aliases; public archives contain no links."""
    entries = []
    total_bytes = 0
    for directory, directories, filenames in os.walk(root, followlinks=False):
        current = Path(directory)
        for name in directories:
            if (current / name).is_symlink():
                raise ValueError("SDK directory aliases are not supported")
        for name in sorted(filenames):
            path = current / name
            resolved = path.resolve(strict=True)
            if not resolved.is_relative_to(root.resolve()) or not resolved.is_file():
                raise ValueError("SDK payload escapes its installed prefix or is not a regular file")
            size = resolved.stat().st_size
            total_bytes += size
            if len(entries) >= MAX_FILES or total_bytes > MAX_INSTALLED_BYTES:
                raise ValueError("Launcher SDK exceeds its 4096-file/512-MiB distribution contract")
            entries.append({"path": path.relative_to(root).as_posix(), "bytes": size,
                            "sha256": digest(resolved)})
    if not entries:
        raise ValueError("Installed launcher SDK is empty")
    return sorted(entries, key=lambda entry: entry["path"])


def write_archive(root, archive, entries):
    with archive.open("xb") as destination:
        with gzip.GzipFile(fileobj=destination, mode="wb", filename="", mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode="w", format=tarfile.PAX_FORMAT) as bundle:
                for entry in entries:
                    source = root / entry["path"]
                    member = tarfile.TarInfo(ARCHIVE_ROOT + "/" + entry["path"])
                    member.size = entry["bytes"]
                    member.mode = 0o755 if source.stat().st_mode & 0o111 else 0o644
                    with source.open("rb") as stream:
                        bundle.addfile(member, stream)


def source_identity(root):
    files = []
    ignored = {".git", ".dockpipe", "__pycache__", "CMakeFiles", "node_modules", "build"}
    for directory, directories, filenames in os.walk(root / LAUNCHER, followlinks=False):
        directories[:] = [name for name in directories
                          if name not in ignored and not name.startswith("build-")]
        if any((Path(directory) / name).is_symlink() for name in directories):
            raise ValueError("Launcher source snapshot cannot import directory aliases")
        for name in filenames:
            path = Path(directory) / name
            if path.is_symlink() or not path.is_file():
                raise ValueError("Launcher source snapshot must contain regular authored files")
            files.append(path)
    files.extend(root / name for name in (
        "LICENSE", "packages/native-sdk/assets/scripts/elf_artifacts.py",
        "release/packaging/desktop/licenses/Qt-LGPL-3.txt",
        "release/packaging/desktop/licenses/Qt-GPL-3.txt"))
    if any(path.is_symlink() or not path.is_file() for path in files):
        raise ValueError("SDK attribution and tooling inputs must be regular authored files")
    identity = [{"path": path.relative_to(root).as_posix(), "bytes": path.stat().st_size,
                 "sha256": digest(path)} for path in sorted(files)]
    return hashlib.sha256(json.dumps(identity, sort_keys=True, separators=(",", ":")).encode()).hexdigest()


def run(arguments, timeout, environment=None):
    subprocess.run([str(argument) for argument in arguments], check=True,
                   timeout=timeout, env=environment)


def compiler_identity(build):
    candidates = list((build / "CMakeFiles").glob("*/CMakeCXXCompiler.cmake"))
    if len(candidates) != 1:
        raise ValueError("Build did not produce one unambiguous C++ compiler identity")
    content = candidates[0].read_text()
    result = {}
    for key, cmake_key in (("id", "CMAKE_CXX_COMPILER_ID"), ("version", "CMAKE_CXX_COMPILER_VERSION")):
        match = re.search(r'set\(' + cmake_key + r' "([^"\n]+)"\)', content)
        if not match:
            raise ValueError(f"Compiler identity is missing {cmake_key}")
        result[key] = match.group(1)
    return result



def protect_runtime(prefix):
    tool_path = prefix / "share/DockPipeLauncherSDK/tools/elf_artifacts.py"
    spec = importlib.util.spec_from_file_location("launcher_sdk_elf", tool_path)
    elf = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(elf)
    library = prefix / "lib/libdockpipe-launcher-ui.so.1.0"
    expected_exports = elf.exports(library)
    before = elf.audit(library, expected_exports)
    run(["strip", "--strip-unneeded", library], 60)
    after = elf.audit(library, expected_exports)
    allocated_before = {name: value for name, value in before["sections"].items() if "A" in value["flags"]}
    allocated_after = {name: value for name, value in after["sections"].items() if "A" in value["flags"]}
    if before["build_id"] != after["build_id"] or allocated_before != allocated_after:
        raise ValueError("Launcher SDK symbol cleanup changed executable runtime bytes")
    if any(name.startswith((".debug", ".zdebug")) or name in {".symtab", ".gnu_debugdata", ".gdb_index"}
           for name in after["sections"]):
        raise ValueError("Private symbol sections remain in the launcher shared runtime")
    # The browser is a linkable SDK archive: its link-time symbols are required.
    run(["strip", "--strip-debug", prefix / "lib/libdockpipe-browser.a"], 60)

def build(source, source_sha, qt, work, output, jobs):
    source = source.resolve(strict=True)
    qt = qt.resolve(strict=True)
    work = work.resolve()
    output = output.resolve()
    if platform.system() != "Linux" or platform.machine() != "x86_64":
        raise ValueError("Launcher SDK candidates currently support Linux amd64 only")
    if work.is_relative_to(source) or output.is_relative_to(source):
        raise ValueError("SDK work and output directories must be outside the supplied source tree")
    if work.exists() or output.exists() or work == output or work.is_relative_to(output) or output.is_relative_to(work):
        raise ValueError("Use separate fresh work and output directories")
    if source_sha is not None and not re.fullmatch(r"[0-9a-f]{40}", source_sha):
        raise ValueError("A frozen source snapshot requires an exact 40-character source SHA")
    qmake = qt / "bin/qmake"
    selected_qt = subprocess.check_output([str(qmake), "-query", "QT_VERSION"], text=True, timeout=30).strip()
    if selected_qt != QT_VERSION:
        raise ValueError("Launcher SDK requires Qt 6.10.3 exactly")
    identity = source_identity(source)
    work.mkdir(parents=True)
    output.mkdir(parents=True)
    native_build = work / "build"
    prefix = work / "installed"
    run(["cmake", "-S", source / LAUNCHER, "-B", native_build,
         "-DCMAKE_BUILD_TYPE=Release", "-DBUILD_TESTING=OFF",
         "-DDOCKPIPE_COCKPIT_ASSETS=", "-DDOCKPIPE_INSTALL_LAUNCHER_SDK=ON",
         f"-DCMAKE_PREFIX_PATH={qt}", f"-DQt6_DIR={qt}/lib/cmake/Qt6",
         f"-DCMAKE_INSTALL_PREFIX={prefix}",
         f"-DCMAKE_CXX_FLAGS_RELEASE=-O2 -DNDEBUG -ffile-prefix-map={source}=. -ffile-prefix-map={work}=."], 300)
    run(["cmake", "--build", native_build, "--target", "dockpipe-launcher-ui", "dockpipe-browser",
         "--parallel", jobs], 1800)
    run(["cmake", "--install", native_build, "--component", "LauncherSDK"], 120)
    relocated = work / ARCHIVE_ROOT
    prefix.rename(relocated)
    protect_runtime(relocated)
    consumer_build = work / "consumer"
    run(["cmake", "-S", source / LAUNCHER / "tests/sdk-consumer", "-B", consumer_build,
         f"-DCMAKE_PREFIX_PATH={relocated};{qt}"], 120)
    run(["cmake", "--build", consumer_build, "--parallel", jobs], 300)
    if source_identity(source) != identity:
        raise ValueError("Source snapshot changed during launcher SDK qualification")
    package_sdk(relocated, native_build, source_sha, identity, output)


def package_sdk(relocated, native_build, source_sha, identity, output):
    entries = inventory(relocated)
    manifest = {"schema": 1, "artifact_class": "sdk", "files": entries}
    manifest_bytes = (json.dumps(manifest, indent=2, sort_keys=True) + "\n").encode()
    # The metadata is the final commit marker. It is external to the archive,
    # avoiding a checksum cycle between the payload and its provenance.
    with tempfile.TemporaryDirectory(prefix=".launcher-sdk-", dir=output) as temporary:
        staging = Path(temporary)
        archive = staging / (ARCHIVE_ROOT + ".tar.gz")
        write_archive(relocated, archive, entries)
        (staging / "sdk-files.json").write_bytes(manifest_bytes)
        metadata = {
            "schema": 1, "product": "DockPipeLauncherSDK", "version": VERSION,
            "platform": "linux-amd64", "qt_version": QT_VERSION,
            "source_repository": "Dockpipe-Industries/dockpipe", "source_sha": source_sha,
            "source_state": "committed-snapshot" if source_sha else "development",
            "source_digest": identity, "compiler": compiler_identity(native_build),
            "abi": {"cxx_standard": 17, "workspace_interface": "com.dockpipe.launcher.WorkspacePackage/2.0"},
            "archive": {"name": archive.name, "size_bytes": archive.stat().st_size, "sha256": digest(archive)},
            "file_manifest": {"name": "sdk-files.json", "sha256": hashlib.sha256(manifest_bytes).hexdigest()},
            "checks": ["sdk-component-install", "shared-runtime-symbol-cleanup", "relocated-external-consumer-compile-link"],
            "symbol_policy": {"shared_library": "strip-unneeded", "static_archive": "link-symbols-retained"},
        }
        (staging / "sdk-release.json").write_text(json.dumps(metadata, indent=2, sort_keys=True) + "\n")
        for name in (archive.name, "sdk-files.json", "sdk-release.json"):
            (staging / name).rename(output / name)
    print(output / "sdk-release.json")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-root", type=Path, required=True)
    identity = parser.add_mutually_exclusive_group(required=True)
    identity.add_argument("--source-sha", help="Exact commit of the caller-supplied frozen source snapshot")
    identity.add_argument("--development-source", action="store_true", help="Record uncommitted inputs without claiming a source commit")
    parser.add_argument("--qt-root", type=Path, required=True)
    parser.add_argument("--work-directory", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--jobs", type=int, default=2)
    args = parser.parse_args()
    if args.jobs < 1:
        parser.error("--jobs must be positive")
    try:
        build(args.source_root, args.source_sha, args.qt_root, args.work_directory, args.output, args.jobs)
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        parser.exit(2, f"Launcher SDK build failed: {error}\n")


if __name__ == "__main__":
    main()
