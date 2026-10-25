"""Platform phases preserve argv and never resolve Bash on non-Linux hosts."""
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[4]
SPECIFICATION = importlib.util.spec_from_file_location(
    'platform_phase', ROOT / 'workflows/build/platform/assets/scripts/run-phase.py')
phase = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(phase)


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

    def test_workflow_does_not_require_host_shell_variable_expansion(self):
        workflow = (ROOT / 'workflows/build/platform/config.yml').read_text()
        self.assertIn('run-phase.py cli', workflow)
        self.assertIn('run-phase.py linux-packages', workflow)
        self.assertNotIn('$PLATFORM_', workflow)
        self.assertNotIn('cmd: bash', workflow)


if __name__ == '__main__':
    unittest.main()
