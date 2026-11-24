"""Exercise the installed CLI's pinned PipeLang SDK with generated typed fixtures."""
from pathlib import Path
import argparse
import os
import platform
import shutil
import sys


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "assets/scripts"))
from contracts import digest_file, read_json, require, write_json
from portable import command, inventory


def module_binding_observed(text, module, version, checksum):
    rows = [line.split() for line in text.splitlines()]
    selected = [index for index, fields in enumerate(rows) if len(fields) >= 2
                and fields[0] == "dep" and fields[1] == module]
    if len(selected) != 1:
        return False
    index = selected[0]
    return (rows[index] == ["dep", module, version, checksum]
            and not (index + 1 < len(rows) and rows[index + 1] and rows[index + 1][0] == "=>"))


def qualify(cli, workspace, environment, output, configuration, artifact_sha256,
            execute=command, inspect=command, inspection_artifact=None):
    """Run one SDK feature contract through explicit language and inspection callbacks."""
    go = Path(configuration["go"])
    inspection_artifact = Path(inspection_artifact) if inspection_artifact is not None else Path(cli)
    require(digest_file(inspection_artifact) == artifact_sha256, "Inspected CLI differs from its artifact pin")
    require(digest_file(go) == configuration["go_sha256"], "Go inspector differs from its pin")
    (workspace / "dockpipe.config.json").write_text('{"compile":{"workflows":["models"]}}\n')
    models = workspace / "models"
    models.mkdir()
    source = models / "config.pipe"
    source.write_text(
        "public Class FixtureModel\n{\n"
        "    public string Name = \"dockpipe-sdk-fixture\";\n"
        "    public int Count = 7;\n"
        "    public int Scale(int factor) => Count * factor;\n"
        "    private bool Hidden() => true;\n}\n")
    source_sha256 = digest_file(source)
    operations = {}

    def invoke(name, arguments, tool=cli):
        runner = inspect if name == "build-info" else execute
        operations[name] = runner(tool, arguments, workspace, environment, output, name)
        return operations[name]

    invoke("build-info", ["version", "-m", str(inspection_artifact)], go)
    build_info = (output / "build-info/driver.stdout").read_text()
    pinned = operations["build-info"] and module_binding_observed(build_info, configuration["module"],
                                                                 configuration["module_version"], configuration["module_sum"])
    invoke("check-valid", ["pipelang", "check", "--in", str(source), "--format", "json"])
    valid = read_json(output / "check-valid/driver.stdout") if operations["check-valid"] else {}
    valid = valid.get("schema") == 1 and valid.get("diagnostics") == []
    outputs = []
    for name in ("compile-first", "compile-repeat"):
        destination = output / (name + "-artifacts")
        invoke(name, ["pipelang", "compile", "--in", str(source), "--entry", "FixtureModel", "--out", str(destination)])
        outputs.append(destination)
    deterministic = all(operations[name] for name in ("compile-first", "compile-repeat"))
    deterministic = deterministic and inventory(outputs[0]) == inventory(outputs[1]) and bool(inventory(outputs[0]))
    invoke("invoke-public", ["pipelang", "invoke", "--in", str(source), "--class", "FixtureModel",
                             "--method", "Scale", "--arg", "6", "--format", "json"])
    value = read_json(output / "invoke-public/driver.stdout") if operations["invoke-public"] else {}
    invoked = value.get("class") == "FixtureModel" and value.get("method") == "Scale"
    invoked = invoked and value.get("type") == "int" and type(value.get("value")) is int and value["value"] == 42
    invoke("materialize", ["pipelang", "materialize", "--workdir", str(workspace), "--force"])
    bindings = list(workspace.rglob("config.FixtureModel.bindings.json"))
    materialized = operations["materialize"] and len(bindings) == 1 and not (models / ".pipelang").exists()
    if materialized:
        fields = read_json(bindings[0]).get("fields", [])
        materialized = {field.get("name"): field.get("value") for field in fields} == {
            "Name": "dockpipe-sdk-fixture", "Count": 7}
    invoke("invoke-private", ["pipelang", "invoke", "--in", str(source), "--class", "FixtureModel", "--method", "Hidden"])
    private_process = read_json(output / "invoke-private/process.json")
    private_denied = type(private_process["exit_code"]) is int and private_process["exit_code"] == 1 and not private_process["failure"]
    private_denied = private_denied and "private" in (output / "invoke-private/driver.stderr").read_text().lower()
    invalid = output / "invalid-source"
    invalid.mkdir()
    broken = invalid / "broken.pipe"
    broken.write_text("Class Broken { int Value = ; }\n")
    before = inventory(workspace)
    invoke("check-invalid", ["pipelang", "check", "--in", str(broken), "--format", "json"])
    invalid_process = read_json(output / "check-invalid/process.json")
    rejected = type(invalid_process["exit_code"]) is int and invalid_process["exit_code"] == 1 and not invalid_process["failure"]
    diagnostics = read_json(output / "check-invalid/driver.stdout") if rejected else {}
    rejected = rejected and diagnostics.get("schema") == 1 and any(
        item.get("code") == "PL2001" and item.get("category") == "syntax" and item.get("severity") == "error"
        and item.get("primary", {}).get("file") == str(broken) for item in diagnostics.get("diagnostics", []))
    unchanged = before == inventory(workspace) and digest_file(source) == source_sha256
    unchanged = unchanged and set(path.name for path in invalid.iterdir()) == {"broken.pipe"}
    require(digest_file(go) == configuration["go_sha256"], "Go inspector changed during execution")
    observed = {"pinned-sdk-consumed": pinned,
                "typed-fixture-materialized": valid and deterministic and invoked and materialized,
                "invalid-input-rejected": rejected and private_denied and unchanged}
    facts = {"operations": operations, "module": configuration["module"], "module_version": configuration["module_version"],
             "module_sum": configuration["module_sum"], "pinned_module_observed": pinned, "inspected_executable_sha256": artifact_sha256,
             "deterministic_compilation": deterministic, "typed_method_exact_result": invoked,
             "materialized_fields_exact": materialized, "syntax_error_rejected": rejected,
             "private_method_rejected": private_denied, "source_and_materialized_output_unchanged_on_failure": unchanged}
    write_json(output / "observations.json", facts)
    paths = [output / "observations.json", source, broken]
    for directory in outputs:
        paths.extend(path for path in directory.rglob("*") if path.is_file())
    paths.extend(path for path in workspace.rglob("*") if path.is_file() and path != source)
    for name in operations:
        paths.extend(sorted((output / name).iterdir()))
    proof = [{"path": path.relative_to(output).as_posix(), "sha256": digest_file(path)} for path in paths]
    failure_observed = {"expected-failure-observed": rejected and private_denied,
                        "no-unowned-effects": unchanged, "actionable-result": rejected and private_denied}
    return observed, failure_observed, facts, proof


def case_assertions(case, observed, failure_observed):
    if case["id"] != "sdk.pipelang":
        return None
    if case["scenario"] == "happy" and case["layer"] == "installed_e2e":
        return observed
    if case["scenario"] == "failure" and case["layer"] == "component":
        return failure_observed
    return None


def run(request, output):
    target = request["target"]
    require(platform.system() == "Linux" and target["platform"] == "linux-portable" and target["kind"] == "local",
            "PipeLang driver currently qualifies an explicitly selected local Linux portable CLI")
    configuration = request["configuration"]
    require(set(configuration) == {"artifact", "go", "go_sha256", "module", "module_version", "module_sum"}
            and all(isinstance(value, str) and value for value in configuration.values()),
            "Select a pinned CLI, Go build-info inspector and exact SDK module identity")
    artifact = Path(configuration["artifact"])
    go = Path(configuration["go"])
    require(artifact.is_absolute() and digest_file(artifact) == target["artifact_sha256"], "Selected CLI differs from its pin")
    require(go.is_absolute() and digest_file(go) == configuration["go_sha256"], "Go inspector differs from its pin")
    output.mkdir(mode=0o700, parents=True)
    cli = output / "dockpipe"
    shutil.copyfile(artifact, cli)
    cli.chmod(0o700)
    require(digest_file(cli) == target["artifact_sha256"], "Copied PipeLang CLI differs from its pin")
    home = output / "home"
    home.mkdir(mode=0o700)
    environment = {name: os.environ[name] for name in ("PATH", "LANG", "LC_ALL", "TZ") if name in os.environ}
    environment.update(HOME=str(home), DOCKPIPE_VAULT_INJECT="0", DOCKPIPE_INTERACTIVE="0",
                       GOTOOLCHAIN="local", GOENV="off", GOPROXY="off")
    for name in ("config", "cache", "data", "state"):
        directory = home / name
        directory.mkdir(mode=0o700)
        environment["XDG_" + name.upper() + "_HOME"] = str(directory)
    workspace = output / "workspace"
    workspace.mkdir(mode=0o700)
    observed, failure_observed, facts, proof = qualify(
        cli, workspace, environment, output, configuration, target["artifact_sha256"])
    rows = []
    for case in request["cases"]:
        row = {name: case[name] for name in ("id", "scenario", "layer")}
        row.update(status="blocked", reason="PipeLang driver does not implement this case", assertions=[], evidence=[])
        assertions = case_assertions(case, observed, failure_observed)
        if assertions is not None:
            require(set(case["required_assertions"]) == set(assertions), "PipeLang assertion contract changed")
            row.update(status="passed" if all(assertions.values()) else "failed",
                       reason="Installed CLI SDK module pin, deterministic typed compile/materialize/invoke, structured syntax rejection and private-method refusal",
                       assertions=[{"id": name, "status": "passed" if assertions[name] else "failed",
                                    "detail": "See exact executable build info, compiler receipts and independent generated artifact readback"}
                                   for name in case["required_assertions"]], evidence=proof)
        rows.append(row)
    receipt = {name: request[name] for name in ("run_id", "plan_sha256", "registry_sha256", "target")}
    receipt.update(schema="dockpipe.product-integration.lane/v1", cases=rows, observed=facts)
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
