#!/usr/bin/env python3
"""Run or resume all PipeLang verification stages inside one bounded job.

Default proof/cache storage is durable user-cache data. Final adoption is a
separate read-only reconciliation after job.py proves aggregate tree removal.
"""
import argparse
import json
from pathlib import Path
import subprocess
import sys
import time
from campaign import Campaign, atomic_json, fingerprint, resource_accepted
from verification import ROOT, dependency_guard
from job import verify_job

HERE = Path(__file__).resolve().parent
MEMORY_FAMILIES = {'TestV1090TerminalCombinedSelectorArmsMemory': 1944,
                   'TestV1100StraightLineSelectorValueArmsMemory': 192}
MEMORY_COUNT = sum(MEMORY_FAMILIES.values())


def validated_stage(output, stage, selected=None):
    dependencies = json.loads((output / 'dependency-inputs.json').read_text())
    current = fingerprint(dependencies['paths'])['digest']
    if current != dependencies['identity']['digest']:
        raise RuntimeError('changed stage dependencies: ' + stage)
    with Campaign(output / 'campaign', 'resume') as campaign:
        campaign.current = {name: value['key'] for name, value in campaign.manifest['stages'].items()}
        accepted = campaign.accepted(stage, current, selected)
        expected = selected if selected is not None else campaign.manifest['stages'][stage]['inventory']
        if set(accepted) != set(expected):
            raise RuntimeError('incomplete or invalid stage: ' + stage)
        return accepted


def reconcile(output, baseline, job_report):
    job = json.loads(job_report.read_text())
    if job.get('outcome') != 'completed' or job.get('exit') != 0 or not job.get('tree_removed') or Path(job['cgroup']).exists() or job.get('swap_current') != 0:
        raise RuntimeError('aggregate job completion/cleanup unavailable')
    # memory.max events can be successful coordinator file-cache reclaim at
    # its unchanged hard cap. OOM events and actual aggregate excess fail.
    if job.get('aggregate_peak_bytes', 2 << 30) > 2 << 30:
        raise RuntimeError('aggregate hard ceiling exceeded')
    for name in ('oom', 'oom_kill'):
        events = lambda text: dict(line.split() for line in text.splitlines())
        if events(job['memory_events_before'])[name] != events(job['memory_events_after'])[name]:
            raise RuntimeError('aggregate or descendant resource crossing: ' + name)
    suite = validated_stage(output / 'suite', 'suite')
    matrix = validated_stage(output / 'matrix', 'matrix')
    integration = validated_stage(output / 'integration', 'integration')
    editor = validated_stage(output / 'integration', 'editor')
    prior = json.loads((baseline / 'suite/inventory.json').read_text())
    current = json.loads((output / 'suite/inventory.json').read_text())
    baseline_cases = {case for names, _ in prior['jobs'] for case in names}
    if not baseline_cases <= set(suite) or not set(prior['tests']) <= set(current['tests']):
        raise RuntimeError('baseline semantic inventory shrank')
    if len(matrix) != MEMORY_COUNT or len(integration) != 9 or len(editor) != 1:
        raise RuntimeError('required matrix/integration/editor inventory mismatch')
    measured = [r['report'] for r in matrix.values()]
    if any(r['child_maxrss_kib'] > 128 * 1024 or r['elapsed_s'] > 5 or 'memory.high' in r['limits'] for r in measured):
        raise RuntimeError('isolated compiler acceptance changed')
    result = dict(status='accepted', language_contract='v0.110.0', functions=len(current['tests']),
                  logical_cases=len(suite), baseline_functions=len(prior['tests']), baseline_logical_cases=len(baseline_cases),
                  isolated_cases=len(matrix), integration_checks=len(integration), editor_checks=len(editor),
                  isolated_max_rss_mib=max(r['child_maxrss_kib'] for r in measured) / 1024,
                  isolated_max_elapsed_s=max(r['elapsed_s'] for r in measured),
                  job_receipt=str(job_report), aggregate_peak_bytes=job['aggregate_peak_bytes'],
                  aggregate_elapsed_s=job['elapsed_s'], all_cgroups_removed=True,
                  suite_summary=json.loads((output / 'suite/summary.json').read_text()))
    atomic_json(output / 'accepted-verification.json', result)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=Path.home() / '.cache/pipelang-verification')
    parser.add_argument('--campaign', required=True, help='Private campaign directory name')
    parser.add_argument('--node', type=Path, required=True)
    parser.add_argument('--go', type=Path, required=True)
    parser.add_argument('--cache', type=Path)
    parser.add_argument('--compiled-cache', type=Path)
    parser.add_argument('--native-build-cache', type=Path)
    parser.add_argument('--baseline', type=Path, required=True)
    parser.add_argument('--mode', choices=['fresh', 'resume'], default='fresh')
    parser.add_argument('--schedule-profile', type=Path)
    parser.add_argument('--disk-budget-gib', type=int, default=96)
    parser.add_argument('--accept-job', type=Path, help='Reconcile completed job without executing stages')
    args = parser.parse_args()
    verify_job()
    if not args.root.is_absolute() or args.root.is_relative_to('/tmp') or Path(args.campaign).name != args.campaign or args.campaign in ('.', '..'):
        parser.error('durable absolute root outside /tmp and a single campaign name required')
    output = args.root / 'campaigns' / args.campaign
    if args.accept_job:
        print(json.dumps(reconcile(output, args.baseline, args.accept_job)))
        return 0
    output.mkdir(parents=True, exist_ok=True, mode=0o700)
    cache = args.cache or args.root / 'go-build-cache'
    compiled = args.compiled_cache or args.root / 'executables'
    native = args.native_build_cache or args.root / 'native-build-cache'
    with Campaign(output / 'controller', args.mode) as controller:
        guard = dependency_guard(args.go, cache, output, 'integration', [args.node])
        controller.register('workflow', guard.check(), ['suite', 'matrix', 'integration'])
        for name in ('suite', 'matrix', 'integration'):
            guard.check()
            stage_output = output / name
            mode = 'resume' if args.mode == 'resume' and (stage_output / 'campaign/manifest.json').exists() else 'fresh'
            driver = 'pipelang_suite.py' if name == 'suite' else name + '.py'
            command = [sys.executable, '-B', str(HERE / driver), '--output', str(stage_output), '--cache', str(cache), '--mode', mode]
            if name == 'suite':
                command += ['--go', str(args.go), '--compiled-cache', str(compiled), '--native-build-cache', str(native),
                            '--build-store', str(args.root / 'builds'), '--workers', '2', '--audit-generated',
                            '--disk-budget-gib', str(args.disk_budget_gib)]
                if args.schedule_profile:
                    command += ['--schedule-profile', str(args.schedule_profile)]
            elif name == 'matrix':
                inventory = json.loads((output / 'suite/inventory.json').read_text())
                paths=[]
                for family,expected in MEMORY_FAMILIES.items():
                    memory_cases=[case for names,_ in inventory['jobs'] for case in names if case.startswith(family+'/')]
                    receipts=validated_stage(output/'suite','suite',memory_cases)
                    family_paths=sorted({p for r in receipts.values() for p in r['artifacts'] if p.endswith('/measurement.json')})
                    if len(family_paths)!=expected:
                        raise RuntimeError('memory fixture inventory mismatch: '+family)
                    paths.extend(family_paths)
                if len(set(paths))!=MEMORY_COUNT:
                    raise RuntimeError('memory fixture identities overlap')
                manifest = output / 'compiler-fixtures.json'
                atomic_json(manifest, paths)
                command += ['--fixture-manifest', str(manifest), '--compiler', str(args.go.parent.parent / 'pkg/tool/linux_amd64/compile')]
            else:
                command += ['--go', str(args.go), '--node', str(args.node)]
            with (output / (name + '-coordinator.log')).open('a') as log:
                rc = subprocess.run(command, cwd=ROOT, stdout=log, stderr=subprocess.STDOUT).returncode
            controller.event('stage_exit', stage=name, exit=rc)
            if rc:
                print(json.dumps(dict(stage=name, status='failed', log=str(output / (name + '-coordinator.log')))))
                return rc
        guard.check()
        atomic_json(output / 'stages-complete.json', dict(status='stages_complete_pending_aggregate_cleanup', inputs=guard.check()))
        print(json.dumps(dict(status='stages_complete_pending_aggregate_cleanup', output=str(output))))
    return 0


if __name__ == '__main__':
    sys.exit(main())
