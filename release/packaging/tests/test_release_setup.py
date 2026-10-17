#!/usr/bin/env python3
"""Verify credential minimization and preflight for GitHub release setup."""
import contextlib
import importlib.util
import io
import os
from pathlib import Path
from unittest.mock import patch
import unittest

ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / 'workflows/package/package-release-setup/assets/scripts/configure-release.py'
spec = importlib.util.spec_from_file_location('release_setup', SCRIPT)
setup = importlib.util.module_from_spec(spec)
spec.loader.exec_module(setup)


class ReleaseSetupTests(unittest.TestCase):
    def setUp(self):
        self.environment = {
            'RELEASE_SETUP_REPOSITORY': 'example/standalone-package',
            'AWS_ACCESS_KEY_ID': 'fixture-upload-id',
            'AWS_SECRET_ACCESS_KEY': 'fixture-upload-secret',
            'R2_ENDPOINT_URL': 'https://example.invalid',
            'DOCKPIPE_RELEASE_BUCKET': 'example-staging',
            'R2_PREFIX': 'packages',
        }

    def test_upload_only_copies_two_credentials_and_never_reads_signing_key(self):
        output = io.StringIO()
        with patch.dict(os.environ, self.environment, clear=True), \
                patch.object(setup, 'run') as run, \
                patch.object(setup, 'normalize_signing_key') as normalize, \
                patch.object(setup, 'signing_fingerprint') as signing, \
                contextlib.redirect_stdout(output):
            setup.configure(True, 'release-staging', upload_only=True)
        names = [call.args[0][3] for call in run.call_args_list]
        self.assertEqual(names, ['AWS_ACCESS_KEY_ID', 'AWS_SECRET_ACCESS_KEY',
                                 'R2_ENDPOINT_URL', 'DOCKPIPE_RELEASE_BUCKET', 'R2_PREFIX'])
        for call in run.call_args_list[:2]:
            self.assertEqual(call.args[0][:3], ['gh', 'secret', 'set'])
            self.assertIn('--env', call.args[0])
            self.assertNotIn(call.kwargs['value'], call.args[0])
        normalize.assert_not_called()
        signing.assert_not_called()
        self.assertNotIn(self.environment['AWS_ACCESS_KEY_ID'], output.getvalue())
        self.assertNotIn(self.environment['AWS_SECRET_ACCESS_KEY'], output.getvalue())

    def test_missing_upload_binding_fails_before_any_github_operation(self):
        del self.environment['AWS_SECRET_ACCESS_KEY']
        with patch.dict(os.environ, self.environment, clear=True), patch.object(setup, 'run') as run:
            with self.assertRaisesRegex(ValueError, 'Missing release settings'):
                setup.configure(True, 'release-staging', upload_only=True)
        run.assert_not_called()

    def test_upload_only_checks_do_not_mutate_github(self):
        with patch.dict(os.environ, self.environment, clear=True), \
                patch.object(setup, 'run') as run, contextlib.redirect_stdout(io.StringIO()):
            setup.configure(False, 'release-staging', upload_only=True)
        run.assert_not_called()

    def test_full_release_still_requires_and_copies_signing_material(self):
        self.environment['APT_SIGNING_KEY'] = 'fixture-signing-key'
        with patch.dict(os.environ, self.environment, clear=True), \
                patch.object(setup, 'run') as run, \
                patch.object(setup, 'normalize_signing_key', return_value='normalized-fixture-key'), \
                patch.object(setup, 'signing_fingerprint', return_value='fixture-fingerprint'), \
                contextlib.redirect_stdout(io.StringIO()):
            setup.configure(True, 'release-staging')
        names = [call.args[0][3] for call in run.call_args_list]
        self.assertEqual(names, ['AWS_ACCESS_KEY_ID', 'AWS_SECRET_ACCESS_KEY', 'APT_SIGNING_KEY',
                                 'R2_ENDPOINT_URL', 'DOCKPIPE_RELEASE_BUCKET', 'R2_PREFIX',
                                 'APT_SIGNING_FINGERPRINT'])


if __name__ == '__main__':
    unittest.main()
