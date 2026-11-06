"""Exercise the DSM report transport without a NAS or browser session."""

from pathlib import Path
import shutil
import subprocess
import unittest


@unittest.skipUnless(shutil.which("node"), "Node is required for the DSM report fixture")
class DsmReportTests(unittest.TestCase):
    def test_authenticated_report_limits_and_window_lifetime(self):
        fixture = Path(__file__).with_name("test-dsm-report.cjs")
        subprocess.run(["node", str(fixture)], check=True, capture_output=True,
                       text=True, timeout=10)


if __name__ == "__main__":
    unittest.main()
