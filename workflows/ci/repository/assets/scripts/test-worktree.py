#!/usr/bin/env python3
"""Run the unchanged private Git clone/include contract with the selected CLI."""
import os
from pathlib import Path
import subprocess
import sys


def main():
    source = Path(__file__).resolve().parents[5]
    environment = dict(os.environ)
    selected = environment.get('DOCKPIPE_TEST_DOCKPIPE_BIN')
    if selected:
        binary = Path(selected).resolve(strict=True)
    else:
        bootstrap = environment['DOCKPIPE_BIN']
        artifacts = subprocess.check_output(
            [bootstrap, 'scope', 'artifacts', 'worktree-cli', '--workdir', str(source)],
            env=environment, text=True).strip()
        output = Path(artifacts)
        output.mkdir(parents=True, exist_ok=True)
        binary = output / ('dockpipe.exe' if sys.platform == 'win32' else 'dockpipe')
        subprocess.run([environment.get('DOCKPIPE_TEST_GO', 'go'), 'build', '-o', str(binary), './src/cmd'],
                       cwd=source, env=environment, check=True)
    environment['DOCKPIPE_TEST_DOCKPIPE_BIN'] = binary.as_posix()
    return subprocess.run(
        [sys.executable, str(source / 'workflows/shared/run.py'), 'bash',
         (source / 'tests/unit-tests/test_clone_worktree_include.sh').as_posix()],
        cwd=source, env=environment, check=False).returncode


if __name__ == '__main__':
    raise SystemExit(main())
