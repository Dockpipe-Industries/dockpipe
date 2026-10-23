"""Reject incompatible installed Display SDK candidates through a real consumer."""
import argparse
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

parser = argparse.ArgumentParser()
parser.add_argument('--sdk', type=Path, required=True)
parser.add_argument('--qt', type=Path, required=True)
arguments = parser.parse_args()
consumer = Path(__file__).resolve().parent / 'consumer'
with tempfile.TemporaryDirectory(prefix='dockpipe-display-sdk-compatibility-') as temporary:
    root = Path(temporary)
    for field, replacement in (
        ('COMPILER_ID', 'incompatible'),
        ('COMPILER_VERSION', '0.0.0'),
        ('SYSTEM_NAME', 'unsupported'),
        ('SYSTEM_PROCESSOR', 'unsupported'),
    ):
        sdk = root / field / 'sdk'
        shutil.copytree(arguments.sdk, sdk)
        config = sdk / 'lib/cmake/DockPipeDisplaySDK/DockPipeDisplaySDKConfig.cmake'
        source = config.read_text()
        source, changes = re.subn(r'set\(DockPipeDisplaySDK_' + field + r' "[^"]*"\)',
                                  'set(DockPipeDisplaySDK_' + field + ' "' + replacement + '")', source)
        if changes != 1:
            raise ValueError('Installed SDK does not declare exactly one compatibility field')
        config.write_text(source)
        result = subprocess.run(['cmake', '-S', str(consumer), '-B', str(root / field / 'build'),
                                 '-DDockPipeDisplaySDK_DIR=' + str(config.parent),
                                 '-DCMAKE_PREFIX_PATH=' + str(arguments.qt)],
                                text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=60)
        if result.returncode == 0 or 'requires' not in result.stdout:
            raise ValueError('Consumer did not reject incompatible SDK ' + field)
        print('Rejected incompatible ' + field)
