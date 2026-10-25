#!/usr/bin/env python3
"""Prepare pinned module dependencies without changing the committed SDK source."""
import hashlib
import importlib.util
import os
from pathlib import Path
import re
import subprocess

SPECIFICATION = importlib.util.spec_from_file_location('locked_go_sdk', Path(__file__).with_name('build-locked.py'))
LOCKED = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(LOCKED)


def prepare(source, go):
    inputs = [source / 'go.mod', source / 'go.sum']
    before = {path.name: hashlib.sha256(path.read_bytes()).hexdigest() for path in inputs}
    declared = re.search(r'^toolchain (go[0-9]+\.[0-9]+\.[0-9]+)$', inputs[0].read_text(), re.MULTILINE)
    if declared is None:
        raise ValueError('SDK source must declare its exact Go toolchain')
    environment = dict(os.environ, GOTOOLCHAIN='local', GOWORK='off')
    actual = subprocess.check_output([go, 'env', 'GOVERSION'], cwd=source, env=environment, text=True).strip()
    if actual != declared[1]:
        raise ValueError('Selected Go toolchain differs from the committed SDK source')
    subprocess.run([go, 'mod', 'download'], cwd=source, env=environment, check=True, timeout=300)
    if before != {path.name: hashlib.sha256(path.read_bytes()).hexdigest() for path in inputs}:
        raise ValueError('Dependency preparation changed committed Go module inputs')


def main():
    arguments = LOCKED.parse_arguments()
    LOCKED.read_lock(arguments.lock)
    prepare(arguments.source_root.resolve(strict=True), arguments.go)


if __name__ == '__main__':
    main()
