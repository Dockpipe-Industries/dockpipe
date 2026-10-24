"""Owning catalog artifacts retain exact source, platform and package boundaries."""
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tarfile
import tempfile
import unittest
from unittest import mock

SCRIPT = Path(__file__).resolve().parents[1] / 'ingest-pinned-stores.py'
spec = importlib.util.spec_from_file_location('catalog_ingestion', SCRIPT)
ingest = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ingest)
SOURCE = 'a' * 40


def catalog_pin(platform='windows-amd64', source=SOURCE):
    return {
        'url': ('https://github.com/Dockpipe-Industries/dockpipe-packages/releases/download/'
                f'catalog-candidate-{source}/dockpipe-catalog-{platform}.tar.gz'),
        'sha256': 'b' * 64,
        'size_bytes': 1,
        'manifest_sha256': 'c' * 64,
    }


def catalog_configuration():
    return {'source_commit': SOURCE, 'owned': [['resolver', 'qemu']],
            'artifacts': {'windows-amd64': catalog_pin()}}


def package(kind, name, version, data):
    prefix = 'dockpipe-core' if kind == 'core' else f'dockpipe-{kind}-{name}'
    return {'name': name, 'version': version, 'tarball': f'{prefix}-{version}.tar.gz',
            'sha256': hashlib.sha256(data).hexdigest(), 'size_bytes': len(data)}


class CatalogPinTests(unittest.TestCase):
    def test_exact_native_owner_source_and_role(self):
        for platform in ingest.PLATFORMS:
            ingest.validate_pin(catalog_pin(platform), catalog_platform=platform,
                                catalog_source_commit=SOURCE)
        pin = catalog_pin()
        rejected = [
            pin['url'].replace('/dockpipe-packages/', '/dockpipe/'),
            pin['url'].replace('github.com/', 'github.com.evil.invalid/'),
            pin['url'].replace('https://', 'http://'),
            pin['url'].replace('https://', 'https://user:password@'),
            pin['url'].replace('catalog-candidate-' + SOURCE, 'latest'),
            pin['url'].replace('catalog-candidate-' + SOURCE, 'catalog-candidate-' + SOURCE[:12]),
            pin['url'].replace('windows-amd64.tar.gz', 'linux-amd64.tar.gz'),
            pin['url'] + '?download=1', pin['url'] + '#fragment',
            pin['url'].replace('/releases/', '/x/../releases/'),
        ]
        for url in rejected:
            with self.subTest(url=url), self.assertRaises(ValueError):
                ingest.validate_pin({**pin, 'url': url}, catalog_platform='windows-amd64',
                                    catalog_source_commit=SOURCE)
        with self.assertRaises(ValueError):
            ingest.validate_pin(pin, catalog_platform='windows-amd64',
                                catalog_source_commit='d' * 40)
        with self.assertRaises(ValueError):
            ingest.validate_pin(pin, catalog_platform='windows-amd64',
                                catalog_source_commit=SOURCE, baseline_platform='windows-amd64')

    def test_native_and_source_candidates_have_disjoint_exact_release_urls(self):
        source_pin = catalog_pin()
        native_pin = {**source_pin, 'url': source_pin['url'].replace(
            '/catalog-candidate-', '/catalog-native-candidate-')}
        for pin in (source_pin, native_pin):
            ingest.validate_pin(pin, catalog_platform='windows-amd64', catalog_source_commit=SOURCE)
        self.assertNotEqual(source_pin['url'], native_pin['url'])
        for invalid in (
                native_pin['url'].replace(SOURCE, SOURCE[:12]),
                native_pin['url'].replace('catalog-native-candidate-', 'catalog-native-latest-'),
                native_pin['url'].replace('catalog-native-candidate-', 'catalog-native-native-candidate-')):
            with self.subTest(url=invalid), self.assertRaises(ValueError):
                ingest.validate_pin({**native_pin, 'url': invalid},
                                    catalog_platform='windows-amd64', catalog_source_commit=SOURCE)

    def test_optional_lock_preserves_legacy_and_rejects_bad_ownership_before_download(self):
        lock = json.loads((SCRIPT.parent / 'pinned-stores.lock.json').read_text())
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            path = root / 'lock.json'
            path.write_text(json.dumps(lock))
            ingest.load_lock(path)
            lock['catalog'] = catalog_configuration()
            path.write_text(json.dumps(lock))
            ingest.load_lock(path)
            variants = []
            for owned in [[['workflow', 'dockpipe-repo-quality']], [['resolver', 'pipeon']],
                          [['core', 'core']], [['resolver', 'qemu'], ['resolver', 'qemu']]]:
                changed = copy.deepcopy(lock)
                changed['catalog']['owned'] = owned
                variants.append(changed)
            for field, value in [('source_commit', 'main'), ('artifacts', {}),
                                 ('artifacts', {'solaris-amd64': catalog_pin()})]:
                changed = copy.deepcopy(lock)
                changed['catalog'][field] = value
                variants.append(changed)
            for changed in variants:
                path.write_text(json.dumps(changed))
                with mock.patch.object(ingest, 'download') as download:
                    with self.assertRaises(ValueError):
                        ingest.ingest(path, 'windows-amd64', root / 'missing-core',
                                      root / 'missing-main', root / 'out')
                    download.assert_not_called()


    def test_unqualified_platform_retains_baseline_without_catalog_download(self):
        lock = json.loads((SCRIPT.parent / 'pinned-stores.lock.json').read_text())
        lock['catalog'] = catalog_configuration()
        lock['catalog']['artifacts'] = {'linux-amd64': catalog_pin('linux-amd64')}
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            path = root / 'lock.json'
            path.write_text(json.dumps(lock))
            baseline = {'schema': 1, 'platform': 'windows-amd64', 'packages': {}}
            original_read = ingest.read_json

            def read(path, maximum):
                if path.name == 'lock.json':
                    return original_read(path, maximum)
                return {'schema': 1, 'platform': 'windows-amd64', 'packages': {}}

            with mock.patch.object(ingest, 'read_json', side_effect=read), \
                    mock.patch.object(ingest, 'download') as download, \
                    mock.patch.object(ingest, 'unpack_bundle', return_value=baseline), \
                    mock.patch.object(ingest, 'compose_inputs', return_value=baseline) as compose:
                ingest.ingest(path, 'windows-amd64', root / 'core', root / 'main', root / 'out')
            download.assert_called_once()
            self.assertEqual(download.call_args.kwargs, {'baseline_platform': 'windows-amd64'})
            self.assertIs(compose.call_args.args[2], baseline)
            self.assertEqual(compose.call_args.args[4], {})


class CatalogCompositionTests(unittest.TestCase):
    def fixtures(self, root):
        baseline_root, candidate_root = root / 'baseline', root / 'catalog'
        baseline_root.mkdir()
        candidate_root.mkdir()
        old = package('resolver', 'qemu', '1.3.5', b'old-qemu')
        unrelated = package('resolver', 'pipeon', '0.6.3', b'independent')
        fresh = package('resolver', 'qemu', '1.3.6', b'new-native-qemu')
        for directory, entry, data in [(baseline_root, old, b'old-qemu'),
                                       (baseline_root, unrelated, b'independent'),
                                       (candidate_root, fresh, b'new-native-qemu')]:
            (directory / entry['tarball']).write_bytes(data)
        baseline = {'schema': 1, 'platform': 'windows-amd64', 'packages': {
            'resolvers': [old, unrelated], 'workflows': []}}
        candidate = {'schema': 1, 'platform': 'windows-amd64', 'packages': {
            'resolvers': [fresh], 'workflows': []}}
        return baseline_root, candidate_root, baseline, candidate, fresh, unrelated

    def test_only_exact_owned_native_packages_change_and_other_publishers_retain_bytes(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            base_root, catalog_root, baseline, candidate, fresh, unrelated = self.fixtures(root)
            output = root / 'out'
            result = ingest.compose_catalog(catalog_configuration(), 'windows-amd64',
                                            baseline, base_root, candidate, catalog_root, output)
            self.assertEqual((output / fresh['tarball']).read_bytes(), b'new-native-qemu')
            self.assertEqual((output / unrelated['tarball']).read_bytes(), b'independent')
            self.assertEqual(len(result['packages']['resolvers']), 2)
            self.assertEqual(baseline['packages']['resolvers'][0]['version'], '1.3.5')

    def test_manifest_ownership_platform_core_and_archive_tampering_fail_closed(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            base_root, catalog_root, baseline, candidate, fresh, unrelated = self.fixtures(root)
            variants = []
            wrong_target = copy.deepcopy(candidate)
            wrong_target['platform'] = 'linux-amd64'
            variants.append(wrong_target)
            extra = copy.deepcopy(candidate)
            extra['packages']['resolvers'].append(unrelated)
            variants.append(extra)
            core = copy.deepcopy(candidate)
            core['packages']['core'] = package('core', 'core', '0.7.0', b'core')
            variants.append(core)
            missing = copy.deepcopy(candidate)
            missing['packages']['resolvers'] = []
            variants.append(missing)
            for index, changed in enumerate(variants):
                output = root / f'out-{index}'
                with self.assertRaises(ValueError):
                    ingest.compose_catalog(catalog_configuration(), 'windows-amd64',
                                           baseline, base_root, changed, catalog_root, output)
                self.assertFalse(output.exists())
            (catalog_root / fresh['tarball']).write_bytes(b'tampered')
            with self.assertRaises(ValueError):
                ingest.compose_catalog(catalog_configuration(), 'windows-amd64',
                                       baseline, base_root, candidate, catalog_root, root / 'tampered')
            self.assertFalse((root / 'tampered').exists())

    def test_existing_archive_name_cannot_gain_different_bytes(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            base_root, catalog_root, baseline, candidate, fresh, unrelated = self.fixtures(root)
            previous = baseline['packages']['resolvers'][0]
            fresh['version'] = previous['version']
            fresh['tarball'] = previous['tarball']
            with self.assertRaisesRegex(ValueError, 'immutable archive'):
                ingest.compose_catalog(catalog_configuration(), 'windows-amd64',
                                       baseline, base_root, candidate, catalog_root, root / 'out')

    def test_flat_catalog_bundle_full_ingestion_replaces_core_main_and_owned_only(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            base_root, catalog_root, baseline, candidate, fresh, unrelated = self.fixtures(root)
            wrapper = package('workflow', 'dockpipe-repo-quality', '0.6.25', b'old-main')
            baseline['packages']['workflows'] = [wrapper]
            (base_root / wrapper['tarball']).write_bytes(b'old-main')
            main_root, core_root = root / 'main', root / 'core'
            main_root.mkdir()
            core_root.mkdir()
            new_main = package('workflow', 'dockpipe-repo-quality', '0.7.0', b'new-main')
            new_core = package('core', 'core', '0.7.0', b'new-core')
            for directory, entry, contents, collection in [
                    (main_root, new_main, b'new-main', 'workflows'),
                    (core_root, new_core, b'new-core', 'core')]:
                (directory / entry['tarball']).write_bytes(contents)
                values = entry if collection == 'core' else [entry]
                manifest = {'schema': 1, 'platform': 'windows-amd64', 'packages': {collection: values}}
                (directory / 'packages-store-manifest.json').write_text(json.dumps(manifest))
            lock = json.loads((SCRIPT.parent / 'pinned-stores.lock.json').read_text())
            lock['catalog'] = catalog_configuration()
            downloads = {}
            for name, directory, manifest, pin in [
                    ('baseline', base_root, baseline, lock['baselines']['windows-amd64']),
                    ('catalog', catalog_root, candidate, lock['catalog']['artifacts']['windows-amd64'])]:
                raw = json.dumps(manifest).encode()
                (directory / 'packages-store-manifest.json').write_bytes(raw)
                bundle = root / (name + '.tar.gz')
                with tarfile.open(bundle, 'w:gz') as archive:
                    for path in sorted(directory.iterdir()):
                        archive.add(path, arcname=path.name)
                pin.update(sha256=hashlib.sha256(bundle.read_bytes()).hexdigest(),
                           size_bytes=bundle.stat().st_size,
                           manifest_sha256=hashlib.sha256(raw).hexdigest())
                downloads[pin['url']] = bundle
            lock_path = root / 'lock.json'
            lock_path.write_text(json.dumps(lock))

            def download(pin, destination, **role):
                ingest.validate_pin(pin, **role)
                shutil.copyfile(downloads[pin['url']], destination)
                ingest.verify_bundle(destination, pin)

            with mock.patch.object(ingest, 'download', side_effect=download) as downloaded:
                result = ingest.ingest(lock_path, 'windows-amd64', core_root, main_root, root / 'out')
            self.assertEqual(downloaded.call_count, 2)
            for entry, contents in [(new_core, b'new-core'), (new_main, b'new-main'),
                                    (fresh, b'new-native-qemu'), (unrelated, b'independent')]:
                self.assertEqual((root / 'out' / entry['tarball']).read_bytes(), contents)
            self.assertEqual(result['platform'], 'windows-amd64')


if __name__ == '__main__':
    unittest.main()
