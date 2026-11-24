"""Installed qualification cannot mutate defaults or infer normal GUI behavior."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
import os
import sys
from types import SimpleNamespace
from unittest.mock import patch


SPEC = importlib.util.spec_from_file_location(
    'installed_product', Path(__file__).resolve().parents[1] / 'assets/scripts/verify-installed-product.py')
PRODUCT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PRODUCT)
GUI_SPEC = importlib.util.spec_from_file_location(
    'installed_product_gui', Path(__file__).resolve().parents[1] / 'assets/scripts/installed-product-gui.py')
GUI = importlib.util.module_from_spec(GUI_SPEC)
GUI_SPEC.loader.exec_module(GUI)
MIGRATION_SPEC = importlib.util.spec_from_file_location(
    'retained_profile', Path(__file__).resolve().parents[1] / 'assets/scripts/retained-profile.py')
MIGRATION = importlib.util.module_from_spec(MIGRATION_SPEC)
MIGRATION_SPEC.loader.exec_module(MIGRATION)


class InstalledProductTests(unittest.TestCase):
    def test_new_installed_target_does_not_inherit_outer_execution_context(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            inherited = {
                'PATH': '/unmounted-outer-tools',
                'DOCKPIPE_SOURCE_ROOT': '/unmounted-outer-project',
                'DOCKPIPE_STEP_CWD': '/unmounted-outer-project',
                'DOCKPIPE_HOST_BASH_BIN': '/unmounted-outer-tools/bash',
                'DOCKPIPE_VAULT_INJECT': '0',
                'LD_PRELOAD': '/unmounted-outer-library.so',
            }
            with patch.dict(os.environ, inherited, clear=True):
                environment = PRODUCT.isolated_environment(output, output / 'installation')
            for name in inherited:
                if name != 'PATH':
                    self.assertNotIn(name, environment)
            self.assertEqual(environment['PATH'], '/usr/bin:/bin')
            self.assertEqual(environment['FLATPAK_USER_DIR'], str(output / 'installation'))
            for name in ['HOME', 'XDG_CONFIG_HOME', 'XDG_DATA_HOME', 'XDG_STATE_HOME', 'XDG_CACHE_HOME']:
                self.assertTrue(Path(environment[name]).is_relative_to(output))

    def test_sdk_failure_case_uses_its_shared_case_key_instead_of_happy_observations(self):
        case = {'id': 'sdk.pipelang', 'scenario': 'failure', 'layer': 'component',
                'required_assertions': ['expected-failure-observed']}
        observations = {'sdk.pipelang': {'pinned-sdk-consumed': (True, 'happy only')},
                        ('sdk.pipelang', 'failure', 'component'): {
                            'expected-failure-observed': (False, 'shared failure not observed')}}
        result = PRODUCT.case_result(case, observations, [], [])
        self.assertEqual(result['status'], 'failed')
        self.assertEqual(result['assertions'][0]['id'], 'expected-failure-observed')

    def test_sdk_inspection_requires_the_exact_deployed_cli_before_namespace_probe(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            deployment = output / 'deployment'
            cli = deployment / 'files/libexec/dockpipe/dockpipe'
            cli.parent.mkdir(parents=True)
            cli.write_bytes(b'another CLI')
            with patch.object(PRODUCT, 'installed_files_namespace_binding') as binding:
                with self.assertRaisesRegex(ValueError, 'differs from the observed installed CLI'):
                    PRODUCT.installed_pipelang_proof(output, {}, {}, deployment, '1' * 64, '2' * 64, [])
                binding.assert_not_called()

    def test_public_empty_list_accepts_omitted_entries_without_accepting_malformed_response(self):
        response = {'schema': 'dockpipe.files/v1', 'path': '.', 'revision': '1' * 64}
        self.assertTrue(PRODUCT.empty_files_list_response(response))
        self.assertTrue(PRODUCT.empty_files_list_response(dict(response, entries=[])))
        for fields in [{'schema': 'another/v1'}, {'path': '../'}, {'revision': 'invalid'},
                       {'entries': [{}]}, {'entries': None}, {'more': True}, {'content': 'unexpected'}]:
            with self.subTest(fields=fields):
                self.assertFalse(PRODUCT.empty_files_list_response(dict(response, **fields)))

    def test_shared_files_fixture_uses_installed_invocation_and_requires_namespace_proof(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            namespace_observed = True
            assertions = {'create-read-write-rename-delete': True,
                          'revision-checks': True, 'workspace-contained': True}

            def shared_qualify(cli, workspace, environment, proof, execute):
                self.assertEqual(cli, '/app/bin/dockpipe')
                self.assertEqual(workspace, proof)
                self.assertTrue(execute(cli, ['files', '--request', '/generated/request.json'],
                                        workspace, environment, proof, 'files-read'))
                return assertions, {'outside_canary_unchanged': True}, []

            def installed_invoke(selected_output, arguments, directory, environment, scope, name, observer):
                self.assertEqual(selected_output, output)
                self.assertEqual(arguments, ['files', '--request', '/generated/request.json'])
                self.assertIn('--unshare=network', observer.command)
                self.assertIn('--command=dockpipe', observer.command)
                logs = scope / name
                logs.mkdir()
                PRODUCT.CONTRACTS.write_json(logs / 'kernel-journal.json', {
                    'contained_cli_observed': namespace_observed, 'owned_processes_closed': True})
                return True

            def namespace_binding(selected_output, environment, proof, observer_module, commit, cli_sha256):
                self.assertEqual(commit, '1' * 64)
                self.assertEqual(cli_sha256, '2' * 64)
                (proof / 'namespace-binding').mkdir()
                return namespace_observed, {}

            with patch.dict(sys.modules, {'workspace_files': SimpleNamespace(qualify=shared_qualify)}):
                with patch.object(PRODUCT, 'installed_fixture_invoke', side_effect=installed_invoke), \
                        patch.object(PRODUCT, 'installed_files_namespace_binding', side_effect=namespace_binding):
                    actual, facts = PRODUCT.installed_workspace_files_proof(output, {}, '1' * 64, '2' * 64)
                    self.assertEqual(actual, assertions)
                    self.assertTrue(facts['outside_canary_unchanged'])
                    self.assertEqual(facts['command_count'], 1)
            with tempfile.TemporaryDirectory() as another:
                output = Path(another)
                namespace_observed = False
                with patch.dict(sys.modules, {'workspace_files': SimpleNamespace(qualify=shared_qualify)}):
                    with patch.object(PRODUCT, 'installed_fixture_invoke', side_effect=installed_invoke), \
                            patch.object(PRODUCT, 'installed_files_namespace_binding', side_effect=namespace_binding):
                        actual, _ = PRODUCT.installed_workspace_files_proof(output, {}, '1' * 64, '2' * 64)
                        self.assertFalse(any(actual.values()))

    def fixture(self, root):
        bundle = root / 'candidate.flatpak'
        bundle.write_bytes(b'candidate')
        source = root / 'source.json'
        source.write_text('{}')
        installation = root / 'installation'
        installation.mkdir(mode=0o700)
        tool = root / 'ostree'
        tool.write_text('#!/bin/sh\nexit 0\n')
        tool.chmod(0o755)
        configuration = {
            'bundle': str(bundle), 'installation': str(installation), 'version': '0.6.38',
            'install_method': 'imported-local-repository', 'source_inventory': str(source),
            'source_inventory_sha256': PRODUCT.CONTRACTS.digest_file(source),
            'repository_tool': {'ostree': str(tool), 'ostree_sha256': PRODUCT.CONTRACTS.digest_file(tool)}}
        return configuration, PRODUCT.CONTRACTS.digest_file(bundle), installation

    def test_candidate_digest_and_existing_app_fail_before_effects(self):
        with tempfile.TemporaryDirectory() as temporary:
            configuration, digest, installation = self.fixture(Path(temporary))
            PRODUCT.admit_configuration(configuration, digest)
            with self.assertRaisesRegex(ValueError, 'bundle changed'):
                PRODUCT.admit_configuration(configuration, '0' * 64)
            (installation / 'app' / PRODUCT.APP_ID).mkdir(parents=True)
            with self.assertRaisesRegex(ValueError, 'replace an existing'):
                PRODUCT.admit_configuration(configuration, digest)

    def test_sdk_inspector_selection_is_exact_before_installation_effects(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            configuration, digest, _ = self.fixture(root)
            inspector = root / 'inspector'
            inspector.write_bytes(b'generated inspector')
            configuration['sdk'] = {'go': str(inspector), 'go_sha256': '0' * 64,
                                    'module': 'example.org/language', 'module_version': 'v1.2.3',
                                    'module_sum': 'h1:generated'}
            with self.assertRaisesRegex(ValueError, 'SDK inspector differs'):
                PRODUCT.admit_configuration(configuration, digest)

    def test_shared_or_symlink_installation_is_refused(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            configuration, digest, installation = self.fixture(root)
            installation.chmod(0o777)
            with self.assertRaisesRegex(ValueError, 'shared write'):
                PRODUCT.admit_configuration(configuration, digest)
            installation.chmod(0o700)
            alias = root / 'alias'
            alias.symlink_to(installation, target_is_directory=True)
            configuration['installation'] = str(alias)
            with self.assertRaisesRegex(ValueError, 'regular task-owned'):
                PRODUCT.admit_configuration(configuration, digest)

    def test_installed_command_uses_physical_cwd_and_denies_host_bridge(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            project = root / 'project with spaces'
            project.mkdir()
            alias = root / 'alias'
            alias.symlink_to(project, target_is_directory=True)
            command = PRODUCT.installed_command(root, 'dockpipe', ['init'], alias)
            self.assertIn('--cwd=' + str(project), command)
            self.assertIn('--no-talk-name=org.freedesktop.Flatpak', command)
            self.assertIn('--unshare=network', command)
            self.assertIn('--nofilesystem=host', command)
            self.assertIn('--nofilesystem=home', command)
            self.assertIn('--nosocket=session-bus', command)
            self.assertIn('--nosocket=system-bus', command)
            self.assertNotIn('--devel', command)
            self.assertEqual(command[-2:], [PRODUCT.APP_ID, 'init'])

    def test_cli_identity_cannot_qualify_normal_gui_startup_and_settings(self):
        requested = {'id': 'install.desktop', 'scenario': 'happy', 'layer': 'installed_e2e',
                     'required_assertions': ['installed-cli-version', 'installed-gui-identity',
                                             'normal-gui-started', 'settings-retained']}
        observations = {'install.desktop': {'installed-cli-version': (True, 'observed'),
                                            'installed-gui-identity': (True, 'observed')}}
        result = PRODUCT.case_result(requested, observations, [], [])
        self.assertEqual(result['status'], 'manual_pending')
        self.assertEqual(len(result['assertions']), 2)
        requested['layer'] = 'guided_ui'
        self.assertEqual(PRODUCT.case_result(requested, observations, [], [])['status'], 'not_run')

    def test_packaged_workflow_requires_real_archive_catalog_locator(self):
        authored = {'display_name': 'Fixture', 'config_path': '/project/workflows/config.yml'}
        packaged = {'display_name': 'Fixture', 'config_path': 'tar:///store/fixture.tar.gz!/config.yml'}
        self.assertEqual(PRODUCT.matching_workflow([authored, packaged], 'Fixture', True), packaged)
        with self.assertRaisesRegex(ValueError, 'Expected one packaged'):
            PRODUCT.matching_workflow([authored], 'Fixture', True)

    def test_normal_gui_keeps_browser_sandbox_and_owned_bus(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            environment = {key: str(root / key) for key in [
                'HOME', 'XDG_CONFIG_HOME', 'XDG_DATA_HOME', 'XDG_STATE_HOME', 'XDG_CACHE_HOME',
                'DISPLAY', 'XAUTHORITY', 'AT_SPI_BUS_ADDRESS']}
            environment['FLATPAK_USER_DIR'] = str(root)
            command = GUI.installed_launch_command(root, root, root, environment, root)
            self.assertIn('--command=dockpipe-launcher', command)
            self.assertIn('--unset-env=QTWEBENGINE_DISABLE_SANDBOX', command)
            self.assertIn('--unset-env=QTWEBENGINE_CHROMIUM_FLAGS', command)
            self.assertIn('--unset-env=QTWEBENGINEPROCESS_PATH', command)
            self.assertIn('--no-talk-name=org.freedesktop.Flatpak', command)
            self.assertIn('--unshare=network', command)
            self.assertNotIn('--devel', command)
            self.assertNotIn('--no-sandbox', command)
            self.assertEqual(command[-2:], [PRODUCT.APP_ID, '--allow-second-instance'])

    def test_native_renderer_paths_are_removed_before_private_services_start(self):
        environment = {
            'QTWEBENGINEPROCESS_PATH': '/native/libexec/QtWebEngineProcess',
            'QTWEBENGINE_RESOURCES_PATH': '/native/resources',
            'QTWEBENGINE_DISABLE_SANDBOX': '1',
            'DOCKPIPE_BIN': '/native/dockpipe',
            'QML2_IMPORT_PATH': '/native/qml',
            'LD_LIBRARY_PATH': '/native/lib',
            'QT_PLUGIN_PATH': '/native/plugins',
            'DISPLAY': ':197',
            'XAUTHORITY': '/private/Xauthority',
        }
        GUI.clear_native_runtime_environment(environment)
        self.assertEqual(environment, {'DISPLAY': ':197', 'XAUTHORITY': '/private/Xauthority'})

    def test_bazzite_profile_requires_observed_bazzite_host_before_install(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            configuration, digest, _ = self.fixture(root)
            request = {'configuration': configuration,
                       'target': {'platform': PRODUCT.BAZZITE_PLATFORM, 'artifact_sha256': digest},
                       'run_id': 'bazzite-check', 'plan_sha256': '1' * 64, 'registry_sha256': '2' * 64,
                       'cases': [{'id': 'init.scopes', 'scenario': 'happy', 'layer': 'installed_e2e',
                                  'required_assertions': ['selected-cwd-initialized']}]}
            output = root / 'output'
            output.mkdir()
            with patch.object(PRODUCT, 'host_identity', return_value={'id': 'ubuntu'}):
                with patch.object(PRODUCT.COMMANDS, 'run_command') as command:
                    result = PRODUCT.execute(request, output)
                    command.assert_not_called()
            self.assertEqual(result['cases'][0]['status'], 'not_run')
            self.assertIn('Requires actual bazzite', result['cases'][0]['reason'])
            self.assertFalse(result['observed_target']['bazzite_qualified'])

    def test_existing_candidate_requires_exact_prior_artifact_readback(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            configuration, digest, installation = self.fixture(root)
            (installation / 'app' / PRODUCT.APP_ID).mkdir(parents=True)
            configuration['install_method'] = 'verified-existing'
            with self.assertRaisesRegex(ValueError, 'exact previous readback'):
                PRODUCT.admit_configuration(configuration, digest)

            proof = {'app_id': PRODUCT.APP_ID, 'branch': 'staging', 'installation': str(installation),
                     'bundle_sha256': digest, 'runtime_commit': PRODUCT.RUNTIME_COMMIT,
                     'source_inventory_sha256': configuration['source_inventory_sha256'],
                     'installed_commit': '1' * 64, 'gui_sha256': '2' * 64, 'cli_sha256': '3' * 64,
                     'browser_sha256': '4' * 64, 'installed_location': str(installation / 'app' / PRODUCT.APP_ID)}
            path = root / 'prior.json'
            PRODUCT.CONTRACTS.write_json(path, proof)
            configuration['installation_proof'] = {'path': str(path), 'sha256': PRODUCT.CONTRACTS.digest_file(path)}
            PRODUCT.admit_configuration(configuration, digest)
            path.write_text('{}')
            with self.assertRaisesRegex(ValueError, 'readback changed'):
                PRODUCT.admit_configuration(configuration, digest)

    def test_native_observer_abort_remains_an_owned_child_failure(self):
        with tempfile.TemporaryDirectory() as temporary:
            log = Path(temporary) / 'observer.log'
            command = [sys.executable, '-c',
                       'import os, resource, signal\nresource.setrlimit(resource.RLIMIT_CORE, (0, 0))\n'
                       'os.kill(os.getpid(), signal.SIGABRT)\n']
            parent = os.getpid()
            with self.assertRaisesRegex(RuntimeError, 'exited -6'):
                GUI.supervise_observer(command, os.environ.copy(), log, 5)
            self.assertEqual(os.getpid(), parent)

    def test_upgrade_requires_distinct_private_metadata_and_complete_baseline(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            configuration, digest, installation = self.fixture(root)
            (installation / 'app' / PRODUCT.APP_ID).mkdir(parents=True)
            (installation / 'repo').mkdir()
            (installation / 'repo/config').write_text('clone')
            baseline = root / 'baseline'
            (baseline / 'repo').mkdir(parents=True)
            baseline.chmod(0o700)
            (baseline / 'repo/config').write_text('baseline')
            configuration.update(install_method='upgrade-private', upgrade_source_installation=str(baseline))
            proof = {'app_id': PRODUCT.APP_ID, 'branch': 'staging', 'installation': str(baseline),
                     'bundle_sha256': '5' * 64, 'runtime_commit': PRODUCT.RUNTIME_COMMIT,
                     'source_inventory_sha256': '6' * 64, 'installed_commit': '1' * 64,
                     'gui_sha256': '2' * 64, 'cli_sha256': '3' * 64, 'browser_sha256': '4' * 64,
                     'ui_library_sha256': '7' * 64,
                     'installed_location': str(baseline / 'app' / PRODUCT.APP_ID)}
            path = root / 'baseline-proof.json'

            def write_proof():
                path.unlink(missing_ok=True)
                PRODUCT.CONTRACTS.write_json(path, proof)
                configuration['installation_proof'] = {'path': str(path), 'sha256': PRODUCT.CONTRACTS.digest_file(path)}

            write_proof()
            PRODUCT.admit_configuration(configuration, digest)
            proof['bundle_sha256'] = digest
            write_proof()
            with self.assertRaisesRegex(ValueError, 'distinct immutable'):
                PRODUCT.admit_configuration(configuration, digest)
            proof['bundle_sha256'] = '5' * 64
            del proof['ui_library_sha256']
            write_proof()
            with self.assertRaisesRegex(ValueError, 'exact UI library'):
                PRODUCT.admit_configuration(configuration, digest)
            proof['ui_library_sha256'] = '7' * 64
            write_proof()
            (installation / 'repo/config').unlink()
            os.link(baseline / 'repo/config', installation / 'repo/config')
            with self.assertRaisesRegex(ValueError, 'mutable repository metadata'):
                PRODUCT.admit_configuration(configuration, digest)

    def test_installed_cache_fixture_executes_only_the_installed_cli_adapter(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            commands = []

            def run(command, logs, timeout, environment, directory, observe=None):
                self.assertIsNone(observe)
                commands.append(command)
                (logs / 'driver.stdout').write_text('{}')
                (logs / 'driver.stderr').write_text('')
                return {'exit_code': 1, 'failure': ''}

            with patch.object(PRODUCT, 'installed_command', wraps=PRODUCT.installed_command):
                scripts = str(PRODUCT.ROOT / 'workflows/qualify/product/assets/scripts')
                with patch.object(sys, 'path', [scripts, *sys.path]):
                    with patch('process.run_driver', side_effect=run):
                        with self.assertRaisesRegex(RuntimeError, 'could not initialize'):
                            PRODUCT.installed_source_cache_proof(output, {})
            self.assertEqual(len(commands), 1)
            self.assertIn('--command=dockpipe', commands[0])
            self.assertEqual(commands[0][-2:], [PRODUCT.APP_ID, 'init'])

    def test_installed_library_soname_resolves_only_within_the_deployment(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            installation = root / 'installation'
            location = installation / 'app/deployment'
            files = location / 'files'
            files.mkdir(parents=True)
            library = files / 'libui.so.1'
            library.write_bytes(b'qualified ELF')
            soname = files / 'libui.so'
            soname.symlink_to(library.name)

            def run(arguments, label, timeout):
                return str(location) if '--show-location' in arguments else '1' * 64

            with patch.object(PRODUCT, 'INSTALLED_ELFS', {'ui_library': 'libui.so'}):
                observed = PRODUCT.inspect_installed(run, installation)
                self.assertEqual(observed['ui_library_sha256'], PRODUCT.CONTRACTS.digest_file(library))
                escaped = root / 'outside.so'
                escaped.write_bytes(b'outside')
                soname.unlink()
                soname.symlink_to(escaped)
                with self.assertRaisesRegex(ValueError, 'ELF escaped'):
                    PRODUCT.inspect_installed(run, installation)

    def test_retained_profile_admits_exact_copied_inputs_and_refuses_alias_or_drift(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            home = root / 'retained-home'
            home.mkdir(mode=0o700)
            workspace = root / 'retained-workspace'
            workspace.mkdir(mode=0o700)
            settings = home / '.var/app' / PRODUCT.APP_ID / 'config/dockpipe/dockpipe-launcher/launcher.json'
            settings.parent.mkdir(parents=True)
            settings.write_text(json.dumps({'defaultDashboardVisible': True, 'projectFolder': str(workspace)}))
            workflow = workspace / 'workflows/authored-fixture/config.yml'
            workflow.parent.mkdir(parents=True)
            workflow.write_text('# previously saved in actual A GUI\n')
            selected = {'home': str(home), 'stage': 'prepare-a',
                        'settings_sha256': GUI.digest(settings), 'workflow_sha256': GUI.digest(workflow),
                        'workspace_access': 'read-only-original'}
            self.assertEqual(GUI.admit_retained_profile(selected, workspace), home)
            alias = root / 'workspace-alias'
            alias.symlink_to(workspace, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, 'private owned'):
                GUI.admit_retained_profile(selected, alias)
            settings.write_text('{"defaultDashboardVisible":false}')
            with self.assertRaisesRegex(ValueError, 'input changed'):
                GUI.admit_retained_profile(selected, workspace)
            settings.write_text(json.dumps({'defaultDashboardVisible': True, 'projectFolder': '/another/project'}))
            selected['settings_sha256'] = GUI.digest(settings)
            with self.assertRaisesRegex(ValueError, 'exact protected original project'):
                GUI.admit_retained_profile(selected, workspace)

    def test_retained_profile_launch_grants_the_saved_workspace_read_only(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            environment = {key: str(root / key) for key in [
                'HOME', 'XDG_CONFIG_HOME', 'XDG_DATA_HOME', 'XDG_STATE_HOME', 'XDG_CACHE_HOME',
                'DISPLAY', 'XAUTHORITY', 'AT_SPI_BUS_ADDRESS']}
            environment['FLATPAK_USER_DIR'] = str(root)
            workspace = root / 'protected project'
            workspace.mkdir()
            command = GUI.installed_launch_command(root, root, workspace, environment, root,
                                                  workspace_read_only=True)
            self.assertIn('--filesystem=' + str(workspace) + ':ro', command)
            self.assertNotIn('--filesystem=' + str(workspace), command)

    def test_retained_profile_observations_refuse_generated_editor_and_path_typing(self):
        workspace = Path('/private/original-A/workspace')
        for value in [str(workspace), '/private/another/workspace', '# guided integration fixture\n']:
            with self.subTest(value=value):
                with self.assertRaisesRegex(ValueError, 'exact generated'):
                    GUI.validate_guided_text(value, workspace, {'stage': 'prepare-a'})

    def test_copied_profile_inventory_accepts_internal_cache_link_and_refuses_escape(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            profile = root / 'profile'
            profile.mkdir(mode=0o700)
            (profile / 'runtime-cache').write_bytes(b'generated cache')
            link = profile / 'active'
            link.symlink_to('runtime-cache')
            records = MIGRATION.tree_inventory(profile)
            self.assertEqual(len(records), 2)
            link.unlink()
            outside = root / 'outside'
            outside.write_bytes(b'outside')
            link.symlink_to('../outside')
            with self.assertRaisesRegex(ValueError, 'escaped'):
                MIGRATION.tree_inventory(profile)



if __name__ == '__main__':
    unittest.main()
