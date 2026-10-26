"""Workflow-owned Windows shell selection never falls through to WSL Bash."""
import importlib.util
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).resolve().parents[1] / 'run.py'
SPECIFICATION = importlib.util.spec_from_file_location('workflow_host_command', SCRIPT)
HOST = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(HOST)


class WorkflowHostBashTests(unittest.TestCase):
    def test_windows_selects_git_installation_even_when_path_bash_is_wsl(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            git = root / 'Git/cmd/git.exe'
            bash = root / 'Git/bin/bash.exe'
            utilities = root / 'Git/usr/bin'
            git.parent.mkdir(parents=True)
            bash.parent.mkdir(parents=True)
            utilities.mkdir(parents=True)
            git.touch()
            bash.touch()
            environment = {'PATH': str(root / 'Windows/System32')}
            with patch.object(HOST.shutil, 'which', return_value=str(git)) as lookup:
                command, selected = HOST.host_command(['bash', 'script with spaces.sh', 'one argument'], environment, 'win32')
            lookup.assert_called_once_with('git.exe', path=environment['PATH'])
            self.assertEqual(command, [str(bash), 'script with spaces.sh', 'one argument'])
            self.assertEqual(selected['DOCKPIPE_HOST_BASH_BIN'], str(bash))
            self.assertEqual(selected['PATH'].split(os.pathsep)[:2], [str(bash.parent), str(utilities)])
            self.assertEqual(environment, {'PATH': str(root / 'Windows/System32')})

    def test_explicit_shell_precedes_discovery_and_invalid_selection_fails(self):
        with tempfile.TemporaryDirectory() as temporary:
            bash = Path(temporary) / 'selected bash.exe'
            bash.touch()
            with patch.object(HOST.shutil, 'which') as lookup:
                selected = HOST.select_windows_bash({'DOCKPIPE_HOST_BASH_BIN': str(bash)})
            self.assertEqual(selected, bash)
            lookup.assert_not_called()
        with self.assertRaises(ValueError):
            HOST.select_windows_bash({'DOCKPIPE_HOST_BASH_BIN': 'relative/bash.exe'})

    def test_missing_git_bash_fails_without_requesting_path_bash_or_wsl(self):
        with patch.object(HOST.shutil, 'which', return_value=None) as lookup:
            with self.assertRaises(FileNotFoundError):
                HOST.select_windows_bash({'PATH': '/Windows/System32'})
        lookup.assert_called_once_with('git.exe', path='/Windows/System32')

    def test_program_files_git_installation_is_supported(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            bash = root / 'Git/bin/bash.exe'
            bash.parent.mkdir(parents=True)
            bash.touch()
            with patch.object(HOST.shutil, 'which', return_value=None):
                self.assertEqual(HOST.select_windows_bash({'ProgramFiles': str(root)}), bash)

    def test_other_commands_and_platforms_preserve_argv(self):
        for platform, command in [('win32', ['python3', 'test.py']), ('linux', ['bash', 'test.sh'])]:
            environment = {'PATH': '/selected/tools'}
            selected, child = HOST.host_command(command, environment, platform)
            self.assertEqual(selected, command)
            self.assertEqual(child, environment)

    def test_actual_forwarder_preserves_child_failure_and_json_arguments(self):
        environment = dict(os.environ, DOCKPIPE_ARGS_JSON='["argument with spaces"]')
        code = 'import sys; assert sys.argv[1:] == ["argument with spaces"]; sys.exit(37)'
        result = subprocess.run([sys.executable, str(SCRIPT), sys.executable, '-c', code],
                                env=environment, capture_output=True, text=True)
        self.assertEqual(result.returncode, 37, result.stderr)


if __name__ == '__main__':
    unittest.main()
