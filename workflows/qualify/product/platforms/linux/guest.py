"""SSH observations restricted to the new qualification guest's private identity."""
from pathlib import Path
import json
import shlex
import subprocess

from desktop import digest


class Guest:
    def __init__(self, private_file, port, known_hosts):
        private_file = Path(private_file)
        if not private_file.is_absolute() or private_file.is_symlink() or private_file.stat().st_mode & 0o077:
            raise ValueError('Guest identity must be an explicit owner-private file.')
        self.identity = json.loads(private_file.read_text())
        self.port = int(port)
        if self.port < 1024 or self.port > 65535:
            raise ValueError('A bounded explicit guest loopback port is required.')
        self.known_hosts = Path(known_hosts)
        self.options = ['-F', '/dev/null', '-i', self.identity['private_key'], '-o', 'IdentitiesOnly=yes',
                        '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=5',
                        '-o', 'StrictHostKeyChecking=accept-new',
                        '-o', 'UserKnownHostsFile=' + str(self.known_hosts)]

    def command(self, command, options=()):
        return ['/usr/bin/ssh', *self.options, '-p', str(self.port),
                *options, self.identity['username'] + '@127.0.0.1', command]

    def run(self, command, timeout=30, input_data=None):
        return subprocess.run(self.command(command), input=input_data, capture_output=True, timeout=timeout)

    def start(self, command, options=()):
        return subprocess.Popen(self.command(command, options), stdin=subprocess.DEVNULL,
                                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

    def observe_install(self):
        command = ('cat /etc/os-release; printf "\\n---IDENTITY---\\n"; '
                   'cat /var/lib/dockpipe-qualification/install-identity.json; '
                   'printf "\\n---RUNTIME---\\n"; uname -m; cat /proc/sys/kernel/random/boot_id')
        result = self.run(command)
        if result.returncode != 0:
            return None
        release, marker = result.stdout.decode().split('\n---IDENTITY---\n', 1)
        marker, runtime = marker.split('\n---RUNTIME---\n', 1)
        architecture, boot_id = runtime.strip().splitlines()
        if architecture != 'x86_64':
            raise RuntimeError('Installed guest architecture differs from the selected baseline.')
        identity = json.loads(marker)
        if identity != {'run_id': self.identity['run_id'], 'token': self.identity['token']}:
            raise RuntimeError('Installed guest marker differs from the fresh fixture identity.')
        fields = {}
        for line in release.splitlines():
            if '=' in line:
                key, value = line.split('=', 1)
                fields[key] = value.strip('"')
        if fields.get('ID') != 'ubuntu' or fields.get('VERSION_ID') != '24.04':
            raise RuntimeError('Installed guest OS differs from the selected baseline.')
        return {'os_release': fields, 'fixture_identity_verified': True,
                'run_id': identity['run_id'], 'ssh_port': self.port,
                'architecture': architecture, 'boot_id': boot_id}

    def install_baseline(self, receipt_file, output):
        if self.observe_install() is None:
            raise RuntimeError('Fresh guest identity must be verified before package installation.')
        output = Path(output)
        receipt = json.loads(Path(receipt_file).read_text())
        packages = [artifact for artifact in receipt['artifacts'] if artifact['path'].endswith('.deb')]
        if len(packages) != 2:
            raise ValueError('Exactly the admitted CLI and desktop DEBs are required.')

        self.install_packages(packages, '/tmp/dockpipe-published-baseline', output / 'published-baseline-install.log')
        version = self.run('dockpipe --version; dpkg-query -W -f='
                           + shlex.quote('${Package} ${Version}\\n') + ' dockpipe dockpipe-desktop')
        if version.returncode != 0:
            raise RuntimeError('Installed guest product identity could not be read back.')
        return {'source': receipt['source'], 'edition': 'published-staging-baseline',
                'version_output': version.stdout.decode(), 'distinct_from_host_candidate': True,
                'artifacts': [{key: artifact[key] for key in ['sha256', 'bytes']} for artifact in packages]}

    def install_enrollment_product(self, packages, output):
        if self.observe_install() is None:
            raise RuntimeError('Fresh guest identity must be verified before the enrollment upgrade.')

        versions = []
        for artifact, name in zip(packages, ('dockpipe', 'dockpipe-desktop'), strict=True):
            fields = subprocess.check_output(['/usr/bin/dpkg-deb', '-f', artifact['path'],
                                              'Package', 'Architecture', 'Version'], timeout=15)
            package, architecture, version = fields.decode().splitlines()
            if package != f'Package: {name}' or architecture != 'Architecture: amd64':
                raise ValueError('Guest enrollment requires matching Dockpipe CLI and desktop amd64 DEBs.')

            versions.append(version.removeprefix('Version: '))

        if versions[0] != versions[1] or Path(packages[0]['path']).name == Path(packages[1]['path']).name:
            raise ValueError('Guest enrollment requires distinct CLI and desktop DEBs at the same version.')

        version = shlex.quote(versions[0])
        upgrade = self.run('installed=$(dpkg-query -W -f=\'${Version}\' dockpipe) && '
                           f'dpkg --compare-versions {version} gt "$installed"')
        if upgrade.returncode != 0:
            raise ValueError('The worker DEB must upgrade the installed baseline CLI.')

        remote = '/tmp/dockpipe-enrollment-product'
        self.install_packages(packages, remote, Path(output) / 'guest-enrollment-install.log')
        worker_deb = shlex.quote(remote + '/' + Path(packages[0]['path']).name)
        desktop_deb = shlex.quote(remote + '/' + Path(packages[1]['path']).name)
        result = self.run('set -eu; '
                          f'dpkg-deb -x {worker_deb} {remote}/worker; '
                          f'dpkg-deb -x {desktop_deb} {remote}/desktop; '
                          f'cmp {remote}/worker/usr/bin/dockpipe /usr/bin/dockpipe; '
                          f'cmp {remote}/desktop/usr/lib/dockpipe/bin/dockpipe-launcher /usr/bin/dockpipe-launcher; '
                          f'test {version} = "$(dpkg-query -W -f=\'${{Version}}\' dockpipe-desktop)"; '
                          f'test {version} = '
                          '"$(dpkg-query -W -f=\'${Version}\' dockpipe)"; '
                          'sha256sum /usr/bin/dockpipe /usr/bin/dockpipe-launcher; /usr/bin/dockpipe --version',
                          timeout=180)
        if result.returncode != 0:
            raise RuntimeError('Installed guest product differs from the selected enrollment DEBs.')

        worker, desktop, version = result.stdout.decode().split('\n', 2)
        return {'worker_deb_sha256': packages[0]['sha256'], 'desktop_deb_sha256': packages[1]['sha256'],
                'cli_sha256': worker.split()[0], 'desktop_sha256': desktop.split()[0], 'package_version': versions[0],
                'version_output': version.strip()}

    def install_packages(self, packages, remote, log):
        setup = self.run('mkdir -m 700 ' + shlex.quote(remote))
        if setup.returncode != 0:
            raise RuntimeError('Guest package destination is not fresh; reconcile before retrying.')
        for artifact in packages:
            path = Path(artifact['path'])
            if digest(path) != artifact['sha256']:
                raise ValueError('Host package artifact changed before transfer.')
            transfer = subprocess.run(['/usr/bin/scp', *self.options, '-P', str(self.port), str(path),
                self.identity['username'] + '@127.0.0.1:' + remote + '/' + path.name],
                capture_output=True, timeout=180)
            if transfer.returncode != 0:
                raise RuntimeError('Guest package transfer did not complete.')
            check = self.run('sha256sum ' + shlex.quote(remote + '/' + path.name))
            if check.returncode != 0 or check.stdout.decode().split()[0] != artifact['sha256']:
                raise RuntimeError('Transferred guest package digest mismatch.')
        package_paths = ' '.join(shlex.quote(remote + '/' + Path(artifact['path']).name) for artifact in packages)
        result = self.run('sudo -S -p "" env DEBIAN_FRONTEND=noninteractive apt-get install --no-remove -y ' + package_paths,
                          timeout=900, input_data=(self.identity['password'] + '\n').encode())
        log.write_bytes(result.stdout + result.stderr)
        log.chmod(0o600)
        if result.returncode != 0:
            raise RuntimeError('Guest package installation failed; exact guest/log retained for reconciliation.')

    def exercise_cli(self, output):
        if self.observe_install() is None:
            raise RuntimeError('Fresh installed guest identity is required before CLI qualification.')
        fixture = '/tmp/dockpipe-product-cli-' + self.identity['run_id']
        program = Path(__file__).with_name('guest_cli.py').read_text()
        result = self.run('python3 - ' + shlex.quote(fixture), timeout=300, input_data=program.encode())
        log = Path(output) / 'guest-cli-proof.json'
        if result.returncode != 0:
            (Path(output) / 'guest-cli-proof.failure.txt').write_bytes(result.stdout + result.stderr)
            raise RuntimeError('Installed guest CLI qualification failed; retain fixture for reconciliation.')
        observed = json.loads(result.stdout)
        log.write_text(json.dumps(observed, indent=2) + '\n')
        return observed
