#!/usr/bin/env python3
"""Publish the locked Go SDK without replacing published bytes or moving a channel."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parents[5]


def load(name, relative):
    specification = importlib.util.spec_from_file_location(name, ROOT / relative)
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


SHARED = load('go_sdk_github_release', 'workflows/publish/shared/github_release.py')
LOCKED = load('go_sdk_lock', 'workflows/build/go-sdk/assets/scripts/build-locked.py')
MAX_ARCHIVE_BYTES = 64 << 20
MAX_MANIFEST_BYTES = 8 << 20


def validate(directory, lock):
    names = {lock['manifest_asset'], lock['archive_asset']}
    if directory.is_symlink() or not directory.is_dir() or {path.name for path in directory.iterdir()} != names:
        raise ValueError('Go SDK publication requires exactly the two locked assets')
    for name in names:
        path = directory / name
        capacity = MAX_MANIFEST_BYTES if name.endswith('.json') else MAX_ARCHIVE_BYTES
        if path.is_symlink() or not path.is_file() or not 0 < path.stat().st_size <= capacity:
            raise ValueError('Go SDK assets must be bounded regular files')
    LOCKED.verify_output(directory, lock)
    manifest = json.loads((directory / lock['manifest_asset']).read_text())
    if (manifest.get('schema') != 1 or manifest.get('module') != 'dockpipe'
            or manifest.get('targets') != list(LOCKED.PRODUCER.TARGETS)):
        raise ValueError('Go SDK manifest differs from its qualified module and target set')
    return {name: {'size': (directory / name).stat().st_size,
                   'digest': 'sha256:' + SHARED.digest(directory / name)} for name in names}


def publish(directory, repository=None):
    repository = SHARED.publication_target(repository)
    if (os.environ.get('GITHUB_REPOSITORY') != SHARED.SOURCE_REPOSITORY
            or os.environ.get('GITHUB_EVENT_NAME') != 'workflow_dispatch'
            or not os.environ.get('GITHUB_REF', '').startswith('refs/heads/')
            or not re.fullmatch(r'[0-9a-f]{40}', os.environ.get('GITHUB_SHA', ''))):
        raise ValueError('Go SDK publication requires an explicit source-repository workflow dispatch')
    lock = LOCKED.read_lock(ROOT / 'workflows/build/go-sdk/assets/source.lock.json')
    source = lock['source_commit']
    tag = 'dockpipe-go-sdk-' + lock['version']
    if os.environ.get('SDK_TAG') != tag or os.environ.get('SDK_COMMIT') != source:
        raise ValueError('Publication inputs differ from the reproduced committed SDK identity')
    assets = validate(directory, lock)
    query = """query($owner:String!,$name:String!,$ref:String!){
      repository(owner:$owner,name:$name){ref(qualifiedName:$ref){target{
        __typename oid ... on Tag {target{
          __typename oid ... on Tag {target{__typename oid}}
        }}
      }}}
    }"""
    owner, name = repository.split('/')
    query_arguments = ['gh', 'api', 'graphql', '-f', 'query=' + query,
                       '-f', 'owner=' + owner, '-f', 'name=' + name, '-f', 'ref=refs/tags/' + tag]

    def verify_reference(allow_missing):
        state = json.loads(SHARED.run(query_arguments))
        if state.get('errors'):
            raise ValueError('Could not verify the immutable Go SDK tag')
        SHARED.verify_tag(state['data']['repository']['ref'], source, allow_missing=allow_missing)

    release = SHARED.find_release(repository, tag)
    verify_reference(release is None or release.get('draft') is True)
    if release is None:
        release = SHARED.create_release(repository, tag, source, 'Dockpipe Go SDK ' + lock['version'],
                                        'Immutable compatibility SDK reproduced from committed source and content pins. '
                                        'No runtime, compiler or production-channel publication.')
    release = SHARED.reconcile_assets(repository, directory, release, tag, source, assets)
    verify_reference(release.get('draft') is True)
    if release.get('draft') is True:
        SHARED.run(['gh', 'release', 'edit', tag, '--repo', repository,
                    '--draft=false', '--prerelease', '--latest=false'])
    final = SHARED.read_release(repository, release['id'], tag)
    SHARED.verify_release(final, tag, source, assets)
    if final.get('draft') is not False:
        raise ValueError('Go SDK candidate remains a draft')
    verify_reference(False)
    print(final['html_url'])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--directory', type=Path, required=True)
    parser.add_argument('--repository', help='Explicit publication destination, otherwise GITHUB_REPOSITORY')
    arguments = parser.parse_args()
    try:
        publish(arguments.directory, arguments.repository)
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        parser.exit(1, 'Go SDK publication failed: ' + str(error) + '\n')


if __name__ == '__main__':
    main()
