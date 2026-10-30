#!/usr/bin/env python3
"""Install the release-locked bulk workflow and its hash-locked SDK runtime."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
from urllib.parse import urlsplit
from urllib.request import HTTPRedirectHandler, ProxyHandler, Request, build_opener

PACKAGE_NAME = "dockpipe.cloudflare.r2bulk"
PACKAGE_FILES = {"package.yml": 64 * 1024, "requirements.lock": 1024 * 1024}
DOWNLOAD_HOSTS = {"github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com"}
MANIFEST_LIMIT = 2 * 1024 * 1024
ARCHIVE_LIMIT = 64 * 1024 * 1024
PROJECT_STORE = ".toolchains/r2bulk/packages"
MATERIALIZATION_RECEIPT = ".r2bulk-materialization.json"


def read_lock(path):
    lock = json.loads(path.read_text())
    if lock.get("schema") != 1 or lock.get("package") != PACKAGE_NAME:
        raise ValueError("Select the explicit bulk workflow release lock")
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", lock.get("version", "")):
        raise ValueError("Bulk workflow requires an exact released version")
    for field in ("manifest_sha256", "archive_sha256", "requirements_lock_sha256"):
        if not re.fullmatch(r"[0-9a-f]{64}", lock.get(field, "")):
            raise ValueError("Bulk workflow requires an exact " + field)
    url = urlsplit(lock.get("manifest_url", ""))
    if (url.scheme != "https" or url.hostname != "github.com" or url.query or url.fragment
            or url.username or url.password or url.port not in (None, 443)
            or not url.path.startswith("/Dockpipe-Industries/dockpipe-packages/releases/download/")
            or not re.fullmatch(r"/Dockpipe-Industries/dockpipe-packages/releases/download/[^/]+/packages-store-manifest\.json", url.path)
            or url.path.endswith("/latest/packages-store-manifest.json")):
        raise ValueError("Bulk workflow requires an immutable official release manifest URL")
    return lock


def public_url(url):
    parsed = urlsplit(url)
    if (parsed.scheme != "https" or parsed.hostname not in DOWNLOAD_HOSTS
            or parsed.username or parsed.password or parsed.port not in (None, 443) or parsed.fragment):
        raise ValueError("Public release download must use an allowed HTTPS host")


class PublicReleaseRedirects(HTTPRedirectHandler):
    def __init__(self):
        super().__init__()
        self.redirects = 0

    def redirect_request(self, request, response, code, message, headers, new_url):
        self.redirects += 1
        if self.redirects > 3:
            raise ValueError("Public release download exceeds three redirects")
        public_url(new_url)
        return Request(new_url, headers={"User-Agent": "DockPipe-r2bulk-bootstrap"})


def fetch_public(url, limit):
    public_url(url)
    # Explicit public transport: no environment proxies, cookies or auth handlers.
    opener = build_opener(ProxyHandler({}), PublicReleaseRedirects())
    request = Request(url, headers={"User-Agent": "DockPipe-r2bulk-bootstrap"})
    with opener.open(request, timeout=30) as response:
        public_url(response.geturl())
        payload = response.read(limit + 1)
    if len(payload) > limit:
        raise ValueError("Public release download exceeds its byte limit")
    return payload


def release_archive(lock, runtime_root):
    manifest_bytes = fetch_public(lock["manifest_url"], MANIFEST_LIMIT)
    if hashlib.sha256(manifest_bytes).hexdigest() != lock["manifest_sha256"]:
        raise ValueError("Public release manifest differs from the release lock")
    manifest = json.loads(manifest_bytes)
    packages = manifest.get("packages", {})
    workflows = packages.get("workflows", [])
    if (manifest.get("schema") != 1 or len(workflows) != 1
            or any(value for key, value in packages.items() if key != "workflows")):
        raise ValueError("Release manifest must contain exactly the locked bulk workflow")
    entry = workflows[0]
    if (entry.get("name") != PACKAGE_NAME or entry.get("version") != lock["version"]
            or entry.get("sha256") != lock["archive_sha256"]):
        raise ValueError("Release manifest workflow differs from the release lock")
    basename = entry.get("tarball", "")
    expected = "dockpipe-workflow-" + PACKAGE_NAME + "-" + lock["version"] + ".tar.gz"
    if basename != expected:
        raise ValueError("Release manifest archive must use its exact published basename")
    store = runtime_root / "package-store" / "workflows"
    store.mkdir(parents=True, exist_ok=True)
    archive = store / basename
    if archive.exists() or archive.is_symlink():
        if archive.is_symlink() or not archive.is_file():
            raise ValueError("Package-store archive must be a regular cached file")
        package_files(archive, lock["archive_sha256"])
        return archive
    archive_url = lock["manifest_url"].rsplit("/", 1)[0] + "/" + basename
    payload = fetch_public(archive_url, ARCHIVE_LIMIT)
    if hashlib.sha256(payload).hexdigest() != lock["archive_sha256"]:
        raise ValueError("Public release archive differs from the release lock")
    # Never replace an existing store object, including after a concurrent bootstrap.
    with archive.open("xb") as destination:
        destination.write(payload)
    return archive


def package_files(package, archive_sha256):
    files = {}
    if package.is_dir():
        for name, limit in PACKAGE_FILES.items():
            path = package / name
            if path.is_symlink() or not path.is_file() or path.stat().st_size > limit:
                raise ValueError("Invalid installed package member: " + name)
            files[name] = path.read_bytes()
        return files
    digest = hashlib.sha256()
    with package.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    if digest.hexdigest() != archive_sha256:
        raise ValueError("Installed bulk workflow archive differs from the release lock")
    prefix = "workflows/" + PACKAGE_NAME + "/"
    with tarfile.open(package, "r:gz") as archive:
        for member in archive:
            name = member.name.removeprefix(prefix)
            if member.name != prefix + name or name not in PACKAGE_FILES:
                continue
            if name in files or not member.isfile() or member.size > PACKAGE_FILES[name]:
                raise ValueError("Invalid installed package archive member: " + member.name)
            source = archive.extractfile(member)
            if source is None:
                raise ValueError("Missing installed package archive member: " + member.name)
            with source:
                files[name] = source.read(PACKAGE_FILES[name] + 1)
            if len(files[name]) != member.size:
                raise ValueError("Invalid installed package archive member size: " + member.name)
    if files.keys() != PACKAGE_FILES.keys():
        raise ValueError("Installed bulk workflow archive is missing its descriptor or SDK lock")
    return files


def declared_store(project_root):
    configuration = json.loads((project_root / "dockpipe.config.json").read_text())
    sources = configuration.get("packages", {}).get("sources", [])
    if not any(source.get("kind") == "store" and source.get("path") == PROJECT_STORE
               for source in sources if isinstance(source, dict)):
        raise ValueError("Declare packages.sources store path " + PROJECT_STORE + " before running this workflow")
    store = project_root / PROJECT_STORE
    for path in (store, *store.parents):
        if path == project_root.parent:
            break
        if path.is_symlink():
            raise ValueError("Declared package store must not contain symlink parents")
    return store


def materialize_archive(archive_path, store, archive_sha256):
    prefix = "workflows/" + PACKAGE_NAME
    entries = {}
    directories = set()
    total = 0
    with tarfile.open(archive_path, "r:gz") as archive:
        for index, member in enumerate(archive):
            if index >= 4096:
                raise ValueError("Compiled package exceeds the member limit")
            name = member.name.rstrip("/")
            parts = name.split("/")
            if (name != prefix and not name.startswith(prefix + "/")
                    or any(part in ("", ".", "..") for part in parts) or "\\" in name
                    or not (member.isfile() or member.isdir())):
                raise ValueError("Invalid compiled package archive member: " + member.name)
            relative = name[len(prefix):].lstrip("/")
            if relative == MATERIALIZATION_RECEIPT or relative in entries or relative in directories:
                raise ValueError("Duplicate or reserved compiled package archive member")
            if member.isdir():
                directories.add(relative)
                continue
            if not relative or member.size < 0 or member.size > 16 * 1024 * 1024:
                raise ValueError("Compiled package exceeds its file size limit")
            total += member.size
            if total > ARCHIVE_LIMIT:
                raise ValueError("Compiled package exceeds its expanded size limit")
            source = archive.extractfile(member)
            if source is None:
                raise ValueError("Missing compiled package archive file")
            with source:
                content = source.read(member.size + 1)
            if len(content) != member.size:
                raise ValueError("Compiled package archive member size mismatch")
            entries[relative] = (content, member.mode & 0o777)
    if any(name not in entries for name in ("config.yml", "package.yml", "requirements.lock")):
        raise ValueError("Compiled package archive is missing its workflow or release metadata")
    receipt = {"schema": 1, "archive_sha256": archive_sha256,
               "files": {name: hashlib.sha256(content).hexdigest()
                         for name, (content, mode) in sorted(entries.items())}}
    workflows = store / "workflows"
    if workflows.is_symlink():
        raise ValueError("Compiled package store workflows path must not be a symlink")
    workflows.mkdir(parents=True, exist_ok=True)
    destination = workflows / PACKAGE_NAME
    if destination.exists() or destination.is_symlink():
        if destination.is_symlink() or not destination.is_dir():
            raise ValueError("Conflicting materialized package cache")
        receipt_path = destination / MATERIALIZATION_RECEIPT
        if receipt_path.is_symlink() or not receipt_path.is_file() or json.loads(receipt_path.read_text()) != receipt:
            raise ValueError("Conflicting materialized package receipt")
        actual = {}
        for path in destination.rglob("*"):
            if path.is_symlink():
                raise ValueError("Conflicting materialized package symlink")
            if path.is_file() and path != receipt_path:
                relative = path.relative_to(destination).as_posix()
                if relative not in entries or path.stat().st_size != len(entries[relative][0]):
                    raise ValueError("Conflicting materialized package file inventory")
                actual[relative] = hashlib.sha256(path.read_bytes()).hexdigest()
            elif not path.is_dir() and path != receipt_path:
                raise ValueError("Conflicting materialized package file type")
        if actual != receipt["files"]:
            raise ValueError("Conflicting materialized package file contents")
        return destination
    temporary = Path(tempfile.mkdtemp(prefix=".r2bulk-materialize-", dir=workflows))
    try:
        for relative in sorted(directories):
            (temporary / relative).mkdir(parents=True, exist_ok=True)
        for relative, (content, mode) in entries.items():
            target = temporary / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(content)
            target.chmod(mode)
        (temporary / MATERIALIZATION_RECEIPT).write_text(json.dumps(receipt, indent=2) + "\n")
        # A completed package becomes visible together. Existing nonempty stores
        # cannot be replaced by this directory rename.
        if destination.exists() or destination.is_symlink():
            raise ValueError("Materialized package cache appeared during preparation; retry validation")
        temporary.rename(destination)
    finally:
        if temporary.exists():
            shutil.rmtree(temporary)
    return destination


def install(lock_path, runtime_root, binary, project_root=None):
    lock = read_lock(lock_path)
    store = declared_store(project_root if project_root is not None else Path.cwd())
    package = release_archive(lock, runtime_root)
    files = package_files(package, lock["archive_sha256"])
    descriptor = files["package.yml"].decode("utf-8")
    if not re.search(r"(?m)^version:\s*" + re.escape(lock["version"]) + r"\s*$", descriptor):
        raise ValueError("Installed bulk workflow version differs from the release lock")
    if hashlib.sha256(files["requirements.lock"]).hexdigest() != lock["requirements_lock_sha256"]:
        raise ValueError("Bulk SDK requirements differ from the release lock")
    if package.is_file():
        materialize_archive(package, store, lock["archive_sha256"])
    runtime = runtime_root / lock["requirements_lock_sha256"]
    python = runtime / ("Scripts/python.exe" if os.name == "nt" else "bin/python")
    receipt = runtime / "qualified.json"
    runtime.mkdir(parents=True, exist_ok=True)
    requirements = runtime / "requirements.lock"
    requirements.write_bytes(files["requirements.lock"])
    if not receipt.is_file():
        if importlib.util.find_spec("venv") is None or importlib.util.find_spec("ensurepip") is None:
            raise RuntimeError("Bootstrap requires Python with working venv and ensurepip support; select that interpreter explicitly")
        # A failed prior setup is repaired by the ordinary venv/pip operations.
        try:
            subprocess.run([sys.executable, "-m", "venv", str(runtime)], check=True)
        except subprocess.CalledProcessError as error:
            raise RuntimeError("Bootstrap requires Python with working venv and ensurepip support; select that interpreter explicitly") from error
        subprocess.run([str(python), "-m", "pip", "install", "--require-hashes", "--no-deps",
                        "-r", str(requirements)], check=True)
        subprocess.run([str(python), "-m", "pip", "check"], check=True)
        receipt.write_text(json.dumps({"requirements_lock_sha256": lock["requirements_lock_sha256"]}) + "\n")
    else:
        if json.loads(receipt.read_text()).get("requirements_lock_sha256") != lock["requirements_lock_sha256"]:
            raise ValueError("Bulk SDK runtime receipt differs from the release lock")
        subprocess.run([str(python), "-m", "pip", "check"], check=True)
    subprocess.run([str(python), str(Path(__file__).with_name("verify-sdk.py")), str(requirements)], check=True)
    if not python.is_file():
        raise ValueError("Bulk SDK virtual environment interpreter is missing")
    return python.absolute()


def main():
    binary = Path(os.environ["DOCKPIPE_BIN"])
    if not binary.is_absolute() or not binary.is_file():
        raise ValueError("DOCKPIPE_BIN must select the released absolute CLI")
    runtime_root = Path(subprocess.check_output(
        [str(binary), "scope", "artifacts", "r2bulk-runtime"], text=True).strip())
    python = install(Path("release/r2bulk.lock.json"), runtime_root, binary)
    outputs = os.environ.get("DOCKPIPE_STEP_OUTPUTS_FILE")
    if not outputs:
        outputs = subprocess.check_output([str(binary), "scope", "artifacts", "bootstrap.env"], text=True).strip()
    environment = {"R2_BULK_PYTHON": str(python)}
    if any("\n" in value or "\r" in value for value in environment.values()):
        raise ValueError("Runtime paths cannot contain newlines")
    Path(outputs).write_text("".join(key + "=" + value + "\n" for key, value in environment.items()))


if __name__ == "__main__":
    main()
