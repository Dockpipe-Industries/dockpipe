#!/usr/bin/env python3
"""Local Linux VM admission and normal-GUI installation qualification driver.

An explicit guided-install operation runs a new private normal GUI lifecycle.
Unobserved assertions remain pending; prior receipts are never imported as proof.
"""
import argparse
import json
from pathlib import Path
import re
import sys

from desktop import digest
from guest_environment import resource_admission

SCHEMA = 'dockpipe.product-integration.request/v1'
DIGEST = re.compile(r'\A[0-9a-f]{64}\Z')
SUPPORTED_CASES = {'install.cli', 'install.desktop'}


def regular_absolute(value, maximum=None):
    path = Path(value)
    if not path.is_absolute() or path.is_symlink() or not path.is_file():
        raise ValueError('An explicit absolute regular file is required.')
    if maximum is not None and path.stat().st_size > maximum:
        raise ValueError('Input exceeds its byte bound.')
    return path


def load_request(path):
    path = regular_absolute(path, 1 << 20)
    request = json.loads(path.read_text())
    required = {'schema', 'run_id', 'plan_sha256', 'registry_sha256', 'target', 'configuration', 'cases'}
    if set(request) != required or request['schema'] != SCHEMA:
        raise ValueError('Invalid Linux lane request envelope.')
    if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_.-]{0,127}', request['run_id']):
        raise ValueError('Invalid bounded run identity.')
    for key in ['plan_sha256', 'registry_sha256']:
        if not DIGEST.fullmatch(request[key]):
            raise ValueError('Invalid request digest binding.')
    target = request['target']
    if set(target) != {'id', 'platform', 'kind', 'artifact_sha256'}:
        raise ValueError('Invalid target shape.')
    if target['platform'] != 'linux-amd64-deb-ubuntu24.04' or target['kind'] != 'vm':
        raise ValueError('This adapter admits only a new Ubuntu 24.04 amd64 VM.')
    if not DIGEST.fullmatch(target['artifact_sha256']):
        raise ValueError('Target artifact digest is required.')
    if not isinstance(request['cases'], list) or len(request['cases']) > 32:
        raise ValueError('Requested cases exceed bounds.')
    seen = set()
    for case in request['cases']:
        identity = (case['id'], case['scenario'], case['layer'])
        if identity in seen:
            raise ValueError('Duplicate Linux lane case.')
        seen.add(identity)
        if not isinstance(case['required_assertions'], list) or len(case['required_assertions']) > 32:
            raise ValueError('Required assertions exceed bounds.')
    return request


def admit_configuration(configuration, output):
    required = {'pins_file', 'pins_sha256', 'media_file', 'media_sha256',
                'baseline_receipt', 'baseline_receipt_sha256'}
    optional = {'operation', 'timeout_seconds', 'guest_enrollment'}
    if not required.issubset(configuration) or set(configuration) - required - optional:
        raise ValueError('Configuration must supply exact host, installer and published baseline pins.')
    if configuration.get('operation', 'admission') not in ['admission', 'guided-install']:
        raise ValueError('Unknown Linux lane operation.')
    if 'guest_enrollment' in configuration and configuration.get('operation') != 'guided-install':
        raise ValueError('Guest enrollment requires a fresh guided install.')
    if 'guest_enrollment' in configuration:
        enrollment = configuration['guest_enrollment']
        required_enrollment = {'grant_file', 'grant_sha256', 'host_state', 'worker_deb', 'worker_deb_sha256',
                               'desktop_deb', 'desktop_deb_sha256'}
        if not isinstance(enrollment, dict) or set(enrollment) != required_enrollment:
            raise ValueError('Guest enrollment requires a private run grant and exact CLI and desktop DEBs.')

        for key in ('worker_deb', 'desktop_deb'):
            package = regular_absolute(enrollment[key], 256 << 20)
            if package.suffix != '.deb' or digest(package) != enrollment[key + '_sha256']:
                raise ValueError(f'Selected {key} differs from the qualification plan.')

    timeout = configuration.get('timeout_seconds', 3600)
    if isinstance(timeout, bool) or not isinstance(timeout, int) or not 60 <= timeout <= 7200:
        raise ValueError('Guided timeout must be an integer from 60 to 7200 seconds.')
    selections = {}
    for stem, key in [('pins', 'pins_file'), ('media', 'media_file'), ('baseline_receipt', 'baseline_receipt')]:
        expected = configuration[stem + '_sha256']
        if not DIGEST.fullmatch(expected):
            raise ValueError('An exact configuration digest is required.')
        path = regular_absolute(configuration[key], 4 << 20 if stem != 'media' else 8 << 30)
        if digest(path) != expected:
            raise ValueError('Selected input changed after planning.')
        selections[stem] = path
    pins = json.loads(selections['pins'].read_text())
    runtime_paths = [pins['gui'], pins['cli']]
    if configuration.get('operation') == 'guided-install':
        if not isinstance(pins.get('xvfb'), str):
            raise ValueError('Guided installation requires an explicitly hash-pinned Xvfb in the host receipt.')
        runtime_paths.append(pins['xvfb'])
        qemu = Path(pins['qemu_prefix']) / 'bin'
        runtime_paths.extend([str(qemu / 'qemu-system-x86_64'), str(qemu / 'qemu-img')])
    for selected_path in runtime_paths:
        path = regular_absolute(selected_path)
        expected = pins['pins'].get(str(path))
        if expected is None or digest(path) != expected:
            raise ValueError('Selected GUI/CLI no longer matches its candidate receipt.')
    runtime_pins = pins['pins']
    if not isinstance(runtime_pins, dict) or len(runtime_pins) > 1024:
        raise ValueError('Runtime artifact inventory exceeds its bound.')
    total_bytes = 0
    for selected_path, expected in runtime_pins.items():
        path = Path(selected_path)
        if not path.is_absolute() or not path.is_file() or not DIGEST.fullmatch(expected):
            raise ValueError('Invalid runtime artifact pin.')
        total_bytes += path.stat().st_size
        if total_bytes > 4 << 30:
            raise ValueError('Runtime artifact inventory exceeds its total byte bound.')
        if digest(path) != expected:
            raise ValueError('A selected runtime dependency changed after planning.')
    baseline = json.loads(selections['baseline_receipt'].read_text())
    for artifact in baseline['artifacts']:
        path = regular_absolute(artifact['path'], 256 << 20)
        if path.stat().st_size != artifact['bytes'] or digest(path) != artifact['sha256']:
            raise ValueError('Published baseline artifact changed.')
    return {'host_gui': pins['gui'], 'host_cli': pins['cli'], 'host_store': pins['store'],
            'media_sha256': configuration['media_sha256'], 'baseline_source': baseline['source'],
            'baseline_distinct_from_host_candidate': True,
            'resources': resource_admission(output), 'guest_started': False}


def execute(request, output):
    eligible = any(case['id'] in SUPPORTED_CASES and case['scenario'] == 'happy'
                   and case['layer'] == 'installed_e2e' for case in request['cases'])
    if eligible:
        facts = admit_configuration(request['configuration'], output)
        baseline = json.loads(Path(request['configuration']['baseline_receipt']).read_text())
        if request['target']['artifact_sha256'] not in {artifact['sha256'] for artifact in baseline['artifacts']
                                                       if artifact['path'].endswith('.deb')}:
            raise ValueError('Target artifact must identify one admitted published baseline DEB.')
    else:
        facts = {'guest_started': False, 'target_observed': False}
    facts['target_role'] = 'guest-product'
    facts['host_provisioning_is_prerequisite'] = True
    admission = output / 'admission.json'
    admission.write_text(json.dumps(facts, indent=2) + '\n')
    evidence = [{'path': admission.name, 'sha256': digest(admission)}]
    observed = {}
    lifecycle_failure = None
    reason = 'No supported guest-product case selected; prerequisites and target were not observed.'
    if eligible:
        reason = 'Prerequisites admitted; this admission-only invocation did not start a guest.'
    if request['configuration'].get('operation') == 'guided-install' and eligible:
        from lifecycle import run
        configuration = request['configuration']
        pins = json.loads(Path(configuration['pins_file']).read_text())
        observed, lifecycle_evidence, failure = run(request, output, pins,
            Path(configuration['media_file']), Path(configuration['baseline_receipt']),
            configuration.get('timeout_seconds', 3600))
        evidence.extend(lifecycle_evidence)
        facts.pop('guest_started', None)
        facts['lifecycle_execution_requested'] = True
        facts['lifecycle_completed'] = failure is None
        lifecycle_failure = failure
        reason = failure or 'Observed fresh normal-GUI lifecycle; unsupported assertions remain pending.'
    cases = []
    for requested in request['cases']:
        supported = (requested['id'] in SUPPORTED_CASES and requested['scenario'] == 'happy'
                     and requested['layer'] == 'installed_e2e')
        assertions = [{'id': assertion,
                       'status': 'passed' if supported and observed.get(assertion) is True else 'failed',
                       'detail': 'Observed by this invocation.' if supported and assertion in observed
                                 else 'Not observed by this invocation.'}
                      for assertion in requested['required_assertions']]
        passed = bool(assertions) and all(item['status'] == 'passed' for item in assertions)
        failed = supported and any(observed.get(assertion) is False
                                   for assertion in requested['required_assertions'])
        status = 'manual_pending'
        case_reason = reason
        if passed:
            status = 'passed'
        elif failed:
            status = 'failed'
        elif requested['id'] == 'vm.install-linux':
            status = 'blocked'
            case_reason = ('This target identifies the installed Ubuntu guest product. '
                           'VM installation executes in the separately pinned host GUI and cannot '
                           'qualify the guest artifact or OS. A host-bound lane is required.')
        cases.append({'id': requested['id'], 'scenario': requested['scenario'], 'layer': requested['layer'],
            'status': status,
            'reason': case_reason,
            'assertions': assertions,
            'evidence': evidence})
    return {'schema': 'dockpipe.product-integration.lane/v1', 'run_id': request['run_id'],
            'plan_sha256': request['plan_sha256'], 'registry_sha256': request['registry_sha256'],
            'target': request['target'], 'observed_target': facts, 'cases': cases,
            'status': 'passed' if not lifecycle_failure and cases
                      and all(case['status'] == 'passed' for case in cases) else 'blocked'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--request', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    if not args.output.is_absolute() or args.output.exists():
        parser.error('--output must be an absent absolute invocation directory.')
    request = load_request(args.request)
    args.output.mkdir(mode=0o700, parents=True)
    result = execute(request, args.output)
    (args.output / 'result.json').write_text(json.dumps(result, indent=2) + '\n')
    if result['observed_target'].get('lifecycle_completed') is False:
        return 2

    return 0


if __name__ == '__main__':
    sys.exit(main())
