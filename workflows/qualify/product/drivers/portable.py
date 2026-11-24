"""Install and exercise an explicitly pinned Linux portable CLI in private fixtures."""
from pathlib import Path
import argparse
import os
import platform
import re
import shutil
import sys


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "assets/scripts"))
from contracts import digest_file, read_json, require, write_json
from process import run_driver
import workspace_files


VERSION = re.compile(r"(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)"
                     r"(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?"
                     r"|dev(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?")


def version_observed(value, expected=None):
    value = value.strip()
    return len(value) <= 128 and VERSION.fullmatch(value) is not None and (expected is None or value == expected)


def command(cli, arguments, workspace, environment, output, name):
    logs = output / name
    logs.mkdir(mode=0o700)
    result = run_driver([str(cli), *arguments], logs, 60, environment, workspace)
    write_json(logs / "process.json", result)
    return result["exit_code"] == 0 and not result["failure"]


def inventory(workspace):
    return {path.relative_to(workspace).as_posix(): digest_file(path)
            for path in sorted(workspace.rglob("*")) if path.is_file() and not path.is_symlink()}


def assertions(names, passed, detail):
    return [{"id": name, "status": "passed" if passed[name] else "failed", "detail": detail[name]}
            for name in names]


def qualify_source_cache(cli, workspace, environment, output, invoke=None):
    if invoke is None:
        invoke = command
    configurations = []
    configuration = ("name: shared-source-proof\nvault: none\ndocker_preflight: false\nsteps:\n"
                     "  - id: identity-proof\n    kind: host\n    cwd: repo\n    run: identity.sh\n")
    script = "set -eu\ncat \"$DOCKPIPE_SCRIPT_DIR/identity.txt\" > selected-source.txt\n"
    for name, identity in (("first-source", "first-source"), ("second-source", "second-source")):
        source = workspace / "workflows" / name
        source.mkdir()
        for filename, contents in (("config.yml", configuration), ("identity.sh", script), ("identity.txt", identity)):
            path = source / filename
            path.write_text(contents)
            os.utime(path, (946684800, 946684800))
            configurations.append(path)
        os.utime(source, (946684800, 946684800))
    marker = workspace / "selected-source.txt"
    first = invoke(cli, ["--workflow", "first-source"], workspace, environment, output, "cache-first")
    first = first and marker.is_file() and marker.read_text() == "first-source"
    marker.unlink(missing_ok=True)
    second = invoke(cli, ["--workflow", "second-source"], workspace, environment, output, "cache-second")
    second = second and marker.is_file() and marker.read_text() == "second-source"
    listed = invoke(cli, ["package", "list", "--format", "json"], workspace, environment, output, "cache-inventory")
    inventory = read_json(output / "cache-inventory/driver.stdout") if listed else {}
    archives = [Path(row["path"]) for row in inventory.get("packages", [])
                if row.get("name") == "shared-source-proof" and row.get("kind") == "workflow"
                and Path(row.get("path", "")).resolve().is_relative_to(workspace)]
    previous = archives[0].stat().st_mtime_ns if len(archives) == 1 else None
    marker.unlink(missing_ok=True)
    third = invoke(cli, ["--workflow", "second-source"], workspace, environment, output, "cache-repeat")
    third = third and marker.is_file() and marker.read_text() == "second-source"
    reused = previous is not None and archives[0].stat().st_mtime_ns == previous
    return first and second and third and reused, {
        "first_source_observed": first, "second_source_observed": second,
        "repeated_second_source_observed": third, "same_source_archive_reused": reused,
        "identical_configurations_with_different_assets": True}, configurations


def qualify_discovery(cli, workspace, environment, output):
    source = output / "package-source"
    source.mkdir()
    configuration = source / "config.yml"
    configuration.write_text(
        "name: packaged-proof\nvault: none\ndocker_preflight: false\n"
        "types:\n  - FixtureWorkflowConfig.pipe\n"
        "vars:\n  PRODUCT_FIXTURE: declared-default\n"
        "steps:\n  - id: archive-proof\n    kind: host\n    cwd: repo\n    run: archive-proof.sh\n")
    (source / "FixtureWorkflowConfig.pipe").write_text(
        "public Class FixtureWorkflowConfig\n{\n"
        "    [EnvName = \"PRODUCT_FIXTURE\"]\n"
        "    [DisplayName = \"Fixture Probe\"]\n"
        "    public string Probe = \"declared-default\";\n}\n")
    (source / "archive-proof.sh").write_text("set -eu\nprintf '%s' archive-script-executed > archive-result.txt\n")
    authored_ok = command(cli, ["catalog", "list", "--format", "json"], workspace, environment, output, "authored-catalog")
    authored = read_json(output / "authored-catalog/driver.stdout") if authored_ok else {}
    authored_found = any(row.get("workflow_id") == "product-proof" for row in authored.get("workflows", []))
    compiled_root = output / "compiled-store"
    compiled_environment = dict(environment, DOCKPIPE_PACKAGES_ROOT=str(compiled_root))
    compiled = command(cli, ["package", "compile", "workflow", "--from", str(source)],
                       workspace, compiled_environment, output, "compile")
    store = output / "exported-store"
    built = command(cli, ["package", "build", "store", "--out", str(store), "--only", "workflows"],
                    workspace, compiled_environment, output, "store")
    consumer = output / "installed-consumer"
    consumer.mkdir()
    (consumer / "dockpipe.config.json").write_text("{}\n")
    consumer_environment = compiled_environment
    listed = command(cli, ["catalog", "list", "--format", "json"], consumer,
                     consumer_environment, output, "installed-catalog")
    catalog = read_json(output / "installed-catalog/driver.stdout") if listed else {}
    selected = next((row for row in catalog.get("workflows", []) if row.get("workflow_id") == "packaged-proof"), {})
    locator = selected.get("config_path", "")
    installed_found = compiled and built and listed and locator.startswith("tar://")
    declared = selected.get("vars", {}).get("PRODUCT_FIXTURE") == "declared-default"
    declared = declared and any(row.get("env_name") == "PRODUCT_FIXTURE"
                                and row.get("default_value") == "declared-default"
                                and row.get("attributes", {}).get("displayname") == "Fixture Probe"
                                for row in selected.get("inputs", []))
    loaded = False
    if installed_found:
        # Exercise exactly the locator the GUI consumes; no workflow-name fallback.
        loaded = command(cli, ["--workflow-file", locator], consumer,
                         consumer_environment, output, "archive-execution")
        marker = consumer / "archive-result.txt"
        loaded = loaded and marker.is_file() and marker.read_text() == "archive-script-executed"
    isolated, cache_facts, cache_sources = qualify_source_cache(cli, workspace, environment, output)
    return {"authored-workflow-discovered": authored_found,
            "installed-workflow-discovered": installed_found and loaded and isolated,
            "declared-inputs-preserved": declared}, {
                "catalog_locator": locator, "compiled": compiled, "store_built": built,
                "exact_archive_locator_executed": loaded, "source_cache": cache_facts}, cache_sources


def run(request, output):
    require(platform.system() == "Linux" and request["target"]["platform"] == "linux-portable"
            and request["target"]["kind"] == "local", "This driver qualifies only a local Linux portable CLI")
    configuration = request["configuration"]
    require(set(configuration) <= {"artifact", "expected_version"} and "artifact" in configuration,
            "Portable configuration requires an artifact path and optional expected_version")
    expected_version = configuration.get("expected_version")
    require(expected_version is None or isinstance(expected_version, str) and version_observed(expected_version),
            "Expected portable version must be a valid version token")
    artifact = Path(configuration["artifact"])
    require(artifact.is_absolute() and digest_file(artifact) == request["target"]["artifact_sha256"],
            "Portable executable differs from its selected artifact pin")
    output.mkdir(mode=0o700, parents=True)
    installation = output / "install/bin"
    installation.mkdir(mode=0o700, parents=True)
    cli = installation / "dockpipe"
    shutil.copyfile(artifact, cli)
    cli.chmod(0o700)
    require(digest_file(cli) == request["target"]["artifact_sha256"], "Installed portable CLI identity changed")
    environment = {key: os.environ[key] for key in ("PATH", "LANG", "LC_ALL", "TZ") if key in os.environ}
    home = output / "home"
    home.mkdir(mode=0o700)
    environment.update(HOME=str(home), DOCKPIPE_INTERACTIVE="0")
    for name in ("data", "config", "cache", "state"):
        directory = home / name
        directory.mkdir(mode=0o700)
        environment["XDG_" + name.upper() + "_HOME"] = str(directory)
    workspace = output / "workspace with spaces"
    workspace.mkdir(mode=0o700)
    # Exercise the pre-existing empty folder regression as well as selected cwd.
    (workspace / "workflows").mkdir()
    version_ok = command(cli, ["--version"], workspace, environment, output, "version")
    version = (output / "version/driver.stdout").read_text().strip()
    version_ok = version_ok and version_observed(version, expected_version)
    init_ok = command(cli, ["init"], workspace, environment, output, "init")
    selected = init_ok and (workspace / "dockpipe.config.json").is_file()
    before = inventory(workspace)
    again_ok = command(cli, ["init"], workspace, environment, output, "init-again")
    idempotent = again_ok and before == inventory(workspace)
    home_unchanged = not any((home / name).exists() for name in ("dockpipe.config.json", "workflows", ".env.vault.template"))
    project_configuration = read_json(workspace / "dockpipe.config.json") if selected else {}
    compile_configuration = {}
    if isinstance(project_configuration, dict):
        compile_configuration = project_configuration.get("compile", {})
    workflow_roots = None
    if isinstance(compile_configuration, dict):
        workflow_roots = compile_configuration.get("workflows")
    folder = ((workspace / "workflows").is_dir() and (workspace / "workflows/example/config.yml").is_file()
              and isinstance(workflow_roots, list) and "workflows" in workflow_roots)
    workflow = workspace / "workflows/product-proof"
    workflow.mkdir()
    (workflow / "config.yml").write_text(
        "name: product-proof\nvault: none\ndocker_preflight: false\nsteps:\n"
        "  - id: host-proof\n    kind: host\n    cwd: repo\n"
        "    vars:\n      PRODUCT_FIXTURE: dockpipe-private-host-proof-v1\n    run: proof.sh\n")
    (workflow / "proof.sh").write_text(
        "set -eu\n"
        "test -f dockpipe.config.json\n"
        "test \"$PRODUCT_FIXTURE\" = dockpipe-private-host-proof-v1\n"
        "printf '%s' \"$PRODUCT_FIXTURE\" > host-result.txt\n"
        "printf '%s\\n' \"$PRODUCT_FIXTURE\"\n")
    executed = command(cli, ["--workflow", "product-proof"], workspace, environment, output, "host")
    marker = workspace / "host-result.txt"
    exact = marker.is_file() and marker.read_text() == "dockpipe-private-host-proof-v1"
    stdout = (output / "host/driver.stdout").read_text(errors="replace")
    output_ok = exact and "dockpipe-private-host-proof-v1" in stdout
    observed = {
        "install.cli": ({"installed-cli-version": version_ok, "initialized-project": selected,
                         "host-workflow-executed": executed and output_ok},
                        "Checks: private pinned portable installation, executable version, project init and real host workflow."),
        "init.scopes": ({"selected-cwd-initialized": selected, "home-unchanged": home_unchanged,
                         "empty-workflows-directory-created": folder, "idempotent-init": idempotent},
                         "Checks: public init twice with an initially empty workflows directory, mapped compile.workflows root and starter; misplaced HOME project/vault scaffold absent."),
        "workflow.host": ({"exit-status-propagated": executed, "fixture-output-exact": output_ok,
                           "cwd-env-output-binding": executed and exact},
                          "Checks: real host workflow script exit, repo cwd, declared environment and exact stdout/file markers."),
    }
    discovery_requested = any(case["id"] == "workflow.discovery" and case["scenario"] == "happy"
                              for case in request["cases"])
    discovery_facts = {}
    cache_sources = []
    if discovery_requested:
        discovery, discovery_facts, cache_sources = qualify_discovery(cli, workspace, environment, output)
        observed["workflow.discovery"] = (discovery,
            "Checks: authored catalog, compiled package catalog with retained input defaults/annotations, and execution of its exact tar locator.")
    file_cases = any(case["id"] == "files.workspace" and case["scenario"] == "happy" for case in request["cases"])
    file_facts = {}
    file_proof_paths = []
    if file_cases:
        file_assertions, file_facts, file_proof_paths = workspace_files.qualify(
            cli, workspace, environment, output, command)
        observed["files.workspace"] = (file_assertions,
            "Checks: real CLI exact-byte file lifecycle, stale revision rejection, traversal/link rejection and unchanged adjacent fixture.")
    facts = {"system": platform.system(), "machine": platform.machine(), "release": platform.release(),
             "version": version,
             "installed_executable_sha256": digest_file(cli), "initial_project_inventory": before,
             "idempotent": idempotent, "home_project_files_absent": home_unchanged,
             "implicit_vault_absent": not (workspace / ".env.vault.template").exists(),
             "discovery": discovery_facts, "files": file_facts}
    write_json(output / "observations.json", facts)
    proof_paths = [output / "observations.json", workflow / "config.yml", workflow / "proof.sh"]
    for name in ("version", "init", "init-again", "host"):
        proof_paths.extend(sorted((output / name).iterdir()))
    if discovery_requested:
        for name in ("authored-catalog", "compile", "store", "installed-catalog", "archive-execution",
                     "cache-first", "cache-second", "cache-inventory", "cache-repeat"):
            if (output / name).is_dir():
                proof_paths.extend(sorted((output / name).iterdir()))
        proof_paths.extend(sorted((output / "package-source").iterdir()))
        proof_paths.extend(cache_sources)
    if (workspace / "dockpipe.config.json").is_file():
        proof_paths.append(workspace / "dockpipe.config.json")
    if marker.is_file():
        proof_paths.append(marker)
    proof = [{"path": path.relative_to(output).as_posix(), "sha256": digest_file(path)}
             for path in proof_paths]
    rows = []
    for case in request["cases"]:
        row = {key: case[key] for key in ("id", "scenario", "layer")}
        row.update(status="blocked", assertions=[], evidence=[], reason="Portable driver does not implement this case")
        if case["scenario"] == "happy" and case["layer"] == "installed_e2e" and case["id"] in observed:
            passed, detail = observed[case["id"]]
            require(set(case["required_assertions"]) == set(passed), "Requested portable assertion contract changed")
            row.update(status="passed" if all(passed.values()) else "failed", reason=detail,
                       assertions=assertions(case["required_assertions"], passed,
                                             {name: detail for name in passed}), evidence=proof)
            if case["id"] == "files.workspace":
                row["evidence"] = [{"path": path.relative_to(output).as_posix(), "sha256": digest_file(path)}
                                   for path in [output / "observations.json", *file_proof_paths]]
        rows.append(row)
    receipt = {key: request[key] for key in ("run_id", "plan_sha256", "registry_sha256", "target")}
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
