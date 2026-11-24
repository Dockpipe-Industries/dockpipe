"""Installed browser fixture boundaries; actual GUI behavior is qualified locally."""
import importlib.util
import hashlib
import json
from pathlib import Path
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1] / 'assets/scripts'
SPEC = importlib.util.spec_from_file_location('browser_product', ROOT / 'installed-browser.py')
BROWSER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(BROWSER)


class BrowserProductTests(unittest.TestCase):
    def test_generated_routes_cannot_serve_user_files_or_other_origins(self):
        nonce = '1' * 32
        self.assertEqual(BROWSER.HTTP.route('/' + nonce + '/a', nonce), 'a')
        self.assertEqual(BROWSER.HTTP.route('/' + nonce + '/download.txt', nonce), 'download.txt')
        for value in ['/etc/passwd', '/' + nonce + '/../private', '/' + nonce + '/a?redirect=elsewhere',
                      'http://another-host/' + nonce + '/a', '/' + '2' * 32 + '/a']:
            with self.subTest(path=value):
                self.assertIsNone(BROWSER.HTTP.route(value, nonce))

    def test_dom_and_download_payloads_use_only_generated_nonce_material(self):
        nonce = '3' * 32
        source = BROWSER.HTTP.page(nonce, 'a').decode()
        self.assertIn('localStorage', source)
        self.assertIn("record('storage', stored)", source)
        self.assertNotIn('https://', source)
        self.assertNotIn('file://', source)
        self.assertEqual(BROWSER.HTTP.payload(nonce), ('Dockpipe generated browser download\nfixture=' + nonce + '\n').encode())

    def test_wrapper_keeps_isolated_network_and_executes_normal_installed_entrypoint(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            BROWSER.prepare(output)
            command = ['flatpak', 'run', '--unshare=network', '--command=dockpipe-launcher',
                       'com.dockpipe.Dockpipe', '--allow-second-instance']
            selected = BROWSER.launch_command(command, output, 1)
            self.assertIn('--unshare=network', selected)
            self.assertIn('--noprofile', selected)
            self.assertIn('--norc', selected)
            self.assertNotIn('--no-sandbox', selected)
            self.assertIn('exec /app/bin/dockpipe-launcher --allow-second-instance',
                          (output / 'browser-start.sh').read_text())
            with self.assertRaisesRegex(ValueError, 'isolated runtime-local network'):
                BROWSER.launch_command([item for item in command if item != '--unshare=network'], output, 1)

    def test_metadata_refuses_a_foreign_nonce_or_invalid_endpoint(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            selected = BROWSER.prepare(output)
            nonce = selected['nonce']
            value = {'schema': BROWSER.HTTP.SCHEMA, 'nonce': nonce, 'launch': 1, 'port': 23123,
                     'namespace_pid': 3, 'payload_sha256': BROWSER.digest(output / 'browser-fixture.py')}
            path = output / 'browser-server-1.json'
            path.write_text(json.dumps(value))
            with self.assertRaisesRegex(ValueError, 'fixture identity differs'):
                BROWSER.metadata(output, 1)
            value['payload_sha256'] = hashlib.sha256(BROWSER.HTTP.payload(nonce)).hexdigest()
            path.write_text(json.dumps(value))
            self.assertEqual(BROWSER.metadata(output, 1)['nonce'], nonce)
            value['nonce'] = '0' * 32
            path.write_text(json.dumps(value))
            with self.assertRaisesRegex(ValueError, 'fixture identity differs'):
                BROWSER.metadata(output, 1)

    def test_cooperative_stop_is_bound_to_the_exact_generated_launch(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            selected = BROWSER.prepare(output)
            nonce = selected['nonce']
            self.assertFalse(BROWSER.HTTP.stop_requested(output, nonce, 1))
            BROWSER.request_stop(output, 1)
            self.assertTrue(BROWSER.HTTP.stop_requested(output, nonce, 1))
            self.assertFalse(BROWSER.HTTP.stop_requested(output, nonce, 2))
            with self.assertRaisesRegex(ValueError, 'another invocation'):
                BROWSER.HTTP.stop_requested(output, '0' * 32, 1)

    def test_saved_download_requires_exact_file_and_actual_public_status(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            selected = BROWSER.prepare(output)
            nonce = selected['nonce']
            home = output / 'home'
            directory = home / 'Downloads'
            directory.mkdir(parents=True)
            filename = 'dockpipe-browser-' + nonce[:8] + '.txt'
            download = directory / filename
            download.write_bytes(BROWSER.HTTP.payload(nonce))
            for launch in [1, 2]:
                metadata = {'schema': BROWSER.HTTP.SCHEMA, 'nonce': nonce, 'launch': launch,
                            'port': 23123, 'namespace_pid': 3,
                            'payload_sha256': hashlib.sha256(download.read_bytes()).hexdigest()}
                (output / ('browser-server-' + str(launch) + '.json')).write_text(json.dumps(metadata))
                (output / ('browser-server-' + str(launch) + '.jsonl')).write_text(
                    json.dumps({'route': 'download.txt'}) + '\n' if launch == 1 else '')
            observations = [{'phase': 'first', 'items': [
                {'role': 'browser-tab', 'text': 'Dockpipe fixture A'},
                {'role': 'browser-tab', 'text': 'Dockpipe fixture B'},
                {'role': 'browser-state', 'text': 'Generated DOM accepted ' + nonce[:8],
                 'state': 'retained after app navigation'},
                {'role': 'download-row', 'text': filename, 'state': 'Saved'},
            ]}]
            settings = {'browser': {'downloadDirectory': str(directory)}}
            self.assertTrue(BROWSER.evaluate(output, home, settings, settings, observations)['tabs_and_downloads'])
            (output / 'browser-independent-readback.json').unlink()
            observations[0]['items'][-1]['state'] = 'Downloading'
            self.assertFalse(BROWSER.evaluate(output, home, settings, settings, observations)['tabs_and_downloads'])
            (output / 'browser-independent-readback.json').unlink()
            observations[0]['items'][-1]['state'] = 'Saved'
            download.write_bytes(b'wrong generated payload')
            self.assertFalse(BROWSER.evaluate(output, home, settings, settings, observations)['tabs_and_downloads'])


if __name__ == '__main__':
    unittest.main()
