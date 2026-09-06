#!/usr/bin/env python3
"""Fresh, isolated warm compiler-only measurements of exported PipeLang fixtures.

First run TestCompilerMemoryLocalSequences with PIPELANG_MEMORY_FIXTURES set to
an absolute private directory. This command measures each fixture in its own unit;
it stops increasing a family after its first failure or performance limit crossing.
"""
import argparse
import json
from pathlib import Path
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--fixtures', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--cache', required=True, type=Path)
    parser.add_argument('--compiler', required=True, type=Path)
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    fixtures = []
    for path in args.fixtures.glob('*/measurement.json'):
        meta = json.loads(path.read_text())
        family = (meta['version'], meta['choices'], meta['branch'])
        fixtures.append((family, meta['locals'], path.parent, meta))
    failed = set()
    rows = []
    for family, count, directory, meta in sorted(fixtures):
        if family in failed:
            continue
        prefix = args.output/directory.name
        compiler_args = [str(args.compiler), '-c=4', '-p', 'pipelanggenerated', '-importcfg', str(directory/'importcfg'),
                        '-o', str(prefix)+'.a', str(directory/'generated.go')]
        command = [sys.executable, str(Path(__file__).with_name('run.py')), '--output', str(prefix),
                   '--cache', str(args.cache), '--timeout', '30', '--']+compiler_args
        with Path(str(prefix)+'.runner.log').open('w') as log:
            result = subprocess.run(command, stdout=log, stderr=subprocess.STDOUT)
        report = json.loads(Path(str(prefix)+'.json').read_text())
        row = dict(meta, isolated=report, cache_state='warm standard-library exports; fresh direct target compile',
                   compiler_flags=['-c=4', 'normal inlining'])
        row['accepted'] = result.returncode == 0 and report['child_maxrss_kib'] <= 128*1024 and report['elapsed_s'] <= 5
        if not row['accepted']:
            failed.add(family)
        rows.append(row)
        (args.output/'matrix.json').write_text(json.dumps(rows, indent=2)+'\n')
    print(json.dumps({'fixtures':len(rows), 'failed_families':[list(f) for f in sorted(failed)],
                      'max_rss_mib':max(r['isolated']['child_maxrss_kib'] for r in rows)/1024,
                      'max_elapsed_s':max(r['isolated']['elapsed_s'] for r in rows)}))
    return int(bool(failed))


if __name__ == '__main__':
    sys.exit(main())
