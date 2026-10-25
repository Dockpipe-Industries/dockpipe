"""The canonical Go child owns fresh package state and no parent run context."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


class GoUnitChildEnvironmentTests(unittest.TestCase):
    def test_real_child_retains_tool_inputs_and_isolates_parent_context(self):
        script = Path(__file__).resolve().parents[1] / 'assets/scripts/test.py'
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            go = root / 'selected-go'
            go.write_text('#!' + sys.executable + '\nimport json, os, sys\n'
                          'print(json.dumps({"arguments":sys.argv[1:], "cwd":os.getcwd(), "environment":dict(os.environ)}))\n')
            go.chmod(0o755)
            environment = dict(os.environ, DOCKPIPE_SOURCE_ROOT=str(root), DOCKPIPE_TEST_GO=str(go),
                               DOCKPIPE_EVENT_LOG='unrelated-parent-event-log', DOCKPIPE_PACKAGE_ROOT='parent-package',
                               DOCKPIPE_PACKAGES_ROOT='parent-packages', DOCKPIPE_BIN='parent-cli',
                               DOCKPIPE_GLOBAL_ROOT='parent-global', DOCKPIPE_SYSTEM_ROOT='parent-system',
                               DOCKPIPE_BOOTSTRAP_ARCHIVE='selected-bootstrap', DOCKPIPE_GO_SDK_ARCHIVE='selected-sdk',
                               GOCACHE='selected-go-cache')
            result = subprocess.run([sys.executable, str(script), '-run', 'OwnedTest', './...'],
                                    env=environment, check=True, capture_output=True, text=True, timeout=15)
            child = json.loads(result.stdout)
            self.assertEqual(child['arguments'], ['test', '-run', 'OwnedTest', './...'])
            self.assertEqual(child['cwd'], str(root))
            actual = child['environment']
            for name in ('DOCKPIPE_EVENT_LOG', 'DOCKPIPE_PACKAGE_ROOT', 'DOCKPIPE_PACKAGES_ROOT',
                         'DOCKPIPE_BIN', 'DOCKPIPE_SOURCE_ROOT'):
                self.assertNotIn(name, actual)
            for name in ('DOCKPIPE_TEST_GO', 'DOCKPIPE_BOOTSTRAP_ARCHIVE', 'DOCKPIPE_GO_SDK_ARCHIVE', 'GOCACHE'):
                self.assertEqual(actual[name], environment[name])
            self.assertNotEqual(actual['DOCKPIPE_GLOBAL_ROOT'], environment['DOCKPIPE_GLOBAL_ROOT'])
            self.assertNotEqual(actual['DOCKPIPE_SYSTEM_ROOT'], environment['DOCKPIPE_SYSTEM_ROOT'])


if __name__ == '__main__':
    unittest.main()
