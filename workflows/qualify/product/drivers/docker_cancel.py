"""Qualify cancellation of an attached Dockpipe workflow on a private Linux daemon."""

import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import platform
import re
import shutil
import signal
import subprocess

from docker import blocked_prerequisite, docker_read
from docker_detach import readback, start_time, wait_for
from contracts import digest_file, read_json, require, write_json
from process import MAX_LOG_BYTES, terminate_owned


CASE = ("workflow.containers", "cancellation", "component")
ASSERTIONS = {"cancellation-observed", "owned-work-stopped", "no-implicit-replay"}


def write_workflow_fixture(workspace, image, name, program):
    workflow = workspace / "workflows" / name
    workflow.mkdir(parents=True)
    config = workflow / "config.yml"
    config.write_text(f"""name: {name}
vault: none
runtime: dockerimage
isolate: {image}
security:
  profile: secure-default
  network:
    mode: offline
  filesystem:
    root: readonly
    writes: workspace-only
  process:
    user: non-root
    pid_limit: 64
    resources:
      cpu: '1'
      memory: 128m
""")
    project = workspace / "dockpipe.config.json"
    project.write_text('{"schema":1}\n')
    script = workspace / "proof.sh"
    script.write_text(program)
    return [project, config, script]


def write_fixture(workspace, image):
    return write_workflow_fixture(workspace, image, "attached-cancel", """set -eu
test "$(id -u)" -gt 0
trap 'printf "%s\\n" term >> cancelled.txt; exit 143' TERM
printf '%s\\n' started >> starts.txt
printf '%s\\n' attached-cancel-ready-v1
for attempt in {1..90}; do
    sleep 1
done
printf '%s\\n' deadline > unexpected-completion.txt
exit 23
""")


def container_facts(endpoint, prefix, image, workspace):
    names = readback(endpoint, ["ps", "--no-trunc", "--quiet", "--filter", "name=" + prefix]).splitlines()
    require(len(names) <= 1, "Attached fixture identity is ambiguous")
    if not names:
        return None

    identifier, = names
    require(re.fullmatch("[0-9a-f]{64}", identifier), "Invalid attached container ID")
    facts, = json.loads(readback(endpoint, ["inspect", identifier]))
    require(facts["Id"] == identifier and facts["Image"] == image
            and facts["Name"].startswith("/" + prefix) and facts["State"]["Running"]
            and facts["HostConfig"]["NetworkMode"] == "none"
            and any(mount["Type"] == "bind" and mount["Source"] == str(workspace)
                    and mount["Destination"] == "/work" for mount in facts["Mounts"]),
            "Container does not match the attached private workflow")
    return identifier, facts["Name"][1:], facts["State"]["Pid"]


def container_workers(endpoint, identifier, worker):
    top = readback(endpoint, ["top", identifier, "-eo", "pid,ppid,comm"])
    require(1 <= len(top.splitlines()) <= 65, "Unexpected attached process count")
    processes = {}
    for line in top.splitlines()[1:]:
        pid = line.split()[0]
        require(re.fullmatch("[1-9][0-9]{0,7}", pid), "Invalid attached process ID")
        token = start_time(pid)
        if token is not None:
            processes[pid] = token
    require(str(worker) in processes and len(processes) >= 2, "Container process identities were not observed")
    return top, processes


def container_events(endpoint, prefix, started, output):
    end = datetime.now(timezone.utc).isoformat()
    events = docker_read(endpoint, ["events", "--since", started, "--until", end,
                                    "--filter", "type=container", "--format", "{{json .}}"])
    if events.returncode != 0:
        (output / "events-error.txt").write_bytes(events.stderr)
    require(events.returncode == 0, "Docker event read-back failed")
    owned_events = []
    for line in events.stdout.splitlines():
        event = json.loads(line)
        actor = event["Actor"]
        attributes = actor["Attributes"]
        if attributes.get("name", "").startswith(prefix):
            owned_events.append({"id": actor["ID"], "action": event["Action"], "name": attributes["name"],
                                 "exit_code": attributes.get("exitCode")})

    return owned_events


def run_attached(cli, workspace, output, environment, endpoint, image):
    command = [str(cli), "--no-data", "--workflow", "attached-cancel", "--", "bash", "./proof.sh"]
    stdout_path = output / "workflow.stdout"
    stderr_path = output / "workflow.stderr"
    started = datetime.now(timezone.utc).isoformat()
    with stdout_path.open("xb") as stdout, stderr_path.open("xb") as stderr:
        process = subprocess.Popen(command, cwd=workspace, env=environment, stdin=subprocess.DEVNULL,
                                   stdout=stdout, stderr=stderr, start_new_session=True)
        prefix = f"dockpipe-{process.pid}-"
        try:
            def reached_barrier():
                require(max(stdout_path.stat().st_size, stderr_path.stat().st_size) <= MAX_LOG_BYTES,
                        "Attached log exceeds its bound")
                with stdout_path.open("rb") as log:
                    return process.poll() is not None or b"attached-cancel-ready-v1\n" in log.read(MAX_LOG_BYTES)

            ready = wait_for(reached_barrier, 25)
            require(ready and process.poll() is None, "Attached workload did not reach its owned readiness barrier")
            active = container_facts(endpoint, prefix, image, workspace)
            require(active is not None, "Ready attached container was not observed")
            identifier, name, worker = active
            top, processes = container_workers(endpoint, identifier, worker)
            write_json(output / "active.json", {"container": identifier, "name": name, "dockpipe_pid": process.pid,
                                                "top": top, "processes": processes})
            require(process.poll() is None, "Dockpipe exited before cancellation")
            process.send_signal(signal.SIGTERM)
            exit_code = process.wait(timeout=20)
        finally:
            if process.poll() is None:
                terminate_owned(process)

    require(max(stdout_path.stat().st_size, stderr_path.stat().st_size) <= MAX_LOG_BYTES, "Attached log exceeds its bound")
    removal = docker_read(endpoint, ["inspect", identifier])
    (output / "cleanup.txt").write_bytes(removal.stderr)
    write_json(output / "completion.json", {"signal": "SIGTERM", "exit_code": exit_code,
                                           "container": identifier, "inspect_exit": removal.returncode})

    event_path = output / "events.json"
    owned_events = container_events(endpoint, prefix, started, output)
    write_json(event_path, owned_events)
    termination = {"id": identifier, "action": "die", "name": name, "exit_code": "143"}
    require(exit_code in (1, 143) and (workspace / "cancelled.txt").read_bytes() == b"term\n"
            and termination in owned_events,
            "Expected explicit container termination after SIGTERM")
    require(removal.returncode != 0 and f"no such object: {identifier}".encode() in removal.stderr.lower()
            and all(start_time(pid) != token for pid, token in processes.items()),
            "Dockpipe did not remove the exact container and its observed workers")
    require([event["action"] for event in owned_events if event["action"] in
             {"create", "start", "die", "destroy"}] == ["create", "start", "die", "destroy"]
            and all(event["id"] == identifier for event in owned_events)
            and (workspace / "starts.txt").read_bytes() == b"started\n"
            and not (workspace / "unexpected-completion.txt").exists(),
            "Attached workflow was replayed or reached its natural deadline")
    return [stdout_path, stderr_path, output / "active.json", output / "completion.json", event_path,
            output / "cleanup.txt", workspace / "starts.txt", workspace / "cancelled.txt"]


def prepare_target(request, output):
    target = request["target"]
    config = request["configuration"]
    require(platform.system() == "Linux" and target["platform"] == "linux-portable" and target["kind"] == "local"
            and os.geteuid() != 0, "Select a non-root local Linux portable target")
    require(set(config) == {"artifact", "image", "endpoint"}, "Select the artifact, cached image and endpoint")
    artifact = Path(config["artifact"])
    image = config["image"]
    endpoint = config["endpoint"]
    require(endpoint == "unix:///var/run/docker.sock" and re.fullmatch("sha256:[0-9a-f]{64}", image),
            "Select the local Docker endpoint and a pinned cached image")
    require(artifact.is_absolute() and digest_file(artifact) == target["artifact_sha256"], "Selected CLI artifact changed")

    try:
        admitted = readback(endpoint, ["image", "inspect", "--format", "{{.Id}}", image]).strip()
        containers = readback(endpoint, ["ps", "--all", "--quiet"]).strip()
    except (OSError, ValueError, subprocess.TimeoutExpired):
        admitted, containers = None, None
    if admitted != image or containers != "":
        blocked_prerequisite(request, output, "Select an empty disposable Docker daemon with the pinned cached image")
        return

    output.mkdir(mode=0o700)
    cli = output / "dockpipe"
    shutil.copyfile(artifact, cli)
    cli.chmod(0o700)
    require(digest_file(cli) == target["artifact_sha256"], "Installed CLI differs from its admitted artifact")
    workspace = output / "workspace"
    home = output / "home"
    home.mkdir(mode=0o700)
    environment = {"HOME": str(home), "PATH": os.environ.get("PATH", os.defpath), "LANG": "C.UTF-8",
                   "DOCKER_HOST": endpoint, "DOCKER_CONFIG": str(home / "docker"), "DOCKPIPE_INTERACTIVE": "0"}
    for name in ("data", "config", "cache", "state"):
        environment[f"XDG_{name.upper()}_HOME"] = str(home / name)
    return cli, workspace, environment, endpoint, image


def run(request, output):
    case, = request["cases"]
    require(tuple(case[key] for key in ("id", "scenario", "layer")) == CASE
            and set(case["required_assertions"]) == ASSERTIONS, "Unsupported attached cancellation case")
    selected = prepare_target(request, output)
    if selected is None:
        return

    cli, workspace, environment, endpoint, image = selected
    evidence = write_fixture(workspace, image)
    evidence.extend(run_attached(cli, workspace, output, environment, endpoint, image))
    row = {key: case[key] for key in ("id", "scenario", "layer")}
    row.update(status="passed", reason="Attached Docker workflow stopped after one signal and one start",
               assertions=[{"id": name, "status": "passed", "detail": "See workflow and exact Docker read-back"}
                           for name in case["required_assertions"]],
               evidence=[{"path": path.relative_to(output).as_posix(), "sha256": digest_file(path)}
                         for path in evidence])
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
