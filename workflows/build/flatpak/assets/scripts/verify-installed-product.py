#!/usr/bin/env python3
"""Opt-in installed Flatpak CLI/project regressions and normal-GUI handoff.

Run only against an explicitly selected private installation. GUI startup and
persistence remain pending until the independent normal-GUI driver observes them.
This lane never opens a remote desktop, rewrites a host shortcut or runs in CI.
"""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import pwd
import re
import subprocess
import sys
import tarfile
import time

ROOT = Path(__file__).resolve().parents[5]
SPEC = importlib.util.spec_from_file_location('flatpak_product_contracts', ROOT / 'workflows/qualify/product/assets/scripts/contracts.py')
CONTRACTS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CONTRACTS)
SPEC = importlib.util.spec_from_file_location('flatpak_product_commands', Path(__file__).with_name('verify-installed-host.py'))
COMMANDS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(COMMANDS)
APP_ID = 'com.dockpipe.Dockpipe'
PLATFORM = 'linux-amd64-flatpak-kde6.10'
BAZZITE_PLATFORM = 'linux-amd64-flatpak-bazzite-kde6.10'
RUNTIME = 'runtime/org.kde.Platform/x86_64/6.10'
RUNTIME_COMMIT = '31742ff0f905913c512ea732960bbdeffa8306dcbebca45eeee62bfed0ca03a3'
SUPPORTED = {'install.desktop', 'install.cli', 'init.scopes', 'workflow.discovery', 'browser.local', 'files.workspace', 'sdk.pipelang'}
INSTALLED_ELFS = {'gui': 'libexec/dockpipe/dockpipe-launcher', 'cli': 'libexec/dockpipe/dockpipe',
                  'browser': 'lib/libSpatialWebBrowser.so.0.1.0',
                  'ui_library': 'libexec/dockpipe/libdockpipe-launcher-ui.so'}


def regular(path, maximum=1 << 30):
    path = Path(path)
    if not path.is_absolute() or path.is_symlink() or not path.is_file() or path.stat().st_size > maximum:
        raise ValueError('Select a bounded absolute regular input file')
    return path


def load_request(path):
    request = CONTRACTS.read_json(regular(path, 4 << 20))
    expected = {'schema', 'run_id', 'plan_sha256', 'registry_sha256', 'target', 'configuration', 'cases'}
    if set(request) != expected or request['schema'] != 'dockpipe.product-integration.request/v1':
        raise ValueError('Invalid installed Flatpak lane request')
    if not CONTRACTS.matches(CONTRACTS.IDENTIFIER, request['run_id']):
        raise ValueError('Invalid local run identity')
    if any(not CONTRACTS.matches(CONTRACTS.DIGEST, request[key]) for key in ('plan_sha256', 'registry_sha256')):
        raise ValueError('Run plan and registry must have exact digest bindings')
    target = request['target']
    if (set(target) != {'id', 'platform', 'kind', 'artifact_sha256'} or target['platform'] not in (PLATFORM, BAZZITE_PLATFORM)
            or target['kind'] != 'flatpak' or not CONTRACTS.matches(CONTRACTS.DIGEST, target['artifact_sha256'])):
        raise ValueError('This driver accepts only an explicitly selected installed amd64 KDE6.10 Flatpak')
    if not isinstance(request['cases'], list) or len(request['cases']) > 32:
        raise ValueError('Case count exceeds its bound')
    seen = set()
    for case in request['cases']:
        key = CONTRACTS.case_key(case)
        if key in seen or not isinstance(case.get('required_assertions'), list) or len(case['required_assertions']) > 32:
            raise ValueError('Invalid or duplicated requested case')
        seen.add(key)
    return request


def admit_configuration(configuration, artifact_sha256):
    required = {'bundle', 'installation', 'version', 'install_method', 'source_inventory', 'source_inventory_sha256'}
    if (not isinstance(configuration, dict) or not required.issubset(configuration)
            or set(configuration) - required - {'require_host_id', 'gui_tools', 'repository_tool', 'installation_proof',
                                                'upgrade_source_installation', 'sdk'}):
        raise ValueError('Select the exact bundle, isolated installation, version, method and source inventory')
    bundle = regular(configuration['bundle'])
    if CONTRACTS.digest_file(bundle, 1 << 30) != artifact_sha256:
        raise ValueError('Selected installed product bundle changed')
    source = regular(configuration['source_inventory'], 4 << 20)
    if CONTRACTS.digest_file(source) != configuration['source_inventory_sha256']:
        raise ValueError('Selected source inventory changed')
    if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', configuration['version']):
        raise ValueError('An exact numeric private candidate version is required')
    if 'sdk' in configuration:
        sdk = configuration['sdk']
        if (not isinstance(sdk, dict) or set(sdk) != {'go', 'go_sha256', 'module', 'module_version', 'module_sum'}
                or not all(isinstance(value, str) and value for value in sdk.values())):
            raise ValueError('Select the exact SDK module and pinned Go build-info inspector')
        if CONTRACTS.digest_file(regular(sdk['go'])) != sdk['go_sha256']:
            raise ValueError('Selected SDK inspector differs from its pin')
    if configuration['install_method'] not in ('bundle', 'imported-local-repository', 'verified-existing', 'upgrade-private'):
        raise ValueError('Choose the installation method explicitly; failures are never silently retried')
    if configuration['install_method'] in ('imported-local-repository', 'upgrade-private'):
        tool = configuration.get('repository_tool')
        if not isinstance(tool, dict) or set(tool) != {'ostree', 'ostree_sha256'}:
            raise ValueError('Select the exact OSTree executable for local repository import')
        if CONTRACTS.digest_file(regular(tool['ostree'])) != tool['ostree_sha256']:
            raise ValueError('Local repository executable changed')
    if 'gui_tools' in configuration:
        tools = configuration['gui_tools']
        required_tools = {'xvfb', 'xvfb_sha256'}
        if (not isinstance(tools, dict) or not required_tools.issubset(tools)
                or set(tools) - required_tools - {'method', 'feature'}
                or tools.get('method', 'atspi') not in ['atspi', 'guided']
                or tools.get('feature', 'workflows') not in ['workflows', 'browser']
                or (tools.get('feature') == 'browser' and tools.get('method') != 'guided')):
            raise ValueError('Select the exact private-display executable and digest')
        if CONTRACTS.digest_file(regular(tools['xvfb'])) != tools['xvfb_sha256']:
            raise ValueError('Private display executable changed')
    installation = Path(configuration['installation'])
    if not installation.is_absolute() or installation.is_symlink() or not installation.is_dir():
        raise ValueError('Select an existing regular task-owned user installation')
    canonical = installation.resolve(strict=True)
    default = (Path(pwd.getpwuid(os.getuid()).pw_dir) / '.local/share/flatpak').resolve()
    if canonical == default or canonical == Path('/var/lib/flatpak'):
        raise ValueError('This qualification must not modify a default Flatpak installation')
    if installation.stat().st_uid != os.getuid() or installation.stat().st_mode & 0o022:
        raise ValueError('The private installation must be owned by this user without shared write access')
    candidate = installation / 'app' / APP_ID
    if configuration['install_method'] in ('verified-existing', 'upgrade-private'):
        upgrading = configuration['install_method'] == 'upgrade-private'
        prior_installation(configuration, artifact_sha256, upgrading)
        if candidate.is_symlink() or not candidate.is_dir():
            raise ValueError('The previously observed private app installation is missing')
        if upgrading:
            origin = Path(configuration['upgrade_source_installation'])
            if not origin.is_absolute() or origin.is_symlink() or not origin.is_dir() or origin.resolve() == canonical:
                raise ValueError('Upgrade requires a separate clone of the private baseline installation')
            if origin.stat().st_uid != os.getuid() or origin.stat().st_mode & 0o022:
                raise ValueError('The baseline installation must remain privately owned')
            for relative in ['repo/config']:
                previous = (origin / relative).stat()
                current = (installation / relative).stat()
                if (previous.st_dev, previous.st_ino) == (current.st_dev, current.st_ino):
                    raise ValueError('Upgrade clone must not share mutable repository metadata with the baseline')
    elif candidate.exists() or candidate.is_symlink():
        raise ValueError('Refusing to replace an existing Dockpipe app')
    return bundle, installation, source


def prior_installation(configuration, artifact_sha256, upgrading=False):
    selected = configuration.get('installation_proof')
    if not isinstance(selected, dict) or set(selected) != {'path', 'sha256'}:
        raise ValueError('Continuing an installed candidate requires its exact previous readback')
    path = regular(selected['path'], 4 << 20)
    if CONTRACTS.digest_file(path) != selected['sha256']:
        raise ValueError('Previous installation readback changed')
    proof = CONTRACTS.read_json(path)
    if not isinstance(proof, dict):
        raise ValueError('Previous installation readback must be an object')
    expected = {'app_id': APP_ID, 'branch': 'staging', 'runtime_commit': RUNTIME_COMMIT,
                'installation': configuration.get('upgrade_source_installation') if upgrading else configuration['installation']}
    if not upgrading:
        expected.update(bundle_sha256=artifact_sha256, source_inventory_sha256=configuration['source_inventory_sha256'])
    if any(proof.get(name) != value for name, value in expected.items()):
        raise ValueError('Previous installation readback belongs to another candidate')
    for name in ['installed_commit', 'gui_sha256', 'cli_sha256', 'browser_sha256', 'bundle_sha256', 'source_inventory_sha256']:
        if not CONTRACTS.matches(CONTRACTS.DIGEST, proof.get(name)):
            raise ValueError('Previous installation readback lacks exact artifact identity')
    if upgrading and not CONTRACTS.matches(CONTRACTS.DIGEST, proof.get('ui_library_sha256')):
        raise ValueError('Upgrade baseline requires its exact UI library identity')
    location = proof.get('installed_location')
    if (not isinstance(location, str) or not Path(location).is_absolute()
            or not Path(location).is_relative_to(Path(proof['installation']))):
        raise ValueError('Previous installed location escaped the private installation')
    if upgrading and proof['bundle_sha256'] == artifact_sha256:
        raise ValueError('Upgrade target must be a distinct immutable candidate')
    return proof


def inspect_installed(run, installation, label=''):
    location = Path(run(['flatpak', 'info', '--user', '--show-location', APP_ID], label + 'installed-location', 30).strip())
    if not location.resolve(strict=True).is_relative_to(installation.resolve(strict=True)):
        raise ValueError('Installed app location escaped the selected task installation')
    result = {'installed_location': str(location),
              'installed_commit': run(['flatpak', 'info', '--user', '--show-commit', APP_ID], label + 'installed-commit', 30).strip()}
    installed = location / 'files'
    for name, relative in INSTALLED_ELFS.items():
        path = (installed / relative).resolve(strict=True)
        if not path.is_relative_to(installed.resolve(strict=True)):
            raise ValueError('Installed ELF escaped the selected app deployment')
        result[name + '_sha256'] = CONTRACTS.digest_file(path)
    return result


def host_identity():
    values = {}
    for line in Path('/etc/os-release').read_text().splitlines():
        if '=' in line:
            name, value = line.split('=', 1)
            values[name] = value.strip('"')
    return {'id': values.get('ID', ''), 'version': values.get('VERSION_ID', ''),
            'ostree_booted': Path('/run/ostree-booted').exists(),
            'selinux_present': Path('/sys/fs/selinux/enforce').exists()}


def isolated_environment(output, installation):
    # This is a new installed target, with its own execution scopes and tools.
    # An outer Dockpipe workflow's paths may not exist in the Flatpak namespace.
    environment = {'PATH': '/usr/bin:/bin', 'LANG': 'C.UTF-8', 'LC_ALL': 'C.UTF-8'}
    environment['FLATPAK_USER_DIR'] = str(installation)
    environment['FLATPAK_GL_DRIVERS'] = 'default'
    for key, directory in [('HOME', 'home'), ('XDG_CONFIG_HOME', 'config'), ('XDG_DATA_HOME', 'data'),
                           ('XDG_STATE_HOME', 'state'), ('XDG_CACHE_HOME', 'cache')]:
        path = output / directory
        path.mkdir(mode=0o700)
        environment[key] = str(path)
    return environment


def installed_command(output, command, arguments=(), cwd=None):
    selected = Path(cwd).resolve(strict=True) if cwd else output
    return ['flatpak', 'run', '--user', '--branch=staging', '--unshare=network',
            '--no-documents-portal', '--no-a11y-bus', '--nofilesystem=host',
            '--nofilesystem=home', '--filesystem=' + str(output), '--cwd=' + str(selected),
            '--nosocket=x11', '--nosocket=wayland', '--no-talk-name=org.freedesktop.Flatpak',
            '--nosocket=session-bus', '--nosocket=system-bus',
            '--env=HOME=' + str(output / 'home'), '--env=XDG_CONFIG_HOME=' + str(output / 'config'),
            '--env=XDG_DATA_HOME=' + str(output / 'data'), '--env=XDG_STATE_HOME=' + str(output / 'state'),
            '--env=XDG_CACHE_HOME=' + str(output / 'cache'), '--env=DOCKPIPE_BIN=/app/bin/dockpipe',
            '--env=DOCKPIPE_SYSTEM_ROOT=/app/share/dockpipe', '--env=LD_LIBRARY_PATH=',
            '--unset-env=LD_PRELOAD', '--unset-env=LD_AUDIT', '--unset-env=QML_IMPORT_PATH',
            '--unset-env=QML2_IMPORT_PATH', '--unset-env=QTWEBENGINE_CHROMIUM_FLAGS',
            '--unset-env=QTWEBENGINE_DISABLE_SANDBOX',
            '--unset-env=DOCKPIPE_PACKAGES_ROOT', '--unset-env=DOCKPIPE_REPO_ROOT',
            '--unset-env=QT_PLUGIN_PATH', '--unset-env=QT_QPA_PLATFORM_PLUGIN_PATH',
            '--env=QT_QPA_PLATFORM=offscreen', '--command=' + command, APP_ID, *arguments]


def workflow_fixture(directory, name, title, namespace=None):
    (directory / 'models').mkdir(parents=True)
    (directory / 'assets').mkdir()
    text = ('name: ' + name + '\ntitle: ' + title + '\ndocker_preflight: false\nplatforms: [flatpak]\n'
            'types: [models/FixtureConfig.pipe]\nvars:\n  DOCKPIPE_FIXTURE_MARKER: product-fixture\n'
            'steps:\n  - id: host-fixture\n    kind: host\n    cwd: repo\n'
            '    cmd: test -f /.flatpak-info && printf "%s" "$DOCKPIPE_FIXTURE_MARKER" > product-workflow-result.txt\n')
    if namespace:
        text = 'namespace: ' + namespace + '\n' + text
    (directory / 'config.yml').write_text(text)
    (directory / 'models/FixtureConfig.pipe').write_text(
        'public Class FixtureConfig\n{\n    [EnvName = "DOCKPIPE_FIXTURE_MARKER"]\n'
        '    [DisplayName = "Fixture Marker"]\n    public string Marker = "product-fixture";\n}\n')
    (directory / 'assets/run.sh').write_text(
        '#!/usr/bin/env bash\nset -euo pipefail\n'
        'test -f /.flatpak-info\nprintf "%s" "$DOCKPIPE_FIXTURE_MARKER" > product-workflow-result.txt\n')
    (directory / 'assets/run.sh').chmod(0o755)


def matching_workflow(catalog, title, packaged):
    matches = [item for item in catalog if item.get('display_name') == title
               and item.get('config_path', '').startswith('tar://') == packaged]
    if len(matches) != 1:
        raise ValueError('Expected one ' + ('packaged' if packaged else 'authored') + ' workflow: ' + title)
    return matches[0]


def installed_fixture_invoke(output, arguments, directory, environment, scope, name, observer=None):
    """Run a shared black-box fixture through the installed public CLI launcher."""
    product = ROOT / 'workflows/qualify/product'
    sys.path.insert(0, str(product / 'assets/scripts'))
    from process import run_driver
    logs = scope / name
    logs.mkdir(mode=0o700)
    command = installed_command(output, 'dockpipe', arguments, directory)
    CONTRACTS.write_json(logs / 'command.json', command)
    try:
        result = run_driver(command, logs, 60, environment, directory,
                            observe=observer.observe if observer is not None else None)
        CONTRACTS.write_json(logs / 'process.json', result)
    finally:
        if observer is not None:
            CONTRACTS.write_json(logs / 'kernel-journal.json', observer.finish())
    return result['exit_code'] == 0 and not result['failure']


def installed_source_cache_proof(output, environment):
    """Reuse the owning blackbox fixture with an actual installed CLI adapter."""
    product = ROOT / 'workflows/qualify/product'
    sys.path.insert(0, str(product / 'drivers'))
    sys.path.insert(0, str(product / 'assets/scripts'))
    specification = importlib.util.spec_from_file_location('flatpak_source_cache_fixture', product / 'drivers/portable.py')
    portable = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(portable)
    workspace = output / 'source-cache-project'
    workspace.mkdir(mode=0o700)
    proof = output / 'source-cache-proof'
    proof.mkdir(mode=0o700)

    def invoke(_cli, arguments, directory, selected_environment, scope, name):
        return installed_fixture_invoke(output, arguments, directory, selected_environment, scope, name)

    if not invoke('/app/bin/dockpipe', ['init'], workspace, environment, proof, 'cache-init'):
        raise RuntimeError('Installed CLI could not initialize its isolated source-cache fixture')
    passed, facts, sources = portable.qualify_source_cache('/app/bin/dockpipe', workspace, environment, proof, invoke=invoke)
    CONTRACTS.write_json(proof / 'observations.json', facts)
    files = [path for path in proof.rglob('*') if path.is_file()]
    files.extend(sources)
    files.append(workspace / 'dockpipe.config.json')
    if sum(path.stat().st_size for path in files) > 64 << 20:
        raise ValueError('Source-cache evidence exceeded its 64MiB bound')
    with tarfile.open(output / 'source-cache-evidence.tar.gz', 'x:gz') as archive:
        for path in files:
            if path.is_symlink() or not path.resolve(strict=True).is_relative_to(output):
                raise ValueError('Source-cache evidence escaped its invocation')
            archive.add(path, arcname=str(path.relative_to(output)), recursive=False)
    return passed, facts


def empty_files_list_response(response):
    """The public Go response omits its empty entries slice."""
    return (isinstance(response, dict) and set(response) <= {'schema', 'path', 'revision', 'entries', 'more', 'after'}
            and response.get('schema') == 'dockpipe.files/v1' and response.get('path') == '.'
            and isinstance(response.get('revision'), str)
            and re.fullmatch(r'[0-9a-f]{64}', response['revision']) is not None
            and response.get('entries', []) == [] and response.get('more', False) is False
            and response.get('after', '') == '')


def installed_files_namespace_binding(output, environment, proof, observer_module, expected_commit, expected_cli_sha256):
    """Hold the public stdin request before dispatch, then list an empty fixture root."""
    from process import terminate_owned
    logs = proof / 'namespace-binding'
    logs.mkdir(mode=0o700)
    root = logs / 'empty-root'
    root.mkdir(mode=0o700)
    request = {'root': str(root), 'request': {
        'schema': 'dockpipe.files/v1', 'action': 'list', 'path': '.', 'limit': 150}}
    CONTRACTS.write_json(logs / 'request.json', request)
    command = installed_command(output, 'dockpipe', ['files', '--request', '-'], proof)
    CONTRACTS.write_json(logs / 'command.json', command)
    observer = observer_module.InstalledCliProcessObserver(command, expected_commit, expected_cli_sha256)
    failure = ''
    with (logs / 'driver.stdout').open('xb') as stdout, (logs / 'driver.stderr').open('xb') as stderr:
        process = subprocess.Popen(command, stdin=subprocess.PIPE, stdout=stdout, stderr=stderr,
                                   env=environment, cwd=proof, start_new_session=True)
        try:
            deadline = time.monotonic() + 8
            while process.poll() is None and time.monotonic() < deadline:
                observer.observe()
                if observer.admitted_cli:
                    break
                time.sleep(0.02)
            if not observer.admitted_cli:
                failure = 'The held public files stdin command was not observed inside the selected app namespace'
            else:
                process.communicate(json.dumps(request).encode('utf-8'), timeout=10)
        finally:
            if process.poll() is None:
                terminate_owned(process)
            process.stdin.close()
            journal = observer.finish()
            CONTRACTS.write_json(logs / 'kernel-journal.json', journal)
            CONTRACTS.write_json(logs / 'process.json', {'exit_code': process.returncode, 'failure': failure})
    for path in [logs / 'driver.stdout', logs / 'driver.stderr']:
        path.chmod(0o600)
        if path.stat().st_size > 4 << 20:
            raise RuntimeError('Held files stdin response exceeded its 4MiB bound')
    response = CONTRACTS.read_json(logs / 'driver.stdout') if process.returncode == 0 else {}
    passed = (not failure and process.returncode == 0 and journal['contained_cli_observed']
              and journal['owned_processes_closed'] and empty_files_list_response(response)
              and not any(root.iterdir()))
    return passed, journal


def installed_workspace_files_proof(output, environment, expected_commit, expected_cli_sha256):
    """Reuse the framework's exact byte, revision and containment assertions."""
    product = ROOT / 'workflows/qualify/product'
    sys.path.insert(0, str(product / 'drivers'))
    sys.path.insert(0, str(product / 'assets/scripts'))
    import workspace_files
    specification = importlib.util.spec_from_file_location('flatpak_cli_process',
                        Path(__file__).with_name('installed-cli-process.py'))
    process_observer = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(process_observer)
    proof = output / 'file-workspace-proof'
    proof.mkdir(mode=0o700)
    binding_passed, binding_journal = installed_files_namespace_binding(
        output, environment, proof, process_observer, expected_commit, expected_cli_sha256)
    journals = []

    def invoke(_cli, arguments, directory, selected_environment, scope, name):
        command = installed_command(output, 'dockpipe', arguments, directory)
        observer = process_observer.InstalledCliProcessObserver(command, expected_commit, expected_cli_sha256)
        result = installed_fixture_invoke(output, arguments, directory, selected_environment, scope, name, observer)
        journals.append(CONTRACTS.read_json(scope / name / 'kernel-journal.json'))
        return result

    assertions, facts, sources = workspace_files.qualify('/app/bin/dockpipe', proof, environment, proof, invoke)
    closed = bool(journals) and all(value['owned_processes_closed'] for value in journals)
    facts.update(held_stdin_namespace_binding=binding_passed, owned_processes_closed=closed,
                 namespace_binding_method='actual public stdin command admitted before read-only dispatch',
                 commands_with_cli_observed=sum(value['contained_cli_observed'] for value in journals),
                 command_count=len(journals), namespace_binding=binding_journal)
    assertions = {name: passed and binding_passed and closed for name, passed in assertions.items()}
    CONTRACTS.write_json(proof / 'observations.json', facts)
    files = set(sources)
    files.add(proof / 'observations.json')
    for path in proof.glob('*/kernel-journal.json'):
        files.add(path)
    files.update(path for path in (proof / 'namespace-binding').iterdir() if path.is_file())
    if sum(path.stat().st_size for path in files) > 64 << 20:
        raise ValueError('Workspace files evidence exceeded its 64MiB bound')
    with tarfile.open(output / 'file-workspace-evidence.tar.gz', 'x:gz') as archive:
        for path in sorted(files):
            if path.is_symlink() or not path.resolve(strict=True).is_relative_to(output):
                raise ValueError('Workspace files evidence escaped its invocation')
            archive.add(path, arcname=str(path.relative_to(output)), recursive=False)
    return assertions, facts


def installed_pipelang_proof(output, environment, configuration, installed_location,
                             expected_commit, expected_cli_sha256, cases):
    """Reuse the SDK owner contract with contained language and host inspection callbacks."""
    product = ROOT / 'workflows/qualify/product'
    sys.path.insert(0, str(product / 'drivers'))
    sys.path.insert(0, str(product / 'assets/scripts'))
    specification = importlib.util.spec_from_file_location('flatpak_pipelang_fixture', product / 'drivers/pipelang.py')
    sdk = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(sdk)
    specification = importlib.util.spec_from_file_location('flatpak_sdk_cli_process',
                        Path(__file__).with_name('installed-cli-process.py'))
    process_observer = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(process_observer)
    deployment = Path(installed_location).resolve(strict=True)
    raw_cli = (deployment / 'files/libexec/dockpipe/dockpipe').resolve(strict=True)
    if not raw_cli.is_relative_to(deployment) or CONTRACTS.digest_file(raw_cli) != expected_cli_sha256:
        raise ValueError('SDK build-info input differs from the observed installed CLI')
    proof = output / 'pipelang-proof'
    proof.mkdir(mode=0o700)
    workspace = proof / 'workspace'
    workspace.mkdir(mode=0o700)
    binding_passed, binding_journal = installed_files_namespace_binding(
        output, environment, proof, process_observer, expected_commit, expected_cli_sha256)
    journals = []

    def execute(_cli, arguments, directory, selected_environment, scope, name):
        command = installed_command(output, 'dockpipe', arguments, directory)
        observer = process_observer.InstalledCliProcessObserver(command, expected_commit, expected_cli_sha256)
        result = installed_fixture_invoke(output, arguments, directory, selected_environment, scope, name, observer)
        journals.append(CONTRACTS.read_json(scope / name / 'kernel-journal.json'))
        return result

    def inspect(tool, arguments, directory, selected_environment, scope, name):
        host_environment = {key: selected_environment[key] for key in [
            'PATH', 'LANG', 'LC_ALL', 'TZ', 'HOME', 'XDG_CONFIG_HOME', 'XDG_DATA_HOME',
            'XDG_STATE_HOME', 'XDG_CACHE_HOME'] if key in selected_environment}
        host_environment.update(GOTOOLCHAIN='local', GOENV='off', GOPROXY='off')
        return sdk.command(tool, arguments, directory, host_environment, scope, name)

    observed, failure_observed, facts, proof_records = sdk.qualify(
        '/app/bin/dockpipe', workspace, environment, proof, configuration, expected_cli_sha256,
        execute=execute, inspect=inspect, inspection_artifact=raw_cli)
    closed = bool(journals) and all(value['owned_processes_closed'] for value in journals)
    qualified_context = binding_passed and closed
    observed = {name: passed and qualified_context for name, passed in observed.items()}
    failure_observed = {name: passed and qualified_context for name, passed in failure_observed.items()}
    facts.update(held_stdin_namespace_binding=binding_passed, namespace_binding=binding_journal,
                 owned_processes_closed=closed, contained_language_commands=len(journals),
                 commands_with_cli_observed=sum(value['contained_cli_observed'] for value in journals),
                 inspected_deployment_file=str(raw_cli), build_info_inspection_location='host pinned Go inspector',
                 language_operations_location='actual installed CLI in app namespace')
    CONTRACTS.write_json(proof / 'installed-observations.json', facts)
    files = {proof / record['path'] for record in proof_records}
    files.add(proof / 'installed-observations.json')
    files.update(proof.glob('*/kernel-journal.json'))
    files.update(path for path in (proof / 'namespace-binding').iterdir() if path.is_file())
    if sum(path.stat().st_size for path in files) > 64 << 20:
        raise ValueError('PipeLang evidence exceeded its 64MiB bound')
    with tarfile.open(output / 'pipelang-evidence.tar.gz', 'x:gz') as archive:
        for path in sorted(files):
            if path.is_symlink() or not path.resolve(strict=True).is_relative_to(output):
                raise ValueError('PipeLang evidence escaped its invocation')
            archive.add(path, arcname=str(path.relative_to(output)), recursive=False)
    observations = {}
    for case in cases:
        assertions = sdk.case_assertions(case, observed, failure_observed)
        if assertions is not None:
            if set(case['required_assertions']) != set(assertions):
                raise ValueError('Shared PipeLang assertion contract changed')
            observations[CONTRACTS.case_key(case)] = {
                name: (passed, 'Shared SDK contract: exact installed module build info, deterministic typed compile/materialize/invoke, structured syntax and private-method rejection, with contained context and owned process cleanup.')
                for name, passed in assertions.items()}
    return observations, facts


def case_result(requested, observations, evidence, failures):
    key = CONTRACTS.case_key(requested)
    supported = requested['scenario'] == 'happy' and (
        (requested['id'] in SUPPORTED and requested['layer'] == 'installed_e2e')
        or (requested['id'] == 'gui.workflow-studio' and requested['layer'] == 'guided_ui'))
    supported = supported or (requested['id'] == 'sdk.pipelang' and requested['scenario'] == 'failure'
                              and requested['layer'] == 'component')
    result = {'id': key[0], 'scenario': key[1], 'layer': key[2], 'assertions': [], 'evidence': evidence,
              'status': 'not_run', 'reason': 'This driver does not observe this requested scenario/layer.'}
    if not supported:
        return result
    selected = observations.get(key, observations.get(requested['id'], {}))
    for name in requested['required_assertions']:
        if name in selected:
            passed, detail = selected[name]
            result['assertions'].append({'id': name, 'status': 'passed' if passed else 'failed', 'detail': detail})
    complete = len(result['assertions']) == len(requested['required_assertions'])
    if complete and all(item['status'] == 'passed' for item in result['assertions']):
        result.update(status='passed', reason='Every requested assertion was observed through the installed product.')
    elif any(item['status'] == 'failed' for item in result['assertions']) or failures:
        result.update(status='failed', reason='; '.join(failures)[-4096:] or 'An installed product assertion failed.')
    else:
        result.update(status='manual_pending', reason='Requested UI actions remain unobserved; CLI checks and partial GUI observations cannot qualify them.')
    return result


def execute(request, output):
    bundle, installation, source = admit_configuration(request['configuration'], request['target']['artifact_sha256'])
    if any(case['id'] == 'sdk.pipelang' for case in request['cases']) and 'sdk' not in request['configuration']:
        raise ValueError('PipeLang qualification requires an explicit pinned SDK inspector and module selection')
    host = host_identity()
    environment = isolated_environment(output, installation)
    logs = []
    def run(arguments, label, timeout=90, selected_environment=None):
        path = output / (label + '.log')
        try:
            return COMMANDS.run_command(arguments, selected_environment or environment, path, timeout)
        finally:
            if path.is_file():
                path.chmod(0o600)
                logs.append(path)
    observations = {}
    failures = []
    facts = {'app_id': APP_ID, 'branch': 'staging', 'installation': str(installation), 'host': host,
             'private_candidate': True, 'bundle_sha256': request['target']['artifact_sha256'],
             'source_inventory_sha256': CONTRACTS.digest_file(source), 'default_installation_changed': False,
             'install_method': request['configuration']['install_method'], 'remote_session_opened': False,
             'gui_observed': False, 'bazzite_qualified': False}
    required_host = ('bazzite' if request['target']['platform'] == BAZZITE_PLATFORM
                     else request['configuration'].get('require_host_id'))
    if required_host and required_host != host['id']:
        facts['warning'] = 'Requires actual ' + required_host + '; this host is ' + host['id']
        CONTRACTS.write_json(output / 'observed-target.json', facts)
        evidence = [{'path': 'observed-target.json', 'sha256': CONTRACTS.digest_file(output / 'observed-target.json')}]
        cases = [{'id': row['id'], 'scenario': row['scenario'], 'layer': row['layer'], 'status': 'not_run',
                  'reason': 'WARNING: ' + facts['warning'], 'assertions': [], 'evidence': evidence} for row in request['cases']]
        return receipt(request, facts, cases)
    try:
        actual_runtime = run(['flatpak', 'info', '--user', '--show-commit', RUNTIME], 'runtime-identity', 30).strip()
        if actual_runtime != RUNTIME_COMMIT:
            raise ValueError('Private installed runtime differs from the exact KDE6.10 pin')
        facts['runtime_commit'] = actual_runtime
        previous = None
        baseline = None
        if request['configuration']['install_method'] == 'upgrade-private':
            baseline = prior_installation(request['configuration'], request['target']['artifact_sha256'], True)
            before = inspect_installed(run, installation, 'before-upgrade-')
            expected_location = installation / Path(baseline['installed_location']).relative_to(baseline['installation'])
            if before['installed_location'] != str(expected_location) or before['installed_commit'] != baseline['installed_commit']:
                raise ValueError('Private upgrade clone differs from the exact baseline deployment')
            for name in INSTALLED_ELFS:
                if name + '_sha256' in baseline and before[name + '_sha256'] != baseline[name + '_sha256']:
                    raise ValueError('Private upgrade clone ELF differs from the baseline: ' + name)
            facts['upgrade'] = {'before': before, 'baseline_bundle_sha256': baseline['bundle_sha256'],
                                'baseline_source_inventory_sha256': baseline['source_inventory_sha256'],
                                'stage': 'baseline-verified'}
            baseline_config = Path(baseline['installation']) / 'repo/config'
            baseline_config_sha256 = CONTRACTS.digest_file(baseline_config)
        if request['configuration']['install_method'] == 'verified-existing':
            previous = prior_installation(request['configuration'], request['target']['artifact_sha256'])
            facts['installation_proof_sha256'] = request['configuration']['installation_proof']['sha256']
            facts['installation_reused_without_replay'] = True
        elif request['configuration']['install_method'] == 'bundle':
            run(['dbus-run-session', '--', 'flatpak', 'install', '--user', '--noninteractive', '--assumeyes',
                 '--no-deps', '--no-related', '--bundle', str(bundle)], 'install', 180)
        else:
            repository = output / 'imported-repository'
            run([request['configuration']['repository_tool']['ostree'], 'init',
                 '--repo=' + str(repository), '--mode=bare-user-only'], 'initialize-local-repository', 30)
            run(['flatpak', 'build-import-bundle', str(repository), str(bundle)], 'import-selected-bundle', 180)
            remote = 'dockpipe-product-' + request['run_id']
            if baseline:
                target_commit = run([request['configuration']['repository_tool']['ostree'], 'rev-parse',
                                     '--repo=' + str(repository), 'app/' + APP_ID + '/x86_64/staging'], 'upgrade-target-commit', 30).strip()
                if not CONTRACTS.matches(CONTRACTS.DIGEST, target_commit) or target_commit == baseline['installed_commit']:
                    raise ValueError('Private upgrade must select a distinct full immutable app commit')
                origin = run(['flatpak', 'info', '--user', '--show-origin', APP_ID], 'upgrade-original-origin', 30).strip()
                if not re.fullmatch(r'[A-Za-z0-9_.-]{1,128}', origin):
                    raise ValueError('Unexpected private baseline origin')
                inventory = run(['flatpak', 'list', '--user', '--columns=ref,origin'], 'upgrade-origin-ref-scope', 30)
                origin_refs = [line.split()[0] for line in inventory.splitlines()
                               if len(line.split()) == 2 and line.split()[1] == origin]
                allowed_refs = {APP_ID + '/x86_64/staging', 'app/' + APP_ID + '/x86_64/staging'}
                if not origin_refs or any(value not in allowed_refs for value in origin_refs):
                    raise ValueError('Upgrade origin is shared with an unselected reference')
                run(['flatpak', 'remote-modify', '--user', '--url=' + repository.as_uri(), origin], 'upgrade-local-origin', 30)
                facts['upgrade'].update(stage='local-origin-selected', target_commit=target_commit, origin=origin)
                run(['dbus-run-session', '--', 'flatpak', 'update', '--user', '--noninteractive', '--assumeyes',
                     '--no-deps', '--no-related', '--commit=' + target_commit, APP_ID], 'upgrade', 180)
            else:
                run(['flatpak', 'remote-add', '--user', '--no-gpg-verify', remote, str(repository)], 'local-source', 30)
                run(['dbus-run-session', '--', 'flatpak', 'install', '--user', '--noninteractive', '--assumeyes',
                     '--no-deps', '--no-related', remote, 'app/' + APP_ID + '/x86_64/staging'], 'install', 180)
        facts.update(inspect_installed(run, installation))
        if previous and (facts['installed_commit'] != previous['installed_commit']
                         or facts['installed_location'] != previous['installed_location']):
            raise ValueError('Existing installed candidate differs from the original readback')
        for name in INSTALLED_ELFS:
            if previous and name + '_sha256' in previous and facts[name + '_sha256'] != previous[name + '_sha256']:
                raise ValueError('Existing installed executable/library changed: ' + name)
        if baseline:
            source_environment = dict(environment, FLATPAK_USER_DIR=baseline['installation'])
            original_commit = run(['flatpak', 'info', '--user', '--show-commit', APP_ID],
                                  'upgrade-source-preserved', 30, source_environment).strip()
            if (facts['installed_commit'] != target_commit or original_commit != baseline['installed_commit']
                    or CONTRACTS.digest_file(baseline_config) != baseline_config_sha256):
                raise ValueError('Upgrade target/read-only baseline metadata identity changed')
            facts['upgrade'].update(stage='upgraded-and-read-back', baseline_metadata_unchanged=True,
                                    after={key: facts[key] for key in facts if key.endswith('_sha256') or key == 'installed_commit'})
        version = run(installed_command(output, 'dockpipe', ['--version']), 'cli-version').strip()
        gui_version = run(installed_command(output, 'dockpipe-launcher', ['--version']), 'gui-version').strip()
        identity = run(installed_command(output, 'dockpipe-launcher', ['--check-installation']), 'gui-installation')
        correct_version = version == request['configuration']['version']
        observations['install.cli'] = {'installed-cli-version': (correct_version, 'Actual installed /app/bin/dockpipe reports ' + version)}
        observations['install.desktop'] = {'installed-cli-version': (correct_version, 'Installed CLI version ' + version),
            'installed-gui-identity': (gui_version == version and 'CLI: /app/bin/dockpipe' in identity,
                                       'Normal installed GUI entrypoint reports matching version and bundled CLI identity.')}
        closure = run(installed_command(output, 'ldd', ['/app/libexec/dockpipe/dockpipe-launcher']), 'gui-runtime-closure')
        if 'not found' in closure or '/run/host/' in closure:
            raise ValueError('Installed GUI dependency closure is missing or resolves from the host')
        facts['gui_static_dependency_closure_verified'] = True
        projects = []
        init_ok = True
        idempotent = True
        no_default_vault = True
        for name, empty_directory in [('blank project', False), ('empty workflows project', True)]:
            project = output / name
            project.mkdir(mode=0o700)
            if empty_directory:
                (project / 'workflows').mkdir()
            run(installed_command(output, 'dockpipe', ['init'], project), name.replace(' ', '-') + '-init')
            config = project / 'dockpipe.config.json'
            starter = project / 'workflows/example/config.yml'
            project_configuration = CONTRACTS.read_json(config) if config.is_file() else {}
            compile_configuration = {}
            if isinstance(project_configuration, dict):
                compile_configuration = project_configuration.get('compile', {})
            workflow_roots = None
            if isinstance(compile_configuration, dict):
                workflow_roots = compile_configuration.get('workflows')
            init_ok = (init_ok and config.is_file() and starter.is_file()
                       and isinstance(workflow_roots, list) and 'workflows' in workflow_roots)
            before = {str(path.relative_to(project)): path.read_bytes() for path in project.rglob('*') if path.is_file()}
            run(installed_command(output, 'dockpipe', ['init'], project), name.replace(' ', '-') + '-repeat-init')
            after = {str(path.relative_to(project)): path.read_bytes() for path in project.rglob('*') if path.is_file()}
            idempotent = idempotent and before == after
            starter_output = run(
                installed_command(output, 'dockpipe', ['--workflow', 'example'], project),
                name.replace(' ', '-') + '-starter-defaults')
            secrets = project_configuration.get('secrets') if isinstance(project_configuration, dict) else None
            no_default_vault = (no_default_vault and secrets in (None, {})
                                and not (project / '.env.vault.template').exists()
                                and not (project / '.env.vault.template.example').exists()
                                and 'Hello from Dockpipe' in starter_output
                                and not any(message in starter_output.lower() for message in (
                                    'vault template file is missing', '1password', 'credential vault')))
            projects.append(project)
        home_unchanged = not (output / 'home/dockpipe.config.json').exists() and not (output / 'home/workflows').exists()
        observations['init.scopes'] = {
            'selected-cwd-initialized': (init_ok, 'Both fresh projects initialized under their explicit physical Flatpak --cwd.'),
            'home-unchanged': (home_unchanged, 'No project metadata or workflows were created in the isolated home.'),
            'empty-workflows-directory-created': (init_ok, 'Absent and existing-empty workflows directories both produced an offline starter and a mapped compile.workflows root.'),
            'idempotent-init': (idempotent, 'Repeated installed CLI init preserved every authored project file.')}
        observations['install.cli']['initialized-project'] = (
            init_ok and no_default_vault,
            'Installed CLI initialized and ran both starters with normal defaults, without vault configuration or setup warnings.')
        facts['initialized_starters_run_without_vault_overrides'] = no_default_vault
        project = projects[0]
        authored = project / 'workflows/authored-fixture'
        workflow_fixture(authored, 'authored-fixture', 'Product authored fixture')
        producer = output / 'fixture-producer'
        namespace = json.loads((project / 'dockpipe.config.json').read_text())['packages']['namespace']
        workflow_fixture(producer / 'workflows/packaged-fixture', 'packaged-fixture', 'Product packaged fixture', namespace)
        run(installed_command(output, 'dockpipe', ['package', 'compile', 'workflows', '--workdir', str(project),
             '--from', str(producer / 'workflows'), '--force'], project), 'compile-packaged-fixture')
        catalog = json.loads(run(installed_command(output, 'dockpipe', ['workflow', 'list', '--workdir', str(project), '--format', 'json'], project), 'discovery-first-launch'))
        authored_record = matching_workflow(catalog, 'Product authored fixture', False)
        packaged_record = matching_workflow(catalog, 'Product packaged fixture', True)
        def input_preserved(record):
            return any(value.get('env_name') == 'DOCKPIPE_FIXTURE_MARKER' and value.get('default_value') == 'product-fixture'
                       for value in record.get('inputs', []))
        reopened = json.loads(run(installed_command(output, 'dockpipe', ['workflow', 'list', '--workdir', str(project), '--format', 'json'], project), 'discovery-separate-launch'))
        observations['workflow.discovery'] = {
            'authored-workflow-discovered': (matching_workflow(reopened, 'Product authored fixture', False) == authored_record, 'Authored workflow remains in the installed CLI catalog across separate process launches.'),
            'installed-workflow-discovered': (matching_workflow(reopened, 'Product packaged fixture', True) == packaged_record, 'Fixture compiled through installed package API is discovered from its real tar:// package locator.'),
            'declared-inputs-preserved': (input_preserved(authored_record) and input_preserved(packaged_record), 'Both catalog records preserve typed input env mapping and declared default.')}
        cache_passed, cache_facts = installed_source_cache_proof(output, environment)
        facts['source_cache_identity'] = cache_facts
        previous_discovery = observations['workflow.discovery']['installed-workflow-discovered']
        observations['workflow.discovery']['installed-workflow-discovered'] = (
            previous_discovery[0] and cache_passed,
            previous_discovery[1] + ' Same-config source A/B assets remain distinct and repeat B reuses its archive.')
        run(installed_command(output, 'dockpipe', ['--workflow-file', packaged_record['config_path'], '--workdir', str(project),
             '--no-compile-deps', '--no-vault', '--no-op-inject'], project), 'execute-real-packaged-locator')
        executed = (project / 'product-workflow-result.txt').read_text() == 'product-fixture'
        observations['install.cli']['host-workflow-executed'] = (executed, 'Actual installed CLI executed the immutable package locator and generated the exact fixture marker.')
        alias = output / 'project-alias'
        alias.symlink_to(project, target_is_directory=True)
        alias_catalog = json.loads(run(installed_command(output, 'dockpipe', ['workflow', 'list', '--format', 'json'], alias), 'physical-symlink-project'))
        facts['symlink_project_physical_cwd_preserved'] = matching_workflow(alias_catalog, 'Product authored fixture', False) == authored_record
        facts['portal_document_grant_tested'] = False
        facts['fixture_workspace'] = str(project)
        if any(row['id'] == 'files.workspace' and row['scenario'] == 'happy'
               and row['layer'] == 'installed_e2e' for row in request['cases']):
            file_assertions, file_facts = installed_workspace_files_proof(
                output, environment, facts['installed_commit'], facts['cli_sha256'])
            facts['workspace_files'] = file_facts
            details = {
                'create-read-write-rename-delete': 'Shared public files fixture preserved BOM/CRLF bytes through create, read, write, rename, list and delete.',
                'revision-checks': 'Shared fixture verified the content revision and exact stale-write diagnostic with replacement bytes unchanged.',
                'workspace-contained': 'Shared fixture verified exact outside-root/link diagnostics and unchanged adjacent canary.'}
            observations['files.workspace'] = {
                name: (passed, details[name] + ' A held public stdin call admitted the exact contained B CLI before read-only dispatch; every command closed its recorded process tree.')
                for name, passed in file_assertions.items()}
        if any(row['id'] == 'sdk.pipelang' for row in request['cases']):
            sdk_observations, sdk_facts = installed_pipelang_proof(
                output, environment, request['configuration']['sdk'], facts['installed_location'],
                facts['installed_commit'], facts['cli_sha256'], request['cases'])
            observations.update(sdk_observations)
            facts['pipelang_sdk'] = sdk_facts
        facts['gui_handoff'] = {'entrypoint': '/app/bin/dockpipe-launcher', 'workspace': str(project),
                                'required_isolation': 'Use owned display/accessibility bus and this private installation; retain /app Qt and CLI.'}
        if 'gui_tools' in request['configuration']:
            tools = request['configuration']['gui_tools']
            xvfb = regular(tools['xvfb'])
            if CONTRACTS.digest_file(xvfb) != tools['xvfb_sha256']:
                raise ValueError('Private display executable changed')
            gui_request = {'output': str(output), 'installation': str(installation), 'workspace': str(project),
                           'xvfb': str(xvfb), 'expected_commit': facts['installed_commit'],
                           'expected_gui_sha256': facts['gui_sha256'], 'method': tools.get('method', 'atspi')}
            if tools.get('feature') == 'browser':
                location = Path(run(['flatpak', 'info', '--user', '--show-location', RUNTIME], 'browser-runtime-location', 30).strip())
                python = location / 'files/bin/python3.13'
                if not location.resolve(strict=True).is_relative_to(installation.resolve(strict=True)):
                    raise ValueError('Browser fixture runtime escaped the private installation')
                gui_request['browser_fixture'] = {'python_sha256': CONTRACTS.digest_file(python)}
            CONTRACTS.write_json(output / 'gui-request.json', gui_request)
            run(['/usr/bin/python3', str(Path(__file__).with_name('installed-product-gui.py')),
                 '--request', str(output / 'gui-request.json')], 'normal-gui-actions',
                840 if tools.get('method') == 'guided' else 240)
            observed = CONTRACTS.read_json(output / 'gui-observed.json')
            facts['gui_observed'] = observed['normal_gui_started']
            facts['gui_observer_method'] = observed.get('observer_method', 'external-atspi')
            facts['atspi_qualified'] = observed.get('atspi_qualified', False)
            detail = ('Exact installed child observed on its private display with screenshot-bound guided inputs.'
                      if facts['gui_observer_method'] == 'agent-guided-visual-keyboard'
                      else 'Exact installed child observed through its private accessibility bus.')
            observations['install.desktop'].update({
                'normal-gui-started': (observed['normal_gui_started'], detail),
                'settings-retained': (observed['settings_retained'] and observed['gui_exit_normal'], 'Normal Settings checkbox saved, application quit and relaunched, then the same setting read through UI.')})
            observations['gui.workflow-studio'] = {
                'selected-project-workflows-visible': (observed['selected_project_workflows_visible'], 'Actual installed workflow picker displays both generated authored and packaged catalog titles.'),
                'project-reopen': (observed['project_reopen'], 'Selected project catalog remains visible after normal installed application relaunch.')}
            if 'edit_save_run' in observed:
                observations['gui.workflow-studio']['edit-save-run'] = (
                    observed['edit_save_run'], 'Recorded generated GUI edit/save/run inputs; exact YAML comment and new workflow marker independently read back.')
            if 'browser' in observed:
                browser = observed['browser']
                closed = observed['gui_exit_normal'] and observed.get('runtime_directory_cleaned') is True
                browser['owned_gui_and_runtime_closed'] = closed
                facts['browser'] = browser
                observations['browser.local'] = {
                    'local-page-loaded': (browser['local_page_loaded'], 'Normal installed browser loaded generated loopback HTML and acknowledged DOM actions inside its isolated runtime namespace.'),
                    'tabs-and-downloads': (browser['tabs_and_downloads'] and closed, 'Two generated tabs retained DOM state across app navigation; actual GUI download completed with exact bytes and owned child/runtime cleanup.'),
                    'profile-restart': (browser['profile_restart'] and closed, 'Normal restart retained browser preferences and started a new empty temporary browsing session; seeded generated localStorage was independently absent afterward.')}
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        failures.append(str(error)[-4096:])
    CONTRACTS.write_json(output / 'observed-target.json', facts)
    logs.append(output / 'observed-target.json')
    for path in [output / 'gui-observed.json', output / 'desktop/normal-window.json',
                 output / 'desktop/first-project-catalog.json', output / 'desktop/reopened-project-catalog.json',
                 output / 'owned-first-kernel-journal.json', output / 'owned-reopen-kernel-journal.json',
                 output / 'owned-cleanup-kernel-journal.json', output / 'observer-first-result.json',
                 output / 'observer-reopen-result.json', output / 'observer-first.log', output / 'observer-reopen.log',
                 output / 'guided-evidence.tar.gz', output / 'guided-observations.json',
                 output / 'owned-guided-first-kernel-journal.json', output / 'owned-guided-reopen-kernel-journal.json',
                 output / 'source-cache-evidence.tar.gz', output / 'file-workspace-evidence.tar.gz',
                 output / 'pipelang-evidence.tar.gz']:
        if path.is_file():
            logs.append(path)
    for pattern in ['desktop/installed-gui-*.log', 'browser-server-*-kernel-journal.json',
                    'browser-owned-children.json', 'browser-server-*.log']:
        logs.extend(path for path in output.glob(pattern) if path.is_file())
    evidence = [{'path': str(path.relative_to(output)), 'sha256': CONTRACTS.digest_file(path)} for path in logs]
    cases = [case_result(row, observations, evidence, failures) for row in request['cases']]
    return receipt(request, facts, cases)


def receipt(request, facts, cases):
    result = {key: request[key] for key in ('run_id', 'plan_sha256', 'registry_sha256', 'target')}
    result.update(schema='dockpipe.product-integration.lane/v1', observed_target=facts, cases=cases)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--request', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    arguments = parser.parse_args()
    request = load_request(arguments.request)
    if not arguments.output.is_absolute() or arguments.output.exists():
        parser.error('Use a fresh absolute invocation output')
    arguments.output.mkdir(mode=0o700, parents=True)
    result = execute(request, arguments.output)
    CONTRACTS.validate_receipt(result, request, arguments.output)
    CONTRACTS.write_json(arguments.output / 'result.json', result)
    # A valid receipt is a completed driver invocation. The shared product
    # runner evaluates failed/pending cases and its coverage denominator.
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
