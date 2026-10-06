#!/usr/bin/env python3
"""Deliver sources absent from the worker through real CLI processes on loopback."""

import json
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import time


def main():
    binary = str(Path(sys.argv[1]).resolve())
    with tempfile.TemporaryDirectory(prefix="dockpipe-delivery-smoke-") as temporary:
        root = Path(temporary)
        sender = root / "sender"
        worker_home = root / "empty-worker"
        worker_home.mkdir()
        broker_state = root / "broker"
        worker_state = root / "worker"
        workflow = sender / "workflows" / "delivered"
        (workflow / "assets").mkdir(parents=True)
        (workflow / "config.yml").write_text(
            "name: delivered\nvault: none\ndocker_preflight: false\nsteps:\n"
            "  - id: source\n    kind: host\n    cwd: repo\n    run: assets/run.sh\n"
            "  - id: dependency\n    workflow: helper\n    package: delivery-example\n"
        )
        (workflow / "assets" / "run.sh").write_text(
            "#!/usr/bin/env bash\nset -euo pipefail\nmkdir -p results\n"
            "cat inputs/value.txt > results/source.txt\nprintf x >> invocations\n"
            "printf 'delivered source executed\\n'\n"
        )
        (workflow / "assets" / "run.sh").chmod(0o700)
        (sender / "inputs").mkdir()
        (sender / "inputs" / "value.txt").write_text("42\n")
        (sender / ".env").write_text("MUST_NOT_COPY=private\n")
        dependency = sender / "helper"
        (dependency / "assets").mkdir(parents=True)
        (dependency / "package.yml").write_text("name: helper\nkind: workflow\nversion: 1.0.0\n")
        (dependency / "config.yml").write_text(
            "name: helper\nnamespace: delivery-example\nvault: none\ndocker_preflight: false\n"
            "steps:\n  - id: helper\n    kind: host\n    cwd: repo\n    run: assets/helper.sh\n"
        )
        (dependency / "assets" / "helper.sh").write_text(
            "#!/usr/bin/env bash\nset -euo pipefail\n"
            "printf 'package arrived\\n' > results/dependency.txt\n"
        )
        (dependency / "assets" / "helper.sh").chmod(0o700)

        def run(command, state=broker_state, *arguments, check=True):
            return subprocess.run(
                [binary, "remote", command, "--state", str(state), *arguments],
                cwd=worker_home, text=True, capture_output=True, timeout=20, check=check,
            )

        with socket.socket() as listener:
            listener.bind(("127.0.0.1", 0))
            port = listener.getsockname()[1]
        run("init", broker_state, "--listen", f"127.0.0.1:{port}")
        processes = []
        with (root / "services.log").open("w") as log:
            try:
                processes.append(subprocess.Popen(
                    [binary, "remote", "serve", "--state", str(broker_state)],
                    cwd=worker_home, stdout=log, stderr=log,
                ))
                deadline = time.monotonic() + 10
                while run("jobs", check=False).returncode:
                    if time.monotonic() > deadline:
                        raise AssertionError("broker did not start")
                    time.sleep(0.05)
                invitation = root / "invitation.json"
                run("invite", broker_state, "--node", "worker", "--out", str(invitation))
                paired = run("pair", worker_state, "--invite", str(invitation), "--allow-delivery", "--timeout", "30")
                assert "Delivery enabled" in paired.stderr
                assert "Next:" in paired.stderr
                run("pair", worker_state, "--invite", str(invitation), "--allow-delivery", "--timeout", "30")
                changed_authority = run("pair", worker_state, "--invite", str(invitation), "--allow-delivery", "--timeout", "31", check=False)
                assert changed_authority.returncode != 0
                arguments = [
                    "--node", "worker", "--id", "delivery-1", "--workdir", str(sender),
                    "--workflow-file", "workflows/delivered/config.yml",
                    "--include", "inputs/value.txt", "--dependency", "helper",
                    "--artifact", "results/source.txt", "--artifact", "results/dependency.txt",
                ]
                preview = json.loads(run("submit", broker_state, *arguments, "--dry-run").stdout)
                assert len(preview["files"]) == 6, preview
                assert ".env" not in preview["files"]
                assert json.loads(run("jobs").stdout) == []
                first = json.loads(run("submit", broker_state, *arguments).stdout)
                repeated = json.loads(run("submit", broker_state, *arguments).stdout)
                assert first == repeated
                (sender / "inputs/value.txt").write_text("changed\n")
                assert run("submit", broker_state, *arguments, check=False).returncode != 0
                # Remove the sender's original paths before any worker execution.
                sender.rename(root / "offline-sender")
                assert not sender.exists()
                assert not (worker_state / "deliveries").exists()
                processes.append(subprocess.Popen(
                    [binary, "remote", "worker", "--state", str(worker_state)],
                    cwd=worker_home, stdout=log, stderr=log,
                ))
                deadline = time.monotonic() + 30
                while True:
                    job = json.loads(run("result", broker_state, "--id", "delivery-1").stdout)
                    if job["status"] not in ("queued", "running"):
                        break
                    if time.monotonic() > deadline:
                        raise AssertionError("delivery timed out")
                    time.sleep(0.05)
                output = root / "download"
                run("result", broker_state, "--id", "delivery-1", "--out", str(output))
                receipt = json.loads((output / "result.json").read_text())
                assert receipt["status"] == "success", receipt
                assert (output / "artifacts/results/source.txt").read_text() == "42\n"
                assert (output / "artifacts/results/dependency.txt").read_text() == "package arrived\n"
                assert "delivered source executed" in (output / "workflow.log").read_text()
                stage = worker_state / "deliveries/delivery-1"
                assert (stage / "invocations").read_text() == "x"
                assert not (stage / ".env").exists()
                print("PASS: absent workflow, assets, extra input, package dependency, opt-in pairing, digest conflict, logs and results")
            finally:
                for process in reversed(processes):
                    process.terminate()
                for process in processes:
                    try:
                        process.wait(timeout=10)
                    except subprocess.TimeoutExpired:
                        process.kill()
                        process.wait()


if __name__ == "__main__":
    main()
