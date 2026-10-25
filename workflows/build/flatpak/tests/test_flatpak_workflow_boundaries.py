"""Required Flatpak host and explicit optional-package qualification boundaries."""
import hashlib
import importlib.util
import json
import importlib.util
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

import yaml


ROOT = Path(__file__).resolve().parents[4]
HOST = Path(__file__).resolve().parents[4] / '.github/workflows/flatpak.yml'
OPTIONAL = Path(__file__).resolve().parents[4] / '.github/workflows/flatpak-catalog.yml'
BUILD = Path(__file__).resolve().parents[4] / 'workflows/build/flatpak/assets/scripts/build.sh'

specification = importlib.util.spec_from_file_location(
    'workflow_contract_testing', ROOT / 'workflows/shared/contract_testing.py')
contracts = importlib.util.module_from_spec(specification)
specification.loader.exec_module(contracts)


def workflow(path):
    return yaml.load(path.read_text(), Loader=yaml.BaseLoader)


class FlatpakWorkflowBoundariesTest(unittest.TestCase):
    def test_host_build_requires_browser_generic_extensions_and_host_runtime_checks(self):
        document = workflow(HOST)
        self.assertIn('workflow_call', document['on'])
        job = document['jobs']['desktop']
        script = '\n'.join(contracts.expanded_run(ROOT, step.get('run', '')) for step in job['steps'])
        for required in ('workflows/build/cli/config.yml', 'prepare-flatpak-browser-sdk.py',
                         'build.sh', 'build-package.sh', 'verify-installed-host.py', 'container-smoke.py'):
            self.assertIn(required, script)
        self.assertNotIn('package compile core', script)
        self.assertNotIn('--only core', script)
        for optional in ('ingest-pinned-stores.py', 'prepare-cockpit.py', 'build-catalog.py',
                         'verify-installed-cockpit.py', 'verify-marketplace-cockpit.py',
                         'DOCKPIPE_FLATPAK_COCKPIT_ASSETS'):
            self.assertNotIn(optional, script)
        stage = next(step['run'] for step in job['steps'] if step.get('name') == 'Stage tested release artifacts')
        self.assertNotIn('core-store', stage)
        self.assertNotIn('flatpak-catalog', stage)
        uploads = [step['with'] for step in job['steps'] if 'upload-artifact@' in step.get('uses', '')]
        self.assertIn('release-flatpak', [item['name'] for item in uploads])
        self.assertIn('flatpak-host-qualification-amd64', [item['name'] for item in uploads])

    def test_optional_lane_preserves_pinned_installed_catalog_and_cockpit_checks(self):
        document = workflow(OPTIONAL)
        self.assertNotIn('workflow_call', document['on'])
        self.assertIn('workflow_dispatch', document['on'])
        script = '\n'.join(contracts.expanded_run(ROOT, step.get('run', '')) for step in document['jobs']['catalog']['steps'])
        for required in ('ingest-pinned-stores.py', '--lock workflows/catalog/compose/assets/pinned-stores.lock.json',
                         'prepare-cockpit.py prepare', '--lock workflows/qualify/cockpit/assets/cockpit.lock.json',
                         'verify-installed-cockpit.py', 'prepare-package-tools.py', '--finalize-cockpit',
                         'build-catalog.py', 'verify-marketplace-cockpit.py', 'container-smoke.py',
                         'verify-installed-host.py', 'build-package.sh'):
            self.assertIn(required, script)
        self.assertNotIn('release/artifacts', script)
        uploads = [step['with'] for step in document['jobs']['catalog']['steps']
                   if 'upload-artifact@' in step.get('uses', '')]
        self.assertEqual([item['name'] for item in uploads], ['flatpak-optional-catalog-qualification-amd64'])
        self.assertIn('flatpak-catalog/store/', uploads[0]['path'])
        self.assertIn('flatpak-cockpit-qualification/', uploads[0]['path'])
        self.assertIn('flatpak-marketplace-cockpit/', uploads[0]['path'])

    def test_host_cmake_keeps_generic_plugin_tests_and_only_real_cockpit_is_conditional(self):
        cmake = (Path(__file__).resolve().parents[4] / 'src/app/tooling/dockpipe-launcher/CMakeLists.txt').read_text()
        conditional = cmake.index('if(DOCKPIPE_COCKPIT_ASSETS)')
        self.assertLess(cmake.index('add_test(NAME package-native-display '), conditional)
        build = BUILD.read_text()
        self.assertIn('--label-exclude installed-platform', build)
        self.assertIn('DOCKPIPE_FLATPAK_BROWSER_SDK:?', build)
        self.assertNotIn('DOCKPIPE_FLATPAK_COCKPIT_ASSETS:?', build)


class FlatpakReleaseStageTest(unittest.TestCase):
    def test_parsed_stage_only_publishes_tested_standalone_bundle(self):
        document = workflow(HOST)
        stage = next(step['run'] for step in document['jobs']['desktop']['steps']
                     if step.get('name') == 'Stage tested release artifacts')
        with tempfile.TemporaryDirectory(prefix='flatpak-release-stage-') as temporary:
            root = Path(temporary)
            runner = root / 'runner'
            flatpak = runner / 'flatpak'
            flatpak.mkdir(parents=True)
            bundle = 'dockpipe-desktop_1.2.3_linux_amd64.flatpak'
            (flatpak / bundle).write_bytes(b'qualified desktop fixture')
            result = subprocess.run(['bash', '-c', stage], cwd=root,
                                    env=dict(os.environ, RUNNER_TEMP=str(runner)),
                                    capture_output=True, text=True, timeout=15)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual((root / 'release/artifacts' / bundle).read_bytes(), b'qualified desktop fixture')
            self.assertFalse((root / 'release/artifacts/stores').exists())
            helper = ROOT / 'workflows/release/metadata/assets/scripts/release-artifacts.py'
            specification = importlib.util.spec_from_file_location('stage_release_artifacts', helper)
            verifier = importlib.util.module_from_spec(specification)
            specification.loader.exec_module(verifier)
            with patch.object(verifier, 'PLATFORMS', ()):
                verifier.prepare(root / 'release/artifacts', '1.2.3')
            catalog = json.loads((root / 'release/artifacts/release-manifest.json').read_text())
            self.assertEqual(catalog['stores'], {})



class FlatpakConfigureRoutingTest(unittest.TestCase):
    """Capture only configure arguments; stubs do not qualify any runtime bytes."""
    def configure(self, optional):
        with tempfile.TemporaryDirectory(prefix='flatpak-configure-routing-') as temporary:
            root = Path(temporary)
            tools = root / 'tools'
            tools.mkdir()
            browser = root / 'browser'
            browser.mkdir()
            store = root / 'core-store'
            store.mkdir()
            fixture = root / 'optional assets'
            fixture.mkdir()
            cli = root / 'dockpipe'
            cli.write_text('#!/bin/sh\nprintf "1.2.3\\n"\n')
            cli.chmod(0o755)
            capture = root / 'configure.json'
            scripts = {
                'python3': '#!/bin/sh\nexit 0\n',
                'sha256sum': '#!/bin/sh\ncat >/dev/null\n',
                'curl': '#!/bin/sh\nexit 0\n',
                'tar': '#!/bin/sh\nexit 0\n',
                'flatpak': f'''#!{sys.executable}
import json, os, pathlib, sys
arguments = sys.argv[1:]
if arguments == ['--default-arch']:
    print('x86_64')
if 'cmake' in arguments and '-S' in arguments:
    pathlib.Path(os.environ['CONFIGURE_CAPTURE']).write_text(json.dumps(arguments))
    sys.exit(97)
''',
            }
            for name, text in scripts.items():
                tool = tools / name
                tool.write_text(text)
                tool.chmod(0o755)
            # The real script installs the source license before configure.
            # Seed that one file instead of invoking any network or extractor.
            output = root / 'output'
            environment = dict(os.environ, PATH=str(tools) + os.pathsep + os.defpath,
                               HOME=str(root), DOCKPIPE_FLATPAK_BROWSER_SDK=str(browser),
                               CONFIGURE_CAPTURE=str(capture))
            environment.pop('DOCKPIPE_FLATPAK_COCKPIT_ASSETS', None)
            if optional:
                environment['DOCKPIPE_FLATPAK_COCKPIT_ASSETS'] = str(fixture)
            # mkdir is reached before the extraction stub. The stub can populate
            # its task-owned license path once output has been created.
            (tools / 'tar').write_text(f'''#!{sys.executable}
from pathlib import Path
import sys
arguments = sys.argv[1:]
destination = Path(arguments[arguments.index('-C') + 1]) / 'libXres-1.2.3'
destination.mkdir(parents=True)
(destination / 'COPYING').write_text('fixture license')
''')
            result = subprocess.run(['bash', str(BUILD), '1.2.3', str(cli), str(store), str(output)],
                                    cwd=ROOT, env=environment, capture_output=True, text=True, timeout=15)
            self.assertEqual(result.returncode, 97, result.stderr)
            return json.loads(capture.read_text()), str(fixture)

    def test_host_configures_without_any_optional_package_fixture(self):
        arguments, _ = self.configure(False)
        self.assertFalse(any('DOCKPIPE_COCKPIT_ASSETS' in item for item in arguments))
        self.assertTrue(any('CMAKE_PREFIX_PATH=' in item for item in arguments))

    def test_explicit_optional_fixture_adds_its_read_only_mount_and_cmake_path(self):
        arguments, fixture = self.configure(True)
        self.assertIn('--filesystem=' + fixture + ':ro', arguments)
        self.assertIn('-DDOCKPIPE_COCKPIT_ASSETS=' + fixture, arguments)


if __name__ == '__main__':
    unittest.main()
