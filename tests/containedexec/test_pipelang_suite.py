"""Inventory coverage regression for the retained-executable suite planner."""
import sys
import json
import unittest
import tempfile
from pathlib import Path
sys.dont_write_bytecode = True
from pipelang_suite import plan, cleanup_native_build_cache, artifact_inventory


class PlanTests(unittest.TestCase):
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
