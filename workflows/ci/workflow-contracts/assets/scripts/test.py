#!/usr/bin/env python3
"""Run tests beside their owning repository workflows."""
import argparse
from pathlib import Path
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--suite', type=Path, action='append')
    parser.add_argument('--pattern', default='test_*.py')
    arguments = parser.parse_args()
    root = Path(__file__).resolve().parents[5]
    workflows = root / 'workflows'
    suites = arguments.suite or sorted(path for path in workflows.rglob('tests')
                                      if path.is_dir() and any(path.glob(arguments.pattern)))
    if not suites:
        parser.error('No workflow test suites found')
    failures = []
    for selected in suites:
        suite = selected.resolve() if selected.is_absolute() else (root / selected).resolve()
        if not suite.is_relative_to(workflows) or suite.name != 'tests' or not suite.is_dir():
            parser.error('Test suites must be existing workflow-owned tests directories')
        if not any(suite.glob(arguments.pattern)):
            parser.error('The selected suite contains no matching test modules: ' + str(suite))
        result = subprocess.run([sys.executable, '-B', '-m', 'unittest', 'discover', '-s', str(suite),
                        '-p', arguments.pattern, '-v'], cwd=root, check=False)
        if result.returncode:
            failures.append(str(suite.relative_to(root)))
    if failures:
        raise SystemExit('Failed workflow suites: ' + ', '.join(failures))


if __name__ == '__main__':
    main()
