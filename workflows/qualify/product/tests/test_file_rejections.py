"""Negative file probes must observe a normal, actionable CLI refusal."""
from pathlib import Path
import json
import sys
import tempfile
import unittest

OWNER = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(OWNER / 'assets/scripts'))
sys.path.insert(0, str(OWNER / 'drivers'))
import workspace_files


class FileRejectionTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.logs = self.root / 'negative-probe'
        self.logs.mkdir()

    def tearDown(self):
        self.temporary.cleanup()

    def observe(self, exit_code=1, failure='', diagnostic=None):
        if diagnostic is None:
            diagnostic = workspace_files.STALE_REVISION_DIAGNOSTIC + '\n'
        (self.logs / 'process.json').write_text(json.dumps({'exit_code': exit_code, 'failure': failure}))
        (self.logs / 'driver.stderr').write_bytes(diagnostic.encode('utf-8'))
        return workspace_files.expected_rejection(self.root, 'negative-probe',
                                                 {workspace_files.STALE_REVISION_DIAGNOSTIC})

    def test_normal_stale_revision_refusal_is_observed(self):
        self.assertTrue(self.observe())

    def test_timeout_never_counts_as_rejection_even_with_expected_stderr(self):
        self.assertFalse(self.observe(failure='Driver exceeded its declared deadline'))

    def test_log_overflow_never_counts_as_rejection(self):
        self.assertFalse(self.observe(failure='Driver log exceeded its 4 MiB per-stream bound'))

    def test_signal_or_other_exit_does_not_count_as_normal_refusal(self):
        for code in (-9, -15, 0, 2, 127, None, True):
            with self.subTest(exit_code=code):
                self.assertFalse(self.observe(exit_code=code))

    def test_unrelated_or_empty_failure_is_not_expected_rejection(self):
        for diagnostic in ('', 'permission denied\n', 'context deadline exceeded\n'):
            with self.subTest(diagnostic=diagnostic):
                self.assertFalse(self.observe(diagnostic=diagnostic))

    def test_negative_probe_requires_its_own_expected_diagnostic(self):
        self.assertFalse(self.observe(diagnostic=workspace_files.OUTSIDE_ROOT_DIAGNOSTIC))
        self.assertTrue(workspace_files.expected_rejection(self.root, 'negative-probe',
                                                         {workspace_files.OUTSIDE_ROOT_DIAGNOSTIC}))

    def test_nonregular_link_refusal_is_actionable_for_the_link_probe(self):
        self.assertFalse(self.observe(diagnostic=workspace_files.NOT_REGULAR_DIAGNOSTIC))
        self.assertTrue(workspace_files.expected_rejection(self.root, 'negative-probe',
            {workspace_files.OUTSIDE_ROOT_DIAGNOSTIC, workspace_files.NOT_REGULAR_DIAGNOSTIC}))

    def test_missing_or_malformed_process_outcome_cannot_pass(self):
        self.observe()
        for record in ({'exit_code': 1}, {'exit_code': 1, 'failure': None}, []):
            with self.subTest(record=record):
                (self.logs / 'process.json').write_text(json.dumps(record))
                self.assertFalse(workspace_files.expected_rejection(self.root, 'negative-probe',
                    {workspace_files.STALE_REVISION_DIAGNOSTIC}))

    def test_invalid_utf8_diagnostic_cannot_count_as_expected_rejection(self):
        self.observe()
        (self.logs / 'driver.stderr').write_bytes(b'\xff')
        self.assertFalse(workspace_files.expected_rejection(self.root, 'negative-probe',
            {workspace_files.STALE_REVISION_DIAGNOSTIC}))

    def test_oversized_diagnostic_cannot_count_as_expected_rejection(self):
        self.observe()
        (self.logs / 'driver.stderr').write_bytes(b'x' * (workspace_files.MAX_LOG_BYTES + 1))
        self.assertFalse(workspace_files.expected_rejection(self.root, 'negative-probe',
            {workspace_files.STALE_REVISION_DIAGNOSTIC}))


if __name__ == '__main__':
    unittest.main()
