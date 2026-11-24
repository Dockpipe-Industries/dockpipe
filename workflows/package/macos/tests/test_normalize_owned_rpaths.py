import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / 'assets/scripts/normalize-owned-rpaths.py'
SPEC = importlib.util.spec_from_file_location('normalize_owned_rpaths', SCRIPT)
NORMALIZER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(NORMALIZER)


def otool(paths):
    lines = ['image:', 'Load command 0', '      cmd LC_SEGMENT_64', '  cmdsize 72', '  segname __TEXT']
    for index, path in enumerate(paths, start=1):
        size = (12 + len(path.encode('utf-8')) + 1 + 7) // 8 * 8
        lines.extend([f'Load command {index}', '          cmd LC_RPATH', f'      cmdsize {size}',
                      f'         path {path} (offset 12)'])
    return '\n'.join(lines) + '\n'


class RpathTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.app = self.root / 'DockPipe.app'
        self.executables = self.app / 'Contents/MacOS'
        self.frameworks = self.app / 'Contents/Frameworks'
        self.resources = self.app / 'Contents/Resources'
        for directory in [self.executables, self.frameworks, self.resources]:
            directory.mkdir(parents=True)
        self.images = [self.executables / 'DockPipe', self.frameworks / 'libdockpipe-launcher-ui.1.0.dylib',
                       self.executables / 'dockpipe-workspace-reader', self.executables / 'dockpipe-terminal-host']
        for image in self.images:
            image.write_bytes(b'pure fixture; never executed')
        (self.frameworks / 'libdockpipe-launcher-ui.1.dylib').symlink_to(self.images[1].name)
        self.receipt = self.resources / 'owned-rpaths.json'
        self.state = {
            self.images[0]: [NORMALIZER.EXECUTABLE_RPATH],
            self.images[1]: ['/private/build/Browser SDK/lib', '/private/build/Qt SDK/lib'],
            self.images[2]: ['/private/build/Qt SDK/lib'],
            self.images[3]: ['/private/build/Qt SDK/lib', '/private/build/Qt SDK/lib', NORMALIZER.EXECUTABLE_RPATH],
        }
        self.calls = []

    def run_tool(self, arguments, deadline):
        self.assertGreater(deadline, 0)
        self.calls.append(list(arguments))
        image = Path(arguments[-1])
        self.assertIn(image, self.images)
        if arguments[0] == '/usr/bin/otool':
            self.assertEqual(arguments[1], '-l')
            return otool(self.state[image])
        self.assertEqual(arguments[0], '/usr/bin/install_name_tool')
        operations = arguments[1:-1]
        self.assertEqual(len(operations) % 2, 0)
        for index in range(0, len(operations), 2):
            option, value = operations[index:index + 2]
            if option == '-delete_rpath':
                self.state[image].remove(value)  # Apple removes one matching instance.
            else:
                self.assertEqual(option, '-add_rpath')
                self.assertNotIn(value, self.state[image])
                self.state[image].append(value)
        return ''

    def test_real_load_command_shape_and_literal_argv(self):
        path = '/private/Qt SDK;$(not-a-shell-command)/lib'
        self.assertEqual(NORMALIZER.parse_rpaths(otool([path, path])), [path, path])
        self.assertEqual(NORMALIZER.deletion_arguments([path, path]), ['-delete_rpath', path])
        self.assertEqual(NORMALIZER.parse_rpaths(otool([])), [])

    def test_normalizes_only_four_owned_images_and_verifies_relative_receipt(self):
        foreign = self.frameworks / 'libForeignBrowser.dylib'
        foreign.write_bytes(b'foreign stays untouched')
        report = NORMALIZER.normalize(self.app, self.receipt, self.run_tool)
        self.assertEqual(len(report['owned_images']), 4)
        self.assertEqual(self.state[self.images[0]], [NORMALIZER.EXECUTABLE_RPATH])
        self.assertEqual(self.state[self.images[1]], [NORMALIZER.LIBRARY_RPATH])
        self.assertEqual(self.state[self.images[2]], [NORMALIZER.EXECUTABLE_RPATH])
        self.assertEqual(self.state[self.images[3]], [NORMALIZER.EXECUTABLE_RPATH])
        self.assertEqual(foreign.read_bytes(), b'foreign stays untouched')
        self.assertFalse(any(call[0].endswith('install_name_tool') and Path(call[-1]) == self.images[0]
                             for call in self.calls))
        self.assertNotIn('/private/build', self.receipt.read_text())
        self.assertNotIn(str(self.root), self.receipt.read_text())
        for record in report['owned_images']:
            self.assertFalse(Path(record['path']).is_absolute())
            self.assertTrue(all(path.startswith('@') for path in record['rpaths']))
        self.assertEqual(json.loads(self.receipt.read_text()), report)
        self.calls.clear()
        second = NORMALIZER.normalize(self.app, self.receipt, self.run_tool)
        self.assertTrue(all(record['mutation_calls'] == 0 for record in second['owned_images']))

    def test_malformed_rpath_records_and_command_sequences_are_rejected(self):
        valid = otool(['/private/build/Qt/lib'])
        invalid = [valid.replace('(offset 12)', '(offset 2)'), valid.replace('cmdsize 40', 'cmdsize 8'),
                   valid.replace('Load command 0', 'Load command 1'), valid + 'Load command 0\ncmd LC_RPATH\ncmdsize 16\n',
                   valid.replace('path /private/build/Qt/lib (offset 12)', 'path ?(bad offset)'),
                   valid.replace('cmd LC_RPATH', 'cmd LC_RPATH\ncmd LC_RPATH')]
        for text in invalid:
            with self.subTest(text=text), self.assertRaises(ValueError):
                NORMALIZER.parse_rpaths(text)

    def test_escaping_library_alias_and_foreign_in_bundle_alias_are_refused(self):
        alias = self.frameworks / 'libdockpipe-launcher-ui.dylib'
        outside = self.root / 'outside.dylib'
        outside.write_bytes(b'outside')
        alias.symlink_to(outside)
        with self.assertRaises(ValueError):
            NORMALIZER.owned_images(self.app)
        alias.unlink()
        foreign = self.frameworks / 'QtCore'
        foreign.write_bytes(b'foreign')
        alias.symlink_to(foreign)
        with self.assertRaises(ValueError):
            NORMALIZER.owned_images(self.app)
        self.assertEqual(self.calls, [])

    def test_terminal_host_and_external_hardlink_are_required_fail_closed(self):
        terminal = self.images[3]
        terminal.unlink()
        terminal.symlink_to(self.root / 'not-present')
        with self.assertRaises(ValueError):
            NORMALIZER.owned_images(self.app)
        terminal.unlink()
        terminal.write_bytes(b'terminal')
        os.link(self.images[0], self.root / 'external-hardlink')
        with self.assertRaises(ValueError):
            NORMALIZER.owned_images(self.app)

    def test_no_progress_or_wrong_final_path_cannot_publish_success(self):
        def no_progress(arguments, deadline):
            if arguments[0].endswith('install_name_tool'):
                return ''
            return self.run_tool(arguments, deadline)
        with self.assertRaises(ValueError):
            NORMALIZER.normalize(self.app, self.receipt, no_progress)
        self.assertFalse(self.receipt.exists())

        def wrong_final_path(arguments, deadline):
            result = self.run_tool(arguments, deadline)
            if '-add_rpath' in arguments:
                self.state[Path(arguments[-1])] = ['/private/unexpected/lib']
            return result

        with self.assertRaises(ValueError):
            NORMALIZER.normalize(self.app, self.receipt, wrong_final_path)
        self.assertFalse(self.receipt.exists())

    def test_all_images_preflight_before_first_mutation(self):
        def malformed_reader(arguments, deadline):
            if arguments[0].endswith('otool') and Path(arguments[-1]) == self.images[2]:
                return 'image:\nLoad command 0\ncmd LC_RPATH\ncmdsize 8\npath bad (offset 12)\n'
            return self.run_tool(arguments, deadline)
        with self.assertRaises(ValueError):
            NORMALIZER.normalize(self.app, self.receipt, malformed_reader)
        self.assertFalse(any(call[0].endswith('install_name_tool') for call in self.calls))
        self.assertFalse(self.receipt.exists())

    def test_duplicate_and_argument_budgets_fail_before_tool_invocation(self):
        with self.assertRaises(ValueError):
            NORMALIZER.deletion_arguments(['@loader_path'] * (NORMALIZER.MAX_COMMANDS + 1))
        with self.assertRaises(ValueError):
            NORMALIZER.deletion_arguments(['/' + str(index) + 'x' * 1024 for index in range(64)])

    def test_escaping_receipt_and_canonical_pre_sign_order(self):
        self.receipt.symlink_to(self.root / 'outside.json')
        with self.assertRaises(ValueError):
            NORMALIZER.normalize(self.app, self.receipt, self.run_tool)
        self.assertEqual(self.calls, [])
        source = (SCRIPT.parent / 'prepare-app-bundle.sh').read_text()
        deploy = source.index('macdeployqt "$app"')
        normalize = source.index('normalize-owned-rpaths.py')
        protect = source.index('protect-owned-binaries.py')
        sign = source.index('codesign "${sign_args[@]}"')
        self.assertLess(deploy, normalize)
        self.assertLess(normalize, protect)
        self.assertLess(protect, sign)


if __name__ == '__main__':
    unittest.main()
