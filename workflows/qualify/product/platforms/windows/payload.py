"""Stage pinned, non-secret test payload through the VM package's bootstrap input."""

import hashlib
import json
from pathlib import Path, PureWindowsPath
import re
import shutil


def digest(path):
    checksum = hashlib.sha256()
    with Path(path).open("rb") as stream:
        for block in iter(lambda: stream.read(4 << 20), b""):
            checksum.update(block)
    return checksum.hexdigest()


def prepare(directory, request, artifacts, guest_output):
    directory = Path(directory)
    if not directory.is_absolute() or directory.exists():
        raise ValueError("Bootstrap payload requires a new absolute directory")
    if not PureWindowsPath(guest_output).is_absolute() or "'" in guest_output:
        raise ValueError("Guest output must be an explicit absolute Windows path without apostrophes")
    if request.get("schema") != "dockpipe.product-integration.request/v1":
        raise ValueError("Unsupported qualification request")
    if request["target"]["artifact_sha256"] != artifacts["msi"]["sha256"]:
        raise ValueError("Request must select the staged MSI artifact")
    sources = {}
    for name in ("msi", "python"):
        selection = artifacts[name]
        path = Path(selection["path"])
        if not path.is_absolute() or not path.is_file() or path.is_symlink():
            raise ValueError(f"{name} must select an absolute regular artifact")
        if not re.fullmatch("[a-f0-9]{64}", selection["sha256"]) or digest(path) != selection["sha256"]:
            raise ValueError(f"{name} artifact digest mismatch")
        sources[name] = path
    if not re.fullmatch("[a-f0-9]{64}", artifacts["cli_sha256"]):
        raise ValueError("Installed CLI digest must be pinned separately")
    directory.mkdir(mode=0o700, parents=True)
    shutil.copyfile(sources["msi"], directory / "dockpipe.msi")
    shutil.copyfile(sources["python"], directory / "python-embed.zip")
    for name in ("driver.py", "inventory.ps1", "prepare-product.ps1", "observe-gui.ps1"):
        shutil.copyfile(Path(__file__).with_name(name), directory / name)
    native_request = {key: request[key] for key in (
        "schema", "run_id", "plan_sha256", "registry_sha256", "target", "cases")}
    native_request["configuration"] = {}
    (directory / "request.json").write_text(json.dumps(native_request, indent=2) + "\n", encoding="utf-8")
    wrapper = (
        "$ErrorActionPreference = 'Stop'\n"
        "& (Join-Path $PSScriptRoot 'prepare-product.ps1') `\n"
        "    -Payload $PSScriptRoot -Request (Join-Path $PSScriptRoot 'request.json') `\n"
        f"    -Output '{guest_output}' -InstallInFreshGuest `\n"
        f"    -ExpectedMsiSha256 '{artifacts['msi']['sha256']}' `\n"
        f"    -ExpectedCliSha256 '{artifacts['cli_sha256']}' `\n"
        f"    -ExpectedPythonSha256 '{artifacts['python']['sha256']}'\n"
    )
    (directory / "Run-ProductQualification.ps1").write_text(wrapper, encoding="utf-8")
    files = [{"path": path.name, "sha256": digest(path)} for path in sorted(directory.iterdir())]
    receipt = {
        "schema": "dockpipe.product-integration.windows-bootstrap-payload/v1",
        "files": files,
        "guest_output": guest_output,
        "guest_execution_observed": False,
        "credential_material": False,
    }
    (directory / "payload.json").write_text(json.dumps(receipt, indent=2) + "\n", encoding="utf-8")
    for path in directory.iterdir():
        path.chmod(0o600)
    return receipt


def guest_command():
    return (
        "$helpers = @(Get-PSDrive -PSProvider FileSystem | ForEach-Object { "
        "$candidate = Join-Path $_.Root 'Run-ProductQualification.ps1'; "
        "if (Test-Path -LiteralPath $candidate) { $candidate } }); "
        "if ($helpers.Count -ne 1) { throw 'Expected one qualification payload on attached media' }; "
        "& $helpers[0]"
    )
