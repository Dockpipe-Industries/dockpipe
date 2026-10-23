"""CLI bootstrap candidates bind public inputs, installer formats and immutable publication."""
import importlib.util
import json
import os
import subprocess
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch
import zipfile


ROOT = Path(__file__).resolve().parents[1]


def load(name, path):
    specification = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


PRODUCER = load("cli_producer_test", ROOT / "build-cli-candidate.py")
PUBLISHER = load("cli_publisher_test", ROOT / "publish-cli-candidate.py")
SOURCE = "012345678901" + "a" * 28


def source_fixture(root):
    source = root / "source"
    source.mkdir()
    for name, content in {"VERSION": "0.6.0\n", "go.mod": "module fixture\n", "go.sum": "pinned\n"}.items():
        (source / name).write_text(content)
    return source


def build_fixture(root):
    source = source_fixture(root)
    work, output = root / "work", root / "output"

    def execute(arguments, directory, environment, timeout=300):
        if arguments[1:] == ["env", "GOVERSION"]:
            return PRODUCER.GO_VERSION
        if arguments[1:] == ["mod", "edit", "-json"]:
            return '{"Replace":null}'
        if arguments[1] == "build":
            binary = Path(arguments[arguments.index("-o") + 1])
            binary.write_bytes(b"owned binary fixture")
        return ""

    with patch.object(PRODUCER, "command", side_effect=execute), \
            patch.object(PRODUCER, "verify_native_cli") as smoke, patch("builtins.print"):
        PRODUCER.build(source, SOURCE, work, output, "go")
    if smoke.call_count != 1:
        raise AssertionError("Native candidate was not qualified exactly once")
    return output


class CLIBootstrapTests(unittest.TestCase):
    def test_version_is_complete_semver_even_for_numeric_leading_zero_hash(self):
        version, tag = PRODUCER.candidate_identity("0.6.0", SOURCE)
        self.assertEqual(version, "0.6.0-staging.cli.sha012345678901")
        self.assertTrue(tag.endswith(SOURCE))
        for base, source in (("v0.6.0", SOURCE), ("0.6", SOURCE), ("0.6.0-rc.1", SOURCE), ("0.6.0", "dev")):
            with self.assertRaises(ValueError):
                PRODUCER.candidate_identity(base, source)

    def test_public_module_environment_removes_local_and_private_overrides(self):
        hostile = {name: "private" for name in ("GOPROXY", "GOSUMDB", "GOFLAGS", "GOPRIVATE", "GONOSUMDB", "GONOPROXY", "GOWORK", "GOCACHEPROG")}
        with patch.dict(os.environ, hostile):
            environment = PRODUCER.public_environment(Path("/fresh"))
        self.assertEqual(environment["GOPROXY"], "https://proxy.golang.org")
        self.assertEqual(environment["GOSUMDB"], "sum.golang.org")
        self.assertEqual(environment["GOMODCACHE"], "/fresh/modules")
        self.assertEqual(environment["GOWORK"], "off")
        self.assertEqual(environment["CGO_ENABLED"], "0")
        self.assertEqual(environment["GOAUTH"], "off")
        self.assertNotIn("GOCACHEPROG", environment)
        self.assertNotIn("GOPRIVATE", environment)
        self.assertNotIn("GONOSUMDB", environment)
        self.assertNotIn("GONOPROXY", environment)

    def test_flat_installer_archive_formats_are_deterministic(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            binary = root / "binary"
            binary.write_bytes(b"single executable")
            for windows in (False, True):
                extension = ".zip" if windows else ".tar.gz"
                first, second = root / ("first" + extension), root / ("second" + extension)
                PRODUCER.archive_binary(binary, first, windows)
                PRODUCER.archive_binary(binary, second, windows)
                self.assertEqual(first.read_bytes(), second.read_bytes())
                if windows:
                    with zipfile.ZipFile(first) as archive:
                        self.assertEqual(archive.namelist(), ["dockpipe.exe"])
                        self.assertEqual(archive.read("dockpipe.exe"), binary.read_bytes())
                else:
                    with tarfile.open(first) as archive:
                        self.assertEqual(archive.getnames(), ["dockpipe"])
                        self.assertTrue(archive.getmember("dockpipe").isfile())
                        self.assertEqual(archive.extractfile("dockpipe").read(), binary.read_bytes())

    def test_replacements_fail_before_module_download(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = source_fixture(root)
            with patch.object(PRODUCER, "command", side_effect=["", PRODUCER.GO_VERSION, '{"Replace":[{"New":{"Path":"../private"}}]}']) as execute:
                with self.assertRaisesRegex(ValueError, "replacements"):
                    PRODUCER.build(source, SOURCE, root / "work", root / "out", "go")
            self.assertEqual(execute.call_count, 3)

    def test_stale_embed_manifest_fails_before_any_go_command(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = source_fixture(root)
            with patch.object(PRODUCER, "command", side_effect=ValueError("stale embed manifest")) as execute:
                with self.assertRaisesRegex(ValueError, "stale embed manifest"):
                    PRODUCER.build(source, SOURCE, root / "work", root / "out", "go")
            self.assertEqual(execute.call_count, 1)
            arguments = execute.call_args.args[0]
            self.assertEqual(arguments[1:], [source / "release/packaging/embedded-inputs.py", "--check"])

    def test_failed_smoke_exposes_bounded_stdout_and_stderr_diagnostics(self):
        error = subprocess.CalledProcessError(23, ["dockpipe"],
                                             output=b"host-step failure details", stderr=b"native error " + b"x" * 9000)
        with patch.object(PRODUCER.subprocess, "run", side_effect=error):
            with self.assertRaises(ValueError) as raised:
                PRODUCER.command(["dockpipe", "--workflow-file", "proof.yml"], Path("."), {})
        message = str(raised.exception)
        self.assertIn("exit 23", message)
        self.assertIn("host-step failure details", message)
        self.assertIn("native error", message)
        self.assertIn("[truncated after 4096 bytes]", message)
        self.assertLess(len(message), 8500)

    def test_five_platform_metadata_and_archive_hashes_validate(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = build_fixture(Path(temporary))
            metadata = json.loads((output / "cli-candidate.json").read_text())
            tag, assets = PUBLISHER.validate(output, SOURCE)
            self.assertEqual(set(metadata["artifacts"]), set(PRODUCER.TARGETS))
            self.assertEqual(len(assets), 6)
            self.assertEqual(tag, metadata["tag"])
            self.assertEqual(metadata["native_execution_platforms"], ["linux/amd64"])

    def test_changed_archive_missing_target_and_missing_hook_receipt_fail(self):
        for mutation in ("bytes", "target", "hook"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                output = build_fixture(Path(temporary))
                path = output / "cli-candidate.json"
                metadata = json.loads(path.read_text())
                if mutation == "bytes":
                    (output / metadata["artifacts"]["linux/amd64"]["name"]).write_bytes(b"changed")
                elif mutation == "target":
                    del metadata["artifacts"]["windows/amd64"]
                else:
                    metadata["checks"].remove("native-python-package-source-hook")
                path.write_text(json.dumps(metadata))
                with self.assertRaises(ValueError):
                    PUBLISHER.validate(output, SOURCE)

    def test_manual_or_production_publication_fails_before_network(self):
        for changes in ({"GITHUB_REF": "refs/heads/master"}, {"GITHUB_EVENT_NAME": "workflow_dispatch"},
                        {"GITHUB_REPOSITORY": "other/repository"}):
            environment = {"GITHUB_REF": "refs/heads/dev", "GITHUB_EVENT_NAME": "push",
                           "GITHUB_REPOSITORY": PUBLISHER.REPOSITORY, "GITHUB_SHA": SOURCE} | changes
            with patch.dict(os.environ, environment, clear=True), patch.object(PUBLISHER.SHARED, "run") as run:
                with self.assertRaisesRegex(ValueError, "accepted dev push"):
                    PUBLISHER.publish(Path("not-read"))
                run.assert_not_called()

    def test_conflicting_remote_archive_never_mutates_a_release(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = build_fixture(Path(temporary))
            tag, assets = PUBLISHER.validate(output, SOURCE)
            release = {"id": 123, "tag_name": tag, "target_commitish": SOURCE,
                       "prerelease": True, "draft": False,
                       "assets": [{"name": name, **data} for name, data in assets.items()]}
            release["assets"][0]["digest"] = "sha256:" + "0" * 64
            reference = {"data": {"repository": {"ref": {"target": {
                "__typename": "Commit", "oid": SOURCE}}}}}
            environment = {"GITHUB_REF": "refs/heads/dev", "GITHUB_EVENT_NAME": "push",
                           "GITHUB_REPOSITORY": PUBLISHER.REPOSITORY, "GITHUB_SHA": SOURCE}
            with patch.dict(os.environ, environment, clear=True), \
                    patch.object(PUBLISHER.SHARED, "find_release", return_value=release), \
                    patch.object(PUBLISHER.SHARED, "run", return_value=json.dumps(reference)) as run:
                with self.assertRaisesRegex(ValueError, "never replaced"):
                    PUBLISHER.publish(output)
            self.assertEqual(run.call_count, 1)
            self.assertIn("graphql", run.call_args.args[0])

    def test_fresh_and_resumed_candidates_preserve_immutable_assets(self):
        for state in ("absent", "draft", "published"):
            with self.subTest(state=state), tempfile.TemporaryDirectory() as temporary:
                output = build_fixture(Path(temporary))
                tag, assets = PUBLISHER.validate(output, SOURCE)
                release = {"id": 123, "tag_name": tag, "target_commitish": SOURCE, "prerelease": True,
                           "draft": state != "published", "html_url": "https://github.com/example/candidate",
                           "assets": [{"name": name, **data} for name, data in assets.items()]}
                exists = state != "absent"
                calls = []

                def execute(arguments):
                    nonlocal exists
                    calls.append(arguments)
                    if arguments[1:3] == ["release", "create"]:
                        self.assertFalse(exists)
                        self.assertIn("--draft", arguments)
                        self.assertIn("--latest=false", arguments)
                        exists = True
                        return ""
                    if arguments[1:3] == ["release", "edit"]:
                        self.assertIn("--latest=false", arguments)
                        release["draft"] = False
                        return ""
                    if "graphql" in arguments:
                        reference = None if release["draft"] else {"target": {"__typename": "Commit", "oid": SOURCE}}
                        return json.dumps({"data": {"repository": {"ref": reference}}})
                    if arguments[-1].endswith("/releases?per_page=100&page=1"):
                        return json.dumps([release] if exists else [])
                    if arguments[-1].endswith("/releases/123"):
                        return json.dumps(release)
                    self.fail("Unexpected publication command")

                environment = {"GITHUB_REF": "refs/heads/dev", "GITHUB_EVENT_NAME": "push",
                               "GITHUB_REPOSITORY": PUBLISHER.REPOSITORY, "GITHUB_SHA": SOURCE}
                with patch.dict(os.environ, environment, clear=True), \
                        patch.object(PUBLISHER.SHARED, "run", side_effect=execute), patch("builtins.print"):
                    PUBLISHER.publish(output)
                self.assertEqual(sum(call[1:3] == ["release", "create"] for call in calls), int(state == "absent"))
                self.assertEqual(sum(call[1:3] == ["release", "edit"] for call in calls), int(state != "published"))
                self.assertFalse(any("--clobber" in call or "upload" in call or "delete" in call for call in calls))


if __name__ == "__main__":
    unittest.main()
