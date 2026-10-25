"""Platform phases preserve argv and never resolve Bash on non-Linux hosts."""
import importlib.util
import json
import os
import subprocess
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[4]
SPECIFICATION = importlib.util.spec_from_file_location(
    'platform_phase', ROOT / 'workflows/build/platform/assets/scripts/run-phase.py')
phase = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(phase)
RUN_SPECIFICATION = importlib.util.spec_from_file_location(
    'platform_run', ROOT / 'workflows/build/platform/assets/scripts/run.py')
runner = importlib.util.module_from_spec(RUN_SPECIFICATION)
RUN_SPECIFICATION.loader.exec_module(runner)


class PlatformPhaseTests(unittest.TestCase):
    def test_windows_and_macos_skip_linux_packaging_before_any_process_or_path_access(self):
        for system in ('windows', 'darwin'):
            with self.subTest(system=system), patch.dict(os.environ, {'PLATFORM_SYSTEM': system}, clear=True), \
                    patch.object(phase.subprocess, 'run') as command:
                self.assertEqual(phase.main(['linux-packages']), 0)
                command.assert_not_called()

    def test_cli_child_receives_actual_environment_values_as_opaque_argv(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            producer = root / 'workflows/build/cli/assets/scripts/build.py'
            producer.parent.mkdir(parents=True)
            producer.write_text('import json,os,sys\nfrom pathlib import Path\n'
                                'Path(os.environ["RECORDED_ARGS"]).write_text(json.dumps(sys.argv[1:]))\n'
                                'raise SystemExit(23)\n')
            output = root / 'arguments.json'
            binary = r'C:\release artifacts\dockpipe.exe'
            environment = {'PLATFORM_SYSTEM': 'windows', 'DOCKPIPE_SOURCE_ROOT': str(root),
                           'PLATFORM_BINARY': binary, 'PLATFORM_VERSION': '0.6.32',
                           'RECORDED_ARGS': str(output)}
            with patch.dict(os.environ, environment, clear=True):
                self.assertEqual(phase.main(['cli']), 23)
            self.assertEqual(json.loads(output.read_text()), ['--output', binary, '--version', '0.6.32'])

    def test_linux_packaging_keeps_its_explicit_shell_and_child_failure(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            with patch.dict(os.environ, {'PLATFORM_SYSTEM': 'linux', 'DOCKPIPE_SOURCE_ROOT': str(root)}, clear=True), \
                    patch.object(phase.subprocess, 'run') as command:
                command.return_value.returncode = 17
                self.assertEqual(phase.main(['linux-packages']), 17)
            self.assertEqual(command.call_args.args[0],
                             ['bash', str(root / 'workflows/build/platform/assets/scripts/package-linux.sh')])

    def test_owner_passes_prepared_state_directly_to_every_phase(self):
        with tempfile.TemporaryDirectory(prefix='platform source spaces ') as temporary:
            prepared = {'PLATFORM_SYSTEM': 'windows', 'PLATFORM_BINARY': r'C:\release artifacts\dockpipe.exe',
                        'PLATFORM_VERSION': '0.6.32'}
            with patch.dict(os.environ, {'DOCKPIPE_SOURCE_ROOT': temporary}, clear=True):
                with patch.object(runner.PREPARE, 'prepare', return_value=prepared) as prepare:
                    with patch.object(runner.subprocess, 'run') as command:
                        self.assertEqual(runner.main(['0.6.32']), 0)
            prepare.assert_called_once_with(['0.6.32'])
            self.assertEqual(len(command.call_args_list), 3)
            phases = [call.args[0][1:] for call in command.call_args_list]
            self.assertEqual(phases, [[str(runner.SCRIPTS / 'run-phase.py'), 'cli'],
                                      [str(runner.SCRIPTS / 'assemble.py')],
                                      [str(runner.SCRIPTS / 'run-phase.py'), 'linux-packages']])
            for call in command.call_args_list:
                for key, value in prepared.items():
                    self.assertEqual(call.kwargs['env'][key], value)
                self.assertNotIn('DOCKPIPE_STEP_OUTPUTS_FILE', call.kwargs['env'])
                self.assertTrue(call.kwargs['check'])

    def test_failed_phase_prevents_assembly_and_packaging(self):
        with tempfile.TemporaryDirectory() as temporary:
            with patch.dict(os.environ, {'DOCKPIPE_SOURCE_ROOT': temporary}, clear=True):
                with patch.object(runner.PREPARE, 'prepare', return_value={}):
                    with patch.object(runner.subprocess, 'run', side_effect=subprocess.CalledProcessError(7, ['cli'])) as command:
                        with self.assertRaises(subprocess.CalledProcessError):
                            runner.main(['0.6.32'])
                        self.assertEqual(command.call_count, 1)

    def test_workflow_does_not_require_host_shell_variable_expansion(self):
        workflow = (ROOT / 'workflows/build/platform/config.yml').read_text()
        self.assertIn('workflows/build/platform/assets/scripts/run.py', workflow)
        self.assertNotIn('outputs:', workflow)
        self.assertNotIn('$PLATFORM_', workflow)
        self.assertNotIn('cmd: bash', workflow)


if __name__ == '__main__':
    unittest.main()
