"""Run the real platform script with a controlled compiler/core producer."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

PACKAGING = Path(__file__).resolve().parents[1]


class BuildPlatformFreshStoreTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        packaging = self.root / 'release/packaging'
        packaging.mkdir(parents=True)
        for name in ('build-platform.sh', 'release-artifacts.py'):
            shutil.copy2(PACKAGING / name, packaging / name)
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
elif args[:3] == ['package', 'compile', 'core']:
    pass
elif args[:3] == ['package', 'build', 'store']:
    destination = Path(args[args.index('--out') + 1])
    destination.mkdir(parents=True)
    payload = b'controlled compiled core fixture'
    name = 'dockpipe-core-0.6.27.tar.gz'
    (destination / name).write_bytes(payload)
    entry = {'name': 'dockpipe.core', 'version': '0.6.27', 'tarball': name,
             'sha256': hashlib.sha256(payload).hexdigest(), 'size_bytes': len(payload)}
    manifest = {'schema': 1, 'platform': 'darwin-arm64',
                'packages': {'core': entry, 'workflows': [], 'resolvers': []}}
    (destination / 'packages-store-manifest.json').write_text(json.dumps(manifest))
else:
    raise SystemExit('Unexpected controlled CLI invocation')
''')
        cli.chmod(0o755)
        go = tools / 'go'
        go.write_text('''#!/usr/bin/env python3
import os
from pathlib import Path
import shutil
import sys
args = sys.argv[1:]
if args[:1] == ['env']:
    print({'GOHOSTOS': 'darwin', 'GOHOSTARCH': 'arm64', 'GOEXE': ''}[args[1]])
elif args[:1] == ['build']:
    Path(os.environ['BUILD_MARKER']).write_text('compiler called')
    shutil.copy2(Path(__file__).with_name('fixture-cli'), Path(args[args.index('-o') + 1]))
else:
    raise SystemExit('Unexpected controlled Go invocation')
''')
        go.chmod(0o755)
        self.environment = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ['PATH'],
                                BUILD_MARKER=str(self.root / 'build-called'))
        self.store = self.root / 'release/artifacts/stores/darwin-arm64'

    def run_build(self):
        return subprocess.run(['/bin/bash', str(self.root / 'release/packaging/build-platform.sh'), '0.6.27'],
                              cwd=self.root, env=self.environment, capture_output=True, text=True, timeout=20)

    def test_fresh_tree_builds_core_only_final_store_and_restores_version(self):
        self.assertFalse(self.store.parent.exists())
        result = self.run_build()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        manifest = json.loads((self.store / 'packages-store-manifest.json').read_text())
        self.assertEqual(manifest['packages']['resolvers'], [])
        self.assertEqual(manifest['packages']['workflows'], [])
        self.assertEqual(manifest['packages']['core']['version'], '0.6.27')
        self.assertTrue((self.root / 'release/artifacts/dockpipe-packages_0.6.27_darwin-arm64.tar.gz').is_file())
        self.assertTrue((self.root / 'release/artifacts/dockpipe_0.6.27_darwin_arm64.tar.gz').is_file())
        self.assertEqual((self.root / 'VERSION').read_text(), '0.6.0\n')

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

    def test_existing_core_leaf_remains_refused_before_build(self):
        core = self.root / 'release/artifacts/core-stores/darwin-arm64'
        core.mkdir(parents=True)
        (core / 'keep').write_bytes(b'preserve')
        result = self.run_build()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Core store must be fresh', result.stdout + result.stderr)
        self.assertEqual((core / 'keep').read_bytes(), b'preserve')
        self.assertFalse((self.root / 'build-called').exists())


if __name__ == '__main__':
    unittest.main()
