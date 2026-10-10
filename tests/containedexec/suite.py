#!/usr/bin/env python3
"""Run a compiled Go test suite in fresh bounded units without cumulative cache charges.

Build the test binary using run.py first. Every discovered top-level test is
included exactly once. Each batch retains the same hard cap and case guards.
"""
import argparse
import json
from pathlib import Path
import re
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--cache', required=True, type=Path)
    parser.add_argument('--batch-size', type=int, default=25)
    parser.add_argument('--split-test', action='append', default=[], help='Run NAME=COUNT numbered subtests in separate units')
    parser.add_argument('--only', help='Optional exact discovered top-level test name')
    args = parser.parse_args()
    if args.batch_size < 1:
        parser.error('positive batch size required')
    args.output.mkdir(parents=True, exist_ok=True)
    runner = Path(__file__).with_name('run.py')

    def run(label, command):
        prefix = args.output/label
        with Path(str(prefix)+'.runner.log').open('w') as log:
            result = subprocess.run([sys.executable,str(runner),'--output',str(prefix),'--cache',str(args.cache),
                                     '--timeout','180','--']+command, stdout=log, stderr=subprocess.STDOUT)
        return result.returncode, json.loads(Path(str(prefix)+'.json').read_text())

    rc, _ = run('list', [str(args.binary),'-test.list','^(Test|Fuzz|Example)'])
    if rc:
        return rc
    tests = [s for s in (args.output/'list.output').read_text().splitlines() if re.fullmatch(r'(?:Test|Fuzz|Example)\w*',s)]
    if not tests or len(tests) != len(set(tests)):
        raise RuntimeError('invalid test inventory')
    splits = {}
    for entry in args.split_test:
        name, count = entry.rsplit('=', 1)
        if name not in tests or int(count) < 1:
            raise RuntimeError('invalid split-test inventory: '+entry)
        splits[name] = int(count)
    if args.only:
        if args.only not in tests:
            raise RuntimeError('test not discovered: '+args.only)
        tests = [args.only]
    jobs = []
    pending = []
    def flush():
        if pending:
            jobs.append((list(pending), '^('+'|'.join(pending)+')$'))
            pending.clear()
    for name in tests:
        if name in splits:
            flush()
            for index in range(splits[name]):
                jobs.append(([name+'/'+str(index)], '^'+name+'$/^'+str(index)+'$'))
        else:
            pending.append(name)
            if len(pending) == args.batch_size:
                flush()
    flush()
    rows = []
    for i, (names, pattern) in enumerate(jobs):
        rc, report = run('batch-%03d'%i,[str(args.binary),'-test.run',pattern,
                         '-test.v','-test.count=1','-test.timeout=170s'])
        rows.append(dict(tests=names,exit=rc,report=report))
        (args.output/'suite.json').write_text(json.dumps(rows,indent=2)+'\n')
    summary = dict(tests=len(tests),batches=len(rows),failed_batches=[i for i,row in enumerate(rows) if row['exit']],
                   max_aggregate_bytes=max(row['report'].get('aggregate_peak_bytes',0) for row in rows))
    print(json.dumps(summary))
    return int(bool(summary['failed_batches']))


if __name__ == '__main__':
    sys.exit(main())
