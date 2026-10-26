#!/usr/bin/env python3
"""Verify the literal source checkout and expose the committed SDK release identity."""
import argparse
import importlib.util
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[5]
SPECIFICATION = importlib.util.spec_from_file_location('locked_go_sdk', Path(__file__).with_name('build-locked.py'))
LOCKED = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(LOCKED)


def inspect(lock_path, workflow_path):
    lock = LOCKED.read_lock(lock_path)
    checkouts = re.findall(r'^          ref: ([0-9a-f]{40})$', workflow_path.read_text(), re.MULTILINE)
    if checkouts != [lock['source_commit']]:
        raise ValueError('The literal source checkout must match the committed SDK lock exactly')
    return {'source_commit': lock['source_commit'], 'tag': 'dockpipe-go-sdk-' + lock['version']}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    arguments = parser.parse_args()
    values = inspect(ROOT / 'workflows/build/go-sdk/assets/source.lock.json', ROOT / '.github/workflows/go-sdk.yml')
    with arguments.output.open('a', encoding='utf-8') as output:
        output.write(''.join(name + '=' + value + '\n' for name, value in values.items()))


if __name__ == '__main__':
    main()
