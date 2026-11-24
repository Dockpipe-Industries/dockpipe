"""Reconcile the original attached container after its Dockpipe owner crashes."""

import argparse
from datetime import datetime, timezone
import json
from pathlib import Path
import signal
import subprocess

from docker import docker_read
from docker_cancel import container_events, container_facts, container_workers, prepare_target, write_workflow_fixture
from docker_detach import readback, start_time, wait_for
from lifecycle import checked_command
from contracts import digest_file, read_json, require, write_json
from process import MAX_LOG_BYTES, terminate_owned


CASE = ("workflow.containers", "restart-recovery", "component")
ASSERTIONS = {"original-identity-retained", "state-reconciled", "no-duplicate-effects"}
FIXTURE = "attached-recovery"
READY_LINE = b"attached-recovery-ready-v1\n"

PROGRAM = """set -eu
test "$(id -u)" -gt 0
printf '%s\\n' started >> starts.txt
printf '%s\\n' attached-recovery-ready-v1
for attempt in {1..90}; do
    if test -f release.txt; then
        test "$(cat release.txt)" = attached-recovery-release-v1
        printf '%s\\n' attached-recovery-result-v1 >> result.txt
        exit 0
    fi
    sleep 1
done
exit 23
"""


def read_record(cli, workspace, environment, output, phase, container_name, owner):
    snapshot = checked_command(cli, ["runs", "observe", "--workdir", str(workspace), "--json"],
                               workspace, environment, output, phase)
    require(snapshot["schema"] == "dockpipe.execution_snapshot.v1" and not snapshot.get("truncated"),
            "Unexpected or truncated project execution snapshot")
    records = [record for record in snapshot["executions"] if record.get("container") == container_name]
    require(len(records) == 1 and records[0]["parent_pid"] == owner and records[0]["runtime"] == "docker",
            "Expected one execution associated with the original Dockpipe owner and container")
    return records[0]


def identity(record):
    return (record["id"], record["parent_pid"], record["container"], record["process"], record["started_at"])


def run_recovery(cli, workspace, output, environment, endpoint, image):
    command = [str(cli), "--no-data", "--workdir", str(workspace), "--workflow", FIXTURE, "--", "bash", "./proof.sh"]
    stdout_path = output / "workflow.stdout"
    stderr_path = output / "workflow.stderr"
    started = datetime.now(timezone.utc).isoformat()
    with stdout_path.open("xb") as stdout, stderr_path.open("xb") as stderr:
        process = subprocess.Popen(command, cwd=workspace, env=environment, stdin=subprocess.DEVNULL,
                                   stdout=stdout, stderr=stderr, start_new_session=True)
        prefix = f"dockpipe-{process.pid}-"
        try:
            owner_token = start_time(process.pid)
            require(owner_token is not None, "Original Dockpipe process identity was not observable")

            def ready():
                require(max(stdout_path.stat().st_size, stderr_path.stat().st_size) <= MAX_LOG_BYTES,
                        "Attached log exceeds its bound")
                with stdout_path.open("rb") as log:
                    return process.poll() is not None or READY_LINE in log.read(MAX_LOG_BYTES)

            require(wait_for(ready, 25) and process.poll() is None, "Original container never signalled readiness")
            active = container_facts(endpoint, prefix, image, workspace)
            require(active is not None, "Ready Docker container could not be identified")
            identifier, name, worker = active
            top, workers = container_workers(endpoint, identifier, worker)
            write_json(output / "active.json", {"container": identifier, "name": name, "worker": worker,
                                                "top": top, "workers": workers})
            original = read_record(cli, workspace, environment, output, "observe-original", name, process.pid)
            docker_process = original["process"]
            token = docker_process.get("start_token", "").rsplit(":", 1)[-1]
            require(original["state"] == "running" and bool(token) and start_time(docker_process["pid"]) == token,
                    "Original Dockpipe observation did not identify its live Docker process")
            require(process.poll() is None and not (workspace / "result.txt").exists(),
                    "Original workflow exited before the injected owner crash")
            process.kill()
            require(process.wait(timeout=5) == -signal.SIGKILL and start_time(process.pid) != owner_token,
                    "Original Dockpipe owner did not exit from the injected SIGKILL")
            write_json(output / "owner-crash.json", {"pid": process.pid, "start_token": owner_token,
                                                     "exit_code": process.returncode, "signal": "SIGKILL"})
        finally:
            if process.poll() is None:
                terminate_owned(process)

    recovered = read_record(cli, workspace, environment, output, "observe-recovered", name, process.pid)
    require(identity(recovered) == identity(original) and recovered["state"] == "running"
            and container_facts(endpoint, prefix, image, workspace) == active and start_time(worker) == workers[str(worker)]
            and not (workspace / "result.txt").exists(),
            "Fresh Dockpipe observer did not recover the same live execution and original container")

    release = workspace / "release.txt"
    pending = workspace / "release.pending"
    pending.write_bytes(b"attached-recovery-release-v1\n")
    pending.rename(release)

    def observe_exit():
        state = readback(endpoint, ["inspect", "--format", "{{json .State}}", identifier])
        facts = json.loads(state)
        return facts if facts["Status"] == "exited" else None

    exited = wait_for(observe_exit, 10)
    require(exited is not None, "Original container did not complete after release")
    write_json(output / "container-exit.json", {"id": identifier, "status": exited["Status"], "exit_code": exited["ExitCode"]})
    require(exited["ExitCode"] == 0 and (workspace / "starts.txt").read_bytes() == b"started\n"
            and (workspace / "result.txt").read_bytes() == b"attached-recovery-result-v1\n",
            "Original container did not complete exactly once")
    require(wait_for(lambda: start_time(docker_process["pid"]) != token
                     and all(start_time(pid) != timestamp for pid, timestamp in workers.items()), 5),
            "An observed attached worker or Docker process remained live")

    final = read_record(cli, workspace, environment, output, "observe-final", name, process.pid)
    require(identity(final) == identity(original) and final["state"] == "unverified"
            and final.get("reason") in {"process_exited_or_inaccessible", "process_identity_unavailable_or_changed"}
            and final.get("exit_code") is None,
            "A fresh Dockpipe observer did not preserve the original identity and uncertain completion")

    removed = readback(endpoint, ["container", "rm", "--", identifier]).strip()
    require(removed == identifier, "Explicit removal did not confirm the original stopped container")
    cleanup = docker_read(endpoint, ["inspect", identifier])
    (output / "cleanup.txt").write_bytes(cleanup.stderr)
    require(cleanup.returncode != 0 and f"no such object: {identifier}".encode() in cleanup.stderr.lower(),
            "Original exited container is still present")
    events = container_events(endpoint, prefix, started, output)
    write_json(output / "events.json", events)
    require([event["action"] for event in events if event["action"] in {"create", "start", "die", "destroy"}]
            == ["create", "start", "die", "destroy"]
            and all(event["id"] == identifier for event in events)
            and {"id": identifier, "action": "die", "name": name, "exit_code": "0"} in events,
            "Docker did not record exactly one lifecycle for the original attached container")
    require(max(stdout_path.stat().st_size, stderr_path.stat().st_size) <= MAX_LOG_BYTES, "Attached log exceeds its bound")
    return [stdout_path, stderr_path, output / "active.json", output / "owner-crash.json",
            output / "container-exit.json", output / "cleanup.txt", output / "events.json",
            release, workspace / "starts.txt", workspace / "result.txt",
            *(output / phase / "driver.stdout" for phase in ("observe-original", "observe-recovered", "observe-final"))]


def run(request, output):
    case, = request["cases"]
    require(tuple(case[key] for key in ("id", "scenario", "layer")) == CASE
            and set(case["required_assertions"]) == ASSERTIONS, "Unsupported attached Docker recovery case")
    selected = prepare_target(request, output)
    if selected is None:
        return

    cli, workspace, environment, endpoint, image = selected
    evidence = write_workflow_fixture(workspace, image, FIXTURE, PROGRAM)
    evidence.extend(run_recovery(cli, workspace, output, environment, endpoint, image))
    row = {key: case[key] for key in ("id", "scenario", "layer")}
    row.update(status="passed", reason="Fresh observers retained the original container identity without replay",
               assertions=[{"id": name, "status": "passed", "detail": "See project snapshots and exact Docker read-back"}
                           for name in case["required_assertions"]],
               evidence=[{"path": path.relative_to(output).as_posix(), "sha256": digest_file(path)} for path in evidence])
    receipt = {key: request[key] for key in ("run_id", "plan_sha256", "registry_sha256", "target")}
    receipt.update(schema="dockpipe.product-integration.lane/v1", cases=[row])
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
