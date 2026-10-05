import json
import os
from pathlib import Path
import tempfile
import unittest

from campaign import InputGuard, fingerprint
from support_inputs import SupportInputs


class SupportInputTests(unittest.TestCase):
    def fixture(self, root):
        compiler = root / 'compiler'
        compiler.write_bytes(b'compiler'); compiler.chmod(0o700)
        header = root / 'header'; header.write_bytes(b'header')
        link = root / 'alias'; link.symlink_to(compiler.name)
        manifest = root / 'inputs.json'
        manifest.write_text(json.dumps(dict(version=1, files=[str(compiler), str(header)],
                                           native_cxx=str(compiler), links={str(link): compiler.name})))
        return manifest, compiler, header, link

    def test_support_bytes_and_alias_retarget_invalidate(self):
        with tempfile.TemporaryDirectory() as tmp:
            manifest, compiler, header, link = self.fixture(Path(tmp))
            support = SupportInputs(manifest)
            before = fingerprint(support.paths)['digest']
            guard = InputGuard(support.paths, watch_directories=support.watch_directories)
            try:
                header.write_bytes(b'changed')
                with self.assertRaises(RuntimeError): guard.check()
                self.assertNotEqual(before, fingerprint(support.paths)['digest'])
            finally: guard.close()
            guard = InputGuard(support.paths, watch_directories=support.watch_directories)
            try:
                link.unlink(); link.symlink_to(header.name)
                with self.assertRaises(RuntimeError): support.check()
                with self.assertRaises(RuntimeError): guard.check()
            finally: guard.close()

    def test_native_cases_require_declared_compiler_and_use_exact_pin(self):
        self.assertEqual(SupportInputs().native_environment(['TestV1140Blocks']), [])
        with self.assertRaisesRegex(RuntimeError, 'require --support-inputs'):
            SupportInputs().native_environment(['TestNativeStreamsGeneratedRuntime'])
        with tempfile.TemporaryDirectory() as tmp:
            manifest, compiler, _, _ = self.fixture(Path(tmp))
            support = SupportInputs(manifest)
            self.assertEqual(support.native_environment(['TestNativeStreamCompositionValueTrace']),
                             ['PIPELANG_TEST_CXX=' + str(compiler), 'PATH=/usr/bin:/bin'])
            compiler.unlink()
            with self.assertRaises(ValueError): SupportInputs(manifest)

    def test_undeclared_or_linked_compiler_is_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            manifest, compiler, header, link = self.fixture(Path(tmp))
            obj = json.loads(manifest.read_text()); obj['native_cxx'] = str(link)
            manifest.write_text(json.dumps(obj))
            with self.assertRaises(ValueError): SupportInputs(manifest)
            obj['native_cxx'] = str(compiler); obj['files'] = [str(header)]
            manifest.write_text(json.dumps(obj))
            with self.assertRaises(ValueError): SupportInputs(manifest)


if __name__ == '__main__':
    unittest.main()
