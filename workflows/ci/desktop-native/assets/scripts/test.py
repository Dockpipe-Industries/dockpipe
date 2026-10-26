#!/usr/bin/env python3
"""Run the selected tests from an explicitly configured native desktop build."""
import argparse
from pathlib import Path
import subprocess


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--build', type=Path, required=True)
    parser.add_argument('--regex', help='CTest name expression; omitted means the complete configured suite')
    parser.add_argument('--jobs', type=int, default=2)
    parser.add_argument('--timeout', type=int, default=1200, help='Maximum seconds per test')
    parser.add_argument('--ctest', default='ctest')
    arguments = parser.parse_args()
    if arguments.jobs < 1 or arguments.timeout < 1:
        parser.error('--jobs and --timeout must be positive')
    build = arguments.build.resolve(strict=True)
    if not (build / 'CMakeCache.txt').is_file() or not (build / 'CTestTestfile.cmake').is_file():
        parser.error('--build must be a configured CMake test directory')
    command = [arguments.ctest, '--test-dir', str(build), '--output-on-failure',
               '--stop-on-failure', '--no-tests=error', '--parallel', str(arguments.jobs), '--timeout', str(arguments.timeout)]
    if arguments.regex is not None:
        command.extend(['--tests-regex', arguments.regex])
    return subprocess.run(command, check=False).returncode


if __name__ == '__main__':
    raise SystemExit(main())
