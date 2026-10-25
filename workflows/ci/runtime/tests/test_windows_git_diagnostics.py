"""Keep opt-in Git diagnostics outside release allocation and publication."""
import ast
from pathlib import Path
import re
import shlex
import unittest

import yaml

ROOT = Path(__file__).resolve().parents[4]


def condition(expression, event, dry_run, diagnostic, layout, enabled, references=None):
    values = {'github.event_name': event, 'inputs.dry_run': dry_run,
              'inputs.diagnose_windows_git': diagnostic, 'inputs.diagnose_layout': layout,
              'vars.DOCKPIPE_RELEASE_ENABLED': 'true' if enabled else 'false'}
    values.update(references or {})
    expression = expression.removeprefix('${{').removesuffix('}}').strip()
    for key, value in values.items():
        expression = expression.replace(key, repr(value))
    expression = expression.replace('&&', ' and ').replace('||', ' or ')
    expression = re.sub(r'!(?!=)', ' not ', expression)
    expression = re.sub(r'\btrue\b', 'True', expression)
    expression = re.sub(r'\bfalse\b', 'False', expression)
    expression = expression.replace("'True'", "'true'").replace("'False'", "'false'")
    tree = ast.parse(expression.strip(), mode='eval')
    allowed = (ast.Expression, ast.BoolOp, ast.And, ast.Or, ast.UnaryOp, ast.Not,
               ast.Compare, ast.Eq, ast.NotEq, ast.Constant)
    if any(not isinstance(node, allowed) for node in ast.walk(tree)):
        raise ValueError('Unexpected diagnostic routing expression')
    return eval(compile(tree, '<workflow-condition>', 'eval'), {'__builtins__': {}})


class WindowsGitDiagnosticsContractTest(unittest.TestCase):
    def setUp(self):
        self.release = yaml.safe_load((ROOT / '.github/workflows/release.yml').read_text())

    def test_actual_job_conditions_preserve_normal_event_matrix(self):
        jobs = self.release['jobs']
        for event in ('push', 'workflow_dispatch', 'workflow_call'):
            for dry_run in (False, True):
                for diagnostic in (False, True):
                    for layout in (False, True):
                        for enabled in (False, True):
                            with self.subTest(event=event, dry_run=dry_run, diagnostic=diagnostic,
                                              layout=layout, enabled=enabled):
                                arguments = (event, dry_run, diagnostic, layout, enabled)
                                self.assertEqual(condition(jobs['meta']['if'], *arguments),
                                                 (event == 'workflow_dispatch' or enabled)
                                                 and not (event == 'workflow_dispatch' and diagnostic))
                                self.assertEqual(condition(jobs['windows-git-diagnostic']['if'], *arguments),
                                                 event == 'workflow_dispatch' and dry_run and diagnostic and not layout)

    def test_diagnostic_is_read_only_bounded_and_isolated_from_publication(self):
        job = self.release['jobs']['windows-git-diagnostic']
        self.assertNotIn('needs', job)
        self.assertNotIn('environment', job)
        self.assertEqual(job['permissions'], {'contents': 'read'})
        self.assertEqual(job['timeout-minutes'], 10)
        self.assertEqual(job['steps'][0]['with'], {'persist-credentials': False})
        self.assertEqual(job['steps'][1]['with'], {'go-version': '1.26.10', 'cache': False})
        step = job['steps'][2]
        self.assertEqual(step['env'], {'GOTOOLCHAIN': 'local', 'GOWORK': 'off', 'GOMAXPROCS': '2'})
        self.assertEqual(step['run'], 'python3 workflows/shared/launch.py workflows/ci/runtime/windows-git-diagnostics.yml')
        self.assertIn("inputs.diagnose_windows_git && format('windows-git-diagnostic-{0}', github.ref)", self.release['concurrency']['group'])
        self.assertFalse(self.release['concurrency']['cancel-in-progress'])
        events = self.release.get('on', self.release.get(True))
        self.assertEqual(events['workflow_dispatch']['inputs']['diagnose_windows_git'], {
            'description': 'Run only bounded Windows Git snapshot diagnostics (manual dry run only)',
            'type': 'boolean', 'default': False,
        })

    def test_all_normal_jobs_remain_downstream_of_metadata(self):
        jobs = self.release['jobs']

        def depends_on_metadata(name, visited):
            if name == 'meta':
                return True
            self.assertNotIn(name, visited)
            needs = jobs[name].get('needs', [])
            if isinstance(needs, str):
                needs = [needs]
            return any(depends_on_metadata(parent, visited | {name}) for parent in needs)

        for name in jobs:
            if name != 'windows-git-diagnostic':
                with self.subTest(job=name):
                    self.assertTrue(depends_on_metadata(name, set()))

    def test_diagnostic_concurrency_never_takes_publication_lock(self):
        expression = self.release['concurrency']['group'].removeprefix('dockpipe-release-')
        for dry_run in (False, True):
            for layout in (False, True):
                for diagnostic in (False, True):
                    ref = 'refs/heads/diagnostic'
                    selected = expression.replace("format('windows-git-diagnostic-{0}', github.ref)",
                                                  repr('windows-git-diagnostic-' + ref))
                    selected = selected.replace(
                        "format('dry-run-{0}{1}', github.ref, inputs.diagnose_layout && '-layout-diagnostic' || '')",
                        repr('dry-run-' + ref + ('-layout-diagnostic' if layout else '')))
                    if diagnostic:
                        expected = 'windows-git-diagnostic-' + ref
                    elif dry_run:
                        expected = 'dry-run-' + ref
                        if layout:
                            expected += '-layout-diagnostic'
                    else:
                        expected = 'publish'
                    self.assertEqual(condition(selected, 'workflow_dispatch', dry_run, diagnostic, layout, True), expected)

    def test_selected_diagnostic_skips_every_normal_job_including_aggregates(self):
        jobs = self.release['jobs']
        for dry_run in (False, True):
            for layout in (False, True):
                for enabled in (False, True):
                    for ref in ('refs/heads/master', 'refs/heads/staging', 'refs/heads/diagnostic'):
                        with self.subTest(dry_run=dry_run, layout=layout, enabled=enabled, ref=ref):
                            results = {}
                            for name, job in jobs.items():
                                needs = job.get('needs', [])
                                if isinstance(needs, str):
                                    needs = [needs]
                                successful_dependencies = all(results[parent] == 'success' for parent in needs)
                                expression = str(job.get('if', 'true'))
                                has_status_check = re.search(r'\b(always|cancelled|failure|success)\(', expression)
                                references = {
                                    'github.ref': ref, 'vars.DEVTO_PUBLISH': 'true',
                                    'vars.DEVTO_ARTICLE_ID': 'fixture', 'vars.DEVTO_ONE_TIME_POST': 'true',
                                }
                                for parent in needs:
                                    references['needs.' + parent + '.result'] = results[parent]
                                for output in re.findall(r'needs\.[A-Za-z0-9_-]+\.outputs\.[A-Za-z0-9_]+', expression):
                                    # Explicit diagnosis never runs metadata, so no release outputs exist.
                                    references[output] = ''
                                for function, value in (('always()', True), ('cancelled()', False),
                                                        ('success()', successful_dependencies), ('failure()', False)):
                                    expression = expression.replace(function, str(value))
                                runnable = (bool(has_status_check) or successful_dependencies) and condition(
                                    expression, 'workflow_dispatch', dry_run, True, layout, enabled, references)
                                results[name] = 'success' if runnable else 'skipped'
                            for name, result in results.items():
                                expected = 'success' if name == 'windows-git-diagnostic' and dry_run and not layout else 'skipped'
                                self.assertEqual(result, expected, name)

    def test_workflow_is_exact_fixture_and_original_failure_remains(self):
        workflow = yaml.safe_load((ROOT / 'workflows/ci/runtime/windows-git-diagnostics.yml').read_text())
        self.assertEqual(len(workflow['steps']), 1)
        self.assertEqual(shlex.split(workflow['steps'][0]['cmd']), [
            'python3', 'workflows/ci/core/assets/scripts/test.py', '-p', '2',
            '-run', '^TestDockerGitSnapshotOrdinaryDirectoryAndPartialClone$',
            '-count', '10', '-timeout', '7m', '-v', './src/lib/infrastructure',
        ])
        source = (ROOT / 'src/lib/infrastructure/docker_git_context_test.go').read_text()
        failure = source[source.index('ordinary, err := prepareDockerGitContext(linked, false)'):]
        self.assertLess(failure.index('dockerGitFailureDiagnostics(t, linked)'), failure.index('t.Fatalf("inactive promisor'))
        self.assertIn('command.Env = append(command.Env, "GIT_TRACE2_EVENT="+trace)', source)
        self.assertIn('io.LimitReader(file, limit+1)', source)


if __name__ == '__main__':
    unittest.main()
