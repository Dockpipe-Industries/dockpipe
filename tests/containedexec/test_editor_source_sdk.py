#!/usr/bin/env python3
"""Exercise exact editor SDK admission with an explicitly supplied released SDK."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
from types import SimpleNamespace
from unittest.mock import patch
import unittest

import editor_source_sdk
from editor_source_sdk import FILES, source_inventory, verify_editor_source
import verification

TEST_SOURCE = Path(os.environ.get('DORKPIPE_EDITOR_TEST_SOURCE', ''))
SOURCE_SHA = 'a' * 40


def canonical_json(value):
    return (json.dumps(value, sort_keys=True, indent=2) + '\n').encode()


def fixture(directory, editor_version='0.0.1'):
    if not TEST_SOURCE.is_absolute():
        raise ValueError('Set DORKPIPE_EDITOR_TEST_SOURCE to an absolute extracted editor SDK path')
    source_inventory(TEST_SOURCE)
    source = directory / 'dockpipe-language-support'
    source.mkdir()
    for relative in FILES:
        destination = source / relative
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(TEST_SOURCE / relative, destination)
    package_path = source / 'package.json'
    package = json.loads(package_path.read_text())
    package['version'] = editor_version
    package_path.write_bytes(canonical_json(package))
    files = source_inventory(source)
    source_digest = hashlib.sha256(json.dumps(files, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
    manifest = {'schema': 1, 'editor_version': editor_version,
                'source_dorkpipe_sha': SOURCE_SHA, 'source_digest': source_digest,
                'files': files}
    manifest_data = canonical_json(manifest)
    (source / 'editor-source.json').write_bytes(manifest_data)
    lock = {'schema': 1, 'editor_version': editor_version,
            'source_dorkpipe_sha': SOURCE_SHA, 'source_digest': source_digest,
            'sdk_manifest_sha256': hashlib.sha256(manifest_data).hexdigest()}
    lock_path = directory / 'editor-source.lock.json'
    lock_path.write_bytes(canonical_json(lock))
    return source, lock_path


class EditorSourceSDKTests(unittest.TestCase):
    def test_version_is_pinned_and_matches_package(self):
        with tempfile.TemporaryDirectory() as temporary:
            source, lock = fixture(Path(temporary), editor_version='0.0.2')
            self.assertEqual(verify_editor_source(source, lock), source)
            manifest_path = source / 'editor-source.json'
            manifest = json.loads(manifest_path.read_text())
            manifest['editor_version'] = '0.0.1'
            manifest_data = canonical_json(manifest)
            manifest_path.write_bytes(manifest_data)
            pin = json.loads(lock.read_text())
            pin['editor_version'] = '0.0.1'
            pin['sdk_manifest_sha256'] = hashlib.sha256(manifest_data).hexdigest()
            lock.write_bytes(canonical_json(pin))
            with self.assertRaisesRegex(ValueError, 'package version differs'):
                verify_editor_source(source, lock)
            for invalid_version in ('0.0.2-', '01.0.2', '0.0.2-01'):
                pin['editor_version'] = invalid_version
                lock.write_bytes(canonical_json(pin))
                with self.assertRaisesRegex(ValueError, 'invalid editor version'):
                    verify_editor_source(source, lock)

    def test_real_editor_assertions_run_from_verified_sdk(self):
        with tempfile.TemporaryDirectory() as temporary:
            source, lock = fixture(Path(temporary))
            self.assertEqual(verify_editor_source(source, lock), source)
            node = subprocess.run(['node', '--test', 'extension.test.js'], cwd=source,
                                  capture_output=True, text=True, timeout=30)
            self.assertEqual(node.returncode, 0, node.stdout + node.stderr)
            self.assertIn('pass 1', node.stdout)
            self.assertIn('fail 0', node.stdout)

    def test_file_growth_during_read_is_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / 'source'
            path.write_bytes(b'original')
            actual = path.stat()
            grown = SimpleNamespace(st_dev=actual.st_dev, st_ino=actual.st_ino,
                                    st_mode=actual.st_mode, st_size=actual.st_size + 1,
                                    st_mtime_ns=actual.st_mtime_ns)
            with patch.object(editor_source_sdk.os, 'fstat', side_effect=[actual, grown]):
                with self.assertRaisesRegex(ValueError, 'grew or changed'):
                    editor_source_sdk.regular_bytes(path, 1024)

    def test_integration_inputs_include_source_and_committed_lock(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source, fixture_lock = fixture(root)
            release = root / 'release'
            release.mkdir()
            lock = release / 'editor-source.lock.json'
            shutil.copyfile(fixture_lock, lock)
            with patch.object(verification, 'ROOT', root):
                with patch.dict('os.environ', {'DORKPIPE_EDITOR_TEST_SOURCE': str(source)}):
                    inputs = verification.source_paths('integration')
            self.assertIn(source, inputs)
            self.assertIn(lock, inputs)

    def test_changed_and_extra_source_are_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            source, lock = fixture(Path(temporary))
            (source / 'extension.js').write_bytes(b'changed')
            with self.assertRaisesRegex(ValueError, 'manifest differs'):
                verify_editor_source(source, lock)
            shutil.copyfile(TEST_SOURCE / 'extension.js', source / 'extension.js')
            (source / 'unlisted.js').write_text('extra')
            with self.assertRaisesRegex(ValueError, 'unknown source file'):
                verify_editor_source(source, lock)

    def test_link_and_lock_drift_are_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            source, lock = fixture(Path(temporary))
            target = source / 'README.md'
            target.unlink()
            target.symlink_to(TEST_SOURCE / 'README.md')
            with self.assertRaisesRegex(ValueError, 'linked'):
                verify_editor_source(source, lock)
            target.unlink()
            shutil.copyfile(TEST_SOURCE / 'README.md', target)
            pin = json.loads(lock.read_text())
            pin['source_digest'] = '0' * 64
            lock.write_bytes(canonical_json(pin))
            with self.assertRaisesRegex(ValueError, 'manifest differs'):
                verify_editor_source(source, lock)

    def test_traversal_and_missing_manifest_are_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            source, lock = fixture(Path(temporary))
            manifest = json.loads((source / 'editor-source.json').read_text())
            manifest['files'][0]['path'] = '../escape'
            (source / 'editor-source.json').write_bytes(canonical_json(manifest))
            with self.assertRaisesRegex(ValueError, 'manifest differs'):
                verify_editor_source(source, lock)
            (source / 'editor-source.json').unlink()
            with self.assertRaises(FileNotFoundError):
                verify_editor_source(source, lock)


if __name__ == '__main__':
    unittest.main()
