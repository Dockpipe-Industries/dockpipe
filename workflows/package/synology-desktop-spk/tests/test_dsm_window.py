"""Exercise the authored DSM window shell; real DSM acceptance is separate."""

from pathlib import Path
import shutil
import subprocess
import unittest


@unittest.skipUnless(shutil.which("node"), "Node is required for the DSM shell lifecycle fixture")
class DsmWindowTests(unittest.TestCase):
    def test_https_window_fit_focus_and_teardown(self):
        script = Path(__file__).with_name("test-dsm-window.cjs")
        subprocess.run(["node", str(script)], check=True, capture_output=True, text=True, timeout=10)


if __name__ == "__main__":
    unittest.main()
