#!/usr/bin/env python3
"""Select a release identity and explicit producer paths for Linux packaging."""
import argparse
import os
from pathlib import Path
import re


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('version')
    parser.add_argument('architecture', choices=('amd64', 'arm64'), nargs='?', default='amd64')
    parser.add_argument('--output', type=Path)
    arguments = parser.parse_args()
    if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', arguments.version):
        parser.error('Expected a numeric release version')
    artifacts = Path(os.environ['DOCKPIPE_ARTIFACT_ROOT']).resolve()
    output = (arguments.output or artifacts / 'packages').resolve()
    output.mkdir(parents=True, exist_ok=True)
    values = {
        'LINUX_VERSION': arguments.version,
        'LINUX_ARCH': arguments.architecture,
        'LINUX_BINARY': str(artifacts / ('dockpipe-linux-' + arguments.architecture)),
        'LINUX_OUTPUT': str(output),
    }
    if any('\n' in value or '\r' in value for value in values.values()):
        parser.error('Paths must not contain line breaks')
    Path(os.environ['DOCKPIPE_STEP_OUTPUTS_FILE']).write_text(
        ''.join(key + '=' + value + '\n' for key, value in values.items()))


if __name__ == '__main__':
    main()
