"""Execute dry-run admission and build argument routing without native builds."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

import yaml

REPOSITORY = Path(__file__).resolve().parents[4]

import importlib.util
_contract_spec = importlib.util.spec_from_file_location(
    'workflow_contract_testing', Path(__file__).resolve().parents[4] / 'workflows/shared/contract_testing.py')
_contracts = importlib.util.module_from_spec(_contract_spec)
_contract_spec.loader.exec_module(_contracts)


def expanded(command):
    return _contracts.expanded_run(Path(__file__).resolve().parents[4], command)



class ReleaseQualificationTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workflow = yaml.load((Path(__file__).resolve().parents[4] / '.github/workflows/release.yml').read_text(), Loader=yaml.BaseLoader)

    def meta(self, **changes):
        script = next(step['run'] for step in self.workflow['jobs']['meta']['steps'] if step.get('id') == 'm')
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'VERSION').write_text('0.6.0\n')
            (root / 'release/releasenotes').mkdir(parents=True)
            (root / 'release/releasenotes/0.6.0.md').write_text('Notes\n')
            environment = dict(os.environ, GITHUB_EVENT_NAME='workflow_dispatch', GITHUB_REF='refs/heads/dev',
                               INPUT_DRY_RUN='true', INPUT_BUILD_MSI='true', INPUT_STAGING='false',
                               INPUT_QUALIFY_PACKAGE_DISPLAY='false', RELEASE_ENABLED='true', INPUT_VERSION='',
                               GITHUB_RUN_ID='123', GITHUB_RUN_ATTEMPT='1', GITHUB_SHA='a' * 40,
                               GITHUB_OUTPUT=str(root / 'outputs'))
            environment.update(changes)
            result = subprocess.run(['/bin/bash', '-c', script], cwd=root, env=environment,
                                    capture_output=True, text=True, timeout=10)
            outputs = (root / 'outputs').read_text() if (root / 'outputs').exists() else ''
            return result, outputs

    def test_manual_default_off_and_explicit_dry_run_on(self):
        self.assertEqual(self.workflow['on']['workflow_dispatch']['inputs']['qualify_package_display']['default'], 'false')
        for value in ('false', 'true'):
            result, outputs = self.meta(INPUT_QUALIFY_PACKAGE_DISPLAY=value)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn('qualify_package_display=' + value + '\n', outputs)
            self.assertIn('dry_run=true\n', outputs)

    def test_enabling_on_production_staging_or_nonmanual_event_fails(self):
        for changes in [dict(INPUT_DRY_RUN='false', GITHUB_REF='refs/heads/master'),
                        dict(GITHUB_EVENT_NAME='push', GITHUB_REF='refs/heads/master'),
                        dict(INPUT_STAGING='true', GITHUB_EVENT_NAME='push', GITHUB_REF='refs/heads/staging')]:
            result, outputs = self.meta(INPUT_QUALIFY_PACKAGE_DISPLAY='true', **changes)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('explicit manual dry run', result.stdout)
            self.assertNotIn('qualify_package_display=true', outputs)

    def test_ordinary_production_and_staging_remain_off(self):
        for changes in [dict(INPUT_DRY_RUN='false', GITHUB_REF='refs/heads/master'),
                        dict(INPUT_STAGING='true', GITHUB_EVENT_NAME='push', GITHUB_REF='refs/heads/staging')]:
            result, outputs = self.meta(**changes)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn('qualify_package_display=false\n', outputs)
            self.assertIn('dry_run=false\n', outputs)

    def fixture(self, root, script):
        directory = root / 'workflows/build/desktop/assets/scripts'
        directory.mkdir(parents=True)
        shutil.copy2(Path(__file__).resolve().parents[4] / 'workflows/build/desktop/assets/scripts' / script, directory / script)
        browser = root / 'workflows/dependencies/browser-sdk/assets/scripts/browser-runtime.py'
        browser.parent.mkdir(parents=True)
        browser.write_text('pass\n')
        (root / 'sdk').mkdir()
        tools = root / 'tools'
        tools.mkdir()
        return directory / script, tools

    def executable(self, path, content):
        path.write_text('#!/bin/sh\n' + content)
        path.chmod(0o755)

    def test_unix_build_passes_explicit_cmake_on_or_off(self):
        for enabled in (False, True):
            with self.subTest(enabled=enabled), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                script, tools = self.fixture(root, 'build-unix.sh')
                self.executable(tools / 'uname', 'if [ "$1" = -m ]; then echo arm64; else echo Darwin; fi\n')
                self.executable(tools / 'cmake', 'printf "%s\\n" "$@" > "$CAPTURE"\nexit 77\n')
                environment = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ['PATH'],
                                   DOCKPIPE_BROWSER_SDK=str(root / 'sdk'), CAPTURE=str(root / 'cmake-args'))
                for key in ('DOCKPIPE_PRIVATE_DEBUG_DIR', 'DOCKPIPE_BUILD_COCKPIT', 'DOCKPIPE_QUALIFY_COCKPIT'):
                    environment.pop(key, None)
                command = ['/bin/bash', str(script)] + (['--enable-qualification'] if enabled else []) + ['0.6.27', str(root / 'artifacts')]
                result = subprocess.run(command, env=environment, capture_output=True, text=True, timeout=10)
                self.assertEqual(result.returncode, 77, result.stdout + result.stderr)
                arguments = (root / 'cmake-args').read_text().splitlines()
                self.assertIn('-DDOCKPIPE_ENABLE_QUALIFICATION=' + ('ON' if enabled else 'OFF'), arguments)
                self.assertIn('-DDOCKPIPE_RELEASE_VERSION=0.6.27', arguments)

    def test_linux_wrapper_forwards_only_explicit_option_to_container(self):
        for enabled in (False, True):
            with self.subTest(enabled=enabled), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                script, tools = self.fixture(root, 'build-linux.sh')
                self.executable(tools / 'uname', 'echo x86_64\n')
                self.executable(tools / 'docker', 'if [ "$1" = run ]; then printf "%s\\n" "$@" > "$CAPTURE"; fi\n')
                environment = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ['PATH'],
                                   DOCKPIPE_BROWSER_SDK=str(root / 'sdk'), CAPTURE=str(root / 'docker-args'))
                for key in ('DOCKPIPE_PRIVATE_DEBUG_DIR', 'DOCKPIPE_QUALIFY_COCKPIT'):
                    environment.pop(key, None)
                command = ['/bin/bash', str(script)] + (['--enable-qualification'] if enabled else []) + ['0.6.27', str(root / 'artifacts')]
                result = subprocess.run(command, env=environment, capture_output=True, text=True, timeout=10)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                arguments = (root / 'docker-args').read_text().splitlines()
                self.assertEqual('--enable-qualification' in arguments, enabled)
                self.assertIn('/source/workflows/build/desktop/assets/scripts/build-unix.sh', arguments)

    def test_workflow_routes_admitted_output_to_existing_windows_and_unix_seams(self):
        jobs = self.workflow['jobs']
        windows = next(step for step in jobs['build-msi']['steps'] if step.get('name') == 'Build Windows desktop runtime')
        self.assertEqual(windows['env']['QUALIFY_PACKAGE_DISPLAY'], '${{ needs.meta.outputs.qualify_package_display }}')
        self.assertIn("-EnableQualification:($env:QUALIFY_PACKAGE_DISPLAY -eq 'true')", expanded(windows['run']))
        for title in ('Build Linux desktop against Ubuntu 22.04', 'Build native macOS desktop installers'):
            step = next(step for step in jobs['build-unix']['steps'] if step.get('name') == title)
            self.assertEqual(step['env']['QUALIFY_PACKAGE_DISPLAY'], '${{ needs.meta.outputs.qualify_package_display }}')
            if title == 'Build native macOS desktop installers':
                self.assertIn('set -- "$@" --enable-qualification', expanded(step['run']))
            else:
                self.assertIn('qualification_args+=(--enable-qualification)', expanded(step['run']))
        for job in ('publish', 'publish-staging'):
            self.assertIn("needs.meta.outputs.dry_run == 'false'", jobs[job]['if'])


if __name__ == '__main__':
    unittest.main()
