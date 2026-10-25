#!/usr/bin/env python3
"""Run a repository workflow with its separately released bootstrap CLI."""
import argparse
import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('workflow', type=Path)
    parser.add_argument('arguments', nargs=argparse.REMAINDER)
    options = parser.parse_args()
    workflow = (ROOT / options.workflow).resolve(strict=True)
    if not workflow.is_relative_to(ROOT / 'workflows') or not workflow.is_file():
        parser.error('Choose an authored repository workflow')
    specification = importlib.util.spec_from_file_location('workflow_bootstrap', Path(__file__).with_name('bootstrap.py'))
    bootstrap = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(bootstrap)
    archive = os.environ.get('DOCKPIPE_BOOTSTRAP_ARCHIVE')
    arguments = options.arguments
    if arguments[:1] == ['--']:
        arguments = arguments[1:]
    with tempfile.TemporaryDirectory(prefix='dockpipe-workflow-bootstrap-') as temporary:
        binary = bootstrap.install(ROOT, Path(temporary), Path(archive) if archive else None, report=False)
        return subprocess.run([str(binary), '--workflow-file', str(workflow), '--workdir', str(ROOT),
                               '--', *arguments], check=False).returncode


if __name__ == '__main__':
    raise SystemExit(main())
