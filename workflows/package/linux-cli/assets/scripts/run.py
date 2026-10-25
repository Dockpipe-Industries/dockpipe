#!/usr/bin/env python3
"""Build and package a standalone Linux CLI with explicit phase arguments."""
import argparse
import os
from pathlib import Path
import re
import subprocess
import sys


def main(arguments=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('version')
    parser.add_argument('architecture', choices=('amd64', 'arm64'), nargs='?', default='amd64')
    parser.add_argument('--output', type=Path)
    options = parser.parse_args(arguments)
    if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', options.version):
        parser.error('Expected a numeric release version')
    source = Path(os.environ['DOCKPIPE_SOURCE_ROOT']).resolve(strict=True)
    artifacts = Path(os.environ['DOCKPIPE_ARTIFACT_ROOT']).resolve()
    output = (options.output or artifacts / 'packages').resolve()
    binary = artifacts / ('dockpipe-linux-' + options.architecture)
    if any('\n' in str(path) or '\r' in str(path) for path in (output, binary)):
        parser.error('Paths must not contain line breaks')
    output.mkdir(parents=True, exist_ok=True)
    environment = dict(os.environ)
    environment['DOCKPIPE_RELEASE_BINARY'] = str(binary)
    subprocess.run([sys.executable, str(source / 'workflows/build/cli/assets/scripts/build.py'),
                    '--target', 'linux/' + options.architecture, '--output', str(binary),
                    '--version', options.version], cwd=source, env=environment, check=True)
    subprocess.run(['bash', str(source / 'workflows/package/linux-cli/assets/scripts/build-deb.sh'),
                    options.version, options.architecture, str(output)],
                   cwd=source, env=environment, check=True)
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
