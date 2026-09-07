"""Inventory coverage regression for the retained-executable suite planner."""
import sys
import unittest
sys.dont_write_bytecode = True
from pipelang_suite import plan


class PlanTests(unittest.TestCase):
    def test_grouping_preserves_every_case_once(self):
        tests = ['TestFirst', 'TestShapes', 'TestCompilerMemoryLocalSequences', 'TestTailMemory', 'FuzzSeed', 'Example']
        split = {'TestShapes': 53}
        separate = plan(tests, split, 1)
        grouped = plan(tests, split, 25)
        flatten = lambda jobs: [name for names, _ in jobs for name in names]
        self.assertEqual(flatten(separate), flatten(grouped))
        self.assertEqual(len(set(flatten(grouped))), len(flatten(grouped)))
        resource = [names for names, _ in grouped if 'Memory' in names[0]]
        self.assertEqual(len(resource), 16)
        self.assertTrue(all(len(names) == 1 for names in resource))
        shapes = [(names, pattern) for names, pattern in grouped if names[0].startswith('TestShapes/')]
        self.assertEqual([len(names) for names, _ in shapes], [25, 25, 3])
        self.assertEqual(shapes[-1][1], '^TestShapes$/^(50|51|52)$')


if __name__ == '__main__':
    unittest.main()
