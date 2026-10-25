#!/usr/bin/env python3
"""Consume only the checksum-pinned public mirror of a qualified Cockpit runtime."""
import argparse
import hashlib
import gzip
import importlib.util
import json
import os
from pathlib import Path, PurePosixPath
import re
import tarfile
import time
import urllib.parse
import urllib.request

PUBLISHER = "Dockpipe-Industries/dockpipe"
SOURCE = "Dockpipe-Industries/SpatialPipe"
MAX_METADATA = 4 << 20
MAX_ARCHIVE = 2 << 30
MAX_EXPANDED = 4 << 30
MAX_MEMBERS = 10000
MAX_TAR_BYTES = MAX_EXPANDED + MAX_MEMBERS * 4096 + MAX_METADATA
LEGACY_CHECKS = {"cockpit-ctest", "x11-spatial-gpu", "x11-package-rendering", "private-symbol-split",
          "bundled-runtime-closure", "bundled-x11-package-rendering"}
CHECKS = LEGACY_CHECKS | {"x11-shared-interior-rendering"}


def composer_module():
    spec = importlib.util.spec_from_file_location("cockpit_store", Path(__file__).resolve().parents[5] / 'workflows/catalog/compose/assets/scripts/compose-package-store.py')
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


composer = composer_module()


def read_json(path):
    if path.is_symlink() or not path.is_file() or path.stat().st_size > MAX_METADATA:
        raise ValueError("Cockpit metadata must be a bounded regular file")
    return json.loads(path.read_text(), object_pairs_hook=composer.reject_duplicate_keys)


def digest(path):
    checksum = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1 << 20), b""):
            checksum.update(block)
    return checksum.hexdigest()


def identity(path):
    return {"name": path.name, "sha256": digest(path), "size_bytes": path.stat().st_size}


def validate_identity(value, maximum):
    if (not isinstance(value, dict) or not isinstance(value.get("sha256"), str)
            or not re.fullmatch(r"[0-9a-f]{64}", value["sha256"])
            or type(value.get("size_bytes")) is not int or not 0 < value["size_bytes"] <= maximum):
        raise ValueError("Cockpit input requires an exact bounded SHA-256/byte-size pin")


def release_url(lock, name):
    tag = urllib.parse.quote(lock["publisher"]["release_tag"], safe="")
    return f"https://github.com/{PUBLISHER}/releases/download/{tag}/{name}"


def load_lock(path):
    lock = read_json(path)
    if (lock.get("schema") != 1 or lock.get("product") != "Cockpit"
            or not isinstance(lock.get("version"), str) or not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", lock["version"])
            or lock.get("platform") != "linux-amd64" or lock.get("qt_version") != "6.10.3"
            or lock.get("source_repository") != SOURCE
            or not re.fullmatch(r"[0-9a-f]{40}", lock.get("source_sha", ""))):
        raise ValueError("Cockpit lock lacks a committed source, version or qualified platform")
    publisher = lock.get("publisher", {})
    if (publisher.get("repository") != PUBLISHER
            or not re.fullmatch(r"[A-Za-z0-9_.-]+", publisher.get("release_tag", ""))
            or not re.fullmatch(r"[0-9a-f]{40}", publisher.get("source_sha", ""))):
        raise ValueError("Cockpit lock requires an exact public mirror repository, tag and host SDK source")
    names = {"candidate": "cockpit-candidate.json", "artifact": f"dockpipe-resolver-cockpit-{lock['version']}.tar.gz",
             "qualification": "cockpit-qualification.json"}
    for key, name in names.items():
        pin = lock.get(key)
        validate_identity(pin, MAX_ARCHIVE if key == "artifact" else MAX_METADATA)
        if set(pin) != {"url", "sha256", "size_bytes"} or pin["url"] != release_url(lock, name):
            raise ValueError("Cockpit pins must belong to one exact public runtime-mirror release")
    return lock


def api(path):
    headers = {"Accept": "application/vnd.github+json", "X-GitHub-Api-Version": "2022-11-28"}
    token = os.environ.get("GH_TOKEN")
    if token:
        headers["Authorization"] = "Bearer " + token
    request = urllib.request.Request("https://api.github.com/" + path, headers=headers)
    # API requests never follow redirects with their optional authorization header.
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, request, file, code, message, headers, new_url):
            return None
    with urllib.request.build_opener(NoRedirect()).open(request, timeout=30) as response:
        payload = response.read(MAX_METADATA + 1)
    if len(payload) > MAX_METADATA:
        raise ValueError("Public publisher API response exceeds metadata capacity")
    return json.loads(payload, object_pairs_hook=composer.reject_duplicate_keys)


def verify_publisher(lock):
    pin = lock["publisher"]
    tag = urllib.parse.quote(pin["release_tag"], safe="")
    release = api(f"repos/{PUBLISHER}/releases/tags/{tag}")
    if (release.get("draft") is not False or release.get("prerelease") is not True
            or release.get("tag_name") != pin["release_tag"]
            or release.get("target_commitish") != pin["source_sha"]):
        raise ValueError("Public mirror does not bind the committed host SDK source")
    reference = api(f"repos/{PUBLISHER}/git/ref/tags/{tag}")["object"]
    visited = set()
    for _ in range(8):
        if reference.get("type") == "commit":
            if reference.get("sha") != pin["source_sha"]:
                raise ValueError("Public mirror tag host SDK source differs")
            return release
        sha = reference.get("sha", "")
        if reference.get("type") != "tag" or not re.fullmatch(r"[0-9a-f]{40}", sha) or sha in visited:
            raise ValueError("Public mirror tag is invalid or cyclic")
        visited.add(sha)
        reference = api(f"repos/{PUBLISHER}/git/tags/{sha}")["object"]
    raise ValueError("Public mirror annotated tag exceeds its bounded peel depth")


def fetch(pin, destination):
    validate_identity(pin, MAX_ARCHIVE if destination.name.endswith(".tar.gz") else MAX_METADATA)
    count = 0
    checksum = hashlib.sha256()
    deadline = time.monotonic() + 300
    # Runtime URLs are public; never forward tokens to release/CDN redirects.
    request = urllib.request.Request(pin["url"], headers={"User-Agent": "Dockpipe-Cockpit-Consumer"})
    with urllib.request.urlopen(request, timeout=30) as response, destination.open("xb") as output:
        if urllib.parse.urlsplit(response.url).scheme != "https":
            raise ValueError("Cockpit download redirected outside HTTPS")
        while block := response.read(min(1 << 20, pin["size_bytes"] - count + 1)):
            count += len(block)
            if time.monotonic() > deadline or count > pin["size_bytes"]:
                raise ValueError("Cockpit download exceeded pinned size or five-minute deadline")
            checksum.update(block)
            output.write(block)
    if count != pin["size_bytes"] or checksum.hexdigest() != pin["sha256"]:
        raise ValueError("Cockpit public asset differs from its committed pin")


def verify_sdk_identity(dependency, pin, product, version, repository):
    expected = {"product": product, "version": version, "source_repository": repository,
                "platform": "linux-amd64", "qt_version": "6.10.3"}
    for value in (dependency, pin):
        if (not isinstance(value, dict) or any(value.get(key) != item for key, item in expected.items())
                or not isinstance(value.get("source_sha"), str)
                or not re.fullmatch(r"[0-9a-f]{40}", value["source_sha"])):
            raise ValueError("Cockpit SDK requires its exact supported product, version, repository and source commit")
        validate_identity(value.get("artifact"), MAX_ARCHIVE)
        validate_identity(value.get("metadata"), MAX_METADATA)
    abi = {"cxx_standard": 17}
    if product == "DockPipeLauncherSDK":
        abi["workspace_interface"] = "com.dockpipe.launcher.WorkspacePackage/2.0"
    if dependency.get("abi") != abi:
        raise ValueError("Cockpit SDK native ABI differs from the host contract")


def verify_store(store, lock):
    if store.is_symlink() or not store.is_dir():
        raise ValueError("Cockpit store must be a regular directory")
    for path in store.iterdir():
        if path.is_symlink() or not path.is_file():
            raise ValueError("Cockpit store permits only regular runtime files")
    candidate = read_json(store / "cockpit-candidate.json")
    if identity(store / "cockpit-candidate.json") != {"name": "cockpit-candidate.json", **{k: lock["candidate"][k] for k in ("sha256", "size_bytes")}}:
        raise ValueError("Cockpit candidate metadata differs from committed pin")
    if (candidate.get("schema") != 1
            or any(candidate.get(key) != lock[key] for key in ("product", "version", "platform", "qt_version", "source_repository", "source_sha"))
            or candidate.get("channel") != "staging"
            or type(candidate.get("qualification_run_id")) is not int or candidate["qualification_run_id"] <= 0
            or type(candidate.get("qualification_run_attempt")) is not int or candidate["qualification_run_attempt"] <= 0):
        raise ValueError("Cockpit candidate source or qualification run differs")
    archive_name = f"dockpipe-resolver-cockpit-{lock['version']}.tar.gz"
    names = {archive_name, archive_name + ".sha256", "packages-store-manifest.json", "cockpit-qualification.json"}
    assets = candidate.get("runtime_assets", [])
    if len(assets) != len(names) or {item.get("name") for item in assets} != names:
        raise ValueError("Cockpit runtime allowlist differs; private diagnostics cannot be consumed")
    if {path.name for path in store.iterdir()} != names | {"cockpit-candidate.json", "SHA256SUMS"}:
        raise ValueError("Cockpit store has unlisted runtime or diagnostic files")
    for asset in assets:
        validate_identity(asset, MAX_ARCHIVE if asset["name"] == archive_name else MAX_METADATA)
        path = store / asset["name"]
        if path.is_symlink() or not path.is_file() or identity(path) != asset:
            raise ValueError("Cockpit runtime bytes differ from candidate metadata")
    for key, name in (("artifact", archive_name), ("qualification", "cockpit-qualification.json")):
        if identity(store / name) != {"name": name, **{k: lock[key][k] for k in ("sha256", "size_bytes")}}:
            raise ValueError("Cockpit runtime/receipt differs from committed pins")
    receipt = read_json(store / "cockpit-qualification.json")
    manifest = read_json(store / "packages-store-manifest.json")
    entries, _ = composer.validate_store(manifest, candidate=True)
    if manifest.get("platform") != lock["platform"] or set(entries) != {("resolver", "cockpit")}:
        raise ValueError("Cockpit candidate must own exactly resolver/cockpit")
    entry = entries[("resolver", "cockpit")]
    if lock["version"] not in {"0.1.2", "0.1.3"}:
        raise ValueError("Cockpit candidate requires a supported dependency contract")
    required_checks = LEGACY_CHECKS if lock["version"] == "0.1.2" else CHECKS
    if (entry["version"] != lock["version"] or entry["tarball"] != archive_name
            or entry["sha256"] != lock["artifact"]["sha256"] or entry["size_bytes"] != lock["artifact"]["size_bytes"]
            or receipt.get("schema") != 1 or receipt.get("resolver") != entry
            or any(receipt.get(key) != lock[key] for key in ("platform", "qt_version", "source_repository", "source_sha"))
            or len(receipt.get("checks", [])) != len(required_checks)
            or set(receipt.get("checks", [])) != required_checks
            or candidate.get("qualification") != identity(store / "cockpit-qualification.json")):
        raise ValueError("Cockpit source, resolver or native qualification differs")
    pins = candidate.get("dependencies", {})
    if lock["version"] == "0.1.2":
        sdk_contracts = (
            ("launcher_sdk", "launcher-sdk.lock.json", "DockPipeLauncherSDK", "1.0.0", PUBLISHER),
            ("spatialpipe_sdk", "spatialpipe-sdk.lock.json", "SpatialPipe", "0.1.0", SOURCE))
    else:
        sdk_contracts = (
            ("launcher_sdk", "launcher-sdk.lock.json", "DockPipeLauncherSDK", "1.1.0", PUBLISHER),
            ("spatialpipe_scene_sdk", "spatialpipe-scene-sdk.lock.json", "SpatialPipeScene", "0.1.0", SOURCE),
            ("spatial_web_projection_sdk", "spatialweb-projection-sdk.lock.json", "SpatialWebProjectionSDK", "0.1.0", "Dockpipe-Industries/Spatial-Web"))
    for key, filename, product, version, repository in sdk_contracts:
        pin = pins[filename]["lock"]
        dependency = receipt["dependencies"][key]
        verify_sdk_identity(dependency, pin, product, version, repository)
        if any(dependency.get(field) != pin.get(field) for field in ("product", "version", "platform", "qt_version", "source_repository", "source_sha")):
            raise ValueError("Cockpit dependency source differs from committed candidate pins")
        for field in ("sha256", "size_bytes"):
            if dependency["artifact"][field] != pin["artifact"][field] or dependency["metadata"][field] != pin["metadata"][field]:
                raise ValueError("Cockpit dependency bytes differ from committed candidate pins")
    dependencies = receipt["dependencies"]
    if dependencies["launcher_sdk"]["source_sha"] != lock["publisher"]["source_sha"]:
        raise ValueError("Public mirror host source differs from qualified LauncherSDK source")
    compiler = candidate.get("compiler")
    if (not isinstance(compiler, dict) or set(compiler) != {"id", "version"}
            or any(not isinstance(value, str) or not value or len(value) > 80 for value in compiler.values())
            or compiler != dependencies["launcher_sdk"]["compiler"]
            or any(compiler != dependencies[key]["compiler"] for key, *_ in sdk_contracts)):
        raise ValueError("Cockpit dependency compiler identities differ")
    forge = pins["forgepipe-sdk.lock.json"]["lock"]
    if receipt.get("sdk_sha256") != forge["artifact"]["sha256"] or receipt.get("sdk_source_sha") != forge["source_sha"]:
        raise ValueError("Cockpit ForgePipe source differs from candidate pin")
    if (store / "SHA256SUMS").stat().st_size > MAX_METADATA:
        raise ValueError("Cockpit checksum inventory exceeds metadata capacity")
    sums = "".join(f"{digest(path)}  {path.name}\n" for path in sorted(store.iterdir()) if path.name != "SHA256SUMS")
    if (store / "SHA256SUMS").read_text() != sums or (store / (archive_name + ".sha256")).read_text().strip() != entry["sha256"]:
        raise ValueError("Cockpit checksum inventory differs")
    return manifest, receipt, candidate


class BoundedTarInfo(tarfile.TarInfo):
    @classmethod
    def frombuf(cls, buffer, encoding, errors):
        member = super().frombuf(buffer, encoding, errors)
        extended = {tarfile.XHDTYPE, tarfile.XGLTYPE, tarfile.GNUTYPE_LONGNAME, tarfile.GNUTYPE_LONGLINK}
        if member.type in extended and member.size > MAX_METADATA:
            raise ValueError("Cockpit extended tar header exceeds metadata capacity")
        return member


class BoundedTarReader:
    def __init__(self, source):
        self.source = source
        self.total = 0

    def read(self, size):
        if size < 0 or size > MAX_METADATA:
            raise ValueError("Cockpit tar read exceeds metadata capacity")
        data = self.source.read(min(size, MAX_TAR_BYTES - self.total + 1))
        self.total += len(data)
        if self.total > MAX_TAR_BYTES:
            raise ValueError("Cockpit decompressed tar exceeds capacity")
        return data


def unpack_runtime(archive, output):
    total = 0
    seen = set()
    with gzip.open(archive, "rb") as compressed, tarfile.open(
            fileobj=BoundedTarReader(compressed), mode="r|", ignore_zeros=True, tarinfo=BoundedTarInfo) as bundle:
        for member in bundle:
            name = member.name.rstrip("/") if member.isdir() else member.name
            relative = PurePosixPath(name)
            if (name != relative.as_posix() or relative.is_absolute() or ".." in relative.parts
                    or "\\" in name or ":" in name or name in seen or len(seen) >= MAX_MEMBERS
                    or not (name == "resolvers/cockpit" and member.isdir() or name.startswith("resolvers/cockpit/"))):
                raise ValueError("Cockpit archive contains unsafe, duplicate or unowned paths")
            seen.add(name)
            path = output / name
            if member.isdir():
                path.mkdir(parents=True, exist_ok=True)
            elif member.type in (tarfile.REGTYPE, tarfile.AREGTYPE) and not member.issparse():
                total += member.size
                if member.size < 0 or total > MAX_EXPANDED or name.endswith(".debug") or "private-debug" in relative.parts:
                    raise ValueError("Cockpit archive exceeds runtime capacity or includes diagnostics")
                path.parent.mkdir(parents=True, exist_ok=True)
                with bundle.extractfile(member) as source, path.open("xb") as destination:
                    while block := source.read(1 << 20):
                        destination.write(block)
                path.chmod(0o755 if member.mode & 0o111 else 0o644)
            else:
                raise ValueError("Cockpit runtime archive cannot contain links, sparse or special files")
    assets = output / "resolvers/cockpit/assets"
    reader = "cockpit-reader" if archive.name == "dockpipe-resolver-cockpit-0.1.2.tar.gz" else "dockpipe-workspace-reader"
    if not (assets / "ui/libdockpipe-cockpit.so").is_file() or not (assets / "ui" / reader).is_file():
        raise ValueError("Cockpit archive lacks its runtime module or bounded reader")
    return assets


def prepare(lock_path, output):
    lock = load_lock(lock_path)
    release = verify_publisher(lock)
    if output.exists() or output.is_symlink():
        raise ValueError("Cockpit preparation output must be fresh")
    output.mkdir(parents=True)
    store = output / "store"
    store.mkdir()
    fetch(lock["candidate"], store / "cockpit-candidate.json")
    candidate = read_json(store / "cockpit-candidate.json")
    archive_name = f"dockpipe-resolver-cockpit-{lock['version']}.tar.gz"
    expected_names = {archive_name, archive_name + ".sha256", "packages-store-manifest.json", "cockpit-qualification.json"}
    assets = candidate.get("runtime_assets", [])
    if len(assets) != 4 or {item.get("name") for item in assets} != expected_names:
        raise ValueError("Public candidate runtime allowlist differs")
    expected = {item["name"]: item for item in assets}
    expected["cockpit-candidate.json"] = {"name": "cockpit-candidate.json", **{k: lock["candidate"][k] for k in ("sha256", "size_bytes")}}
    remote = release.get("assets", [])
    if len(remote) != 6 or {item.get("name") for item in remote} != expected_names | {"cockpit-candidate.json", "SHA256SUMS"}:
        raise ValueError("Public mirror contains unexpected assets, including possible diagnostics")
    for item in remote:
        if item["name"] == "SHA256SUMS":
            validate_identity({"sha256": item.get("digest", "").removeprefix("sha256:"), "size_bytes": item.get("size")}, MAX_METADATA)
            continue
        pin = expected[item["name"]]
        if item.get("digest") != "sha256:" + pin["sha256"] or item.get("size") != pin["size_bytes"]:
            raise ValueError("Public mirror API bytes differ from committed candidate")
    for name, pin in expected.items():
        if name != "cockpit-candidate.json":
            fetch({**pin, "url": release_url(lock, name)}, store / name)
    sums = next(item for item in remote if item["name"] == "SHA256SUMS")
    fetch({"url": release_url(lock, "SHA256SUMS"), "sha256": sums["digest"].removeprefix("sha256:"), "size_bytes": sums["size"]}, store / "SHA256SUMS")
    verify_store(store, lock)
    assets_path = unpack_runtime(store / archive_name, output / "package")
    (output / "public-publisher.json").write_text(json.dumps({"publisher": lock["publisher"], "candidate_sha256": lock["candidate"]["sha256"]}, indent=2) + "\n")
    return assets_path


def verify_compiler(build, candidate):
    paths = list((build / "CMakeFiles").glob("*/CMakeCXXCompiler.cmake"))
    if len(paths) != 1:
        raise ValueError("Host qualification requires an unambiguous configured compiler")
    text = paths[0].read_text()
    compiler = {}
    for key, variable in (("id", "CMAKE_CXX_COMPILER_ID"), ("version", "CMAKE_CXX_COMPILER_VERSION")):
        matches = re.findall(r'set\(' + variable + r' "([^"\r\n]+)"\)', text)
        if len(matches) != 1:
            raise ValueError("Host compiler identity is missing or ambiguous")
        compiler[key] = matches[0]
    if compiler != candidate["compiler"]:
        raise ValueError("Host compiler differs from the qualified Cockpit dependency ABI")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    subcommands = parser.add_subparsers(dest="command", required=True)
    for command in ("prepare", "verify-store", "extract", "verify-compiler"):
        subcommand = subcommands.add_parser(command)
        subcommand.add_argument("--lock", type=Path, required=True)
        if command != "prepare":
            subcommand.add_argument("--store", type=Path, required=True)
        if command in {"prepare", "extract"}:
            subcommand.add_argument("--output", type=Path, required=True)
        if command == "verify-compiler":
            subcommand.add_argument("--build", type=Path, required=True)
    args = parser.parse_args()
    try:
        if args.command == "prepare":
            print(prepare(args.lock, args.output))
        else:
            lock = load_lock(args.lock)
            _, _, candidate = verify_store(args.store, lock)
            if args.command == "verify-compiler":
                verify_compiler(args.build, candidate)
            if args.command == "extract":
                if args.output.exists() or args.output.is_symlink():
                    raise ValueError("Cockpit extraction output must be fresh")
                print(unpack_runtime(args.store / f"dockpipe-resolver-cockpit-{lock['version']}.tar.gz", args.output))
    except (OSError, ValueError, KeyError, TypeError, tarfile.TarError) as error:
        parser.exit(1, f"Pinned Cockpit consumption failed: {error}\n")


if __name__ == "__main__":
    main()
