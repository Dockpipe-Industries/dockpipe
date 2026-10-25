#!/usr/bin/env python3
"""Admit fresh platform output paths before compiling a release CLI."""
import argparse
import os
from pathlib import Path
import re
import subprocess


def prepare(arguments=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('version')
    parser.add_argument('--output', type=Path)
    arguments = parser.parse_args(arguments)
    if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', arguments.version):
        parser.error('Expected a numeric release version')
    source = Path(os.environ['DOCKPIPE_SOURCE_ROOT']).resolve(strict=True)
    system, architecture = subprocess.check_output(
        ['go', 'env', 'GOHOSTOS', 'GOHOSTARCH'], text=True).splitlines()
    if (system, architecture) not in {
        ('linux', 'amd64'), ('linux', 'arm64'), ('darwin', 'amd64'),
        ('darwin', 'arm64'), ('windows', 'amd64'),
    }:
        parser.error('Unsupported release platform')
    output = (arguments.output or source / 'release/artifacts').resolve()
    store = output / 'stores' / (system + '-' + architecture)
    if store.exists() or store.is_symlink():
        parser.error('Final store must be fresh')
    output.mkdir(parents=True, exist_ok=True)
    binary = source / 'src/bin' / ('dockpipe.exe' if system == 'windows' else 'dockpipe')
    values = {
        'PLATFORM_VERSION': arguments.version,
        'PLATFORM_SYSTEM': system,
        'PLATFORM_ARCH': architecture,
        'PLATFORM_BINARY': str(binary),
        'PLATFORM_OUTPUT': str(output),
    }
    if any('\n' in value or '\r' in value for value in values.values()):
        parser.error('Output paths must not contain line breaks')
    return values
