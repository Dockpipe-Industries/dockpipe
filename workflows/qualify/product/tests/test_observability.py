"""Observability needs current, scoped, independently checked resource data."""
from pathlib import Path
import sys
import unittest


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'drivers'))
import lifecycle


class ObservabilityTests(unittest.TestCase):
    def facts(self):
        started = {name: {'worker': {'pid': pid, 'start_token': 'boot:' + str(pid)}}
                   for name, pid in [('first', 11), ('second', 12)]}
        facts = {'owned_processes_closed': True, 'remaining_owned_processes': [],
                 'supervisor_cleanup_used': False, 'error': '', 'exit_code': 0,
                 'terminal_records': [{'state': 'succeeded', 'exit_code': 0}] * 2,
                 'result_bound': True, 'stdout_bound': True, 'events_bound': True,
                 'started': started, 'cli_process': {'pid': 10},
                 'independent_system': {'logical_cpus': 4, 'memory_total': 1024, 'disk_total': 4096},
                 'resource_observations': []}
        for index in range(3):
            captured = '2026-01-01T00:00:0' + str(index) + '.123456789Z'
            records = []
            for name, reference in started.items():
                records.append({'task': name, 'parent_pid': 10, 'process': reference['worker'],
                    'state': 'running', 'machine': 'local',
                    'metrics': {**reference['worker'], 'machine': 'local', 'memory_bytes': 100, 'cpu_seconds': 0.1}})
            snapshot = {'schema': 'dockpipe.execution_snapshot.v1', 'machine': 'local', 'captured_at': captured,
                        'truncated': False, 'processes': [], 'executions': records,
                        'system': {'boot_id': 'boot', **facts['independent_system'], 'memory_available': 512,
                                   'cpu_total': 100 + index, 'uptime_seconds': 50 + index}}
            facts['resource_observations'].append({'snapshot': snapshot, 'elapsed_seconds': 0.1,
                'received_unix_seconds': lifecycle.captured_timestamp(captured) + 0.1, 'response_bytes': 4096})
        return facts

    def test_current_scoped_independently_checked_samples_pass(self):
        self.assertTrue(all(lifecycle.observability_assertions(self.facts()).values()))

    def test_wrong_environment_units_missing_metrics_or_cleanup_cannot_pass(self):
        changes = [('boot_id', 'different'), ('memory_total', 1), ('logical_cpus', False), ('disk_total', 1)]
        for field, value in changes:
            with self.subTest(field=field):
                facts = self.facts()
                facts['resource_observations'][0]['snapshot']['system'][field] = value
                self.assertFalse(any(lifecycle.observability_assertions(facts).values()))
        for field, value in [('supervisor_cleanup_used', True), ('remaining_owned_processes', [{'pid': 11}])]:
            facts = self.facts()
            facts[field] = value
            self.assertFalse(any(lifecycle.observability_assertions(facts).values()))

    def test_slow_stale_truncated_or_retargeted_snapshot_is_rejected(self):
        for change in ('slow', 'stale', 'truncated', 'identity', 'missing-metrics', 'old-capture'):
            with self.subTest(change=change):
                facts = self.facts()
                sample = facts['resource_observations'][1]
                if change == 'slow':
                    sample['elapsed_seconds'] = 6
                elif change == 'stale':
                    sample['snapshot']['system']['uptime_seconds'] = 50
                elif change == 'truncated':
                    sample['snapshot']['truncated'] = True
                elif change == 'identity':
                    sample['snapshot']['executions'][0]['metrics']['start_token'] = 'reused'
                elif change == 'missing-metrics':
                    sample['snapshot']['system'] = None
                else:
                    sample['received_unix_seconds'] += 30
                self.assertFalse(all(lifecycle.observability_assertions(facts).values()))

    def test_utc_capture_parser_rejects_ambiguous_or_invalid_dates(self):
        for value in ('2026-01-01T00:00:00', '2026-01-01T00:00:00+01:00',
                      '2026-02-30T00:00:00Z', '2026-01-01T00:00:00.1234567890Z', None):
            with self.subTest(value=value):
                self.assertIsNone(lifecycle.captured_timestamp(value))
        self.assertEqual(lifecycle.captured_timestamp('2026-01-01T00:00:00.123456Z'),
                         lifecycle.captured_timestamp('2026-01-01T00:00:00.123456789Z'))


if __name__ == '__main__':
    unittest.main()
