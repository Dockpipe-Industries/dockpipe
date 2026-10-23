"""Exercise DMG creation failure recovery without mounting or deleting volumes."""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


REPOSITORY = Path(__file__).resolve().parents[3]
SCRIPT = REPOSITORY / "release/packaging/desktop/macos/create-dmg.sh"
VERIFY_SCRIPT = SCRIPT.with_name("verify-dmg.sh")

HDIUTIL_FIXTURE = r"""#!/usr/bin/env python3
import json
import os
from pathlib import Path
import sys

if sys.argv[1] == "verify":
    if os.environ["DMG_TEST_MODE"] == "verify-permission":
        print("hdiutil: verify failed - Permission denied", file=sys.stderr)
        sys.exit(7)
    sys.exit(0)

record = Path(os.environ["DMG_TEST_RECORD"])
calls = json.loads(record.read_text()) if record.exists() else []
calls.append(sys.argv[1:])
record.write_text(json.dumps(calls))
mode = os.environ["DMG_TEST_MODE"]
candidate = Path(sys.argv[-1])
candidate.write_bytes(b"partial image")
if mode == "busy" or (mode == "recover" and len(calls) < 3):
    print("hdiutil: create failed - Resource busy", file=sys.stderr)
    sys.exit(1)
if mode == "cancel":
    print("hdiutil: create failed - Resource busy", file=sys.stderr)
    sys.exit(130)
if mode == "other-busy-status":
    print("hdiutil: create failed - Resource busy", file=sys.stderr)
    sys.exit(7)
if mode == "permission":
    print("hdiutil: create failed - Permission denied", file=sys.stderr)
    sys.exit(7)
if mode == "empty":
    candidate.write_bytes(b"")
    sys.exit(0)
if mode == "race":
    Path(os.environ["DMG_TEST_OUTPUT"]).write_bytes(b"other owner")
if mode == "race-directory":
    Path(os.environ["DMG_TEST_OUTPUT"]).mkdir()
if mode == "race-symlink":
    Path(os.environ["DMG_TEST_OUTPUT"]).symlink_to(
        Path(os.environ["DMG_TEST_OUTPUT"]).parent, target_is_directory=True
    )
candidate.write_bytes(b"qualified image")
"""

SLEEP_FIXTURE = r"""#!/usr/bin/env python3
import os
from pathlib import Path
import sys

with Path(os.environ["DMG_TEST_DELAYS"]).open("a") as stream:
    stream.write(sys.argv[1] + "\n")
sys.exit(int(os.environ.get("DMG_TEST_SLEEP_EXIT", "0")))
"""


class MacosDmgTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="dockpipe dmg test ")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.source = self.root / "installer source"
        self.source.mkdir()
        (self.source / "Install Dockpipe.pkg").write_bytes(b"preserved installer")
        self.output = self.root / "new installer.dmg"
        self.record = self.root / "calls.json"
        self.delays = self.root / "delays.txt"
        tools = self.root / "tools"
        tools.mkdir()
        for name, content in (("hdiutil", HDIUTIL_FIXTURE), ("sleep", SLEEP_FIXTURE)):
            executable = tools / name
            executable.write_text(content)
            executable.chmod(0o755)
        self.environment = dict(
            os.environ,
            PATH=str(tools) + os.pathsep + os.environ["PATH"],
            TMPDIR=str(self.root),
            DMG_TEST_RECORD=str(self.record),
            DMG_TEST_DELAYS=str(self.delays),
            DMG_TEST_OUTPUT=str(self.output),
        )

    def run_creation(self, mode):
        result = subprocess.run(
            ["bash", str(SCRIPT), str(self.source), str(self.output)],
            env=dict(self.environment, DMG_TEST_MODE=mode),
            capture_output=True,
            text=True,
            timeout=10,
        )
        self.assertEqual(
            (self.source / "Install Dockpipe.pkg").read_bytes(), b"preserved installer"
        )
        calls = json.loads(self.record.read_text()) if self.record.exists() else []
        evidence = list(self.output.parent.glob("*.work.*"))
        if result.returncode == 0 or not calls:
            self.assertEqual(evidence, [])
        else:
            self.assertEqual(len(evidence), 1)
            self.assertEqual(len(list(evidence[0].glob("attempt-*.log"))), len(calls))
            self.assertIn(str(evidence[0]), result.stderr)
        for call in calls:
            self.assertEqual(
                call[:-1],
                ["create", "-volname", "Dockpipe", "-srcfolder", str(self.source),
                 "-format", "UDZO"],
            )
        return result, calls

    def test_success_publishes_without_retry(self):
        result, calls = self.run_creation("success")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(calls), 1)
        self.assertEqual(self.output.read_bytes(), b"qualified image")
        self.assertFalse(self.delays.exists())

    def test_busy_recovery_uses_distinct_images_and_bounded_backoff(self):
        result, calls = self.run_creation("recover")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(calls), 3)
        self.assertEqual(len({call[-1] for call in calls}), 3)
        self.assertEqual(self.delays.read_text(), "2\n4\n")
        self.assertEqual(self.output.read_bytes(), b"qualified image")
        self.assertEqual(result.stderr.count("hdiutil: create failed - Resource busy"), 2)

    def test_persistent_busy_failure_stops_after_three_attempts(self):
        result, calls = self.run_creation("busy")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(len(calls), 3)
        self.assertEqual(self.delays.read_text(), "2\n4\n")
        self.assertFalse(self.output.exists())

    def test_other_failure_and_cancellation_are_not_retried(self):
        for mode, status in (("permission", 7), ("cancel", 130), ("other-busy-status", 7), ("verify-permission", 7)):
            with self.subTest(mode=mode):
                with tempfile.TemporaryDirectory(dir=self.root) as evidence_root:
                    original_output = self.output
                    self.output = Path(evidence_root) / "image.dmg"
                    self.environment["DMG_TEST_OUTPUT"] = str(self.output)
                    if self.record.exists():
                        self.record.unlink()
                    result, calls = self.run_creation(mode)
                    self.assertEqual(result.returncode, status)
                    self.assertEqual(len(calls), 1)
                    self.assertFalse(self.delays.exists())
                    self.assertFalse(self.output.exists())
                    self.output = original_output

    def test_existing_output_is_preserved(self):
        self.output.write_bytes(b"existing image")
        result, calls = self.run_creation("success")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(calls, [])
        self.assertEqual(self.output.read_bytes(), b"existing image")

    def test_racing_output_is_preserved(self):
        result, calls = self.run_creation("race")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(len(calls), 1)
        self.assertEqual(self.output.read_bytes(), b"other owner")

    def test_empty_success_is_rejected(self):
        result, calls = self.run_creation("empty")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(len(calls), 1)
        self.assertFalse(self.output.exists())

    def test_existing_directory_and_symlink_are_preserved(self):
        for kind in ("directory", "symlink"):
            with self.subTest(kind=kind):
                if kind == "directory":
                    self.output.mkdir()
                else:
                    self.output.symlink_to(self.source, target_is_directory=True)
                result, calls = self.run_creation("success")
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(calls, [])
                if kind == "directory":
                    self.assertEqual(list(self.output.iterdir()), [])
                    self.output.rmdir()
                else:
                    self.assertTrue(self.output.is_symlink())
                    self.assertEqual(self.output.readlink(), self.source)
                    self.output.unlink()

    def test_racing_directory_and_symlink_receive_no_image(self):
        for mode in ("race-directory", "race-symlink"):
            with self.subTest(mode=mode):
                with tempfile.TemporaryDirectory(dir=self.root) as evidence_root:
                    original_output = self.output
                    self.output = Path(evidence_root) / "image.dmg"
                    self.environment["DMG_TEST_OUTPUT"] = str(self.output)
                    if self.record.exists():
                        self.record.unlink()
                    result, calls = self.run_creation(mode)
                    self.assertNotEqual(result.returncode, 0)
                    self.assertEqual(len(calls), 1)
                    self.assertFalse((self.output / "attempt-1.dmg").exists())
                    self.assertEqual(self.output.is_symlink(), mode == "race-symlink")
                    self.output = original_output


VERIFY_FIXTURE = r"""#!/usr/bin/env python3
import json
import os
from pathlib import Path
import sys

record = Path(os.environ["DMG_TEST_RECORD"])
calls = json.loads(record.read_text()) if record.exists() else []
calls.append(sys.argv[1:])
record.write_text(json.dumps(calls))
mode = os.environ["DMG_TEST_MODE"]
if mode == "success" or (mode == "recover" and len(calls) == 3):
    print("verified checksum")
    sys.exit(0)
if mode == "corrupt":
    print("hdiutil: verify failed - checksum mismatch", file=sys.stderr)
    sys.exit(1)
if mode == "other-busy":
    print("hdiutil: verify failed - Resource busy", file=sys.stderr)
    sys.exit(1)
print('hdiutil: verify: unable to recognize image. (Resource temporarily unavailable)', file=sys.stderr)
print("hdiutil: verify failed - Resource temporarily unavailable", file=sys.stderr)
sys.exit({"cancel": 130, "other-status": 7}.get(mode, 1))
"""


class MacosDmgVerificationTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="dockpipe dmg verify test ")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.image = self.root / "image.dmg"
        self.image.write_bytes(b"unchanged image bytes")
        self.record = self.root / "calls.json"
        self.delays = self.root / "delays.txt"
        tools = self.root / "tools"
        tools.mkdir()
        for name, content in (("hdiutil", VERIFY_FIXTURE), ("sleep", SLEEP_FIXTURE)):
            executable = tools / name
            executable.write_text(content)
            executable.chmod(0o755)
        self.environment = dict(
            os.environ,
            PATH=str(tools) + os.pathsep + os.environ["PATH"],
            TMPDIR=str(self.root),
            DMG_TEST_RECORD=str(self.record),
            DMG_TEST_DELAYS=str(self.delays),
        )

    def verify(self, mode):
        result = subprocess.run(
            ["bash", str(VERIFY_SCRIPT), str(self.image)],
            env=dict(self.environment, DMG_TEST_MODE=mode),
            capture_output=True,
            text=True,
            timeout=10,
        )
        self.assertEqual(self.image.read_bytes(), b"unchanged image bytes")
        calls = json.loads(self.record.read_text()) if self.record.exists() else []
        self.assertTrue(all(call == ["verify", str(self.image)] for call in calls))
        evidence = list(self.root.glob("dockpipe-dmg-verify.*"))
        if result.returncode == 0:
            self.assertEqual(evidence, [])
        elif calls:
            self.assertEqual(len(evidence), 1)
            self.assertEqual(len(list(evidence[0].glob("attempt-*.log"))), len(calls))
            self.assertIn(str(evidence[0]), result.stderr)
        return result, calls

    def test_success_requires_no_retry(self):
        result, calls = self.verify("success")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(calls), 1)
        self.assertFalse(self.delays.exists())

    def test_measured_eagain_recovery_verifies_same_image(self):
        result, calls = self.verify("recover")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(calls), 3)
        self.assertEqual(self.delays.read_text(), "2\n4\n")
        self.assertIn("verified checksum", result.stdout)

    def test_persistent_eagain_remains_failure(self):
        result, calls = self.verify("busy")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(len(calls), 3)
        self.assertEqual(self.delays.read_text(), "2\n4\n")

    def test_corruption_and_other_busy_text_are_not_retried(self):
        for mode in ("corrupt", "other-busy"):
            with self.subTest(mode=mode):
                self.record.unlink(missing_ok=True)
                result, calls = self.verify(mode)
                self.assertEqual(result.returncode, 1)
                self.assertEqual(len(calls), 1)
                self.assertFalse(self.delays.exists())
                for evidence in self.root.glob("dockpipe-dmg-verify.*"):
                    for log in evidence.iterdir():
                        log.unlink()
                    evidence.rmdir()

    def test_cancellation_and_other_status_are_not_retried(self):
        for mode, status in (("cancel", 130), ("other-status", 7)):
            with self.subTest(mode=mode):
                self.record.unlink(missing_ok=True)
                result, calls = self.verify(mode)
                self.assertEqual(result.returncode, status)
                self.assertEqual(len(calls), 1)
                self.assertFalse(self.delays.exists())
                for evidence in self.root.glob("dockpipe-dmg-verify.*"):
                    for log in evidence.iterdir():
                        log.unlink()
                    evidence.rmdir()

    def test_interrupted_backoff_stops_before_another_verify(self):
        self.environment["DMG_TEST_SLEEP_EXIT"] = "143"
        result, calls = self.verify("busy")
        self.assertEqual(result.returncode, 143)
        self.assertEqual(len(calls), 1)
        self.assertEqual(self.delays.read_text(), "2\n")

    def test_symlink_is_rejected_before_hdiutil(self):
        original = self.root / "original.dmg"
        self.image.rename(original)
        self.image.symlink_to(original)
        result, calls = self.verify("success")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(calls, [])


if __name__ == "__main__":
    unittest.main()
