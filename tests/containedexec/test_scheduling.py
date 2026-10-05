import copy
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest

from campaign import atomic_json, file_identity
from scheduling import load_profile, measured_plan, observed_profile, warm_profile
from verification_campaign import publish_profile, scheduling_arguments


class SchedulingTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.key = 'a' * 64
        directory = self.root / self.key
        directory.mkdir()
        self.binary = directory / 'program.test'
        self.binary.write_bytes(b'validated native binary')
        self.sha = file_identity(self.binary)['sha256']
        atomic_json(directory / 'record.json', dict(Version='pipelang-native-validation-v2',
                    Key=self.key, BinarySHA256=self.sha))
        self.identity = dict(inputs='current', host='current', workers=2, policy={'memory': 128})
        self.names = ['TestLayouts/0', 'TestLayouts/1']
        self.measurement = dict(warm=True, elapsed_s=.8, peak_bytes=80 << 20, artifacts={self.key: self.sha})
        self.profile = dict(identity=self.identity, cases={name: copy.deepcopy(self.measurement) for name in self.names})
        self.path = self.root / 'profile.json'
        atomic_json(self.path, self.profile)

    def test_missing_malformed_and_stale_hints_fall_back_but_explicit_profiles_fail(self):
        variants = [None, '{broken', '[]', '{}']
        for field in self.identity:
            stale = copy.deepcopy(self.profile)
            stale['identity'][field] = 'changed'
            variants.append(json.dumps(stale))
        for content in variants:
            with self.subTest(content=content):
                if content is None:
                    self.path.unlink(missing_ok=True)
                else:
                    self.path.write_text(content)
                admitted, decision = load_profile(self.path, self.identity, self.root, automatic=True)
                self.assertEqual(admitted, {})
                self.assertEqual(decision['status'], 'singleton_fallback')
                with self.assertRaises((OSError, ValueError)):
                    load_profile(self.path, self.identity, self.root)

    def test_each_artifact_digest_is_checked_even_when_keys_repeat(self):
        self.profile['cases'][self.names[1]]['artifacts'][self.key] = 'b' * 64
        self.assertEqual(list(warm_profile(self.profile, self.identity, self.root)), self.names[:1])
        self.binary.write_bytes(b'changed')
        self.assertFalse(warm_profile(self.profile, self.identity, self.root))

    def test_invalid_measurements_and_artifact_paths_stay_single(self):
        variants = [None, {}, dict(self.measurement, elapsed_s=float('nan')),
                    dict(self.measurement, elapsed_s=-1), dict(self.measurement, peak_bytes=True),
                    dict(self.measurement, peak_bytes=float('inf')), dict(self.measurement, warm=1),
                    dict(self.measurement, artifacts={'../escape': self.sha}),
                    dict(self.measurement, artifacts={self.key: None})]
        for value in variants:
            with self.subTest(value=value):
                self.profile['cases'][self.names[1]] = value
                admitted = warm_profile(self.profile, self.identity, self.root)
                self.assertEqual(list(admitted), self.names[:1])
                jobs = measured_plan([([name], name) for name in self.names], admitted)
                self.assertEqual([len(names) for names, _ in jobs], [1, 1])

    def test_corrupt_cache_record_stays_single(self):
        (self.root / self.key / 'record.json').write_text('[]')
        admitted, decision = load_profile(self.path, self.identity, self.root, automatic=True)
        self.assertEqual(admitted, {})
        self.assertEqual(decision['status'], 'singleton_fallback')

    def test_repeated_groups_retain_only_real_verified_singleton_measurements(self):
        rows = [dict(exit=0, tests=self.names)]
        for _ in range(3):
            admitted, _ = load_profile(self.path, self.identity, self.root, automatic=True)
            jobs = measured_plan([([name], name) for name in self.names], admitted)
            self.assertEqual([names for names, _ in jobs], [self.names])
            next_profile = observed_profile(rows, self.identity, self.root, inherited=admitted)
            self.assertEqual(next_profile, self.profile)
            atomic_json(self.path, next_profile)
        # A new grouped observation has no singleton timings of its own.
        self.assertFalse(observed_profile(rows, self.identity, self.root)['cases'])
        self.binary.unlink()
        self.assertFalse(load_profile(self.path, self.identity, self.root, automatic=True)[0])

    def test_failed_and_unexecuted_cases_cannot_carry_observations(self):
        rows = [dict(exit=1, tests=self.names[:1])]
        self.assertFalse(observed_profile(rows, self.identity, self.root, inherited=self.profile['cases'])['cases'])

    def test_fresh_cold_singleton_replaces_inherited_warm_timing(self):
        (self.root / 'unit.output').write_text(f'generated_compiled_artifact cache_hit=false key={self.key}\n')
        row = dict(exit=0, tests=self.names[:1], directory=str(self.root),
                   report=dict(elapsed_s=2, aggregate_peak_bytes=90 << 20))
        cases = observed_profile([row], self.identity, self.root, inherited=self.profile['cases'])['cases']
        self.assertEqual(list(cases), self.names[:1])
        self.assertFalse(cases[self.names[0]]['warm'])
        self.assertEqual(cases[self.names[0]]['elapsed_s'], 2)

    def test_complete_campaign_defaults_and_explicit_overrides(self):
        args = SimpleNamespace(root=self.root, baseline=self.root / 'baseline',
                               schedule_profile=None, no_pair_scheduling=False)
        self.assertEqual(scheduling_arguments(args), ['--auto-schedule-profile', str(args.baseline / 'suite/schedule-profile.json')])
        saved = self.root / 'schedule-profile.json'
        atomic_json(saved, self.profile)
        self.assertEqual(scheduling_arguments(args), ['--auto-schedule-profile', str(saved)])
        args.schedule_profile = self.path
        self.assertEqual(scheduling_arguments(args), ['--schedule-profile', str(self.path)])
        args.schedule_profile = None
        args.no_pair_scheduling = True
        self.assertEqual(scheduling_arguments(args), [])

    def test_only_complete_successful_suites_publish_predictions(self):
        suite = self.root / 'suite'
        suite.mkdir()
        atomic_json(suite / 'schedule-profile.json', self.profile)
        summary = dict(partial_suite=False, failed=[], source_unchanged=True,
                       toolchain_unchanged=True, reconciliation=dict(missing=[]))
        for field, value in [('partial_suite', True), ('failed', [0]), ('source_unchanged', False),
                             ('toolchain_unchanged', False), ('reconciliation', dict(missing=['case']))]:
            atomic_json(suite / 'summary.json', dict(summary, **{field: value}))
            with self.assertRaises(RuntimeError):
                publish_profile(suite, self.root)
            self.assertFalse((self.root / 'schedule-profile.json').exists())
        atomic_json(suite / 'summary.json', summary)
        publish_profile(suite, self.root)
        self.assertEqual(json.loads((self.root / 'schedule-profile.json').read_text()), self.profile)


if __name__ == '__main__':
    unittest.main()
