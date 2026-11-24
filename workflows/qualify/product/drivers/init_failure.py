"""Exercise normal initialization refusal in a private read-only project."""
import argparse
import os
from pathlib import Path
import platform
import shutil
import sys


OWNER = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(OWNER / 'assets/scripts'))
from contracts import digest_file, read_json, require, validate_receipt, write_json
from process import run_driver


ASSERTIONS = {'expected-failure-observed', 'no-unowned-effects', 'actionable-result'}


def evaluate(facts):
    process = facts['process']
    refused = (type(process.get('exit_code')) is int and process['exit_code'] == 1
               and process.get('failure') == '')
    permission_error = any('permission denied' in line.lower() and facts['project'] in line
                           for line in facts['diagnostic'].splitlines())
    return {
        'expected-failure-observed': refused and permission_error,
        'no-unowned-effects': facts['project_unchanged'] and facts['outside_canary_unchanged']
                             and facts['private_home_not_initialized'] and facts['artifact_unchanged'],
        'actionable-result': refused and permission_error,
    }


def run(request, output):
    require(platform.system() == 'Linux' and os.geteuid() != 0
            and request['target']['platform'] == 'linux-portable'
            and request['target']['kind'] == 'local',
            'Read-only initialization fixture requires a non-root local Linux portable target')
    require(set(request['configuration']) == {'artifact'}, 'Select only the pinned installed CLI')
    artifact = Path(request['configuration']['artifact'])
    expected_sha256 = request['target']['artifact_sha256']
    require(artifact.is_absolute() and digest_file(artifact) == expected_sha256,
            'Initialization CLI differs from its selected pin')
    for case in request['cases']:
        require((case['id'], case['scenario'], case['layer']) == ('init.scopes', 'failure', 'component')
                and set(case['required_assertions']) == ASSERTIONS,
                'Read-only initialization assertion contract changed')
    output.mkdir(mode=0o700, parents=True)
    cli = output / 'dockpipe'
    shutil.copyfile(artifact, cli)
    cli.chmod(0o700)
    home = output / 'home'
    home.mkdir(mode=0o700)
    environment = {'PATH': '/usr/local/bin:/usr/bin:/bin', 'HOME': str(home),
                   'LANG': 'C.UTF-8', 'LC_ALL': 'C.UTF-8',
                   'DOCKPIPE_VAULT_INJECT': '0', 'DOCKPIPE_INTERACTIVE': '0'}
    for name in ['config', 'cache', 'data', 'state', 'runtime']:
        directory = home / name
        directory.mkdir(mode=0o700)
        suffix = '_DIR' if name == 'runtime' else '_HOME'
        environment['XDG_' + name.upper() + suffix] = str(directory)
    project = output / 'read only project'
    project.mkdir(mode=0o700)
    marker = project / 'retained.txt'
    marker.write_bytes(b'Generated project content must remain exact.\n')
    marker_sha256 = digest_file(marker)
    outside = output / 'outside-canary.txt'
    outside.write_bytes(b'Generated adjacent fixture must remain exact.\n')
    outside_sha256 = digest_file(outside)
    logs = output / 'init'
    logs.mkdir(mode=0o700)
    command = [str(cli), 'init']
    write_json(logs / 'command.json', command)
    project.chmod(0o500)
    try:
        process = run_driver(command, logs, 20, environment, project)
        project_unchanged = sorted(path.name for path in project.iterdir()) == ['retained.txt']
        project_unchanged = project_unchanged and digest_file(marker) == marker_sha256
    finally:
        project.chmod(0o700)
    write_json(logs / 'process.json', process)
    diagnostic = '\n'.join((logs / name).read_text() for name in ['driver.stdout', 'driver.stderr'])
    facts = {
        'process': process, 'diagnostic': diagnostic, 'project': str(project),
        'project_unchanged': project_unchanged,
        'outside_canary_unchanged': digest_file(outside) == outside_sha256,
        'private_home_not_initialized': not (home / 'dockpipe.config.json').exists()
                                        and not (home / 'workflows').exists(),
        'artifact_unchanged': digest_file(cli) == expected_sha256 and digest_file(artifact) == expected_sha256,
    }
    write_json(output / 'observations.json', facts)
    observed = evaluate(facts)
    reason = 'Actual init in a read-only generated project must refuse normally with an actionable permission error'
    evidence = [{'path': str(path.relative_to(output)), 'sha256': digest_file(path)}
                for path in [output / 'observations.json', logs / 'command.json', logs / 'process.json',
                             logs / 'driver.stdout', logs / 'driver.stderr', marker, outside]]
    rows = []
    for case in request['cases']:
        row = {key: case[key] for key in ['id', 'scenario', 'layer']}
        row.update(status='passed' if all(observed.values()) else 'failed', reason=reason, evidence=evidence,
                   assertions=[{'id': name, 'status': 'passed' if observed[name] else 'failed', 'detail': reason}
                               for name in case['required_assertions']])
        rows.append(row)
    receipt = {key: request[key] for key in ['run_id', 'plan_sha256', 'registry_sha256', 'target']}
    receipt.update(schema='dockpipe.product-integration.lane/v1', cases=rows, target_role='local-installed-cli')
    validate_receipt(receipt, request, output)
    write_json(output / 'result.json', receipt)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--request', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    arguments = parser.parse_args()
    require(arguments.output.is_absolute() and not arguments.output.exists(), 'Select fresh absolute init output')
    request = read_json(arguments.request)
    require(request.get('schema') == 'dockpipe.product-integration.request/v1', 'Unknown initialization request schema')
    run(request, arguments.output)


if __name__ == '__main__':
    main()
