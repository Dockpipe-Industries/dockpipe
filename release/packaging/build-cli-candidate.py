#!/usr/bin/env python3
"""Build a store-free, pure-Go CLI bootstrap candidate from public module pins."""
import argparse
import gzip
import hashlib
import io
import importlib.util
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import sys
import tarfile
import zipfile


TARGETS = ("darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64", "windows/amd64")
GO_VERSION = "go1.26.10"
REPOSITORY = "Dockpipe-Industries/dockpipe"
MAX_BINARY_BYTES = 64 << 20
MAX_HISTORY_BYTES = 64 << 10


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def candidate_identity(base, source_sha):
    if not re.fullmatch(r"(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)", base):
        raise ValueError("VERSION must be a complete numeric release-line version")
    if not re.fullmatch(r"[0-9a-f]{40}", source_sha):
        raise ValueError("CLI candidate requires an exact source commit")
    # Prefix the hash so an all-numeric hash cannot form an invalid leading-zero
    # SemVer prerelease identifier.
    return base + "-staging.cli.sha" + source_sha[:12], "cli-candidate-" + base + "-staging." + source_sha



def release_allocator():
    specification = importlib.util.spec_from_file_location(
        "cli_release_allocator", Path(__file__).with_name("release-version.py"))
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


def allocation_receipt(release_line, source_sha, tags):
    history = "".join(tag + "\n" for tag in sorted(set(tags)))
    if len(history.encode("utf-8")) > MAX_HISTORY_BYTES:
        raise ValueError("CLI published tag history exceeds 64 KiB")
    allocated_base = release_allocator().next_version(release_line, history.splitlines())
    candidate_identity(allocated_base, source_sha)
    return {
        "schema": 1, "source_sha": source_sha, "release_line": release_line,
        "allocated_base": allocated_base, "history": history,
        "history_sha256": hashlib.sha256(history.encode("utf-8")).hexdigest(),
        "history_source": "https://api.github.com/repos/" + REPOSITORY + "/tags",
    }


def validate_allocation(receipt, release_line, source_sha):
    fields = {"schema", "source_sha", "release_line", "allocated_base", "history",
              "history_sha256", "history_source"}
    if (not isinstance(receipt, dict) or set(receipt) != fields
            or type(receipt.get("schema")) is not int or receipt["schema"] != 1
            or receipt.get("source_sha") != source_sha or receipt.get("release_line") != release_line
            or not isinstance(receipt.get("history"), str)
            or len(receipt["history"].encode("utf-8")) > MAX_HISTORY_BYTES):
        raise ValueError("CLI version allocation requires bounded source-bound published history")
    expected = allocation_receipt(release_line, source_sha, receipt["history"].splitlines())
    if receipt != expected:
        raise ValueError("CLI version allocation history, digest or selected base differs")
    return candidate_identity(receipt["allocated_base"], source_sha)


def acquire_version_allocation(source, source_sha, environment):
    """Read history once; reuse an existing source candidate's immutable receipt."""
    release_line = (source / "VERSION").read_text().strip()
    candidate_identity(release_line, source_sha)
    # gh cannot combine --slurp with --jq. @json emits exactly one compact
    # array line per page, retaining pagination without release bodies/authors.
    history_filter = (
        'map({tag_name,draft,prerelease,target_commitish,assets:['
        '.assets[] | select(.name == "cli-candidate.json") | {name,id,size,digest}]}) | @json')
    response = command([
        "gh", "api", "--hostname", "github.com", "--paginate",
        f"repos/{REPOSITORY}/releases?per_page=100", "--jq", history_filter], source, environment)
    pages = [json.loads(line) for line in response.splitlines()]
    if (not isinstance(pages, list) or any(not isinstance(page, list) for page in pages)):
        raise ValueError("CLI release history response must contain paginated release lists")
    releases = [release for page in pages for release in page]
    if any(not isinstance(release, dict) or not isinstance(release.get("tag_name"), str)
           or type(release.get("draft")) is not bool for release in releases):
        raise ValueError("CLI release history contains invalid release metadata")
    existing = [release for release in releases
                if re.fullmatch(r"cli-candidate-[0-9]+\.[0-9]+\.[0-9]+-staging\." + source_sha,
                                release["tag_name"])]
    if len(existing) > 1:
        raise ValueError("Multiple CLI candidates already claim the exact source")
    if existing:
        release = existing[0]
        if release.get("target_commitish") != source_sha or release.get("prerelease") is not True:
            raise ValueError("Existing CLI candidate source identity differs")
        assets = release.get("assets", [])
        if not isinstance(assets, list):
            raise ValueError("Existing CLI candidate assets are invalid")
        manifests = [asset for asset in assets if isinstance(asset, dict)
                     and asset.get("name") == "cli-candidate.json"]
        if len(manifests) != 1:
            raise ValueError("Existing CLI candidate requires one immutable allocation manifest")
        asset = manifests[0]
        if (type(asset.get("id")) is not int or asset["id"] <= 0
                or type(asset.get("size")) is not int or not 0 < asset["size"] <= 128 << 10):
            raise ValueError("Existing CLI candidate manifest must be bounded")
        content = command(["gh", "api", "--hostname", "github.com", "-H", "Accept: application/octet-stream",
                           f"repos/{REPOSITORY}/releases/assets/{asset['id']}"], source, environment)
        data = content.encode("utf-8")
        if (len(data) != asset["size"]
                or "sha256:" + hashlib.sha256(data).hexdigest() != asset.get("digest")):
            raise ValueError("Existing CLI candidate allocation manifest bytes differ")
        manifest = json.loads(content)
        if (not isinstance(manifest, dict) or manifest.get("source_sha") != source_sha
                or manifest.get("source_repository") != REPOSITORY
                or manifest.get("product") != "DockpipeCLI"
                or manifest.get("source_state") != "committed-snapshot"):
            raise ValueError("Existing CLI candidate allocation manifest source differs")
        receipt = manifest.get("version_allocation")
        version, tag = validate_allocation(receipt, release_line, source_sha)
        if manifest.get("version") != version or manifest.get("tag") != tag or release["tag_name"] != tag:
            raise ValueError("Existing CLI candidate identity differs from its allocation")
        return receipt
    # A public native tag consumes its version even if release publication was
    # interrupted. Use the same tag history as the native release allocator.
    response = command([
        "gh", "api", "--hostname", "github.com", "--paginate",
        f"repos/{REPOSITORY}/tags?per_page=100", "--jq", "map(.name) | @json"], source, environment)
    pages = [json.loads(line) for line in response.splitlines()]
    if not pages or any(not isinstance(page, list) for page in pages):
        raise ValueError("CLI version history must contain paginated tag lists")
    tags = [tag for page in pages for tag in page]
    if any(not isinstance(tag, str) or not tag or "\n" in tag or "\r" in tag for tag in tags):
        raise ValueError("CLI version history contains an invalid Git tag")
    return allocation_receipt(release_line, source_sha, tags)


def public_environment(work):
    environment = dict(os.environ)
    for key in ("GOFLAGS", "GOPRIVATE", "GONOPROXY", "GONOSUMDB", "GOPATH", "GOOS", "GOARCH", "GOTOOLDIR", "GOCACHEPROG"):
        environment.pop(key, None)
    environment.update({
        "GOWORK": "off", "GOENV": "off", "GO111MODULE": "on", "GOTOOLCHAIN": "local", "CGO_ENABLED": "0",
        "GOPROXY": "https://proxy.golang.org", "GOSUMDB": "sum.golang.org", "GOAUTH": "off",
        "GOMODCACHE": str(work / "modules"), "GOCACHE": str(work / "go-cache"),
        "GOFLAGS": "-mod=readonly",
    })
    return environment


def archive_binary(binary, archive, windows):
    if binary.is_symlink() or not binary.is_file() or not 0 < binary.stat().st_size <= MAX_BINARY_BYTES:
        raise ValueError("CLI binary must be a bounded regular file")
    data = binary.read_bytes()
    executable = "dockpipe.exe" if windows else "dockpipe"
    if windows:
        with zipfile.ZipFile(archive, "x", zipfile.ZIP_DEFLATED) as bundle:
            member = zipfile.ZipInfo(executable, (1980, 1, 1, 0, 0, 0))
            member.external_attr = 0o100755 << 16
            member.compress_type = zipfile.ZIP_DEFLATED
            bundle.writestr(member, data)
    else:
        with archive.open("xb") as stream, gzip.GzipFile(fileobj=stream, mode="wb", filename="", mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode="w") as bundle:
                member = tarfile.TarInfo(executable)
                member.size = len(data)
                member.mode = 0o755
                bundle.addfile(member, io.BytesIO(data))
    return {"executable": executable, "executable_size_bytes": len(data),
            "executable_sha256": hashlib.sha256(data).hexdigest(),
            "name": archive.name, "size_bytes": archive.stat().st_size, "sha256": digest(archive)}


def command(arguments, source, environment, timeout=300):
    try:
        result = subprocess.run([str(argument) for argument in arguments], cwd=source,
                                env=environment, check=True, capture_output=True, timeout=timeout)
    except subprocess.CalledProcessError as error:
        diagnostics = []
        for label, content in (("stdout", error.stdout), ("stderr", error.stderr)):
            if not content:
                continue
            text = content[:4096].decode("utf-8", errors="replace")
            if len(content) > 4096:
                text += "\n[truncated after 4096 bytes]"
            diagnostics.append(label + ":\n" + text)
        details = "\n".join(diagnostics)
        raise ValueError(
            f"CLI producer {Path(arguments[0]).name} failed with exit {error.returncode}\n{details}") from error
    if len(result.stdout) > 2 << 20:
        raise ValueError("CLI producer command metadata exceeds 2 MiB")
    return result.stdout.decode("utf-8")


def verify_native_cli(binary, version, work, environment):
    smoke = work / "native-smoke"
    smoke.mkdir()
    environment = dict(environment, DOCKPIPE_BIN=str(binary), XDG_STATE_HOME=str(work / "state"),
                       XDG_CACHE_HOME=str(work / "cache"), XDG_CONFIG_HOME=str(work / "config"))
    for key in ("DOCKPIPE_PACKAGES_ROOT", "DOCKPIPE_GLOBAL_ROOT", "DOCKPIPE_WORKSPACE_PACKAGE"):
        environment.pop(key, None)
    actual = command([binary, "--version"], smoke, environment, 15).strip()
    if actual != version:
        raise ValueError("Native CLI candidate version differs")
    (smoke / "dockpipe.config.json").write_text(json.dumps({
        "schema": 1, "dockpipe_version": version, "compile": {"workflows": ["packages"]}}))
    package = smoke / "packages/fixture"
    package.mkdir(parents=True)
    (package / "package.yml").write_text(
        "schema: 1\nkind: package\nname: cli-python-fixture\nversion: 1.0.0\n"
        "build:\n  source:\n    script: build.py\n")
    (package / "build.py").write_text(
        "from pathlib import Path\nPath(__file__).with_name('source-built.txt').write_text('python-source-hook')\n")
    command([binary, "package", "build", "source", "--workdir", smoke], smoke, environment, 30)
    if (package / "source-built.txt").read_text() != "python-source-hook":
        raise ValueError("Native CLI Python package source hook did not run")
    (smoke / "host.py").write_text(
        "from pathlib import Path\nPath(__file__).with_name('host-built.txt').write_text('python-host-step')\n")
    (smoke / "proof.yml").write_text(
        "name: cli-python-host-smoke\ndocker_preflight: false\nsteps:\n"
        "  - id: python-host\n    kind: host\n    run: host.py\n")
    command([binary, "--no-compile-deps", "--no-vault", "--no-op-inject", "--workdir", smoke,
             "--workflow-file", smoke / "proof.yml"], smoke, environment, 30)
    if (smoke / "host-built.txt").read_text() != "python-host-step":
        raise ValueError("Native CLI Python host step did not run")


def build(source, source_sha, work, output, go, version_allocation=None):
    source = source.resolve(strict=True)
    work, output = work.resolve(), output.resolve()
    if (platform.system(), platform.machine()) != ("Linux", "x86_64"):
        raise ValueError("CLI candidate producer requires native Linux amd64")
    if (work.exists() or output.exists() or work.is_relative_to(source) or output.is_relative_to(source)
            or work.is_relative_to(output) or output.is_relative_to(work)):
        raise ValueError("Use separate fresh work/output directories outside the source")
    release_line = (source / "VERSION").read_text().strip()
    if version_allocation is None:
        version_allocation = acquire_version_allocation(source, source_sha, dict(os.environ))
    version, tag = validate_allocation(version_allocation, release_line, source_sha)
    work.mkdir(parents=True)
    output.mkdir(parents=True)
    environment = public_environment(work)
    command([sys.executable, source / "release/packaging/embedded-inputs.py", "--check"],
            source, environment, 120)
    if command([go, "env", "GOVERSION"], source, environment).strip() != GO_VERSION:
        raise ValueError("CLI candidates require the pinned Go 1.26.10 toolchain")
    module = json.loads(command([go, "mod", "edit", "-json"], source, environment))
    if module.get("Replace"):
        raise ValueError("CLI candidates cannot use Go module replacements")
    source_hashes = {name: digest(source / name) for name in ("go.mod", "go.sum")}
    command([go, "mod", "download", "all"], source, environment, 600)
    command([go, "mod", "verify"], source, environment, 300)
    artifacts = {}
    binaries = work / "binaries"
    binaries.mkdir()
    for target in TARGETS:
        operating_system, architecture = target.split("/")
        windows = operating_system == "windows"
        binary = binaries / (target.replace("/", "-") + (".exe" if windows else ""))
        target_environment = dict(environment, GOOS=operating_system, GOARCH=architecture)
        command([go, "build", "-buildvcs=false", "-trimpath", "-ldflags",
                 "-s -w -X main.Version=" + version, "-o", binary, "./src/cmd"],
                source, target_environment, 600)
        if target == "linux/amd64":
            verify_native_cli(binary, version, work, environment)
        name = f"dockpipe_{version}_{operating_system}_{architecture}" + (".zip" if windows else ".tar.gz")
        artifacts[target] = archive_binary(binary, output / name, windows)
    if source_hashes != {name: digest(source / name) for name in source_hashes}:
        raise ValueError("Go module pins changed during CLI qualification")
    metadata = {
        "schema": 1, "product": "DockpipeCLI", "artifact_class": "cli-only", "version": version,
        "tag": tag, "source_repository": REPOSITORY, "source_sha": source_sha,
        "version_allocation": version_allocation,
        "source_state": "committed-snapshot", "go_version": GO_VERSION, "module_files": source_hashes,
        "module_acquisition": "fresh-public-proxy-and-checksum-database", "artifacts": artifacts,
        "checks": ["authored-embed-manifest", "public-module-download", "module-integrity", "pure-go-five-platform-build",
                   "native-linux-version", "native-python-package-source-hook", "native-python-host-step"],
        "native_execution_platforms": ["linux/amd64"],
        "excluded": ["Qt", "package-stores", "installers", "application-release", "production-channel"],
    }
    (output / "cli-candidate.json").write_text(json.dumps(metadata, indent=2, sort_keys=True) + "\n")
    print(output / "cli-candidate.json")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-root", type=Path, required=True)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--work-directory", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--go", default="go")
    args = parser.parse_args()
    try:
        build(args.source_root, args.source_sha, args.work_directory, args.output, args.go)
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        parser.exit(1, f"CLI candidate build failed: {error}\n")


if __name__ == "__main__":
    main()
