#!/usr/bin/env python3
"""Prepare a deliberately selected private, locally qualified Flatpak Browser SDK.

This path never creates publisher attestations or replaces the public SDK lock.
The selection binds the owning producer's archive, installed DOM proof and exact
source scope. Public builds continue to require their ordinary attested SDK.
"""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import shutil

SPEC = importlib.util.spec_from_file_location('private_flatpak_profile', Path(__file__).with_name('prepare-flatpak-browser-sdk.py'))
PROFILE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PROFILE)
BASE = PROFILE.BASE
STEM = PROFILE.PRODUCT + '-' + PROFILE.VERSION + '-' + PROFILE.PLATFORM
SIDECARS = {
    'release': 'private-release.json', 'manifest': 'private-manifest.json',
    'qualification': 'private-qualification.json', 'runtime_proof': 'private-runtime-proof.json',
    'source_context': 'private-source-context.json',
}


def selected_document(record):
    if not isinstance(record, dict) or set(record) != {'path', 'sha256'}:
        raise ValueError('Private producer input requires an exact path and SHA-256')
    if not isinstance(record.get('path'), str):
        raise ValueError('Private producer input path must be a string')
    path = Path(record['path'])
    if not path.is_absolute() or not BASE.valid_digest(record['sha256']):
        raise ValueError('Private producer input must have an absolute path and exact digest')
    data = BASE.read_regular(path, BASE.MAX_METADATA_BYTES)
    if hashlib.sha256(data).hexdigest() != record['sha256']:
        raise ValueError('Private producer metadata differs from the selected digest')
    return BASE.decode_json(data), data, path


def load_selection(path):
    if not path.is_absolute():
        raise ValueError('Select an absolute private Browser proof file')
    data = BASE.read_regular(path, BASE.MAX_METADATA_BYTES)
    selection = BASE.decode_json(data)
    fields = {'schema', 'source_sha', 'release', 'qualification', 'runtime_proof', 'source_context'}
    if (not isinstance(selection, dict) or set(selection) != fields
            or selection.get('schema') != 'dockpipe.flatpak.private-browser-selection/v1'
            or not isinstance(selection.get('source_sha'), str)
            or not re.fullmatch(r'[0-9a-f]{40}', selection['source_sha'])):
        raise ValueError('Private Browser SDK selection must bind its exact owning source')
    documents = {}
    raw = {}
    paths = {}
    for name in ('release', 'qualification', 'runtime_proof', 'source_context'):
        documents[name], raw[name], paths[name] = selected_document(selection[name])
    release = documents['release']
    expected = {'schema': 1, 'product': PROFILE.PRODUCT, 'version': PROFILE.VERSION,
                'platform': PROFILE.PLATFORM, 'source_repository': PROFILE.REPOSITORY,
                'source_sha': selection['source_sha'], 'source_state': 'local-proof',
                'compiler': PROFILE.COMPILER, 'qt_components': PROFILE.QT_COMPONENTS,
                'flatpak': PROFILE.FLATPAK, 'runtime_bundled': False, 'abi': {'cxx_standard': 17}}
    if (not isinstance(release, dict) or type(release.get('schema')) is not int
            or any(release.get(key) != value for key, value in expected.items())):
        raise ValueError('Private Browser SDK release differs from the exact local ABI/source tuple')
    checks = {'sdk-component-install', 'focused-browser-tests',
              'relocated-external-consumer-compile-link-run', 'flatpak-installed-runtime-dom'}
    def has_checks(value):
        values = value.get('checks', [])
        return isinstance(values, list) and all(isinstance(item, str) for item in values) and checks.issubset(values)
    qualification = documents['qualification']
    if (not isinstance(qualification, dict)
            or any(qualification.get(key) != release.get(key) for key in
                   ('platform', 'source_sha', 'source_state', 'compiler', 'qt_components', 'flatpak'))
            or not has_checks(release) or not has_checks(qualification)
            or qualification.get('runtime_proof_sha256') != selection['runtime_proof']['sha256']):
        raise ValueError('Private SDK lacks the selected owning installed-runtime qualification')
    driver = qualification.get('qualification_driver', {})
    if (not isinstance(driver, dict) or set(driver) != {'ref', 'commit'}
            or driver.get('ref') != 'runtime/org.freedesktop.Platform.GL.default/x86_64/25.08'
            or not BASE.valid_digest(driver.get('commit'))):
        raise ValueError('Private SDK must identify its independently qualified software renderer')
    proof = documents['runtime_proof']
    if (not isinstance(proof, dict) or proof.get('check') != 'flatpak-installed-runtime-dom'
            or proof.get('dom') != 'real-dom' or proof.get('sandbox') != 'default'
            or proof.get('Qt6Core') != '6.10.3' or proof.get('Qt6WebEngineCore') != '6.10.2'
            or proof.get('runtime_commit') != PROFILE.FLATPAK['runtime']['commit']
            or not isinstance(proof.get('renderer'), str)
            or 'llvmpipe' not in proof['renderer'].lower()):
        raise ValueError('Private SDK installed DOM/default Chromium sandbox proof differs')
    context = documents['source_context']
    if not isinstance(context, dict) or context.get('base_head') != selection['source_sha']:
        raise ValueError('Private source scope does not identify the selected base revision')
    archive = release.get('archive', {})
    BASE.validate_pin(archive, STEM + '.tar.gz', BASE.MAX_ARCHIVE_BYTES)
    archive_path = paths['release'].parent / archive['name']
    if (archive_path.is_symlink() or not archive_path.is_file()
            or archive_path.stat().st_size != archive['size_bytes']
            or BASE.digest(archive_path) != archive['sha256']):
        raise ValueError('Private SDK archive differs from the owning producer identity')
    manifest_record = release.get('file_manifest', {})
    if not isinstance(manifest_record, dict) or manifest_record.get('name') != STEM + '.files.json':
        raise ValueError('Private SDK file manifest identity is invalid')
    manifest_path = paths['release'].parent / manifest_record['name']
    manifest_bytes = BASE.read_regular(manifest_path, BASE.MAX_METADATA_BYTES)
    if hashlib.sha256(manifest_bytes).hexdigest() != manifest_record.get('sha256'):
        raise ValueError('Private SDK file manifest differs from its release')
    entries = PROFILE.verify_manifest(manifest_bytes, set(SIDECARS.values()) | {'private-selection.json', 'private-consumer-receipt.json'})
    raw['manifest'] = manifest_bytes
    identity = {'schema': 1, 'product': PROFILE.PRODUCT, 'selected_platform': PROFILE.PLATFORM,
                'source_sha': selection['source_sha'], 'source_state': 'local-proof',
                'compiler': release['compiler'], 'qt_components': release['qt_components'],
                'flatpak': release['flatpak'], 'qualification_driver': driver,
                'selection_sha256': hashlib.sha256(data).hexdigest(),
                'archive_sha256': archive['sha256'], 'attestations_verified': False,
                'public_release': False, 'files': len(entries),
                'installed_bytes': sum(row['bytes'] for row in entries.values())}
    return data, raw, archive_path, entries, identity


def prepare(selection_path, output):
    selection, raw, archive, entries, receipt = load_selection(selection_path)
    if not output.is_absolute() or output.exists() or output.is_symlink():
        raise ValueError('Private SDK output must be a fresh absolute task prefix')
    output.mkdir(parents=True)
    BASE.unpack(archive, output, {'prefix': ''}, entries)
    for name, filename in SIDECARS.items():
        (output / filename).write_bytes(raw[name])
    (output / 'private-selection.json').write_bytes(selection)
    (output / 'private-consumer-receipt.json').write_text(json.dumps(receipt, indent=2, sort_keys=True) + '\n')
    return receipt


def verify(prefix, selection_path):
    selection, raw, _, entries, receipt = load_selection(selection_path)
    if not prefix.is_absolute() or prefix.is_symlink() or not prefix.is_dir():
        raise ValueError('Select the prepared private SDK regular prefix')
    expected = set(entries) | set(SIDECARS.values()) | {'private-selection.json', 'private-consumer-receipt.json'}
    actual = set()
    for path in prefix.rglob('*'):
        if path.is_symlink():
            raise ValueError('Prepared private SDK must not contain aliases')
        if path.is_file():
            actual.add(path.relative_to(prefix).as_posix())
    if actual != expected:
        raise ValueError('Prepared private SDK has missing or extra files')
    for name, record in entries.items():
        path = prefix / name
        if path.stat().st_size != record['bytes'] or BASE.digest(path) != record['sha256']:
            raise ValueError('Prepared private SDK payload differs from its exact inventory')
    for name, filename in SIDECARS.items():
        if BASE.read_regular(prefix / filename, BASE.MAX_METADATA_BYTES) != raw[name]:
            raise ValueError('Private SDK proof sidecar changed')
    if BASE.read_regular(prefix / 'private-selection.json', BASE.MAX_METADATA_BYTES) != selection:
        raise ValueError('Prepared private SDK selection changed')
    if BASE.decode_json(BASE.read_regular(prefix / 'private-consumer-receipt.json', BASE.MAX_METADATA_BYTES)) != receipt:
        raise ValueError('Private SDK receipt differs from the selected local proof')
    return receipt, entries


def stage(prefix, selection_path, runtime=None, licenses=None):
    receipt, entries = verify(prefix, selection_path)
    if runtime:
        names = [name for name in entries if Path(name).parent.as_posix() == 'lib'
                 and Path(name).name.startswith('libSpatialWebBrowser.so')]
        if not names:
            raise ValueError('Private SDK lacks its Browser shared runtime')
        runtime.mkdir(parents=True, exist_ok=True)
        for name in names:
            target = runtime / Path(name).name
            if target.exists() or target.is_symlink():
                raise ValueError('Private Browser runtime destination already exists')
            shutil.copy2(prefix / name, target)
    if licenses:
        names = [name for name in entries if name.startswith('share/SpatialWebBrowser/')]
        if not names:
            raise ValueError('Private SDK lacks its installed third-party notices')
        licenses.mkdir(parents=True, exist_ok=False)
        for name in names:
            target = licenses / Path(name).relative_to('share/SpatialWebBrowser')
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(prefix / name, target)
        (licenses / 'private-consumer-receipt.json').write_text(json.dumps(receipt, indent=2, sort_keys=True) + '\n')
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--selection', type=Path, required=True)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument('--output', type=Path)
    mode.add_argument('--sdk', type=Path)
    parser.add_argument('--runtime', type=Path)
    parser.add_argument('--licenses', type=Path)
    arguments = parser.parse_args()
    try:
        if arguments.output:
            if arguments.runtime or arguments.licenses:
                raise ValueError('Prepare the private SDK before staging runtime files')
            receipt = prepare(arguments.selection, arguments.output)
        else:
            receipt = stage(arguments.sdk, arguments.selection, arguments.runtime, arguments.licenses)
    except (OSError, ValueError) as error:
        parser.exit(1, 'Private Flatpak Browser SDK failed: ' + str(error) + '\n')
    print(json.dumps(receipt, indent=2, sort_keys=True))


if __name__ == '__main__':
    main()
