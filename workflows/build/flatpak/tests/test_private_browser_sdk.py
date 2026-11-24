"""Private proof bindings stay separate from the public attested Browser SDK path."""
import hashlib
import importlib.util
import json
from pathlib import Path
import tarfile
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / 'assets/scripts/prepare-private-browser-sdk.py'
SPEC = importlib.util.spec_from_file_location('private_browser_consumer_test', SCRIPT)
CONSUMER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CONSUMER)


def checksum(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


class PrivateBrowserSDKTest(unittest.TestCase):
    def fixture(self, root):
        producer = root / 'producer'
        producer.mkdir()
        prefix = root / 'owning-sdk'
        files = {
            'include/SpatialWeb/Browser/BrowserWidget.h': b'private interface fixture\n',
            'lib/cmake/SpatialWebFlatpakBrowser/SpatialWebFlatpakBrowserConfig.cmake': b'private CMake fixture\n',
            'lib/cmake/SpatialWebFlatpakBrowser/SpatialWebFlatpakBrowserConfigVersion.cmake': b'version fixture\n',
            'lib/cmake/SpatialWebFlatpakBrowser/SpatialWebFlatpakBrowserTargets.cmake': b'target fixture\n',
            'lib/libSpatialWebBrowser.so.0.1.0': b'synthetic SDK bytes; never loaded\n',
            'share/SpatialWebBrowser/LICENSE': b'Apache-2.0 fixture\n',
        }
        entries = []
        for relative, data in files.items():
            path = prefix / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
            entries.append({'path': relative, 'bytes': len(data), 'sha256': checksum(path)})
        manifest = producer / (CONSUMER.STEM + '.files.json')
        manifest.write_text(json.dumps({'schema': 1, 'artifact_class': 'sdk', 'files': entries}))
        archive = producer / (CONSUMER.STEM + '.tar.gz')
        with tarfile.open(archive, 'w:gz') as output:
            for row in entries:
                output.add(prefix / row['path'], arcname=row['path'])
        source = 'a' * 40
        checks = ['sdk-component-install', 'focused-browser-tests',
                  'relocated-external-consumer-compile-link-run', 'flatpak-installed-runtime-dom']
        runtime = root / 'installed-runtime-proof.json'
        runtime.write_text(json.dumps({'check': 'flatpak-installed-runtime-dom', 'dom': 'real-dom', 'sandbox': 'default',
            'Qt6Core': '6.10.3', 'Qt6WebEngineCore': '6.10.2',
            'runtime_commit': CONSUMER.PROFILE.FLATPAK['runtime']['commit'], 'renderer': 'llvmpipe fixture'}))
        qualification = root / 'qualification.json'
        proof = {'schema': 1, 'platform': CONSUMER.PROFILE.PLATFORM, 'source_sha': source, 'source_state': 'local-proof',
                 'compiler': CONSUMER.PROFILE.COMPILER, 'qt_components': CONSUMER.PROFILE.QT_COMPONENTS,
                 'flatpak': CONSUMER.PROFILE.FLATPAK, 'checks': checks,
                 'qualification_driver': {'ref': 'runtime/org.freedesktop.Platform.GL.default/x86_64/25.08', 'commit': 'b' * 64},
                 'runtime_proof_sha256': checksum(runtime)}
        qualification.write_text(json.dumps(proof))
        context = root / 'source-context.json'
        context.write_text(json.dumps({'base_head': source, 'dirty_source_files': []}))
        release = dict(proof)
        release.pop('qualification_driver')
        release.pop('runtime_proof_sha256')
        release.update(product=CONSUMER.PROFILE.PRODUCT, version=CONSUMER.PROFILE.VERSION,
                       source_repository=CONSUMER.PROFILE.REPOSITORY, runtime_bundled=False,
                       abi={'cxx_standard': 17}, archive={'name': archive.name, 'size_bytes': archive.stat().st_size, 'sha256': checksum(archive)},
                       file_manifest={'name': manifest.name, 'sha256': checksum(manifest)})
        release_path = producer / (CONSUMER.STEM + '.release.json')
        release_path.write_text(json.dumps(release))
        selected = {'schema': 'dockpipe.flatpak.private-browser-selection/v1', 'source_sha': source}
        for name, path in [('release', release_path), ('qualification', qualification), ('runtime_proof', runtime), ('source_context', context)]:
            selected[name] = {'path': str(path), 'sha256': checksum(path)}
        selection = root / 'selection.json'
        selection.write_text(json.dumps(selected))
        return selection

    def test_prepare_verify_and_stage_preserve_private_identity(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            selected = self.fixture(root)
            output = root / 'prepared'
            receipt = CONSUMER.prepare(selected, output)
            self.assertFalse(receipt['attestations_verified'])
            self.assertFalse(receipt['public_release'])
            self.assertEqual(receipt['source_state'], 'local-proof')
            CONSUMER.stage(output, selected, root / 'runtime', root / 'licenses')
            self.assertTrue((root / 'runtime/libSpatialWebBrowser.so.0.1.0').is_file())
            self.assertFalse((output / 'consumer-receipt.json').exists())

    def test_tampered_producer_proof_cannot_enter_private_prefix(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            selected = self.fixture(root)
            proof = root / 'installed-runtime-proof.json'
            proof.write_text('{}')
            with self.assertRaisesRegex(ValueError, 'selected digest'):
                CONSUMER.prepare(selected, root / 'prepared')

    def test_disabled_chromium_sandbox_is_rejected_even_when_rebound(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            selected = self.fixture(root)
            selection = json.loads(selected.read_text())
            runtime = Path(selection['runtime_proof']['path'])
            value = json.loads(runtime.read_text())
            value['sandbox'] = 'disabled'
            runtime.write_text(json.dumps(value))
            selection['runtime_proof']['sha256'] = checksum(runtime)
            qualification = Path(selection['qualification']['path'])
            value = json.loads(qualification.read_text())
            value['runtime_proof_sha256'] = checksum(runtime)
            qualification.write_text(json.dumps(value))
            selection['qualification']['sha256'] = checksum(qualification)
            selected.write_text(json.dumps(selection))
            with self.assertRaisesRegex(ValueError, 'Chromium sandbox'):
                CONSUMER.prepare(selected, root / 'prepared')

    def test_payload_mutation_and_extra_file_fail_readback(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            selected = self.fixture(root)
            output = root / 'prepared'
            CONSUMER.prepare(selected, output)
            header = output / 'include/SpatialWeb/Browser/BrowserWidget.h'
            original = header.read_bytes()
            header.write_bytes(b'changed')
            with self.assertRaisesRegex(ValueError, 'exact inventory'):
                CONSUMER.verify(output, selected)
            header.write_bytes(original)
            (output / 'unowned-file').write_text('extra')
            with self.assertRaisesRegex(ValueError, 'extra files'):
                CONSUMER.verify(output, selected)

    def test_private_opt_in_is_not_inherited_by_default_builder(self):
        text = (SCRIPT.parent / 'build.sh').read_text()
        self.assertIn('DOCKPIPE_FLATPAK_ALLOW_PRIVATE_BROWSER:-0', text)
        self.assertIn('A private Browser SDK selection requires explicit local opt-in', text)
        self.assertIn('--lock "$browser_lock" --profile flatpak', text)


if __name__ == '__main__':
    unittest.main()
