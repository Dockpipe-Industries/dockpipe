"""Recover an interrupted pinned reinstall in a new private catalog/store only."""
from pathlib import Path
import argparse
import functools
import itertools
import os
import platform
import shutil
import ssl
import sys
import threading

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'assets/scripts'))
from contracts import digest_file, read_json, require, write_json
from package_lifecycle import FixtureHandler, FixtureServer
from portable import command

IDENTITY = 'lifecycle-reinstall'
BYSTANDER = 'lifecycle-bystander'
ASSERTIONS = {'original-identity-retained', 'state-reconciled', 'no-duplicate-effects'}


class InterruptedArchiveHandler(FixtureHandler):
    def do_GET(self):
        if self.path == self.server.interrupted_path and self.server.interrupt_next:
            if self.path not in self.server.allowed_paths or len(self.server.requests) >= 64:
                self.send_error(503)
                return
            self.server.interrupt_next = False
            self.server.requests.append(self.path)
            source = Path(self.directory) / self.path.lstrip('/')
            require(source.is_file() and not source.is_symlink() and 1 < source.stat().st_size <= 8 * 1024 * 1024,
                    'Generated interrupted archive exceeds fixture scope')
            data = source.read_bytes()
            partial = max(1, len(data) // 2)
            self.send_response(200)
            self.send_header('Content-Type', 'application/octet-stream')
            self.send_header('Content-Length', str(len(data)))
            self.end_headers()
            self.wfile.write(data[:partial])
            self.wfile.flush()
            self.close_connection = True
            self.server.interruptions.append({'path': self.path, 'declared_bytes': len(data), 'sent_bytes': partial})
            return
        super().do_GET()


def selected(case):
    return (case['id'], case['scenario'], case['layer']) == ('packages.lifecycle', 'restart-recovery', 'component')


def store_paths(root):
    paths = list(itertools.islice(root.rglob('*'), 257))
    require(len(paths) <= 256, 'Private installed store exceeds its fixture bound')
    require(all(not path.is_symlink() for path in paths), 'Private installed store contains a link')
    return sorted(paths)


def store_inventory(root):
    result = {}
    for path in store_paths(root):
        if path.is_file():
            result[path.relative_to(root).as_posix()] = digest_file(path)
    return result


def no_partial_install(root):
    return not any(path.name.startswith(('.install-', '.download-')) for path in store_paths(root))


def interrupted_refusal(process, diagnostic, interruption, expected_path):
    if not isinstance(process, dict):
        return False
    if type(process.get('exit_code')) is not int or process['exit_code'] != 1:
        return False
    if process.get('failure') != '':
        return False
    if not isinstance(diagnostic, str) or len(diagnostic) > 65536:
        return False
    if not diagnostic.strip().endswith('unexpected EOF'):
        return False
    if not isinstance(interruption, dict) or interruption.get('path') != expected_path:
        return False
    if type(interruption.get('sent_bytes')) is not int:
        return False
    if type(interruption.get('declared_bytes')) is not int:
        return False
    return 0 < interruption['sent_bytes'] < interruption['declared_bytes'] <= 8 * 1024 * 1024


def failure_is_reconciled(facts):
    required = [
        'interrupted_archive_refused', 'one_interruption', 'one_failed_download',
        'original_store_unchanged', 'original_marker_unchanged',
        'bystander_unchanged', 'no_partials_after_failure',
    ]
    return all(facts.get(name) is True for name in required)


def retry_is_reconciled(facts):
    required = [
        'fresh_retry_succeeded', 'same_path_and_digest',
        'one_explicit_retry_download', 'no_partials_after_retry',
    ]
    return failure_is_reconciled(facts) and all(facts.get(name) is True for name in required)


def recover_if_known(facts, retry, execute):
    """Never repeat an uncertain install or execute an unverified recovery."""
    if not failure_is_reconciled(facts):
        return
    retry()
    if retry_is_reconciled(facts):
        execute()


def evaluate(facts):
    required_observations = {
        'original-identity-retained': [
            'interrupted_archive_refused',
            'original_store_unchanged',
            'original_marker_unchanged',
            'bystander_unchanged',
        ],
        'state-reconciled': [
            'no_partials_after_failure',
            'fresh_retry_succeeded',
            'same_path_and_digest',
            'single_catalog_identity_after_retry',
            'no_partials_after_retry',
            'fixture_server_stopped',
            'artifact_pins_unchanged',
        ],
        'no-duplicate-effects': [
            'one_interruption',
            'one_failed_download',
            'one_explicit_retry_download',
            'only_two_explicit_workflow_executions',
            'final_store_unchanged',
        ],
    }
    return {
        assertion: all(facts.get(name) is True for name in names)
        for assertion, names in required_observations.items()
    }


def run(request, output):
    output.mkdir(mode=0o700, parents=True)
    cases = [case for case in request['cases'] if selected(case)]
    if not cases:
        emit(request, output, {}, [], 'blocked', 'No supported package restart-recovery case selected')
        return
    require(all(set(case['required_assertions']) == ASSERTIONS for case in cases), 'Package recovery assertion contract changed')
    target = request['target']
    require(platform.system() == 'Linux' and target['platform'] == 'linux-portable' and target['kind'] == 'local',
            'Pinned reinstall recovery requires the selected local Linux portable CLI')
    configuration = request['configuration']
    require(set(configuration) == {'artifact', 'tls_certificate', 'tls_key', 'tls_certificate_sha256', 'tls_key_sha256'},
            'Use existing pinned CLI and generated private TLS fixture references')
    paths = {name: Path(configuration[name]) for name in ['artifact', 'tls_certificate', 'tls_key']}
    for path in paths.values():
        require(path.is_absolute() and path.is_file() and not path.is_symlink(), 'Select absolute regular fixture files')
    require(paths['tls_key'].stat().st_mode & 0o077 == 0, 'Fixture TLS key must be owner-private')
    require(all(paths[name].stat().st_size <= 65536 and digest_file(paths[name]) == configuration[name + '_sha256']
                for name in ['tls_certificate', 'tls_key']), 'Fixture TLS pin or byte bound differs')
    require(digest_file(paths['artifact']) == target['artifact_sha256'], 'Selected CLI pin differs')
    cli = output / 'dockpipe'
    shutil.copyfile(paths['artifact'], cli)
    cli.chmod(0o700)
    require(digest_file(cli) == target['artifact_sha256'], 'Copied CLI pin differs')
    home = output / 'home'
    home.mkdir(mode=0o700)
    environment = {'PATH': '/usr/bin:/bin', 'LANG': 'C.UTF-8', 'LC_ALL': 'C.UTF-8', 'HOME': str(home),
                   'DOCKPIPE_INTERACTIVE': '0', 'DOCKPIPE_VAULT_INJECT': '0',
                   'DOCKPIPE_GLOBAL_ROOT': str(output / 'user-store'), 'DOCKPIPE_SYSTEM_ROOT': str(output / 'system-store')}
    for name in ['config', 'cache', 'data', 'state']:
        directory = home / name
        directory.mkdir(mode=0o700)
        environment['XDG_' + name.upper() + '_HOME'] = str(directory)
    producer, consumer = output / 'producer', output / 'consumer'
    for directory in [producer, consumer]:
        directory.mkdir(mode=0o700)
        (directory / 'dockpipe.config.json').write_text('{}\n')
    producer_environment = dict(environment, DOCKPIPE_PACKAGES_ROOT=str(output / 'compiled-store'))
    operations = {}
    def invoke(name, arguments, workspace=consumer, selected_environment=environment):
        operations[name] = command(cli, arguments, workspace, selected_environment, output, name)
        return operations[name]
    for identity in [IDENTITY, BYSTANDER]:
        source = producer / identity
        source.mkdir()
        (source / 'package.yml').write_text('schema: 1\nkind: workflow\nname: ' + identity
            + '\nversion: 1.0.0\nnamespace: recovery-fixture\n')
        (source / 'config.yml').write_text('name: ' + identity
            + '\nvault: none\ndocker_preflight: false\nsteps:\n  - id: marker\n    kind: host\n    cwd: repo\n    run: marker.sh\n')
        (source / 'marker.sh').write_text("set -eu\nprintf '%s\\n' '" + identity + "' >> " + identity + '.txt\n')
        require(invoke('compile-' + identity, ['package', 'compile', 'workflow', '--from', str(source)],
                       producer, producer_environment), 'Generated recovery fixture compile failed')
    site = output / 'catalog-site'
    require(invoke('export', ['package', 'build', 'store', '--out', str(site), '--only', 'workflows'],
                   producer, producer_environment), 'Private recovery catalog export failed')
    manifest_path = site / 'packages-store-manifest.json'
    entries = {row['name']: row for row in read_json(manifest_path)['packages']['workflows']}
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.load_cert_chain(paths['tls_certificate'], paths['tls_key'])
    server = FixtureServer(('127.0.0.1', 0), functools.partial(InterruptedArchiveHandler, directory=str(site)))
    server.tls_context = context
    server.allowed_paths = {'/packages-store-manifest.json', *('/' + row['tarball'] for row in entries.values())}
    server.requests = []
    server.interrupted_path = '/' + entries[IDENTITY]['tarball']
    server.interrupt_next = False
    server.interruptions = []
    thread = threading.Thread(target=server.serve_forever, name='package-recovery-private-tls', daemon=True)
    environment['SSL_CERT_FILE'] = str(paths['tls_certificate'])
    remote = 'https://127.0.0.1:' + str(server.server_port) + '/packages-store-manifest.json'
    facts = {}
    thread.start()
    try:
        installed = {}
        def install(name):
            return invoke(name, ['package', 'install', '--remote', remote, '--kind', 'workflow',
                                 '--name', IDENTITY, '--sha256', entries[IDENTITY]['sha256']])
        for identity in [IDENTITY, BYSTANDER]:
            name = 'baseline-install-' + identity
            require(invoke(name, ['package', 'install', '--remote', remote, '--kind', 'workflow',
                                 '--name', identity, '--sha256', entries[identity]['sha256']]), 'Baseline private install failed')
            path = Path(read_json(output / name / 'driver.stdout')['path'])
            require(path.is_absolute() and path.resolve().is_relative_to(output / 'user-store')
                    and digest_file(path) == entries[identity]['sha256'], 'Installed baseline archive escaped its exact private pin')
            installed[identity] = path
        selected_archive = installed[IDENTITY]
        locator = 'tar://' + str(selected_archive) + '##workflows/' + IDENTITY + '/config.yml'
        require(invoke('baseline-execution', ['--workflow-file', locator]), 'Initial exact installed workflow did not execute')
        marker = consumer / (IDENTITY + '.txt')
        expected_line = (IDENTITY + '\n').encode()
        require(marker.read_bytes() == expected_line, 'Baseline generated execution count differs')
        before = store_inventory(output / 'user-store')
        marker_before = digest_file(marker)
        start = len(server.requests)
        server.interrupt_next = True
        install('interrupted-reinstall')
        interruption = server.interruptions[0] if len(server.interruptions) == 1 else None
        process = read_json(output / 'interrupted-reinstall/process.json')
        with (output / 'interrupted-reinstall/driver.stderr').open('rb') as stream:
            diagnostic_bytes = stream.read(65537)
        diagnostic = diagnostic_bytes.decode('utf-8', errors='replace')
        facts.update(
            interrupted_archive_refused=interrupted_refusal(process, diagnostic, interruption, server.interrupted_path),
            one_interruption=len(server.interruptions) == 1 and not server.interrupt_next,
            one_failed_download=server.requests[start:].count(server.interrupted_path) == 1,
            original_store_unchanged=store_inventory(output / 'user-store') == before,
            original_marker_unchanged=digest_file(marker) == marker_before,
            bystander_unchanged=digest_file(installed[BYSTANDER]) == entries[BYSTANDER]['sha256'],
            no_partials_after_failure=no_partial_install(output / 'user-store'),
        )
        def retry():
            start = len(server.requests)
            retried = install('fresh-retry')
            receipt = read_json(output / 'fresh-retry/driver.stdout') if retried else {}
            facts.update(
                fresh_retry_succeeded=retried,
                same_path_and_digest=receipt.get('path') == str(selected_archive)
                    and digest_file(selected_archive) == entries[IDENTITY]['sha256'],
                one_explicit_retry_download=server.requests[start:].count(server.interrupted_path) == 1,
                no_partials_after_retry=no_partial_install(output / 'user-store'),
            )

        def execute():
            listed = invoke('reconciled-catalog', ['catalog', 'list', '--format', 'json'])
            catalog = read_json(output / 'reconciled-catalog/driver.stdout') if listed else {}
            facts['single_catalog_identity_after_retry'] = sum(row.get('workflow_id') == IDENTITY
                for row in catalog.get('workflows', [])) == 1
            if not facts['single_catalog_identity_after_retry']:
                return
            executed = invoke('recovered-execution', ['--workflow-file', locator])
            facts['only_two_explicit_workflow_executions'] = executed and marker.read_bytes() == expected_line * 2
            facts['final_store_unchanged'] = store_inventory(output / 'user-store') == before

        recover_if_known(facts, retry, execute)
        facts['requests'] = list(server.requests)
        facts['interruption'] = interruption
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=10)
    facts['fixture_server_stopped'] = not thread.is_alive() and server.socket.fileno() == -1
    facts['artifact_pins_unchanged'] = (digest_file(cli) == target['artifact_sha256']
        and digest_file(paths['artifact']) == target['artifact_sha256']
        and all(digest_file(paths[name]) == configuration[name + '_sha256'] for name in ['tls_certificate', 'tls_key']))
    write_json(output / 'observations.json', facts)
    evidence_paths = [output / 'observations.json', manifest_path, consumer / (IDENTITY + '.txt')]
    for name in operations:
        evidence_paths.extend(sorted((output / name).iterdir()))
    evidence = [{'path': path.relative_to(output).as_posix(), 'sha256': digest_file(path)} for path in evidence_paths]
    require(len(evidence) <= 64, 'Recovery evidence exceeds standard receipt bound')
    assertions = evaluate(facts)
    emit(request, output, facts, evidence, 'passed' if all(assertions.values()) else 'failed',
         'Actual interrupted pinned reinstall and explicit fresh retry; no package version update, promotion or rollback implied')


def emit(request, output, facts, evidence, status, reason):
    rows = []
    evaluated = evaluate(facts)
    for case in request['cases']:
        row = {name: case[name] for name in ['id', 'scenario', 'layer']}
        if selected(case):
            row.update(status=status, reason=reason, evidence=evidence,
                assertions=[{'id': name, 'status': 'passed' if value else 'failed', 'detail': reason}
                            for name, value in evaluated.items()] if status in ['passed', 'failed'] else [])
        else:
            row.update(status='blocked', reason='This driver implements only pinned reinstall restart-recovery', evidence=[], assertions=[])
        rows.append(row)
    result = {name: request[name] for name in ['run_id', 'plan_sha256', 'registry_sha256', 'target']}
    result.update(schema='dockpipe.product-integration.lane/v1', cases=rows, observed=facts)
    write_json(output / 'result.json', result)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--request', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    require(args.output.is_absolute() and not os.path.lexists(args.output), 'Select fresh absolute private output')
    request = read_json(args.request)
    require(request.get('schema') == 'dockpipe.product-integration.request/v1', 'Unknown product request')
    run(request, args.output)


if __name__ == '__main__':
    main()
