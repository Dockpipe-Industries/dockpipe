"""The compiled provider protocol must be selected explicitly before launch."""
from pathlib import Path
import importlib.util
import io
import json
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

SOURCE = Path(__file__).resolve().parents[1] / 'assets/scripts/installed-remote.py'
SPEC = importlib.util.spec_from_file_location('installed_remote_protocol_tests', SOURCE)
REMOTE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(REMOTE)


class InstalledRemoteProtocolTests(unittest.TestCase):
    def test_explicit_protocol_binds_request_schema_public_flag_and_exact_archive_member(self):
        selection = {'cli': {'path': '/selected/cli'}, 'archive': {'path': '/selected/owner.tar.gz'},
                     'member': 'workflows/qualify/config.yml'}
        for protocol, version, flag in [
            ('nonce', 'v1', '--nonce-fixture-facade'),
            ('nonce-v2', 'v2', '--nonce-fixture-facade-v2'),
        ]:
            with self.subTest(protocol=protocol), tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / 'owner'
                schema = 'dockpipe.desktop.nonce-fixture-facade/' + version
                process = SimpleNamespace(stdin=io.BytesIO(), stdout=io.BytesIO(), returncode=0,
                                          wait=lambda **arguments: 0)
                with patch.object(REMOTE, 'pinned', side_effect=lambda value: Path(value['path'])), \
                        patch.object(REMOTE.subprocess, 'Popen', return_value=process) as launch, \
                        patch.object(REMOTE.FacadeClient, 'event', return_value={
                            'schema': schema, 'stage': 'ready', 'check_only': False}):
                    client = REMOTE.FacadeClient(selection, {'generated': 'configuration'}, output, protocol)
                    client.stop()
                request = json.loads((output / 'request.json').read_text())
                self.assertEqual(schema, request['schema'])
                command = launch.call_args.args[0]
                self.assertEqual('tar:///selected/owner.tar.gz##workflows/qualify/config.yml', command[2])
                self.assertIn(flag, command)
                self.assertEqual(0, json.loads((output / 'workflow-exit.json').read_text())['exit_code'])

    def test_unknown_protocol_is_rejected_before_files_or_processes(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'must-not-exist'
            with patch.object(REMOTE.subprocess, 'Popen') as launch:
                with self.assertRaises(ValueError):
                    REMOTE.FacadeClient({}, {}, output, protocol='caller-guessed-policy')
            self.assertFalse(output.exists())
            launch.assert_not_called()


if __name__ == '__main__':
    unittest.main()
