#!/usr/bin/env python3
"""Require genuine, non-skipped passes from a fresh CTest JUnit report."""
import argparse
from pathlib import Path
import xml.etree.ElementTree as ET

# Native qualification reports are metadata, not arbitrary test artifacts. Keep
# the supported report capacity explicit even when a fixture prints extra logs.
MAX_REPORT_BYTES = 16 * 1024 * 1024


def require_passes(report, required):
    if not required or len(set(required)) != len(required):
        raise ValueError('Select each required CTest name exactly once')
    if report.is_symlink() or not report.is_file():
        raise ValueError('CTest must produce a fresh regular JUnit report')
    with report.open('rb') as stream:
        payload = stream.read(MAX_REPORT_BYTES + 1)
    if not payload or len(payload) > MAX_REPORT_BYTES:
        raise ValueError('CTest JUnit report exceeds its supported metadata capacity')
    declarations = payload.upper()
    if b'<!DOCTYPE' in declarations or b'<!ENTITY' in declarations:
        raise ValueError('CTest JUnit report must not declare entities')
    root = ET.fromstring(payload)
    cases = list(root.iter('testcase'))
    for name in required:
        matches = [case for case in cases if case.get('name') == name]
        if len(matches) != 1:
            raise ValueError('Required CTest must appear exactly once: ' + name)
        case = matches[0]
        if case.get('status') != 'run' or any(case.find(tag) is not None for tag in ('skipped', 'failure', 'error')):
            raise ValueError('Required CTest did not genuinely pass: ' + name)
    return tuple(required)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--report', type=Path, required=True)
    parser.add_argument('--require', action='append', required=True, dest='required')
    options = parser.parse_args()
    try:
        passed = require_passes(options.report, options.required)
    except (OSError, ValueError, ET.ParseError) as error:
        parser.exit(1, 'Native qualification rejected: ' + str(error) + '\n')
    print('PASS: required native CTest results: ' + ', '.join(passed))


if __name__ == '__main__':
    main()
