#!/usr/bin/env python3
"""Build the standalone developer CLI through an explicit Go toolchain."""
import argparse
import os
from pathlib import Path
import subprocess
import shutil


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--version', default='dev')
    parser.add_argument('--alias', type=Path, help='Optional second maintainer binary path')
    parser.add_argument('--target', choices=('native', 'linux/amd64', 'linux/arm64', 'darwin/amd64', 'darwin/arm64', 'windows/amd64'), default='native')
    parser.add_argument('--go', default='go')
    arguments = parser.parse_args()
    source = Path(os.environ['DOCKPIPE_SOURCE_ROOT']).resolve(strict=True)
    output = arguments.output.resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    environment = dict(os.environ)
    environment['CGO_ENABLED'] = '0'
    environment.pop('GOOS', None)
    environment.pop('GOARCH', None)
    if arguments.target != 'native':
        environment['GOOS'], environment['GOARCH'] = arguments.target.split('/')
    subprocess.run([arguments.go, 'build', '-trimpath', '-ldflags',
                    '-s -w -X main.Version=' + arguments.version, '-o', str(output), './src/cmd'],
                   cwd=source, env=environment, check=True)
    if arguments.alias:
        alias = arguments.alias.resolve()
        alias.parent.mkdir(parents=True, exist_ok=True)
        if alias != output:
            shutil.copy2(output, alias)
    print('Built ' + str(output))


if __name__ == '__main__':
    main()
