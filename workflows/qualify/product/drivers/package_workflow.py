"""Delegate qualification to an exact package-owned workflow archive."""
from pathlib import Path, PurePosixPath
import argparse
import copy
import os
import shutil
import sys


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "assets/scripts"))
from contracts import digest_file, read_json, require, validate_receipt, write_json
from process import run_driver


def workflow_arguments(member, arguments, request_sha256):
    require(isinstance(member, str) and member and not member.startswith("/")
            and "\\" not in member and "\x00" not in member
            and PurePosixPath(member).as_posix() == member
            and all(part not in (".", "..") for part in member.split("/"))
            and ":" not in member.split("/")[0], "Select a canonical contained workflow archive member")
    require(isinstance(arguments, list) and len(arguments) <= 16
            and all(isinstance(value, str) and len(value) <= 512 and "\x00" not in value for value in arguments),
            "Workflow argument list exceeds its bound")
    require(all("@request-sha256" not in value or value == "@request-sha256" for value in arguments),
            "Request digest substitution must occupy one complete argument")
    require(all(value not in ("--request", "--output", "--")
                and not value.startswith(("--request=", "--output=")) for value in arguments),
            "Provider arguments cannot replace the bound request or output")
    return [request_sha256 if value == "@request-sha256" else value for value in arguments]


def forwarded_cases(receipt, request, operation, process, proof):
    if process["exit_code"] != 0 or process["failure"]:
        return [dict({name: case[name] for name in ("id", "scenario", "layer")}, status="failed",
                     reason="Owning workflow did not complete normally; external resource cleanup remains unproven",
                     assertions=[], evidence=proof) for case in request["cases"]]
    validate_receipt(receipt, request, operation)
    cases = copy.deepcopy(receipt["cases"])
    for case in cases:
        evidence = case.get("evidence", [])
        require(len(evidence) <= 61, "Owning workflow must reserve three evidence slots for the invocation bindings")
        case["evidence"] = [dict(item, path="operation/" + item["path"]) for item in evidence] + proof
    return cases


def run(request, output):
    configuration = request["configuration"]
    require(set(configuration) == {"cli", "cli_sha256", "workflow_archive", "workflow_archive_sha256",
                                   "workflow_member", "provider_configuration", "workflow_arguments", "timeout_seconds"},
            "Select exact CLI/archive pins, workflow member, provider configuration, arguments and deadline")
    require(isinstance(configuration["provider_configuration"], dict), "Provider configuration must be an object")
    require(all(isinstance(configuration[name], str) for name in
                ("cli", "cli_sha256", "workflow_archive", "workflow_archive_sha256")),
            "CLI and archive selections must contain string paths and digests")
    deadline = configuration["timeout_seconds"]
    require(type(deadline) is int and 1 <= deadline <= 14400, "Owning workflow deadline exceeds four hours")
    workflow_arguments(configuration["workflow_member"], configuration["workflow_arguments"], "0" * 64)
    inputs = {}
    for name, field in (("cli", "cli"), ("archive", "workflow_archive")):
        path = Path(configuration[field])
        require(path.is_absolute() and digest_file(path) == configuration[field + "_sha256"],
                "Selected qualification " + name + " differs from its artifact pin")
        inputs[name] = path
    output.mkdir(mode=0o700, parents=True)
    installation = output / "artifacts"
    installation.mkdir(mode=0o700)
    (installation / "cli").mkdir(mode=0o700)
    (installation / "workflow").mkdir(mode=0o700)
    cli = installation / "cli" / inputs["cli"].name
    archive = installation / "workflow" / inputs["archive"].name
    for source, destination in ((inputs["cli"], cli), (inputs["archive"], archive)):
        shutil.copyfile(source, destination)
    cli.chmod(0o700)
    require(digest_file(cli) == configuration["cli_sha256"]
            and digest_file(archive) == configuration["workflow_archive_sha256"], "Copied workflow artifacts differ from their pins")
    provider_request = copy.deepcopy(request)
    provider_request["configuration"] = configuration["provider_configuration"]
    request_path = output / "provider-request.json"
    write_json(request_path, provider_request)
    request_sha256 = digest_file(request_path)
    arguments = workflow_arguments(configuration["workflow_member"], configuration["workflow_arguments"], request_sha256)
    environment = {name: os.environ[name] for name in
                   ("PATH", "LANG", "LC_ALL", "TZ", "SYSTEMROOT", "WINDIR", "COMSPEC") if name in os.environ}
    home = output / "home"
    home.mkdir(mode=0o700)
    environment.update(HOME=str(home), USERPROFILE=str(home), APPDATA=str(home / "config"),
                       LOCALAPPDATA=str(home / "data"), DOCKPIPE_VAULT_INJECT="0", DOCKPIPE_INTERACTIVE="0")
    for name in ("config", "cache", "data", "state"):
        directory = home / name
        directory.mkdir(mode=0o700)
        environment["XDG_" + name.upper() + "_HOME"] = str(directory)
    runtime = home / "runtime"
    runtime.mkdir(mode=0o700)
    environment.update(XDG_RUNTIME_DIR=str(runtime), TMPDIR=str(runtime), TMP=str(runtime), TEMP=str(runtime))
    workspace = output / "workspace"
    workspace.mkdir(mode=0o700)
    (workspace / "dockpipe.config.json").write_text("{}\n")
    logs = output / "workflow-process"
    logs.mkdir(mode=0o700)
    operation = output / "operation"
    locator = "tar://" + str(archive) + "##" + configuration["workflow_member"]
    process = run_driver([str(cli), "--workflow-file", locator, "--no-vault", "--",
                          "--request", str(request_path), "--output", str(operation), *arguments],
                         logs, deadline, environment, workspace)
    write_json(logs / "process.json", process)
    require(digest_file(cli) == configuration["cli_sha256"]
            and digest_file(archive) == configuration["workflow_archive_sha256"], "Qualification artifacts changed during execution")
    facts = {"cli_sha256": digest_file(cli), "workflow_archive_sha256": digest_file(archive),
             "workflow_member": configuration["workflow_member"], "provider_request_sha256": request_sha256,
             "provider_semantics_owned_by_workflow": True,
             "external_cleanup_requires_provider_evidence": True}
    write_json(output / "bridge.json", facts)
    receipt_path = operation / "result.json"
    owner_receipt = read_json(receipt_path) if process["exit_code"] == 0 and not process["failure"] else None
    proof_paths = [output / "bridge.json", request_path, logs / "process.json"]
    proof = [{"path": path.relative_to(output).as_posix(), "sha256": digest_file(path)} for path in proof_paths]
    cases = forwarded_cases(owner_receipt, provider_request, operation, process, proof)
    receipt = {name: request[name] for name in ("run_id", "plan_sha256", "registry_sha256", "target")}
    receipt.update(schema="dockpipe.product-integration.lane/v1", cases=cases, observed=facts)
    write_json(output / "result.json", receipt)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--request", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    arguments = parser.parse_args()
    require(arguments.output.is_absolute() and not arguments.output.exists(), "Select fresh absolute output")
    request = read_json(arguments.request)
    require(request.get("schema") == "dockpipe.product-integration.request/v1", "Unknown request schema")
    run(request, arguments.output)


if __name__ == "__main__":
    main()
