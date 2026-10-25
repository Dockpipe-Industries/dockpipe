#!/usr/bin/env python3
"""Run generic Go unit tests outside the invoking workflow's runtime context."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile

INTENTIONAL_INPUTS = ('DOCKPIPE_TEST_', 'DOCKPIPE_BOOTSTRAP_', 'DOCKPIPE_GO_SDK_')


def unit_environment(environment, root):
    selected = {key: value for key, value in environment.items()
                if not key.startswith('DOCKPIPE_') or key.startswith(INTENTIONAL_INPUTS)}
    for key, directory in (('DOCKPIPE_GLOBAL_ROOT', 'global'), ('DOCKPIPE_SYSTEM_ROOT', 'system')):
        path = root / directory
        path.mkdir()
        selected[key] = str(path)
    return selected


def main():
    source = Path(os.environ.get('DOCKPIPE_SOURCE_ROOT', os.getcwd())).resolve(strict=True)
    go = os.environ.get('DOCKPIPE_TEST_GO', 'go')
    arguments = sys.argv[1:] or ['./...']
    with tempfile.TemporaryDirectory(prefix='dockpipe-go-unit-') as temporary:
        environment = unit_environment(os.environ, Path(temporary))
        return subprocess.run([go, 'test', *arguments], cwd=source, env=environment, check=False).returncode


if __name__ == '__main__':
    raise SystemExit(main())
