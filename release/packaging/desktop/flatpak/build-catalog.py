#!/usr/bin/env python3
"""Rebuild optional archives with package-owned Flatpak dependency profiles.

Every input must have an explicit profile. Qualification checks the shipped
Platform, packaged dependencies and required release payloads. Host application
authentication, live provider operations and VM boot remain separate tests.
"""
import argparse
import importlib.util
import json
from pathlib import Path
import platform
import shlex
import shutil
import subprocess
import tarfile
import tempfile

import yaml


TOOLS = {
    "cloudflared": ("cloudflared", ["--version"]),
    "act": ("act", ["--version"]),
    "infisical": ("infisical", ["--version"]),
    "terraform": ("terraform", ["version"]),
    "node": ("node", ["--version"]),
    "npm": ("node", ["--version"]),
    "go": ("go", ["version"]),
    "aws": ("aws", ["--version"]),
    "az": ("azure", ["version"]),
    "wrangler": ("wrangler", ["--version"]),
}
RUNTIME_COMMANDS = {"bash", "curl", "docker", "git", "python3", "ssh"}
HOST_COMMANDS = {"op", "code", "cursor", "codex", "claude", "npm", "systemctl", "loginctl",
                 "pkexec", "nvidia-smi", "nvidia-ctk", "qemu-img", "qemu-system-x86_64"}


def profiles(root):
    result = {}
    for base in (root / "packages", root / "workflows", root / "src/core/resolvers"):
        for path in base.rglob("assets/flatpak/package.json"):
            profile = json.loads(path.read_text())
            if profile.get("schema") != 1:
                raise ValueError(f"Unknown profile schema: {path}")
            for artifact in profile["artifacts"]:
                key = (artifact["kind"], artifact["name"])
                if key in result:
                    raise ValueError(f"Duplicate Flatpak profile: {key}")
                result[key] = (profile, path)
    return result


def artifact_identity(path):
    with tarfile.open(path) as archive:
        roots = {tuple(Path(member.name).parts[:2]) for member in archive
                 if len(Path(member.name).parts) >= 2}
    if len(roots) != 1:
        raise ValueError(f"Expected one package root: {path}")
    directory, name = roots.pop()
    if directory not in ("resolvers", "workflows"):
        raise ValueError(f"Not an optional package: {path}")
    return directory[:-1], name


def adapt_metadata(package, default_namespace):
    commands = set()
    for filename in ("package.yml", "config.yml"):
        path = package / filename
        if not path.exists():
            continue
        data = yaml.safe_load(path.read_text())
        if not isinstance(data, dict):
            raise ValueError(f"Invalid package metadata: {path}")
        # This generated artifact is admitted only after the profile checks pass.
        data["platforms"] = ["flatpak"]
        if package.parent.name == "workflows" and not data.get("namespace"):
            data["namespace"] = default_namespace
        for dependency in (data.get("dependencies") or {}).get("host", []):
            command = dependency.get("command") or dependency.get("id")
            if command:
                commands.add(command)
        path.write_text(yaml.safe_dump(data, sort_keys=False))
    return commands


def copy_tools(package, profile, commands, tools):
    host_commands = set(profile["host_commands"])
    host_commands.update(commands & HOST_COMMANDS - set(TOOLS))
    if not host_commands <= HOST_COMMANDS:
        raise ValueError(f"Unapproved host integration: {host_commands - HOST_COMMANDS}")
    bundles = set(profile["tools"])
    for command in commands - host_commands:
        if command in TOOLS:
            bundles.add(TOOLS[command][0])
        elif command not in RUNTIME_COMMANDS:
            raise ValueError(f"Dependency has no Flatpak implementation: {command}")
    if "wrangler" in bundles:
        bundles.add("node")
    destination = package / "assets/tooling/bin"
    destination.mkdir(parents=True, exist_ok=True)
    for bundle in sorted(bundles):
        source = tools / bundle
        if not source.is_dir():
            raise ValueError(f"Missing prepared tool: {bundle}")
        shutil.copytree(source, destination, dirs_exist_ok=True,
                        ignore=shutil.ignore_patterns("__pycache__", "*.pyc"))
    for command in sorted(host_commands):
        path = destination / command
        path.write_text('#!/usr/bin/env bash\nexec /app/bin/dockpipe-host-command '
                        + shlex.quote(command) + ' "$@"\n')
        path.chmod(0o755)
    return bundles, host_commands


def qualify(app, stage, package, commands, bundles, host_commands):
    # Preflight and execute from the selected package, not a checkout or SDK PATH.
    script = package / "assets/flatpak/qualification.sh"
    script.parent.mkdir(parents=True, exist_ok=True)
    lines = ["#!/usr/bin/env bash", "set -euo pipefail", "test -f /.flatpak-info"]
    for command in sorted(commands | host_commands):
        lines.append("command -v " + shlex.quote(command) + " >/dev/null")
    for command, (bundle, arguments) in TOOLS.items():
        if bundle in bundles and command not in host_commands:
            lines.append(shlex.join([command, *arguments]))
    script.write_text("\n".join(lines) + "\n")
    script.chmod(0o755)
    workflow = package / "flatpak-qualification.yml"
    workflow.write_text(yaml.safe_dump({"name": "flatpak-package-check", "platforms": ["flatpak"],
                                      "docker_preflight": False,
                                      "steps": [{"id": "dependencies", "kind": "host",
                                                 "run": "assets/flatpak/qualification.sh"}]}))
    options = ["flatpak", "build", "--runtime", "--unshare=network", "--nofilesystem=host",
               "--nofilesystem=home", "--nofilesystem=/run/docker.sock",
               "--nofilesystem=xdg-run/docker.sock", "--nofilesystem=xdg-run/podman/podman.sock",
               "--no-talk-name=org.freedesktop.Flatpak", f"--filesystem={stage}",
               f"--build-dir={stage}", "--env=AZURE_CORE_COLLECT_TELEMETRY=0",
               "--env=WRANGLER_SEND_METRICS=false", "--env=PYTHONDONTWRITEBYTECODE=1"]
    for key in ("HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME"):
        directory = stage / "test-environment" / key.lower()
        directory.mkdir(parents=True)
        options.append(f"--env={key}={directory}")
    workdir = stage / "test-project"
    workdir.mkdir()
    subprocess.run([*options, str(app), "/app/bin/dockpipe", "--workflow-file", str(workflow),
                    "--workdir", str(workdir)], check=True, timeout=180)
    script.unlink()
    workflow.unlink()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("app", type=Path)
    parser.add_argument("compiled", type=Path, help="fresh native release inputs, including source-build hooks")
    parser.add_argument("tools", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    if platform.machine() != "x86_64":
        raise SystemExit("Optional dependency locks are qualified for Linux amd64 only")
    root = Path(__file__).resolve().parents[4]
    spec = importlib.util.spec_from_file_location("package_tools", Path(__file__).with_name("prepare-package-tools.py"))
    helpers = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(helpers)
    profile_map = profiles(root)
    default_namespace = json.loads((root / "dockpipe.config.json").read_text())["packages"]["namespace"]
    if args.output.exists():
        raise SystemExit("Use a fresh catalog output directory")
    args.output.mkdir(parents=True)
    output = args.output.resolve()
    app = args.app.resolve()
    prepared_tools = args.tools.resolve()
    inputs = sorted(path for kind in ("resolvers", "workflows") for path in (args.compiled / kind).glob("*.tar.gz"))
    identities = {artifact_identity(path) for path in inputs}
    if identities != set(profile_map):
        raise ValueError(f"Catalog/profile mismatch: missing={set(profile_map) - identities}, unowned={identities - set(profile_map)}")
    receipts = []
    for source in inputs:
        kind, name = artifact_identity(source)
        profile, profile_path = profile_map[(kind, name)]
        print(f"Qualifying {kind} {name}", flush=True)
        with tempfile.TemporaryDirectory(prefix="package-", dir=output) as temporary:
            stage = Path(temporary)
            helpers.unpack_regular_files(source, stage, "tar")
            package = stage / (kind + "s") / name
            commands = adapt_metadata(package, default_namespace)
            bundles, host_commands = copy_tools(package, profile, commands, prepared_tools)
            for required in profile["required_files"]:
                helpers.safe_relative(required)
                if not (package / required).is_file():
                    raise ValueError(f"{name} missing release payload {required}; run its source build")
            qualify(app, stage, package, commands, bundles, host_commands)
            target = output / "compiled" / (kind + "s") / source.name
            target.parent.mkdir(parents=True, exist_ok=True)
            with tarfile.open(target, "w:gz", format=tarfile.PAX_FORMAT, compresslevel=3) as archive:
                archive.add(package, arcname=f"{kind}s/{name}")
            digest = helpers.checksum(target)
            target.with_name(target.name + ".sha256").write_text(f"{digest}  {target.name}\n")
            receipts.append({"kind": kind, "name": name, "compiled_sha256": digest,
                             "profile": str(profile_path.relative_to(root)),
                             "bundled_tools": sorted(bundles), "host_requirements": sorted(host_commands),
                             "proof": "Platform dependency execution and package preflight"})
    # Core is platform neutral and was checksum-verified by the desktop builder.
    # It remains separate from the optional archives and is the only package in
    # the base installer; a complete Marketplace store also lists it.
    shutil.copytree(app / "files/share/dockpipe/packages/core", output / "compiled/core")
    version = subprocess.run(["flatpak", "build", "--runtime", str(app),
                              "/app/libexec/dockpipe/dockpipe", "--version"],
                             check=True, text=True, stdout=subprocess.PIPE).stdout.strip()
    subprocess.run(["flatpak", "build", "--runtime", "--unshare=network", f"--filesystem={output}",
                    f"--filesystem={root}:ro",
                    f"--env=XDG_CONFIG_HOME={output / 'config'}", f"--env=XDG_CACHE_HOME={output / 'cache'}",
                    f"--env=XDG_STATE_HOME={output / 'state'}", f"--env=XDG_DATA_HOME={output / 'data'}",
                    f"--env=DOCKPIPE_PACKAGES_ROOT={output / 'compiled'}", str(app),
                    "/app/bin/dockpipe", "package", "build", "store", "--workdir", str(root),
                    "--out", str(output / "store"), "--version", version], check=True)
    manifest = json.loads((output / "store/packages-store-manifest.json").read_text())
    for receipt in receipts:
        entry = next(item for item in manifest["packages"][receipt["kind"] + "s"]
                     if item["name"] == receipt["name"])
        receipt["exported_sha256"] = entry["sha256"]
    (output / "qualification.json").write_text(json.dumps(receipts, indent=2) + "\n")
    print(f"PASS: {len(receipts)} optional package artifacts qualified for the Flatpak Platform")


if __name__ == "__main__":
    main()
