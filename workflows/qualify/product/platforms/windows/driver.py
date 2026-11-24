"""Native Windows installed CLI qualification through the public product CLI."""

import argparse
import ctypes
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import struct
import subprocess
import time
import zipfile


ASSERTIONS = (
    "installed-cli-version",
    "initialized-project",
    "host-workflow-executed",
)
MAX_LOG_BYTES = 1 << 20
MAX_INSTALL_LOG_BYTES = 16 << 20
MAX_INSTALL_RECEIPT_BYTES = 64 << 10
MAX_CLI_BYTES = 256 << 20
PROFILE_MACHINES = {
    "windows-amd64-msi": 0x8664,
    "windows-11-25h2-arm64": 0xAA64,
    "windows-server-2025-amd64": 0x8664,
}
RUN_BINDINGS = ("run_id", "plan_sha256", "registry_sha256", "target")
MSI_RECEIPT_SCHEMA = "dockpipe.product-integration.windows-msi-install/v1"


def digest(path):
    checksum = hashlib.sha256()
    with Path(path).open("rb") as stream:
        for block in iter(lambda: stream.read(4 << 20), b""):
            checksum.update(block)
    return checksum.hexdigest()


def absolute_file(value, name):
    if not isinstance(value, str) or not value:
        raise ValueError(f"{name} must select an existing absolute file")
    path = Path(value)
    if not path.is_absolute() or not path.is_file() or path.is_symlink():
        raise ValueError(f"{name} must select an existing absolute file")
    return path


def validate_request(request):
    if request.get("schema") != "dockpipe.product-integration.request/v1":
        raise ValueError("Unsupported request schema")
    for field in ("plan_sha256", "registry_sha256"):
        if not re.fullmatch("[0-9a-f]{64}", request.get(field, "")):
            raise ValueError(f"Invalid {field}")
    target = request["target"]
    if target.get("platform") not in PROFILE_MACHINES:
        raise ValueError("Windows driver requires a registered Windows target with a known architecture")
    if not re.fullmatch("[0-9a-f]{64}", target.get("artifact_sha256", "")):
        raise ValueError("Invalid target artifact digest")
    configuration = request["configuration"]
    allowed = {"cli", "cli_sha256", "artifact", "install_receipt", "timeout_seconds"}
    if set(configuration) - allowed:
        raise ValueError("Unknown Windows driver configuration keys")
    for name in ("cli", "artifact"):
        if not isinstance(configuration.get(name), str) or not configuration[name]:
            raise ValueError(f"configuration.{name} is required")
    if not re.fullmatch("[0-9a-f]{64}", configuration.get("cli_sha256", "")):
        raise ValueError("configuration.cli_sha256 must pin the installed executable")
    timeout = configuration.get("timeout_seconds", 60)
    if isinstance(timeout, bool) or not isinstance(timeout, int) or not 5 <= timeout <= 180:
        raise ValueError("timeout_seconds must be an integer from 5 to 180")
    for case in request["cases"]:
        if not isinstance(case.get("required_assertions"), list):
            raise ValueError("Each case must declare required_assertions")
    return configuration


def native_windows_machine():
    # This returns the OS machine type even under an emulated Python process.
    kernel = ctypes.WinDLL("kernel32", use_last_error=True)
    kernel.GetCurrentProcess.restype = ctypes.c_void_p
    query = kernel.IsWow64Process2
    query.argtypes = (ctypes.c_void_p, ctypes.POINTER(ctypes.c_ushort), ctypes.POINTER(ctypes.c_ushort))
    query.restype = ctypes.c_int
    process_machine = ctypes.c_ushort()
    native_machine = ctypes.c_ushort()
    if not query(kernel.GetCurrentProcess(), ctypes.byref(process_machine), ctypes.byref(native_machine)):
        raise ctypes.WinError(ctypes.get_last_error())
    return native_machine.value


def executable_machine(path):
    with path.open("rb") as stream:
        dos_header = stream.read(64)
        if len(dos_header) != 64 or dos_header[:2] != b"MZ":
            raise ValueError("Installed CLI must be a Windows PE executable")
        offset = struct.unpack_from("<I", dos_header, 60)[0]
        if not 64 <= offset <= path.stat().st_size - 6:
            raise ValueError("Installed CLI has an invalid PE header offset")
        stream.seek(offset)
        header = stream.read(6)
        if len(header) != 6:
            raise ValueError("Installed CLI PE header was truncated")
        signature, machine = struct.unpack("<4sH", header)
        if signature != b"PE\0\0":
            raise ValueError("Installed CLI has an invalid PE signature")
        return machine


def bounded_evidence(path, maximum):
    with path.open("rb") as stream:
        data = stream.read(maximum + 1)
    if not data or len(data) > maximum:
        raise ValueError(f"{path.name} is empty or exceeds its evidence size limit")
    return data


def msi_install_evidence(configuration, request, artifact, cli):
    receipt_path = absolute_file(configuration.get("install_receipt"), "MSI installation receipt")
    receipt_data = bounded_evidence(receipt_path, MAX_INSTALL_RECEIPT_BYTES)
    receipt = json.loads(receipt_data.decode("utf-8-sig"))
    if not isinstance(receipt, dict) or receipt.get("schema") != MSI_RECEIPT_SCHEMA:
        raise ValueError("Invalid Windows MSI installation receipt")
    if any(receipt.get(key) != request[key] for key in RUN_BINDINGS):
        raise ValueError("MSI installation receipt belongs to another qualification request")
    if (receipt.get("msi_sha256") != request["target"]["artifact_sha256"]
            or receipt.get("installed_cli_sha256") != configuration["cli_sha256"]
            or not isinstance(receipt.get("artifact"), str) or Path(receipt["artifact"]) != artifact
            or not isinstance(receipt.get("cli"), str) or Path(receipt["cli"]) != cli
            or type(receipt.get("installer_exit_code")) is not int
            or receipt["installer_exit_code"] not in (0, 3010)):
        raise ValueError("MSI installation receipt does not bind the selected installer to the installed CLI")

    log = absolute_file(str(receipt_path.with_name("msi-install.log")), "MSI installation log")
    log_data = bounded_evidence(log, MAX_INSTALL_LOG_BYTES)
    if hashlib.sha256(log_data).hexdigest() != receipt.get("installer_log_sha256"):
        raise ValueError("MSI installation log digest differs from its receipt")
    return {"msi-install.json": receipt_data, "msi-install.log": log_data}


def verify_portable_cli(artifact, cli, expected_digest):
    with zipfile.ZipFile(artifact) as archive:
        matches = [entry for entry in archive.infolist() if entry.filename.casefold() == "dockpipe.exe"]
        if len(matches) != 1 or matches[0].file_size != cli.stat().st_size:
            raise ValueError("Selected ZIP must contain one root dockpipe.exe matching the installed CLI size")

        checksum = hashlib.sha256()
        with archive.open(matches[0]) as source:
            for block in iter(lambda: source.read(1 << 20), b""):
                checksum.update(block)
        if checksum.hexdigest() != expected_digest:
            raise ValueError("Installed CLI digest differs from the executable in the selected ZIP")


def verify_delivery(configuration, request, artifact, cli):
    if artifact.suffix.lower() == ".msi":
        return msi_install_evidence(configuration, request, artifact, cli)
    if artifact.suffix.lower() != ".zip" or configuration.get("install_receipt") is not None:
        raise ValueError("Select an MSI with its installation receipt or a portable ZIP")
    if request["target"]["platform"] == "windows-amd64-msi":
        raise ValueError("The windows-amd64-msi profile requires an MSI installation")

    verify_portable_cli(artifact, cli, configuration["cli_sha256"])
    return {}


def stop_owned_process(process):
    if process.poll() is not None:
        return
    subprocess.run(
        ["taskkill.exe", "/PID", str(process.pid), "/T", "/F"],
        stdin=subprocess.DEVNULL,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        timeout=10,
        check=False,
    )
    process.wait(timeout=10)


def run_command(arguments, directory, environment, output, name, timeout):
    stdout_path = output / f"{name}.stdout.txt"
    stderr_path = output / f"{name}.stderr.txt"
    started = time.monotonic()
    reason = "completed"
    with stdout_path.open("xb") as stdout, stderr_path.open("xb") as stderr:
        process = subprocess.Popen(
            arguments,
            cwd=directory,
            env=environment,
            stdin=subprocess.DEVNULL,
            stdout=stdout,
            stderr=stderr,
        )
        try:
            while process.poll() is None:
                if time.monotonic() - started > timeout:
                    reason = "timeout"
                    break
                if stdout_path.stat().st_size + stderr_path.stat().st_size > MAX_LOG_BYTES:
                    reason = "output-limit"
                    break
                time.sleep(0.05)
        finally:
            stop_owned_process(process)
    if stdout_path.stat().st_size + stderr_path.stat().st_size > MAX_LOG_BYTES:
        reason = "output-limit"
    record = {
        "arguments": arguments,
        "exit_code": process.returncode,
        "reason": reason,
        "duration_seconds": round(time.monotonic() - started, 3),
    }
    (output / f"{name}.json").write_text(json.dumps(record, indent=2) + "\n", encoding="utf-8")
    return record


def evidence(output, paths):
    return [{"path": str(path.relative_to(output)).replace("\\", "/"), "sha256": digest(path)}
            for path in paths]


def private_environment(output):
    environment = {key: value for key, value in os.environ.items()
                   if not key.upper().startswith("DOCKPIPE_")}
    profile = output / "profile"
    for name in ("data", "config", "cache", "state", "runtime", "appdata", "localappdata", "temp"):
        (profile / name).mkdir(parents=True)
    for name in ("data", "config", "cache", "state"):
        environment[f"XDG_{name.upper()}_HOME"] = str(profile / name)
    environment.update(
        XDG_RUNTIME_DIR=str(profile / "runtime"),
        APPDATA=str(profile / "appdata"),
        LOCALAPPDATA=str(profile / "localappdata"),
        TEMP=str(profile / "temp"),
        TMP=str(profile / "temp"),
        DOCKPIPE_INTERACTIVE="0",
    )
    return environment


def write_host_fixture(workspace):
    workflow = workspace / "workflows/product-cli-proof"
    workflow.mkdir(parents=True)
    marker = "dockpipe-product-integration-windows-host-step-v1"
    (workflow / "config.yml").write_text(
        "name: product-cli-proof\nvault: none\ndocker_preflight: false\nsteps:\n"
        "  - id: native-host-proof\n    kind: host\n    shell: powershell\n"
        "    cwd: repo\n    run: proof.ps1\n", encoding="utf-8")
    (workflow / "proof.ps1").write_text(
        "$ErrorActionPreference = 'Stop'\n"
        f"[System.IO.File]::WriteAllText((Join-Path (Get-Location).Path 'host-result.txt'), '{marker}')\n"
        f"Write-Output '{marker}'\n", encoding="utf-8")
    return workflow, marker


def qualify_cli(configuration, request, output):
    target = request["target"]
    cli = absolute_file(configuration["cli"], "cli")
    artifact = absolute_file(configuration["artifact"], "artifact")
    if cli.stat().st_size > MAX_CLI_BYTES or digest(cli) != configuration["cli_sha256"]:
        raise ValueError("Selected installed CLI digest differs from configuration")
    if digest(artifact) != target["artifact_sha256"]:
        raise ValueError("Selected product artifact digest differs from target")
    expected_machine = PROFILE_MACHINES[target["platform"]]
    native_machine = native_windows_machine()
    if native_machine != expected_machine:
        raise ValueError("Native Windows architecture differs from the selected platform profile")
    cli_machine = executable_machine(cli)
    if cli_machine != expected_machine:
        raise ValueError("Installed CLI PE architecture differs from the selected platform profile")
    installation = verify_delivery(configuration, request, artifact, cli)
    for name, data in installation.items():
        with (output / name).open("xb") as stream:
            stream.write(data)

    workspace = output / "workspace"
    workspace.mkdir()
    environment = private_environment(output)
    timeout = configuration.get("timeout_seconds", 60)
    inventory = run_command(
        ["powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File",
         str(Path(__file__).with_name("inventory.ps1")), "-Dockpipe", str(cli),
         "-Output", str(output / "identity")],
        workspace, environment, output, "inventory", timeout,
    )
    if inventory["exit_code"] != 0 or inventory["reason"] != "completed":
        raise RuntimeError("Native Windows identity and selected CLI admission failed")
    inventory_path = output / "identity/windows-inventory.json"
    facts = json.loads(inventory_path.read_text(encoding="utf-8-sig"))
    if facts["dockpipe"]["sha256"] != configuration["cli_sha256"]:
        raise ValueError("Observed CLI digest differs from the admitted executable")
    facts["native_machine"] = f"0x{native_machine:04x}"
    facts["dockpipe"]["pe_machine"] = f"0x{cli_machine:04x}"
    initialization = run_command([str(cli), "init"], workspace, environment, output, "init", timeout)
    initialized = (initialization["exit_code"] == 0 and initialization["reason"] == "completed"
                   and (workspace / "dockpipe.config.json").is_file())
    initialization_detail = "Public init exited successfully and generated project metadata."
    if not initialized:
        initialization_detail = "Public init failed or project metadata is absent; inspect init.json."
    assertion_results = {
        "installed-cli-version": (True, "Pinned native Windows executable returned --version successfully."),
        "initialized-project": (initialized, initialization_detail),
    }
    if initialized:
        workflow, marker = write_host_fixture(workspace)
        execution = run_command(
            [str(cli), "--workflow", "product-cli-proof"],
            workspace, environment, output, "host-workflow", timeout,
        )
        marker_file = workspace / "host-result.txt"
        executed = (execution["exit_code"] == 0 and execution["reason"] == "completed"
                    and marker_file.is_file() and marker_file.read_text(encoding="utf-8-sig") == marker
                    and marker in (output / "host-workflow.stdout.txt").read_text(encoding="utf-8", errors="replace"))
    else:
        executed = False
    execution_detail = "Public workflow ran a native PowerShell host step and produced the exact file/stdout marker."
    if not executed:
        execution_detail = "Workflow did not produce a successful exact file/stdout marker observation."
    assertion_results["host-workflow-executed"] = (executed, execution_detail)
    paths = [path for path in output.iterdir() if path.is_file()]
    paths.append(inventory_path)
    if initialized:
        paths.extend([workspace / "dockpipe.config.json", workflow / "config.yml", workflow / "proof.ps1"])
        if marker_file.is_file():
            paths.append(marker_file)
    return facts, assertion_results, evidence(output, sorted(paths))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--request", required=True)
    parser.add_argument("--output", required=True)
    arguments = parser.parse_args()
    request_path = absolute_file(arguments.request, "request")
    output = Path(arguments.output)
    if not output.is_absolute() or output.exists():
        parser.error("--output must be an absolute new invocation-owned directory")
    request = json.loads(request_path.read_text(encoding="utf-8-sig"))
    configuration = validate_request(request)
    output.mkdir(parents=True)
    result = {key: request[key] for key in ("run_id", "plan_sha256", "registry_sha256", "target")}
    result.update(schema="dockpipe.product-integration.lane/v1", cases=[],
                  observed={"driver_platform": platform.system(), "driver_architecture": platform.machine()})
    admitted = None
    admission_error = None
    for case in request["cases"]:
        row = {key: case[key] for key in ("id", "scenario", "layer")}
        row.update(status="blocked", reason="No native Windows implementation for this requested case.",
                   assertions=[], evidence=[])
        if os.name != "nt":
            row["reason"] = "Native Windows execution required; this host cannot qualify a Windows case."
        elif case["id"] == "install.cli" and case["scenario"] == "happy" and case["layer"] == "installed_e2e":
            if set(case["required_assertions"]) != set(ASSERTIONS):
                row["reason"] = "Requested CLI assertion set differs from the implemented contract."
            else:
                if admitted is None and admission_error is None:
                    try:
                        admitted = qualify_cli(configuration, request, output)
                    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError, zipfile.BadZipFile) as error:
                        admission_error = str(error)
                if admitted is None:
                    row["reason"] = admission_error
                else:
                    facts, observations, proof = admitted
                    result["observed"]["windows"] = facts
                    row["assertions"] = [
                        {"id": identifier, "status": "passed" if observations[identifier][0] else "failed",
                         "detail": observations[identifier][1]}
                        for identifier in case["required_assertions"]]
                    row["evidence"] = proof
                    row["status"] = "passed" if all(value[0] for value in observations.values()) else "failed"
                    row["reason"] = "Native Windows installed CLI observations completed."
        result["cases"].append(row)
    (output / "result.json").write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
