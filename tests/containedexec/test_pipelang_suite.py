"""Inventory coverage regression for the retained-executable suite planner."""
import sys
import json
import unittest
import tempfile
from pathlib import Path
sys.dont_write_bytecode = True
from pipelang_suite import plan, cleanup_native_build_cache, artifact_inventory
from reporting import CompactRows
from campaign import atomic_json, atomic_json_array, atomic_json_stream, canonical


class PlanTests(unittest.TestCase):
    def test_streamed_report_matches_canonical_json_and_rejects_nonfinite_values(self):
        value = {'z': [None, True, False, -0.0, 5e-324, 1.7976931348623157e308],
                 'a': [{'text': 'nul\0 \u00ff \ud800', 'integer': 1 << 130}] * 5000}
        with tempfile.TemporaryDirectory() as raw:
            target = Path(raw) / 'report.json'
            expected = canonical(value) + b'\n'
            self.assertEqual(atomic_json_stream(target, value), len(expected))
            self.assertEqual(target.read_bytes(), expected)
            for invalid in (float('nan'), float('inf'), -float('inf')):
                with self.assertRaises(ValueError):
                    atomic_json_stream(target, {'a': value, 'z': invalid})
                self.assertEqual(target.read_bytes(), expected)

    def test_streamed_report_preserves_publication_boundaries(self):
        for phase in ('written', 'synced', 'renamed', 'committed'):
            with self.subTest(phase=phase), tempfile.TemporaryDirectory() as raw:
                target = Path(raw) / 'report.json'
                atomic_json(target, {'old': True})
                def interrupt(current):
                    if current == phase:
                        raise RuntimeError('interrupted')
                with self.assertRaisesRegex(RuntimeError, 'interrupted'):
                    atomic_json_stream(target, {'new': True}, interrupt)
                expected = {'new': True} if phase in ('renamed', 'committed') else {'old': True}
                self.assertEqual(json.loads(target.read_bytes()), expected)

    def test_compact_rows_preserve_canonical_bytes_and_stable_order(self):
        original = [dict(index=2, tests=['late'], report={'elapsed_s': .25}),
                    dict(index=1, tests=['first'], report={'text': 'nul\0 and \u00ff'}, resumed=True),
                    dict(index=1, tests=['second'], report={'tree_removed': False}, exit=1)]
        rows = CompactRows(); rows.extend(original); rows.sort()
        expected = sorted(original, key=lambda row: row['index'])
        self.assertEqual(len(rows), len(expected))
        self.assertEqual(list(rows), expected)
        # Consumers cannot mutate the retained snapshot between reporting passes.
        next(iter(rows))['report'].clear()
        self.assertEqual(list(rows), expected)
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            old_size = atomic_json(root / 'old.json', expected)
            self.assertEqual(rows.write(root / 'new.json'), old_size)
            self.assertEqual((root / 'new.json').read_bytes(), (root / 'old.json').read_bytes())
            self.assertEqual(CompactRows().write(root / 'empty.json'), 3)
            self.assertEqual((root / 'empty.json').read_bytes(), b'[]\n')

    def test_streamed_array_retains_atomic_publication_boundaries(self):
        for phase in ('written', 'synced', 'renamed', 'committed'):
            with self.subTest(phase=phase), tempfile.TemporaryDirectory() as raw:
                target = Path(raw) / 'rows.json'
                atomic_json(target, [{'old': True}])
                def interrupt(current):
                    if current == phase:
                        raise RuntimeError('interrupted')
                with self.assertRaisesRegex(RuntimeError, 'interrupted'):
                    atomic_json_array(target, iter([{'new': True}]), interrupt)
                expected = [{'new': True}] if phase in ('renamed', 'committed') else [{'old': True}]
                self.assertEqual(json.loads(target.read_bytes()), expected)

    def test_compact_matrix_rows_keep_schema_order_and_summary(self):
        original = [dict(accepted=True, isolated={'child_maxrss_kib': 1024, 'elapsed_s': .5}),
                    dict(accepted=False, isolated={'child_maxrss_kib': 2048, 'elapsed_s': 1.25})]
        rows = CompactRows()
        for index, row in enumerate(original):
            rows.append(row, index=index)
        self.assertEqual(list(rows), original)
        self.assertEqual(max(r['isolated']['child_maxrss_kib'] for r in rows), 2048)
        self.assertEqual(max(r['isolated']['elapsed_s'] for r in rows), 1.25)
        with tempfile.TemporaryDirectory() as raw:
            target = Path(raw) / 'matrix.json'
            rows.write(target)
            self.assertEqual(target.read_bytes(), canonical(original) + b'\n')

    def test_artifact_inventory_includes_only_executed_keys(self):
        with tempfile.TemporaryDirectory() as temporary:
            cache = Path(temporary)
            key, unused = 'a' * 64, 'b' * 64
            for name in [key, unused]:
                entry = cache / name
                entry.mkdir()
                (entry / 'program.test').write_bytes(b'binary')
                (entry / 'record.json').write_text(json.dumps(dict(Version='pipelang-native-validation-v2', Key=name, BinarySHA256='c' * 64)))
            logs = f'generated_compiled_artifact packages=32 cache_hit=true key={key}\n' * 2
            inventory = artifact_inventory(cache, logs)
            self.assertEqual(artifact_inventory(cache, iter(logs.splitlines())), inventory)
            self.assertEqual(set(inventory['entries']), {key})
            self.assertEqual((inventory['hits'], inventory['misses']), (2, 0))
            self.assertEqual(inventory['retained_bytes'], sum(p.stat().st_size for p in (cache / key).iterdir()))
            with self.assertRaises(RuntimeError):
                artifact_inventory(cache, '')
            (cache / key / 'record.json').write_text('{}')
            with self.assertRaises(RuntimeError):
                artifact_inventory(cache, logs)

    def test_packed_only_inventory_counts_originals_as_zero(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            key = 'a' * 64
            cache, representation = root / 'cache', root / 'packed'
            (cache / key).mkdir(parents=True)
            (representation / key).mkdir(parents=True)
            record = cache / key / 'record.json'
            record.write_text(json.dumps(dict(Version='pipelang-native-validation-v2', Key=key, BinarySHA256='b' * 64)))
            recipe = representation / key / 'recipe.json'
            recipe.write_text(json.dumps(dict(key=key, sha256='b' * 64, size=1234)))
            logs = f'generated_compiled_artifact packages=4 cache_hit=true key={key}'
            inventory = artifact_inventory(cache, logs, representation)
            self.assertEqual(inventory['retained_bytes'], record.stat().st_size)
            self.assertEqual(inventory['logical_binary_bytes'], 1234)
            self.assertEqual(inventory['entries'][key]['original_bytes'], 0)
            recipe.write_text(json.dumps(dict(key=key, sha256='c' * 64, size=1234)))
            with self.assertRaises(RuntimeError):
                artifact_inventory(cache, logs, representation)

    def test_grouping_preserves_every_case_once(self):
        tests = ['TestFirst', 'TestShapes', 'TestCompilerMemoryLocalSequences', 'TestTailMemory', 'FuzzSeed', 'Example']
        split = {'TestShapes': 53}
        separate = plan(tests, split, 1)
        grouped = plan(tests, split, 25)
        flatten = lambda jobs: [name for names, _ in jobs for name in names]
        self.assertEqual(flatten(separate), flatten(grouped))
        self.assertEqual(len(set(flatten(grouped))), len(flatten(grouped)))
        resource = [names for names, _ in grouped if 'Memory' in names[0]]
        self.assertEqual(len(resource), 37)
        self.assertTrue(all(len(names) == 1 for names in resource))
        shapes = [(names, pattern) for names, pattern in grouped if names[0].startswith('TestShapes/')]
        self.assertEqual([len(names) for names, _ in shapes], [25, 25, 3])
        self.assertEqual(shapes[-1][1], '^TestShapes$/^(50|51|52)$')

    def test_v111_scaling_partitions_cover_each_family_once(self):
        import re
        name='TestV1110TerminalLeafSelectorValueArmsMemory'
        jobs=plan([name], {}, 1)
        self.assertEqual(len(jobs), 36)
        for shape in range(36):
            components=[name,'v0.111.0','choices'+str(shape),'branchtrue','straightfalse','unusedfalse','256']
            matches=[names for names,pattern in jobs if all(re.search(p,c) for p,c in zip(pattern.split('/'),components))]
            self.assertEqual(matches, [[name+'/choices'+str(shape)]])

    def test_v113_scaling_partitions_cover_each_family_once(self):
        import re
        name='TestV1130EnumsMemory'
        jobs=plan([name], {}, 1)
        self.assertEqual(len(jobs), 3)
        for shape in range(3):
            components=[name,'v0.113.0','choices'+str(shape),'256']
            matches=[names for names,pattern in jobs if all(re.search(p,c) for p,c in zip(pattern.split('/'),components))]
            self.assertEqual(matches, [[name+'/choices'+str(shape)]])

    def test_v112_scaling_partitions_cover_each_family_once(self):
        import re
        name='TestV1120ArrowSelectorValueArmsMemory'
        jobs=plan([name], {}, 1)
        self.assertEqual(len(jobs), 12)
        for shape in range(12):
            components=[name,'v0.112.0','choices'+str(shape),'branchfalse','straighttrue','unusedfalse','256']
            matches=[names for names,pattern in jobs if all(re.search(p,c) for p,c in zip(pattern.split('/'),components))]
            self.assertEqual(matches, [[name+'/choices'+str(shape)]])

    def test_v110_scaling_partitions_cover_each_family_once(self):
        import re
        name='TestV1100StraightLineSelectorValueArmsMemory'
        jobs=plan([name], {}, 1)
        self.assertEqual(len(jobs), 12)
        for shape in range(12):
            components=[name,'v0.110.0','choices'+str(shape),'branchfalse','straighttrue','unusedfalse','256']
            matches=[names for names,pattern in jobs if all(re.search(p,c) for p,c in zip(pattern.split('/'),components))]
            self.assertEqual(matches, [[name+'/choices'+str(shape)]])

    def test_disposable_cache_waits_for_unit_removal(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            cache = root / 'owned-cache'
            cache.mkdir()
            (cache / 'object').write_bytes(b'intermediate')
            group = root / 'active-group'
            group.mkdir()
            rows = [{'report': {'tree_removed': True, 'cgroup': str(group)}}]
            self.assertEqual(cleanup_native_build_cache(cache, rows), (12, False))
            group.rmdir()
            rows[0]['report']['tree_removed'] = False
            self.assertEqual(cleanup_native_build_cache(cache, rows), (12, False))
            self.assertEqual(cleanup_native_build_cache(cache, []), (12, False))
            rows[0]['report']['tree_removed'] = True
            self.assertEqual(cleanup_native_build_cache(cache, rows), (12, True))
            self.assertFalse(cache.exists())


if __name__ == '__main__':
    unittest.main()
