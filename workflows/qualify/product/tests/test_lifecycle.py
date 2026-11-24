"""Lifecycle credit requires product observations and exact process closure."""
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'drivers'))
import lifecycle


class LifecycleTests(unittest.TestCase):
    def facts(self, scenario='cancellation'):
        task_exit = {'happy': 0, 'failure': 7, 'cancellation': 143}[scenario]
        return {
            'owned_processes_closed': True, 'remaining_owned_processes': [],
            'supervisor_cleanup_used': False, 'error': '',
            'terminal_records': [{'state': 'succeeded' if task_exit == 0 else 'failed',
                                  'exit_code': task_exit} for _ in range(2)],
            'exit_code': 0 if scenario == 'happy' else 1,
            'outside_canary_unchanged': True, 'stop_requested_to_cli_only': True,
            'cancel_acknowledged': True, 'no_join_result': True, 'no_implicit_replay': True,
            'result_bound': True, 'stdout_bound': True, 'events_bound': True,
            'stderr': 'Error: exit status 7',
        }

    def test_acknowledged_product_cancellation_and_closure_pass(self):
        self.assertTrue(all(lifecycle.scenario_assertions('cancellation', self.facts()).values()))

    def test_cleanup_timeout_unknown_or_live_process_cannot_pass(self):
        for change in ({'supervisor_cleanup_used': True}, {'error': 'deadline'},
                       {'remaining_owned_processes': [{'pid': 42}]},
                       {'owned_processes_closed': False}, {'terminal_records': []}):
            with self.subTest(change=change):
                facts = self.facts()
                facts.update(change)
                self.assertFalse(any(lifecycle.scenario_assertions('cancellation', facts).values()))

    def test_wrong_task_exit_or_missing_acknowledgement_rejects_cancellation(self):
        for change in ({'terminal_records': [{'state': 'failed', 'exit_code': 7}] * 2},
                       {'cancel_acknowledged': False}, {'stop_requested_to_cli_only': False},
                       {'no_join_result': False}, {'exit_code': -15}):
            with self.subTest(change=change):
                facts = self.facts()
                facts.update(change)
                self.assertFalse(lifecycle.scenario_assertions('cancellation', facts)['cancellation-observed'])

    def test_replayed_work_never_gets_no_replay_credit(self):
        facts = self.facts()
        facts['no_implicit_replay'] = False
        self.assertFalse(lifecycle.scenario_assertions('cancellation', facts)['no-implicit-replay'])

    def test_owner_crash_recovery_requires_original_state_and_unassisted_cleanup(self):
        facts = self.facts()
        facts.update(exit_code=-lifecycle.signal.SIGKILL, original_identity_retained=True,
                     state_reconciled=True, no_duplicate_effects=True)
        self.assertTrue(all(lifecycle.scenario_assertions('restart-recovery', facts).values()))

        for change in ({'exit_code': 1}, {'supervisor_cleanup_used': True},
                       {'remaining_owned_processes': [{'pid': 42}]}, {'state_reconciled': False}):
            with self.subTest(change=change):
                observed = lifecycle.scenario_assertions('restart-recovery', dict(facts, **change))
                self.assertFalse(all(observed.values()))

    def test_failure_requires_normal_expected_exit_and_diagnostic(self):
        self.assertTrue(all(lifecycle.scenario_assertions('failure', self.facts('failure')).values()))
        for change in ({'stderr': 'permission denied'}, {'exit_code': -9},
                       {'terminal_records': [{'state': 'failed', 'exit_code': True}] * 2}):
            facts = self.facts('failure')
            facts.update(change)
            self.assertFalse(lifecycle.scenario_assertions('failure', facts)['expected-failure-observed'])

    def test_host_happy_is_blocked_only_when_supported_checks_pass(self):
        observed = lifecycle.scenario_assertions('happy', self.facts('happy'))
        self.assertEqual('blocked', lifecycle.case_status('happy', observed))
        observed['logs-and-result-bound'] = False
        self.assertEqual('failed', lifecycle.case_status('happy', observed))

    def snapshot(self):
        started = {name: {'worker': {'pid': pid, 'start_token': 'boot:' + str(pid)}}
                   for name, pid in [('first', 11), ('second', 12)]}
        records = [{'task': name, 'parent_pid': 10, 'process': record['worker']}
                   for name, record in started.items()]
        return {'schema': 'dockpipe.execution_snapshot.v1', 'executions': records}, started

    def test_observation_requires_exact_kernel_identity(self):
        snapshot, started = self.snapshot()
        self.assertEqual(2, len(lifecycle.observed_members(snapshot, started, 10)))
        snapshot['executions'][0]['process'] = {'pid': 11, 'start_token': 'reused'}
        with self.assertRaises(ValueError):
            lifecycle.observed_members(snapshot, started, 10)

    def test_duplicate_replayed_or_unexpected_join_is_rejected(self):
        for extra in ({'task': 'join', 'parent_pid': 10}, {'task': 'first', 'parent_pid': 10}):
            snapshot, started = self.snapshot()
            snapshot['executions'].append(extra)
            with self.assertRaises(ValueError):
                lifecycle.observed_members(snapshot, started, 10)

    def test_malformed_public_observation_is_controlled_failure(self):
        _, started = self.snapshot()
        for snapshot in ([], {}, {'schema': 'dockpipe.execution_snapshot.v1', 'executions': [None]}):
            with self.subTest(snapshot=snapshot), self.assertRaises(ValueError):
                lifecycle.observed_members(snapshot, started, 10)


if __name__ == '__main__':
    unittest.main()
