"""Run inside the newly installed guest, through its private SSH standard input."""
from pathlib import Path
import json
import os
import subprocess
import sys


def main():
    root = Path(sys.argv[1])
    root.mkdir(mode=0o700)
    workspace = root / 'workspace with spaces'
    workspace.mkdir(mode=0o700)
    environment = {key: os.environ[key] for key in ['PATH', 'LANG', 'LC_ALL'] if key in os.environ}
    environment.update(HOME=str(root / 'home'), DOCKPIPE_INTERACTIVE='0')
    for name in ['home', 'state', 'config', 'cache', 'data']:
        directory = root / name
        directory.mkdir(mode=0o700, exist_ok=True)
        if name != 'home':
            environment['XDG_' + name.upper() + '_HOME'] = str(directory)
    records = []

    def run(arguments):
        result = subprocess.run(arguments, cwd=workspace, env=environment, capture_output=True, timeout=90)
        records.append({'arguments': arguments, 'returncode': result.returncode,
                        'stdout': result.stdout.decode(errors='replace')[:1048576],
                        'stderr': result.stderr.decode(errors='replace')[:1048576]})
        return result.returncode == 0

    version = run(['/usr/bin/dockpipe', '--version'])
    initialized = run(['/usr/bin/dockpipe', 'init']) and (workspace / 'dockpipe.config.json').is_file()
    workflow = workspace / 'workflows/product-proof'
    workflow.mkdir(parents=True)
    (workflow / 'config.yml').write_text('''name: product-proof
vault: none
docker_preflight: false
steps:
  - id: proof
    kind: host
    cwd: repo
    run: proof.sh
''')
    (workflow / 'proof.sh').write_text('''set -eu
test -f dockpipe.config.json
printf '%s' 'dockpipe-guest-host-proof-v1' > host-result.txt
''')
    executed = run(['/usr/bin/dockpipe', '--workflow-file', str(workflow / 'config.yml')])
    marker = workspace / 'host-result.txt'
    executed = executed and marker.is_file() and marker.read_text() == 'dockpipe-guest-host-proof-v1'
    print(json.dumps({'fixture': str(root), 'installed-cli-version': version,
                      'initialized-project': initialized, 'host-workflow-executed': executed,
                      'commands': records}))


if __name__ == '__main__':
    main()
