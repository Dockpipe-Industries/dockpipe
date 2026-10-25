"""Full-context diagnosis preserves the owning release and all existing test gates."""
import importlib.util
import io
import json
import os
from pathlib import Path
import signal
import shutil
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

import yaml

HELPER = Path(__file__).resolve().parents[4] / 'workflows/build/desktop/assets/scripts/layout-diagnostics.py'
SPEC = importlib.util.spec_from_file_location('layout_diagnostics', HELPER)
DIAGNOSTICS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(DIAGNOSTICS)


class FakeSampler:
    closed = False

    def sample(self):
        return {'monotonic_seconds': 1, 'cpu_ticks': [1, 2, 3, 4],
                'page_size_bytes': 4096, 'pages': {'Pages free': 10}, 'load_average': [0, 0, 0]}

    def close(self):
        self.closed = True


class DiagnosticTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.tools = self.directory / 'bin'
        self.tools.mkdir()
        ctest = self.tools / 'ctest'
        ctest.write_text('#!/usr/bin/env python3\nimport json,os,sys,time\n'
                         "open(os.environ['ARGV'], 'w').write(json.dumps(sys.argv[1:]))\n"
                         "time.sleep(float(os.environ.get('SLEEP', '0')))\n"
                         "sys.exit(int(os.environ.get('EXIT', '0')))\n")
        ctest.chmod(0o755)
        self.environment = patch.dict(os.environ, PATH=str(self.tools) + os.pathsep + os.environ['PATH'],
                                      ARGV=str(self.directory / 'argv.json'))
        self.environment.start()
        self.addCleanup(self.environment.stop)

    def test_full_ctest_runs_once_without_filter_parallel_retry_or_timeout_override(self):
        sampler = FakeSampler()
        result = DIAGNOSTICS.run_ctest(self.directory / 'build', self.directory / 'evidence', lambda: sampler)
        self.assertEqual(result, 0)
        self.assertEqual(json.loads((self.directory / 'argv.json').read_text()),
                         ['--test-dir', str(self.directory / 'build'), '--verbose'])
        self.assertTrue(sampler.closed)
        self.assertEqual(len((self.directory / 'evidence/host-resources.jsonl').read_text().splitlines()), 1)

    def test_original_ctest_failure_is_propagated(self):
        with patch.dict(os.environ, EXIT='8'):
            self.assertEqual(DIAGNOSTICS.run_ctest('build', self.directory, FakeSampler), 8)

    def test_sampler_admission_failure_does_not_start_test(self):
        sampler = FakeSampler()
        with patch.object(sampler, 'sample', side_effect=RuntimeError('unavailable')):
            with self.assertRaises(RuntimeError):
                DIAGNOSTICS.run_ctest('build', self.directory, lambda: sampler)
        self.assertTrue(sampler.closed)
        self.assertFalse((self.directory / 'argv.json').exists())

    def test_periodic_sampler_failure_fails_diagnostic_without_retrying_test(self):
        sampler = FakeSampler()
        first = sampler.sample()
        with patch.object(sampler, 'sample', side_effect=[first, RuntimeError('unavailable')]), \
                patch.object(DIAGNOSTICS, 'INTERVAL_SECONDS', 0.01), patch.dict(os.environ, SLEEP='0.1'):
            self.assertEqual(DIAGNOSTICS.run_ctest('build', self.directory, lambda: sampler), 1)
        self.assertTrue(sampler.closed)

    def test_sample_capacity_is_bounded(self):
        with patch.object(DIAGNOSTICS, 'MAX_SAMPLES', 1), \
                patch.object(DIAGNOSTICS, 'INTERVAL_SECONDS', 0.001), patch.dict(os.environ, SLEEP='0.02'):
            self.assertEqual(DIAGNOSTICS.run_ctest('build', self.directory, FakeSampler), 1)
        self.assertEqual(len((self.directory / 'host-resources.jsonl').read_text().splitlines()), 1)

    def test_telemetry_never_overwrites_prior_evidence(self):
        (self.directory / 'host-resources.jsonl').write_text('retained')
        with self.assertRaises(FileExistsError):
            DIAGNOSTICS.run_ctest('build', self.directory, FakeSampler)
        self.assertEqual((self.directory / 'host-resources.jsonl').read_text(), 'retained')

    def test_cpu_counter_wrap_and_busy_percentage(self):
        stream = io.StringIO()
        DIAGNOSTICS.write_sample(stream, {'cpu_ticks': [2, 1, 8, 0]},
                                 {'cpu_ticks': [2 ** 32 - 2, 0, 3, 0]})
        self.assertEqual(json.loads(stream.getvalue())['busy_cpu_percent'], 50)

    def test_guard_rejects_abbreviated_and_revision_sources_before_commands(self):
        for source in ['HEAD', 'abc', 'a' * 40 + '~1']:
            with self.subTest(source=source), patch.object(DIAGNOSTICS, 'command') as command:
                with self.assertRaises(ValueError):
                    DIAGNOSTICS.guard_source(source)
                command.assert_not_called()

    def test_guard_requires_native_intel(self):
        for platform in [('Linux', 'x86_64'), ('Darwin', 'arm64')]:
            with self.subTest(platform=platform), patch.object(DIAGNOSTICS, 'command', side_effect=platform):
                with self.assertRaises(ValueError):
                    DIAGNOSTICS.guard_source('a' * 40)

    def test_guard_compares_whole_launcher_and_browser_lock(self):
        with patch.object(DIAGNOSTICS, 'command', side_effect=['Darwin', 'x86_64', 'commit', 'b' * 40, 'c' * 40, 'd' * 40]), \
                patch.object(DIAGNOSTICS.subprocess, 'run') as run:
            receipt = DIAGNOSTICS.guard_source('a' * 40)
        self.assertEqual(receipt['diagnostic_source'], 'b' * 40)
        self.assertEqual(run.call_args_list[0].args[0], ['git', 'diff', '--quiet', 'a' * 40, 'HEAD', '--',
                                              'src/dockpipe-app',
                                              'workflows/dependencies/browser-sdk/assets/spatialweb-browser.lock.json'])
        self.assertTrue(run.call_args.kwargs['check'])
        self.assertEqual(run.call_args_list[1].args[0], ['git', 'diff', '--quiet', 'HEAD', '--', *DIAGNOSTICS.SOURCE_PATHS])

    def test_source_conflict_fails_admission(self):
        with patch.object(DIAGNOSTICS, 'command', side_effect=['Darwin', 'x86_64', 'commit']), \
                patch.object(DIAGNOSTICS.subprocess, 'run', side_effect=subprocess.CalledProcessError(1, 'git')):
            with self.assertRaises(subprocess.CalledProcessError):
                DIAGNOSTICS.guard_source('a' * 40)

    def test_cancellation_reaps_owned_ctest(self):
        driver = self.directory / 'driver.py'
        driver.write_text('import importlib.util,os\n'
                          f's=importlib.util.spec_from_file_location("d", {str(HELPER)!r})\n'
                          'd=importlib.util.module_from_spec(s);s.loader.exec_module(d)\n'
                          'class Sampler:\n'
                          ' def sample(self): return {"cpu_ticks":[1,2,3,4]}\n'
                          ' def close(self): pass\n'
                          'try: d.run_ctest("build",os.environ["EVIDENCE"],Sampler)\n'
                          'except KeyboardInterrupt: raise SystemExit(130)\n')
        ctest = self.tools / 'ctest'
        ctest.write_text('#!/usr/bin/env python3\nimport os,time\n'
                         'open(os.environ["PIDFILE"],"w").write(str(os.getpid()))\n'
                         'time.sleep(30)\n')
        ctest.chmod(0o755)
        pidfile = self.directory / 'pid'
        environment = dict(os.environ, PIDFILE=str(pidfile), EVIDENCE=str(self.directory / 'evidence'))
        child = subprocess.Popen([sys.executable, str(driver)], env=environment)
        try:
            deadline = time.monotonic() + 5
            while not pidfile.exists() and time.monotonic() < deadline:
                time.sleep(0.01)
            self.assertTrue(pidfile.exists())
            pid = int(pidfile.read_text())
            child.send_signal(signal.SIGTERM)
            self.assertEqual(child.wait(timeout=5), 130)
            with self.assertRaises(ProcessLookupError):
                os.kill(pid, 0)
        finally:
            if child.poll() is None:
                child.kill()
                child.wait()


class BuildUnixDiagnosticTests(unittest.TestCase):
    def test_actual_build_orchestration_preserves_failure_and_retains_private_logs(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            desktop = root / 'workflows/build/desktop/assets/scripts'
            desktop.mkdir(parents=True)
            shutil.copyfile(Path(__file__).resolve().parents[4] / 'workflows/build/desktop/assets/scripts/build-unix.sh', desktop / 'build-unix.sh')
            browser = root / 'workflows/dependencies/browser-sdk/assets/scripts/browser-runtime.py'
            browser.parent.mkdir(parents=True)
            browser.write_text('')
            helper = desktop / 'layout-diagnostics.py'
            helper.write_text(
                'import pathlib,sys\n'
                'args=sys.argv; evidence=pathlib.Path(args[args.index("--evidence")+1]);evidence.mkdir(parents=True,exist_ok=True)\n'
                'if args[1]=="guard": (evidence/"source.json").write_text("{}")\n'
                'else:\n'
                ' build=pathlib.Path(args[args.index("--build")+1]);log=build/"Testing/Temporary/LastTest.log";log.parent.mkdir(parents=True);log.write_text("retained actual CTest failure fixture")\n'
                ' raise SystemExit(8)\n')
            tools = root / 'bin'
            tools.mkdir()
            (tools / 'uname').write_text('#!/bin/sh\nif [ "$1" = -s ]; then echo Darwin; else echo x86_64; fi\n')
            (tools / 'cmake').write_text('#!/usr/bin/env python3\nimport json,os,pathlib,sys\n'
                                         'with open(os.environ["COMMANDS"],"a") as f: f.write(json.dumps(sys.argv[1:])+"\\n")\n'
                                         'if "-B" in sys.argv:\n'
                                         ' build=pathlib.Path(sys.argv[sys.argv.index("-B")+1]);build.mkdir(parents=True,exist_ok=True);(build/"CMakeCache.txt").write_text("fixture cache")\n')
            for tool in tools.iterdir():
                tool.chmod(0o755)
            sdk = root / 'sdk'
            sdk.mkdir()
            evidence = root / 'private-evidence'
            environment = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ['PATH'],
                               COMMANDS=str(root / 'commands.jsonl'), DOCKPIPE_BROWSER_SDK=str(sdk),
                               DOCKPIPE_LAYOUT_DIAGNOSTICS=str(evidence))
            result = subprocess.run(['bash', str(desktop / 'build-unix.sh'), '--enable-qualification',
                                     '--diagnose-layout', 'a' * 40, '0.6.27', str(root / 'artifacts')],
                                    env=environment, capture_output=True, text=True)
            self.assertEqual(result.returncode, 8, result.stdout + result.stderr)
            self.assertEqual((evidence / 'LastTest.log').read_text(), 'retained actual CTest failure fixture')
            self.assertEqual((evidence / 'CMakeCache.txt').read_text(), 'fixture cache')
            commands = [json.loads(line) for line in (root / 'commands.jsonl').read_text().splitlines()]
            configure = next(row for row in commands if '-B' in row)
            self.assertIn('-DDOCKPIPE_ENABLE_QUALIFICATION=ON', configure)
            self.assertIn('-DCMAKE_CXX_FLAGS=-DDOCKPIPE_PROFILE_DASHBOARD_LAYOUT', configure)
            build = next(row for row in commands if '--build' in row)
            self.assertEqual(build[-2:], ['--parallel', '2'])
            self.assertNotIn('--target', build)
            self.assertFalse(Path(build[1]).exists())
            self.assertFalse(any((root / 'artifacts').iterdir()))

    def test_build_flag_without_qualification_is_rejected(self):
        result = subprocess.run(['bash', str(Path(__file__).resolve().parents[4] / 'workflows/build/desktop/assets/scripts/build-unix.sh'),
                                 '--diagnose-layout', 'a' * 40, '0.6.27', '/unused-output'],
                                capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('requires package display qualification', result.stderr)


class WorkflowTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workflow = yaml.load((Path(__file__).resolve().parents[4] / '.github/workflows/release.yml').read_text(), Loader=yaml.BaseLoader)

    def test_default_off_manual_dryrun_qualification_admission(self):
        inputs = self.workflow['on']['workflow_dispatch']['inputs']
        self.assertEqual(inputs['diagnose_layout']['default'], 'false')
        meta = next(step for step in self.workflow['jobs']['meta']['steps'] if step.get('id') == 'm')
        script = meta['run']
        self.assertIn('"$GITHUB_EVENT_NAME" != workflow_dispatch || "$dry_run" != true || "$qualify_package_display" != true', script)
        self.assertIn('[[ "$diagnostic_product_source" =~ ^[0-9a-f]{40}$ ]]', script)

    def test_actual_metadata_flag_accepts_only_explicit_manual_dryrun_with_qualification(self):
        metadata = next(step for step in self.workflow['jobs']['meta']['steps'] if step.get('id') == 'm')
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            (directory / 'VERSION').write_text('0.6.0\n')
            output = directory / 'outputs'
            for event, dry_run, qualification, source, allowed in [
                    ('workflow_dispatch', 'true', 'true', 'a' * 40, True),
                    ('workflow_dispatch', 'false', 'true', 'a' * 40, False),
                    ('push', 'true', 'true', 'a' * 40, False),
                    ('workflow_dispatch', 'true', 'false', 'a' * 40, False),
                    ('workflow_dispatch', 'true', 'true', 'HEAD', False)]:
                with self.subTest(event=event, dry_run=dry_run, qualification=qualification, source=source):
                    output.write_text('')
                    environment = dict(os.environ, GITHUB_REF='refs/heads/master', GITHUB_EVENT_NAME=event,
                                       GITHUB_OUTPUT=str(output), INPUT_VERSION='', RELEASE_ENABLED='true',
                                       INPUT_DRY_RUN=dry_run, INPUT_BUILD_MSI='true', INPUT_STAGING='false',
                                       INPUT_QUALIFY_PACKAGE_DISPLAY=qualification, INPUT_DIAGNOSE_LAYOUT='true',
                                       INPUT_DIAGNOSTIC_PRODUCT_SOURCE=source)
                    result = subprocess.run(['bash', '-c', metadata['run']], cwd=directory,
                                            env=environment, capture_output=True, text=True)
                    self.assertEqual(result.returncode == 0, allowed, result.stdout + result.stderr)
                    if allowed:
                        self.assertIn('diagnose_layout=true\n', output.read_text())
                        self.assertIn('diagnostic_product_source=' + source + '\n', output.read_text())
                    else:
                        self.assertEqual(output.read_text(), '')

    def test_intel_only_diagnostic_uses_owning_preparation_and_disables_assembly(self):
        jobs = self.workflow['jobs']
        include = jobs['build-unix']['strategy']['matrix']['include']
        self.assertIn('fromJSON(\'[{"runner":"macos-15-intel","platform":"darwin-amd64"}]\')', include)
        steps = jobs['build-unix']['steps']
        prepare = next(step for step in steps if step.get('name') == 'Build native CLI and installers')
        self.assertEqual(prepare['run'], 'python3 workflows/shared/launch.py workflows/build/platform/config.yml -- "${{ needs.meta.outputs.version }}"')
        self.assertIn("needs.meta.outputs.diagnose_layout != 'true'", jobs['assemble']['if'])
        for name in ['build-msi', 'build-flatpak']:
            self.assertEqual(jobs[name]['if'], "needs.meta.outputs.diagnose_layout != 'true'")
        guard = next(index for index, step in enumerate(steps) if step.get('name') == 'Bind unchanged diagnostic product source')
        prepare_index = steps.index(prepare)
        self.assertLess(guard, prepare_index)

    def test_full_build_and_original_ctest_remain_default_and_logs_private(self):
        script = (Path(__file__).resolve().parents[4] / 'workflows/build/desktop/assets/scripts/build-unix.sh').read_text()
        self.assertIn('cmake --build "$build" --parallel 2\n', script)
        self.assertIn('ctest --test-dir "$build" --output-on-failure', script)
        self.assertIn('args+=(-DCMAKE_CXX_FLAGS=-DDOCKPIPE_PROFILE_DASHBOARD_LAYOUT)', script)
        self.assertIn('"$enable_qualification" == ON', script)
        self.assertIn('LastTest.log', script)
        step = next(step for step in self.workflow['jobs']['build-unix']['steps'] if step.get('name') == 'Retain full-context layout diagnostic evidence')
        self.assertIn('always()', step['if'])
        self.assertNotIn('release/artifacts', step['with']['path'])


if __name__ == '__main__':
    unittest.main()
