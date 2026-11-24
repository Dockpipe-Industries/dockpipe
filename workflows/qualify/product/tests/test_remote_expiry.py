"""Explicit-only remote expiry registration; generated plans, no GUI or network."""
from pathlib import Path
import hashlib
import importlib.util
import json
import sys
import subprocess
import tempfile
import unittest
from unittest.mock import patch

OWNER = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(OWNER / 'assets/scripts'))
import contracts
import remote_expiry_plan


class RemoteExpiryPlanTests(unittest.TestCase):
    def test_registry_keeps_input_happy_case_separate_from_no_input_expiry(self):
        registry = contracts.validate_registry(contracts.read_json(OWNER / 'registry.json'))
        features = {value['id']: value for value in registry['features']}
        expiry = features['remote.desktop.viewer.lease-expiry']
        self.assertEqual('linux-viewer', expiry['selector'])
        self.assertTrue(expiry['critical'])
        self.assertEqual([('failure', 'installed_e2e')],
                         [(case['scenario'], case['layer']) for case in expiry['cases']])
        happy = next(case for case in features['remote.desktop.viewer']['cases']
                     if case['scenario'] == 'happy' and case['layer'] == 'installed_e2e')
        self.assertIn('generated-physical-input', happy['required_assertions'])
        self.assertNotIn('generated-physical-input', expiry['cases'][0]['required_assertions'])

    def test_private_plan_digest_run_registry_and_review_choice_are_explicit(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'plan.json'
            registry_sha = contracts.digest_file(OWNER / 'registry.json')
            plan = {'schema': 'dockpipe.flatpak.remote-lease-expiry-plan/v1', 'run_id': 'generated',
                    'registry': {'sha256': registry_sha}, 'configuration': {'bundle': {'sha256': '1' * 64}},
                    'source_pins': []}
            path.write_text(json.dumps(plan))
            selected_sha = contracts.digest_file(path)
            with patch.object(remote_expiry_plan.importlib.util, 'module_from_spec') as factory:
                factory.return_value.host_profile.return_value = {'ID': 'generated', 'VERSION_ID': '1'}
                factory.return_value.observer_context.return_value = {'pid_namespace': 'generated', 'procfs_device': 17}
                with patch.object(remote_expiry_plan.importlib.util, 'spec_from_file_location') as spec:
                    target = remote_expiry_plan.prepare(path, selected_sha, 'generated', registry_sha)
                    self.assertFalse(target['drivers'][0]['configuration']['execute_reviewed'])
                    self.assertEqual('flatpak', target['kind'])
                    self.assertEqual(['remote.desktop.viewer.lease-expiry'], target['drivers'][0]['features'])
                    dependencies = target['drivers'][0]['dependencies']
                    self.assertTrue(all(item['path'].startswith('repo://') for item in dependencies))
            for digest, run, registry in [('0' * 64, 'generated', registry_sha),
                                          (selected_sha, 'old-run', registry_sha),
                                          (selected_sha, 'generated', '0' * 64)]:
                with self.assertRaises(ValueError):
                    remote_expiry_plan.prepare(path, digest, run, registry)

    def test_existing_preparer_preserves_defaults_and_rejects_unpaired_selection(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'default.json'
            command = [sys.executable, str(OWNER / 'assets/scripts/prepare-plan.py'),
                       '--portable-artifact', '/usr/bin/true', '--run-id', 'generated-default',
                       '--output', str(output)]
            result = subprocess.run(command, capture_output=True, text=True, timeout=10)
            self.assertEqual(0, result.returncode, result.stderr)
            plan = contracts.read_json(output)
            self.assertEqual(1, len(plan['targets']))
            self.assertFalse(any(driver['id'] == 'remote-view-expiry'
                                 for target in plan['targets'] for driver in target['drivers']))
            invalid = subprocess.run(command[:-1] + [str(Path(directory) / 'invalid.json'),
                                     '--remote-expiry-plan-sha256', '0' * 64],
                                     capture_output=True, text=True, timeout=10)
            self.assertNotEqual(0, invalid.returncode)
            self.assertIn('Select both the remote expiry plan and its exact digest', invalid.stderr)


if __name__ == '__main__':
    unittest.main()
