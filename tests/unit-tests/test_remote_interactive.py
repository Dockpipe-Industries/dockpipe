#!/usr/bin/env python3
"""Check remote dependency prompts on a real PTY without installing anything."""

import errno
import os
from pathlib import Path
import pty
import select
import signal
import subprocess
import sys
import tempfile
import time


def read_terminal(descriptor, timeout):
    if not select.select([descriptor], [], [], timeout)[0]:
        return b""
    try:
        return os.read(descriptor, 65536)
    except OSError as error:
        if error.errno == errno.EIO:
            return b""
        raise


def wait_for(descriptor, expected, timeout=10):
    output = b""
    deadline = time.monotonic() + timeout
    while expected not in output and time.monotonic() < deadline:
        output += read_terminal(descriptor, 0.1)
    assert expected in output, output.decode(errors="replace")
    return output


def main():
    binary = str(Path(sys.argv[1]).resolve())
    with tempfile.TemporaryDirectory(prefix="dockpipe-interactive-test-") as temporary:
        root = Path(temporary)
        package = root / "packages/resolvers/prompt-fixture"
        package.mkdir(parents=True)
        installer = root / "installer.sh"
        installer.write_text(
            "#!/bin/sh\nset -eu\nstty -echo\ntrap 'stty echo' EXIT\n"
            "printf '[fixture] Password: '\nIFS= read -r answer\nprintf '\\n'\n"
            "[ \"$answer\" = 'fixture-input' ] || exit 24\nexit 23\n"
        )
        package.joinpath("profile").write_text("DOCKPIPE_REMOTE_EDGE_SETUP=unused.sh\n")
        package.joinpath("package.yml").write_text(
            "schema: 1\nkind: resolver\nname: prompt-fixture\nversion: 1.0.0\n"
            "dependencies:\n  host:\n    - id: dockpipe-missing-prompt-fixture\n"
            "      install:\n"
            f"        linux: sh {installer}\n        macos: sh {installer}\n"
        )
        environment = dict(os.environ)
        environment.pop("DOCKPIPE_APPROVE_PROMPTS", None)
        environment.pop("FLATPAK_ID", None)
        environment["DOCKPIPE_PACKAGES_ROOT"] = str(root / "packages")
        environment["DOCKPIPE_GLOBAL_ROOT"] = str(root / "global")
        for key in ("XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME"):
            environment[key] = str(root / key.lower())
        master, slave = pty.openpty()
        process = subprocess.Popen(
            [binary, "remote", "setup", "--resolver", "prompt-fixture",
             "--hostname", "fixture.invalid", "--state", str(root / "remote")],
            cwd=root, env=environment, stdin=slave, stdout=slave, stderr=slave,
            start_new_session=True,
        )
        os.close(slave)
        try:
            output = wait_for(master, b"Run this installer now? [y/N]: ")
            os.write(master, b"y\n")
            output += wait_for(master, b"[fixture] Password: ")
            # Exceed the old five-second heartbeat interval while input is pending.
            deadline = time.monotonic() + 6
            interference = b""
            while time.monotonic() < deadline:
                interference += read_terminal(master, 0.1)
            assert not interference, f"Output obscured the prompt: {interference!r}"
            assert process.poll() is None, "installer did not wait for input"
            os.write(master, b"fixture-input\n")
            output += wait_for(master, b"exit status 23")
            assert process.wait(timeout=10) != 0, "installer failure was swallowed"
            assert b"fixture-input" not in output, "password input was echoed"
            assert b"status=progress" not in output, output
            assert b"\r  Remote setup" not in output, output
            assert b"unit=dependency.host.install" in output, output
            assert b"status=fail" in output, output
        finally:
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait(timeout=10)
            os.close(master)
    print("PASS: approval, visible password prompt, no spinner/heartbeat interference, failure propagation")


if __name__ == "__main__":
    main()
