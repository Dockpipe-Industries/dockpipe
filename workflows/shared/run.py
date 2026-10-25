#!/usr/bin/env python3
"""Forward a workflow's JSON arguments to its explicit host command."""
import json
import os
import subprocess
import sys


def main():
    arguments = json.loads(os.environ.get('DOCKPIPE_ARGS_JSON', '[]'))
    if not isinstance(arguments, list) or any(not isinstance(value, str) for value in arguments):
        raise ValueError('Workflow arguments must be a JSON array of strings')
    command = sys.argv[1:]
    if not command:
        raise ValueError('An explicit host command is required')
    return subprocess.run([*command, *arguments], check=False).returncode


if __name__ == '__main__':
    raise SystemExit(main())
