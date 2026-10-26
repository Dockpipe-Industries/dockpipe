#!/usr/bin/env python3
"""Archive a produced CLI and an explicitly selected, validated optional store."""
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import zipfile


def main():
    root = Path(os.environ['DOCKPIPE_SOURCE_ROOT']).resolve(strict=True)
    version = os.environ['PLATFORM_VERSION']
    system = os.environ['PLATFORM_SYSTEM']
    architecture = os.environ['PLATFORM_ARCH']
    binary = Path(os.environ['PLATFORM_BINARY'])
    output = Path(os.environ['PLATFORM_OUTPUT'])
    if binary.is_symlink() or not binary.is_file():
        raise ValueError('CLI input must be a regular file')
    subprocess.run([str(binary), '--version'], check=True)
    suffix = system + '_' + architecture
    if system == 'windows':
        with zipfile.ZipFile(output / f'dockpipe_{version}_{suffix}.zip', 'x', zipfile.ZIP_DEFLATED) as archive:
            archive.write(binary, 'dockpipe.exe')
    else:
        with tarfile.open(output / f'dockpipe_{version}_{suffix}.tar.gz', 'x:gz') as archive:
            archive.add(binary, arcname='dockpipe', recursive=False)
    selected = os.environ.get('DOCKPIPE_RELEASE_STORE')
    if selected:
        helper = root / 'workflows/release/metadata/assets/scripts/release-artifacts.py'
        subprocess.run([sys.executable, str(helper), 'verify-store', selected], check=True)
        destination = output / 'stores' / (system + '-' + architecture)
        if destination.exists() or destination.is_symlink():
            raise ValueError('Final store must be fresh')
        shutil.copytree(selected, destination)
        with tarfile.open(output / f'dockpipe-packages_{version}_{system}-{architecture}.tar.gz', 'x:gz') as archive:
            for item in sorted(destination.iterdir()):
                archive.add(item, arcname=item.name)


if __name__ == '__main__':
    main()
