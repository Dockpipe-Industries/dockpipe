"""Real external SDK and native-store lifecycle; run under run.py."""
import copy
import hashlib
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import sys
from native_artifacts import run
from native_nucleon import Store, initialize
from shared_runtime_probe import containment, dump

@unittest.skipUnless(os.environ.get('NUCLEON_TEST_SDK'), 'external SDK required')
class NucleonLifecycleTests(unittest.TestCase):
    def setUp(self):
        containment()
        temp = tempfile.TemporaryDirectory(); self.addCleanup(temp.cleanup)
        self.work = Path(temp.name); self.cache = self.work / 'cache'; self.cache.mkdir(mode=0o700)
        self.config = initialize(self.work / 'store', Path(os.environ['NUCLEON_TEST_SDK']))
        self.store = Store(self.cache, self.config); self.addCleanup(self.store.close)
        self.key = 'a' * 64; self.data = bytes(range(256)) * 300 + b'tail'
        self.expected = hashlib.sha256(self.data).hexdigest(); self.add_key(self.key, self.data)

    def add_key(self, key, data):
        target = self.cache / key; target.mkdir(); (target / 'program.test').write_bytes(data)
        dump(target / 'record.json', dict(Key=key, Version='pipelang-native-validation-v2', BinarySHA256=hashlib.sha256(data).hexdigest()))

    def test_new_key_sealed_replay_without_original(self):
        recipe = self.store.prepare(self.key); self.assertLess(recipe['packed_bytes'], len(self.data))
        (self.cache / self.key / 'program.test').unlink()
        before = len(list(Path('/proc/self/fd').iterdir()))
        for _ in range(3):
            fd = self.store.acquire(self.key, self.expected)
            try:
                self.assertEqual(os.pread(fd, len(self.data), 0), self.data)
                with self.assertRaises(OSError): os.pwrite(fd, b'changed', 0)
            finally: os.close(fd)
        self.assertEqual(before, len(list(Path('/proc/self/fd').iterdir())))
        self.add_key('b' * 64, self.data + b'new')
        os.close(self.store.acquire('b' * 64, self.store.record('b' * 64)))

    def test_record_and_payload_corruption_fail_closed(self):
        with self.assertRaisesRegex(ValueError, 'requested native digest'): self.store.acquire(self.key, '0' * 64)
        self.store.prepare(self.key)
        payload = Path(self.config['root']) / self.key / 'program.nuc'; data = payload.read_bytes()
        payload.write_bytes(b'x' + data[1:])
        with self.assertRaisesRegex(ValueError, 'packed digest'): self.store.acquire(self.key, self.expected)
        payload.write_bytes(data)
        record = self.cache / self.key / 'record.json'; value = json.loads(record.read_text())
        value['BinarySHA256'] = '0' * 64; dump(record, value)
        with self.assertRaisesRegex(ValueError, 'recipe identity'): self.store.acquire(self.key, '0' * 64)

    def test_sdk_bytes_bound_to_identity(self):
        bad = copy.deepcopy(self.config); bad['sdk_sha256'] = '0' * 64
        with self.assertRaisesRegex(ValueError, 'SDK source changed'): Store(self.cache, bad)
        bad = copy.deepcopy(self.config); bad['support']['libnucleon.so'] = '0' * 64
        with self.assertRaisesRegex(ValueError, 'artifact digest'): Store(self.cache, bad)

    def test_oversized_original_uses_ordinary_path(self):
        key = 'c' * 64; self.add_key(key, b'')
        with (self.cache / key / 'program.test').open('wb') as f: f.truncate((16 << 20) + 1)
        self.assertIsNone(self.store.acquire(key, self.store.record(key)))

    def test_transport_from_long_contained_temporary_path(self):
        nested = self.work / ('long-' * 30)
        nested.mkdir()
        client = '''import array, fcntl, hashlib, json, os, socket, sys
s=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET)
s.connect(os.environ['PIPELANG_NATIVE_SOCKET'])
s.send(json.dumps(dict(cache=sys.argv[1],key=sys.argv[2],sha256=sys.argv[3])).encode())
data,control,flags,_=s.recvmsg(4096,socket.CMSG_SPACE(4))
assert data==b'ok' and not flags
assert len(control)==1 and control[0][:2]==(socket.SOL_SOCKET,socket.SCM_RIGHTS)
fds=array.array('i');fds.frombytes(control[0][2]);assert len(fds)==1
fd=fds[0]
assert fcntl.fcntl(fd,fcntl.F_GET_SEALS)&15==15
assert hashlib.sha256(os.pread(fd,1<<20,0)).hexdigest()==sys.argv[3]
os.close(fd);s.close()
'''
        receipt = self.work / 'transport.json'
        with patch('tempfile.tempdir', str(nested)):
            self.assertEqual(run(self.cache, self.config, receipt,
                                 [sys.executable, '-c', client, str(self.cache), self.key, self.expected]), 0)
        self.assertEqual(list(nested.iterdir()), [])
        self.assertTrue(any(not e['prepared'] for e in json.loads(receipt.read_text())))

if __name__ == '__main__': unittest.main()
