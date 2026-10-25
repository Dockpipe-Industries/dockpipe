"""Exercise the real status helper with pipes and an actual pseudo-terminal."""
import os
from pathlib import Path
import shutil
import signal
import subprocess
import tempfile
import unittest


HELPER = Path(os.environ.get(
    "DOCKPIPE_TEST_STATUS_HELPER", Path(__file__).resolve().parents[4] / 'workflows/shared/lib/status.sh'))


@unittest.skipUnless(os.name == "posix", "The packaging helper uses a POSIX shell")
class ElapsedStatusTests(unittest.TestCase):
    def run_helper(self, child_status, terminal):
        bash = os.environ.get("DOCKPIPE_TEST_BASH") or shutil.which("bash")
        self.assertIsNotNone(bash)
        arguments = [bash, "-c", '''
set -euo pipefail
source "$1"
run_with_elapsed_status fixture "$2" -c 'printf "child-output\\n"; sleep 0.4; exit "$1"' child "$3"
''', "status-test", str(HELPER), bash, str(child_status)]
        with tempfile.TemporaryDirectory(prefix="dockpipe-elapsed-status-") as temporary:
            environment = dict(os.environ, TMPDIR=temporary)
            if terminal:
                import pty
                master, slave = pty.openpty()
                self.addCleanup(os.close, master)
                self.addCleanup(os.close, slave)
            else:
                slave = None
            with tempfile.TemporaryFile() as output:
                process = subprocess.Popen(
                    arguments,
                    env=environment,
                    stdout=output,
                    stderr=slave if terminal else output,
                    start_new_session=True,
                )
                try:
                    process.wait(timeout=10)
                finally:
                    if process.poll() is None:
                        os.killpg(process.pid, signal.SIGKILL)
                        process.wait(timeout=5)
                output.seek(0)
                data = output.read(65537)
                if terminal:
                    # Keep the parent's slave open until output is drained. BSD
                    # PTYs may discard queued output when the last slave closes.
                    import select
                    while len(data) <= 65536:
                        readable, _, _ = select.select([master], [], [], 0.1)
                        if not readable:
                            break
                        block = os.read(master, 4096)
                        if not block:
                            break
                        data += block
            self.assertLessEqual(len(data), 65536)
            self.assertEqual(process.returncode, child_status, data.decode(errors="replace"))
            self.assertIn(b"child-output", data)
            self.assertNotIn(b"syntax error", data)
            expected = b"done in" if child_status == 0 else b"failed after"
            self.assertIn(expected, data)
            self.assertEqual(list(Path(temporary).iterdir()), [],
                             "The helper must remove its terminal output file")

    def test_success_with_pipe(self):
        self.run_helper(0, False)

    def test_failure_with_pipe_preserves_child_status(self):
        self.run_helper(17, False)

    def test_success_with_terminal(self):
        self.run_helper(0, True)

    def test_failure_with_terminal_preserves_child_status(self):
        self.run_helper(17, True)


if __name__ == "__main__":
    unittest.main()
