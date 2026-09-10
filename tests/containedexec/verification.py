"""Shared contained campaign execution and conservative input discovery."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time
from campaign import Campaign, atomic_json, digest, file_identity, fingerprint, host_identity, provenance
from job import verify_job

ROOT = Path(__file__).resolve().parents[2]
RUNNER = Path(__file__).with_name('run.py')
POLICY = dict(environment='offline-normal-build-settings-v1', unit_max=1 << 30, unit_tasks=128, unit_swap=0, stop=800 << 20,
              shared_max=2 << 30, shared_high=1536 << 20, shared_stop=1800 << 20,
              shared_tasks=384, coordinator_max=512 << 20, coordinator_tasks=64,
              timeout=30, test_timeout=25, compiler_rss=128 << 20, compiler_seconds=5,
              goenv='off', toolchain='local', proxy='off', sumdb='off', work='off', gomaxprocs=4)


def source_paths(stage='suite'):
    # All local Go packages/assets rather than only the compiler snapshot. Extra
    # invalidation is deliberate until narrower closure is independently proven.
    paths = [ROOT / p for p in ('src/lib', 'src/cmd', 'src/core', 'go.mod', 'go.sum', 'tests/pipelangcompat')]
    paths.append(Path(sys.executable).resolve())
    paths.append(RUNNER.parent)  # runner, split discovery and nested transport helpers
    if stage == 'integration':
        paths.append(ROOT / 'src/app/tooling/vscode-extensions/dockpipe-language-support')
    return paths


def toolchain_identity(go):
    go = Path(go)
    if not go.is_absolute() or go.is_symlink():
        raise ValueError('absolute Go toolchain executable required')
    root = go.parent.parent
    if not (root / 'VERSION').read_text().startswith('go1.25.13\n'):
        raise ValueError('offline cached Go 1.25.13 required')
    return fingerprint([root / p for p in ('bin', 'pkg/tool', 'src', 'lib', 'VERSION', 'go.env')])


def completed_go_cases(log, cases):
    passed = []
    with Path(log).open() as stream:
        for line in stream:
            match = re.match(r'\s*--- PASS: (\S+)', line)
            if match:
                passed.append(match.group(1))
    def present(case):
        bits = case.split('/')
        if bits[0] not in passed:
            return False
        if len(bits) == 1:
            return True
        # Memory selectors include an intervening version dimension. Every
        # requested logical suffix must occur in the passed subtest hierarchy.
        for name in passed:
            parts = name.split('/')
            if parts[0] != bits[0]:
                continue
            for start in range(1, len(parts)):
                if parts[start:start + len(bits) - 1] == bits[1:]:
                    return True
        return False
    return [case for case in cases if present(case)]


class StageRunner:
    def __init__(self, campaign, stage, identity, cases, cache, inputs, parents=()):
        self.campaign, self.stage, self.cache = campaign, stage, cache
        self.input_fn = inputs
        self.host = host_identity()
        self.before = inputs()
        self.key = campaign.register(stage, identity, cases, parents)
        self.prior = campaign.accepted(stage, self.before)
        self.reused = 0
        self.executed = 0

    def run(self, cases, command, cwd=ROOT, high=700, go_cases=False, accept=lambda report: True,
            artifacts=lambda directory: [], retry_of=None):
        if all(case in self.prior for case in cases):
            receipt = self.prior[cases[0]]
            if receipt['cases'] == cases:
                self.reused += 1
                return dict(receipt['result'], report=receipt['report'], resumed=True, receipt=receipt['id'])
        verify_job()
        before = self.input_fn()
        if host_identity() != self.host:
            raise RuntimeError('host/resource policy changed before unit')
        if before != self.before:
            raise RuntimeError('input drift before unit; restart to revise the manifest')
        attempt = self.campaign.begin(self.stage, cases, before, retry_of)
        directory = self.campaign.root / 'attempts' / attempt['id']
        prefix = directory / 'unit'
        actual = command(directory) if callable(command) else command
        invocation = [sys.executable, '-B', str(RUNNER), '--output', str(prefix),
                      '--cache', str(self.cache), '--timeout', '30']
        if high is not None:
            invocation += ['--memory-high-mib', str(high)]
        start = time.monotonic_ns()
        with (directory / 'runner.log').open('w') as log:
            rc = subprocess.run(invocation + ['--'] + actual, cwd=cwd, stdout=log, stderr=subprocess.STDOUT).returncode
        report_path = directory / 'unit.json'
        report = json.loads(report_path.read_text()) if report_path.exists() else {}
        complete = completed_go_cases(directory / 'unit.output', cases) if go_cases and (directory / 'unit.output').exists() else cases
        row = dict(label=str(prefix.relative_to(self.campaign.root)), exit=rc, report=report,
                   tests=cases, directory=str(directory), elapsed_wall_s=(time.monotonic_ns() - start) / 1e9)
        required = [directory / 'unit.output', report_path, *artifacts(directory)]
        receipt = self.campaign.finish(attempt, report, required, self.input_fn(), complete, row,
                                       accepted=rc == 0 and accept(report) and host_identity() == self.host)
        row['exit'] = 0 if receipt['state'] == 'passed' else 1
        row['receipt'] = receipt['id']
        self.executed += 1
        return row

    def finish(self):
        if self.input_fn() != self.before or host_identity() != self.host:
            raise RuntimeError('stage inputs or host policy changed during execution')
        return dict(self.campaign.reconcile(self.stage, self.before), executed=self.executed, reused=self.reused)


def dependency_guard(go, cache, output, stage='suite', extra=()):
    """Resolve modules offline in a contained unit, then guard their full sources.

    Discovery runs on every process admission; no stale go-list graph is trusted.
    Module roots include transitive imports, tests, embeds and replacements.
    """
    import uuid
    from campaign import InputGuard, resource_accepted
    paths = source_paths(stage) + list(extra)
    goroot = Path(go).parent.parent
    paths += [goroot / p for p in ('bin', 'pkg/tool', 'src', 'lib', 'VERSION', 'go.env')]
    pre = InputGuard(paths)
    prefix = Path(output) / ('dependencies-' + uuid.uuid4().hex)
    command = [sys.executable, '-B', str(RUNNER), '--output', str(prefix), '--cache', str(cache),
               '--timeout', '30', '--memory-high-mib', '700', '--', 'env', 'GOENV=off', str(go),
               'list', '-deps', '-test', '-f', '{{if .Module}}{{.Module.Dir}}{{end}}',
               './src/lib/pipelang/...', './src/lib/applicationir', './src/lib/application', './src/cmd', './tests/pipelangcompat']
    with Path(str(prefix) + '.runner.log').open('w') as log:
        rc = subprocess.run(command, cwd=ROOT, stdout=log, stderr=subprocess.STDOUT).returncode
    if rc or not resource_accepted(json.loads(Path(str(prefix) + '.json').read_text())):
        raise RuntimeError('contained dependency discovery failed: ' + str(prefix))
    pre.check()
    roots = sorted(set(Path(line) for line in Path(str(prefix) + '.output').read_text().splitlines() if line.strip()))
    if ROOT not in roots or any(not p.is_absolute() or not p.is_dir() for p in roots):
        raise RuntimeError('incomplete module dependency discovery')
    modules = [p for p in roots if p != ROOT]
    guard = InputGuard(paths + modules)
    pre.check()
    pre.close()
    atomic_json(Path(output) / 'dependency-inputs.json', dict(paths=list(map(str, paths + modules)), identity=guard.identity))
    return guard
