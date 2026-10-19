"""Pinned release inputs are authenticated before composing a native store."""
import copy
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest import mock
import subprocess


SCRIPT = Path(__file__).resolve().parents[1] / 'ingest-pinned-stores.py'
spec = importlib.util.spec_from_file_location('pinned_stores', SCRIPT)
ingest = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ingest)


def archive_entry(kind, name, version, contents):
    prefix = 'dockpipe-core' if kind == 'core' else f'dockpipe-{kind}-{name}'
    return {
        'name': name,
        'version': version,
        'tarball': f'{prefix}-{version}.tar.gz',
        'sha256': hashlib.sha256(contents).hexdigest(),
        'size_bytes': len(contents),
    }


def write_archive(directory, entry, contents):
    (directory / entry['tarball']).write_bytes(contents)


def fake_pin(size=1, digest=None):
    checksum = digest or 'a' * 64
    return {
        'url': 'https://github.com/Dockpipe-Industries/dockpipe/releases/download/immutable-tag/store.tar.gz',
        'sha256': checksum,
        'size_bytes': size,
        'manifest_sha256': 'b' * 64,
    }


def candidate_pin(name, version, *, stable=False, contents=b'data'):
    if stable:
        url = (f'https://packages.dockpipe.com/packages/standalone/{name}/releases/'
               f'{version}/{name}-packages-{version}-linux-amd64.tar.gz')
    else:
        url = (f'https://packages.staging.dockpipe.com/packages/standalone/{name}/candidates/'
               f'{version}-staging.615330339a21/{name}-packages-{version}-linux-amd64.tar.gz')
    return {**fake_pin(len(contents), hashlib.sha256(contents).hexdigest()), 'url': url}


class PinValidationTests(unittest.TestCase):
    def test_hidden_policy_marks_all_locked_identities_only(self):
        lock = json.loads((SCRIPT.parent / 'pinned-stores.lock.json').read_text())
        hidden = ingest.validate_hidden_identities(
            lock['hidden_identities'],
            set().union(*(ingest.composer.validate_ownership(candidate['owned'])
                          for candidate in lock['linux_candidates'].values())))
        self.assertEqual(len(hidden), 6)
        manifest = {'packages': {'resolvers': [], 'workflows': []}}
        for kind, name in hidden | {('workflow', 'unrelated')}:
            collection = 'resolvers' if kind == 'resolver' else 'workflows'
            manifest['packages'][collection].append({'name': name, 'hidden': False})
        ingest.apply_hidden_policy(manifest, hidden)
        for kind, collection in (('resolver', 'resolvers'), ('workflow', 'workflows')):
            for entry in manifest['packages'][collection]:
                self.assertEqual(entry['hidden'], (kind, entry['name']) in hidden)

    def test_lock_requires_all_platforms_exact_ownership_and_complete_artifacts(self):
        source = json.loads((SCRIPT.parent / 'pinned-stores.lock.json').read_text())
        for name, candidate in source['linux_candidates'].items():
            candidate['artifact'] = candidate_pin(name, candidate['version'])
        with tempfile.TemporaryDirectory() as temporary:
            lock_path = Path(temporary) / 'lock.json'

            def check(lock):
                lock_path.write_text(json.dumps(lock))
                return ingest.load_lock(lock_path)

            self.assertEqual(set(check(source)['baselines']), ingest.PLATFORMS)
            missing_platform = copy.deepcopy(source)
            del missing_platform['baselines']['windows-amd64']
            with self.assertRaises(ValueError):
                check(missing_platform)
            overlap = copy.deepcopy(source)
            overlap['linux_candidates']['pipeon']['owned'][0] = ['resolver', 'dorkpipe']
            with self.assertRaises(ValueError):
                check(overlap)
            missing_owned = copy.deepcopy(source)
            missing_owned['linux_candidates']['dorkpipe']['owned'].pop()
            with self.assertRaises(ValueError):
                check(missing_owned)
            missing_hash = copy.deepcopy(source)
            del missing_hash['linux_candidates']['pipeon']['artifact']['sha256']
            with self.assertRaises(ValueError):
                check(missing_hash)
            unrelated_hidden = copy.deepcopy(source)
            unrelated_hidden['hidden_identities'].append(['workflow', 'unrelated'])
            with self.assertRaises(ValueError):
                check(unrelated_hidden)
            duplicate_hidden = copy.deepcopy(source)
            duplicate_hidden['hidden_identities'].append(['workflow', 'pipeon'])
            with self.assertRaises(ValueError):
                check(duplicate_hidden)

    def test_url_role_owner_version_and_immutable_path_are_exact(self):
        lock = json.loads((SCRIPT.parent / 'pinned-stores.lock.json').read_text())
        for platform, pin in lock['baselines'].items():
            ingest.validate_pin(pin, baseline_platform=platform)
            with self.assertRaises(ValueError):
                ingest.validate_pin(pin, candidate_name='dorkpipe', candidate_version='0.6.3')
        baseline = lock['baselines']['linux-amd64']
        with self.assertRaises(ValueError):
            ingest.validate_pin(baseline, baseline_platform='linux-arm64')

        for name, version in (('dorkpipe', '0.6.3'), ('pipeon', '0.6.4')):
            for stable in (False, True):
                with self.subTest(name=name, stable=stable):
                    pin = candidate_pin(name, version, stable=stable)
                    ingest.validate_pin(pin, candidate_name=name, candidate_version=version)
                    other = 'pipeon' if name == 'dorkpipe' else 'dorkpipe'
                    with self.assertRaises(ValueError):
                        ingest.validate_pin(pin, candidate_name=other,
                                            candidate_version=version)
                    with self.assertRaises(ValueError):
                        ingest.validate_pin(pin, candidate_name=name,
                                            candidate_version='9.9.9')
                    with self.assertRaises(ValueError):
                        ingest.validate_pin(pin, baseline_platform='linux-amd64')

        pin = candidate_pin('dorkpipe', '0.6.3')
        for invalid in (
            pin['url'].replace('https://', 'http://', 1),
            pin['url'].replace('https://', 'https://user:password@', 1),
            pin['url'].replace('.com/', '.com:443/', 1),
            pin['url'] + '?download=1',
            pin['url'] + '#archive',
            pin['url'].replace('/candidates/', '/candidates/../candidates/'),
            pin['url'].replace('/candidates/', '/candidates/%2e%2e/'),
            pin['url'].replace('/candidates/', '/latest/'),
            pin['url'].replace('615330339a21', 'main'),
            pin['url'].replace('615330339a21', '615330339a2'),
            pin['url'].replace('dorkpipe-packages-', 'pipeon-packages-'),
            pin['url'].replace('packages.staging.dockpipe.com', 'other.example.com'),
            pin['url'].replace('/packages/standalone/', '/prefix/packages/standalone/'),
        ):
            with self.subTest(url=invalid):
                with self.assertRaises(ValueError):
                    ingest.validate_pin({**pin, 'url': invalid}, candidate_name='dorkpipe',
                                        candidate_version='0.6.3')

    def test_candidate_download_forbids_redirects_before_hash_admission(self):
        pin = candidate_pin('dorkpipe', '0.6.3')
        with tempfile.TemporaryDirectory() as temporary:
            destination = Path(temporary) / 'bundle.tar.gz'
            with mock.patch.object(ingest.subprocess, 'run', side_effect=subprocess.CalledProcessError(47, 'curl')) as run:
                with self.assertRaises(subprocess.CalledProcessError):
                    ingest.download(pin, destination, candidate_name='dorkpipe',
                                    candidate_version='0.6.3')
            command = run.call_args.args[0]
            self.assertEqual(command[command.index('--max-redirs') + 1], '0')
            self.assertEqual(command[-1], pin['url'])
            self.assertFalse(destination.exists())

    def test_bundle_size_and_checksum_are_exact(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / 'bundle.tar.gz'
            path.write_bytes(b'data')
            pin = fake_pin(size=4, digest=hashlib.sha256(b'data').hexdigest())
            ingest.verify_bundle(path, pin)
            with self.assertRaises(ValueError):
                ingest.verify_bundle(path, {**pin, 'size_bytes': 3})
            with self.assertRaises(ValueError):
                ingest.verify_bundle(path, {**pin, 'sha256': '0' * 64})

    def test_bundle_growth_after_stat_is_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / 'bundle.tar.gz'
            path.write_bytes(b'data!')
            pin = fake_pin(size=4, digest=hashlib.sha256(b'data').hexdigest())
            with mock.patch.object(ingest, 'regular_file', return_value=4):
                with self.assertRaisesRegex(ValueError, 'grew beyond'):
                    ingest.verify_bundle(path, pin)


class BundleSafetyTests(unittest.TestCase):
    def write_bundle(self, directory, members):
        path = directory / 'bundle.tar.gz'
        with tarfile.open(path, 'w:gz') as archive:
            for name, payload, kind in members:
                info = tarfile.TarInfo(name)
                info.size = len(payload)
                if kind is not None:
                    info.type = kind
                archive.addfile(info, io.BytesIO(payload))
        pin = fake_pin(path.stat().st_size, hashlib.sha256(path.read_bytes()).hexdigest())
        return path, pin

    def candidate_members(self):
        payload = b'candidate-archive'
        entry = archive_entry('workflow', 'pipeon', '0.6.4', payload)
        manifest = {'schema': 1, 'platform': 'linux-amd64', 'packages': {
            'resolvers': [], 'workflows': [entry]}}
        encoded = json.dumps(manifest).encode()
        return [
            ('packages/packages-store-manifest.json', encoded, None),
            ('packages/' + entry['tarball'], payload, None),
            ('LICENSE', b'Public license text\n', None),
        ], encoded

    def test_candidate_requires_exact_prefixed_store_and_root_license(self):
        members, manifest_bytes = self.candidate_members()
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for index, changed in enumerate([
                members,
                members[:-1],
                [(name.removeprefix('packages/'), data, kind) if index == 0 else (name, data, kind)
                 for index, (name, data, kind) in enumerate(members)],
                [*members, ('packages/nested/other.tar.gz', b'x', None)],
                [*members, ('packages/../escape', b'x', None)],
                [*members, members[1]],
                [*members, ('LICENSE', b'other license', None)],
            ]):
                directory = root / str(index)
                directory.mkdir()
                path, pin = self.write_bundle(directory, changed)
                pin['manifest_sha256'] = hashlib.sha256(manifest_bytes).hexdigest()
                if index == 0:
                    manifest = ingest.unpack_bundle(path, pin, directory / 'out',
                                                    'linux-amd64', candidate=True)
                    self.assertEqual(manifest['packages']['workflows'][0]['name'], 'pipeon')
                    self.assertEqual({item.name for item in (directory / 'out').iterdir()},
                                     {'packages-store-manifest.json',
                                      manifest['packages']['workflows'][0]['tarball']})
                    with self.assertRaises(ValueError):
                        ingest.unpack_bundle(path, pin, directory / 'other',
                                             'darwin-amd64', candidate=True)
                else:
                    with self.assertRaises(ValueError):
                        ingest.unpack_bundle(path, pin, directory / 'out',
                                             'linux-amd64', candidate=True)

    def test_rejects_traversal_link_duplicate_and_expansion_bound(self):
        cases = [
            [('packages-store-manifest.json', b'{}', None), ('../escape', b'x', None)],
            [('packages-store-manifest.json', b'{}', None), ('link', b'', tarfile.SYMTYPE)],
            [('packages-store-manifest.json', b'{}', None)] * 2,
        ]
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for index, members in enumerate(cases):
                directory = root / str(index)
                directory.mkdir()
                path, pin = self.write_bundle(directory, members)
                with self.assertRaises(ValueError):
                    ingest.unpack_bundle(path, pin, directory / 'out', 'linux-amd64')
            directory = root / 'large'
            directory.mkdir()
            path, pin = self.write_bundle(directory, [('packages-store-manifest.json', b'{}', None)])
            original_limit = ingest.MAX_EXPANDED_BYTES
            try:
                ingest.MAX_EXPANDED_BYTES = 1
                with self.assertRaises(ValueError):
                    ingest.unpack_bundle(path, pin, directory / 'out', 'linux-amd64')
            finally:
                ingest.MAX_EXPANDED_BYTES = original_limit


class CompositionTests(unittest.TestCase):
    def cockpit_inputs(self, root):
        baseline = root / 'baseline'
        cockpit = root / 'cockpit'
        baseline.mkdir()
        cockpit.mkdir()
        core = archive_entry('core', 'dockpipe.core', '0.6.20', b'current-core')
        unrelated = archive_entry('workflow', 'unrelated', '1.2.3', b'preserve-unrelated')
        old = archive_entry('resolver', 'cockpit', '0.1.0', b'old-cockpit')
        new = archive_entry('resolver', 'cockpit', '0.1.1', b'qualified-cockpit')
        for entry, payload in ((core, b'current-core'), (unrelated, b'preserve-unrelated'), (old, b'old-cockpit')):
            write_archive(baseline, entry, payload)
        write_archive(cockpit, new, b'qualified-cockpit')
        base_manifest = {'schema': 1, 'platform': 'linux-amd64', 'packages': {
            'core': core, 'resolvers': [old], 'workflows': [unrelated]}}
        candidate = {'schema': 1, 'platform': 'linux-amd64', 'packages': {
            'resolvers': [new], 'workflows': []}}
        lock = {'artifact': {'sha256': 'a' * 64}, 'source_sha': 'b' * 40, 'public_redistribution': True}
        receipt = {'schema': 1, 'platform': 'linux-amd64', 'qt_version': '6.10.3',
                   'source_sha': 'd' * 40,
                   'sdk_sha256': lock['artifact']['sha256'], 'sdk_source_sha': lock['source_sha'],
                   'resolver': new,
                   'checks': ['desktop-ctest', 'x11-spatial-gpu', 'x11-package-rendering', 'private-symbol-split',
                              'bundled-runtime-closure', 'bundled-x11-package-rendering']}
        for directory, manifest in ((baseline, base_manifest), (cockpit, candidate)):
            (directory / 'packages-store-manifest.json').write_text(json.dumps(manifest))
        (cockpit / 'cockpit-qualification.json').write_text(json.dumps(receipt))
        return baseline, cockpit, base_manifest, candidate, receipt, lock

    def test_cockpit_finalization_preserves_core_and_all_unowned_bytes(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            baseline, cockpit, base, candidate, receipt, lock = self.cockpit_inputs(root)
            read_json = ingest.read_json

            def read(path, maximum):
                return lock if path.name == 'forgepipe-sdk.lock.json' else read_json(path, maximum)

            with mock.patch.object(ingest, 'read_json', side_effect=read), \
                    mock.patch.dict(ingest.os.environ, {'DOCKPIPE_RELEASE_SOURCE_SHA': 'd' * 40}):
                result = ingest.finalize_cockpit(baseline, cockpit, root / 'out', '0.6.20')
            self.assertEqual(result['packages']['core'], base['packages']['core'])
            self.assertEqual(result['packages']['workflows'], base['packages']['workflows'])
            self.assertEqual(result['packages']['resolvers'], candidate['packages']['resolvers'])
            for entry in (base['packages']['core'], base['packages']['workflows'][0]):
                self.assertEqual((root / 'out' / entry['tarball']).read_bytes(), (baseline / entry['tarball']).read_bytes())
            self.assertFalse((root / 'out' / base['packages']['resolvers'][0]['tarball']).exists())

    def test_cockpit_finalization_rejects_wrong_owner_platform_sdk_and_modified_archive(self):
        for failure in ('owner', 'platform', 'sdk', 'source', 'checks', 'core-version', 'archive'):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                baseline, cockpit, base, candidate, receipt, lock = self.cockpit_inputs(root)
                version = '0.6.20'
                if failure == 'owner':
                    candidate['packages']['workflows'] = [base['packages']['workflows'][0]]
                elif failure == 'platform':
                    candidate['platform'] = 'linux-arm64'
                elif failure == 'sdk':
                    receipt['sdk_sha256'] = 'c' * 64
                elif failure == 'source':
                    receipt['source_sha'] = 'e' * 40
                elif failure == 'checks':
                    receipt['checks'].remove('x11-package-rendering')
                elif failure == 'core-version':
                    version = '0.6.21'
                else:
                    (cockpit / candidate['packages']['resolvers'][0]['tarball']).write_bytes(b'corrupted')
                (cockpit / 'packages-store-manifest.json').write_text(json.dumps(candidate))
                (cockpit / 'cockpit-qualification.json').write_text(json.dumps(receipt))
                read_json = ingest.read_json

                def read(path, maximum):
                    return lock if path.name == 'forgepipe-sdk.lock.json' else read_json(path, maximum)

                with mock.patch.object(ingest, 'read_json', side_effect=read), \
                        mock.patch.dict(ingest.os.environ, {'DOCKPIPE_RELEASE_SOURCE_SHA': 'd' * 40}):
                    with self.assertRaises(ValueError):
                        ingest.finalize_cockpit(baseline, cockpit, root / 'out', version)
                self.assertFalse((root / 'out').exists())

    def test_replaces_core_and_owned_identity_while_retaining_unrelated_archive(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            baseline_root = root / 'baseline'
            baseline_root.mkdir()
            core_root = root / 'core'
            core_root.mkdir()
            main_root = root / 'main'
            main_root.mkdir()
            candidate_root = root / 'candidate'
            candidate_root.mkdir()
            old_core = archive_entry('core', 'dockpipe.core', '0.6.16', b'old-core')
            new_core = archive_entry('core', 'dockpipe.core', '0.6.17', b'new-core')
            unrelated = archive_entry('resolver', 'unrelated', '0.6.1', b'unrelated')
            old_owned = archive_entry('workflow', 'pipeon', '0.6.3', b'old-pipeon')
            new_owned = archive_entry('workflow', 'pipeon', '0.6.4', b'new-pipeon')
            old_wrapper = archive_entry('workflow', 'dockpipe-repo-quality', '0.6.16', b'old-wrapper')
            new_wrapper = archive_entry('workflow', 'dockpipe-repo-quality', '0.6.17', b'new-wrapper')
            new_owned['min_dockpipe_version'] = '0.6.17'
            for entry, contents in ((old_core, b'old-core'), (unrelated, b'unrelated'),
                                    (old_owned, b'old-pipeon'), (old_wrapper, b'old-wrapper')):
                write_archive(baseline_root, entry, contents)
            write_archive(core_root, new_core, b'new-core')
            write_archive(main_root, new_wrapper, b'new-wrapper')
            write_archive(candidate_root, new_owned, b'new-pipeon')
            baseline = {'schema': 1, 'platform': 'linux-amd64', 'packages': {
                'core': old_core, 'resolvers': [unrelated], 'workflows': [old_owned, old_wrapper]}}
            core = {'schema': 1, 'platform': 'linux-amd64', 'packages': {
                'core': new_core, 'resolvers': [], 'workflows': []}}
            main = {'schema': 1, 'platform': 'linux-amd64', 'packages': {
                'resolvers': [], 'workflows': [new_wrapper]}}
            candidate = {'schema': 1, 'platform': 'linux-amd64', 'packages': {
                'resolvers': [], 'workflows': [new_owned]}}
            output = root / 'composed'
            lock = {'linux_candidates': {
                'pipeon': {'version': '0.6.4', 'owned': [['workflow', 'pipeon']]}},
                'hidden_identities': [['workflow', 'pipeon']]}
            manifest = ingest.compose_inputs(
                lock, 'linux-amd64', baseline, baseline_root,
                {'pipeon': (candidate, candidate_root)}, core, core_root, main, main_root, output)
            self.assertEqual(manifest['packages']['core']['version'], '0.6.17')
            workflows = {entry['name']: entry for entry in manifest['packages']['workflows']}
            self.assertEqual(workflows['pipeon']['version'], '0.6.4')
            self.assertTrue(workflows['pipeon']['hidden'])
            self.assertEqual(workflows['pipeon']['sha256'], new_owned['sha256'])
            self.assertEqual(workflows['dockpipe-repo-quality']['version'], '0.6.17')
            self.assertEqual(workflows['dockpipe-repo-quality']['sha256'], new_wrapper['sha256'])
            self.assertEqual(manifest['packages']['resolvers'][0], unrelated)
            self.assertEqual((output / unrelated['tarball']).read_bytes(), b'unrelated')
            self.assertEqual((output / new_core['tarball']).read_bytes(), b'new-core')
            self.assertEqual((output / new_owned['tarball']).read_bytes(), b'new-pipeon')
            self.assertEqual((output / new_wrapper['tarball']).read_bytes(), b'new-wrapper')
            self.assertFalse((output / old_owned['tarball']).exists())
            self.assertFalse((output / old_wrapper['tarball']).exists())
            with self.assertRaises(ValueError):
                ingest.compose_inputs({}, 'darwin-amd64', baseline, baseline_root,
                                      {'pipeon': (candidate, candidate_root)}, core,
                                      core_root, main, main_root, root / 'invalid')
            invalid_main = {'schema': 1, 'platform': 'linux-amd64', 'packages': {
                'resolvers': [], 'workflows': []}}
            with self.assertRaises(ValueError):
                ingest.compose_inputs(lock, 'linux-amd64', baseline, baseline_root,
                                      {'pipeon': (candidate, candidate_root)}, core,
                                      core_root, invalid_main, main_root, root / 'invalid-main')
            overlapping = copy.deepcopy(lock)
            overlapping['linux_candidates']['pipeon']['owned'].append(
                ['workflow', 'dockpipe-repo-quality'])
            with self.assertRaisesRegex(ValueError, 'External candidate cannot own'):
                ingest.compose_inputs(overlapping, 'linux-amd64', baseline, baseline_root,
                                      {'pipeon': (candidate, candidate_root)}, core,
                                      core_root, main, main_root, root / 'invalid-owner')

    def test_main_wrapper_version_is_independent_of_newer_core(self):
        cases = (
            ('newer_core', '0.6.16', '0.6.17', b'new-wrapper', None),
            ('same_version_same_archive', '0.6.17', '0.6.17', b'old-wrapper', None),
            ('same_version_changed_archive', '0.6.17', '0.6.17',
             b'bad-wrapper', 'immutable archive'),
            ('downgrade', '0.6.17', '0.6.16', b'older-wrapper', 'cannot downgrade'),
        )
        for name, old_version, version, contents, error in cases:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                baseline_root = root / 'baseline'
                core_root = root / 'core'
                main_root = root / 'main'
                for directory in (baseline_root, core_root, main_root):
                    directory.mkdir()

                old_core = archive_entry('core', 'dockpipe.core', '0.6.16', b'old-core')
                new_core = archive_entry('core', 'dockpipe.core', '0.6.18', b'new-core')
                old_wrapper = archive_entry('workflow', 'dockpipe-repo-quality',
                                            old_version, b'old-wrapper')
                new_wrapper = archive_entry('workflow', 'dockpipe-repo-quality',
                                            version, contents)
                write_archive(baseline_root, old_core, b'old-core')
                write_archive(baseline_root, old_wrapper, b'old-wrapper')
                write_archive(core_root, new_core, b'new-core')
                write_archive(main_root, new_wrapper, contents)

                baseline = {'schema': 1, 'platform': 'darwin-amd64', 'packages': {
                    'core': old_core, 'resolvers': [], 'workflows': [old_wrapper]}}
                core = {'schema': 1, 'platform': 'darwin-amd64', 'packages': {
                    'core': new_core, 'resolvers': [], 'workflows': []}}
                main = {'schema': 1, 'platform': 'darwin-amd64', 'packages': {
                    'resolvers': [], 'workflows': [new_wrapper]}}
                lock = {'linux_candidates': {}, 'hidden_identities': []}
                output = root / 'composed'

                if error:
                    with self.assertRaisesRegex(ValueError, error):
                        ingest.compose_inputs(lock, 'darwin-amd64', baseline, baseline_root,
                                              {}, core, core_root, main, main_root, output)
                    self.assertFalse(output.exists())
                else:
                    manifest = ingest.compose_inputs(lock, 'darwin-amd64', baseline,
                                                     baseline_root, {}, core, core_root,
                                                     main, main_root, output)
                    self.assertEqual(manifest['packages']['core']['version'], '0.6.18')
                    self.assertEqual(manifest['packages']['workflows'][0]['version'], version)
                    self.assertEqual((output / new_wrapper['tarball']).read_bytes(), contents)

    def test_retained_platform_hides_owned_identity_without_changing_archive(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            baseline_root = root / 'baseline'
            baseline_root.mkdir()
            core_root = root / 'core'
            core_root.mkdir()
            main_root = root / 'main'
            main_root.mkdir()
            old_core = archive_entry('core', 'dockpipe.core', '0.6.16', b'old-core')
            new_core = archive_entry('core', 'dockpipe.core', '0.6.17', b'new-core')
            retained = archive_entry('workflow', 'pipeon', '0.6.3', b'retained-pipeon')
            old_wrapper = archive_entry('workflow', 'dockpipe-repo-quality', '0.6.16', b'old-wrapper')
            new_wrapper = archive_entry('workflow', 'dockpipe-repo-quality', '0.6.17', b'new-wrapper')
            unrelated = archive_entry('workflow', 'unrelated', '0.6.1', b'unrelated')
            unrelated['hidden'] = False
            for entry, contents in ((old_core, b'old-core'), (retained, b'retained-pipeon'),
                                    (old_wrapper, b'old-wrapper'),
                                    (unrelated, b'unrelated')):
                write_archive(baseline_root, entry, contents)
            write_archive(core_root, new_core, b'new-core')
            write_archive(main_root, new_wrapper, b'new-wrapper')
            baseline = {'schema': 1, 'platform': 'darwin-amd64', 'packages': {
                'core': old_core, 'resolvers': [], 'workflows': [retained, unrelated, old_wrapper]}}
            core = {'schema': 1, 'platform': 'darwin-amd64', 'packages': {
                'core': new_core, 'resolvers': [], 'workflows': []}}
            main = {'schema': 1, 'platform': 'darwin-amd64', 'packages': {
                'resolvers': [], 'workflows': [new_wrapper]}}
            lock = {'linux_candidates': {
                'pipeon': {'owned': [['workflow', 'pipeon']]}},
                'hidden_identities': [['workflow', 'pipeon']]}
            for platform in ('linux-arm64', 'darwin-amd64', 'darwin-arm64', 'windows-amd64'):
                with self.subTest(platform=platform):
                    baseline['platform'] = platform
                    core['platform'] = platform
                    main['platform'] = platform
                    output = root / platform
                    manifest = ingest.compose_inputs(lock, platform, baseline, baseline_root,
                                                    {}, core, core_root, main, main_root, output)
                    entries = {entry['name']: entry for entry in manifest['packages']['workflows']}
                    self.assertTrue(entries['pipeon']['hidden'])
                    self.assertEqual(entries['pipeon']['version'], '0.6.3')
                    self.assertEqual(entries['pipeon']['sha256'], retained['sha256'])
                    self.assertEqual(entries['unrelated'], unrelated)
                    self.assertEqual(entries['dockpipe-repo-quality']['version'], '0.6.17')
                    self.assertEqual(entries['dockpipe-repo-quality']['sha256'], new_wrapper['sha256'])
                    self.assertEqual((output / retained['tarball']).read_bytes(), b'retained-pipeon')
                    self.assertEqual((output / unrelated['tarball']).read_bytes(), b'unrelated')
                    self.assertFalse((output / old_wrapper['tarball']).exists())
                    self.assertEqual((output / new_wrapper['tarball']).read_bytes(), b'new-wrapper')


if __name__ == '__main__':
    unittest.main()
