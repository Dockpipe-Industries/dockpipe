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
    parser.add_argument(
        '--configure-argument',
        action='append',
        default=[],
        help='One explicit CMake configure argument; repeat for SDK and test options',
    )
    arguments = parser.parse_args()
    if arguments.jobs < 1:
        parser.error('--jobs must be positive')

    source = Path(os.environ['DOCKPIPE_SOURCE_ROOT']).resolve(strict=True)
    output = arguments.output.resolve()
    configure = [
        arguments.cmake,
        '-S', str(source / 'src/app/tooling/dockpipe-launcher'),
        '-B', str(output),
        *arguments.configure_argument,
    ]
    subprocess.run(configure, check=True)
    subprocess.run(
        [arguments.cmake, '--build', str(output), '--parallel', str(arguments.jobs)],
        check=True,
    )


if __name__ == '__main__':
    main()
