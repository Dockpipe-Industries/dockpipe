"""Rejection, exactness and lifetime checks. Integration inputs are explicit env vars."""
import concurrent.futures
import copy
import errno
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from unittest import mock

from shared_runtime_probe import containment, digest
from transcript_runtime import Decoder, Service
from transcript_replay import execute
from transcript_suite import derive_plan, identity, source_identity, validate_plan


class PlanTests(unittest.TestCase):
    def test_topology_rejects_cycles_missing_and_duplicate_jobs(self):
        plan = derive_plan({format(i, '064x'): dict(size=i) for i in range(67)})
        validate_plan(plan)
        leaf = next(k for k in plan['recipes'] if k not in plan['anchors'])
        for change in ['cycle', 'root', 'missing', 'duplicate']:
            broken = copy.deepcopy(plan)
            if change == 'cycle':
                broken['recipes'][leaf]['base'] = leaf
            elif change == 'root':
                broken['recipes'][plan['root']]['base'] = leaf
            elif change == 'missing':
                del broken['recipes'][leaf]
            else:
                broken['schedule'][0][0] = broken['schedule'][1][0]
            with self.subTest(change=change), self.assertRaises(ValueError):
                validate_plan(broken)

    def test_plans_follow_current_inventory_without_corpus_counts(self):
        for count in [1, 2, 17, 67, 201, 2719]:
            records = {format(i, '064x'): dict(size=(i % 17) * 4096) for i in range(count)}
            plan = derive_plan(records)
            self.assertEqual(plan, derive_plan(dict(reversed(list(records.items())))))
            self.assertLessEqual(len(plan['anchors']), 16)
            self.assertEqual(set(plan['recipes']), set(records))
            self.assertTrue(all(len(part) <= 32 for part in plan['schedule']))
            changed = dict(records, **{'f' * 64: dict(size=12345)})
            self.assertIn('f' * 64, derive_plan(changed)['recipes'])

    def test_current_fixture_changes_are_not_source_cache_keys(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / 'x.go').write_text('package x')
            (root / 'fixture.json').write_text('[]')
            original = source_identity(root)
            (root / 'fixture.json').write_text('["current"]')
            self.assertEqual(source_identity(root), original)
            (root / 'x.go').write_text('package y')
            self.assertNotEqual(source_identity(root), original)

    def test_pipe_deadlines_cover_blocked_reader_and_writer(self):
        containment()
        for payload in [{}, {'data': 'x' * 900000}]:
            service = Service.__new__(Service)
            service.fd = None
            service.timeout = .05
            service.p = subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(20)'],
                                         stdin=subprocess.PIPE, stdout=subprocess.PIPE, bufsize=0)
            os.set_blocking(service.p.stdin.fileno(), False)
            os.set_blocking(service.p.stdout.fileno(), False)
            start = time.monotonic()
            try:
                with self.assertRaises(TimeoutError):
                    service.send(payload)
                self.assertLess(time.monotonic() - start, 1)
            finally:
                service.close()


@unittest.skipUnless(os.getenv('PIPELANG_TRANSCRIPT_DIRECTORY'), 'explicit prepared representation required')
class IntegrationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        containment()
        cls.directory = Path(os.environ['PIPELANG_TRANSCRIPT_DIRECTORY'])
        cls.exports = Path(os.environ['PIPELANG_TRANSCRIPT_EXPORTS'])
        cls.value = identity(cls.directory)
        cls.manifest = json.loads((cls.directory / 'manifest.json').read_text())
        cls.recipes = cls.manifest['recipes']
        cls.root = cls.manifest['root']
        cls.leaf = next(k for k, v in cls.recipes.items()
                        if k not in cls.manifest['anchors'] and v['base'] != cls.root)
        cls.base = cls.recipes[cls.leaf]['base']

    def setUp(self):
        support = self.manifest['support']
        self.decoder = Decoder(self.directory / 'libzstd.so', support['libzstd.so'])
        self.service = Service(self.directory / 'splice', support['splice'])
        self.rootdict = self.reference(self.root, None)
        self.basedict = self.reference(self.base, self.rootdict)

    def tearDown(self):
        self.decoder.close()
        self.service.close()

    def reference(self, key, base):
        fd, buffers = self.decoder.splice(self.recipes[key], base, self.service, self.directory, reference=True)
        try:
            self.assertEqual(digest(Path(f'/proc/self/fd/{fd}')), self.recipes[key]['sha256'])
            return tuple(self.decoder.dictionary(buffer) for buffer in buffers)
        finally:
            os.close(fd)

    def test_exact_elf_and_immutable_sealing(self):
        fd, _ = self.decoder.splice(self.recipes[self.leaf], self.basedict, self.service, self.directory)
        try:
            data = Path(f'/proc/self/fd/{fd}').read_bytes()
            self.assertEqual(data, Path(self.value['baseline'][self.leaf]).read_bytes())
            self.assertIn(b'.debug_line', data)
            for operation in [lambda: os.pwrite(fd, b'x', 0), lambda: os.ftruncate(fd, 0),
                              lambda: os.ftruncate(fd, len(data) + 1)]:
                with self.assertRaises(OSError) as caught:
                    operation()
                self.assertEqual(caught.exception.errno, errno.EPERM)
        finally:
            os.close(fd)

    def test_malformed_recipes_recover_descriptors(self):
        original = self.recipes[self.leaf]
        before = len(list(Path(f'/proc/{self.service.p.pid}/fd').iterdir()))
        local = len(list(Path('/proc/self/fd').iterdir()))
        faults = ['output-hash', 'scaffold-hash', 'token-hash', 'output-limit', 'token-limit',
                  'overlap', 'outside', 'token-truncation', 'token-gap', 'missing-span',
                  'shifted-span', 'absolute-path', 'parent-path']
        for fault in faults:
            recipe = copy.deepcopy(original)
            if fault == 'output-hash': recipe['sha256'] = '0' * 64
            elif fault == 'scaffold-hash': recipe['scaffold_sha256'] = '0' * 64
            elif fault == 'token-hash': recipe['tokens_sha256'] = '0' * 64
            elif fault == 'output-limit': recipe['size'] = 65 << 20
            elif fault == 'token-limit': recipe['token_bytes'] = 65 << 20
            elif fault == 'overlap': recipe['spans'][1]['Offset'] = recipe['spans'][0]['Offset']
            elif fault == 'outside': recipe['spans'][0]['Offset'] = recipe['size'] + 1
            elif fault == 'token-truncation': recipe['spans'][-1]['TokenSize'] -= 1
            elif fault == 'token-gap': recipe['spans'][1]['TokenOffset'] += 1
            elif fault == 'missing-span': recipe['spans'].pop()
            elif fault == 'shifted-span': recipe['spans'][0]['Offset'] += 1
            elif fault == 'absolute-path': recipe['encoded']['tokens']['path'] = '/etc/passwd'
            else: recipe['encoded']['tokens']['path'] = '../escape'
            with self.subTest(fault=fault), self.assertRaises((ValueError, OSError)):
                fd, _ = self.decoder.splice(recipe, self.basedict, self.service, self.directory)
                os.close(fd)
            self.assertEqual(len(list(Path(f'/proc/{self.service.p.pid}/fd').iterdir())), before)
            self.assertEqual(len(list(Path('/proc/self/fd').iterdir())), local)

    def test_root_anchor_leaf_corruption_and_wrong_dictionary(self):
        for key, base in [(self.root, None), (self.base, self.rootdict), (self.leaf, self.basedict)]:
            recipe = self.recipes[key]
            for kind in ['scaffold', 'tokens']:
                blobs = {k: (self.directory / v['path']).read_bytes() for k, v in recipe['encoded'].items()}
                data = bytearray(blobs[kind]); data[len(data) // 2] ^= 128; blobs[kind] = bytes(data)
                with self.subTest(key=key, kind=kind), self.assertRaises(ValueError):
                    self.decoder.splice(recipe, base, self.service, self.directory, blobs=blobs)
        with self.assertRaises(ValueError):
            self.decoder.splice(self.recipes[self.leaf], None, self.service, self.directory)

    def test_two_readers_dictionary_lifetime_and_fault_recovery(self):
        before = len(list(Path('/proc/self/fd').iterdir()))
        barrier = threading.Barrier(2)
        def worker(_):
            decoder = Decoder(self.directory / 'libzstd.so', self.manifest['support']['libzstd.so'])
            service = Service(self.directory / 'splice', self.manifest['support']['splice'])
            try:
                barrier.wait(timeout=5)
                for _ in range(4):
                    fd, _ = decoder.splice(self.recipes[self.leaf], self.basedict, service, self.directory)
                    os.close(fd)
                    broken = copy.deepcopy(self.recipes[self.leaf]); broken['sha256'] = '0' * 64
                    with self.assertRaises(ValueError):
                        decoder.splice(broken, self.basedict, service, self.directory)
            finally:
                decoder.close(); service.close()
        with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
            list(pool.map(worker, range(2)))
        self.assertEqual(len(list(Path('/proc/self/fd').iterdir())), before)

    def test_current_value_and_trace_oracle_fail_independently(self):
        for field in ['Value', 'Trace']:
            with tempfile.TemporaryDirectory() as temporary:
                work = Path(temporary)
                source = self.exports / self.leaf
                target = work / self.leaf
                shutil.copytree(source, target)
                case = sorted(target.glob('case*'))[0 if field == 'Value' else 1]
                fixture = next(case.glob('*.json'))
                data = json.loads(fixture.read_text())
                data[0][field] = 'FAULT' if field == 'Value' else ['FAULT']
                fixture.write_text(json.dumps(data))
                fd, _ = self.decoder.splice(self.recipes[self.leaf], self.basedict, self.service, self.directory)
                with self.subTest(field=field), self.assertRaisesRegex(ValueError, 'FAULT'):
                    execute((self.leaf, fd, 0), work, work)

    def test_support_settings_and_toolchain_invalidation(self):
        with mock.patch.dict(os.environ, GOFLAGS='-race'):
            with self.assertRaisesRegex(ValueError, 'settings changed'):
                identity(self.directory)
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            value = copy.deepcopy(self.value)
            tool = root / 'tool'; tool.write_text('current')
            value['toolchain'] = {str(tool): '0' * 64}
            (root / 'identity.json').write_text(json.dumps(value))
            with self.assertRaisesRegex(ValueError, 'toolchain changed'):
                identity(root)
            value['toolchain'] = {}
            value['support'] = {'support': '0' * 64}
            (root / 'support').write_text('changed')
            (root / 'identity.json').write_text(json.dumps(value))
            with self.assertRaisesRegex(ValueError, 'support changed'):
                identity(root)


if __name__ == '__main__':
    unittest.main()
