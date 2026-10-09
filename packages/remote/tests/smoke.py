#!/usr/bin/env python3
"""Exercise real Dockpipe broker, worker, workflow, and results on loopback."""

import json
import os
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import time


def main():
    binary = str(Path(sys.argv[1]).resolve())
    with tempfile.TemporaryDirectory(prefix="dockpipe-remote-smoke-") as temporary:
        root = Path(temporary)
        broker_state = root / "broker"
        worker_state = root / "worker"
        checkout = root / "checkout"
        workflow = checkout / "workflows" / "bench"
        workflow.mkdir(parents=True)
        (workflow / "config.yml").write_text(
            "name: bench\ndocker_preflight: false\nsteps:\n"
            "  - id: benchmark\n    kind: host\n    cwd: repo\n"
            "    run: assets/bench.sh\n"
        )
        (workflow / "assets").mkdir()
        (workflow / "assets" / "bench.sh").write_text(
            "#!/usr/bin/env bash\nset -euo pipefail\n"
            "mkdir -p results\nprintf x >> invocations\n"
            "printf '{\"benchmark\":\"smoke\",\"value\":42}' > results/bench.json\n"
            "printf 'native workflow finished\\n'\n"
        )
        (workflow / "assets" / "bench.sh").chmod(0o700)
        profiles = root / "profiles.json"
        profiles.write_text(json.dumps({"bench": {
            "workdir": str(checkout), "workflow_file": str(workflow / "config.yml"), "timeout_seconds": 30,
            "artifacts": ["results/bench.json"],
        }}))
        with socket.socket() as listener:
            listener.bind(("127.0.0.1", 0))
            port = listener.getsockname()[1]

        def run(command, state=broker_state, *arguments):
            return subprocess.run(
                [binary, "remote", command, "--state", str(state), *arguments],
                cwd=checkout, text=True, capture_output=True, timeout=15, check=True,
            )

        run("init", broker_state, "--listen", f"127.0.0.1:{port}")
        processes = []
        with (root / "services.log").open("w") as log:
            try:
                processes.append(subprocess.Popen(
                    [binary, "remote", "serve", "--state", str(broker_state)],
                    cwd=checkout, stdout=log, stderr=log,
                ))
                deadline = time.monotonic() + 10
                while True:
                    try:
                        run("jobs")
                        break
                    except subprocess.CalledProcessError:
                        if time.monotonic() > deadline:
                            raise
                        time.sleep(0.05)
                invitation = root / "invitation.json"
                run("invite", broker_state, "--node", "mac", "--out", str(invitation))
                run("pair", worker_state, "--invite", str(invitation), "--profiles", str(profiles))
                processes.append(subprocess.Popen(
                    [binary, "remote", "worker", "--state", str(worker_state)],
                    cwd=checkout, stdout=log, stderr=log,
                ))
                run("submit", broker_state, "--node", "mac", "--profile", "bench", "--id", "smoke-1")
                deadline = time.monotonic() + 30
                while True:
                    job = json.loads(run("result", broker_state, "--id", "smoke-1").stdout)
                    if job["status"] not in ("queued", "running"):
                        break
                    if time.monotonic() > deadline:
                        raise AssertionError("workflow timed out")
                    time.sleep(0.05)
                output = root / "download"
                run("result", broker_state, "--id", "smoke-1", "--out", str(output))
                receipt = json.loads((output / "result.json").read_text())
                assert receipt["status"] == "success", receipt
                assert json.loads((output / "artifacts" / "results" / "bench.json").read_text())["value"] == 42
                assert "native workflow finished" in (output / "workflow.log").read_text()
                run("submit", broker_state, "--node", "mac", "--profile", "bench", "--id", "smoke-1")
                assert (checkout / "invocations").read_text() == "x"
                print("PASS: real CLI pairing, outbound worker, native workflow, artifact download, idempotent submission")
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
