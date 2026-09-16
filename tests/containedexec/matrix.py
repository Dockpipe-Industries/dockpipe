#!/usr/bin/env python3
"""Fresh isolated compiler measurements, with explicit same-campaign resumption."""
import argparse
import json
from pathlib import Path
import sys
from campaign import Campaign, InputGuard, atomic_json, fingerprint, host_identity, file_identity
from verification import StageRunner, POLICY, source_paths
from job import verify_job
from reporting import CompactRows


def fixture_inputs(fixtures, compiler):
    paths = [compiler]
    for _, _, directory, _ in fixtures:
        paths.extend(directory / name for name in ('measurement.json', 'generated.go', 'importcfg'))
        for line in (directory / 'importcfg').read_text().splitlines():
            if line.startswith('packagefile '):
                paths.append(Path(line.split('=', 1)[1]))
            elif line and not line.startswith('importmap '):
                raise ValueError('unknown importcfg input: ' + line)
    return sorted(set(paths))


def discover(root=None, manifest=None):
    fixtures = []
    paths = [Path(p) for p in json.loads(manifest.read_text())] if manifest else root.glob('*/measurement.json')
    for path in paths:
        meta = json.loads(path.read_text())
        family = (meta['version'], meta['choices'], meta['branch'], meta.get('straight', False), meta.get('unused', False))
        fixtures.append((family, meta['locals'], path.parent, meta))
    if not fixtures:
        raise ValueError('no compiler fixtures discovered')
    return sorted(fixtures)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--fixtures', type=Path)
    parser.add_argument('--fixture-manifest', type=Path)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--cache', required=True, type=Path)
    parser.add_argument('--compiler', required=True, type=Path)
    parser.add_argument('--mode', choices=['fresh', 'resume'], default='fresh')
    args = parser.parse_args()
    verify_job()
    args.output.mkdir(parents=True, exist_ok=True, mode=0o700)
    if bool(args.fixtures) == bool(args.fixture_manifest):
        parser.error('provide exactly one fixture directory or manifest')
    fixtures = discover(args.fixtures, args.fixture_manifest)
    paths = fixture_inputs(fixtures, args.compiler) + source_paths('matrix')
    guard = InputGuard(paths)
    inputs = guard.check
    atomic_json(args.output / 'dependency-inputs.json', dict(paths=list(map(str, paths)), identity=guard.identity))
    identity = dict(inputs=inputs(), host=host_identity(), policy=POLICY, high=None)
    cases = [directory.name for _, _, directory, _ in fixtures]
    failed, rows = set(), CompactRows()
    with Campaign(args.output / 'campaign', args.mode) as campaign:
        stage = StageRunner(campaign, 'matrix', identity, cases, args.cache, inputs)
        for family, count, directory, meta in fixtures:
            if family in failed:
                continue
            command = lambda d: [str(args.compiler), '-c=4', '-p', 'pipelanggenerated',
                                 '-importcfg', str(directory / 'importcfg'), '-o', str(d / 'target.a'), str(directory / 'generated.go')]
            result = stage.run([directory.name], command, high=None,
                               accept=lambda r: r['child_maxrss_kib'] <= 128 * 1024 and r['elapsed_s'] <= 5,
                               artifacts=lambda d: [d / 'target.a'])
            row = dict(meta, isolated=result['report'], accepted=result['exit'] == 0, receipt=result['receipt'],
                       cache_state='warm standard-library exports; fresh direct target compile',
                       compiler_flags=['-c=4', 'normal inlining'])
            if not row['accepted']:
                failed.add(family)
            rows.append(row, index=len(rows))
        rows.write(args.output / 'matrix.json')
        reconciliation = stage.finish()
        summary = dict(fixtures=len(rows), failed_families=[list(f) for f in sorted(failed)],
                       max_rss_mib=max(r['isolated'].get('child_maxrss_kib', 0) for r in rows) / 1024,
                       max_elapsed_s=max(r['isolated'].get('elapsed_s', 0) for r in rows), reconciliation=reconciliation)
        atomic_json(args.output / 'summary.json', summary)
        print(json.dumps(summary))
        return int(bool(failed) or bool(reconciliation['missing']))


if __name__ == '__main__':
    sys.exit(main())
