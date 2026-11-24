"""Optional Git registration preserves defaults and pins the owning fixture tools."""
import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch


OWNER = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(OWNER / 'assets/scripts'))
SPEC = importlib.util.spec_from_file_location('prepare_git_plan', OWNER / 'assets/scripts/prepare-plan.py')
PREPARE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PREPARE)


class PrepareGitTests(unittest.TestCase):
    def fixture(self, root):
        artifact = root / 'dockpipe'
        artifact.write_bytes(b'generated selected CLI')
        binary = root / 'broker'
        binary.write_bytes(b'generated fixture broker')
        git = root / 'git'
        git.write_bytes(b'generated selected Git')
        manifest = root / 'manifest.json'
        manifest.write_text(json.dumps({
            'schema': 'dockpipe.product-git-fixture-build/v1', 'source_overlay_only': True,
            'configuration_fault_tests_passed': True, 'binary': str(binary),
            'binary_sha256': PREPARE.digest_file(binary), 'git': str(git),
            'git_sha256': PREPARE.digest_file(git)}))
        return artifact, binary, git, manifest

    def prepare(self, artifact, output, extra=()):
        arguments = ['prepare-plan.py', '--portable-artifact', str(artifact), '--run-id', 'generated-plan',
                     '--output', str(output), *extra]
        with patch.object(sys, 'argv', arguments):
            PREPARE.main()
        return json.loads(output.read_text())

    def test_default_plan_has_no_git_lane_and_keeps_existing_dependencies(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            artifact, _, _, _ = self.fixture(root)
            plan = self.prepare(artifact, root / 'plan.json')
            drivers = plan['targets'][0]['drivers']
            self.assertEqual([driver['id'] for driver in drivers],
                             ['portable', 'package-authoring', 'observability', 'lifecycle-host',
                              'init-refusal', 'host-failure', 'host-cancellation', 'host-recovery', 'policy-refusal'])
            self.assertEqual([item['path'] for item in drivers[0]['dependencies']],
                             ['assets/scripts/contracts.py', 'assets/scripts/process.py', 'drivers/workspace_files.py'])
            host_failure, = [driver for driver in drivers if driver['id'] == 'host-failure']
            self.assertEqual(host_failure['features'], ['workflow.host'])
            self.assertEqual(host_failure['scenarios'], ['failure'])
            self.assertEqual(host_failure['layers'], ['component'])
            host_cancellation, = [driver for driver in drivers if driver['id'] == 'host-cancellation']
            self.assertEqual(host_cancellation['features'], ['workflow.host'])
            self.assertEqual(host_cancellation['scenarios'], ['cancellation'])
            self.assertEqual(host_cancellation['layers'], ['component'])
            host_recovery, = [driver for driver in drivers if driver['id'] == 'host-recovery']
            self.assertEqual(host_recovery['features'], ['workflow.host'])
            self.assertEqual(host_recovery['scenarios'], ['restart-recovery'])
            self.assertEqual(host_recovery['layers'], ['component'])

    def test_explicit_manifest_adds_exact_pin_and_visible_cancellation_case(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            artifact, _, _, manifest = self.fixture(root)
            default = self.prepare(artifact, root / 'default.json')
            selected = self.prepare(artifact, root / 'selected.json', ['--git-fixture-manifest', str(manifest)])
            before = default['targets'][0]['drivers']
            after = selected['targets'][0]['drivers']
            self.assertEqual(after[:-1], before)
            git = after[-1]
            self.assertEqual(git['id'], 'git-workspace')
            self.assertEqual(git['configuration'], {'artifact': str(artifact), 'fixture_manifest': str(manifest),
                                                  'fixture_manifest_sha256': PREPARE.digest_file(manifest)})
            self.assertEqual(git['scenarios'], ['happy', 'failure', 'cancellation'])
            self.assertIn('fixtures/workspace-git/broker_test.go', [item['path'] for item in git['dependencies']])

    def test_changed_broker_or_git_is_refused_before_plan_write(self):
        for selected in ['binary', 'git']:
            with self.subTest(selected=selected), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                artifact, binary, git, manifest = self.fixture(root)
                (binary if selected == 'binary' else git).write_bytes(b'changed after producer receipt')
                output = root / 'plan.json'
                with self.assertRaisesRegex(ValueError, 'differs from its producer pin'):
                    self.prepare(artifact, output, ['--git-fixture-manifest', str(manifest)])
                self.assertFalse(output.exists())

    def test_unknown_or_malformed_producer_manifest_is_refused(self):
        for value in [[], {'schema': 'another/v1'},
                      {'schema': 'dockpipe.product-git-fixture-build/v1', 'source_overlay_only': True,
                       'configuration_fault_tests_passed': True, 'binary': None}]:
            with self.subTest(value=value), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                artifact, _, _, manifest = self.fixture(root)
                manifest.write_text(json.dumps(value))
                output = root / 'plan.json'
                with self.assertRaises(ValueError):
                    self.prepare(artifact, output, ['--git-fixture-manifest', str(manifest)])
                self.assertFalse(output.exists())


if __name__ == '__main__':
    unittest.main()
