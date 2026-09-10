#!/usr/bin/env python3
"""Supported nine-check PipeLang integration campaign plus editor validation."""
import argparse
import json
from pathlib import Path
import sys
from campaign import Campaign, InputGuard, atomic_json, fingerprint, host_identity
from verification import StageRunner, POLICY, source_paths, toolchain_identity, dependency_guard, ROOT
from job import verify_job

CHECKS = [('core', './src/lib/pipelang/coreir'), ('hir', './src/lib/pipelang/hir'),
          ('evaluator', './src/lib/pipelang/coreeval'), ('backend', './src/lib/pipelang/gobackend'),
          ('applicationir', './src/lib/applicationir'), ('compatibility', './tests/pipelangcompat'),
          ('application', './src/lib/application'), ('cli', './src/cmd'), ('vet', None)]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--node', type=Path, required=True, help='Absolute editor-test Node executable')
    parser.add_argument('--go', type=Path, required=True)
    parser.add_argument('--cache', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--mode', choices=['fresh', 'resume'], default='fresh')
    args = parser.parse_args()
    verify_job()
    if not args.node.is_absolute() or not args.node.is_file():
        parser.error('absolute installed Node executable required')
    args.output.mkdir(parents=True, exist_ok=True, mode=0o700)
    guard = dependency_guard(args.go, args.cache, args.output, 'integration', [args.node])
    inputs = guard.check
    toolchain = toolchain_identity(args.go)
    identity = dict(source=inputs(), toolchain=toolchain['digest'], policy=POLICY, host=host_identity())
    with Campaign(args.output / 'campaign', args.mode) as campaign:
        preparation = StageRunner(campaign, 'preparation', identity, ['prepare-application', 'prepare-cli'], args.cache, inputs)
        stage = StageRunner(campaign, 'integration', identity, [name for name, _ in CHECKS], args.cache, inputs)
        rows = []
        for name, package in CHECKS:
            if name in ('application', 'cli'):
                build = preparation.run(['prepare-' + name], lambda d: ['env', 'GOENV=off', 'GOMEMLIMIT=600MiB', str(args.go),
                                        'test', '-p', '1', '-c', '-o', str(d / 'program.test'), package],
                                        artifacts=lambda d: [d / 'program.test'])
                if build['exit']:
                    rows.append(build)
                    break
            command = ['env', 'GOENV=off', str(args.go)]
            if name == 'vet':
                command += ['vet', '-p', '1', './src/lib/pipelang/...', './src/lib/applicationir', './src/lib/application', './src/cmd']
            else:
                command += ['test', '-p', '1', '-count=1', '-timeout=25s', package]
                if name == 'application':
                    command += ['-run', 'PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe']
            row = stage.run([name], command)
            rows.append(row)
        editor = StageRunner(campaign, 'editor', identity, ['editor'], args.cache, inputs)
        editor_row = editor.run(['editor'], [str(args.node), '--test', 'extension.test.js'], cwd=ROOT / 'src/app/tooling/vscode-extensions/dockpipe-language-support')
        editor_result = editor.finish()
        result = stage.finish()
        result['editor'] = editor_result
        result['toolchain_unchanged'] = toolchain_identity(args.go) == toolchain
        atomic_json(args.output / 'checks.json', rows)
        atomic_json(args.output / 'summary.json', result)
        print(json.dumps(result))
        return int(bool(result['missing']) or bool(editor_result['missing']) or not result['toolchain_unchanged'])


if __name__ == '__main__':
    sys.exit(main())
