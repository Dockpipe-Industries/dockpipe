#!/usr/bin/env python3
"""Run architecture-specific Linux package workflows with the same release identity."""
import argparse
from pathlib import Path
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('version')
    parser.add_argument('--output', type=Path)
    arguments = parser.parse_args()
    root = Path(__file__).resolve().parents[5]
    for architecture in ('amd64', 'arm64'):
        command = [sys.executable, str(root / 'workflows/shared/launch.py'),
                   'workflows/package/linux-cli/config.yml', '--', arguments.version, architecture]
        if arguments.output:
            command.extend(['--output', str(arguments.output.resolve())])
        subprocess.run(command, cwd=root, check=True)


if __name__ == '__main__':
    main()
