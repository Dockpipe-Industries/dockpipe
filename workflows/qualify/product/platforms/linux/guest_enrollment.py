"""Run-scoped enrollment through the installed CLI and the owned guest transport."""
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
from urllib.parse import urlsplit


IDENTIFIER = re.compile(r'[A-Za-z0-9][A-Za-z0-9_-]{0,63}')
SHA256 = re.compile(r'[a-f0-9]{64}')
STATE = '$HOME/.local/state/dockpipe-qualification/worker'
REQUEST = '$HOME/.local/state/dockpipe-qualification/prepare.json'


def private_path(value, directory=False):
    path = Path(value)
    if not path.is_absolute() or path.resolve(strict=True) != path:
        raise ValueError('Guest enrollment requires canonical private paths.')

    info = path.stat()
    expected_kind = path.is_dir() if directory else path.is_file()
    if not expected_kind or info.st_uid != os.getuid() or info.st_mode & 0o077:
        raise ValueError('Guest enrollment requires owner-only inputs.')
    if not directory and info.st_size > 64 * 1024:
        raise ValueError('Guest enrollment JSON exceeds its byte bound.')

    return path


def expires(value):
    # The broker emits RFC 3339 with nanoseconds. Python 3.10 accepts
    # microseconds and a numeric UTC offset.
    normalized = re.sub(r'(\.\d{6})\d+(?=Z$|[+-]\d{2}:\d{2}$)', r'\1', value)
    if normalized.endswith('Z'):
        normalized = normalized[:-1] + '+00:00'

    expiry = datetime.fromisoformat(normalized)
    if expiry.tzinfo is None:
        raise ValueError('Expected a grant expiry with an explicit time zone.')

    return expiry


class GuestEnrollment:
    def __init__(self, selection, run_id, host_cli, output):
        if not isinstance(selection, dict) or set(selection) != {'grant_file', 'grant_sha256', 'host_state'}:
            raise ValueError('Select an exact guest grant receipt and paired host state.')

        grant_data = private_path(selection['grant_file']).read_bytes()
        if hashlib.sha256(grant_data).hexdigest() != selection['grant_sha256']:
            raise ValueError('Guest grant receipt differs from the selected digest.')

        grant = json.loads(grant_data)
        public_fields = {'id', 'host', 'run', 'expires_at', 'broker_key'}
        if not isinstance(grant, dict) or set(grant) != public_fields | {'guests'}:
            raise ValueError('Invalid broker guest grant receipt.')
        if not all(isinstance(grant[field], str) for field in public_fields):
            raise ValueError('Invalid guest grant value.')
        if type(grant['guests']) is not int or grant['guests'] != 1:
            raise ValueError('The guided installer requires a dedicated one-guest grant for teardown.')

        if (grant['run'] != run_id or len(run_id) > 48 or not IDENTIFIER.fullmatch(run_id)
                or not SHA256.fullmatch(grant['id']) or not IDENTIFIER.fullmatch(grant['host'])):
            raise ValueError('Guest grant must bind the exact qualification run and paired host.')

        host_state = private_path(selection['host_state'], directory=True)
        self.host_state = host_state
        self.host_cli = host_cli
        self.grant = grant
        self.node = run_id + '-vm'
        self.output = output
        self.worker = None
        self.enroll_attempted = False
        self.enrolled = False
        self.endpoint = self.read_host_endpoint()
        self.require_active()

    def read_host_endpoint(self):
        worker_path = private_path(self.host_state / 'worker.json')
        host = json.loads(worker_path.read_bytes())
        if (not isinstance(host, dict) or host.get('node') != self.grant['host']
                or host.get('guest_grant') or not host.get('machine_key')
                or not self.grant['broker_key'] or host.get('broker_key') != self.grant['broker_key']
                or not isinstance(host.get('endpoint'), str) or not host['endpoint']):
            raise ValueError('Paired host state differs from the selected broker grant.')

        return host['endpoint']

    def require_active(self):
        if expires(self.grant['expires_at']) <= datetime.now(timezone.utc):
            raise ValueError('The selected guest grant has expired.')

    def require_running(self):
        self.require_active()
        if self.worker is None or self.worker.poll() is not None:
            raise RuntimeError(f'The installed workflow worker for {self.node} exited before release.')

    def call_host(self, command, *arguments):
        if self.read_host_endpoint() != self.endpoint:
            raise ValueError('The selected paired host endpoint changed during the run.')

        result = subprocess.run([self.host_cli, 'remote', command, '--state', str(self.host_state), *arguments],
                                stdin=subprocess.DEVNULL, capture_output=True, timeout=30)
        if result.returncode != 0:
            raise RuntimeError(f'Host {command} failed. Reconcile the exact grant and node on the broker.')

        receipt = json.loads(result.stdout)
        if not isinstance(receipt, dict):
            raise ValueError('Broker returned an invalid guest enrollment receipt.')

        return receipt

    def start(self, guest):
        self.require_active()
        if guest.observe_install() is None:
            raise RuntimeError('Guest identity must be verified before delegated enrollment.')

        # The slot count is host-side policy. Keep the guest request compatible
        # with installed workers that decode the original grant shape strictly.
        guest_grant = {key: value for key, value in self.grant.items() if key != 'guests'}
        request = json.dumps({'endpoint': self.endpoint, 'node': self.node, 'grant': guest_grant}).encode()
        command = ('umask 077 && mkdir -p "${HOME}/.local/state" && '
                   'mkdir "${HOME}/.local/state/dockpipe-qualification" && '
                   f'cat > "{REQUEST}" && /usr/bin/dockpipe remote guest-prepare '
                   f'--state "{STATE}" --request "{REQUEST}"')
        prepared = guest.run(command, input_data=request)
        if prepared.returncode != 0:
            raise RuntimeError('Installed guest CLI could not prepare its workflow-worker identity.')

        claim = json.loads(prepared.stdout)
        required = {'node', 'grant', 'public_key', 'token_hash', 'proof'}
        if (not isinstance(claim, dict) or set(claim) != required or claim['node'] != self.node
                or claim['grant'] != self.grant['id']):
            raise RuntimeError('Guest claim did not bind the selected node and grant.')

        claim_path = self.output / 'guest-enrollment-claim.json'
        descriptor = os.open(claim_path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
        with os.fdopen(descriptor, 'w') as stream:
            json.dump(claim, stream)

        self.enroll_attempted = True
        receipt = self.call_host('guest-enroll', '--request', str(claim_path))
        if (set(receipt) != {'node', 'grant', 'expires_at'} or receipt['node'] != self.node
                or receipt['grant'] != self.grant['id'] or receipt['expires_at'] != self.grant['expires_at']):
            raise RuntimeError('Broker enrollment receipt did not match the selected node and grant.')

        self.enrolled = True
        command = f'exec /usr/bin/dockpipe remote worker --state "{STATE}"'
        origin = urlsplit(self.endpoint)
        if origin.scheme == 'http' and origin.hostname == '127.0.0.1' and origin.port:
            loopback = f'127.0.0.1:{origin.port}'
            self.worker = guest.start(command, ('-o', 'ExitOnForwardFailure=yes', '-R', f'{loopback}:{loopback}'))
        else:
            self.worker = guest.start(command)
        self.require_running()
        return {'node': self.node, 'host': self.grant['host'], 'grant': self.grant['id'],
                'expires_at': receipt['expires_at']}

    def stop(self):
        if not self.enroll_attempted:
            return None

        try:
            worker_running = self.worker is not None and self.worker.poll() is None
            receipt = self.call_host('guest-release', '--id', self.grant['id'])
            accepted_removals = ([self.node],) if self.enrolled else ([], [self.node])
            if (receipt.get('grant') != self.grant['id'] or receipt.get('removed') not in accepted_removals
                    or receipt.get('retained') != []):
                raise RuntimeError('The release receipt did not confirm exact guest removal.')

            if self.worker is not None:
                self.worker.wait(timeout=15)

            return {'node': self.node, 'grant': self.grant['id'], 'removed': receipt['removed'],
                    'worker_transport_running_at_release': worker_running,
                    'worker_transport_exited': self.worker is not None}
        finally:
            if self.worker is not None and self.worker.poll() is None:
                self.worker.kill()
                self.worker.wait(timeout=5)
