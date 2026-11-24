"""Initialization refusal cannot pass on a crash, deadline or changed fixture."""
from pathlib import Path
import sys
import unittest


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'drivers'))
import init_failure


class InitFailureTests(unittest.TestCase):
    def facts(self):
        return {'process': {'exit_code': 1, 'failure': ''},
                'diagnostic': 'mkdir /private/generated/project/workflows: permission denied',
                'project': '/private/generated/project', 'project_unchanged': True,
                'outside_canary_unchanged': True, 'private_home_not_initialized': True,
                'artifact_unchanged': True}

    def test_normal_permission_refusal_passes(self):
        self.assertTrue(all(init_failure.evaluate(self.facts()).values()))

    def test_crash_deadline_boolean_or_success_never_counts_as_refusal(self):
        for process in ({'exit_code': -9, 'failure': ''}, {'exit_code': True, 'failure': ''},
                        {'exit_code': 1, 'failure': 'deadline'}, {'exit_code': 0, 'failure': ''}):
            with self.subTest(process=process):
                facts = self.facts()
                facts['process'] = process
                self.assertFalse(init_failure.evaluate(facts)['expected-failure-observed'])

    def test_unrelated_diagnostic_does_not_count(self):
        for diagnostic in ['unknown command', 'permission denied at an unrelated path',
                           '/private/generated/project: starting\npermission denied at another path']:
            facts = self.facts()
            facts['diagnostic'] = diagnostic
            self.assertFalse(init_failure.evaluate(facts)['actionable-result'])

    def test_each_unexpected_effect_prevents_credit(self):
        for name in ['project_unchanged', 'outside_canary_unchanged',
                     'private_home_not_initialized', 'artifact_unchanged']:
            facts = self.facts()
            facts[name] = False
            self.assertFalse(init_failure.evaluate(facts)['no-unowned-effects'])


if __name__ == '__main__':
    unittest.main()
