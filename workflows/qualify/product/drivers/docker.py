"""One real offline container workflow on an explicitly selected local daemon."""
from pathlib import Path
import argparse
import json
import os
import platform
import re
import shutil
import stat
import subprocess
import sys
import time


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "assets/scripts"))
from contracts import digest_file, read_json, require, write_json
from process import run_driver


ASSERTIONS = {"declared-image-ran", "mount-env-output-binding", "owned-container-cleanup", "declared-policy-enforced"}
ENABLED_NO_NEW_PRIVILEGES = {"no-new-privileges", "no-new-privileges=true"}


def declared_policy_observed(observations):
    return (observations.get("network") == "none"
            and observations.get("readonly_root") is True
            and observations.get("memory_bytes") == 128 * 1024 * 1024
            and observations.get("cpu_nanoseconds") == 1000000000
            and observations.get("pid_limit") == 64
            and type(observations.get("effective_uid")) is int and 0 < observations["effective_uid"] < 4294967295
            and type(observations.get("kernel_no_new_privileges")) is int
            and observations["kernel_no_new_privileges"] == 1
            and "ALL" in observations.get("dropped_capabilities", [])
            and any(value in ENABLED_NO_NEW_PRIVILEGES for value in observations.get("security_options", [])))


def read_fixture_integer(path):
    try:
        information = path.lstat()
    except FileNotFoundError:
        return None
    require(stat.S_ISREG(information.st_mode) and information.st_size <= 32,
            "Fixture identity must be a bounded regular scalar file")
    with path.open("rb") as source:
        value = source.read(33)
    if not re.fullmatch(rb"[0-9]{1,10}\s*", value):
        return None
    return int(value)


def blocked_prerequisite(request, output, reason):
    output.mkdir(mode=0o700, parents=True)
    receipt = {key: request[key] for key in ("run_id", "plan_sha256", "registry_sha256", "target")}
    rows = []
    for case in request["cases"]:
        row = {key: case[key] for key in ("id", "scenario", "layer")}
        row.update(status="blocked", reason=reason, assertions=[], evidence=[])
        rows.append(row)
    receipt.update(schema="dockpipe.product-integration.lane/v1", cases=rows)
    write_json(output / "result.json", receipt)


def docker_read(endpoint, arguments):
    result = subprocess.run(["docker", "--host", endpoint, *arguments], stdin=subprocess.DEVNULL,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=10, check=False)
    require(len(result.stdout) <= 1024 * 1024 and len(result.stderr) <= 1024 * 1024,
            "Docker read response exceeds its metadata bound")
    return result


def run(request, output):
    require(platform.system() == "Linux" and request["target"]["platform"] == "linux-portable"
            and request["target"]["kind"] == "local", "This adapter qualifies local Linux portable execution")
    configuration = request["configuration"]
    require(set(configuration) == {"artifact", "image", "endpoint"}, "Select the artifact, cached image and local endpoint explicitly")
    require(configuration["endpoint"] == "unix:///var/run/docker.sock", "Only the explicitly selected local Docker endpoint is admitted")
    image = configuration["image"]
    require(isinstance(image, str) and re.fullmatch("sha256:[0-9a-f]{64}", image), "Select an immutable cached image ID")
    artifact = Path(configuration["artifact"])
    require(artifact.is_absolute() and digest_file(artifact) == request["target"]["artifact_sha256"], "Selected CLI artifact changed")
    require(all(case["id"] == "workflow.containers" and case["scenario"] == "happy"
                and case["layer"] == "installed_e2e" and set(case["required_assertions"]) == ASSERTIONS
                for case in request["cases"]), "Unsupported Docker qualification case")
    # Read-back admission happens before creating a workflow or container.
    try:
        inspected = docker_read(configuration["endpoint"], ["image", "inspect", "--format", "{{.Id}}", image])
    except (FileNotFoundError, subprocess.TimeoutExpired):
        blocked_prerequisite(request, output, "Selected local Docker command/daemon is unavailable; no container or installation was started")
        return
    if inspected.returncode != 0 or inspected.stdout.decode().strip() != image:
        blocked_prerequisite(request, output,
                             "Pinned cached image cannot be admitted on the selected local daemon; check its permissions, connection and image prerequisite. No image download or container was started")
        return
    output.mkdir(mode=0o700, parents=True)
    installation = output / "install/bin"
    installation.mkdir(mode=0o700, parents=True)
    cli = installation / "dockpipe"
    shutil.copyfile(artifact, cli)
    cli.chmod(0o700)
    require(digest_file(cli) == request["target"]["artifact_sha256"], "Installed CLI differs from its admitted artifact")
    workspace = output / "workspace"
    workspace.mkdir(mode=0o700)
    (workspace / "dockpipe.config.json").write_text("{}\n")
    (workspace / "input.txt").write_text("generated-container-input-v1")
    workflow = workspace / "workflows/offline-container"
    workflow.mkdir(parents=True)
    (workflow / "config.yml").write_text(
        "name: offline-container\nvault: none\nruntime: dockerimage\nisolate: " + image + "\n"
        "security:\n  profile: secure-default\n  network:\n    mode: offline\n"
        "  filesystem:\n    root: readonly\n    writes: workspace-only\n"
        "  process:\n    user: non-root\n    pid_limit: 64\n"
        "    resources:\n      cpu: '1'\n      memory: 128m\n"
        "steps:\n  - id: container-proof\n    cwd: repo\n"
        "    vars:\n      PRODUCT_FIXTURE: generated-container-environment-v1\n"
        "    cmd: bash workflows/offline-container/proof.sh\n")
    (workflow / "proof.sh").write_text(
        "set -eu\n"
        "test \"$(cat input.txt)\" = generated-container-input-v1\n"
        "test \"$PRODUCT_FIXTURE\" = generated-container-environment-v1\n"
        "id -u > effective-uid.txt\n"
        "awk '/^NoNewPrivs:/{print $2}' /proc/self/status > no-new-privileges.txt\n"
        "printf '%s' generated-container-output-v1 > output.txt\n"
        "printf '%s\\n' generated-container-stdout-v1\n"
        "printf '%s' ready > ready.txt\n"
        "sleep 4\n")
    home = output / "home"
    home.mkdir(mode=0o700)
    environment = {key: os.environ[key] for key in ("PATH", "LANG", "LC_ALL", "TZ") if key in os.environ}
    environment.update(HOME=str(home), DOCKER_HOST=configuration["endpoint"], DOCKPIPE_INTERACTIVE="0")
    for name in ("data", "config", "cache", "state"):
        directory = home / name
        directory.mkdir(mode=0o700)
        environment["XDG_" + name.upper() + "_HOME"] = str(directory)
    observations = {"image": image, "endpoint": configuration["endpoint"], "container_id": None}
    last_poll = 0.0

    def observe():
        nonlocal last_poll
        if observations["container_id"] is not None or not (workspace / "ready.txt").is_file():
            return
        if time.monotonic() - last_poll < 0.5:
            return
        last_poll = time.monotonic()
        listing = docker_read(configuration["endpoint"], ["ps", "--no-trunc", "--quiet", "--filter", "ancestor=" + image])
        require(listing.returncode == 0, "Cannot read back the selected daemon's running fixtures")
        matches = []
        for identifier in listing.stdout.decode().splitlines():
            require(re.fullmatch("[0-9a-f]{64}", identifier), "Docker returned an invalid container identity")
            result = docker_read(configuration["endpoint"], ["inspect", identifier])
            if result.returncode != 0:
                continue
            facts = json.loads(result.stdout)[0]
            if any(mount.get("Source") == str(workspace) for mount in facts.get("Mounts", [])):
                matches.append(facts)
        require(len(matches) <= 1, "Fixture container identity is ambiguous; no cleanup was guessed")
        if matches:
            facts = matches[0]
            observations.update(container_id=facts["Id"], observed_image=facts["Image"],
                                network=facts["HostConfig"]["NetworkMode"],
                                readonly_root=facts["HostConfig"]["ReadonlyRootfs"],
                                memory_bytes=facts["HostConfig"]["Memory"],
                                cpu_nanoseconds=facts["HostConfig"].get("NanoCpus", 0),
                                pid_limit=facts["HostConfig"].get("PidsLimit"),
                                security_options=facts["HostConfig"].get("SecurityOpt", []),
                                dropped_capabilities=facts["HostConfig"].get("CapDrop", []),
                                process_user=facts["Config"]["User"])
            write_json(output / "captured-container.json", observations)

    logs = output / "workflow"
    logs.mkdir(mode=0o700)
    execution = run_driver([str(cli), "--no-data", "--workflow", "offline-container"], logs, 90, environment,
                           workspace, observe)
    write_json(logs / "process.json", execution)
    identifier = observations["container_id"]
    removed = False
    if identifier is not None:
        readback = docker_read(configuration["endpoint"], ["inspect", identifier])
        removed = readback.returncode != 0 and b"no such object" in readback.stderr.lower()
        (output / "cleanup-readback.txt").write_bytes(readback.stderr)
    marker = workspace / "output.txt"
    exact = marker.is_file() and marker.read_text() == "generated-container-output-v1"
    stdout = (logs / "driver.stdout").read_text(errors="replace")
    successful = execution["exit_code"] == 0 and not execution["failure"]
    uid_path = workspace / "effective-uid.txt"
    observations["effective_uid"] = read_fixture_integer(uid_path)
    privilege_path = workspace / "no-new-privileges.txt"
    observations["kernel_no_new_privileges"] = read_fixture_integer(privilege_path)
    passed = {"declared-image-ran": successful and observations.get("observed_image") == image,
              "mount-env-output-binding": successful and exact and "generated-container-stdout-v1" in stdout,
              "owned-container-cleanup": removed,
              "declared-policy-enforced": declared_policy_observed(observations)}
    observations.update(execution=execution, assertions=passed,
                        cleanup_note="Exact observed container was read back after normal Dockpipe cleanup; no direct Docker removal was used")
    write_json(output / "observations.json", observations)
    paths = [output / "observations.json", workflow / "config.yml", workflow / "proof.sh", *logs.iterdir()]
    for name in ("captured-container.json", "cleanup-readback.txt"):
        if (output / name).is_file():
            paths.append(output / name)
    paths.extend(path for path in (uid_path, privilege_path) if path.is_file())
    evidence = [{"path": path.relative_to(output).as_posix(), "sha256": digest_file(path)} for path in paths]
    rows = []
    for case in request["cases"]:
        row = {key: case[key] for key in ("id", "scenario", "layer")}
        row.update(status="passed" if all(passed.values()) else "failed", evidence=evidence,
                   reason="Actual pinned offline workflow and exact container identity/cleanup observed",
                   assertions=[{"id": name, "status": "passed" if passed[name] else "failed",
                                "detail": "See captured-container.json, workflow logs and cleanup read-back"}
                               for name in case["required_assertions"]])
        rows.append(row)
    receipt = {key: request[key] for key in ("run_id", "plan_sha256", "registry_sha256", "target")}
    receipt.update(schema="dockpipe.product-integration.lane/v1", cases=rows)
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
