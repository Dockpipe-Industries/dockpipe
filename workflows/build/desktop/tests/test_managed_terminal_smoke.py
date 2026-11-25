"""Exercise the installed-terminal smoke against bounded protocol processes."""
import importlib.util
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import textwrap
import time
import unittest
from unittest.mock import patch


SCRIPT = Path(__file__).resolve().parents[1] / "assets/scripts/smoke.py"
SPEC = importlib.util.spec_from_file_location("managed_terminal_smoke", SCRIPT)
smoke = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(smoke)


class ManagedTerminalSmokeTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="dockpipe terminal smoke ")
        self.addCleanup(self.temporary.cleanup)
        self.home = Path(self.temporary.name)
        self.fixture = self.home / "fixture.py"
        self.fixture.write_text(textwrap.dedent("""
            import base64
            import json
            import os
            import sys
            import time

            mode = sys.argv[1]
            assert sys.argv[2:] == ["terminal", "--stdio"]
            assert "DOCKPIPE_TERMINAL_PROVIDER" not in os.environ
            assert "PRIVATE_AMBIENT_VALUE" not in os.environ
            schema = "dockpipe.local-terminal/v1"

            def receive(kind, identity):
                value = json.loads(sys.stdin.readline())
                assert value["schema"] == schema
                assert value["type"] == kind
                assert value["id"] == identity
                return value

            def emit(kind, **fields):
                print(json.dumps(dict(schema=schema, type=kind, **fields)), flush=True)

            if mode == "stalled":
                time.sleep(10)
                sys.exit(1)
            # Stderr must be drained or discarded, never a second unread PIPE.
            sys.stderr.write("fixture diagnostic\\n" * 16384)
            sys.stderr.flush()
            start = receive("start", 1)
            profile = start["profile"]
            assert profile["route"] == "native"
            assert profile["environment_mode"] == "complete"
            assert os.path.isabs(profile["executable"])
            assert profile["dir"] == os.getcwd()
            assert not any(value.startswith("DOCKPIPE_TERMINAL_PROVIDER=") for value in profile["env"])
            if mode == "missing-provider":
                emit("error", id=1, code="start_failed")
                emit("closed", cleanup="complete")
                sys.exit(0)
            if mode == "oversized-output":
                sys.stdout.write("x" * (100 * 1024))
                sys.stdout.flush()
                time.sleep(10)
                sys.exit(1)
            emit("ready", id=1, session_id="a" * 64)
            request = receive("input", 2)
            data = base64.b64decode(request["data"], validate=True)
            marker = b"__DOCKPIPE_DESKTOP_TERMINAL_SMOKE__"
            assert marker not in data
            if mode == "echo-only":
                emit("output", data=request["data"])
            else:
                # Split the marker across output chunks, independently of ACK.
                emit("output", data=base64.b64encode(marker[:17]).decode())
                emit("output", data=base64.b64encode(marker[17:] + b"\\r\\n").decode())
            count = len(data)
            if mode == "wrong-ack":
                count -= 1
            emit("ack", id=2, bytes=count)
            resize = receive("resize", 3)
            assert (resize["columns"], resize["rows"]) == (113, 37)
            emit("ack", id=3)
            receive("close", 4)
            if mode == "unknown-cleanup":
                emit("closed", cleanup="unknown")
            else:
                emit("closed", cleanup="complete")
            if mode == "exit-stalled":
                time.sleep(10)
        """))

    def run_fixture(self, mode, timeout=2):
        original = subprocess.Popen
        processes = []

        def launch(arguments, **options):
            self.assertEqual(arguments, [str(self.fixture.resolve()), "terminal", "--stdio"])
            self.assertEqual(options["stderr"], subprocess.DEVNULL)
            process = original([sys.executable, str(self.fixture), mode, *arguments[1:]], **options)
            processes.append(process)
            return process

        environment = dict(os.environ, DOCKPIPE_TERMINAL_PROVIDER="unqualified-provider",
                           PRIVATE_AMBIENT_VALUE="must-not-be-inherited")
        try:
            with patch.object(smoke.subprocess, "Popen", side_effect=launch):
                smoke.check_managed_terminal(self.fixture, self.home, environment, timeout=timeout)
        finally:
            self.assertEqual(len(processes), 1)
            self.assertIsNotNone(processes[0].poll(), "fixture process survived smoke completion")

    def test_actual_output_ack_resize_and_cleanup_required(self):
        self.run_fixture("success")

    @unittest.skipIf(os.name == "nt", "macOS bundle layout uses Unix executable names")
    def test_macos_startup_probe_finds_terminal_provider_beside_native_cli(self):
        contents = self.home / "DockPipe.app" / "Contents"
        executables = contents / "MacOS"
        helpers = contents / "Helpers"
        executables.mkdir(parents=True)
        helpers.mkdir()
        launcher = executables / "DockPipe"
        launcher.touch()
        (executables / "dockpipe-workspace-reader").touch()
        terminal = helpers / "dockpipe-terminal-host"
        terminal.touch()

        with patch.object(smoke.sys, "platform", "darwin"), patch.object(smoke.subprocess, "run") as run:
            run.return_value.returncode = 1
            with self.assertRaisesRegex(RuntimeError, "failed its no-session startup probe"):
                smoke.check(launcher, executables / "dockpipe-cli")

        self.assertEqual(run.call_args.args[0], [str(terminal.resolve())])

    def test_existing_cli_with_missing_provider_fails(self):
        with self.assertRaisesRegex(RuntimeError, "protocol smoke"):
            self.run_fixture("missing-provider")

    def test_unknown_cleanup_cannot_pass(self):
        with self.assertRaisesRegex(RuntimeError, "cleanup could not be verified"):
            self.run_fixture("unknown-cleanup")

    def test_changed_input_ack_cannot_pass(self):
        with self.assertRaisesRegex(RuntimeError, "acknowledgement changed"):
            self.run_fixture("wrong-ack")

    def test_oversized_output_is_bounded_and_does_not_block_cleanup(self):
        with self.assertRaisesRegex(RuntimeError, "exceeded its smoke budget"):
            self.run_fixture("oversized-output")

    def test_echoed_input_is_not_proof_of_shell_execution(self):
        with self.assertRaisesRegex(RuntimeError, "timed out"):
            self.run_fixture("echo-only", timeout=0.6)

    def test_unresponsive_cli_and_post_closure_hang_are_bounded(self):
        for mode in ("stalled", "exit-stalled"):
            with self.subTest(mode=mode):
                started = time.monotonic()
                with self.assertRaisesRegex(RuntimeError, "timed out|did not exit"):
                    self.run_fixture(mode, timeout=0.6)
                self.assertLess(time.monotonic() - started, 2)


if __name__ == "__main__":
    unittest.main()
