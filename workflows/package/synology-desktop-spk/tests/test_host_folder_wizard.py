"""Exercise the DSM folder wizard without a live browser or DSM mutations."""

from pathlib import Path
import shutil
import subprocess
import unittest


class HostFolderWizardTests(unittest.TestCase):
    @unittest.skipUnless(shutil.which("node"), "Node.js is required for the isolated Vue fixture")
    def test_repeatable_folder_wizard_contract(self):
        fixture = Path(__file__).with_name("test-host-folders-wizard.cjs")
        result = subprocess.run(["node", str(fixture)], capture_output=True, text=True, timeout=10)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
