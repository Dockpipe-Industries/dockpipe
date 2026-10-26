#!/usr/bin/env python3
"""Reproduce the committed Go SDK selected by the workflow-owned content lock."""
import argparse
import hashlib
import importlib.util
import json
import os
import sys
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[5]
SPEC = importlib.util.spec_from_file_location('go_sdk_producer', Path(__file__).with_name('build.py'))
PRODUCER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PRODUCER)


def read_lock(path):
    if path.is_symlink() or not path.is_file() or path.stat().st_size > 64 << 10:
        raise ValueError('SDK lock must be a bounded regular file')
    lock = json.loads(path.read_text())
    fields = {'schema', 'source_commit', 'version', 'source_digest', 'manifest_asset',
              'manifest_sha256', 'archive_asset', 'archive_sha256', 'archive_bytes'}
    if (not isinstance(lock, dict) or set(lock) != fields
            or type(lock.get('schema')) is not int or lock['schema'] != 1
            or any(not isinstance(lock[field], str) for field in fields - {'schema', 'archive_bytes'})
            or not re.fullmatch(r'[0-9a-f]{40}', lock['source_commit'])):
        raise ValueError('SDK lock requires an exact committed source and complete content identity')
    for field in ('source_digest', 'manifest_sha256', 'archive_sha256'):
        if not re.fullmatch(r'[0-9a-f]{64}', lock[field]):
            raise ValueError('SDK lock requires an exact SHA-256 for ' + field)
    stem = 'dockpipe-go-sdk-' + lock['source_digest'][:16]
    if lock['version'] != '0.1.0-source.' + lock['source_digest'][:16]:
        raise ValueError('SDK version must identify its source inventory')
    if lock['manifest_asset'] != stem + '.json' or lock['archive_asset'] != stem + '.tar.gz':
        raise ValueError('SDK asset names must identify their source inventory')
    if type(lock['archive_bytes']) is not int or lock['archive_bytes'] <= 0:
        raise ValueError('SDK archive must have an exact positive byte count')
    return lock


def verify_output(directory, lock):
    manifest_path = directory / lock['manifest_asset']
    archive_path = directory / lock['archive_asset']
    for path, field in ((manifest_path, 'manifest_sha256'), (archive_path, 'archive_sha256')):
        if hashlib.sha256(path.read_bytes()).hexdigest() != lock[field]:
            raise ValueError('Reproduced SDK differs from the committed lock: ' + path.name)
    if archive_path.stat().st_size != lock['archive_bytes']:
        raise ValueError('Reproduced SDK archive size differs from the committed lock')
    manifest = json.loads(manifest_path.read_text())
    expected = {
        'sdk_version': lock['version'],
        'source_commit': lock['source_commit'],
        'source_digest': lock['source_digest'],
        'artifact': lock['archive_asset'],
        'sha256': lock['archive_sha256'],
        'bytes': lock['archive_bytes'],
    }
    if any(manifest.get(key) != value for key, value in expected.items()):
        raise ValueError('Reproduced SDK metadata differs from the committed lock')


def parse_arguments():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source-root', type=Path, required=True,
                        help='Checkout of the source commit in workflows/build/go-sdk/assets/source.lock.json')
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--go', default='go')
    parser.add_argument('--lock', type=Path, default=ROOT / 'workflows/build/go-sdk/assets/source.lock.json')
    arguments = sys.argv[1:] or json.loads(os.environ.get("DOCKPIPE_ARGS_JSON", "[]"))
    return parser.parse_args(arguments)


def main():
    args = parse_arguments()
    lock = read_lock(args.lock)
    PRODUCER.export(args.source_root.resolve(), args.output.resolve(), PRODUCER.TARGETS,
                    args.go, source_commit=lock['source_commit'])
    verify_output(args.output.resolve(), lock)


if __name__ == '__main__':
    main()
