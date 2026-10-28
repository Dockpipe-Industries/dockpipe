"""Required host qualification cannot become an optional-package or skipped-job gate."""
import json
import os
from pathlib import Path
import subprocess
import sys
import unittest
from unittest.mock import patch

import yaml

WORKFLOW = Path(__file__).resolve().parents[4] / '.github/workflows/native-macos-symbols.yml'


def workflow():
    return yaml.safe_load(WORKFLOW.read_text())


class NativeHostGatewayTests(unittest.TestCase):
    def test_every_protected_pr_and_dispatch_emits_the_host_gateway(self):
        document = workflow()
        events = document.get('on', document.get(True))
        self.assertEqual(events['pull_request'], {'branches': ['dev', 'staging', 'master']})
        self.assertIn('workflow_dispatch', events)
        gateway = document['jobs']['required-host']
        self.assertEqual(gateway['name'], 'Native desktop host qualification')
        self.assertIn('always()', gateway['if'])
        self.assertEqual(gateway['permissions'], {})
        self.assertEqual(set(gateway['needs']), {'owned-symbols', 'windows-terminal', 'flatpak-host'})

    def test_real_native_assertions_and_standalone_flatpak_are_required(self):
        jobs = workflow()['jobs']
        self.assertEqual(jobs['owned-symbols']['strategy']['matrix']['runner'],
                         ['macos-15', 'macos-15-intel'])
        windows = (WORKFLOW.parents[2] / 'workflows/qualify/native-desktop/assets/scripts/windows.ps1').read_text()
        adapter = '\n'.join(step.get('run', '') for step in jobs['windows-terminal']['steps'])
        self.assertIn('workflows/qualify/native-desktop/config.yml -- --platform windows', adapter)
        self.assertIn('--target terminal-session-win-test hardware-key-transport-test', windows)
        self.assertIn("-R '^(terminal-session-win|hardware-key-transport)$'", windows)
        self.assertIn('--output-junit $transportReport', windows)
        self.assertIn('--require terminal-session-win --require hardware-key-transport', windows)
        self.assertIn("-R '^desktop-app-window-win$'", windows)
        self.assertIn('--stop-on-failure', windows)
        self.assertEqual(jobs['flatpak-host']['uses'], './.github/workflows/flatpak.yml')
        self.assertEqual(jobs['flatpak-host']['needs'], 'host-version')
        self.assertNotIn('secrets', jobs['flatpak-host'])
        self.assertEqual(jobs['flatpak-host']['permissions'], {'contents': 'read', 'attestations': 'read'})
        self.assertFalse(any('cockpit' in name.lower() for name in jobs['required-host']['needs']))

    def test_gateway_executes_and_rejects_each_failed_skipped_cancelled_or_missing_dependency(self):
        gate = workflow()['jobs']['required-host']
        shell = gate['steps'][0]['run']
        code = shell.split("<<'PYTHON'\n", 1)[1].rsplit('PYTHON', 1)[0]
        for dependency in gate['needs']:
            for result in ('success', 'failure', 'skipped', 'cancelled', 'missing'):
                results = {name: {'result': 'success'} for name in gate['needs']}
                if result == 'missing':
                    del results[dependency]
                else:
                    results[dependency]['result'] = result
                with self.subTest(dependency=dependency, result=result), \
                        patch.dict(os.environ, QUALIFICATION_RESULTS=json.dumps(results)):
                    execution = subprocess.run([sys.executable, '-c', code], capture_output=True, timeout=10)
                self.assertEqual(execution.returncode == 0, result == 'success')

    def test_no_native_matrix_job_or_assertion_can_ignore_or_skip_failure(self):
        jobs = workflow()['jobs']
        self.assertFalse(jobs['owned-symbols']['strategy']['fail-fast'])
        for name in ('owned-symbols', 'windows-terminal', 'flatpak-host'):
            job = jobs[name]
            self.assertNotIn('if', job)
            self.assertFalse(job.get('continue-on-error', False))
            for step in job.get('steps', []):
                self.assertFalse(step.get('continue-on-error', False))
                if 'ctest ' in step.get('run', ''):
                    self.assertNotIn('if', step)

    def test_flatpak_reuse_uses_the_checked_out_product_version_without_secrets(self):
        jobs = workflow()['jobs']
        self.assertEqual(jobs['flatpak-host']['with']['version'], '${{ needs.host-version.outputs.version }}')
        version = jobs['host-version']
        self.assertEqual(version['outputs']['version'], '${{ steps.version.outputs.version }}')
        self.assertIn('cat VERSION', version['steps'][1]['run'])
        self.assertEqual(version['permissions'], {'contents': 'read'})
        self.assertNotIn('secrets', jobs['flatpak-host'])

    def test_optional_package_checks_keep_separate_strict_entrypoints(self):
        for name in ('cockpit-sdk-compatibility.yml', 'flatpak-catalog.yml'):
            document = yaml.safe_load((Path(__file__).resolve().parents[4] / '.github/workflows' / name).read_text())
            events = document.get('on', document.get(True))
            self.assertIn('workflow_dispatch', events)
            self.assertIn('pull_request', events)
            self.assertIn('paths', events['pull_request'])
        serialized = json.dumps(workflow()['jobs']['required-host'])
        self.assertNotIn('cockpit', serialized.lower())
        self.assertNotIn('catalog', serialized.lower())

    def test_reused_flatpak_job_keeps_required_browser_terminal_and_docker_checks(self):
        document = yaml.safe_load((Path(__file__).resolve().parents[4] / '.github/workflows/flatpak.yml').read_text())
        job = document['jobs']['desktop']
        self.assertFalse(job.get('continue-on-error', False))
        steps = {step.get('name'): step for step in job['steps']}
        for name in ('Prepare exact attested Flatpak Browser SDK',
                     'Build and test Flatpak desktop and generic bundled dependency',
                     'Qualify installed Flatpak host terminals',
                     'Qualify real Docker and Compose bind mounts'):
            self.assertIn(name, steps)
            self.assertFalse(steps[name].get('continue-on-error', False))
            self.assertNotIn('if', steps[name])


if __name__ == '__main__':
    unittest.main()
