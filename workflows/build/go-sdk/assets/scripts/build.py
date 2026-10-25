#!/usr/bin/env python3
"""Export an immutable Go compatibility SDK without private package implementations."""
import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[5]
ENTRYPOINTS = (
    'dockpipe/src/lib/infrastructure',
    'dockpipe/src/lib/infrastructure/filepublication',
    'dockpipe/src/lib/infrastructure/remote',
)
EXTERNAL = ('packages/',)
TARGETS = ('linux/amd64', 'linux/arm64', 'windows/amd64', 'darwin/amd64', 'darwin/arm64')


def entrypoints(root):
    """Retain historical SDK inventories while exporting newly public helpers."""
    helper = root / 'src/lib/sdk/repotools'
    if helper.is_symlink():
        raise ValueError('Public SDK helper source cannot be a directory alias')
    if helper.is_dir():
        return (*ENTRYPOINTS, 'dockpipe/src/lib/sdk/repotools')
    return ENTRYPOINTS


def objects(text):
    decoder = json.JSONDecoder()
    position = 0
    while position < len(text):
        if text[position].isspace():
            position += 1
            continue
        value, position = decoder.raw_decode(text, position)
        yield value


def source_files(root, targets, go, selected_entrypoints=None):
    if selected_entrypoints is None:
        selected_entrypoints = entrypoints(root)
    paths = {'go.mod', 'go.sum', 'LICENSE'}
    for target in targets:
        system, architecture = target.split('/')
        env = dict(os.environ, GOTOOLCHAIN='local', GOWORK='off', GOPROXY='off', GOSUMDB='off',
                   GOOS=system, GOARCH=architecture, CGO_ENABLED='0')
        result = subprocess.run([go, 'list', '-mod=readonly', '-deps', '-json', *selected_entrypoints], cwd=root,
                                env=env, capture_output=True, text=True, timeout=120)
        if result.returncode:
            raise ValueError(f'Offline dependency discovery failed for {target}: {result.stderr.strip()}')
        for package in objects(result.stdout):
            if package.get('Module', {}).get('Path') != 'dockpipe':
                continue
            directory = Path(package['Dir'])
            relative = directory.relative_to(root)
            for field in ('GoFiles', 'CgoFiles', 'CFiles', 'HFiles', 'SFiles', 'SysoFiles', 'EmbedFiles'):
                for name in package.get(field, []):
                    path = (relative / name).as_posix()
                    if not path.startswith(EXTERNAL):
                        paths.add(path)
    # Go reports each package's own EmbedFiles. Generic library resources
    # therefore travel with their owning package, without a repository workflow
    # inventory or a synthetic root embed declaration.
    content = {}
    for name in sorted(paths):
        path = root / name
        if not path.is_file() or any(parent.is_symlink() for parent in [path, *path.parents]):
            raise ValueError(f'SDK input must be a regular file: {name}')
        content[name] = path.read_bytes()
    if 'embed_release_generated.go' in content:
        raise ValueError('Clean release-only embeds before exporting the compatibility SDK')
    return content


def export(root, output, targets, go, source_commit=None):
    selected_entrypoints = entrypoints(root)
    files = source_files(root, targets, go, selected_entrypoints)
    inventory = [{'path': name, 'bytes': len(data), 'sha256': hashlib.sha256(data).hexdigest()}
                 for name, data in sorted(files.items())]
    identity = hashlib.sha256(json.dumps(inventory, separators=(',', ':')).encode()).hexdigest()
    name = f'dockpipe-go-sdk-{identity[:16]}.tar.gz'
    output.mkdir(parents=True, exist_ok=True)
    archive_path = output / name
    manifest_path = output / name.replace('.tar.gz', '.json')
    if archive_path.exists() or manifest_path.exists():
        raise ValueError('Use a fresh output directory; SDK artifacts are immutable')
    with tempfile.TemporaryDirectory(prefix='go-sdk-', dir=output) as temporary:
        archive = Path(temporary) / name
        with archive.open('wb') as destination, gzip.GzipFile(fileobj=destination, mode='wb', filename='', mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode='w') as tar:
                for path, data in sorted(files.items()):
                    entry = tarfile.TarInfo('dockpipe-go-sdk/' + path)
                    entry.size = len(data)
                    entry.mode = 0o644
                    tar.addfile(entry, io.BytesIO(data))
        checksum = hashlib.sha256(archive.read_bytes()).hexdigest()
        metadata = {'schema': 1, 'module': 'dockpipe', 'sdk_version': f'0.1.0-source.{identity[:16]}',
                    'artifact': name, 'sha256': checksum,
                    'bytes': archive.stat().st_size, 'source_digest': identity,
                    'targets': list(targets), 'entrypoints': list(selected_entrypoints),
                    'distribution_approved': False, 'files': inventory}
        if source_commit is not None:
            metadata['source_commit'] = source_commit
        archive.rename(archive_path)
        manifest_path.write_text(json.dumps(metadata, indent=2) + '\n')
    print(manifest_path)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--target', action='append', choices=TARGETS)
    parser.add_argument('--go', default='go', help='Go executable; automatic toolchain downloads are disabled')
    parser.add_argument('--source-root', type=Path, default=ROOT, help='Source tree to export; defaults to this checkout')
    parser.add_argument('--source-commit', help='Commit used to create the source snapshot; recorded with its content identity')
    arguments = sys.argv[1:] or json.loads(os.environ.get("DOCKPIPE_ARGS_JSON", "[]"))
    args = parser.parse_args(arguments)
    if args.source_commit and not re.fullmatch(r'[0-9a-f]{40}', args.source_commit):
        parser.error('--source-commit requires an exact lowercase Git commit')
    export(args.source_root.resolve(), args.output.resolve(), args.target or TARGETS, args.go,
           source_commit=args.source_commit)
