"""Verify the gate against actual CTest pass, skip, failure and missing reports."""
import importlib.util
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

SPECIFICATION = importlib.util.spec_from_file_location('ctest_result', Path(__file__).resolve().parents[1] / 'ctest_result.py')
results = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(results)


class CtestResultTests(unittest.TestCase):
    def report(self, directory, code):
        root = Path(directory)
        python = sys.executable.replace('\\', '/')
        (root / 'CTestTestfile.cmake').write_text(
            'add_test([=[desktop-app-window-win]=] [=[' + python + ']=] "-c" "import sys; sys.exit(' + str(code) + ')")\n'
            'set_tests_properties([=[desktop-app-window-win]=] PROPERTIES SKIP_RETURN_CODE 77)\n')
        report = root / 'native-results.xml'
        ctest = shutil.which('ctest')
        if not ctest:
            self.fail('CTest is required to qualify its actual result contract')
        completed = subprocess.run(
            [ctest, '--test-dir', str(root), '--no-tests=error', '--output-junit', str(report)],
            capture_output=True, text=True, timeout=30)
        return report, completed.returncode

    def test_exact_genuine_ctest_pass_is_accepted(self):
        with tempfile.TemporaryDirectory() as directory:
            report, code = self.report(directory, 0)
            self.assertEqual(code, 0)
            self.assertEqual(results.require_passes(report, ['desktop-app-window-win']), ('desktop-app-window-win',))

    def test_skip_rejects_even_when_ctest_itself_succeeds(self):
        with tempfile.TemporaryDirectory() as directory:
            report, code = self.report(directory, 77)
            self.assertEqual(code, 0, 'CTest skip behavior must be reproduced')
            with self.assertRaisesRegex(ValueError, 'genuinely pass'):
                results.require_passes(report, ['desktop-app-window-win'])

    def test_failed_fixture_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            report, code = self.report(directory, 1)
            self.assertNotEqual(code, 0)
            with self.assertRaisesRegex(ValueError, 'genuinely pass'):
                results.require_passes(report, ['desktop-app-window-win'])

    def test_missing_duplicate_or_notrun_required_test_is_rejected(self):
        for cases in ('', '<testcase name="other" status="run"/>',
                      '<testcase name="desktop-app-window-win" status="notrun"/>',
                      '<testcase name="desktop-app-window-win" status="run"/>' * 2):
            with self.subTest(cases=cases):
                with tempfile.TemporaryDirectory() as directory:
                    report = Path(directory) / 'native.xml'
                    report.write_text('<testsuite>' + cases + '</testsuite>')
                    with self.assertRaises(ValueError):
                        results.require_passes(report, ['desktop-app-window-win'])


if __name__ == '__main__':
    unittest.main()
