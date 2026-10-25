#!/usr/bin/env python3
"""Admit platform inputs once and pass explicit state to each owned phase."""
import importlib.util
import os
from pathlib import Path
import subprocess
import sys


SCRIPTS = Path(__file__).resolve().parent
SPECIFICATION = importlib.util.spec_from_file_location('platform_prepare', SCRIPTS / 'prepare.py')
PREPARE = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(PREPARE)


def main(arguments=None):
    environment = dict(os.environ)
    environment.update(PREPARE.prepare(arguments))
    source = Path(environment['DOCKPIPE_SOURCE_ROOT']).resolve(strict=True)
    commands = (
        [sys.executable, str(SCRIPTS / 'run-phase.py'), 'cli'],
        [sys.executable, str(SCRIPTS / 'assemble.py')],
        [sys.executable, str(SCRIPTS / 'run-phase.py'), 'linux-packages'],
    )
    for command in commands:
        subprocess.run(command, cwd=source, env=environment, check=True)
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
