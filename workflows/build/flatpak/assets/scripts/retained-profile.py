#!/usr/bin/env python3
"""Opt-in migration of a copied, previously qualified generated Flatpak GUI profile."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[4]


def module(name, filename):
    specification = importlib.util.spec_from_file_location(name, HERE / filename)
    value = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(value)
    return value


PRODUCT = module('retained_product', 'verify-installed-product.py')
GUI = module('retained_gui', 'installed-product-gui.py')
CONTRACTS = PRODUCT.CONTRACTS


def tree_inventory(root):
    root = Path(root)
    if (not root.is_absolute() or root.is_symlink() or not root.is_dir()
            or root.stat().st_uid != os.getuid() or root.stat().st_mode & 0o022):
        raise ValueError('Select a private owned generated fixture directory')
    records = []
    total = 0
    for index, path in enumerate(root.rglob('*')):
        if index >= 4096:
            raise ValueError('Generated profile/workspace exceeds its file-count bound')
        relative = str(path.relative_to(root))
        if path.is_symlink():
            target = os.readlink(path)
            if Path(target).is_absolute() or not path.resolve(strict=True).is_relative_to(root):
                raise ValueError('Copied fixture link escaped its generated root')
            records.append({'path': relative, 'kind': 'symlink', 'target': target})
        elif path.is_file():
            total += path.stat().st_size
            if total > 64 << 20:
                raise ValueError('Generated profile/workspace exceeds its 64MiB bound')
            records.append({'path': relative, 'kind': 'file', 'sha256': CONTRACTS.digest_file(path)})
        elif not path.is_dir():
            raise ValueError('Generated fixture contains an unsupported filesystem object')
    return sorted(records, key=lambda record: record['path'])


def selected_json(selection):
    if not isinstance(selection, dict) or set(selection) != {'path', 'sha256'}:
        raise ValueError('Select an exact previous generated qualification receipt')
    path = PRODUCT.regular(selection['path'], 4 << 20)
    if CONTRACTS.digest_file(path) != selection['sha256']:
        raise ValueError('Selected previous qualification receipt changed')
    return path, CONTRACTS.read_json(path)


def admit(request):
    if (not isinstance(request, dict) or set(request) != {'schema', 'run_id', 'baseline', 'baseline_gui',
            'bundle', 'bundle_sha256', 'version', 'source_inventory', 'source_inventory_sha256', 'repository_tool', 'gui_tools', 'workspace_access'}
            or request['schema'] != 'dockpipe.flatpak.retained-profile/v1'
            or not CONTRACTS.matches(CONTRACTS.IDENTIFIER, request['run_id'])):
        raise ValueError('Invalid retained-profile migration selection')
    proof_path, baseline = selected_json(request['baseline'])
    gui_path, observed = selected_json(request['baseline_gui'])
    source_workspace = Path(baseline['fixture_workspace'])
    source_home = gui_path.parent / 'gui-home'
    required_gui = ['normal_gui_started', 'settings_retained', 'selected_project_workflows_visible',
                    'project_reopen', 'gui_exit_normal']
    if source_workspace.parent != gui_path.parent or not all(observed.get(key) is True for key in required_gui):
        raise ValueError('Select the same previously qualified generated GUI invocation')
    if baseline.get('app_id') != PRODUCT.APP_ID or not baseline.get('private_candidate'):
        raise ValueError('Retained profile must come from an explicitly qualified private Dockpipe artifact')
    profile = source_home / '.var/app' / PRODUCT.APP_ID / 'config/dockpipe/dockpipe-launcher/launcher.json'
    workflow = source_workspace / 'workflows/authored-fixture/config.yml'
    expected = observed['independent_readback']
    if (CONTRACTS.digest_file(profile) != expected['settings_sha256']
            or CONTRACTS.digest_file(workflow) != expected['workflow_sha256']):
        raise ValueError('Previously observed generated profile/workflow changed')
    if json.loads(profile.read_text()).get('projectFolder') != str(source_workspace):
        raise ValueError('The retained profile must still select its original generated workspace')
    for name in ['bundle', 'source_inventory']:
        if CONTRACTS.digest_file(PRODUCT.regular(request[name])) != request[name + '_sha256']:
            raise ValueError('Migration target input changed: ' + name)
    if request['workspace_access'] != 'read-only-original' or request['gui_tools'].get('method') != 'guided':
        raise ValueError('Select explicit guided observations with the original generated workspace read-only')
    return proof_path, baseline, source_home, source_workspace


def pin_helpers():
    paths = [Path(__file__), HERE / 'installed-product-gui.py', HERE / 'verify-installed-product.py',
             HERE / 'verify-installed-host.py']
    product = ROOT / 'workflows/qualify/product'
    paths.extend(product / name for name in ['registry.json', 'assets/scripts/contracts.py',
        'assets/scripts/process.py', 'drivers/portable.py', 'drivers/workspace_files.py',
        'platforms/linux/desktop.py', 'platforms/linux/ui.py'])
    return [{'path': str(path), 'sha256': CONTRACTS.digest_file(path)} for path in paths]


def run(request, output, request_sha256):
    proof_path, baseline, source_home, source_workspace = admit(request)
    output.mkdir(mode=0o700)
    original = {'home': tree_inventory(source_home), 'workspace': tree_inventory(source_workspace)}
    helpers = pin_helpers()
    CONTRACTS.write_json(output / 'input-inventory.json', {'sources': original, 'helpers': helpers})
    home = output / 'retained-home'
    workspace_snapshot = output / 'retained-workspace-snapshot'
    workspace = source_workspace
    shutil.copytree(source_home, home, symlinks=True)
    shutil.copytree(source_workspace, workspace_snapshot, symlinks=True)
    home.chmod(0o700)
    workspace_snapshot.chmod(0o700)
    if tree_inventory(home) != original['home'] or tree_inventory(workspace_snapshot) != original['workspace']:
        raise ValueError('Copied retained fixture content differs before normal A launch')
    installation = output / 'installation'
    environment = {'PATH': '/usr/bin:/bin', 'HOME': str(output), 'FLATPAK_USER_DIR': str(installation),
                   'PYTHONDONTWRITEBYTECODE': '1'}
    PRODUCT.COMMANDS.run_command(['cp', '-a', '--reflink=auto', baseline['installation'], str(installation)],
                                 environment, output / 'copy-installation.log', 180)
    count = 0
    for previous in Path(baseline['installation']).rglob('*'):
        if previous.is_symlink() or not previous.is_file():
            continue
        count += 1
        if count > 1_000_000:
            raise ValueError('Private installation file count exceeded its bound')
        current = installation / previous.relative_to(baseline['installation'])
        if (previous.stat().st_dev, previous.stat().st_ino) == (current.stat().st_dev, current.stat().st_ino):
            raise ValueError('Private installation copy shares a baseline file inode')
    configuration = {key: request[key] for key in ['bundle', 'source_inventory', 'source_inventory_sha256', 'repository_tool', 'gui_tools']}
    configuration.update(installation=str(installation), version=request['version'], install_method='upgrade-private',
                         installation_proof=request['baseline'], upgrade_source_installation=baseline['installation'])
    PRODUCT.admit_configuration(configuration, request['bundle_sha256'])
    def initial_readback(arguments, label, timeout):
        return PRODUCT.COMMANDS.run_command(arguments, environment, output / (label + '.log'), timeout)
    before = PRODUCT.inspect_installed(initial_readback, installation, 'baseline-')
    for name in ['installed_commit', *[key + '_sha256' for key in PRODUCT.INSTALLED_ELFS]]:
        if before[name] != baseline[name]:
            raise ValueError('Copied A deployment identity differs: ' + name)
    runtime = initial_readback(['flatpak', 'info', '--user', '--show-commit', PRODUCT.RUNTIME], 'baseline-runtime', 30).strip()
    if runtime != PRODUCT.RUNTIME_COMMIT:
        raise ValueError('Copied A runtime differs from the pinned Platform')
    settings = home / '.var/app' / PRODUCT.APP_ID / 'config/dockpipe/dockpipe-launcher/launcher.json'
    workflow = workspace / 'workflows/authored-fixture/config.yml'

    def gui_stage(name, commit, executable_sha256):
        stage = output / name
        stage.mkdir(mode=0o700)
        selected = {'output': str(stage), 'installation': str(installation), 'workspace': str(workspace),
                    'xvfb': request['gui_tools']['xvfb'], 'expected_commit': commit,
                    'expected_gui_sha256': executable_sha256, 'method': 'guided',
                    'retained_profile': {'home': str(home), 'stage': name,
                        'settings_sha256': CONTRACTS.digest_file(settings),
                        'workflow_sha256': CONTRACTS.digest_file(workflow),
                        'workspace_access': 'read-only-original'}}
        CONTRACTS.write_json(stage / 'request.json', selected)
        PRODUCT.COMMANDS.run_command(['/usr/bin/python3', str(HERE / 'installed-product-gui.py'),
                                     '--request', str(stage / 'request.json')], environment, stage / 'supervisor.log', 840)
        facts = CONTRACTS.read_json(stage / 'gui-observed.json')
        required = ['normal_gui_started', 'settings_retained', 'selected_project_workflows_visible',
                    'project_reopen', 'gui_exit_normal', 'runtime_directory_cleaned']
        if (not all(facts.get(key) is True for key in required)
                or not facts['retained_profile']['authored_workflow_unchanged']):
            raise RuntimeError('Retained-profile normal GUI observations failed: ' + name)
        return facts

    prepared = gui_stage('prepare-a', baseline['installed_commit'], baseline['gui_sha256'])
    before_upgrade = CONTRACTS.digest_file(settings)
    registry = ROOT / 'workflows/qualify/product/registry.json'
    catalog = CONTRACTS.read_json(registry)
    cases = []
    for feature in catalog['features']:
        if feature['id'] in ['install.cli', 'init.scopes', 'workflow.discovery']:
            for case in feature['cases']:
                if case['scenario'] == 'happy' and case['layer'] == 'installed_e2e':
                    cases.append({'id': feature['id'], **{key: case[key] for key in ['scenario', 'layer', 'required_assertions']}})
    configuration.pop('gui_tools')
    upgrade_request = {'schema': 'dockpipe.product-integration.request/v1', 'run_id': request['run_id'],
                       'plan_sha256': request_sha256, 'registry_sha256': CONTRACTS.digest_file(registry),
                       'target': {'id': 'retained-profile', 'platform': PRODUCT.PLATFORM, 'kind': 'flatpak',
                                  'artifact_sha256': request['bundle_sha256']},
                       'configuration': configuration, 'cases': cases}
    CONTRACTS.write_json(output / 'upgrade-request.json', upgrade_request)
    PRODUCT.COMMANDS.run_command(['/usr/bin/python3', str(HERE / 'verify-installed-product.py'), '--request',
        str(output / 'upgrade-request.json'), '--output', str(output / 'upgrade')], environment, output / 'upgrade.log', 300)
    upgrade = CONTRACTS.read_json(output / 'upgrade/result.json')
    if len(upgrade['cases']) != 3 or any(case['status'] != 'passed' for case in upgrade['cases']):
        raise RuntimeError('The actual private artifact upgrade and CLI baseline did not pass')
    if CONTRACTS.digest_file(settings) != before_upgrade:
        raise ValueError('The GUI profile changed during the artifact-only upgrade')
    target = upgrade['observed_target']
    verified = gui_stage('verify-b', target['installed_commit'], target['gui_sha256'])
    unchanged = original == {'home': tree_inventory(source_home), 'workspace': tree_inventory(source_workspace)}
    if not unchanged or helpers != pin_helpers():
        raise ValueError('Original retained fixture or source-pinned helper changed')
    result = {'schema': 'dockpipe.flatpak.retained-profile-result/v1', 'run_id': request['run_id'],
              'request_sha256': request_sha256, 'passed': True, 'coverage_credit': [],
              'project_path_preserved': str(source_workspace), 'workspace_access': 'read-only-original',
              'workspace_snapshot_used_for_gui': False,
              'seed_profile_used': False, 'original_generated_fixture_unchanged': unchanged,
              'independent_installation_files': count, 'profile_sha256_before_upgrade': before_upgrade,
              'profile_sha256_after_upgrade': before_upgrade, 'upgrade': target['upgrade'],
              'prepare_a': prepared, 'verify_b': verified, 'helpers': helpers,
              'limitations': ['Generated task profile only; no real user profile, remote, VM, Bazzite or portal migration proof.']}
    CONTRACTS.write_json(output / 'result.json', result)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--request', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    arguments = parser.parse_args()
    request_path = PRODUCT.regular(arguments.request, 4 << 20)
    if not arguments.output.is_absolute() or arguments.output.exists():
        parser.error('Select a fresh absolute private migration output')
    def interrupted(_number, _frame):
        raise KeyboardInterrupt
    previous = signal.signal(signal.SIGTERM, interrupted)
    try:
        result = run(CONTRACTS.read_json(request_path), arguments.output, CONTRACTS.digest_file(request_path))
        print(json.dumps({'passed': result['passed'], 'coverage_credit': result['coverage_credit']}))
    finally:
        signal.signal(signal.SIGTERM, previous)


if __name__ == '__main__':
    main()
