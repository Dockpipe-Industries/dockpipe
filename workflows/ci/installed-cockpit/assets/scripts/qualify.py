#!/usr/bin/env python3
"""Verify external package bytes before building the core host's native consumer tests."""
import gzip
import importlib.util
import json
import os
from pathlib import Path, PurePosixPath
import platform
import subprocess
import tarfile
import tempfile
import xml.etree.ElementTree as ET


ROOT = Path(__file__).resolve().parents[5]
MAX_PACKAGE_MEMBERS = 10000
REQUIRED_TESTS = {'cockpit-package-rendering', 'workspace-package', 'workspace-package-manager'}


def ingestion_module():
    path = ROOT / 'workflows/catalog/compose/assets/scripts/ingest-pinned-stores.py'
    spec = importlib.util.spec_from_file_location('installed_cockpit_ingestion', path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


INGEST = ingestion_module()


def extract_cockpit(store, manifest, output):
    entries, _ = INGEST.composer.validate_store(manifest)
    if manifest['platform'] != 'linux-amd64':
        raise ValueError('Installed Cockpit requires the pinned Linux amd64 store')
    entry = entries.get(('resolver', 'cockpit'))
    if entry is None:
        raise ValueError('Pinned store has no Cockpit resolver; publish and pin a qualified external package')
    archive_path = store / entry['tarball']
    pin = {'sha256': entry['sha256'],
           'size_bytes': entry.get('size_bytes', INGEST.regular_file(archive_path, INGEST.MAX_BUNDLE_BYTES))}
    INGEST.verify_bundle(archive_path, pin)
    if output.exists() or output.is_symlink():
        raise ValueError('Installed Cockpit extraction requires a fresh directory')
    output.mkdir()
    seen = set()
    expanded = 0
    prefix = PurePosixPath('resolvers/cockpit')
    with INGEST.composer.open_archive(archive_path) as reader, gzip.GzipFile(fileobj=reader) as compressed:
        with tarfile.open(fileobj=INGEST.ExpandedReader(compressed), mode='r|') as archive:
            for member in archive:
                name = member.name.rstrip('/') if member.isdir() else member.name
                relative = PurePosixPath(name)
                if (not name or name != relative.as_posix() or relative.is_absolute()
                        or '..' in relative.parts or '\\' in name or ':' in name
                        or name in seen or len(seen) >= MAX_PACKAGE_MEMBERS
                        or not (member.isdir() or member.isfile())):
                    raise ValueError('Cockpit archive has an unsafe, duplicate, linked or excessive member')
                if relative != prefix and prefix not in relative.parents:
                    raise ValueError('Cockpit archive contains files outside its resolver identity')
                seen.add(name)
                destination = output / name
                if member.isdir():
                    destination.mkdir(parents=True, exist_ok=True)
                    continue
                expanded += member.size
                if member.size < 0 or expanded > INGEST.MAX_EXPANDED_BYTES:
                    raise ValueError('Cockpit package exceeds the pinned-store expanded byte budget')
                destination.parent.mkdir(parents=True, exist_ok=True)
                copied = 0
                with archive.extractfile(member) as source, destination.open('xb') as target:
                    while block := source.read(1 << 20):
                        copied += len(block)
                        if copied > member.size:
                            raise ValueError('Cockpit package member exceeds its declared size')
                        target.write(block)
                if copied != member.size:
                    raise ValueError('Cockpit package member is truncated')
                destination.chmod(0o755 if member.mode & 0o111 else 0o644)
    assets = output / 'resolvers/cockpit/assets'
    for name in ('ui/libdockpipe-cockpit.so', 'ui/cockpit-reader'):
        payload = assets / name
        if not payload.is_file():
            raise ValueError(f'Pinned Cockpit lacks {name}; its external package ABI must be qualified before this check can pass')
    if not os.access(assets / 'ui/cockpit-reader', os.X_OK):
        raise ValueError('Pinned Cockpit reader is not executable')
    return assets, entry


def require_complete_ctest_report(path):
    cases = ET.parse(path).getroot().findall('.//testcase')
    if len(cases) != len(REQUIRED_TESTS) or {case.get('name') for case in cases} != REQUIRED_TESTS:
        raise ValueError('Native consumer qualification did not execute exactly the required tests')
    for case in cases:
        if (case.get('status') in {'notrun', 'disabled'} or case.find('skipped') is not None
                or case.find('failure') is not None or case.find('error') is not None):
            raise ValueError('A native consumer test failed or skipped: ' + case.get('name', 'unknown'))


def qualify(work, assets, qt_root, environment):
    version = subprocess.check_output([str(qt_root / 'bin/qmake'), '-query', 'QT_VERSION'],
                                      text=True, env=environment).strip()
    if version != '6.10.3':
        raise ValueError('Installed Cockpit requires the exact Qt 6.10.3 host ABI')
    build = work / 'host-build'
    subprocess.run(['cmake', '-S', str(ROOT / 'src/dockpipe-app'), '-B', str(build),
                    '-DCMAKE_BUILD_TYPE=Release', '-DBUILD_TESTING=ON',
                    f'-DCMAKE_PREFIX_PATH={qt_root}', f'-DQt6_DIR={qt_root}/lib/cmake/Qt6',
                    f'-DDOCKPIPE_COCKPIT_ASSETS={assets}'], env=environment, check=True)
    subprocess.run(['cmake', '--build', str(build), '--parallel', '2', '--target',
                    'cockpit-package-rendering-test', 'workspace-package-test', 'workspace-package-manager-test'],
                   env=environment, check=True)
    report = work / 'native-consumer-tests.xml'
    subprocess.run(['ctest', '--test-dir', str(build), '--output-on-failure', '--no-tests=error',
                    '--output-junit', str(report), '-R',
                    '^(cockpit-package-rendering|workspace-package|workspace-package-manager)$'],
                   env=environment, check=True)
    require_complete_ctest_report(report)


def main():
    if platform.system() != 'Linux' or platform.machine() not in {'x86_64', 'AMD64'}:
        raise ValueError('Installed Cockpit qualification requires a Linux amd64 host')
    binary = Path(os.environ.get('DOCKPIPE_BIN', ''))
    qt_root = Path(os.environ.get('DOCKPIPE_QT_ROOT', ''))
    if not binary.is_absolute() or not os.access(binary, os.X_OK) or not qt_root.is_absolute():
        raise ValueError('Select absolute DOCKPIPE_BIN and exact DOCKPIPE_QT_ROOT inputs')
    output = Path(subprocess.check_output([str(binary), 'scope', 'artifacts', 'installed-cockpit'], text=True).strip())
    output.mkdir(parents=True, exist_ok=True)
    # Keep failed downloads/builds and their native-test report for inspection.
    work = Path(tempfile.mkdtemp(prefix='native-consumer-', dir=output))
    lock = INGEST.load_lock(ROOT / 'workflows/catalog/compose/assets/pinned-stores.lock.json')
    pin = lock['baselines']['linux-amd64']
    bundle = work / 'baseline.tar.gz'
    INGEST.download(pin, bundle, baseline_platform='linux-amd64')
    store = work / 'store'
    manifest = INGEST.unpack_bundle(bundle, pin, store, 'linux-amd64')
    assets, entry = extract_cockpit(store, manifest, work / 'installed-package')
    environment = dict(os.environ, DOCKPIPE_BIN=str(binary), QTWEBENGINE_DISABLE_SANDBOX='1',
                       QT_PLUGIN_PATH=str(qt_root / 'plugins'),
                       QTWEBENGINEPROCESS_PATH=str(qt_root / 'libexec/QtWebEngineProcess'))
    library_paths = [str(qt_root / 'lib'), str(assets / 'lib')]
    if os.environ.get('LD_LIBRARY_PATH'):
        library_paths.append(os.environ['LD_LIBRARY_PATH'])
    environment['LD_LIBRARY_PATH'] = os.pathsep.join(library_paths)
    qualify(work, assets, qt_root, environment)
    receipt = {'schema': 1, 'platform': 'linux-amd64', 'qt_version': '6.10.3',
               'baseline': pin, 'resolver': entry, 'checks': sorted(REQUIRED_TESTS)}
    (work / 'qualification.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print('PASS: pinned external Cockpit bytes, host SDK loader and required X11 package rendering')


if __name__ == '__main__':
    main()
