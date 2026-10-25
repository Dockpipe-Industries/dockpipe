"""The source-layout guard must reject retired paths after source extraction."""
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / 'assets/scripts/check-paths.sh'


class RepositoryLayoutTests(unittest.TestCase):
    def run_guard(self, path, contents):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            script = root / 'workflows/ci/repository-layout/assets/scripts/check-paths.sh'
            script.parent.mkdir(parents=True)
            shutil.copyfile(SCRIPT, script)
            source = root / path
            source.parent.mkdir(parents=True, exist_ok=True)
            source.write_text(contents)
            return subprocess.run(
                ['bash', str(script)], cwd=root, capture_output=True,
                text=True, timeout=10,
            )

    def test_missing_legacy_directories_do_not_hide_document_findings(self):
        result = self.run_guard('README.md', 'templates/core/scripts/obsolete.sh\n')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('README.md:1:', result.stderr)

    def test_current_go_source_roots_reject_retired_paths(self):
        for source in ('src/lib/fixture.go', 'src/cmd/fixture.go'):
            for retired in ('templates/core/images/icon.png',
                            'src/core/core/workflows/old.yml',
                            'templates/run-worktree/old.yml'):
                with self.subTest(source=source, retired=retired):
                    result = self.run_guard(source, retired + '\n')
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn(source + ':1:', result.stderr)

    def test_current_layout_without_removed_directories_passes(self):
        result = self.run_guard('src/lib/fixture.go', 'workflows/build/cli/config.yml\n')
        self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == '__main__':
    unittest.main()
