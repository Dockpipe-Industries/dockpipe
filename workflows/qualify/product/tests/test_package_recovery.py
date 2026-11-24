"""Interrupted reinstall recovery must never replay an uncertain operation."""
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch
import io
import sys
import tempfile
import unittest

OWNER = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(OWNER / 'drivers'))
import package_recovery as recovery


class PackageRecoveryTests(unittest.TestCase):
    def facts(self):
        names = [
            'interrupted_archive_refused', 'one_interruption', 'one_failed_download',
            'original_store_unchanged', 'original_marker_unchanged', 'bystander_unchanged',
            'no_partials_after_failure', 'fresh_retry_succeeded', 'same_path_and_digest',
            'one_explicit_retry_download', 'no_partials_after_retry',
            'single_catalog_identity_after_retry', 'fixture_server_stopped',
            'artifact_pins_unchanged', 'only_two_explicit_workflow_executions',
            'final_store_unchanged',
        ]
        return dict.fromkeys(names, True)

    def test_every_required_observation_is_necessary(self):
        self.assertTrue(all(recovery.evaluate(self.facts()).values()))
        for name in self.facts():
            with self.subTest(name=name):
                facts = self.facts()
                facts[name] = False
                self.assertFalse(all(recovery.evaluate(facts).values()))

    def test_uncertain_failure_never_retries(self):
        for name in self.facts():
            facts = self.facts()
            facts[name] = False
            if recovery.failure_is_reconciled(facts):
                continue
            calls = []
            recovery.recover_if_known(facts, lambda: calls.append('retry'), lambda: calls.append('execute'))
            self.assertEqual(calls, [], name)

    def test_retry_failure_never_executes(self):
        facts = self.facts()
        calls = []
        def retry():
            calls.append('retry')
            facts['fresh_retry_succeeded'] = False
        recovery.recover_if_known(facts, retry, lambda: calls.append('execute'))
        self.assertEqual(calls, ['retry'])

    def test_known_recovery_executes_once_after_retry(self):
        calls = []
        recovery.recover_if_known(self.facts(), lambda: calls.append('retry'), lambda: calls.append('execute'))
        self.assertEqual(calls, ['retry', 'execute'])

    def test_only_normal_eof_refusal_is_accepted(self):
        process = {'exit_code': 1, 'failure': ''}
        fault = {'path': '/fixture.tar.gz', 'sent_bytes': 3, 'declared_bytes': 8}
        def observe(proc=process, diagnostic='download: unexpected EOF\n', interruption=fault):
            return recovery.interrupted_refusal(proc, diagnostic, interruption, '/fixture.tar.gz')
        self.assertTrue(observe())
        for code in [True, 0, -15, None, 2]:
            self.assertFalse(observe(proc=dict(process, exit_code=code)))
        for failure in ['deadline', 'log overflow', 'signal']:
            self.assertFalse(observe(proc=dict(process, failure=failure)))
        for diagnostic in ['checksum mismatch', 'unexpected EOF\nother failure', 'x' * 65537]:
            self.assertFalse(observe(diagnostic=diagnostic))
        for changed in [dict(fault, path='/foreign'), dict(fault, sent_bytes=True),
                        dict(fault, sent_bytes=0), dict(fault, sent_bytes=8), None]:
            self.assertFalse(observe(interruption=changed))

    def test_handler_truncates_once_with_full_declared_length(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / 'fixture.tar.gz'
            path.write_bytes(b'abcdefgh')
            handler = recovery.InterruptedArchiveHandler.__new__(recovery.InterruptedArchiveHandler)
            handler.path = '/fixture.tar.gz'
            handler.directory = temporary
            handler.server = SimpleNamespace(interrupted_path=handler.path, interrupt_next=True,
                allowed_paths={handler.path}, requests=[], interruptions=[])
            handler.wfile = io.BytesIO()
            headers = {}
            handler.send_response = lambda status: self.assertEqual(status, 200)
            handler.send_header = lambda name, value: headers.update({name: value})
            handler.end_headers = lambda: None
            handler.do_GET()
            self.assertEqual(headers['Content-Length'], '8')
            self.assertEqual(handler.wfile.getvalue(), b'abcd')
            self.assertTrue(handler.close_connection)
            self.assertFalse(handler.server.interrupt_next)
            self.assertEqual(handler.server.requests, ['/fixture.tar.gz'])
            with patch.object(recovery.FixtureHandler, 'do_GET') as original:
                handler.do_GET()
                original.assert_called_once()
            self.assertEqual(len(handler.server.interruptions), 1)

    def test_inventory_detects_bytes_symlinks_and_partial_state(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            archive = root / 'installed.tar.gz'
            archive.write_bytes(b'original')
            before = recovery.store_inventory(root)
            archive.write_bytes(b'changed')
            self.assertNotEqual(before, recovery.store_inventory(root))
            self.assertTrue(recovery.no_partial_install(root))
            (root / '.install-owned').mkdir()
            self.assertFalse(recovery.no_partial_install(root))
            (root / 'alias').symlink_to(archive)
            with self.assertRaises(ValueError):
                recovery.store_inventory(root)

    def test_unsupported_case_has_no_cli_or_listener_effect(self):
        request = {'run_id': 'test', 'plan_sha256': 'a' * 64, 'registry_sha256': 'b' * 64,
                   'target': {}, 'cases': [{'id': 'packages.lifecycle', 'scenario': 'happy', 'layer': 'component'}]}
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / 'output'
            with patch.object(recovery, 'command') as command, patch.object(recovery, 'FixtureServer') as server:
                recovery.run(request, output)
                command.assert_not_called()
                server.assert_not_called()
            self.assertEqual(recovery.read_json(output / 'result.json')['cases'][0]['status'], 'blocked')


if __name__ == '__main__':
    unittest.main()
