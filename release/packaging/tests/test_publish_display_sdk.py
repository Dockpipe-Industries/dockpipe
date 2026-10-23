"""Immutable native Display SDK candidates never promote or replace releases."""
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


SPEC = importlib.util.spec_from_file_location(
    "display_candidate", Path(__file__).resolve().parents[1] / "publish-display-sdk.py")
PUBLISHER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PUBLISHER)
SOURCE = "a" * 40
TARGET = "darwin-arm64"


def environment():
    return {"GITHUB_REPOSITORY": PUBLISHER.REPOSITORY, "GITHUB_REF": "refs/heads/dev",
            "GITHUB_EVENT_NAME": "push", "GITHUB_SHA": SOURCE}


def delivery(root, target=TARGET):
    archive = PUBLISHER.archive_name(target)
    (root / archive).write_bytes(b"qualified immutable archive")
    (root / "sdk-files.json").write_text('{"schema":1,"artifact_class":"sdk","files":[]}\n')
    metadata = {
        "schema": 1, "product": "DockPipeDisplaySDK", "version": "1.0.0", "platform": target,
        "qt_version": "6.10.3", "interface": "com.dockpipe.launcher.DisplayPackage/1.0",
        "source_repository": PUBLISHER.REPOSITORY, "source_sha": SOURCE,
        "source_state": "committed-snapshot", "source_digest": "b" * 64,
        "compiler": {"id": "AppleClang", "version": "17.0"},
        "archive": {"name": archive, "size_bytes": (root / archive).stat().st_size,
                    "sha256": PUBLISHER.SHARED.digest(root / archive)},
        "file_manifest": {"name": "sdk-files.json", "sha256": PUBLISHER.SHARED.digest(root / "sdk-files.json")},
        "checks": ["sdk-component-install", "relocated-external-consumer-compile-link", "native-consumer-run"],
    }
    (root / "sdk-release.json").write_text(json.dumps(metadata))
    return metadata


class DisplayCandidateTests(unittest.TestCase):
    def test_manual_production_pr_wrong_repository_and_invalid_sha_fail_before_network(self):
        cases = [
            {"GITHUB_EVENT_NAME": "workflow_dispatch"}, {"GITHUB_EVENT_NAME": "pull_request"},
            {"GITHUB_REF": "refs/heads/master"}, {"GITHUB_REF": "refs/heads/staging"},
            {"GITHUB_REPOSITORY": "other/repository"}, {"GITHUB_SHA": "short"},
        ]
        for changed in cases:
            with self.subTest(changed=changed), patch.dict(os.environ, environment() | changed, clear=True), \
                    patch.object(PUBLISHER.SHARED, "run") as run:
                with self.assertRaisesRegex(ValueError, "accepted dev push"):
                    PUBLISHER.publish(Path("not-read"), TARGET)
                run.assert_not_called()

    def test_each_qualified_platform_has_a_distinct_archive(self):
        self.assertEqual(len({PUBLISHER.archive_name(target) for target in PUBLISHER.PLATFORMS}), 4)
        with self.assertRaisesRegex(ValueError, "qualification lane"):
            PUBLISHER.archive_name("linux-arm64")

    def test_delivery_binds_source_platform_and_native_checks(self):
        for changes in ({"source_sha": "c" * 40}, {"platform": "darwin-amd64"},
                        {"source_state": "development"}, {"interface": "wrong"},
                        {"checks": ["sdk-component-install"]}, {"source_digest": ""},
                        {"compiler": {"id": "", "version": "17"}}):
            with self.subTest(changes=changes), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                metadata = delivery(root)
                PUBLISHER.validate(root, SOURCE, TARGET)
                metadata.update(changes)
                (root / "sdk-release.json").write_text(json.dumps(metadata))
                with self.assertRaises(ValueError):
                    PUBLISHER.validate(root, SOURCE, TARGET)

    def test_archive_and_manifest_tampering_are_rejected(self):
        for name in (PUBLISHER.archive_name(TARGET), "sdk-files.json"):
            with self.subTest(name=name), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                delivery(root)
                (root / name).write_bytes(b"changed bytes")
                with self.assertRaisesRegex(ValueError, "qualification receipt"):
                    PUBLISHER.validate(root, SOURCE, TARGET)

    def test_unexpected_and_oversized_files_are_rejected_before_hashing(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            delivery(root)
            (root / "extra").write_text("unexpected")
            with self.assertRaisesRegex(ValueError, "unexpected assets"):
                PUBLISHER.validate(root, SOURCE, TARGET)
            (root / "extra").unlink()
            with (root / PUBLISHER.archive_name(TARGET)).open("wb") as stream:
                stream.truncate(PUBLISHER.MAX_ARCHIVE_BYTES + 1)
            with patch.object(PUBLISHER.SHARED, "digest") as digest:
                with self.assertRaisesRegex(ValueError, "bounded regular"):
                    PUBLISHER.validate(root, SOURCE, TARGET)
                digest.assert_not_called()

    def test_existing_tag_at_another_commit_never_creates_a_release(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            delivery(root)
            reference = {"data": {"repository": {"ref": {"target": {
                "__typename": "Commit", "oid": "c" * 40}}}}}
            with patch.dict(os.environ, environment(), clear=True), \
                    patch.object(PUBLISHER.SHARED, "find_release", return_value=None), \
                    patch.object(PUBLISHER.SHARED, "run", return_value=json.dumps(reference)) as run:
                with self.assertRaisesRegex(ValueError, "exact qualified source"):
                    PUBLISHER.publish(root, TARGET)
            self.assertEqual(run.call_count, 1)
            self.assertIn("graphql", run.call_args.args[0])

    def test_conflicting_existing_candidate_never_mutates(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            delivery(root)
            assets = PUBLISHER.validate(root, SOURCE, TARGET)
            tag = f"display-sdk-1.0.0-{TARGET}-staging.{SOURCE}"
            release = {"id": 123, "tag_name": tag, "target_commitish": SOURCE,
                       "prerelease": True, "draft": False,
                       "assets": [{"name": name, "state": "uploaded", **data} for name, data in assets.items()]}
            release["assets"][0]["digest"] = "sha256:" + "0" * 64
            reference = {"data": {"repository": {"ref": {"target": {"__typename": "Commit", "oid": SOURCE}}}}}
            with patch.dict(os.environ, environment(), clear=True), \
                    patch.object(PUBLISHER.SHARED, "find_release", return_value=release), \
                    patch.object(PUBLISHER.SHARED, "run", return_value=json.dumps(reference)) as run:
                with self.assertRaisesRegex(ValueError, "never replaced"):
                    PUBLISHER.publish(root, TARGET)
            self.assertTrue(all("graphql" in call.args[0] for call in run.call_args_list))


if __name__ == "__main__":
    unittest.main()
