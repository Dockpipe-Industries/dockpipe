"""Manual local missing-manifest regression; never contacts a real Docker daemon."""
from pathlib import Path
import argparse
import os
import platform
import shutil
import socket
import stat
import sys
import tempfile
import threading
import time


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "assets/scripts"))
from contracts import digest_file, read_json, require, write_json
from process import MAX_LOG_BYTES, run_driver


ASSERTIONS = {"expected-failure-observed", "no-unowned-effects", "actionable-result"}
WORKFLOW_DIAGNOSTIC = (
    "container workflow declares security policy but has no compiled runtime manifest; "
    "compile the workflow and run its compiled catalog entry")
STEP_DIAGNOSTIC = (
    'container step "guarded-container" declares security policy but has no compiled runtime manifest; '
    "compile the workflow and run its compiled catalog entry")


class SocketPrerequisiteUnavailable(PermissionError):
    """The environment denied a private AF_UNIX allocation, bind or connection."""


class FakeDockerEndpoint:
    """Count connections to one private rejecting socket, then remove its exact inode."""

    def __init__(self):
        self.directory = Path(tempfile.mkdtemp(prefix="dp-policy-"))
        self.directory.chmod(0o700)
        self.directory_identity = self.identity(self.directory)
        self.path = self.directory / "docker.sock"
        self.listener = None
        self.connections = 0
        self.error = ""
        self.stopping = threading.Event()
        self.thread = None
        self.socket_identity = None
        try:
            try:
                self.listener = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
                self.listener.bind(str(self.path))
            except PermissionError as error:
                raise SocketPrerequisiteUnavailable("Private AF_UNIX allocation or bind denied") from error
            self.path.chmod(0o600)
            self.socket_identity = self.identity(self.path)
            self.listener.listen(32)
            self.listener.settimeout(0.05)
            self.thread = threading.Thread(target=self.listen, name="private-docker-counter", daemon=True)
            self.thread.start()
        except BaseException:
            self.close()
            raise

    @staticmethod
    def identity(path):
        information = path.lstat()
        return [information.st_dev, information.st_ino]

    def listen(self):
        try:
            while True:
                try:
                    connection, _ = self.listener.accept()
                except socket.timeout:
                    if self.stopping.is_set():
                        return
                    continue
                connection.close()
                self.connections += 1
                if self.connections > 32:
                    self.error = "Connection counter exceeded its fixture bound"
                    return
        except OSError as error:
            self.error = type(error).__name__ + ": " + str(error)

    def close(self):
        self.stopping.set()
        if self.thread is not None:
            self.thread.join(timeout=2)
            require(not self.thread.is_alive(), "Private endpoint listener did not stop")
        if self.listener is not None:
            self.listener.close()
        if self.socket_identity is not None:
            require(self.identity(self.path) == self.socket_identity and stat.S_ISSOCK(self.path.lstat().st_mode),
                    "Private endpoint socket identity changed; refusing cleanup")
            self.path.unlink()
        require(self.identity(self.directory) == self.directory_identity,
                "Private endpoint directory identity changed; refusing cleanup")
        self.directory.rmdir()
        return {"path": str(self.path), "directory_identity": self.directory_identity,
                "socket_identity": self.socket_identity, "connections": self.connections,
                "listener_error": self.error, "listener_stopped": self.thread is None or not self.thread.is_alive(),
                "socket_removed": not self.path.exists(), "directory_removed": not self.directory.exists()}


def endpoint_self_test():
    endpoint = FakeDockerEndpoint()
    try:
        try:
            with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as client:
                client.connect(str(endpoint.path))
        except PermissionError as error:
            raise SocketPrerequisiteUnavailable("Private AF_UNIX self-test connection denied") from error
        deadline = time.monotonic() + 2
        while endpoint.connections == 0 and time.monotonic() < deadline:
            time.sleep(0.01)
    finally:
        facts = endpoint.close()
    require(facts["connections"] == 1 and endpoint_clean(facts), "Private endpoint counter self-test failed")
    return facts


def endpoint_clean(facts):
    return (facts.get("listener_error") == "" and facts.get("listener_stopped") is True
            and facts.get("socket_removed") is True and facts.get("directory_removed") is True)


def expected_refusal(execution, stderr_path, diagnostic):
    if (not isinstance(execution, dict) or type(execution.get("exit_code")) is not int
            or execution["exit_code"] != 1 or execution.get("failure") != ""):
        return False
    with stderr_path.open("rb") as stream:
        contents = stream.read(MAX_LOG_BYTES + 1)
    if len(contents) > MAX_LOG_BYTES:
        return False
    try:
        lines = contents.decode("utf-8").strip().splitlines()
    except UnicodeDecodeError:
        return False
    return bool(lines) and lines[-1] == diagnostic


def fixture_source(level):
    header = "name: declared-policy\nvault: none\ndocker_preflight: false\n"
    image = "sha256:" + "1" * 64
    if level == "workflow":
        return (header + "runtime: dockerimage\nisolate: " + image
                + "\nsecurity:\n  network:\n    mode: offline\nsteps: []\n")
    require(level == "step", "Unknown fixture policy level")
    return (header + "steps:\n  - id: guarded-container\n    runtime: dockerimage\n    isolate: " + image
            + "\n    security:\n      network:\n        mode: offline\n"
            + "    cmd: printf unexpected-container-effect > effect-marker\n")


def protected_inventory(root):
    result = {}
    for path in sorted(root.rglob("*")):
        information = path.lstat()
        require(stat.S_ISREG(information.st_mode) or stat.S_ISDIR(information.st_mode),
                "Unexpected special file in protected fixture")
        relative = path.relative_to(root).as_posix()
        result[relative] = digest_file(path) if path.is_file() else "directory"
    return result


def authored_fixture_unchanged(before, after):
    """Preserve every original entry; admit only task-local generated bin additions."""
    if any(after.get(path) != value for path, value in before.items()):
        return False
    additions = set(after) - set(before)
    return all(path == "workspace/bin" or path.startswith("workspace/bin/") for path in additions)


def observed_assertions(variants, endpoint, docker_invoked, cli_unchanged):
    refused = len(variants) == 4 and all(row.get("expected_refusal") is True for row in variants)
    confined = (len(variants) == 4 and all(row.get("protected_fixture_unchanged") is True for row in variants)
                and type(endpoint.get("connections")) is int and endpoint["connections"] == 0
                and endpoint_clean(endpoint) and not docker_invoked and cli_unchanged)
    return {"expected-failure-observed": refused, "no-unowned-effects": confined,
            "actionable-result": refused}


def blocked_prerequisite(request, output):
    reason = ("Private AF_UNIX socket capability is unavailable; run this manual lane in an environment "
              "that permits its task-owned socket. No CLI workflow, Docker command or container was started.")
    print("WARNING: " + reason, file=sys.stderr, flush=True)
    rows = []
    for case in request["cases"]:
        row = {key: case[key] for key in ("id", "scenario", "layer")}
        row.update(status="blocked", reason=reason, assertions=[], evidence=[])
        rows.append(row)
    receipt = {key: request[key] for key in ("run_id", "plan_sha256", "registry_sha256", "target")}
    receipt.update(schema="dockpipe.product-integration.lane/v1", cases=rows,
                   observed={"prerequisite": "private-af-unix-socket", "cli_workflows_started": 0,
                             "docker_commands_started": 0, "real_docker_daemon_used": False})
    write_json(output / "result.json", receipt)


def run(request, output):
    target = request["target"]
    require(platform.system() == "Linux" and target["platform"] == "linux-portable" and target["kind"] == "local",
            "Policy failure driver currently qualifies only a local Linux portable CLI")
    configuration = request["configuration"]
    require(set(configuration) == {"artifact"} and isinstance(configuration["artifact"], str),
            "Select exactly one pinned CLI artifact")
    artifact = Path(configuration["artifact"])
    require(artifact.is_absolute() and digest_file(artifact) == target["artifact_sha256"], "CLI artifact differs from its pin")
    require(bool(request["cases"]) and all(case["id"] == "workflow.containers" and case["scenario"] == "failure"
            and case["layer"] == "component" and set(case["required_assertions"]) == ASSERTIONS
            for case in request["cases"]), "Unsupported policy failure case")
    output.mkdir(mode=0o700, parents=True)
    cli = output / "dockpipe"
    shutil.copyfile(artifact, cli)
    cli.chmod(0o700)
    require(digest_file(cli) == target["artifact_sha256"], "Copied CLI differs from its pin")
    home = output / "home"
    home.mkdir(mode=0o700)
    environment = {"PATH": "/usr/bin:/bin", "LANG": "C.UTF-8", "LC_ALL": "C.UTF-8", "HOME": str(home),
                   "DOCKPIPE_INTERACTIVE": "0", "DOCKPIPE_VAULT_INJECT": "0"}
    for name in ("config", "cache", "data", "state"):
        directory = home / name
        directory.mkdir(mode=0o700)
        environment["XDG_" + name.upper() + "_HOME"] = str(directory)
    temporary = home / "tmp"
    temporary.mkdir(mode=0o700)
    environment["TMPDIR"] = str(temporary)
    docker_config = home / "docker"
    docker_config.mkdir(mode=0o700)
    environment["DOCKER_CONFIG"] = str(docker_config)
    marker = output / "docker-invoked"
    docker = output / "rejecting-docker"
    docker.write_text(
        "#!/usr/bin/python3 -I\n"
        "import os\nimport socket\nfrom pathlib import Path\n"
        "Path(os.environ['POLICY_DOCKER_MARKER']).write_bytes(b'invoked\\n')\n"
        "with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as client:\n"
        "    client.settimeout(1)\n"
        "    client.connect(os.environ['DOCKER_HOST'].removeprefix('unix://'))\n"
        "raise SystemExit(2)\n")
    docker.chmod(0o700)
    environment.update(DOCKPIPE_DOCKER_BIN=str(docker), POLICY_DOCKER_MARKER=str(marker))
    try:
        self_test = endpoint_self_test()
        endpoint = FakeDockerEndpoint()
    except SocketPrerequisiteUnavailable:
        blocked_prerequisite(request, output)
        return
    environment["DOCKER_HOST"] = "unix://" + str(endpoint.path)
    variants = []
    endpoint_journal = output / "endpoint-created.json"
    write_json(endpoint_journal, {"path": str(endpoint.path), "directory_identity": endpoint.directory_identity,
                                  "socket_identity": endpoint.socket_identity})
    proof_paths = [docker, endpoint_journal]
    try:
        for level in ("workflow", "step"):
            for selection in ("file", "named"):
                name = level + "-" + selection
                directory = output / name
                directory.mkdir(mode=0o700)
                protected = directory / "protected"
                workspace = protected / "workspace"
                source = workspace / "workflows/declared-policy"
                source.mkdir(mode=0o700, parents=True)
                (workspace / "dockpipe.config.json").write_text("{}\n")
                canary = protected / "outside-canary.txt"
                canary.write_bytes(b"generated adjacent canary must remain unchanged\r\n")
                configuration_path = source / "config.yml"
                configuration_path.write_text(fixture_source(level))
                before = protected_inventory(protected)
                arguments = ["--workdir", str(workspace), "--no-vault", "--no-compile-deps"]
                if selection == "file":
                    arguments.extend(["--workflow-file", str(configuration_path)])
                else:
                    arguments.extend(["--workflow", "declared-policy"])
                arguments.extend(["--", "true"])
                logs = directory / "logs"
                logs.mkdir(mode=0o700)
                execution = run_driver([str(cli), *arguments], logs, 20, environment, workspace)
                write_json(logs / "process.json", execution)
                diagnostic = WORKFLOW_DIAGNOSTIC if level == "workflow" else STEP_DIAGNOSTIC
                after = protected_inventory(protected)
                row = {"level": level, "selection": selection, "arguments": arguments,
                       "execution": execution, "expected_diagnostic": diagnostic,
                       "expected_refusal": expected_refusal(execution, logs / "driver.stderr", diagnostic),
                       "protected_fixture_unchanged": authored_fixture_unchanged(before, after),
                       "protected_before": before, "protected_after": after,
                       "generated_bin_additions": sorted(set(after) - set(before))}
                write_json(directory / "observation.json", row)
                variants.append(row)
                proof_paths.extend([configuration_path, canary, directory / "observation.json", *sorted(logs.iterdir())])
    finally:
        endpoint_facts = endpoint.close()
    cli_unchanged = digest_file(cli) == target["artifact_sha256"] and digest_file(artifact) == target["artifact_sha256"]
    assertions = observed_assertions(variants, endpoint_facts, marker.exists(), cli_unchanged)
    facts = {"variants": variants, "endpoint": endpoint_facts, "endpoint_self_test": self_test,
             "docker_executable_invoked": marker.exists(), "cli_sha256": digest_file(cli),
             "cli_unchanged": cli_unchanged, "real_docker_daemon_used": False,
             "scope": "Missing compiled runtime policy refusal only; no container execution or cancellation qualification"}
    write_json(output / "observations.json", facts)
    proof_paths.append(output / "observations.json")
    if marker.exists():
        proof_paths.append(marker)
    proof = [{"path": path.relative_to(output).as_posix(), "sha256": digest_file(path)} for path in proof_paths]
    rows = []
    for case in request["cases"]:
        row = {key: case[key] for key in ("id", "scenario", "layer")}
        row.update(status="passed" if all(assertions.values()) else "failed",
                   reason="Authored workflow and step policy must refuse uncompiled execution before any Docker invocation",
                   assertions=[{"id": key, "status": "passed" if assertions[key] else "failed",
                                "detail": "See four exact CLI refusals, protected inventories and private endpoint counter/cleanup"}
                               for key in case["required_assertions"]], evidence=proof)
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
