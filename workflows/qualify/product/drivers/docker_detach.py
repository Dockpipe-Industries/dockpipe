"""Qualify a real detached Dockpipe workload on a selected Linux Docker daemon."""
from datetime import datetime, timezone
from pathlib import Path
import argparse
import os
import platform
import re
import shutil
import subprocess
import time

from docker import blocked_prerequisite, docker_read
from contracts import digest_file, read_json, require, write_json
from process import run_driver


ASSERTIONS = {"detached-run-observed", "logs-and-result-bound", "owned-process-tree-closed"}


def readback(endpoint, arguments):
    result = docker_read(endpoint, arguments)
    require(result.returncode == 0, "Docker read-back failed for " + arguments[0])
    return result.stdout.decode()


def start_time(pid):
    try:
        return Path(f"/proc/{pid}/stat").read_text().rsplit(") ", 1)[1].split()[19]
    except (FileNotFoundError, ProcessLookupError):
        return None


def wait_for(observe, timeout):
    deadline = time.monotonic() + timeout
    while True:
        result = observe()
        if result or time.monotonic() >= deadline:
            return result
        time.sleep(0.1)


def write_fixture(workspace, image):
    workflow = workspace / "workflows/detached-proof"
    workflow.mkdir(parents=True)
    config = workflow / "config.yml"
    config.write_text(f"""name: detached-proof
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
    (workspace / "dockpipe.config.json").write_text('{"schema":1}\n')
    script = workspace / "proof.sh"
    script.write_text("""set -euo pipefail
test "$FIXTURE_MARKER" = detach-selected-v1
test "$(id -u)" -gt 0
test "$(awk '/^NoNewPrivs:/{print $2}' /proc/self/status)" = 1
set -o noclobber
printf 'detach-log-ready-v1\\n'
for n in {1..150}; do
    if test -f release.txt; then
        test "$(cat release.txt)" = detach-release-v1
        printf 'detach-result-v1\\n' > result.txt
        exit 0
    fi
    sleep 0.5
done
exit 22
""")
    return [config, script]


def run(request, output):
    target = request["target"]
    configuration = request["configuration"]
    cases = request["cases"]
    require(platform.system() == "Linux" and target["platform"] == "linux-portable"
            and target["kind"] == "local" and os.geteuid() != 0, "Select a non-root local Linux portable target")
    require(len(cases) == 1, "Select exactly the detached lifecycle case")
    case = cases[0]
    require((case["id"], case["scenario"], case["layer"]) == ("workflow.lifecycle", "happy", "installed_e2e")
            and set(case["required_assertions"]) == ASSERTIONS, "Unsupported detach qualification case")
    require(set(configuration) == {"artifact", "image", "endpoint"}, "Select the artifact, cached image and endpoint")
    endpoint = configuration["endpoint"]
    image = configuration["image"]
    artifact = Path(configuration["artifact"])
    require(endpoint == "unix:///var/run/docker.sock", "Select the admitted local Docker endpoint")
    require(re.fullmatch("sha256:[0-9a-f]{64}", image), "Select an immutable cached image ID")
    require(artifact.is_absolute() and digest_file(artifact) == target["artifact_sha256"], "Selected CLI artifact changed")
    try:
        admitted_image = readback(endpoint, ["image", "inspect", "--format", "{{.Id}}", image]).strip()
    except (OSError, ValueError, subprocess.TimeoutExpired):
        admitted_image = None
    if admitted_image != image:
        blocked_prerequisite(request, output, "Pinned image is unavailable on the selected daemon before launch")
        return

    output.mkdir(mode=0o700)
    cli = output / "dockpipe"
    shutil.copyfile(artifact, cli)
    cli.chmod(0o700)
    require(digest_file(cli) == target["artifact_sha256"], "Installed CLI differs from its admitted artifact")
    workspace = output / "workspace"
    result_path = workspace / "result.txt"
    evidence = write_fixture(workspace, image)
    home = output / "home"
    home.mkdir(mode=0o700)
    environment = {"HOME": str(home), "PATH": os.environ.get("PATH", os.defpath), "LANG": "C.UTF-8",
                   "DOCKER_HOST": endpoint, "DOCKER_CONFIG": str(home / "docker"), "DOCKPIPE_INTERACTIVE": "0"}
    for name in ("data", "config", "cache", "state"):
        environment[f"XDG_{name.upper()}_HOME"] = str(home / name)
    started = datetime.now(timezone.utc).isoformat()
    logs = output / "launch"
    logs.mkdir(mode=0o700)
    launch = run_driver([str(cli), "--no-data", "--detach", "--workflow", "detached-proof",
                         "--env", "FIXTURE_MARKER=detach-selected-v1", "--", "bash", "./proof.sh"],
                        logs, 25, environment, workspace)
    write_json(logs / "process.json", launch)
    require(launch["exit_code"] == 0 and not launch["failure"], "Detached Dockpipe launch did not return normally")
    identifiers = re.findall(r"(?m)^[a-f0-9]{64}$", (logs / "driver.stdout").read_text())
    require(len(identifiers) == 1, "Dockpipe did not return one full container identity")
    identifier = identifiers[0]
    facts = readback(endpoint, ["inspect", "--format",
                                "{{.Id}} {{.Image}} {{.State.Running}} {{.HostConfig.AutoRemove}} "
                                "{{.HostConfig.NetworkMode}} {{.HostConfig.ReadonlyRootfs}} "
                                "{{.HostConfig.Memory}} {{.HostConfig.NanoCpus}} {{.HostConfig.PidsLimit}} {{.State.Pid}}",
                                identifier]).split()
    require(len(facts) == 10 and facts[:-1] == [identifier, image, "true", "true", "none", "true",
                                             "134217728", "1000000000", "64"] and not result_path.exists()
            and re.fullmatch(r"[1-9][0-9]{0,7}", facts[-1]), "Detached container state or policy did not match")
    mounts = readback(endpoint, ["inspect", "--format",
                                 '{{range .Mounts}}{{.Type}} {{.Source}} {{.Destination}}{{println}}{{end}}', identifier])
    require(f"bind {workspace} /work" in mounts.splitlines()
            and not any(line.startswith("volume ") for line in mounts.splitlines()),
            "Container did not use the exact fixture workspace with no data volume")
    top = readback(endpoint, ["top", identifier, "-eo", "pid,ppid,comm"])
    processes = {}
    for line in top.splitlines()[1:]:
        pid = line.split()[0]
        require(re.fullmatch(r"[1-9][0-9]{0,7}", pid), "Invalid container process PID")
        token = start_time(pid)
        if token is not None:
            processes[pid] = token
    require(facts[-1] in processes and len(processes) >= 2, "Owned container worker identities were not observable")
    observation = output / "observed.json"
    write_json(observation, {"container": identifier, "state": facts, "mounts": mounts, "top": top, "processes": processes})

    def ready_log():
        log = readback(endpoint, ["logs", identifier])
        return log if log == "detach-log-ready-v1\n" else None

    log = wait_for(ready_log, 12)
    require(log is not None and not result_path.exists(), "Exact detached log was not observed before release")
    container_log = output / "container.log"
    container_log.write_text(log)
    temporary_release = workspace / "release.pending"
    temporary_release.write_text("detach-release-v1\n")
    temporary_release.rename(workspace / "release.txt")

    def removal():
        result = docker_read(endpoint, ["inspect", identifier])
        if result.returncode != 0 and f"no such object: {identifier}" in result.stderr.decode().lower():
            return result.stderr
        return None

    removed = wait_for(removal, 20)
    require(removed is not None, "Exact detached container did not auto-remove")
    cleanup = output / "cleanup.txt"
    cleanup.write_bytes(removed)
    require(result_path.is_file() and result_path.read_text() == "detach-result-v1\n", "Detached result differed")
    require(all(start_time(pid) != token for pid, token in processes.items()), "An observed worker identity remains")
    end = datetime.now(timezone.utc).isoformat()
    events = readback(endpoint, ["events", "--since", started, "--until", end, "--filter", f"container={identifier}",
                                 "--format", '{{.Action}} {{index .Actor.Attributes "exitCode"}}'])
    require({"create ", "start ", "die 0", "destroy "} <= set(events.splitlines()), "Docker events did not confirm exact normal removal")
    event_log = output / "events.txt"
    event_log.write_text(events)
    evidence.extend([*sorted(logs.iterdir()), observation, container_log, result_path, cleanup, event_log])
    row = {key: case[key] for key in ("id", "scenario", "layer")}
    row.update(status="passed", reason="Real detached workload, exact log/result, and owned cleanup observed",
               assertions=[{"id": name, "status": "passed", "detail": "See bounded runtime and process evidence"}
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
