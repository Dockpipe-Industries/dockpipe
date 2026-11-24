"""Negative file qualification requires known refusal and exact fixture integrity."""
from pathlib import Path
from unittest.mock import patch
import base64
import hashlib
import json
import sys
import tempfile
import unittest

OWNER = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(OWNER / 'drivers'))
import file_failures as driver


class FileFailureTests(unittest.TestCase):
    def exercise(self, root, fault=None, mutate=None, positive=True):
        operations = []
        def execute(cli, arguments, workspace, environment, output, name):
            operations.append(name)
            request = driver.read_json(Path(arguments[2]))
            self.assertEqual(arguments[:2], ['files', '--request'])
            self.assertEqual(request['request']['schema'], 'dockpipe.files/v1')
            logs = output / name
            logs.mkdir()
            body = request['request']
            if name == 'positive-read':
                content = (Path(request['root']) / 'owned.txt').read_bytes()
                response = {'schema': 'dockpipe.files/v1', 'revision': hashlib.sha256(content).hexdigest(),
                            'content': base64.b64encode(content).decode('ascii')}
                driver.write_json(logs / 'driver.stdout', response if positive else {})
                return True
            self.assertEqual(body['action'], 'write')
            self.assertEqual(base64.b64decode(body['content']), b'must never publish rejected edit\n')
            expected = driver.workspace_files.STALE_REVISION_DIAGNOSTIC if name == 'stale-write' else driver.workspace_files.OUTSIDE_ROOT_DIAGNOSTIC
            record = {'exit_code': 1, 'failure': ''}
            if name == 'stale-write' and fault:
                record.update(fault)
            if name == 'stale-write' and mutate:
                target = output / 'adjacent-canary.txt' if mutate == 'canary' else Path(request['root']) / 'bystander.txt'
                target.write_bytes(b'changed unexpectedly')
            driver.write_json(logs / 'process.json', record)
            (logs / 'driver.stderr').write_text(expected + '\n')
            (logs / 'driver.stdout').write_text('')
            return record['exit_code'] == 0
        facts, evidence = driver.qualify(Path('/unused/pinned-cli'), root, {}, root, execute)
        facts['artifact_pins_unchanged'] = True
        return facts, evidence, operations

    def test_all_exact_refusals_preserve_bytes_and_collect_public_requests(self):
        with tempfile.TemporaryDirectory() as temporary:
            facts, evidence, operations = self.exercise(Path(temporary))
            self.assertTrue(all(driver.evaluate(facts).values()))
            self.assertEqual(operations, ['positive-read', 'stale-write', 'escaped-write', 'linked-write'])
            self.assertEqual(facts['before_snapshot'], facts['after_snapshot'])
            self.assertTrue(any(path.name == 'linked-write.request.json' for path in evidence))

    def test_uncertain_refusal_stops_subsequent_probes_and_never_passes(self):
        for fault in [{'exit_code': -15}, {'exit_code': 0}, {'exit_code': True},
                      {'failure': 'deadline'}, {'failure': 'log overflow'}]:
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as temporary:
                facts, _, operations = self.exercise(Path(temporary), fault=fault)
                self.assertFalse(all(driver.evaluate(facts).values()))
                self.assertEqual(operations, ['positive-read', 'stale-write'])

    def test_canary_or_bystander_change_fails_and_stops(self):
        for target in ['canary', 'bystander']:
            with self.subTest(target=target), tempfile.TemporaryDirectory() as temporary:
                facts, _, operations = self.exercise(Path(temporary), mutate=target)
                self.assertFalse(driver.evaluate(facts)['no-unowned-effects'])
                self.assertEqual(operations, ['positive-read', 'stale-write'])

    def test_failed_positive_admission_performs_no_negative_requests(self):
        with tempfile.TemporaryDirectory() as temporary:
            facts, _, operations = self.exercise(Path(temporary), positive=False)
            self.assertFalse(all(driver.evaluate(facts).values()))
            self.assertEqual(operations, ['positive-read'])

    def test_missing_probe_or_pin_cannot_pass(self):
        with tempfile.TemporaryDirectory() as temporary:
            facts, _, _ = self.exercise(Path(temporary))
            facts['artifact_pins_unchanged'] = False
            self.assertFalse(all(driver.evaluate(facts).values()))
            facts['artifact_pins_unchanged'] = True
            del facts['probes']['linked-write']
            self.assertFalse(all(driver.evaluate(facts).values()))

    def test_snapshot_rejects_unexpected_entries_without_traversing_link(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            self.exercise(root)
            workspace = root / 'file-workspace'
            (workspace / 'unexpected').mkdir()
            with self.assertRaises(ValueError):
                driver.snapshot(workspace, root / 'adjacent-canary.txt')

    def test_unsupported_row_is_blocked_without_cli(self):
        request = {'run_id': 'fixture', 'plan_sha256': 'a' * 64, 'registry_sha256': 'b' * 64,
                   'target': {}, 'cases': [{'id': 'files.workspace', 'scenario': 'cancellation', 'layer': 'component'}]}
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / 'output'
            with patch.object(driver, 'qualify') as qualify:
                driver.run(request, output)
                qualify.assert_not_called()
            self.assertEqual(driver.read_json(output / 'result.json')['cases'][0]['status'], 'blocked')


if __name__ == '__main__':
    unittest.main()
