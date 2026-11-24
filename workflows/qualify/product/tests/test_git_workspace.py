"""Generated project Git admission and stale diagnostics fail closed."""
import importlib.util
import json
import os
from pathlib import Path
import sys
import tempfile
from types import SimpleNamespace
import unittest


OWNER = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(OWNER / 'drivers'))
import git_workspace

SPEC = importlib.util.spec_from_file_location('git_fixture_producer', OWNER / 'fixtures/workspace-git/build.py')
BUILDER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(BUILDER)


class GitWorkspaceTests(unittest.TestCase):
    def test_empty_or_foreign_success_response_cannot_qualify_snapshot_refresh(self):
        root = Path('/generated/private/repo')
        response = {'root': str(root), 'head': 'a' * 40, 'snapshot': 'b' * 64, 'changes': []}
        self.assertTrue(git_workspace.valid_snapshot(response, root))
        for fields in [{'head': ''}, {'head': 'short'}, {'snapshot': ''}, {'root': '/another/repo'},
                       {'changes': None}, {'changes': [{}] * 129}]:
            with self.subTest(fields=fields):
                self.assertFalse(git_workspace.valid_snapshot(dict(response, **fields), root))
        self.assertFalse(git_workspace.valid_snapshot({}, root))

    def test_ready_requires_current_nonce_pid_and_private_contained_repo(self):
        with tempfile.TemporaryDirectory() as temporary:
            runtime = Path(temporary)
            root = runtime / 'generated-repo'
            root.mkdir(mode=0o700)
            nonce = 'a' * 32
            broker = SimpleNamespace(pid=12345)
            ready = {'schema': git_workspace.FIXTURE_SCHEMA, 'nonce': nonce,
                     'pid': broker.pid, 'root': str(root)}
            self.assertEqual(git_workspace.admit_ready(ready, nonce, broker, runtime), root)
            for fields in [{'nonce': 'b' * 32}, {'pid': broker.pid + 1}, {'root': '/tmp'}]:
                with self.subTest(fields=fields):
                    with self.assertRaises(ValueError):
                        git_workspace.admit_ready(dict(ready, **fields), nonce, broker, runtime)
            alias = runtime / 'alias'
            alias.symlink_to(root, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, 'escaped'):
                git_workspace.admit_ready(dict(ready, root=str(alias)), nonce, broker, runtime)
            root.chmod(0o755)
            with self.assertRaisesRegex(ValueError, 'not private'):
                git_workspace.admit_ready(ready, nonce, broker, runtime)

    def test_stale_rejection_requires_exact_normal_diagnostic_and_exit(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            logs = output / 'stale-commit'
            logs.mkdir()
            process = logs / 'process.json'
            diagnostic = logs / 'driver.stderr'
            process.write_text(json.dumps({'exit_code': 1, 'failure': ''}))
            diagnostic.write_text('operation logging\n' + git_workspace.STALE_DIAGNOSTIC + '\n')
            self.assertTrue(git_workspace.stale_rejected(output))
            for text in ['wrapper refused', git_workspace.STALE_DIAGNOSTIC + '\nother failure']:
                diagnostic.write_text(text)
                self.assertFalse(git_workspace.stale_rejected(output))
            diagnostic.write_text(git_workspace.STALE_DIAGNOSTIC)
            process.write_text(json.dumps({'exit_code': 1, 'failure': 'timeout'}))
            self.assertFalse(git_workspace.stale_rejected(output))

    def test_current_unowned_process_cannot_match_another_starttime(self):
        records = {}
        git_workspace.observe_tree(os.getpid(), 'not-this-kernel-identity', set(), records)
        self.assertEqual(records, {})

    def test_go_list_documents_preserve_independent_package_records(self):
        self.assertEqual(list(BUILDER.documents(' {"Dir":"first"}\n {"Dir":"second"}\n')),
                         [{'Dir': 'first'}, {'Dir': 'second'}])
        with self.assertRaises(json.JSONDecodeError):
            list(BUILDER.documents('{malformed}'))


if __name__ == '__main__':
    unittest.main()
