"""Exercise the separate DSM task-setup application without a NAS session."""

from pathlib import Path
import shutil
import subprocess
import unittest


@unittest.skipUnless(shutil.which("node"), "Node is required for the DSM task-setup fixture")
class TaskSetupTests(unittest.TestCase):
    def test_explicit_setup_ownership_uncertainty_and_cleanup(self):
        fixture = Path(__file__).with_name("test-task-setup.cjs")
        subprocess.run(["node", str(fixture)], check=True, capture_output=True,
                       text=True, timeout=15)


if __name__ == "__main__":
    unittest.main()
