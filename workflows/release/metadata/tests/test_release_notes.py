"""Release summaries bind exact inputs without an authored version-named body."""
import importlib.util
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "assets/scripts/release-notes.py"
SPEC = importlib.util.spec_from_file_location("release_notes", SCRIPT)
NOTES = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(NOTES)


class ReleaseNotesTests(unittest.TestCase):
    def test_stable_summary_pins_installation_and_comparison_to_source(self):
        source = "a" * 40
        body = NOTES.render("owner/repo", source, "v1.2.3", "v1.2.0")
        self.assertIn("# Dockpipe v1.2.3", body)
        self.assertIn(f"/blob/{source}/docs/install.md", body)
        self.assertIn(f"/compare/v1.2.0...{source}", body)
        self.assertNotIn("staging candidate", body)

    def test_first_staging_summary_does_not_invent_a_previous_release(self):
        tag = "v1.2.3-staging.123.1.aaaaaaaaaaaa"
        body = NOTES.render("owner/repo", "a" * 40, tag)
        self.assertIn("staging candidate", body)
        self.assertNotIn("/compare/", body)

    def test_invalid_input_does_not_replace_existing_output(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "notes.md"
            output.write_text("Existing summary\n")
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--repository", "owner/repo",
                 "--source-sha", "master", "--tag", "v1.2.3", "--output", str(output)],
                capture_output=True, text=True,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(output.read_text(), "Existing summary\n")

    def test_rejects_untrusted_repository_and_tag_values(self):
        for repository, tag, previous in [
            ("owner/repo\nextra", "v1.2.3", ""),
            ("owner/repo", "v1.2.3\nextra", ""),
            ("owner/repo", "v1.2.3", "master"),
        ]:
            with self.subTest(repository=repository, tag=tag, previous=previous):
                with self.assertRaises(ValueError):
                    NOTES.render(repository, "a" * 40, tag, previous)


if __name__ == "__main__":
    unittest.main()
