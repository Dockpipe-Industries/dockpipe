"""Qualify synchronous host step cancellation and owner-crash recovery."""

import argparse
from pathlib import Path
import platform
import shutil
import signal
import subprocess
import sys

from lifecycle import alive, checked_command, identity, stop_fixture_group, wait_for
from contracts import digest_file, read_json, require, validate_receipt, write_json
from process import MAX_LOG_BYTES


ASSERTIONS = {
    "cancellation": {"cancellation-observed", "owned-work-stopped", "no-implicit-replay"},
    "restart-recovery": {"original-identity-retained", "state-reconciled", "no-duplicate-effects"},
}
NAME = "cancel-host-proof"
TASK = "wait-for-cancel"
CONFIGURATION = """name: cancel-host-proof
vault: none
docker_preflight: false
steps:
  - id: wait-for-cancel
    kind: host
    cwd: repo
    run: worker.sh
  - id: after-cancellation
    kind: host
    cwd: repo
    cmd: touch unexpected-following-step.txt
"""

WORKER = """import json
import os
from pathlib import Path
import signal
import subprocess
import time


def request_stop(_signal, _frame):
    global cancelled
    cancelled = True


cancelled = False
signal.signal(signal.SIGTERM, request_stop)
with Path('starts.txt').open('a') as starts:
    starts.write('started\\n')
child = subprocess.Popen(['/usr/bin/sleep', '25'])
ready = Path('ready.pending')
ready.write_text(json.dumps({'worker': os.getpid(), 'child': child.pid}))
ready.replace('ready.json')
try:
    deadline = time.monotonic() + 20
    while not cancelled and time.monotonic() < deadline:
        time.sleep(0.02)
finally:
    if child.poll() is None:
        child.terminate()
    child.wait(timeout=3)
if cancelled:
    Path('cancelled.txt').write_bytes(b'cancelled\\n')
    raise SystemExit(143)
raise SystemExit(77)
"""


def observe(cli, workspace, environment, output, phase, owner, worker):
    snapshot = checked_command(cli, ["runs", "observe", "--workdir", str(workspace), "--json"],
                               workspace, environment, output, phase)
    require(snapshot.get("schema") == "dockpipe.execution_snapshot.v1" and snapshot.get("truncated") is False,
            "Unexpected project execution snapshot")
    records = [record for record in snapshot["executions"] if record.get("workflow") == NAME]
    require(len(records) == 1, "Expected only one execution of the synchronous fixture")
    record = records[0]
    require(record.get("runtime") == "host" and record.get("task") == TASK
            and record.get("parent_pid") == owner["pid"] and record.get("process") == worker,
            "Public observation did not bind the exact original host worker")
    return record


def exercise(cli, workspace, environment, output, scenario):
    stdout_path = output / "workflow.stdout"
    stderr_path = output / "workflow.stderr"
    command = [str(cli), "--no-data", "--workdir", str(workspace), "--workflow", NAME]
    original = None
    references = []
    facts = {"command": command, "fallback_used": False, "error": ""}
    with stdout_path.open("xb") as stdout, stderr_path.open("xb") as stderr:
        process = subprocess.Popen(command, cwd=output, env=environment, stdin=subprocess.DEVNULL,
                                   stdout=stdout, stderr=stderr, start_new_session=True)
        try:
            original = identity(process.pid)
            require(original is not None, "Original Dockpipe process identity is unavailable")
            ready = workspace / "ready.json"
            wait_for(ready.is_file, 6, process)
            started = read_json(ready)
            references = [identity(started[name]) for name in ("worker", "child")]
            require(all(references), "The original host worker or child was unavailable")
            worker, child = references
            child_status = Path("/proc", str(child["pid"]), "status").read_text().splitlines()
            parent, = [line.split()[1] for line in child_status if line.startswith("PPid:")]
            require(int(parent) == worker["pid"], "The observed child does not belong to the fixture worker")
            facts["original"] = original
            facts["worker"] = worker
            facts["child"] = child
            started_record = observe(cli, workspace, environment, output, "observe-active", original, worker)
            require(started_record["state"] == "running" and all(alive(item) for item in references)
                    and process.poll() is None, "The original CLI and fixture tree were not live together")

            if scenario == "restart-recovery":
                process.kill()
                facts["signal"] = "SIGKILL"
                facts["cli_exit"] = process.wait(timeout=5)
                require(facts["cli_exit"] == -signal.SIGKILL and not alive(original),
                        "Original CLI owner did not exit after SIGKILL")
                recovered = observe(cli, workspace, environment, output, "observe-recovered", original, worker)
                facts["original_identity_retained"] = (recovered["id"] == started_record["id"]
                                                       and recovered["started_at"] == started_record["started_at"]
                                                       and recovered["state"] in {"running", "unverified"})
            else:
                process.send_signal(signal.SIGTERM)
                facts["signal"] = "SIGTERM"
                facts["cli_exit"] = process.wait(timeout=8)

            wait_for(lambda: not any(alive(item) for item in references), 4)
            terminal = observe(cli, workspace, environment, output, "observe-final", original, worker)
            repeated = observe(cli, workspace, environment, output, "observe-repeat", original, worker)
            facts["terminal_record"] = terminal
            facts["same_execution"] = terminal["id"] == started_record["id"] and terminal == repeated
            facts["workflow_attempts"] = (workspace / "starts.txt").read_text().splitlines()
            facts["following_step_absent"] = not (workspace / "unexpected-following-step.txt").exists()
            facts["cancel_acknowledged"] = (workspace / "cancelled.txt").read_bytes() == b"cancelled\n"
            require(max(stdout_path.stat().st_size, stderr_path.stat().st_size) <= MAX_LOG_BYTES,
                    "Fixture log exceeded its bound")
        except (OSError, RuntimeError, ValueError, TimeoutError, subprocess.SubprocessError) as error:
            facts["error"] = str(error)
        finally:
            references = [item for item in references if item is not None]
            if process.poll() is None or any(alive(item) for item in references):
                facts["fallback_used"] = True
                if original is not None:
                    stop_fixture_group(process, original, references)
            owned = references if original is None else [original, *references]
            facts["remaining"] = [item for item in owned if alive(item)]
    return facts, [stdout_path, stderr_path]


def assertions(scenario, facts):
    terminal = facts.get("terminal_record", {})
    stopped = not facts["error"] and not facts["fallback_used"] and facts["remaining"] == []
    once = facts.get("workflow_attempts") == ["started"] and facts.get("following_step_absent") is True
    if scenario == "restart-recovery":
        retained = (stopped and facts.get("signal") == "SIGKILL" and facts.get("cli_exit") == -signal.SIGKILL
                    and facts.get("original_identity_retained") is True)
        reconciled = (retained and facts.get("cancel_acknowledged") is True and facts.get("same_execution") is True
                      and terminal.get("state") == "unverified"
                      and terminal.get("exit_code") is None and terminal.get("reason") in {
                          "process_exited_or_inaccessible", "process_identity_unavailable_or_changed"})
        return {
            "original-identity-retained": retained,
            "state-reconciled": reconciled,
            "no-duplicate-effects": reconciled and once,
        }

    observed = (stopped and facts.get("signal") == "SIGTERM" and facts.get("cli_exit") == 1
                and facts.get("cancel_acknowledged") is True and terminal.get("exit_code") == 143
                and terminal.get("state") in {"cancelled", "terminated", "failed"})
    return {
        "cancellation-observed": observed,
        "owned-work-stopped": stopped,
        "no-implicit-replay": observed and once and facts.get("same_execution") is True,
    }


def run(request, output):
    target = request["target"]
    require(platform.system() == "Linux" and target["platform"] == "linux-portable" and target["kind"] == "local",
            "Host cancellation requires a local Linux portable CLI")
    case, = request["cases"]
    scenario = case["scenario"]
    require(case["id"] == "workflow.host" and case["layer"] == "component" and scenario in ASSERTIONS
            and set(case["required_assertions"]) == ASSERTIONS[scenario], "Unexpected host step assertion contract")
    require(set(request["configuration"]) == {"artifact"}, "Select exactly one CLI artifact")
    artifact = Path(request["configuration"]["artifact"])
    pin = target["artifact_sha256"]
    require(artifact.is_absolute() and digest_file(artifact) == pin, "CLI artifact differs from its pin")

    output.mkdir(mode=0o700, parents=True)
    cli = output / "dockpipe"
    shutil.copyfile(artifact, cli)
    cli.chmod(0o700)
    require(digest_file(cli) == pin, "Copied CLI artifact differs from its pin")
    home = output / "home"
    home.mkdir(mode=0o700)
    environment = {"HOME": str(home), "PATH": "/usr/bin:/bin", "LANG": "C.UTF-8",
                   "DOCKPIPE_INTERACTIVE": "0", "DOCKPIPE_VAULT_INJECT": "0"}
    workspace = output / "generated project"
    source = workspace / "workflows" / NAME
    source.mkdir(parents=True)
    project = workspace / "dockpipe.config.json"
    project.write_text("{}\n")
    config = source / "config.yml"
    config.write_text(CONFIGURATION)
    wrapper = source / "worker.sh"
    wrapper.write_text('set -eu\nexec /usr/bin/python3 "$DOCKPIPE_SCRIPT_DIR/worker.py"\n')
    worker = source / "worker.py"
    worker.write_text(WORKER)

    facts, evidence = exercise(cli, workspace, environment, output, scenario)
    observations = output / "observations.json"
    write_json(observations, facts)
    checks = assertions(scenario, facts)
    evidence.extend([observations, project, config, wrapper, worker])
    for name in ("ready.json", "starts.txt", "cancelled.txt"):
        path = workspace / name
        if path.is_file():
            evidence.append(path)
    for phase in ("observe-active", "observe-recovered", "observe-final", "observe-repeat"):
        path = output / phase / "driver.stdout"
        if path.is_file():
            evidence.append(path)
    row = {key: case[key] for key in ("id", "scenario", "layer")}
    reason = facts["error"] or {
        "cancellation": "One CLI signal and exact synchronous host process and observer read-back",
        "restart-recovery": "Fresh observers retained the original host step through its owner's crash",
    }[scenario]
    row.update(status="passed" if all(checks.values()) else "failed", reason=reason,
               assertions=[{"id": name, "status": "passed" if passed else "failed", "detail": reason}
                           for name, passed in checks.items()],
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
    require(request.get("schema") == "dockpipe.product-integration.request/v1", "Unknown qualification request schema")
    run(request, arguments.output)


if __name__ == "__main__":
    main()
