"""Run the real platform script with a controlled compiler."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest



class BuildPlatformFreshStoreTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        repository = Path(__file__).resolve().parents[4]
        inputs = (
            'workflows/build/platform/assets/scripts/prepare.py',
            'workflows/build/platform/assets/scripts/run.py',
            'workflows/build/platform/assets/scripts/run-phase.py',
            'workflows/build/platform/assets/scripts/assemble.py',
            'workflows/build/cli/assets/scripts/build.py',
            'workflows/release/metadata/assets/scripts/release-artifacts.py',
        )
        for relative in inputs:
            destination = self.root / relative
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(repository / relative, destination)
        (self.root / 'VERSION').write_text('0.6.0\n')
        tools = self.root / 'tools'
        tools.mkdir()
        cli = tools / 'fixture-cli'
        cli.write_text('''#!/usr/bin/env python3
import hashlib
import json
from pathlib import Path
import sys
args = sys.argv[1:]
if args == ['--version']:
    print('0.6.27')
else:
    raise SystemExit('Unexpected controlled CLI invocation')
''')
        cli.chmod(0o755)
        go = tools / 'go'
        go.write_text('''#!/usr/bin/env python3
import json
import os
from pathlib import Path
import shutil
import sys
args = sys.argv[1:]
if args[:1] == ['env']:
    for name in args[1:]:
        print({'GOHOSTOS': 'darwin', 'GOHOSTARCH': 'arm64', 'GOEXE': ''}[name])
elif args[:1] == ['build']:
    Path(os.environ['BUILD_MARKER']).write_text(json.dumps({name: os.environ.get(name) for name in ('CGO_ENABLED', 'GOOS', 'GOARCH')}))
    shutil.copy2(Path(__file__).with_name('fixture-cli'), Path(args[args.index('-o') + 1]))
else:
    raise SystemExit('Unexpected controlled Go invocation')
''')
        go.chmod(0o755)
        self.environment = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ['PATH'],
                                BUILD_MARKER=str(self.root / 'build-called'), GOOS='windows', GOARCH='amd64', CGO_ENABLED='1')
        self.store = self.root / 'release/artifacts/stores/darwin-arm64'

    def run_build(self):
        environment = dict(self.environment, DOCKPIPE_SOURCE_ROOT=str(self.root))
        return subprocess.run(
            ['python3', 'workflows/build/platform/assets/scripts/run.py', '0.6.27'],
            cwd=self.root, env=environment, capture_output=True, text=True, timeout=20)

    def test_fresh_tree_builds_standalone_cli_without_catalog_or_source_mutation(self):
        result = self.run_build()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertFalse(self.store.exists())
        self.assertFalse((self.root / 'release/artifacts/dockpipe-packages_0.6.27_darwin-arm64.tar.gz').exists())
        self.assertTrue((self.root / 'release/artifacts/dockpipe_0.6.27_darwin_arm64.tar.gz').is_file())
        self.assertEqual((self.root / 'VERSION').read_text(), '0.6.0\n')
        compiler = json.loads((self.root / 'build-called').read_text())
        self.assertEqual(compiler, {'CGO_ENABLED': '0', 'GOOS': None, 'GOARCH': None})

    def test_explicit_optional_catalog_is_verified_and_archived(self):
        catalog = self.root / 'catalog'
        catalog.mkdir()
        (catalog / 'packages-store-manifest.json').write_text(json.dumps({
            'schema': 1, 'packages': {'resolvers': [], 'workflows': []}}))
        self.environment['DOCKPIPE_RELEASE_STORE'] = str(catalog)
        result = self.run_build()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertTrue((self.store / 'packages-store-manifest.json').is_file())
        self.assertTrue((self.root / 'release/artifacts/dockpipe-packages_0.6.27_darwin-arm64.tar.gz').is_file())

    def test_existing_leaf_is_refused_before_build_and_preserved(self):
        self.store.mkdir(parents=True)
        sentinel = self.store / 'keep'
        sentinel.write_bytes(b'preserve')
        result = self.run_build()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Final store must be fresh', result.stdout + result.stderr)
        self.assertEqual(sentinel.read_bytes(), b'preserve')
        self.assertFalse((self.root / 'build-called').exists())
        self.assertEqual((self.root / 'VERSION').read_text(), '0.6.0\n')

    def test_symlink_leaf_is_refused_without_following_it(self):
        target = self.root / 'unrelated'
        target.mkdir()
        (target / 'keep').write_bytes(b'preserve')
        self.store.parent.mkdir(parents=True)
        self.store.symlink_to(target, target_is_directory=True)
        result = self.run_build()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Final store must be fresh', result.stdout + result.stderr)
        self.assertEqual((target / 'keep').read_bytes(), b'preserve')
        self.assertFalse((self.root / 'build-called').exists())

    def test_unrelated_old_core_leaf_is_preserved_and_not_consumed(self):
        core = self.root / 'release/artifacts/core-stores/darwin-arm64'
        core.mkdir(parents=True)
        (core / 'keep').write_bytes(b'preserve')
        result = self.run_build()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual((core / 'keep').read_bytes(), b'preserve')
        self.assertFalse(self.store.exists())



if __name__ == '__main__':
    unittest.main()
