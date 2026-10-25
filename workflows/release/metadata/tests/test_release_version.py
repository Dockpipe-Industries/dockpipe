"""Generated patch numbers must upgrade cleanly across every native installer."""
import importlib.util
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


spec = importlib.util.spec_from_file_location(
    "release_version", Path(__file__).resolve().parents[4] / 'workflows/release/metadata/assets/scripts/release-version.py')
versions = importlib.util.module_from_spec(spec)
spec.loader.exec_module(versions)


class ReleaseVersionTests(unittest.TestCase):
    def test_cli_build_preserves_authored_version_after_failure(self):
        source = Path(__file__).resolve().parents[4] / 'workflows/build/cli/assets/scripts/build.py'
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            baseline = root / 'VERSION'
            baseline.write_bytes(b'0.6.0\n')
            go = root / 'go'
            go.write_text('#!/bin/sh\n[ "$(cat VERSION)" = 0.6.0 ] || exit 99\nexit 42\n')
            go.chmod(0o755)
            result = subprocess.run(['python3', str(source), '--version', '0.6.1',
                                     '--output', str(root / 'dockpipe'), '--go', str(go)],
                                    cwd=root, env=dict(os.environ, DOCKPIPE_SOURCE_ROOT=str(root)),
                                    capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('42', result.stderr)
            self.assertEqual(baseline.read_bytes(), b'0.6.0\n')

    def test_stable_and_staging_share_one_increasing_native_sequence(self):
        history = ["v0.5.8", "v0.6.0-staging.123.1.aaaaaaaaaaaa", "unrelated-tag"]
        staging = versions.metadata("0.6.0", history, "staging.124.1.bbbbbbbbbbbb")
        self.assertEqual(staging["version"], "0.6.1")
        self.assertEqual(staging["candidate"], "0.6.1-staging.124.1.bbbbbbbbbbbb")
        history.append(staging["tag"])
        stable = versions.metadata("0.6.0", history)
        self.assertEqual(stable, {"version": "0.6.2", "candidate": "", "tag": "v0.6.2", "previous_tag": "v0.5.8"})
        history.append(stable["tag"])
        self.assertEqual(versions.next_version("0.6.0", history), "0.6.3")

    def test_baseline_and_unordered_history_do_not_reuse_versions(self):
        history = ["v0.6.9", "v0.6.2-staging.999.4.aaaaaaaaaaaa", "v0.5.100"]
        self.assertEqual(versions.next_version("0.6.0", history), "0.6.10")
        self.assertEqual(versions.next_version("0.7.0", history), "0.7.0")
        self.assertEqual(versions.next_version("0.6.20", history), "0.6.20")

    def test_notes_compare_stable_to_stable_and_staging_to_latest_candidate(self):
        history = ["v0.6.2", "v0.6.12-staging.123.1.aaaaaaaaaaaa", "v0.6.9", "other"]
        self.assertEqual(versions.metadata("0.6.0", history)["previous_tag"], "v0.6.9")
        staging = versions.metadata("0.6.0", history, "staging.124.1.bbbbbbbbbbbb")
        self.assertEqual(staging["previous_tag"], "v0.6.12-staging.123.1.aaaaaaaaaaaa")
        self.assertEqual(versions.metadata("1.0.0", ["unrelated"])["previous_tag"], "")
        staging_only = ["v0.6.12-staging.123.1.aaaaaaaaaaaa"]
        self.assertEqual(versions.metadata("0.6.0", staging_only)["previous_tag"], "")

    def test_downgrades_and_windows_version_overflow_fail(self):
        for baseline, history in [
            ("0.6.0", ["v0.7.0"]), ("0.6.0", ["v0.6.65535"]),
            ("256.0.0", []), ("0.256.0", []), ("0.6.65536", []), ("0.06.0", []),
        ]:
            with self.subTest(baseline=baseline, history=history), self.assertRaises(ValueError):
                versions.next_version(baseline, history)

    @unittest.skipUnless(shutil.which("dpkg"), "dpkg required")
    def test_native_versions_advance_under_real_debian_ordering(self):
        history = ["v0.6.0", "v0.6.9-staging.10.1.aaaaaaaaaaaa"]
        generated = versions.next_version("0.6.0", history)
        for old in ("0.6.0", "0.6.9"):
            subprocess.run(["dpkg", "--compare-versions", generated, "gt", old], check=True)


if __name__ == "__main__":
    unittest.main()
