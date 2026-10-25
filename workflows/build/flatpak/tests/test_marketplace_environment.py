"""The independent installed Marketplace consumer owns all runtime context."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location(
    'marketplace_environment_fixture', Path(__file__).resolve().parents[4]
    / 'workflows/build/flatpak/assets/scripts/verify-marketplace-cockpit.py')
CHECK = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECK)


class MarketplaceEnvironmentTest(unittest.TestCase):
    def exercise_resolution(self, outside):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            installation = root / 'installation'
            installation.mkdir()
            catalog = root / 'catalog'
            catalog.mkdir()
            build = root / 'build'
            build.mkdir()
            for name in ('workspace-package-test', 'cockpit-package-rendering-test'):
                (build / name).touch()
            output = root / 'consumer'
            manifest = {'platform': 'linux-amd64-flatpak-org.kde.Platform-6.10',
                        'packages': {'resolvers': [{'name': 'cockpit', 'version': '0.1.3'}]}}
            (catalog / 'packages-store-manifest.json').write_text(json.dumps(manifest))
            archive_bytes = b'unit fixture archive, not a native catalog qualification'
            archive = output / 'global/packages/resolvers/fixture.tar.gz'
            entry = {'kind': 'resolver', 'name': 'cockpit', 'version': '0.1.3',
                     'sha256': hashlib.sha256(archive_bytes).hexdigest(),
                     'path': str(archive), 'removable': True}
            discovery = {'platform': manifest['platform'], 'packages': [entry]}
            commands = []

            def run(command, environment, log, timeout):
                commands.append((command, environment.copy()))
                self.assertEqual(timeout, 180)
                if 'package' in command and 'catalog' in command:
                    value = discovery
                elif 'package' in command and 'list' in command:
                    value = {'packages': [entry] if archive.exists() else []}
                elif 'scope' in command:
                    assets = root / 'parent-cache' if outside else output / 'project/cache/resolvers/cockpit/assets'
                    value = str(assets)
                else:
                    archive.parent.mkdir(parents=True)
                    archive.write_bytes(archive_bytes)
                    return 'mocked Settings enable'
                destination = next(item.split('=', 2)[2] for item in command
                                   if item.startswith('--env=DOCKPIPE_FIXTURE_STDOUT='))
                Path(destination).write_text(value if isinstance(value, str) else json.dumps(value))
                return ''

            inherited = {'FLATPAK_USER_DIR': str(installation), 'DOCKPIPE_WORKDIR': '/producer/workdir',
                         'DOCKPIPE_SOURCE_ROOT': '/producer/source', 'DOCKPIPE_ARTIFACT_ROOT': '/producer/artifacts',
                         'DOCKPIPE_PACKAGES_ROOT': '/producer/store', 'DOCKPIPE_BIN': '/producer/cli',
                         'DOCKPIPE_GLOBAL_ROOT': '/producer/global', 'DOCKPIPE_WORKFLOW_RUN_ID': 'parent-run'}
            server = mock.MagicMock()
            server.server_port = 32123
            with mock.patch.dict(os.environ, inherited), \
                 mock.patch.object(CHECK.COMPATIBILITY.COCKPIT, 'load_lock', return_value={'version': '0.1.3'}), \
                 mock.patch.object(CHECK.subprocess, 'run'), \
                 mock.patch.object(CHECK.http.server, 'ThreadingHTTPServer', return_value=server), \
                 mock.patch.object(CHECK.ssl, 'SSLContext'), \
                 mock.patch.object(CHECK.threading, 'Thread'), \
                 mock.patch.object(CHECK.HOST, 'run_command', side_effect=run):
                message = 'outside the isolated fixture state' if outside else 'Expected one fixture-owned persisted launcher settings file'
                with self.assertRaisesRegex(ValueError, message):
                    CHECK.verify(catalog, build, root / 'lock.json', output)
            self.assertGreaterEqual(len(commands), 5)
            for command, environment in commands:
                self.assertFalse(any(key.startswith('DOCKPIPE_') for key in environment))
                self.assertEqual(environment['FLATPAK_USER_DIR'], str(installation))
                self.assertEqual(environment['HOME'], str(output / 'home'))
                self.assertIn('--cwd=' + str(output / 'project'), command)
                self.assertIn('--env=DOCKPIPE_WORKDIR=' + str(output / 'project'), command)
                self.assertIn('--env=DOCKPIPE_GLOBAL_ROOT=' + str(output / 'global'), command)
                self.assertIn('--env=DOCKPIPE_BIN=/app/bin/dockpipe', command)
                self.assertIn('--unset-env=DOCKPIPE_PACKAGES_ROOT', command)
                self.assertIn('--unset-env=DOCKPIPE_WORKSPACE_PACKAGE', command)
            enable = next(command for command, _ in commands if '--command=' + str(build / 'workspace-package-test') in command)
            self.assertIn('--env=DOCKPIPE_PACKAGE_TEST_ACTION=enable', enable)
            self.assertIn('--env=DOCKPIPE_PACKAGE_TEST_REMOTE=https://127.0.0.1:32123/packages-store-manifest.json', enable)
            server.shutdown.assert_called_once()
            server.server_close.assert_called_once()

    def test_parent_context_is_removed_and_fixture_workdir_is_explicit(self):
        self.exercise_resolution(outside=False)

    def test_outside_asset_is_still_rejected(self):
        self.exercise_resolution(outside=True)


if __name__ == '__main__':
    unittest.main()
