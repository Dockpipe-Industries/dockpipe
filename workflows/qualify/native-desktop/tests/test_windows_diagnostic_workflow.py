"""Windows-only dispatch records fixture timings without satisfying native gates."""
import ast
from pathlib import Path
import re
import unittest

import yaml

ROOT = Path(__file__).resolve().parents[4]
WORKFLOW = ROOT / '.github/workflows/native-macos-symbols.yml'


def evaluate(expression, event, windows_only):
    value = expression.removeprefix('${{').removesuffix('}}').strip()
    value = value.replace('github.event_name', repr(event))
    value = value.replace('inputs.windows_only', repr(windows_only))
    value = value.replace('always()', 'True')
    value = value.replace('&&', ' and ').replace('||', ' or ')
    value = re.sub(r'!(?!=)', ' not ', value).strip()
    tree = ast.parse(value, mode='eval')
    allowed = (ast.Expression, ast.BoolOp, ast.And, ast.Or, ast.UnaryOp, ast.Not,
               ast.Compare, ast.Eq, ast.NotEq, ast.Constant)
    if any(not isinstance(node, allowed) for node in ast.walk(tree)):
        raise AssertionError('Unexpected diagnostic expression: ' + expression)
    return eval(compile(tree, '<diagnostic condition>', 'eval'), {'__builtins__': {}})


class WindowsDiagnosticWorkflowTests(unittest.TestCase):
    def setUp(self):
        self.workflow = yaml.load(WORKFLOW.read_text(), Loader=yaml.BaseLoader)
        self.jobs = self.workflow['jobs']

    def test_diagnostic_is_false_by_default_and_does_not_expand_triggers(self):
        definition = self.workflow['on']['workflow_dispatch']['inputs']['windows_only']
        self.assertEqual(definition['type'], 'boolean')
        self.assertEqual(definition['required'], 'false')
        self.assertEqual(definition['default'], 'false')
        self.assertEqual(set(self.workflow['on']), {'workflow_dispatch', 'pull_request'})
        self.assertEqual(self.workflow['on']['pull_request']['branches'], ['dev', 'staging', 'master'])

    def test_only_selected_manual_diagnostic_omits_other_hosts(self):
        for event in ('pull_request', 'workflow_dispatch', 'push'):
            for requested in (False, True):
                with self.subTest(event=event, requested=requested):
                    diagnostic = event == 'workflow_dispatch' and requested
                    for name in ('owned-symbols', 'host-version', 'flatpak-host', 'required-host'):
                        self.assertEqual(evaluate(self.jobs[name]['if'], event, requested), not diagnostic)
                    self.assertEqual(evaluate(self.jobs['windows-diagnostic-result']['if'], event, requested),
                                     diagnostic)
        windows = self.jobs['windows-terminal']
        self.assertNotIn('if', windows)
        step = next(step for step in windows['steps'] if 'run' in step)
        self.assertIn('workflows/qualify/native-desktop/config.yml -- --platform windows --jobs 2', step['run'])
        self.assertNotIn('--check', step['run'])

    def test_diagnostic_context_never_emits_real_required_gateway_name(self):
        for event in ('pull_request', 'workflow_dispatch'):
            for requested in (False, True):
                with self.subTest(event=event, requested=requested):
                    diagnostic = event == 'workflow_dispatch' and requested
                    name = evaluate(self.jobs['required-host']['name'], event, requested)
                    windows_name = evaluate(self.jobs['windows-terminal']['name'], event, requested)
                    if diagnostic:
                        self.assertNotEqual(name, 'Native desktop host qualification')
                        self.assertEqual(windows_name, 'Windows fixture phase diagnostic')
                    else:
                        self.assertEqual(name, 'Native desktop host qualification')
                        self.assertEqual(windows_name, 'windows-terminal')
        self.assertEqual(self.jobs['required-host']['needs'], ['owned-symbols', 'windows-terminal', 'flatpak-host'])
        diagnostic = self.jobs['windows-diagnostic-result']
        self.assertEqual(diagnostic['name'], 'Windows fixture diagnostic result')
        self.assertEqual(diagnostic['needs'], ['windows-terminal'])
        self.assertEqual(diagnostic['permissions'], {})
        self.assertEqual(diagnostic['steps'][0]['run'], 'test "$WINDOWS_RESULT" = success')
        self.assertNotEqual(self.jobs['required-host']['name'], 'Native desktop host qualification')

    def test_diagnostic_concurrency_cannot_cancel_normal_qualification(self):
        concurrency = self.workflow['concurrency']
        self.assertIn("&& '-windows-diagnostic' || ''", concurrency['group'])
        self.assertFalse(evaluate(concurrency['cancel-in-progress'], 'workflow_dispatch', True))
        self.assertTrue(evaluate(concurrency['cancel-in-progress'], 'workflow_dispatch', False))
        self.assertTrue(evaluate(concurrency['cancel-in-progress'], 'pull_request', True))

    def test_fixture_markers_bracket_existing_operations_without_relaxing_deadlines(self):
        source = (ROOT / 'src/app/tooling/dockpipe-launcher/tests/DesktopAppWindowWinTest.cpp').read_text()
        operations = {
            'QWidget fixture construction': 'QWidget window;',
            'QWidget setWindowTitle': 'window.setWindowTitle("DockPipe isolated Windows embedding fixture");',
            'QWidget resize': 'window.resize(560, 360);',
            'QVBoxLayout construction': 'auto *layout = new QVBoxLayout(&window);',
            'QLabel construction': 'auto *label = new QLabel("Only this disposable test window may be embedded", &window);',
            'QVBoxLayout add QLabel': 'layout->addWidget(label);',
            'QLineEdit construction': 'auto *input = new QLineEdit("Native keyboard input", &window);',
            'QVBoxLayout add QLineEdit': 'layout->addWidget(input);',
        }
        for phase, operation in operations.items():
            block = f'reportPhase("before {phase}");\n        {operation}\n        reportPhase("after {phase}");'
            self.assertIn(block, source)
        self.assertIn('deadline.elapsed() < 5000', source)
        self.assertIn('CHECK(fixtureReady);', source)
        self.assertIn('QTimer::singleShot(60000, &app, &QApplication::quit);', source)
        cmake = (ROOT / 'src/app/tooling/dockpipe-launcher/CMakeLists.txt').read_text()
        self.assertIn('set_tests_properties(desktop-app-window-win PROPERTIES TIMEOUT 50 SKIP_RETURN_CODE 77)', cmake)


if __name__ == '__main__':
    unittest.main()
