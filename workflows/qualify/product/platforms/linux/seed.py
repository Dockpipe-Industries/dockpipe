"""Private NoCloud fixture inputs for one new, disposable Ubuntu guest."""
from pathlib import Path
import hashlib
import json
import os
import secrets
import subprocess


def prepare(directory, run_id):
    password = secrets.token_urlsafe(24)
    password_hash = subprocess.run(
        ['/usr/bin/openssl', 'passwd', '-6', '-stdin'], input=password + '\n',
        text=True, capture_output=True, check=True, timeout=10,
    ).stdout.strip()
    if not password_hash.startswith('$6$') or '\n' in password_hash:
        raise ValueError('OpenSSL did not return a SHA-512 password hash')

    directory = Path(directory)
    directory.mkdir(mode=0o700, parents=True, exist_ok=False)
    key = directory / 'guest-key'
    subprocess.run(['/usr/bin/ssh-keygen', '-q', '-t', 'ed25519', '-N', '', '-f', str(key)],
                   check=True, timeout=10)
    public_key = key.with_suffix('.pub').read_text().strip()
    token = secrets.token_hex(24)
    metadata = {'instance-id': run_id, 'local-hostname': 'dockpipe-qualification'}
    (directory / 'meta-data').write_text(json.dumps(metadata) + '\n')
    config = {'autoinstall': {'version': 1, 'locale': 'en_US.UTF-8',
        'keyboard': {'layout': 'us'},
        'identity': {'hostname': 'dockpipe-qualification', 'username': 'dockpipe', 'password': password_hash},
        'ssh': {'install-server': True, 'allow-pw': False, 'authorized-keys': [public_key]},
        'storage': {'layout': {'name': 'direct'}},
        'updates': 'security',
        'late-commands': [
            'curtin in-target --target=/target -- mkdir -p /var/lib/dockpipe-qualification',
            "printf '%s\\n' '" + json.dumps({'run_id': run_id, 'token': token})
            + "' > /target/var/lib/dockpipe-qualification/install-identity.json"
        ],
        'shutdown': 'reboot'}}
    # JSON is a YAML subset; cloud-init still requires the explicit magic header.
    (directory / 'user-data').write_text('#cloud-config\n' + json.dumps(config, indent=2) + '\n')
    private = {'run_id': run_id, 'username': 'dockpipe', 'password': password, 'token': token,
               'private_key': str(key)}
    (directory / 'fixture-private.json').write_text(json.dumps(private) + '\n')
    for path in directory.iterdir():
        if path.is_file():
            path.chmod(0o600)
    return {'run_id': run_id, 'seed_directory': str(directory), 'private_key': str(key),
            'user_data_sha256': hashlib.sha256((directory / 'user-data').read_bytes()).hexdigest()}
