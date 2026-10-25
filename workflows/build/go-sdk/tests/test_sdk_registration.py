"""An explicit registered CI dispatch reuses the protected SDK transaction."""
import ast
from pathlib import Path
import re
import unittest

import yaml

ROOT = Path(__file__).resolve().parents[4]


def workflow(name):
    return yaml.load((ROOT / '.github/workflows' / name).read_text(), Loader=yaml.BaseLoader)


def selected(condition, event, build=False, publish=False, ref='refs/heads/dev'):
    expression = condition.removeprefix('${{').removesuffix('}}').strip()
    values = {
        'github.event_name': event,
        'github.ref': ref,
        'inputs.build_go_sdk': build,
        'inputs.publish_go_sdk': publish,
    }
    for name, value in values.items():
        expression = expression.replace(name, repr(value))
    expression = expression.replace('&&', ' and ').replace('||', ' or ')
    expression = re.sub(r'!(?!=)', ' not ', expression).strip()
    tree = ast.parse(expression, mode='eval')
    allowed = (ast.Expression, ast.BoolOp, ast.And, ast.Or, ast.UnaryOp, ast.Not,
               ast.Compare, ast.Eq, ast.NotEq, ast.Constant)
    if any(not isinstance(node, allowed) for node in ast.walk(tree)):
        raise AssertionError('Unsupported dispatch condition: ' + condition)
    return eval(compile(tree, '<dispatch condition>', 'eval'), {'__builtins__': {}})


class SDKRegistrationTests(unittest.TestCase):
    def setUp(self):
        self.ci = workflow('ci.yml')
        self.sdk = workflow('go-sdk.yml')

    def test_both_entrypoints_have_false_boolean_defaults(self):
        inputs = self.ci['on']['workflow_dispatch']['inputs']
        self.assertEqual(set(inputs), {'build_go_sdk', 'publish_go_sdk'})
        for definition in inputs.values():
            self.assertEqual(definition['type'], 'boolean')
            self.assertEqual(definition['required'], 'false')
            self.assertEqual(definition['default'], 'false')
        for event in ('workflow_call', 'workflow_dispatch'):
            definition = self.sdk['on'][event]['inputs']['publish']
            self.assertEqual(definition['type'], 'boolean')
            self.assertEqual(definition['required'], 'false')
            self.assertEqual(definition['default'], 'false')

    def test_only_explicit_manual_selection_calls_sdk_and_skips_full_ci(self):
        sdk = self.ci['jobs']['go-sdk']
        for event in ('push', 'pull_request', 'workflow_dispatch'):
            for build in (False, True):
                for publish in (False, True):
                    with self.subTest(event=event, build=build, publish=publish):
                        expected = event == 'workflow_dispatch' and (build or publish)
                        self.assertEqual(selected(sdk['if'], event, build, publish), expected)
                        for name in ('test', 'test-windows'):
                            self.assertEqual(selected(self.ci['jobs'][name]['if'], event, build, publish),
                                             not expected)
        self.assertEqual(self.ci['on']['pull_request']['branches'], ['master', 'staging', 'dev'])
        self.assertEqual(self.ci['on']['push']['branches'], ['master', 'staging', 'dev'])

    def test_sdk_dispatch_skips_never_emit_required_pr_check_names(self):
        required_names = {'test', 'test-windows'}
        for event in ('push', 'pull_request', 'workflow_dispatch'):
            for build in (False, True):
                for publish in (False, True):
                    sdk_selected = event == 'workflow_dispatch' and (build or publish)
                    with self.subTest(event=event, build=build, publish=publish):
                        names = {selected(self.ci['jobs'][job]['name'], event, build, publish)
                                 for job in ('test', 'test-windows')}
                        if sdk_selected:
                            self.assertEqual(names, {'sdk-selection-linux-tests-skipped',
                                                     'sdk-selection-windows-tests-skipped'})
                            self.assertTrue(names.isdisjoint(required_names))
                        else:
                            self.assertEqual(names, required_names)

    def test_local_call_forwards_only_publication_opt_in_with_minimal_permission(self):
        job = self.ci['jobs']['go-sdk']
        self.assertEqual(job['uses'], './.github/workflows/go-sdk.yml')
        self.assertEqual(job['with'], {'publish': '${{ inputs.publish_go_sdk }}'})
        self.assertEqual(job['permissions'], {'contents': 'write'})
        self.assertNotIn('secrets', job)
        self.assertNotIn('steps', job)
        self.assertNotIn('needs', job)
        for name in ('test', 'test-windows'):
            self.assertEqual(self.ci['jobs'][name]['permissions'], {'contents': 'read'})

    def test_called_workflow_retains_reproduction_and_protected_publication(self):
        self.assertEqual(self.sdk['permissions'], {'contents': 'read'})
        jobs = self.sdk['jobs']
        self.assertEqual(set(jobs), {'reproduce', 'publish'})
        publish = jobs['publish']
        self.assertEqual(publish['if'], 'inputs.publish')
        self.assertEqual(publish['needs'], 'reproduce')
        self.assertEqual(publish['environment'], 'release')
        self.assertEqual(publish['permissions'], {'contents': 'write'})
        self.assertEqual(publish['steps'][-1]['env']['GH_TOKEN'], '${{ github.token }}')
        self.assertIn('workflows/publish/go-sdk/config.yml', publish['steps'][-1]['run'])
        reproduce = jobs['reproduce']
        self.assertTrue(any('workflows/build/go-sdk/locked.yml' in step.get('run', '')
                            for step in reproduce['steps']))

    def test_sdk_caller_cannot_cancel_publication_and_normal_concurrency_is_preserved(self):
        concurrency = self.ci['concurrency']
        self.assertIn("&& '-go-sdk' || ''", concurrency['group'])
        self.assertEqual(self.sdk['concurrency']['cancel-in-progress'], 'false')
        for event in ('push', 'pull_request', 'workflow_dispatch'):
            for build, publish in ((False, False), (True, False), (False, True), (True, True)):
                for ref in ('refs/heads/staging', 'refs/heads/dev'):
                    with self.subTest(event=event, build=build, publish=publish, ref=ref):
                        sdk_selected = event == 'workflow_dispatch' and (build or publish)
                        expected = ref != 'refs/heads/staging' and not sdk_selected
                        self.assertEqual(selected(concurrency['cancel-in-progress'], event, build, publish, ref),
                                         expected)


if __name__ == '__main__':
    unittest.main()
