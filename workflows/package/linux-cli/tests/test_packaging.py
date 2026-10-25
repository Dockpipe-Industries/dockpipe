"""Linux package phases keep explicit paths and fail before packaging a bad CLI."""
import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock

import yaml

ROOT = Path(__file__).resolve().parents[4]
SCRIPT = ROOT / 'workflows/package/linux-cli/assets/scripts/run.py'
SPECIFICATION = importlib.util.spec_from_file_location('linux_package_run', SCRIPT)
PACKAGING = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(PACKAGING)


class LinuxPackagingTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix='linux package spaces ')
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.source = self.root / 'source tree'
        self.source.mkdir()
        self.artifacts = self.root / 'artifact tree'
        self.output = self.root / 'package output'
        self.environment = {
            'DOCKPIPE_SOURCE_ROOT': str(self.source),
            'DOCKPIPE_ARTIFACT_ROOT': str(self.artifacts),
        }

    def test_exact_arguments_and_explicit_binary_survive_spaces_without_host_outputs(self):
        for architecture in ('amd64', 'arm64'):
            with self.subTest(architecture=architecture):
                with mock.patch.dict(os.environ, self.environment, clear=True):
                    with mock.patch.object(PACKAGING.subprocess, 'run') as run:
                        self.assertEqual(PACKAGING.main(['0.6.27', architecture, '--output', str(self.output)]), 0)
                self.assertEqual(run.call_count, 2)
                build, package = run.call_args_list
                self.assertEqual(build.args[0][2:], ['--target', 'linux/' + architecture,
                                 '--output', str(self.artifacts / ('dockpipe-linux-' + architecture)),
                                 '--version', '0.6.27'])
                self.assertEqual(package.args[0], ['bash', str(self.source / 'workflows/package/linux-cli/assets/scripts/build-deb.sh'),
                                                  '0.6.27', architecture, str(self.output)])
                for call in (build, package):
                    self.assertEqual(call.kwargs['env']['DOCKPIPE_RELEASE_BINARY'],
                                     str(self.artifacts / ('dockpipe-linux-' + architecture)))
                    self.assertNotIn('DOCKPIPE_STEP_OUTPUTS_FILE', call.kwargs['env'])
                    self.assertTrue(call.kwargs['check'])

    def test_compiler_failure_prevents_packaging(self):
        with mock.patch.dict(os.environ, self.environment, clear=True):
            with mock.patch.object(PACKAGING.subprocess, 'run', side_effect=subprocess.CalledProcessError(9, ['go'])) as run:
                with self.assertRaises(subprocess.CalledProcessError):
                    PACKAGING.main(['0.6.27'])
                self.assertEqual(run.call_count, 1)

    def test_invalid_identity_fails_before_any_phase(self):
        with mock.patch.dict(os.environ, self.environment, clear=True):
            with mock.patch.object(PACKAGING.subprocess, 'run') as run:
                with self.assertRaises(SystemExit):
                    PACKAGING.main(['0.6.27; echo unsafe'])
                run.assert_not_called()

    def test_migrated_host_workflows_do_not_require_container_outputs(self):
        for path in ('workflows/package/linux-cli/config.yml', 'workflows/build/platform/config.yml'):
            workflow = yaml.load((ROOT / path).read_text(), Loader=yaml.BaseLoader)
            self.assertEqual(len(workflow['steps']), 1)
            step = workflow['steps'][0]
            self.assertEqual(step['kind'], 'host')
            self.assertNotIn('outputs', step)
            self.assertNotIn('$LINUX_', step['cmd'])
            self.assertIn('/assets/scripts/run.py', step['cmd'])


if __name__ == '__main__':
    unittest.main()
