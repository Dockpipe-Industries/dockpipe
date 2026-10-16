"""Produced-CLI qualification detects skipped leaves and corrupted delivery."""
import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


spec = importlib.util.spec_from_file_location('compiler_bootstrap', Path(__file__).with_name('build.py'))
bootstrap = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bootstrap)


class CompilerQualificationTests(unittest.TestCase):
    def test_real_command_contract_and_rejected_deliveries(self):
        for scenario in ['valid', 'skipped', 'missing', 'hidden', 'minimum', 'checksum', 'payload', 'scope']:
            with self.subTest(scenario=scenario), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                cli = root / 'fixture-cli'
                cli.write_text('''#!/usr/bin/env python3
import hashlib
import io
import json
import os
from pathlib import Path
import sys
import tarfile

arguments = sys.argv[1:]
if arguments[:3] == ['package', 'compile', 'resolvers']:
    assert '--force' in arguments and '--from' in arguments
    sys.exit(0)
if arguments[:4] == ['scope', 'resolver', 'assets-only', 'assets-dir']:
    consumer = Path(arguments[arguments.index('--workdir') + 1])
    configuration = json.loads((consumer / 'dockpipe.config.json').read_text())
    source = configuration['packages']['sources'][0]
    assert source['kind'] == 'tarball_dir'
    package = Path(source['path']) / 'dockpipe-resolver-assets-only-1.0.0.tar.gz'
    with tarfile.open(package, 'r:gz') as archive:
        data = archive.extractfile('resolvers/assets-only/assets/payload.txt').read()
    assets = consumer / 'extracted-assets'
    assets.mkdir()
    if os.environ['FIXTURE_SCENARIO'] == 'scope':
        data = b'wrong scoped payload'
    (assets / 'payload.txt').write_bytes(data)
    print(assets)
    sys.exit(0)
assert arguments[:3] == ['package', 'build', 'store']
assert arguments[arguments.index('--only') + 1] == 'resolvers'
output = Path(arguments[arguments.index('--out') + 1])
output.mkdir()
scenario = os.environ['FIXTURE_SCENARIO']
archive = output / 'dockpipe-resolver-assets-only-1.0.0.tar.gz'
payload = b'asset-only bootstrap regression\\n'
if scenario == 'payload':
    payload = b'changed asset'
with tarfile.open(archive, 'w:gz') as package:
    member = tarfile.TarInfo('resolvers/assets-only/assets/payload.txt')
    member.size = len(payload)
    package.addfile(member, io.BytesIO(payload))
entry = {
    'name': 'assets-only', 'version': '1.0.0', 'tarball': archive.name,
    'hidden': scenario != 'hidden', 'min_dockpipe_version': '1.2.3',
    'sha256': hashlib.sha256(archive.read_bytes()).hexdigest(),
    'size_bytes': archive.stat().st_size,
}
if scenario == 'minimum':
    entry['min_dockpipe_version'] = '0.0.1'
if scenario == 'checksum':
    entry['sha256'] = '0' * 64
if scenario == 'missing':
    archive.unlink()
manifest = {'schema': 1, 'platform': 'linux-amd64',
            'packages': {'resolvers': [] if scenario == 'skipped' else [entry]}}
(output / 'packages-store-manifest.json').write_text(json.dumps(manifest))
''')
                cli.chmod(0o755)
                with patch.dict(os.environ, {'FIXTURE_SCENARIO': scenario}):
                    if scenario == 'valid':
                        bootstrap.verify_compiler(cli, '1.2.3', root / 'qualification')
                    else:
                        with self.assertRaises(ValueError):
                            bootstrap.verify_compiler(cli, '1.2.3', root / 'qualification')


if __name__ == '__main__':
    unittest.main()
