import json
import os
from pathlib import Path
import tempfile
import threading
import time
import unittest
from unittest.mock import patch
from budget import DiskBudget
from estate import CampaignBudget, SharedBudget, accepted_storage, canonical_roots, campaign_scope, storage_limit


class EstateTests(unittest.TestCase):
    def test_scope_covers_all_stages_builds_external_stores_support_and_history(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            paths = {n: root / n for n in ('active', 'external-go', 'external-native', 'old', 'tools', 'research')}
            for p in paths.values(): p.mkdir()
            for n in ('suite', 'matrix', 'integration', 'controller', 'builds', 'scratch', 'old-campaign'):
                d = paths['active'] / n; d.mkdir(); (d / 'payload').write_bytes(b'x' * 19)
            for n, p in paths.items():
                if n != 'active': (p / 'payload').write_bytes(b'x' * 23)
            roots = campaign_scope(paths['active'], paths['external-go'], paths['active'] / 'builds',
                                   paths['external-native'], paths['old'], [paths['tools']], [paths['research']])
            self.assertEqual(set(roots), set(paths.values()))
            budget = DiskBudget(roots, root, 1 << 20, 0, lock_roots=[], create_roots=False, record_population=False)
            try:
                self.assertEqual(budget.check()['retained_bytes'], 7 * 19 + 5 * 23)
                (paths['active'] / 'integration' / 'oversize').write_bytes(b'x' * (1 << 20))
                with self.assertRaisesRegex(RuntimeError, 'budget'): budget.check()
                self.assertTrue((paths['old'] / 'payload').exists())
            finally: budget.close()

    def test_missing_roots_cap_raise_aliases_and_scope_drift_are_refused(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw); output = root / 'out'; output.mkdir()
            with self.assertRaisesRegex(ValueError, 'missing'): canonical_roots([root / 'missing'])
            link = root / 'alias'; link.symlink_to(output, target_is_directory=True)
            with self.assertRaises(ValueError): canonical_roots([link])
            for cap in (0, -1, 97, 104, 112):
                with self.assertRaises(ValueError): storage_limit(cap)
            self.assertEqual(storage_limit(96), 96 << 30)
            # Separate scope excludes the alias fixture.
            with CampaignBudget([output], output, 1 << 20, 0): pass
            with self.assertRaisesRegex(RuntimeError, 'scope or cap changed'):
                CampaignBudget([output], output, 2 << 20, 0)

    def test_background_monitor_stops_population_and_preserves_evidence(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw); stopped = threading.Event()
            owner = CampaignBudget([root], root, 128 << 10, 0, on_failure=stopped.set)
            payload = root / 'integration-output'; payload.write_bytes(b'x' * (129 << 10))
            self.assertTrue(stopped.wait(3))
            with self.assertRaises(RuntimeError): owner.check()
            owner.close()
            self.assertEqual(payload.stat().st_size, 129 << 10)
            self.assertEqual(json.loads(owner.path.read_text())['status'], 'failed')

    def test_nested_roots_hardlinks_sparse_files_moves_and_internal_links(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw); cache = root / 'cache'; cache.mkdir()
            data = cache / 'data'; data.write_bytes(b'x' * 4096)
            os.link(data, cache / 'hardlink')
            (cache / 'link').symlink_to(data)
            budget = DiskBudget([cache, root], root, 16 << 20, 0, lock_roots=[], allow_internal_links=True, record_population=False)
            try:
                state = budget.check()
                self.assertEqual(state['retained_bytes'], 8192 + (cache / 'link').lstat().st_size)
                self.assertEqual(state['allocated_file_bytes'], 4096)
                data.write_bytes(b'z' * 8193)
                changed = budget.check()
                self.assertEqual(changed['retained_bytes'], 2 * 8193 + (cache / 'link').lstat().st_size)
                self.assertEqual(changed['allocated_file_bytes'], data.stat().st_blocks * 512)
                temporary = root / 'scratch'; temporary.mkdir(); (temporary / 'unit').write_bytes(b'a' * 200)
                budget.check()
                (temporary / 'unit').unlink(); temporary.rmdir()
                budget.check()  # ordinary compiler cleanup is not a lost root
                with (cache / 'sparse').open('wb') as f: f.truncate(2 << 20)
                self.assertGreater(budget.check()['retained_bytes'], 2 << 20)
                (cache / 'outside').symlink_to('/etc/passwd')
                with self.assertRaisesRegex(RuntimeError, 'outside'): budget.check()
            finally: budget.close()

    def test_controller_installs_gate_before_discovery_and_catches_integration_growth(self):
        import sys
        from types import SimpleNamespace
        import verification_campaign as controller
        with tempfile.TemporaryDirectory(dir=Path(__file__).parent) as raw:
            root = Path(raw)
            active = root / 'active'; active.mkdir()
            baseline = root / 'baseline'; baseline.mkdir()
            (baseline / 'pinned').write_text('keep')
            go = root / 'toolchain/bin/go'; go.parent.mkdir(parents=True); go.write_text('fixture')
            node = root / 'node'; node.write_text('fixture')
            output = active / 'campaigns/probe'
            seen = []
            def guard(*args):
                self.assertTrue((output / 'storage-budget.json').exists())
                (output / 'dependency-inputs.json').write_text(json.dumps({'paths': [str(node)]}))
                return SimpleNamespace(check=lambda: 'fixed')
            def stage(command, **kwargs):
                name = Path(command[2]).stem
                seen.append(name)
                if name == 'pipelang_suite':
                    self.assertIn('--campaign-budget', command)
                    suite = output / 'suite'; suite.mkdir()
                    (suite / 'inventory.json').write_text('{"jobs": []}')
                elif name == 'integration':
                    d = output / 'integration'; d.mkdir()
                    (d / 'binary').write_bytes(b'x' * (256 << 10))
                return SimpleNamespace(returncode=0)
            def owner(*args, **kwargs):
                kwargs.update(reserve=0, on_failure=lambda: None)
                return CampaignBudget(*args, **kwargs)
            argv = ['verification_campaign.py', '--root', str(active), '--campaign', 'probe',
                    '--node', str(node), '--go', str(go), '--baseline', str(baseline)]
            with patch.object(sys, 'argv', argv), patch.object(controller, 'verify_job'), \
                 patch.object(controller, 'storage_limit', return_value=256 << 10), \
                 patch.object(controller, 'source_paths', return_value=[node]), \
                 patch.object(controller, 'dependency_guard', side_effect=guard), \
                 patch.object(controller, 'publish_profile'), patch.object(controller, 'MEMORY_FAMILIES', {}), \
                 patch.object(controller, 'MEMORY_COUNT', 0), \
                 patch.object(controller, 'CampaignBudget', side_effect=owner), \
                 patch.object(controller.subprocess, 'run', side_effect=stage):
                with self.assertRaisesRegex(RuntimeError, 'budget'):
                    controller.main()
            self.assertEqual(seen, ['pipelang_suite', 'matrix', 'integration'])
            self.assertEqual((baseline / 'pinned').read_text(), 'keep')
            self.assertFalse((output / 'stages-complete.json').exists())

    def test_dangling_link_is_counted_but_new_external_payload_is_refused(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw); source = root / 'source'; source.mkdir()
            target = root / 'external'
            link = source / 'link'; link.symlink_to(target)
            budget = DiskBudget([source], root, 1 << 20, 0, lock_roots=[], allow_internal_links=True, record_population=False)
            try:
                self.assertEqual(budget.check()['retained_bytes'], link.lstat().st_size)
                target.write_text('previously absent storage')
                with self.assertRaisesRegex(RuntimeError, 'outside'): budget.check()
            finally: budget.close()

    def test_failed_initialization_releases_population_lock(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw); (root / 'large').write_bytes(b'x' * 8192)
            with self.assertRaises(RuntimeError): DiskBudget([root], root, 1, 0)
            budget = DiskBudget([root], root, 1 << 20, 0)
            budget.close()

    def test_shared_owner_identity_freshness_scope_and_terminal_metadata_recheck(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            owner = CampaignBudget([root], root, 1 << 20, 0)
            try:
                with patch('estate.os.getppid', return_value=os.getpid()):
                    client = SharedBudget(owner.path, [root / 'suite'], 1 << 20)
                    self.assertEqual(client.record, owner.record)
                    with self.assertRaisesRegex(RuntimeError, 'outside'):
                        SharedBudget(owner.path, ['/var'], 1 << 20)
                with patch('estate.os.getppid', return_value=-1):
                    with self.assertRaisesRegex(RuntimeError, 'owner unavailable'): SharedBudget(owner.path, [root], 1 << 20)
                with patch('estate.time.monotonic_ns', return_value=time.monotonic_ns() + 10_000_000_000):
                    with self.assertRaisesRegex(RuntimeError, 'owner unavailable'): SharedBudget(owner.path, [root], 1 << 20)
            finally: owner.close()
            reservation = owner.record.read_bytes()
            self.assertEqual(accepted_storage(root)['status'], 'accepted')
            self.assertEqual(owner.record.read_bytes(), reservation)
            (root / 'late-output').write_bytes(b'x' * (2 << 20))
            with self.assertRaisesRegex(RuntimeError, 'budget'): accepted_storage(root)


if __name__ == '__main__': unittest.main()
