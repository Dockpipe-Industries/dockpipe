"""Exercise actual CMake test properties; platform flags simulated, not native proof."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

REPOSITORY_ROOT = Path(__file__).resolve().parents[4]
HELPER = Path(__file__).resolve().parents[4] / 'src/app/tooling/dockpipe-launcher/cmake/WindowsOffscreenTestFonts.cmake'


class FontFixtureTests(unittest.TestCase):
    def configure(self, platform='native', fonts=True, default_offscreen=True):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        root = Path(temporary.name)
        source = root / 'source'
        source.mkdir()
        os_root = root / 'OS root'
        if fonts:
            (os_root / 'Fonts').mkdir(parents=True)
        platform_flags = {
            'native': 'set(WIN32 TRUE)\nset(CMAKE_HOST_WIN32 TRUE)',
            'linux': 'set(WIN32 FALSE)\nset(CMAKE_HOST_WIN32 FALSE)',
            'cross': 'set(WIN32 TRUE)\nset(CMAKE_HOST_WIN32 FALSE)',
        }[platform]
        project = f'''cmake_minimum_required(VERSION 3.21)
project(FontFixture NONE)
enable_testing()
set(BUILD_TESTING TRUE)
{platform_flags}
add_test(NAME native COMMAND "${{CMAKE_COMMAND}}" -E true)
set_tests_properties(native PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=windows" TIMEOUT 11)
add_test(NAME explicit COMMAND "${{CMAKE_COMMAND}}" -E true)
set_tests_properties(explicit PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen;QT_QPA_FONTDIR=/explicit" TIMEOUT 23)
add_test(NAME plain COMMAND "${{CMAKE_COMMAND}}" -E true)
'''
        names = ['native', 'explicit', 'plain']
        if default_offscreen:
            project += '''add_test(NAME offscreen COMMAND "${CMAKE_COMMAND}" -E echo "quoted argument")
set_tests_properties(offscreen PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen;KEEP=one" TIMEOUT 17 SKIP_RETURN_CODE 77)
'''
            names.append('offscreen')
        project += 'include("' + HELPER.as_posix() + '")\n'
        project += (
            'dockpipe_configure_windows_offscreen_test_fonts("'
            + (os_root / 'Fonts').as_posix() + '")\n'
        )
        project += (
            'file(WRITE "${CMAKE_BINARY_DIR}/system-root.txt" "$ENV{SystemRoot}")\n'
        )
        for name in names:
            for property_name, suffix in (('ENVIRONMENT', ''), ('TIMEOUT', '_timeout')):
                project += (
                    f'get_test_property({name} {property_name} value)\n'
                    'file(APPEND "${CMAKE_BINARY_DIR}/observed.txt"\n'
                    f'    "{name}{suffix}=${{value}}\\n")\n'
                )
        if default_offscreen:
            project += (
                'get_test_property(offscreen SKIP_RETURN_CODE value)\n'
                'file(APPEND "${CMAKE_BINARY_DIR}/observed.txt"\n'
                '    "offscreen_skip=${value}\\n")\n'
            )
        (source / 'CMakeLists.txt').write_text(project)
        environment = dict(os.environ)
        result = subprocess.run(
            ['cmake', '-S', str(source), '-B', str(root / 'build')],
            env=environment, capture_output=True, text=True, timeout=15,
        )
        observed = {}
        system_root_record = root / 'build/system-root.txt'
        if result.returncode == 0:
            self.assertEqual(
                os.environ.get('SystemRoot', ''), system_root_record.read_text(),
                'Nested CMake must retain the actual OS SystemRoot for tool discovery',
            )
        path = root / 'build/observed.txt'
        if path.exists():
            observed = dict(line.split('=', 1) for line in path.read_text().splitlines())
        generated = root / 'build/CTestTestfile.cmake'
        if generated.exists():
            observed['generated_tests'] = generated.read_text()
        return result, observed, (os_root / 'Fonts').as_posix()

    def test_native_offscreen_appends_only_missing_font_fixture(self):
        result, observed, fonts = self.configure()
        self.assertEqual(0, result.returncode, result.stderr)
        expected_environment = 'QT_QPA_PLATFORM=offscreen;KEEP=one;QT_QPA_FONTDIR=' + fonts
        self.assertEqual(expected_environment, observed['offscreen'])
        self.assertEqual('QT_QPA_PLATFORM=windows', observed['native'])
        self.assertEqual('QT_QPA_PLATFORM=offscreen;QT_QPA_FONTDIR=/explicit', observed['explicit'])
        self.assertEqual('NOTFOUND', observed['plain'])
        self.assertEqual('11', observed['native_timeout'])
        self.assertEqual('23', observed['explicit_timeout'])
        self.assertEqual('17', observed['offscreen_timeout'])
        self.assertEqual('77', observed['offscreen_skip'])
        self.assertRegex(
            observed['generated_tests'],
            r'add_test\(\[=\[offscreen\]=\].*"-E" "echo" "quoted argument"\)',
        )

    def test_other_hosts_do_not_acquire_the_windows_fixture(self):
        for platform in ('linux', 'cross'):
            with self.subTest(platform=platform):
                result, observed, _ = self.configure(platform=platform, fonts=False)
                self.assertEqual(0, result.returncode, result.stderr)
                self.assertEqual('QT_QPA_PLATFORM=offscreen;KEEP=one', observed['offscreen'])

    def test_missing_required_os_fonts_is_actionable(self):
        result, _, _ = self.configure(fonts=False)
        self.assertNotEqual(0, result.returncode)
        self.assertIn('installed OS Fonts directory', result.stderr)

    def test_native_or_explicit_font_tests_do_not_require_os_font_fallback(self):
        result, observed, _ = self.configure(fonts=False, default_offscreen=False)
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertEqual('QT_QPA_PLATFORM=windows', observed['native'])
        self.assertEqual('QT_QPA_PLATFORM=offscreen;QT_QPA_FONTDIR=/explicit', observed['explicit'])


if __name__ == '__main__':
    unittest.main()
