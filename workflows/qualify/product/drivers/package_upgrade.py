"""Qualify installed CLI package upgrades in version-pinned disposable Linux containers."""
from pathlib import Path
import argparse
import os
import re
import subprocess
import sys
import uuid


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "assets/scripts"))
from contracts import digest_file, read_json, require, write_json
from docker import blocked_prerequisite, docker_read
from process import run_driver


ENDPOINT = "unix:///var/run/docker.sock"
FIXTURE = ROOT / "fixtures/package-upgrade/cli.sh"
PLATFORMS = {
    "alpine-linux-3.24.4-amd64": ("alpine", "3.24.4", "apk"),
    "rhel-ubi-9.6-amd64": ("rhel", "9.6", "rpm"),
    "rocky-linux-9.6-amd64": ("rocky", "9.6", "rpm"),
    "amazon-linux-2023.11.20260526-amd64": ("amzn", "2023.11.20260526", "rpm"),
}
ASSERTIONS = {
    "install.cli": {
        "installed-cli-version", "initialized-project", "host-workflow-executed",
    },
    "install.upgrade": {
        "prior-version-installed", "prior-workflow-executed", "new-version-installed",
        "project-state-retained", "upgraded-workflow-executed",
    },
}


def absent_container(observation):
    return observation.returncode != 0 and b"No such container" in observation.stderr


def clean_container(name):
    template = '{{index .Config.Labels "org.dockpipe.qualification"}}'
    seen = docker_read(ENDPOINT, ["container", "inspect", "--format", template, name])
    if absent_container(seen):
        return True

    require(seen.returncode == 0 and seen.stdout.decode().strip() == name,
            "Owned container identity could not be verified for cleanup")
    stopped = docker_read(ENDPOINT, ["container", "stop", "--time=3", name])
    require(stopped.returncode == 0 or absent_container(stopped), "Owned container could not be stopped")
    return absent_container(docker_read(ENDPOINT, ["container", "inspect", name]))


def run(request, output):
    target = request["target"]
    require(target["kind"] == "container" and target["platform"] in PLATFORMS,
            "This driver requires an explicit supported Linux package container profile")
    os_id, release, extension = PLATFORMS[target["platform"]]
    config = request["configuration"]
    require(set(config) == {"image", "endpoint", "artifact", "version", "prior_artifact",
                            "prior_version", "prior_artifact_sha256"} and config["endpoint"] == ENDPOINT,
            "Select the local Docker endpoint, cached image, and pinned package pair")
    image = config["image"]
    require(isinstance(image, str) and re.fullmatch(r"sha256:[0-9a-f]{64}", image),
            "Select an immutable cached Docker image ID")
    versions = config["prior_version"], config["version"]
    require(all(isinstance(value, str) and re.fullmatch(r"[0-9]{1,8}(?:\.[0-9]{1,8}){2}", value)
                for value in versions), "Select two explicit numeric Dockpipe releases")
    prior, current = versions
    require(tuple(map(int, prior.split("."))) < tuple(map(int, current.split("."))),
            "The candidate must be newer than the upgrade baseline")

    inputs = [(Path(config["prior_artifact"]), config["prior_artifact_sha256"]),
              (Path(config["artifact"]), target["artifact_sha256"])]
    for artifact, expected in inputs:
        require(artifact.is_absolute() and "," not in str(artifact)
                and digest_file(artifact) == expected, "Package input differs from its pin")
    require(inputs[0][1] != inputs[1][1], "Upgrade artifacts must be distinct")
    require(all(case["id"] in ASSERTIONS and case["scenario"] == "happy"
                and case["layer"] == "installed_e2e"
                and set(case["required_assertions"]) == ASSERTIONS[case["id"]]
                for case in request["cases"]), "Unsupported package upgrade case")

    try:
        inspected = docker_read(ENDPOINT, [
            "image", "inspect", "--format", "{{.Id}} {{.Os}} {{.Architecture}}", image,
        ])
    except (FileNotFoundError, subprocess.TimeoutExpired):
        blocked_prerequisite(request, output, "The selected local Docker daemon is unavailable")
        return
    if inspected.returncode != 0 or inspected.stdout.decode().strip() != f"{image} linux amd64":
        blocked_prerequisite(request, output, "The pinned Linux amd64 image is unavailable on the selected daemon")
        return

    output.mkdir(mode=0o700, parents=True)
    cidfile = output / "container.id"
    name = "dockpipe-qualification-" + uuid.uuid4().hex
    command = [
        "docker", "--host", ENDPOINT, "run", "--pull=never", "--rm", "--name", name,
        "--label", "org.dockpipe.qualification=" + name, "--cidfile", str(cidfile),
        "--memory=768m", "--cpus=1", "--pids-limit=160", "--entrypoint", "/bin/sh",
    ]
    mounts = [(inputs[0][0], f"previous.{extension}"), (inputs[1][0], f"current.{extension}"),
              (FIXTURE, "cli.sh")]
    for source, destination in mounts:
        command.extend(["--mount", f"type=bind,source={source},target=/fixture/{destination},readonly"])
    command.extend([image, "/fixture/cli.sh", "install", os_id, release, prior, current,
                    inputs[0][1], inputs[1][1]])
    try:
        execution = run_driver(command, output, 240, os.environ)
    finally:
        removed = clean_container(name)
    require(removed, "The owned package-test container remains present")
    require(all(digest_file(artifact) == expected for artifact, expected in inputs),
            "Package artifact identity changed during the run")
    executed = execution["exit_code"] == 0 and not execution["failure"] and cidfile.is_file()

    write_json(output / "observations.json", {
        "image": image, "os": os_id, "release": release, "architecture": "amd64",
        "prior_version": prior, "version": current, "prior_artifact_sha256": inputs[0][1],
        "artifact_sha256": inputs[1][1], "container_removed": removed, "process": execution,
    })
    evidence = [{"path": name, "sha256": digest_file(output / name)}
                for name in ("observations.json", "driver.stdout", "driver.stderr")]
    rows = []
    for case in request["cases"]:
        row = {key: case[key] for key in ("id", "scenario", "layer")}
        row.update(
            status="passed" if executed else "failed",
            reason="" if executed else "The package lifecycle check did not complete. See the installer and workflow logs.",
            assertions=[{"id": name, "status": "passed", "detail": "Observed by the installed package fixture"}
                        for name in case["required_assertions"]] if executed else [],
            evidence=evidence,
        )
        rows.append(row)
    result = {key: request[key] for key in ("run_id", "plan_sha256", "registry_sha256", "target")}
    result.update(schema="dockpipe.product-integration.lane/v1", cases=rows)
    write_json(output / "result.json", result)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--request", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    arguments = parser.parse_args()
    run(read_json(arguments.request), arguments.output)
