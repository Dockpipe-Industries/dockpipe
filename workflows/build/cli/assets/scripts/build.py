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
    parser.add_argument('--worker-host', action='store_true',
                        help='Build the Windows background worker host beside the CLI')
    parser.add_argument(
        '--target',
        choices=('native', 'linux/amd64', 'linux/arm64', 'darwin/amd64', 'darwin/arm64',
                 'windows/amd64', 'windows/arm64'),
        default='native',
    )
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
    if arguments.worker_host:
        target_system = environment.get('GOOS')
        if target_system is None:
            target_system = subprocess.check_output([arguments.go, 'env', 'GOHOSTOS'],
                                                    cwd=source, env=environment, text=True).strip()
        if target_system != 'windows' or output.name.casefold() == 'dockpipe-worker.exe':
            parser.error('--worker-host requires a Windows CLI output separate from dockpipe-worker.exe')

    subprocess.run([arguments.go, 'build', '-trimpath', '-ldflags',
                    '-s -w -X main.Version=' + arguments.version, '-o', str(output), './src/cmd'],
                   cwd=source, env=environment, check=True)
    if arguments.worker_host:
        worker = output.with_name('dockpipe-worker.exe')
        subprocess.run([arguments.go, 'build', '-trimpath', '-ldflags', '-s -w -H=windowsgui',
                        '-o', str(worker), './src/cmd/workerhost'], cwd=source, env=environment, check=True)
        print('Built ' + str(worker))

    if arguments.alias:
        alias = arguments.alias.resolve()
        alias.parent.mkdir(parents=True, exist_ok=True)
        if alias != output:
            shutil.copy2(output, alias)
    print('Built ' + str(output))


if __name__ == '__main__':
    main()
