#!/usr/bin/env python3
"""Forward a workflow's JSON arguments to its explicit host command."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys


def select_windows_bash(environment):
    configured = environment.get('DOCKPIPE_HOST_BASH_BIN')
    if configured:
        selected = Path(configured)
        if not selected.is_absolute() or not selected.is_file():
            raise ValueError('DOCKPIPE_HOST_BASH_BIN must select an existing absolute Bash executable')
        return selected

    candidates = []
    git = shutil.which('git.exe', path=environment.get('PATH'))
    if git:
        installation = Path(git).resolve()
        for parent in list(installation.parents)[:3]:
            candidates.append(parent / 'bin/bash.exe')
    for key in ('ProgramFiles', 'ProgramFiles(x86)'):
        if environment.get(key):
            candidates.append(Path(environment[key]) / 'Git/bin/bash.exe')
    for candidate in candidates:
        if candidate.is_file():
            return candidate
    raise FileNotFoundError('Git for Windows Bash is required; set DOCKPIPE_HOST_BASH_BIN to its absolute executable')


def host_command(command, environment, platform):
    environment = dict(environment)
    if platform == 'win32' and command[0] == 'bash':
        bash = select_windows_bash(environment)
        command = [str(bash), *command[1:]]
        directories = [bash.parent]
        utilities = bash.parent.parent / 'usr/bin'
        if utilities.is_dir():
            directories.append(utilities)
        environment['PATH'] = os.pathsep.join([*(str(path) for path in directories), environment.get('PATH', '')])
        environment['DOCKPIPE_HOST_BASH_BIN'] = str(bash)
    return command, environment


def main():
    arguments = json.loads(os.environ.get('DOCKPIPE_ARGS_JSON', '[]'))
    if not isinstance(arguments, list) or any(not isinstance(value, str) for value in arguments):
        raise ValueError('Workflow arguments must be a JSON array of strings')
    command = sys.argv[1:]
    if not command:
        raise ValueError('An explicit host command is required')
    command, environment = host_command([*command, *arguments], os.environ, sys.platform)
    return subprocess.run(command, env=environment, check=False).returncode


if __name__ == '__main__':
    raise SystemExit(main())
