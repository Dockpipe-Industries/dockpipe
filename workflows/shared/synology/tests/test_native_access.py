"""Exercise the shared DSM scheduler adapter without a NAS session."""

from pathlib import Path
import shutil
import subprocess
import unittest


@unittest.skipUnless(shutil.which("node"), "Node is required for the shared DSM adapter fixture")
class NativeAccessTests(unittest.TestCase):
    def test_qualification_and_persistent_worker_lifecycles(self):
        fixture = Path(__file__).with_name("test-native-access.cjs")
        subprocess.run(["node", str(fixture)], check=True, capture_output=True,
                       text=True, timeout=10)


if __name__ == "__main__":
    unittest.main()
