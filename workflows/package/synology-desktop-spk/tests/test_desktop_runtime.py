"""Regression checks for DSM's arbitrary, enforced non-root container identity."""

import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


SOURCE = Path(__file__).resolve().parents[1] / 'assets/desktop/desktop.py'
SPECIFICATION = importlib.util.spec_from_file_location('desktop_runtime', SOURCE)
RUNTIME = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(RUNTIME)


class DesktopRuntimeTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.settings = self.root / 'settings'
        self.settings.mkdir()
        (self.settings / 'uid').write_text('54321')
        (self.settings / 'gid').write_text('54321')
        self.runtime = self.root / 'runtime'
        self.runtime.mkdir(mode=0o700)
        settings = patch.object(RUNTIME, 'SETTINGS', self.settings)
        settings.start()
        self.addCleanup(settings.stop)

    def test_arbitrary_dsm_uid_gets_process_local_identity_without_host_privileges(self):
        with patch.object(os, 'getuid', return_value=54321), \
                patch.object(os, 'getgid', return_value=54321), patch.dict(os.environ):
            RUNTIME.prepare_identity(self.runtime)
            self.assertIn('dockpipe:x:54321:54321:', (self.runtime / 'passwd').read_text())
            self.assertIn('dockpipe:x:54321:', (self.runtime / 'group').read_text())
            self.assertEqual(os.environ['NSS_WRAPPER_PASSWD'], str(self.runtime / 'passwd'))
            self.assertEqual(os.environ['HOME'], '/config')

    def test_root_and_mismatched_runtime_identities_are_rejected(self):
        for uid, gid in [(0, 54321), (54321, 0), (1000, 1000)]:
            with self.subTest(uid=uid, gid=gid), \
                    patch.object(os, 'getuid', return_value=uid), \
                    patch.object(os, 'getgid', return_value=gid), self.assertRaises(ValueError):
                RUNTIME.prepare_identity(self.runtime)
            self.assertFalse((self.runtime / 'passwd').exists())

    def test_partial_tls_state_fails_without_overwriting_existing_material(self):
        certificate = self.root / 'ssl/cert.pem'
        certificate.parent.mkdir()
        certificate.write_text('existing certificate')
        with patch.object(RUNTIME, 'DATA', self.root), self.assertRaisesRegex(ValueError, 'incomplete'):
            RUNTIME.prepare_tls()
        self.assertEqual(certificate.read_text(), 'existing certificate')

    def test_existing_tls_pair_is_reused_with_private_key_permissions(self):
        directory = self.root / 'ssl'
        directory.mkdir()
        certificate = directory / 'cert.pem'
        key = directory / 'cert.key'
        certificate.write_text('existing certificate')
        key.write_text('existing key')
        with patch.object(RUNTIME, 'DATA', self.root), \
                patch.object(RUNTIME.subprocess, 'run') as run:
            self.assertEqual(RUNTIME.prepare_tls(), (certificate, key))
        run.assert_not_called()
        self.assertEqual(key.stat().st_mode & 0o777, 0o600)

    def test_host_network_service_binds_only_loopback_and_keeps_credentials_out_of_argv(self):
        password = 'fixture-private-password'
        (self.settings / 'username').write_text('operator')
        (self.settings / 'password').write_text(password)
        with patch.object(RUNTIME, 'DATA', self.root), \
                patch.object(RUNTIME, 'prepare_identity'), \
                patch.object(RUNTIME, 'prepare_tls', return_value=('certificate', 'key')), \
                patch.object(RUNTIME.os, 'execv') as execute, patch.dict(os.environ):
            RUNTIME.main()
            arguments = execute.call_args.args[1]
            self.assertNotIn('--public', arguments)
            self.assertEqual(arguments[arguments.index('--addr') + 1], '127.0.0.1')
            self.assertEqual(arguments[arguments.index('--port') + 1], '38139')
            self.assertNotIn(password, ' '.join(arguments))


if __name__ == '__main__':
    unittest.main()
