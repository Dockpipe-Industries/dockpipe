"""Flatpak qualification requires each pinned archive's own profile."""
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[4] / 'workflows/build/flatpak/assets/scripts/build-catalog.py'
spec = importlib.util.spec_from_file_location('flatpak_catalog', SCRIPT)
catalog = importlib.util.module_from_spec(spec)
spec.loader.exec_module(catalog)


def package_archive(directory, kind, name, declared):
    path = directory / f'dockpipe-{kind}-{name}-0.6.1.tar.gz'
    root = f'{kind}s/{name}'
    profile = {
        'schema': 1,
        'artifacts': [{'kind': item_kind, 'name': item_name} for item_kind, item_name in declared],
        'tools': [],
        'host_commands': [],
        'required_files': [],
    }
    with tarfile.open(path, 'w:gz') as archive:
        entry = tarfile.TarInfo(f'{root}/config.yml')
        payload = b'name: example\n'
        entry.size = len(payload)
        archive.addfile(entry, io.BytesIO(payload))
        entry = tarfile.TarInfo(f'{root}/assets/flatpak/package.json')
        payload = json.dumps(profile).encode()
        entry.size = len(payload)
        archive.addfile(entry, io.BytesIO(payload))
    return path


class PinnedProfileTests(unittest.TestCase):
    def test_each_archive_must_declare_own_identity_even_when_profiles_overlap(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            identities = [('resolver', 'pipeon'), ('workflow', 'pipeon')]
            resolver = package_archive(root, 'resolver', 'pipeon', identities)
            workflow = package_archive(root, 'workflow', 'pipeon', identities)
            profiles = catalog.profiles([resolver, workflow])
            self.assertEqual(set(profiles), set(identities))
            missing = package_archive(root, 'workflow', 'pipeon', [('resolver', 'pipeon')])
            with self.assertRaises(ValueError):
                catalog.profiles([resolver, missing])

    def test_native_store_archive_checksum_is_checked(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            archive = package_archive(root, 'workflow', 'pipeon', [('workflow', 'pipeon')])
            digest = hashlib.sha256(archive.read_bytes()).hexdigest()
            entry = {'name': 'pipeon', 'tarball': archive.name,
                     'size_bytes': archive.stat().st_size, 'sha256': digest}
            manifest = {'platform': 'linux-amd64', 'packages': {
                'resolvers': [], 'workflows': [entry]}}
            (root / 'packages-store-manifest.json').write_text(json.dumps(manifest))
            self.assertEqual(catalog.pinned_inputs(root), [archive])
            entry['sha256'] = '0' * 64
            (root / 'packages-store-manifest.json').write_text(json.dumps(manifest))
            with self.assertRaises(ValueError):
                catalog.pinned_inputs(root)


if __name__ == '__main__':
    unittest.main()
