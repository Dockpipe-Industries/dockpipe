#!/usr/bin/env python3
"""Build the developer desktop with explicitly configured SDK inputs."""
import argparse
import os
from pathlib import Path
import subprocess


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--cmake', default='cmake')
    parser.add_argument('--jobs', type=int, default=2)
    parser.add_argument('--configure-only', action='store_true', help='Validate configuration without compiling targets')
    parser.add_argument('--target', action='append', default=[], help='Build only this target; repeat for focused verification')
    parser.add_argument(
        '--configure-argument',
        action='append',
        default=[],
        help='One explicit CMake configure argument; repeat for SDK and test options',
    )
    arguments = parser.parse_args()
    if arguments.jobs < 1:
        parser.error('--jobs must be positive')
    if arguments.configure_only and arguments.target:
        parser.error('--configure-only cannot be combined with --target')

    source = Path(os.environ['DOCKPIPE_SOURCE_ROOT']).resolve(strict=True)
    output = arguments.output.resolve()
    configure = [
        arguments.cmake,
        '-S', str(source / 'src/dockpipe-app'),
        '-B', str(output),
        *arguments.configure_argument,
    ]
    subprocess.run(configure, check=True)
    if arguments.configure_only:
        return
    build = [arguments.cmake, '--build', str(output), '--parallel', str(arguments.jobs)]
    if arguments.target:
        build.extend(['--target', *arguments.target])
    subprocess.run(build, check=True)


if __name__ == '__main__':
    main()
