#!/usr/bin/env python3
"""Forward platform outputs without host-shell expansion or a non-Linux Bash probe."""
import argparse
import os
from pathlib import Path
import subprocess
import sys


def main(arguments=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('phase', choices=('cli', 'linux-packages'))
    options = parser.parse_args(arguments)
    environment = dict(os.environ)
    system = environment.get('PLATFORM_SYSTEM')
    if system not in ('linux', 'darwin', 'windows'):
        parser.error('The admitted platform system is required')
    if options.phase == 'linux-packages' and system != 'linux':
        return 0
    source = Path(environment['DOCKPIPE_SOURCE_ROOT']).resolve(strict=True)
    if options.phase == 'cli':
        command = [sys.executable, str(source / 'workflows/build/cli/assets/scripts/build.py'),
                   '--output', environment['PLATFORM_BINARY'], '--version', environment['PLATFORM_VERSION']]
    else:
        command = ['bash', str(source / 'workflows/build/platform/assets/scripts/package-linux.sh')]
    return subprocess.run(command, cwd=source, env=environment, check=False).returncode


if __name__ == '__main__':
    raise SystemExit(main())
