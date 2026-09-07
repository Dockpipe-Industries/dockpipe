"""Fail-closed identities and immutable snapshots for the opt-in experiment."""
import json
import os
from pathlib import Path
import tempfile
import unittest

import shared_runtime_probe as probe


class SharedRuntimeIdentityTests(unittest.TestCase):
    def test_current_fixture_is_not_a_compiler_input(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = root / 'source'
            source.mkdir()
            (source / 'generated.go').write_text('package generated\n')
            (source / 'current.json').write_text('[1]')
            settings = root / 'settings.json'
            settings.write_text('{"flags": "-linkshared"}')
            record = dict(inputs=probe.inputs(source), settings_sha256=probe.digest(settings))
            (source / 'current.json').write_text('[999]')
            probe.validate_record(record, source, settings)
            (source / 'generated.go').write_text('package changed\n')
            with self.assertRaisesRegex(ValueError, 'source'):
                probe.validate_record(record, source, settings)

    def test_flags_and_toolchain_bytes_invalidate(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = root / 'source'
            source.mkdir()
            settings = root / 'settings.json'
            settings.write_text('{"flags": "-linkshared"}')
            record = dict(inputs={}, settings_sha256=probe.digest(settings))
            settings.write_text('{"flags": "different"}')
            with self.assertRaisesRegex(ValueError, 'identity'):
                probe.validate_record(record, source, settings)
            compiler = root / 'compiler'
            compiler.write_bytes(b'original toolchain at the same path')
            identity = dict(toolchain={str(compiler): probe.digest(compiler)})
            probe.validate_toolchain(identity)
            compiler.write_bytes(b'replaced toolchain')
            with self.assertRaisesRegex(ValueError, 'toolchain'):
                probe.validate_toolchain(identity)

    @unittest.skipUnless(hasattr(os, 'memfd_create'), 'Linux sealed experiment')
    def test_snapshot_survives_path_replacement_and_refuses_writes(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'library.so'
            path.write_bytes(b'original shared library')
            expected = probe.digest(path)
            fd = probe.seal(path, expected)
            try:
                replacement = Path(tmp) / 'replacement'
                replacement.write_bytes(b'changed shared library')
                replacement.replace(path)
                self.assertEqual(os.pread(fd, 100, 0), b'original shared library')
                with self.assertRaises(OSError):
                    os.pwrite(fd, b'changed', 0)
                with self.assertRaises(OSError):
                    os.ftruncate(fd, 0)
                with self.assertRaisesRegex(ValueError, 'digest'):
                    probe.seal(path, expected)
            finally:
                os.close(fd)


if __name__ == '__main__':
    unittest.main()
