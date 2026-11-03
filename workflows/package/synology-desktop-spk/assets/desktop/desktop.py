#!/usr/bin/env python3
"""Start the pinned Selkies desktop as DSM's unprivileged package identity."""

import os
from pathlib import Path
import subprocess
import sys


SETTINGS = Path('/run/dockpipe-package/desktop-settings')
DATA = Path('/config')


def prepare_identity(runtime):
    uid = os.getuid()
    gid = os.getgid()
    if uid == 0 or gid == 0:
        raise ValueError('The desktop must run as the unprivileged DSM package account.')
    if (SETTINGS / 'uid').read_text() != str(uid) or (SETTINGS / 'gid').read_text() != str(gid):
        raise ValueError('The container identity does not match the DSM package account.')
    if runtime.is_symlink():
        raise ValueError('The desktop runtime directory must not be a symbolic link.')
    runtime.mkdir(mode=0o700, exist_ok=True)

    # DSM assigns an arbitrary UID which is absent from the image's account files.
    # NSS wrapper supplies a process-local account without editing /etc or changing IDs.
    passwd = [line for line in Path('/etc/passwd').read_text().splitlines()
              if line.split(':')[0] != 'dockpipe' and line.split(':')[2] != str(uid)]
    groups = [line for line in Path('/etc/group').read_text().splitlines()
              if line.split(':')[0] != 'dockpipe' and line.split(':')[2] != str(gid)]
    passwd.append(f'dockpipe:x:{uid}:{gid}:Dockpipe:/config:/bin/bash')
    groups.append(f'dockpipe:x:{gid}:')
    for name, lines in [('passwd', passwd), ('group', groups)]:
        (runtime / name).write_text('\n'.join(lines) + '\n')
    os.environ['NSS_WRAPPER_PASSWD'] = str(runtime / 'passwd')
    os.environ['NSS_WRAPPER_GROUP'] = str(runtime / 'group')
    os.environ['LD_PRELOAD'] = 'libnss_wrapper.so'
    os.environ['HOME'] = str(DATA)
    os.environ['USER'] = 'dockpipe'
    os.environ['LOGNAME'] = 'dockpipe'
    os.environ['XDG_RUNTIME_DIR'] = str(runtime)


def prepare_tls():
    directory = DATA / 'ssl'
    directory.mkdir(mode=0o700, exist_ok=True)
    certificate = directory / 'cert.pem'
    key = directory / 'cert.key'
    if certificate.exists() != key.exists():
        raise ValueError('The desktop TLS certificate/key pair is incomplete.')
    if not certificate.exists():
        subprocess.run(
            ['openssl', 'req', '-new', '-x509', '-newkey', 'rsa:2048', '-days', '3650',
             '-nodes', '-out', str(certificate), '-keyout', str(key), '-subj', '/CN=Dockpipe'],
            check=True, timeout=30, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE,
        )
    key.chmod(0o600)
    return certificate, key


def main():
    os.umask(0o077)
    runtime = DATA / '.dockpipe-runtime'
    prepare_identity(runtime)
    certificate, key = prepare_tls()

    # Only the child process receives the password; neither argv nor Docker's
    # persisted container environment contains the resolved credential.
    os.environ['SELKIES_BASIC_AUTH_USER'] = (SETTINGS / 'username').read_text()
    os.environ['SELKIES_BASIC_AUTH_PASSWORD'] = (SETTINGS / 'password').read_text()
    arguments = [
        '/lsiopy/bin/selkies', 'session', '--session', '/usr/local/bin/dockpipe-desktop-session',
        '--public', '--port', '3001', '--enable-https=true',
        '--https-cert', str(certificate), '--https-key', str(key),
        '--enable-basic-auth=true', '--allowed-origins', '',
        '--subfolder', '/webman/3rdparty/dockpipe-desktop/desktop',
        '--frame-ancestors', 'self',
        '--ui-title', 'Dockpipe', '--ui-show-logo=false',
        '--web-root', '/usr/share/selkies/selkies-dashboard',
    ]
    os.execv(arguments[0], arguments)


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        print(f'Dockpipe desktop startup failed: {error}', file=sys.stderr)
        sys.exit(1)
