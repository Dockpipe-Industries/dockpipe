import json
from pathlib import Path
import tempfile
import unittest
from budget import DiskBudget
from scheduling import measured_plan, warm_profile
from verification import StageRunner, completed_go_cases
from campaign import Campaign, file_identity, atomic_json, sealed
from test_campaign import report
from unittest.mock import patch
from reporting import attempt_costs


class VerificationTests(unittest.TestCase):
    def test_resumed_runner_returns_exact_result_without_dispatch(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            artifact = root / 'artifact'; artifact.write_text('proof')
            inputs = ['fixed']
            with Campaign(root / 'campaign') as campaign:
                stage = StageRunner(campaign, 'suite', 'source', ['one', 'two'], root / 'cache', lambda: inputs[0])
                attempt = campaign.begin('suite', ['one', 'two'], 'fixed')
                saved = campaign.finish(attempt, report(root), [artifact], 'fixed', ['one', 'two'], {'exit': 0, 'tests': ['one', 'two'], 'marker': 'exact'})
            with Campaign(root / 'campaign', 'resume') as campaign:
                stage = StageRunner(campaign, 'suite', 'source', ['one', 'two'], root / 'cache', lambda: inputs[0])
                with patch('verification.subprocess.run', side_effect=AssertionError('unexpected dispatch')):
                    row = stage.run(['one', 'two'], ['never'])
                self.assertEqual(row, dict(saved['result'], report=saved['report'], resumed=True, receipt=saved['id']))
                self.assertEqual(stage.finish()['reused'], 1)
                inputs[0] = 'drift'
                with self.assertRaisesRegex(RuntimeError, 'changed before reuse'):
                    stage.resume('one')
                inputs[0] = 'fixed'
                with patch('verification.host_identity', return_value={'changed': True}):
                    with self.assertRaisesRegex(RuntimeError, 'changed before reuse'):
                        stage.resume('one')
                artifact.write_text('corrupt')
                with self.assertRaises(RuntimeError):
                    stage.run(['one', 'two'], ['never'])

    def test_nested_memory_selectors_and_empty_results(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / 'log'
            path.write_text('--- PASS: TestMemory (0.2s)\n  --- PASS: TestMemory/v0.109.0/choices0/branchtrue/32 (0.1s)\n')
            self.assertEqual(completed_go_cases(path, ['TestMemory/choices0']), ['TestMemory/choices0'])
            self.assertEqual(completed_go_cases(path, ['TestMemory/choices1']), [])
            self.assertEqual(completed_go_cases(path, ['TestOther']), [])

    def test_measured_groups_retain_inventory_and_heavy_unknown_singletons(self):
        names = ['TestLayouts/0', 'TestLayouts/1', 'TestLayouts/2', 'TestMemory/0', 'TestLayouts/3']
        jobs = [([n], n) for n in names]
        profile = {n: dict(elapsed_s=.8, peak_bytes=80 << 20, warm=True) for n in names[:4]}
        profile[names[2]]['elapsed_s'] = 8
        result = measured_plan(jobs, profile)
        self.assertEqual([n for group, _ in result for n in group], names)
        self.assertEqual([len(group) for group, _ in result], [2, 1, 1, 1])

    def test_profile_rejects_host_policy_changes_and_lost_cached_binary(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw); key = 'a' * 64; (root / key).mkdir()
            binary = root / key / 'program.test'; binary.write_bytes(b'verified')
            value = file_identity(binary)['sha256']
            (root / key / 'record.json').write_text(json.dumps(dict(Version='pipelang-native-validation-v2', Key=key, BinarySHA256=value)))
            profile = dict(identity={'host': 'current', 'workers': 2}, cases={'TestLayout/0': dict(warm=True, elapsed_s=.8, peak_bytes=80 << 20, artifacts={key:value})})
            self.assertTrue(warm_profile(profile, profile['identity'], root))
            self.assertFalse(warm_profile(profile, {'host':'changed', 'workers':2}, root))
            self.assertFalse(warm_profile(profile, {'host':'current', 'workers':1}, root))
            binary.write_bytes(b'corrupt')
            self.assertFalse(warm_profile(profile, profile['identity'], root))
            binary.unlink()
            self.assertFalse(warm_profile(profile, profile['identity'], root))

    def test_retry_cost_keeps_failed_work_and_unknown_interruption_separate(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            for name, state in [('failed', {'state':'failed','report':{'elapsed_s':2.5}}), ('interrupted', {'state':'interrupted'})]:
                directory = root / 'attempts' / name; directory.mkdir(parents=True)
                atomic_json(directory / 'state.json', sealed(state))
            result = attempt_costs(root)
            self.assertEqual(result['states'], {'failed':1, 'interrupted':1})
            self.assertEqual(result['measured_workload_s'], {'failed':2.5})
            self.assertEqual(result['unknown_elapsed_attempts'], 1)

    def test_incremental_disk_budget_counts_population_and_preserves_pins(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            cache = root / 'cache'; cache.mkdir()
            pinned = cache / 'pinned'; pinned.write_bytes(b'pinned')
            budget = DiskBudget([cache], root, 100, 0)
            self.assertEqual(budget.check()['retained_bytes'], 6)
            (cache / 'new').mkdir()
            (cache / 'new/object').write_bytes(b'new object')
            self.assertEqual(budget.check()['retained_bytes'], 16)
            (cache / 'new/object').write_bytes(b'X' * 101)
            with self.assertRaisesRegex(RuntimeError, 'disk budget'):
                budget.check()
            self.assertEqual(pinned.read_bytes(), b'pinned')
            budget.close()


if __name__ == '__main__': unittest.main()
