import json
import errno
import os
from pathlib import Path
import tempfile
import threading
import time
from types import SimpleNamespace
import unittest
from unittest.mock import patch
from budget import DiskBudget, _DirectoryEntries, within_roots
from estate import CampaignBudget, SharedBudget, accepted_storage, canonical_roots, campaign_scope, storage_limit, _CoordinatorReclaim


class EstateTests(unittest.TestCase):
    def test_compact_metadata_preserves_counter_boundaries_and_hardlinks(self):
        values = {0, 1, (1 << 64) - 1, 1 << 64, -1}
        for width in range(1, 9):
            values.update({(1 << (width * 8)) - 1, 1 << (width * 8)})
        entries = _DirectoryEntries()
        try:
            expected = {}
            for size in values:
                for blocks in values:
                    for key in (None, (7, 19)):
                        path = '/counters/' + str(len(expected))
                        expected[path] = (size, blocks, key)
                        entries[path] = expected[path]
            self.assertEqual(dict(entries.items()), expected)
            for path, value in expected.items():
                self.assertEqual(entries.pop(path), value)
            self.assertEqual(len(entries), 0)
        finally: entries.close()

    def test_packed_and_raw_filename_bytes_never_alias(self):
        names = [b'a' * 64, b'a' * 64 + b'-a', b'a' * 64 + b'-d',
                 b'A' * 64, b'a' * 63, b'a' * 65, b'a' * 64 + b'-x',
                 b'\x01' + b'\xaa' * 32, b'raw-\xff', 'caf\u00e9'.encode()]
        expected = {'/files/' + os.fsdecode(name): (i, i * 512, None)
                    for i, name in enumerate(names)}
        entries = _DirectoryEntries()
        try:
            for path, value in expected.items(): entries[path] = value
            self.assertEqual(dict(entries.items()), expected)
            self.assertEqual(set(entries), set(expected))
            self.assertEqual(set(entries.paths_under('/files')), set(expected))
            for path, value in expected.items(): self.assertEqual(entries[path], value)
            for path in expected: del entries[path]
            self.assertEqual(len(entries), 0)
        finally: entries.close()

    def test_queued_temporary_entry_after_parent_removal(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            budget = DiskBudget([root], root, 16 << 20, 0, lock_roots=[], record_population=False)
            try:
                parent = root / 'temporary'; parent.mkdir()
                child = parent / 'child'; child.mkdir()
                child.rmdir(); parent.rmdir()
                budget.scan(str(child))
                self.assertEqual(budget.check()['retained_bytes'], 0)
                parent.mkdir(); child.mkdir(); (child / 'live').write_bytes(b'fresh')
                self.assertEqual(budget.check()['retained_bytes'], len(b'fresh'))
                # Do not hide resource exhaustion, even for an absent entry.
                with patch.object(budget.libc, 'inotify_add_watch', return_value=-1), \
                     patch('budget.ctypes.get_errno', return_value=errno.ENOSPC):
                    with self.assertRaisesRegex(RuntimeError, 'watch unavailable'):
                        budget.scan(str(root / 'missing'))
            finally: budget.close()

    def test_removed_temporary_directory_during_scan_keeps_exact_accounting(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            budget = DiskBudget([root], root, 16 << 20, 0, lock_roots=[], record_population=False)
            try:
                temporary = root / 'compiler-temporary'; temporary.mkdir()
                real_scandir = os.scandir
                def vanish(path):
                    if path == str(temporary): temporary.rmdir()
                    return real_scandir(path)
                with patch('budget.os.scandir', side_effect=vanish): budget.scan(str(temporary))
                self.assertEqual(budget.check()['retained_bytes'], 0)
                temporary.mkdir(); (temporary / 'live').write_bytes(b'recreated')
                self.assertEqual(budget.check()['retained_bytes'], len(b'recreated'))
                with patch('budget.os.scandir', side_effect=PermissionError('unreadable')):
                    with self.assertRaises(PermissionError): budget.scan(str(temporary))
                with patch('budget.os.scandir', side_effect=FileNotFoundError('declared root')):
                    with self.assertRaises(FileNotFoundError): budget.scan(str(root))
            finally: budget.close()

    def test_subtree_index_matches_independent_paths_through_directory_reuse(self):
        entries = _DirectoryEntries()
        paths = ['/a/file', '/a/nested/file', '/ab/file', '/a0/file',
                 '/root-file', '/a/' + os.fsdecode(b'raw-\xff') + '/file']
        expected = {path: (i + 1, (i + 1) * 512, None) for i, path in enumerate(paths)}
        def compare():
            for root in ['/a', '/ab', '/a/nested', '/missing', '/root-file']:
                wanted = {p:v for p,v in expected.items()
                          if p == root or Path(root) in Path(p).parents}
                descendants = {p for p in wanted if p != root}
                self.assertEqual(set(entries.paths_under(root)), descendants)
                self.assertCountEqual(entries.values_under(root), wanted.values())
            # Retain the existing helper's root-separator boundary behavior.
            self.assertEqual(list(entries.paths_under('/')), [])
            self.assertEqual(list(entries.values_under('/')), [])
        try:
            for path, value in expected.items(): entries[path] = value
            compare()
            for path in list(expected):
                if Path('/a') in Path(path).parents:
                    del entries[path]; del expected[path]
            compare()
            self.assertEqual(list(entries.directories_under('/a')), [])
            expected['/a/new/file'] = (19, 1024, None)
            entries['/a/new/file'] = expected['/a/new/file']
            compare()
        finally: entries.close()

    def test_shared_ancestry_is_fresh_and_rejects_root_or_parent_substitution(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            parent = root / 'parent'; parent.mkdir()
            files = [parent / str(i) for i in range(20)]
            for path in files: path.write_bytes(b'x')
            budget = DiskBudget(files, root, 16 << 20, 0, lock_roots=[],
                                create_roots=False, record_population=False)
            try:
                real_lstat = os.lstat
                with patch('budget.os.lstat', wraps=real_lstat) as observed:
                    modes = budget.check_roots()
                    self.assertEqual(set(modes), set(files))
                    self.assertEqual(observed.call_count, len(budget.root_ancestry))
                    self.assertEqual(sum(call.args[0] == str(parent) for call in observed.call_args_list), 1)
                saved = root / 'saved'
                parent.rename(saved); parent.symlink_to(saved, target_is_directory=True)
                with self.assertRaisesRegex(RuntimeError, 'root changed'): budget.check_roots()
                parent.unlink(); saved.rename(parent)
                budget.check_roots()
                files[0].unlink(); files[0].symlink_to(files[1])
                with self.assertRaisesRegex(RuntimeError, 'root changed'): budget.check_roots()
                files[0].unlink()
                with self.assertRaisesRegex(RuntimeError, 'disappeared'): budget.check_roots()
            finally: budget.close()

    def test_free_space_samples_distinct_paths_and_refreshes_every_check(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            left, right = root / 'left', root / 'right'
            left.mkdir(); right.mkdir()
            files = [left / 'a', left / 'b', right / 'c']
            for path in files: path.write_bytes(b'x')
            budget = DiskBudget(files, root, 16 << 20, 0, lock_roots=[],
                                create_roots=False, record_population=False)
            try:
                free = {left: 100, right: 80}
                with patch('budget.shutil.disk_usage', side_effect=lambda p: SimpleNamespace(free=free[p])) as usage:
                    self.assertEqual(budget.check()['available_bytes'], 80)
                    self.assertCountEqual([call.args[0] for call in usage.call_args_list], [left, right])
                    usage.reset_mock()
                    free[left] = 60
                    self.assertEqual(budget.check()['available_bytes'], 60)
                    self.assertEqual(usage.call_count, 2)
                    budget.reserve = 70
                    with self.assertRaisesRegex(RuntimeError, 'headroom'): budget.check()
                files[0].unlink()
                with self.assertRaisesRegex(RuntimeError, 'root.*(changed|removed)'): budget.check()
            finally: budget.close()

    def test_many_exact_roots_match_independent_union_and_preserve_refusals(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            parent = root / 'covered'; parent.mkdir()
            child = parent / 'child'; child.write_bytes(b'covered')
            siblings = []
            for i in range(1000):
                path = root / str(i); path.write_bytes(b'x'); siblings.append(path)
            supplied = [child, *reversed(siblings), parent, child]
            expected = sorted([parent, *siblings])
            self.assertEqual(canonical_roots(supplied), expected)
            budget = DiskBudget(supplied, root, 16 << 20, 0, lock_roots=[],
                                create_roots=False, record_population=False)
            try:
                self.assertEqual(budget.roots, expected)
                self.assertEqual(budget.used, len(siblings) + len(b'covered'))
                for path in [child, parent, root, root / 'absent', *siblings[:4]]:
                    self.assertEqual(within_roots(path, set(expected)),
                                     any(path == p or p in path.parents for p in expected))
            finally: budget.close()
            link = parent / 'link'; link.symlink_to(child)
            with self.assertRaises(ValueError): canonical_roots([parent, link])
            with self.assertRaises(ValueError): canonical_roots([parent, parent / 'absent'])

    def test_coordinator_reclaim_is_scoped_and_keeps_existing_limits(self):
        with tempfile.TemporaryDirectory() as raw:
            group = Path(raw) / 'coordinator.service'; group.mkdir()
            limits = {'memory.max': 512 << 20, 'memory.swap.max': 0, 'pids.max': 64}
            for name, value in limits.items(): (group / name).write_text(str(value))
            (group / 'memory.current').write_text(str(450 << 20))
            write = Path.write_text
            def reclaim(path, value, *args, **kwargs):
                self.assertEqual(path, group / 'memory.reclaim')
                write(group / 'memory.current', str(250 << 20))
                return write(path, value, *args, **kwargs)
            with patch.dict(os.environ, {'CONTAINED_JOB_UNIT': group.name}), \
                 patch('estate.current_group', return_value=group), patch('estate.verify_job'):
                owner = _CoordinatorReclaim()
                with patch.object(Path, 'write_text', reclaim): owner.check()
                self.assertEqual(owner.requests, 1)
                self.assertEqual(int((group / 'memory.reclaim').read_text()), 194 << 20)
                owner.check()
                self.assertEqual(owner.requests, 1)
                self.assertEqual({k:int((group/k).read_text()) for k in limits}, limits)
                (group / 'memory.max').write_text(str(1 << 30))
                with self.assertRaisesRegex(RuntimeError, 'containment'): _CoordinatorReclaim()
            with patch.dict(os.environ, {'CONTAINED_JOB_UNIT': 'different.service'}), \
                 patch('estate.current_group', return_value=group), \
                 patch('estate.verify_job', side_effect=AssertionError('wrong group')):
                owner = _CoordinatorReclaim(); owner.check()
                self.assertEqual(owner.requests, 0)

    def test_partial_reclaim_requires_read_back_and_other_failures_propagate(self):
        with tempfile.TemporaryDirectory() as raw:
            group = Path(raw) / 'coordinator.service'; group.mkdir()
            for name, value in {'memory.max':512<<20,'memory.swap.max':0,'pids.max':64}.items():
                (group/name).write_text(str(value))
            (group/'memory.current').write_text(str(450<<20))
            with patch.dict(os.environ, {'CONTAINED_JOB_UNIT':group.name}), \
                 patch('estate.current_group', return_value=group), patch('estate.verify_job'):
                owner = _CoordinatorReclaim()
            with patch.object(Path, 'write_text', side_effect=BlockingIOError(errno.EAGAIN, 'partial')):
                with self.assertRaisesRegex(RuntimeError, 'headroom'): owner.check()
            with patch.object(Path, 'write_text', side_effect=PermissionError(errno.EACCES, 'denied')):
                with self.assertRaises(PermissionError): owner.check()
            write = Path.write_text
            def partial(path, value):
                write(group/'memory.current', str(300<<20))
                raise BlockingIOError(errno.EAGAIN, 'partial')
            with patch.object(Path, 'write_text', partial): owner.check()
            self.assertEqual(int((group/'memory.max').read_text()), 512<<20)

    def test_inventory_metadata_has_no_integer_or_identity_range_loss(self):
        entries = _DirectoryEntries()
        expected = {
            '/one/file': (0, 0, None),
            '/two/file': ((1 << 64) - 1, (1 << 64) - 1, None),
            '/three/file': (1 << 64, 1 << 65, None),
            '/four/file': (-1, -2, None),
            '/five/file': (123, 4096, (1 << 64, 1 << 65)),
            '/root-file': (42, 4096, None),
            '/odd/\udcff': (8, 4096, None),
            '/odd/quote\'";select': (9, 4096, None),
        }
        entries.update(expected)
        self.assertEqual(dict(entries.items()), expected)
        self.assertEqual(len(entries), len(expected))
        for root in ('/', '/one', '/one/file', '/on', '/missing'):
            self.assertEqual(sorted(entries.paths_under(root)), sorted(
                p for p in expected if p.startswith(root + '/')))
            self.assertCountEqual(entries.values_under(root), [
                value for p, value in expected.items() if p.startswith(root + '/') or p == root])
        for path, value in expected.items():
            self.assertEqual(entries.pop(path), value)
        self.assertEqual(len(entries), 0)
        self.assertEqual(list(entries), [])

    def test_incremental_inventory_matches_independent_walk_after_mutations(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            # Identical basenames in distinct directories must remain separate.
            for directory in ('left', 'right', 'left/nested'):
                parent = root / directory; parent.mkdir(parents=True, exist_ok=True)
                for i in range(40):
                    (parent / str(i)).write_bytes(bytes([i]) * (i + 1))
            os.link(root / 'left/0', root / 'right/shared')
            (root / 'right/link').symlink_to(root / 'left/1')
            with (root / 'right/sparse').open('wb') as stream:
                stream.truncate(2 << 20)
            budget = DiskBudget([root], root, 16 << 20, 0, lock_roots=[],
                                allow_internal_links=True, record_population=False)
            def compare():
                logical, allocated, inodes = 0, 0, set()
                for directory, _, files in os.walk(root):
                    for name in files:
                        info = (Path(directory) / name).lstat()
                        logical += info.st_size
                        identity = (info.st_dev, info.st_ino)
                        if identity not in inodes:
                            allocated += info.st_blocks * 512
                            inodes.add(identity)
                state = budget.check()
                self.assertEqual(state['retained_bytes'], logical)
                self.assertEqual(state['allocated_file_bytes'], allocated)
                self.assertEqual(budget.root_bytes(root), logical)
                self.assertEqual(budget.root_bytes(root / 'left'), sum(
                    p.lstat().st_size for p in (root / 'left').rglob('*') if p.is_file()))
            try:
                compare()
                (root / 'right/shared').write_bytes(b'x' * 8193)
                compare()  # Updating either hardlink updates both logical sizes.
                (root / 'left/nested').rename(root / 'right/moved')
                compare()
                (root / 'left/0').unlink()
                (root / 'left/0').write_bytes(b'replacement inode')
                compare()
                for p in (root / 'right/moved').iterdir(): p.unlink()
                (root / 'right/moved').rmdir()
                # Localized directory cleanup must not rebuild every unrelated path.
                with patch.object(type(budget.sizes), '__iter__', side_effect=AssertionError('global path scan')):
                    compare()
                (root / 'left/1').write_bytes(b'changed linked target')
                (root / 'right/shared').unlink()
                compare()
            finally: budget.close()

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
                # Long durable TMPDIR paths can make the symlink allocate a
                # block rather than fitting in the inode. Charge it separately.
                link_blocks = (cache / 'link').lstat().st_blocks * 512
                self.assertEqual(state['allocated_file_bytes'], data.stat().st_blocks * 512 + link_blocks)
                data.write_bytes(b'z' * 8193)
                changed = budget.check()
                self.assertEqual(changed['retained_bytes'], 2 * 8193 + (cache / 'link').lstat().st_size)
                self.assertEqual(changed['allocated_file_bytes'], data.stat().st_blocks * 512 + link_blocks)
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
