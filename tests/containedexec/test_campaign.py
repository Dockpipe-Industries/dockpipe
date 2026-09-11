"""Synthetic recovery and admission proof; no compiler or host mutations."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch
from campaign import Campaign, BuildStore, InputGuard, atomic_json, digest, fingerprint, provenance, read_sealed, sealed


def report(root):
    events = 'low 0\nhigh 0\nmax 0\noom 0\noom_kill 0\noom_group_kill 0\n'
    return dict(exit=0, unit_exit=0, outcome='completed', tree_removed=True,
                cgroup=str(root / 'removed-cgroup'), swap_current=0,
                limits={'memory.max': 1 << 30, 'memory.swap.max': 0, 'pids.max': 128},
                memory_events_before=events, memory_events_after=events,
                swap_events_before='high 0\nmax 0\nfail 0\n', swap_events_after='high 0\nmax 0\nfail 0\n')


class CampaignTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.artifact = self.root / 'artifact'
        self.artifact.write_bytes(b'proof')
        self.path = self.root / 'campaign'

    def tearDown(self):
        self.temporary.cleanup()

    def register(self, campaign):
        campaign.register('suite', 'source-v1', ['case/a', 'case/b'])

    def pass_case(self, campaign, case='case/a', **overrides):
        attempt = campaign.begin('suite', [case], 'inputs-v1')
        args = dict(report=report(self.root), artifacts=[self.artifact], inputs_after='inputs-v1',
                    complete_cases=[case], result={'exit': 0})
        args.update(overrides)
        return campaign.finish(attempt, **args)

    def test_compact_reconciliation_preserves_admission_and_drops_payloads(self):
        with Campaign(self.path) as c:
            self.register(c)
            self.pass_case(c, result={'exit': 0, 'large': 'x' * 100000})
            self.pass_case(c, 'case/b')
            full = c.accepted('suite', 'inputs-v1')
            compact = c.accepted('suite', 'inputs-v1', summary_only=True)
            self.assertEqual({k: v['id'] for k, v in full.items()},
                             {k: v['id'] for k, v in compact.items()})
            self.assertTrue(all(set(v) == {'id', 'supersedes'} for v in compact.values()))
            self.assertEqual(set(c.accepted('suite', 'inputs-v1', ['case/b'], summary_only=True)), {'case/b'})
            self.assertEqual(c.reconcile('suite', 'inputs-v1')['accepted'], 2)
            self.artifact.write_bytes(b'corrupted')
            self.assertFalse(c.accepted('suite', 'inputs-v1'))
            self.assertFalse(c.accepted('suite', 'inputs-v1', summary_only=True))

    def test_compact_reconciliation_rejects_ambiguous_success(self):
        with Campaign(self.path) as c:
            self.register(c)
            first = self.pass_case(c)
            duplicate = dict(first, id='independent-proof')
            atomic_json(self.path / 'receipts/independent-proof.json', sealed(duplicate))
            for compact in (False, True):
                with self.assertRaisesRegex(RuntimeError, 'overlapping successful proofs'):
                    c.accepted('suite', 'inputs-v1', summary_only=compact)
            duplicate['supersedes'] = [first['id']]
            atomic_json(self.path / 'receipts/independent-proof.json', sealed(duplicate))
            for compact in (False, True):
                self.assertEqual(c.accepted('suite', 'inputs-v1', summary_only=compact)['case/a']['id'], 'independent-proof')

    def test_crash_publication_windows(self):
        for phase in ('written', 'synced', 'renamed', 'committed'):
            with self.subTest(phase=phase):
                target = self.root / (phase + '.json')
                atomic_json(target, sealed({'old': True}))
                code = 'from campaign import *; import os; atomic_json(Path(os.environ["TARGET"]), sealed({"new":True}), lambda p: os._exit(17) if p == os.environ["PHASE"] else None)'
                rc = subprocess.run([sys.executable, '-B', '-c', code], cwd=Path(__file__).parent,
                                    env=dict(os.environ, TARGET=str(target), PHASE=phase)).returncode
                self.assertEqual(rc, 17)
                self.assertEqual(read_sealed(target), {'new': True} if phase in ('renamed', 'committed') else {'old': True})

    def test_process_crash_releases_lock_and_preserves_completed_receipt(self):
        code = '''from campaign import *
import os
from test_campaign import report
p=Path(os.environ['ROOT']); c=Campaign(p/'campaign'); c.register('suite','source-v1',['case/a','case/b'])
a=c.begin('suite',['case/a'],'inputs-v1'); c.finish(a,report(p),[p/'artifact'],'inputs-v1',['case/a'],{'exit':0})
c.begin('suite',['case/b'],'inputs-v1'); os._exit(19)
'''
        self.assertEqual(subprocess.run([sys.executable, '-B', '-c', code], cwd=Path(__file__).parent,
                                        env=dict(os.environ, ROOT=str(self.root))).returncode, 19)
        with Campaign(self.path, 'resume') as c:
            self.register(c)
            self.assertEqual(set(c.accepted('suite', 'inputs-v1')), {'case/a'})
            states = [read_sealed(p)['state'] for p in (self.path / 'attempts').glob('*/state.json')]
            self.assertCountEqual(states, ['passed', 'interrupted'])
            self.assertEqual(c.reconcile('suite', 'inputs-v1')['missing'], ['case/b'])

    def test_writer_lock_not_pid_metadata(self):
        with Campaign(self.path) as c:
            with self.assertRaisesRegex(RuntimeError, 'live writer'):
                Campaign(self.path, 'resume')
        (self.path / 'writer.lock').write_text('{"pid":1,"boot_id":"old"}')
        with Campaign(self.path, 'resume'):
            pass

    def test_boot_provenance_does_not_invalidate_receipts(self):
        with Campaign(self.path) as c:
            self.register(c)
            self.pass_case(c)
        with patch('campaign.provenance', return_value=dict(utc='later', boot_id='new', monotonic_ns=1)):
            with Campaign(self.path, 'resume') as c:
                self.register(c)
                self.assertEqual(len(c.accepted('suite', 'inputs-v1')), 1)

    def test_corruption_and_missing_artifacts(self):
        with Campaign(self.path) as c:
            self.register(c)
            receipt = self.pass_case(c)
            path = self.path / 'receipts' / (receipt['id'] + '.json')
            original = path.read_bytes()
            for data in (b'{', json.dumps(sealed(dict(receipt, state='failed'))).encode(), original.replace(b'inputs-v1', b'inputs-v2')):
                path.write_bytes(data)
                self.assertFalse(c.accepted('suite', 'inputs-v1'))
            path.write_bytes(original)
            self.artifact.unlink()
            self.assertFalse(c.accepted('suite', 'inputs-v1'))
            self.artifact.write_bytes(b'wrong')
            self.assertFalse(c.accepted('suite', 'inputs-v1'))

    def test_cleanup_cannot_be_inferred_after_restart(self):
        with Campaign(self.path) as c:
            self.register(c)
            r = report(self.root)
            r['tree_removed'] = False
            self.assertEqual(self.pass_case(c, report=r)['state'], 'failed')
            self.assertFalse(c.accepted('suite', 'inputs-v1'))

    def test_resource_and_inventory_admission(self):
        with Campaign(self.path) as c:
            self.register(c)
            for key, value in [('swap_current', 1), ('exit', 1), ('unit_exit', 1), ('outcome', 'timeout')]:
                r = report(self.root); r[key] = value
                self.assertEqual(self.pass_case(c, report=r)['state'], 'failed')
            self.assertEqual(self.pass_case(c, complete_cases=[])['state'], 'failed')
            self.assertEqual(self.pass_case(c, inputs_after='drift')['state'], 'failed')
            self.assertEqual(self.pass_case(c, artifacts=[])['state'], 'failed')
            self.assertFalse(c.accepted('suite', 'inputs-v1'))

    def test_conservative_dependency_closure_and_independent_stages(self):
        with Campaign(self.path) as c:
            self.register(c); self.pass_case(c)
            c.register('editor', 'editor-v1', ['editor'])
            key = c.register('matrix', 'host-v1', ['matrix'], parents=['suite'])
            c.register('editor', 'editor-v2', ['editor'])
            self.assertEqual(len(c.accepted('suite', 'inputs-v1')), 1)
            c.register('suite', 'source-v2', ['case/a', 'case/b'])
            self.assertFalse(c.accepted('suite', 'inputs-v1'))
            self.assertNotEqual(c.register('matrix', 'host-v1', ['matrix'], parents=['suite']), key)

    def test_dependency_add_delete_modes_and_same_timestamp_content(self):
        source = self.root / 'source'; source.mkdir()
        path = source / 'input'; path.write_bytes(b'aaa')
        before = fingerprint([source], {'flags': 'normal'})
        stamp = path.stat()
        path.write_bytes(b'bbb'); os.utime(path, ns=(stamp.st_atime_ns, stamp.st_mtime_ns))
        self.assertNotEqual(before, fingerprint([source], {'flags': 'normal'}))
        path.write_bytes(b'aaa'); self.assertEqual(before, fingerprint([source], {'flags': 'normal'}))
        path.chmod(0o700); self.assertNotEqual(before, fingerprint([source], {'flags': 'normal'}))
        path.chmod(0o600); path.unlink(); self.assertNotEqual(before, fingerprint([source], {'flags': 'normal'}))
        path.symlink_to(self.artifact)
        with self.assertRaises(ValueError): fingerprint([source])

    def test_live_guard_detects_restored_bytes_and_new_files(self):
        source = self.root / 'source'; source.mkdir()
        path = source / 'file'; path.write_bytes(b'original')
        guard = InputGuard([source])
        initial = guard.check()
        self.assertEqual(initial, guard.check())
        path.write_bytes(b'changed'); path.write_bytes(b'original')
        with self.assertRaises(RuntimeError): guard.check()
        guard.close()
        guard = InputGuard([source])
        self.assertEqual(initial, guard.check())
        (source / 'new').write_bytes(b'new')
        with self.assertRaises(RuntimeError): guard.check()
        guard.close()

    def test_build_store_verifies_repairs_and_pins_budget(self):
        binary = self.root / 'program'
        binary.write_bytes(b'compiled'); binary.chmod(0o500)
        store = BuildStore(self.root / 'builds', 10000)
        key = digest({'sources': 'one', 'settings': 'normal'})
        path = store.publish(key, binary, 'contained-receipt')
        self.assertEqual(store.get(key), path)
        self.assertIsNone(store.get(digest({'sources': 'changed'})))
        path.chmod(0o700); path.write_bytes(b'corrupt'); path.chmod(0o500)
        self.assertIsNone(store.get(key))
        repaired = store.publish(key, binary, 'retry-receipt')
        self.assertNotEqual(path, repaired)
        self.assertEqual(path.read_bytes(), b'corrupt')
        store.budget = 1
        with self.assertRaisesRegex(RuntimeError, 'disk budget'):
            store.publish(digest('new key'), binary, 'receipt')
        self.assertEqual(store.get(key), repaired)

    def test_repaired_artifact_keeps_linked_retry_and_supersedes_old_success(self):
        with Campaign(self.path) as c:
            self.register(c)
            original = self.pass_case(c)
            self.artifact.write_bytes(b'changed')
            self.assertFalse(c.accepted('suite', 'inputs-v1'))
            self.artifact.write_bytes(b'proof')
            retry = self.pass_case(c)
            self.assertEqual(retry['retry_of'], original['id'])
            self.assertEqual(c.accepted('suite', 'inputs-v1')['case/a']['id'], retry['id'])
            self.assertTrue((self.path / 'receipts' / (original['id'] + '.json')).exists())

    def test_receipt_committed_before_state_is_recoverable(self):
        with Campaign(self.path) as c:
            self.register(c)
            receipt = self.pass_case(c)
            statepath = self.path / 'attempts' / receipt['id'] / 'state.json'
            c.write(statepath, dict(receipt, state='running'))
        with Campaign(self.path, 'resume') as c:
            self.register(c)
            self.assertEqual(read_sealed(statepath)['state'], 'passed')
            self.assertEqual(len(c.accepted('suite', 'inputs-v1')), 1)


if __name__ == '__main__':
    unittest.main()
