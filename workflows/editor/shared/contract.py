"""Qualified VSIX contract shared by this repository's editor workflows."""
import fnmatch
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import zipfile
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parents[3]
EXTENSION = ROOT / "tooling/vscode-extensions/dockpipe-language-support"


def manifest():
    return json.loads((EXTENSION / "package.json").read_text())


def inventory():
    records = []
    for path in sorted(EXTENSION.rglob("*")):
        relative = path.relative_to(EXTENSION)
        if any(part in {"node_modules", "__pycache__", ".git"} for part in relative.parts) or path.suffix == ".vsix":
            continue
        if path.is_symlink():
            raise ValueError("Editor source must not contain symbolic links")
        if path.is_file():
            records.append({"path": relative.as_posix(), "bytes": path.stat().st_size,
                            "sha256": digest(path)})
    return records


def digest(path):
    checksum = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            checksum.update(chunk)
    return checksum.hexdigest()


def binary():
    selected = os.environ.get("DOCKPIPE_BOOTSTRAP_BIN") or os.environ.get("DOCKPIPE_BIN", "")
    path = Path(selected)
    if not selected or not path.is_absolute() or not path.is_file() or not os.access(path, os.X_OK):
        raise ValueError("Select an absolute pinned DOCKPIPE_BOOTSTRAP_BIN or DOCKPIPE_BIN")
    return path


def scope(component):
    result = subprocess.run([str(binary()), "scope", "workflow", "editor.build", component,
                             "--workdir", str(ROOT)], check=True, capture_output=True, text=True, timeout=15)
    value = result.stdout.strip()
    if not Path(value).is_absolute() or any(character in value for character in "\x00\r\n"):
        raise ValueError("Dockpipe did not return a single absolute editor scope")
    path = Path(value)
    path.mkdir(parents=True, exist_ok=True)
    return path


def run(arguments, directory, environment, timeout=600):
    return subprocess.run([str(value) for value in arguments], cwd=directory, env=environment,
                          check=True, timeout=timeout)


def clean_environment():
    environment = dict(os.environ)
    environment.pop("VSCE_PAT", None)
    environment["VSCE_STORE"] = "file"
    environment["npm_config_cache"] = str(scope("npm-cache"))
    return environment


def stage(directory):
    destination = directory / "extension"
    shutil.copytree(EXTENSION, destination, ignore=shutil.ignore_patterns("node_modules", "__pycache__", "*.vsix"))
    return destination


def install_tools(staging, environment):
    npm = shutil.which("npm")
    node = shutil.which("node")
    if not npm or not node:
        raise ValueError("Node 22+ and npm must be installed in the selected host environment")
    version = subprocess.check_output([node, "--version"], text=True, timeout=10).strip()
    if int(version.lstrip("v").split(".")[0]) < 22:
        raise ValueError("Editor tooling requires Node 22 or newer")
    arguments = [npm, "ci", "--ignore-scripts", "--no-audit", "--no-fund"]
    if os.environ.get("DOCKPIPE_EDITOR_NPM_OFFLINE") == "1":
        arguments.append("--offline")
    run(arguments, staging, environment)
    tool = staging / "node_modules/@vscode/vsce/vsce"
    if not tool.is_file():
        raise ValueError("Locked vsce executable is unavailable")
    return node, tool


def inspect_vsix(path, expected_version):
    current = manifest()
    identity = current["publisher"] + "." + current["name"]
    if current["version"] != expected_version:
        raise ValueError("Reviewed version differs from the checked-out extension manifest")
    if path.is_symlink() or not path.is_file():
        raise ValueError("VSIX must be a regular file")
    with zipfile.ZipFile(path) as archive:
        names = archive.namelist()
        if len(set(names)) != len(names) or any("\\" in name or name.startswith("/") or ".." in Path(name).parts for name in names):
            raise ValueError("VSIX contains duplicate or unsafe member names")
        metadata = archive.getinfo("extension/package.json")
        source = (EXTENSION / "package.json").read_bytes()
        if metadata.file_size != len(source):
            raise ValueError("VSIX package metadata bytes differ from the qualified source")
        descriptor = archive.getinfo("extension.vsixmanifest")
        if descriptor.file_size > 1024 * 1024:
            raise ValueError("VSIX manifest exceeds the supported metadata budget")
        descriptor_bytes = archive.read(descriptor)
        if b"<!DOCTYPE" in descriptor_bytes or b"<!ENTITY" in descriptor_bytes:
            raise ValueError("VSIX manifest must not declare entities")
        descriptor_xml = ET.fromstring(descriptor_bytes)
        descriptor_identity = descriptor_xml.find(".//{*}Identity")
        if descriptor_identity is None or descriptor_identity.get("Id") != current["name"] or descriptor_identity.get("Publisher") != current["publisher"] or descriptor_identity.get("Version") != expected_version:
            raise ValueError("VSIX manifest identity or version differs from the qualified source")
        packaged = json.loads(archive.read(metadata))
        if packaged["publisher"] + "." + packaged["name"] != identity or packaged["version"] != expected_version:
            raise ValueError("VSIX identity or version differs from the qualified source")
        for record in inventory():
            packaged_name = {"LICENSE": "LICENSE.txt", "README.md": "readme.md", "CHANGELOG.md": "changelog.md"}.get(record["path"], record["path"])
            member = "extension/" + packaged_name
            required = any(fnmatch.fnmatch(record["path"], pattern) for pattern in current.get("files", []))
            if member not in names:
                if required:
                    raise ValueError("VSIX omits a declared runtime file: " + member)
                continue
            entry = archive.getinfo(member)
            if entry.file_size != record["bytes"]:
                raise ValueError("Packaged editor source has an unexpected size: " + member)
            if hashlib.sha256(archive.read(entry)).hexdigest() != record["sha256"]:
                raise ValueError("Packaged editor source differs from qualification: " + member)
        if "extension/extension.js" not in names:
            raise ValueError("VSIX lacks its runtime entrypoint")
    return identity


def qualify_artifact(directory, version, expected_sha256):
    if len(expected_sha256) != 64 or any(value not in "0123456789abcdef" for value in expected_sha256):
        raise ValueError("Supply the exact reviewed lowercase SHA-256")
    current = manifest()
    if current["version"] != version:
        raise ValueError("Reviewed version differs from the checked-out source")
    path = directory / (current["name"] + "-" + version + ".vsix")
    receipt = json.loads((directory / "extension.json").read_text())
    if receipt.get("schema") != 1 or receipt.get("version") != version or receipt.get("filename") != path.name:
        raise ValueError("VSIX receipt does not match the selected artifact")
    actual = digest(path)
    if actual != expected_sha256 or receipt.get("sha256") != actual or receipt.get("bytes") != path.stat().st_size:
        raise ValueError("VSIX bytes differ from the exact reviewed artifact")
    identity = inspect_vsix(path, version)
    if receipt.get("identity") != identity or receipt.get("source_inventory") != inventory() or receipt.get("qualification") != "tests-and-vsce-package-passed":
        raise ValueError("VSIX source qualification does not match this checkout")
    return path
