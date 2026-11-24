"""Screen selection contracts without starting Xvfb, an application or a guest."""
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

specification = importlib.util.spec_from_file_location('generic_owned_desktop',
    Path(__file__).resolve().parents[1] / 'platforms/linux/desktop.py')
desktop = importlib.util.module_from_spec(specification)
specification.loader.exec_module(desktop)


class DesktopGeometryTests(unittest.TestCase):
    def test_default_optin_and_UHD_fit_the_bounded_fixed_depth_contract(self):
        self.assertEqual(desktop.screen_geometry({'width': 1600, 'height': 1000}), '1600x1000x24')
        self.assertEqual(desktop.screen_geometry({'width': 1600, 'height': 1200}), '1600x1200x24')
        self.assertEqual(desktop.screen_geometry({'width': 3840, 'height': 2160}), '3840x2160x24')

    def test_invalid_geometry_refuses_before_any_effect(self):
        invalid = [None, [], '1600x1200x24', {'width': True, 'height': 1200}, {'width': 1600, 'height': False},
                   {'width': 0, 'height': 1000}, {'width': 4097, 'height': 1000},
                   {'width': 4096, 'height': 4096}, {'width': 1600, 'height': 1200, 'depth': 32}]
        for value in invalid:
            with self.subTest(value=value), tempfile.TemporaryDirectory() as temporary:
                output = Path(temporary) / 'not-created'
                prepare = unittest.mock.Mock(side_effect=AssertionError('must not prepare'))
                owned = desktop.OwnedDesktop(output, {'screen_geometry': value, 'prepare_environment': prepare})
                with patch.object(desktop.subprocess, 'Popen', side_effect=AssertionError('must not spawn')):
                    with self.assertRaises(ValueError):
                        owned.start()
                self.assertFalse(output.exists())
                prepare.assert_not_called()

    def test_actual_first_spawn_argv_uses_default_or_selected_geometry(self):
        for selected, expected in ((None, '1600x1000x24'), ({'width': 1600, 'height': 1200}, '1600x1200x24')):
            with self.subTest(selected=selected), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                bus = root / 'private-bus'
                bus.mkdir(mode=0o700)
                selections = {'prepare_environment': lambda environment: None, 'xvfb': '/pinned/Xvfb'}
                if selected is not None:
                    selections['screen_geometry'] = selected
                owned = desktop.OwnedDesktop(root / 'output', selections)
                with patch.object(desktop.tempfile, 'mkdtemp', return_value=str(bus)), \
                        patch.object(desktop.subprocess, 'run'), \
                        patch.object(desktop.subprocess, 'Popen', side_effect=RuntimeError('stop before any real spawn')) as spawn:
                    with self.assertRaisesRegex(RuntimeError, 'stop before'):
                        owned.start()
                arguments = spawn.call_args.args[0]
                self.assertEqual(arguments[0], '/pinned/Xvfb')
                self.assertEqual(arguments[arguments.index('-screen'):arguments.index('-screen') + 3], ['-screen', '0', expected])
                self.assertIn('-auth', arguments)
                self.assertIn('-nolisten', arguments)


if __name__ == '__main__':
    unittest.main()
