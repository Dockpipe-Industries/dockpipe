#!/usr/bin/env python3
"""Export an immutable Go compatibility SDK without private package implementations."""
import argparse
import gzip
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[2]
ENTRYPOINTS = (
    'dockpipe/src/lib/infrastructure',
    'dockpipe/src/lib/infrastructure/filepublication',
    'dockpipe/src/lib/infrastructure/remote',
)
EXTERNAL = ('packages/',)
TARGETS = ('linux/amd64', 'linux/arm64', 'windows/amd64', 'darwin/amd64', 'darwin/arm64')


def objects(text):
    decoder = json.JSONDecoder()
    position = 0
    while position < len(text):
        if text[position].isspace():
            position += 1
            continue
        value, position = decoder.raw_decode(text, position)
        yield value


def source_files(root, targets, go, committed_embeds=False):
    paths = {'go.mod', 'go.sum', 'LICENSE'}
    for target in targets:
        system, architecture = target.split('/')
        env = dict(os.environ, GOTOOLCHAIN='local', GOWORK='off', GOPROXY='off', GOSUMDB='off',
                   GOOS=system, GOARCH=architecture, CGO_ENABLED='0')
        result = subprocess.run([go, 'list', '-mod=readonly', '-deps', '-json', *ENTRYPOINTS], cwd=root,
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
    # Render the root's embed declaration from the SDK's selected payload, never
    # carry a declaration that names retired/private package sources.
    spec = importlib.util.spec_from_file_location('embedded_inputs', ROOT / 'release/packaging/embedded-inputs.py')
    embedded = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(embedded)
    if committed_embeds:
        prefix = '//go:embed '
        authored = [json.loads(line[len(prefix):]) for line in (root / 'embed_assets.go').read_text().splitlines()
                    if line.startswith(prefix)]
        if not authored:
            raise ValueError('Committed source snapshot must contain an exact embed declaration')
    else:
        authored = embedded.authored(root)
    payload = [name for name in authored if not name.startswith(EXTERNAL)]
    paths.update(payload)
    content = {}
    for name in sorted(paths):
        path = root / name
        if not path.is_file() or any(parent.is_symlink() for parent in [path, *path.parents]):
            raise ValueError(f'SDK input must be a regular file: {name}')
        content[name] = path.read_bytes()
    content['embed_assets.go'] = embedded.render(payload, 'authoredFS').encode()
    if 'embed_release_generated.go' in content:
        raise ValueError('Clean release-only embeds before exporting the compatibility SDK')
    return content


def export(root, output, targets, go, committed_embeds=False, source_commit=None):
    files = source_files(root, targets, go, committed_embeds)
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
                    'targets': list(targets), 'entrypoints': list(ENTRYPOINTS),
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
    parser.add_argument('--committed-embeds', action='store_true', help='Use the exact embed declaration in a committed source snapshot')
    parser.add_argument('--source-commit', help='Commit used to create the source snapshot; recorded with its content identity')
    args = parser.parse_args()
    if args.source_commit and (not args.committed_embeds or not re.fullmatch(r'[0-9a-f]{40}', args.source_commit)):
        parser.error('--source-commit requires --committed-embeds and an exact lowercase Git commit')
    export(args.source_root.resolve(), args.output.resolve(), args.target or TARGETS, args.go,
           args.committed_embeds, args.source_commit)
