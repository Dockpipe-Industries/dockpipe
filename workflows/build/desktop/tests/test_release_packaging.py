"""Exercise release adapter arguments and Docker build context without building."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[4]
SCRIPTS = Path('workflows/build/desktop/assets/scripts')
BASH = shutil.which('bash')


@unittest.skipUnless(BASH and os.name == 'posix', 'POSIX release adapter contracts')
class ReleasePackagingTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix='release packaging ')
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.tools = self.directory / 'tools'
        self.tools.mkdir()
        self.log = self.directory / 'arguments.json'
        self.environment = dict(os.environ)
        self.environment.update(
            PATH=str(self.tools) + os.pathsep + os.environ['PATH'],
            ARGUMENT_LOG=str(self.log),
        )
        self.write_tool('python3', '#!/bin/sh\nexit 0\n')

    def write_tool(self, name, content):
        path = self.tools / name
        path.write_text(content)
        path.chmod(0o755)

    def test_macos_release_preserves_exact_arguments_for_all_flag_combinations(self):
        destination = self.directory / SCRIPTS
        destination.mkdir(parents=True)
        shutil.copy2(ROOT / SCRIPTS / 'release-macos.sh', destination)
        capture = self.directory / 'capture.py'
        capture.write_text(
            'import json, os, pathlib, sys\n'
            'pathlib.Path(os.environ["ARGUMENT_LOG"]).write_text(json.dumps(sys.argv[1:]))\n'
        )
        (destination / 'build-unix.sh').write_text(
            '#!/usr/bin/env bash\nset -euo pipefail\n'
            'exec "$CAPTURE_PYTHON" "$CAPTURE_SCRIPT" "$@"\n'
        )
        for qualify in (False, True):
            for diagnose in (False, True):
                with self.subTest(qualify=qualify, diagnose=diagnose):
                    environment = dict(self.environment)
                    environment.update(
                        QUALIFY_PACKAGE_DISPLAY=str(qualify).lower(),
                        DIAGNOSE_LAYOUT=str(diagnose).lower(),
                        PRODUCT_SOURCE='a' * 40,
                        DOCKPIPE_ADAPTER_0='0.6.31',
                        CAPTURE_PYTHON=sys.executable,
                        CAPTURE_SCRIPT=str(capture),
                    )
                    result = subprocess.run(
                        [BASH, str(SCRIPTS / 'release-macos.sh')],
                        cwd=self.directory, env=environment,
                        capture_output=True, text=True, timeout=15,
                    )
                    self.assertEqual(result.returncode, 0, result.stderr)
                    expected = []
                    if qualify:
                        expected.append('--enable-qualification')
                    if diagnose:
                        expected.extend(['--diagnose-layout', 'a' * 40])
                    expected.extend(['0.6.31', 'release/artifacts'])
                    self.assertEqual(json.loads(self.log.read_text()), expected)

    def test_linux_builder_copy_inputs_exist_in_the_actual_build_context(self):
        self.write_tool('docker', '#!' + sys.executable + '\n'
                        'import json, os, pathlib, sys\n'
                        'pathlib.Path(os.environ["ARGUMENT_LOG"]).write_text(json.dumps(sys.argv[1:]))\n'
                        'sys.exit(39)\n')
        for architecture, ubuntu in (('x86_64', '22.04'), ('aarch64', '24.04')):
            with self.subTest(architecture=architecture):
                self.write_tool('uname', '#!/bin/sh\necho ' + architecture + '\n')
                environment = dict(self.environment, DOCKPIPE_BROWSER_SDK=str(self.directory))
                result = subprocess.run(
                    [BASH, str(ROOT / SCRIPTS / 'build-linux.sh'), '0.6.31',
                     str(self.directory / 'artifacts')],
                    cwd=self.directory, env=environment,
                    capture_output=True, text=True, timeout=15,
                )
                self.assertEqual(result.returncode, 39, result.stderr)
                arguments = json.loads(self.log.read_text())
                self.assertEqual(arguments[0], 'build')
                self.assertIn('UBUNTU_VERSION=' + ubuntu, arguments)
                context = Path(arguments[-1])
                self.assertEqual(context, ROOT / 'workflows/build/desktop/assets')
                dockerfile = Path(arguments[arguments.index('--file') + 1])
                copy_sources = [line.split()[1] for line in dockerfile.read_text().splitlines()
                                if line.startswith('COPY ')]
                self.assertEqual(copy_sources, ['scripts/install-linux-qt.sh'])
                for source in copy_sources:
                    self.assertTrue((context / source).is_file(), source)


if __name__ == '__main__':
    unittest.main()
