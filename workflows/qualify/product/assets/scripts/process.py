"""Bounded local driver processes with private output and owned-tree cancellation."""
from pathlib import Path
import os
import signal
import subprocess
import time


MAX_LOG_BYTES = 4 * 1024 * 1024
TERMINATION_GRACE_SECONDS = 10


def terminate_owned(process):
    if process.returncode is not None:
        return
    if os.name == "nt":
        if process.poll() is not None:
            return
        subprocess.run(["taskkill.exe", "/PID", str(process.pid), "/T", "/F"],
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=15, check=False)
        process.wait(timeout=5)
        return

    try:
        os.killpg(process.pid, signal.SIGTERM)
        deadline = time.monotonic() + TERMINATION_GRACE_SECONDS
        # Keep the leader unreaped until the final group signal. Waiting only
        # for the leader can leave descendants alive or release its group ID.
        while time.monotonic() < deadline:
            if hasattr(os, "waitid") and os.waitid(
                    os.P_PID, process.pid, os.WEXITED | os.WNOHANG | os.WNOWAIT) is not None:
                break
            time.sleep(0.05)
        os.killpg(process.pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    process.wait(timeout=5)


def run_driver(command, output, timeout, environment, directory=None, observe=None):
    output = Path(output)
    stdout_path = output / "driver.stdout"
    stderr_path = output / "driver.stderr"
    with stdout_path.open("xb") as stdout, stderr_path.open("xb") as stderr:
        stdout_path.chmod(0o600)
        stderr_path.chmod(0o600)
        options = {"start_new_session": True} if os.name != "nt" else {
            "creationflags": subprocess.CREATE_NEW_PROCESS_GROUP}
        process = subprocess.Popen(command, stdin=subprocess.DEVNULL, stdout=stdout, stderr=stderr,
                                   env=environment, cwd=directory or output, **options)
        deadline = time.monotonic() + timeout
        reason = ""
        try:
            while process.poll() is None:
                if observe is not None:
                    observe()
                if stdout_path.stat().st_size > MAX_LOG_BYTES or stderr_path.stat().st_size > MAX_LOG_BYTES:
                    reason = "Driver log exceeded its 4 MiB per-stream bound"
                    break
                if time.monotonic() >= deadline:
                    reason = "Driver exceeded its declared deadline"
                    break
                time.sleep(0.05)
            if observe is not None:
                observe()
        finally:
            if process.returncode is None:
                terminate_owned(process)
        if stdout_path.stat().st_size > MAX_LOG_BYTES or stderr_path.stat().st_size > MAX_LOG_BYTES:
            reason = "Driver log exceeded its 4 MiB per-stream bound"
        if reason:
            return {"exit_code": process.returncode, "failure": reason}
        return {"exit_code": process.returncode, "failure": ""}
