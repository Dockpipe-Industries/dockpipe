import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

MODULE_PATHS = {
    'prepare-cockpit.py': Path(__file__).resolve().parents[4] / 'workflows/qualify/cockpit/assets/scripts/prepare-cockpit.py',
    'ingest-pinned-stores.py': Path(__file__).resolve().parents[4] / 'workflows/catalog/compose/assets/scripts/ingest-pinned-stores.py',
}


def module(name, filename):
    spec = importlib.util.spec_from_file_location(name, MODULE_PATHS[filename])
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


consumer = module('consumer', 'prepare-cockpit.py')
ingestion = module('ingestion', 'ingest-pinned-stores.py')


def write_json(path, value):
    path.write_text(json.dumps(value, sort_keys=True) + '\n')


def make_archive(path, paths):
    with tarfile.open(path, 'w:gz') as archive:
        for name, content in paths.items():
            member = tarfile.TarInfo(name)
            member.size = len(content)
            archive.addfile(member, io.BytesIO(content))


class ConsumerTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.store = self.root / 'store'
        self.store.mkdir()
        self.archive_name = 'dockpipe-resolver-cockpit-0.1.3.tar.gz'
        make_archive(self.store / self.archive_name, {
            'resolvers/cockpit/assets/ui/libdockpipe-cockpit.so': b'module',
            'resolvers/cockpit/assets/ui/dockpipe-workspace-reader': b'reader'})
        archive = consumer.identity(self.store / self.archive_name)
        self.entry = {'name': 'cockpit', 'version': '0.1.3', 'tarball': archive['name'],
                      'sha256': archive['sha256'], 'size_bytes': archive['size_bytes']}
        self.manifest = {'schema': 1, 'platform': 'linux-amd64',
                         'packages': {'resolvers': [self.entry], 'workflows': []}}
        write_json(self.store / 'packages-store-manifest.json', self.manifest)
        (self.store / (self.archive_name + '.sha256')).write_text(archive['sha256'] + '\n')
        self.pins = {}
        receipt_dependencies = {}
        for key, filename, product in (
                ('launcher_sdk', 'launcher-sdk.lock.json', 'DockPipeLauncherSDK'),
                ('spatialpipe_scene_sdk', 'spatialpipe-scene-sdk.lock.json', 'SpatialPipeScene'),
                ('spatial_web_projection_sdk', 'spatialweb-projection-sdk.lock.json', 'SpatialWebProjectionSDK')):
            sdk = {'product': product, 'version': '1.1.0' if key == 'launcher_sdk' else '0.1.0',
                   'platform': 'linux-amd64', 'qt_version': '6.10.3',
                   'source_repository': consumer.PUBLISHER if key == 'launcher_sdk' else ('Dockpipe-Industries/Spatial-Web' if key == 'spatial_web_projection_sdk' else consumer.SOURCE),
                   'source_sha': ('b' if key == 'launcher_sdk' else 'c') * 40, 'artifact': {'sha256': 'c' * 64, 'size_bytes': 1},
                   'metadata': {'sha256': 'd' * 64, 'size_bytes': 2}}
            self.pins[filename] = {'lock': sdk}
            abi = {'cxx_standard': 17}
            if key == 'launcher_sdk':
                abi['workspace_interface'] = 'com.dockpipe.launcher.WorkspacePackage/2.0'
            receipt_dependencies[key] = {**sdk, 'compiler': {'id': 'GNU', 'version': '11.4.0'}, 'abi': abi}
        self.pins['forgepipe-sdk.lock.json'] = {'lock': {'artifact': {'sha256': 'e' * 64}, 'source_sha': 'f' * 40}}
        self.receipt = {'schema': 1, 'platform': 'linux-amd64', 'qt_version': '6.10.3',
                        'source_repository': consumer.SOURCE, 'source_sha': 'a' * 40,
                        'resolver': self.entry, 'checks': sorted(consumer.CHECKS),
                        'dependencies': receipt_dependencies, 'sdk_sha256': 'e' * 64, 'sdk_source_sha': 'f' * 40}
        self.candidate = {'schema': 1, 'product': 'Cockpit', 'version': '0.1.3', 'platform': 'linux-amd64',
                          'qt_version': '6.10.3', 'source_repository': consumer.SOURCE, 'source_sha': 'a' * 40,
                          'compiler': {'id': 'GNU', 'version': '11.4.0'},
                          'channel': 'staging', 'qualification_run_id': 1, 'qualification_run_attempt': 1,
                          'dependencies': self.pins, 'private_debug': {'artifact': {'sha256': '9' * 64}}}
        self.lock = {key: self.candidate[key] for key in ('schema', 'product', 'version', 'platform', 'qt_version', 'source_repository', 'source_sha')}
        self.lock['publisher'] = {'repository': consumer.PUBLISHER, 'source_sha': 'b' * 40,
                                  'release_tag': 'cockpit-test'}
        self.lock_path = self.root / 'cockpit.lock.json'
        self.refresh()

    def refresh(self):
        write_json(self.store / 'cockpit-qualification.json', self.receipt)
        self.candidate['qualification'] = consumer.identity(self.store / 'cockpit-qualification.json')
        self.candidate['runtime_assets'] = [consumer.identity(self.store / name) for name in (
            self.archive_name, self.archive_name + '.sha256', 'packages-store-manifest.json', 'cockpit-qualification.json')]
        write_json(self.store / 'cockpit-candidate.json', self.candidate)
        for key, name in (('candidate', 'cockpit-candidate.json'), ('artifact', self.archive_name), ('qualification', 'cockpit-qualification.json')):
            value = consumer.identity(self.store / name)
            self.lock[key] = {'url': consumer.release_url(self.lock, name), 'sha256': value['sha256'], 'size_bytes': value['size_bytes']}
        (self.store / 'SHA256SUMS').write_text(''.join(
            f'{consumer.digest(path)}  {path.name}\n' for path in sorted(self.store.iterdir()) if path.name != 'SHA256SUMS'))
        write_json(self.lock_path, self.lock)

    def test_verified_runtime_extracts_without_private_diagnostics(self):
        consumer.verify_store(self.store, consumer.load_lock(self.lock_path))
        assets = consumer.unpack_runtime(self.store / self.archive_name, self.root / 'package')
        self.assertEqual((assets / 'ui/dockpipe-workspace-reader').read_bytes(), b'reader')

    def test_prepare_requests_only_public_runtime_bytes(self):
        release = {'assets': [
            {'name': path.name, 'digest': 'sha256:' + consumer.digest(path), 'size': path.stat().st_size}
            for path in self.store.iterdir()]}
        requests = []

        def fetch(pin, destination):
            requests.append(pin['url'])
            destination.write_bytes((self.store / destination.name).read_bytes())

        with patch.object(consumer, 'verify_publisher', return_value=release), patch.object(consumer, 'fetch', side_effect=fetch):
            assets = consumer.prepare(self.lock_path, self.root / 'prepared')
        self.assertTrue((assets / 'ui/libdockpipe-cockpit.so').is_file())
        self.assertEqual(len(requests), 6)
        self.assertTrue(all(url.startswith('https://github.com/' + consumer.PUBLISHER + '/releases/download/') for url in requests))
        self.assertTrue(all('debug' not in url for url in requests))

    def test_archive_or_receipt_tampering_is_rejected(self):
        for name in (self.archive_name, 'cockpit-qualification.json'):
            with self.subTest(name=name):
                path = self.store / name
                original = path.read_bytes()
                path.write_bytes(original + b' ')
                with self.assertRaises(ValueError):
                    consumer.verify_store(self.store, self.lock)
                path.write_bytes(original)

    def test_unlisted_diagnostics_and_symlinks_are_rejected(self):
        extra = self.store / 'debug.zip'
        extra.write_bytes(b'private')
        with self.assertRaises(ValueError):
            consumer.verify_store(self.store, self.lock)
        extra.unlink()
        extra.symlink_to(self.store / self.archive_name)
        with self.assertRaises(ValueError):
            consumer.verify_store(self.store, self.lock)

    def test_source_and_compiler_mismatch_are_rejected(self):
        self.receipt['dependencies']['launcher_sdk']['source_sha'] = '0' * 40
        self.refresh()
        with self.assertRaises(ValueError):
            consumer.verify_store(self.store, self.lock)
        self.receipt['dependencies']['launcher_sdk']['source_sha'] = 'b' * 40
        self.receipt['dependencies']['launcher_sdk']['compiler']['version'] = '12.0'
        self.refresh()
        with self.assertRaises(ValueError):
            consumer.verify_store(self.store, self.lock)

    def test_self_consistent_wrong_sdk_identities_are_rejected(self):
        for key, filename, field, value in (
                ('spatialpipe_scene_sdk', 'spatialpipe-scene-sdk.lock.json', 'version', '1.0.0'),
                ('spatialpipe_scene_sdk', 'spatialpipe-scene-sdk.lock.json', 'source_repository', 'owner/SpatialPipe'),
                ('spatialpipe_scene_sdk', 'spatialpipe-scene-sdk.lock.json', 'source_sha', 'dev'),
                ('launcher_sdk', 'launcher-sdk.lock.json', 'product', 'OtherSDK'),
                ('launcher_sdk', 'launcher-sdk.lock.json', 'platform', 'windows-amd64'),
                ('launcher_sdk', 'launcher-sdk.lock.json', 'qt_version', '6.8.3')):
            with self.subTest(field=field, value=value):
                pin = self.pins[filename]['lock']
                dependency = self.receipt['dependencies'][key]
                original = pin[field]
                pin[field] = value
                dependency[field] = value
                self.refresh()
                with self.assertRaisesRegex(ValueError, 'exact supported'):
                    consumer.verify_store(self.store, self.lock)
                pin[field] = original
                dependency[field] = original
                self.refresh()

    def test_workspace_interface_abi_must_match_host(self):
        self.receipt['dependencies']['launcher_sdk']['abi']['workspace_interface'] = 'old/1.0'
        self.refresh()
        with self.assertRaisesRegex(ValueError, 'native ABI'):
            consumer.verify_store(self.store, self.lock)

    def test_tar_extended_headers_are_bounded_before_allocation(self):
        member = tarfile.TarInfo('extended')
        member.type = tarfile.XHDTYPE
        member.size = consumer.MAX_METADATA + 1
        with self.assertRaisesRegex(ValueError, 'metadata capacity'):
            consumer.BoundedTarInfo.frombuf(member.tobuf(), 'utf-8', 'strict')

    def test_tar_padding_and_sparse_payloads_are_rejected(self):
        with patch.object(consumer, 'MAX_TAR_BYTES', 512):
            with self.assertRaisesRegex(ValueError, 'decompressed tar'):
                consumer.unpack_runtime(self.store / self.archive_name, self.root / 'padding')
        archive = self.root / 'sparse.tar.gz'
        with tarfile.open(archive, 'w:gz', format=tarfile.GNU_FORMAT) as stream:
            member = tarfile.TarInfo('resolvers/cockpit/assets/sparse')
            member.type = tarfile.GNUTYPE_SPARSE
            stream.addfile(member)
        with self.assertRaisesRegex(ValueError, 'sparse'):
            consumer.unpack_runtime(archive, self.root / 'sparse')

    def test_archive_traversal_and_symlink_are_rejected(self):
        archive = self.root / 'malicious.tar.gz'
        make_archive(archive, {'resolvers/cockpit/../../outside': b'escape'})
        with self.assertRaises(ValueError):
            consumer.unpack_runtime(archive, self.root / 'unsafe')
        with tarfile.open(archive, 'w:gz') as stream:
            member = tarfile.TarInfo('resolvers/cockpit/assets/link')
            member.type = tarfile.SYMTYPE
            member.linkname = '/etc/passwd'
            stream.addfile(member)
        with self.assertRaises(ValueError):
            consumer.unpack_runtime(archive, self.root / 'unsafe-link')

    def publisher_equivalence_responses(self):
        pin = self.lock['publisher']
        pin['tag_commit_sha'] = 'd' * 40
        pin['source_tree_sha'] = 'e' * 40
        prefix = 'repos/' + consumer.PUBLISHER
        return {
            prefix + '/releases/tags/cockpit-test': {
                'draft': False, 'prerelease': True, 'tag_name': 'cockpit-test',
                'target_commitish': pin['source_sha'],
            },
            prefix + '/git/ref/tags/cockpit-test': {
                'object': {'type': 'commit', 'sha': pin['tag_commit_sha']},
            },
            prefix + '/git/commits/' + pin['source_sha']: {
                'sha': pin['source_sha'], 'tree': {'sha': pin['source_tree_sha']},
            },
            prefix + '/git/commits/' + pin['tag_commit_sha']: {
                'sha': pin['tag_commit_sha'], 'tree': {'sha': pin['source_tree_sha']},
            },
        }

    def test_publisher_accepts_only_exact_tag_and_original_source_tree(self):
        responses = self.publisher_equivalence_responses()
        with patch.object(consumer, 'api', side_effect=lambda path: responses[path]) as request:
            release = consumer.verify_publisher(self.lock)
        self.assertEqual(release['target_commitish'], self.lock['publisher']['source_sha'])
        self.assertEqual(request.call_count, 4)

    def test_publisher_source_equivalence_rejects_each_wrong_tree_or_commit(self):
        for which in ('original-tree', 'tag-tree', 'returned-commit', 'tag-target', 'release-target'):
            with self.subTest(which=which):
                responses = self.publisher_equivalence_responses()
                prefix = 'repos/' + consumer.PUBLISHER
                pin = self.lock['publisher']
                if which == 'original-tree':
                    responses[prefix + '/git/commits/' + pin['source_sha']]['tree']['sha'] = 'f' * 40
                elif which == 'tag-tree':
                    responses[prefix + '/git/commits/' + pin['tag_commit_sha']]['tree']['sha'] = 'f' * 40
                elif which == 'returned-commit':
                    responses[prefix + '/git/commits/' + pin['tag_commit_sha']]['sha'] = 'f' * 40
                elif which == 'tag-target':
                    responses[prefix + '/git/ref/tags/cockpit-test']['object']['sha'] = pin['source_sha']
                else:
                    responses[prefix + '/releases/tags/cockpit-test']['target_commitish'] = pin['tag_commit_sha']
                with patch.object(consumer, 'api', side_effect=lambda path: responses[path]):
                    with self.assertRaises(ValueError):
                        consumer.verify_publisher(self.lock)

    def test_publisher_source_equivalence_requires_both_exact_pins(self):
        original = dict(self.lock['publisher'])
        for extra in (
                {'tag_commit_sha': 'd' * 40},
                {'source_tree_sha': 'e' * 40},
                {'tag_commit_sha': original['source_sha'], 'source_tree_sha': 'e' * 40},
                {'tag_commit_sha': 'd' * 40, 'source_tree_sha': 'not-a-tree'},
                {'tag_commit_sha': 'not-a-commit', 'source_tree_sha': 'e' * 40}):
            with self.subTest(extra=extra):
                with self.assertRaises(ValueError):
                    consumer.publisher_tag_commit({**original, **extra})

    def test_publisher_rejects_draft_release_or_wrong_host_source_tag(self):
        release = {'draft': False, 'prerelease': True, 'tag_name': 'cockpit-test', 'target_commitish': 'b' * 40}
        reference = {'object': {'type': 'commit', 'sha': 'b' * 40}}
        with patch.object(consumer, 'api', side_effect=[release, reference]) as request:
            consumer.verify_publisher(self.lock)
            self.assertTrue(all(call.args[0].startswith('repos/' + consumer.PUBLISHER + '/') for call in request.call_args_list))
        with patch.object(consumer, 'api', return_value={**release, 'draft': True}):
            with self.assertRaises(ValueError):
                consumer.verify_publisher(self.lock)
        with patch.object(consumer, 'api', side_effect=[release, {'object': {'type': 'commit', 'sha': '2' * 40}}]):
            with self.assertRaises(ValueError):
                consumer.verify_publisher(self.lock)
        self.lock['publisher']['repository'] = consumer.SOURCE
        write_json(self.lock_path, self.lock)
        with self.assertRaises(ValueError):
            consumer.load_lock(self.lock_path)

    def test_wrong_owner_platform_or_missing_native_check_is_rejected(self):
        for failure in ('owner', 'platform', 'checks'):
            with self.subTest(failure=failure):
                original_manifest = json.loads(json.dumps(self.manifest))
                original_receipt = json.loads(json.dumps(self.receipt))
                if failure == 'owner':
                    self.manifest['packages']['workflows'] = [dict(self.entry, tarball='dockpipe-workflow-cockpit-0.1.3.tar.gz')]
                elif failure == 'platform':
                    self.manifest['platform'] = 'linux-arm64'
                else:
                    self.receipt['checks'].remove('x11-package-rendering')
                write_json(self.store / 'packages-store-manifest.json', self.manifest)
                self.refresh()
                with self.assertRaises(ValueError):
                    consumer.verify_store(self.store, self.lock)
                self.manifest = original_manifest
                self.receipt = original_receipt
                write_json(self.store / 'packages-store-manifest.json', self.manifest)
                self.refresh()

    def test_host_compiler_must_match_qualified_dependency_abi(self):
        build = self.root / 'build'
        configured = build / 'CMakeFiles/3.22.1/CMakeCXXCompiler.cmake'
        configured.parent.mkdir(parents=True)
        configured.write_text('set(CMAKE_CXX_COMPILER_ID "GNU")\nset(CMAKE_CXX_COMPILER_VERSION "11.4.0")\n')
        consumer.verify_compiler(build, self.candidate)
        configured.write_text('set(CMAKE_CXX_COMPILER_ID "GNU")\nset(CMAKE_CXX_COMPILER_VERSION "12.0.0")\n')
        with self.assertRaises(ValueError):
            consumer.verify_compiler(build, self.candidate)

    def test_catalog_composition_preserves_other_entries_and_prevents_downgrade(self):
        baseline = self.root / 'baseline'
        baseline.mkdir()
        entries = {}
        for kind, name, version in (('core', 'core', '0.6.19'), ('resolver', 'other', '2.0.0'), ('resolver', 'cockpit', '0.1.1')):
            filename = f'dockpipe-core-{version}.tar.gz' if kind == 'core' else f'dockpipe-{kind}-{name}-{version}.tar.gz'
            path = baseline / filename
            path.write_bytes((kind + name + version).encode())
            entries[name] = {'name': name, 'version': version, 'tarball': filename,
                             'sha256': consumer.digest(path), 'size_bytes': path.stat().st_size}
        manifest = {'schema': 1, 'platform': 'linux-amd64', 'packages': {
            'core': entries['core'], 'resolvers': [entries['other'], entries['cockpit']], 'workflows': []}}
        write_json(baseline / 'packages-store-manifest.json', manifest)
        output = self.root / 'composed'
        result = ingestion.finalize_cockpit(baseline, self.store, output, '0.6.19', self.lock_path)
        self.assertEqual(result['packages']['core'], entries['core'])
        self.assertIn(entries['other'], result['packages']['resolvers'])
        self.assertIn(self.entry, result['packages']['resolvers'])
        self.assertEqual((output / entries['other']['tarball']).read_bytes(), (baseline / entries['other']['tarball']).read_bytes())
        manifest['packages']['resolvers'][1]['version'] = '0.1.4'
        manifest['packages']['resolvers'][1]['tarball'] = 'dockpipe-resolver-cockpit-0.1.4.tar.gz'
        write_json(baseline / 'packages-store-manifest.json', manifest)
        with self.assertRaisesRegex(ValueError, 'downgrade'):
            ingestion.finalize_cockpit(baseline, self.store, self.root / 'downgrade', '0.6.19', self.lock_path)


if __name__ == '__main__':
    unittest.main()
