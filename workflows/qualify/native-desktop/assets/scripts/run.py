#!/usr/bin/env python3
"""Dispatch the owning host's real native desktop qualification."""
import argparse
import os
from pathlib import Path
import platform
import subprocess
import sys

SCRIPTS = Path(__file__).resolve().parent


def main(arguments=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--platform', choices=('macos', 'windows'))
    parser.add_argument('--jobs', type=int, default=2)
    parser.add_argument('--check', action='store_true', help='Run dispatch contract tests without native builds')
    options = parser.parse_args(arguments)
    if options.jobs < 1:
        parser.error('--jobs must be positive')
    if options.check:
        subprocess.run(
            [sys.executable, '-m', 'unittest', 'discover', '-s', str(SCRIPTS.parent.parent / 'tests'), '-v'],
            check=True,
        )
        subprocess.run(
            [sys.executable, '-m', 'unittest', 'discover', '-s', str(SCRIPTS.parents[3] / 'shared/tests'),
             '-p', 'test_ctest_result.py', '-v'],
            check=True,
        )
        return
    host = {'Darwin': 'macos', 'Windows': 'windows'}.get(platform.system())
    selected = options.platform or host
    if host is None or selected != host:
        parser.error('Native qualification must run on its matching macOS or Windows host')
    source = Path(os.environ['DOCKPIPE_SOURCE_ROOT']).resolve(strict=True)
    environment = dict(os.environ)
    temporary = environment.get('RUNNER_TEMP')
    if not temporary:
        temporary = str(Path(environment['DOCKPIPE_ARTIFACT_ROOT']) / 'native-qualification')
    Path(temporary).mkdir(parents=True, exist_ok=True)
    environment['RUNNER_TEMP'] = temporary
    if selected == 'macos':
        command = ['bash', str(SCRIPTS / 'macos.sh'), str(options.jobs)]
    else:
        command = ['pwsh', '-NoLogo', '-NoProfile', '-File', str(SCRIPTS / 'windows.ps1'), '-Jobs', str(options.jobs)]
    subprocess.run(command, cwd=source, env=environment, check=True)


if __name__ == '__main__':
    main()
