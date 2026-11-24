"""Qualify a synchronous host step failure with the pinned Linux portable CLI."""

import argparse
from pathlib import Path
import platform
import shutil
import sys


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "assets/scripts"))
from contracts import digest_file, read_json, require, validate_receipt, write_json
from process import run_driver


CASE = ("workflow.host", "failure", "component")
DIAGNOSTIC = "fixture input rejected. Correct the fixture and rerun."
CONFIGURATION = """name: failure-proof
vault: none
docker_preflight: false
steps:
  - id: before-failure
    kind: host
    cwd: repo
    cmd: printf '%s\\n' before > before.txt
  - id: reject-input
    kind: host
    cwd: repo
    run: reject-input.sh
  - id: after-failure
    kind: host
    cwd: repo
    cmd: printf '%s\\n' after > after.txt
"""


def run(request, output):
    target = request["target"]
    require(platform.system() == "Linux" and target["platform"] == "linux-portable" and target["kind"] == "local",
            "Host failure requires a local Linux portable CLI")
    case, = request["cases"]
    require(tuple(case[key] for key in ("id", "scenario", "layer")) == CASE
            and set(case["required_assertions"]) == {"expected-failure-observed", "no-unowned-effects", "actionable-result"},
            "Unexpected host failure assertion contract")
    require(set(request["configuration"]) == {"artifact"}, "Select exactly one CLI artifact")
    artifact = Path(request["configuration"]["artifact"])
    require(artifact.is_absolute() and digest_file(artifact) == target["artifact_sha256"], "CLI artifact differs from its pin")

    output.mkdir(mode=0o700, parents=True)
    cli = output / "dockpipe"
    shutil.copyfile(artifact, cli)
    cli.chmod(0o700)
    require(digest_file(cli) == target["artifact_sha256"], "Copied CLI artifact differs from its pin")
    workspace = output / "generated project"
    fixture = workspace / "workflows/failure-proof"
    fixture.mkdir(parents=True)
    project = workspace / "dockpipe.config.json"
    project.write_text("{}\n")
    config = fixture / "config.yml"
    config.write_text(CONFIGURATION)
    script = fixture / "reject-input.sh"
    script.write_text(f"printf '%s\\n' '{DIAGNOSTIC}' >&2\nexit 23\n")
    adjacent = output / "adjacent-canary.txt"
    adjacent.write_bytes(b"Generated sibling must remain unchanged.\n")
    expected_hashes = {path: digest_file(path) for path in (project, config, script, adjacent)}
    expected_paths = {path.relative_to(workspace) for path in workspace.rglob("*")}

    home = output / "home"
    home.mkdir(mode=0o700)
    environment = {"HOME": str(home), "PATH": "/usr/bin:/bin", "LANG": "C.UTF-8",
                   "DOCKPIPE_INTERACTIVE": "0", "DOCKPIPE_VAULT_INJECT": "0"}
    directories = {"XDG_CONFIG_HOME": "config", "XDG_STATE_HOME": "state", "XDG_CACHE_HOME": "cache",
                   "XDG_DATA_HOME": "data", "XDG_RUNTIME_DIR": "runtime", "TMPDIR": "tmp"}
    for variable, name in directories.items():
        directory = home / name
        directory.mkdir(mode=0o700)
        environment[variable] = str(directory)

    logs = output / "invocation"
    logs.mkdir(mode=0o700)
    command = [str(cli), "--no-data", "--workdir", str(workspace), "--workflow", "failure-proof"]
    outcome = run_driver(command, logs, 20, environment, output)
    process = logs / "process.json"
    write_json(process, {"command": command, **outcome})
    stdout = logs / "driver.stdout"
    stderr = logs / "driver.stderr"
    diagnostic = stderr.read_text(errors="replace").splitlines()
    final_line = diagnostic[-1] if diagnostic else ""
    step_exit = final_line.startswith("host script ") and final_line.endswith("/reject-input.sh: exit status 23")
    before = workspace / "before.txt"
    preceding_step_ran = before.is_file() and before.read_bytes() == b"before\n"
    following_step_skipped = not (workspace / "after.txt").exists()
    intact = all(path.is_file() and not path.is_symlink() and digest_file(path) == expected
                 for path, expected in expected_hashes.items())
    project_paths = {path.relative_to(workspace) for path in workspace.rglob("*")}
    visible_paths = {path for path in project_paths if path.parts[0] != "bin"}
    expected_paths.add(Path("before.txt"))
    normal_failure = outcome["exit_code"] == 1 and outcome["failure"] == ""
    checks = {
        "expected-failure-observed": (normal_failure and preceding_step_ran and step_exit,
                                      "The preceding host command ran and the selected script's exit 23 failed Dockpipe."),
        "no-unowned-effects": (intact and following_step_skipped and visible_paths == expected_paths,
                               "Original inputs and adjacent canary retained their hashes and the next step stayed absent."),
        "actionable-result": (normal_failure and step_exit and DIAGNOSTIC in diagnostic,
                              "Stderr identifies the failing host script, its exit code and the step's corrective diagnostic."),
    }
    observations = output / "observations.json"
    write_json(observations, {"preceding_step_ran": preceding_step_ran, "following_step_skipped": following_step_skipped,
                              "original_hashes": {str(path.relative_to(output)): value for path, value in expected_hashes.items()},
                              "original_files_unchanged": intact, "remaining_project_entries": sorted(map(str, visible_paths))})
    evidence = [process, stdout, stderr, observations, project, config, script, adjacent]
    if before.is_file():
        evidence.append(before)
    row = {key: case[key] for key in ("id", "scenario", "layer")}
    row.update(status="passed" if all(passed for passed, _ in checks.values()) else "failed",
               reason="Synchronous host command failure and short-circuit readback",
               assertions=[{"id": name, "status": "passed" if passed else "failed", "detail": detail}
                           for name, (passed, detail) in checks.items()],
               evidence=[{"path": path.relative_to(output).as_posix(), "sha256": digest_file(path)} for path in evidence])
    receipt = {key: request[key] for key in ("run_id", "plan_sha256", "registry_sha256", "target")}
    receipt.update(schema="dockpipe.product-integration.lane/v1", cases=[row])
    validate_receipt(receipt, request, output)
    write_json(output / "result.json", receipt)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--request", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    arguments = parser.parse_args()
    require(arguments.output.is_absolute() and not arguments.output.exists(), "Select a fresh absolute output directory")
    request = read_json(arguments.request)
    require(request.get("schema") == "dockpipe.product-integration.request/v1", "Unknown request schema")
    run(request, arguments.output)


if __name__ == "__main__":
    main()
