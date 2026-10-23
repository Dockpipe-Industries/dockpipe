"""CLI bootstrap candidates bind public inputs, installer formats and immutable publication."""
import hashlib
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


def allocation():
    return PRODUCER.allocation_receipt("0.6.0", SOURCE, ["v0.6.25"])


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
        PRODUCER.build(source, SOURCE, work, output, "go", allocation())
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

    def test_allocated_candidate_advances_published_native_release_without_version_edit(self):
        receipt = allocation()
        version, tag = PRODUCER.validate_allocation(receipt, "0.6.0", SOURCE)
        self.assertEqual(receipt["allocated_base"], "0.6.26")
        self.assertEqual(version, "0.6.26-staging.cli.sha012345678901")
        self.assertIn("0.6.26", tag)
        self.assertEqual(receipt["history"], "v0.6.25\n")
        self.assertEqual(receipt["history_sha256"], hashlib.sha256(b"v0.6.25\n").hexdigest())
        for key, value in (("history", "v0.6.24\n"), ("history_sha256", "0" * 64),
                           ("allocated_base", "0.6.25"), ("source_sha", "a" * 40),
                           ("release_line", "0.7.0"), ("schema", True)):
            with self.subTest(key=key), self.assertRaises(ValueError):
                PRODUCER.validate_allocation(receipt | {key: value}, "0.6.0", SOURCE)

    def test_allocation_history_is_bounded_before_build(self):
        with self.assertRaisesRegex(ValueError, "history exceeds"):
            PRODUCER.allocation_receipt("0.6.0", SOURCE, ["x" * (PRODUCER.MAX_HISTORY_BYTES + 1)])

    def test_history_acquisition_uses_public_tags_and_ignores_untagged_drafts(self):
        with tempfile.TemporaryDirectory() as temporary:
            source = source_fixture(Path(temporary))
            releases = [[{"tag_name": "v0.6.25", "draft": False},
                         {"tag_name": "v0.6.99", "draft": True}]]
            responses = ["\n".join(json.dumps(page) for page in releases), '["v0.6.25"]']
            with patch.object(PRODUCER, "command", side_effect=responses) as execute:
                receipt = PRODUCER.acquire_version_allocation(source, SOURCE, {})
            self.assertEqual(receipt["allocated_base"], "0.6.26")
            self.assertEqual(execute.call_count, 2)
            arguments = execute.call_args_list[0].args[0]
            self.assertEqual(arguments[:2], ["gh", "api"])
            self.assertIn("--paginate", arguments)
            self.assertNotIn("--slurp", arguments)
            self.assertIn("--jq", arguments)
            self.assertIn('select(.name == "cli-candidate.json")', arguments[-1])
            self.assertTrue(execute.call_args_list[1].args[0][-3].endswith('/tags?per_page=100'))
            self.assertNotIn("v0.6.99", receipt["history"])

    def test_existing_native_tag_consumes_version_before_release_is_completed(self):
        with tempfile.TemporaryDirectory() as temporary:
            source = source_fixture(Path(temporary))
            tags = ["v0.6.25", "v0.6.26-staging.123.1.aaaaaaaaaaaa"]
            # The later native tag exists, but its release listing is absent.
            with patch.object(PRODUCER, "command", side_effect=['[]', json.dumps(tags)]):
                receipt = PRODUCER.acquire_version_allocation(source, SOURCE, {})
            self.assertEqual(receipt["allocated_base"], "0.6.27")
            self.assertTrue(receipt["history_source"].endswith('/tags'))

    def test_retry_reuses_original_bound_history_after_newer_native_release(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = build_fixture(Path(temporary))
            manifest = (output / "cli-candidate.json").read_bytes()
            metadata = json.loads(manifest)
            release = {
                "tag_name": metadata["tag"], "draft": False, "prerelease": True,
                "target_commitish": SOURCE,
                "assets": [{"name": "cli-candidate.json", "id": 17, "size": len(manifest),
                            "digest": "sha256:" + hashlib.sha256(manifest).hexdigest()}],
            }
            pages = [[{"tag_name": "v0.6.29", "draft": False}, release]]
            for draft in (True, False):
                release["draft"] = draft
                with self.subTest(draft=draft), patch.object(
                        PRODUCER, "command", side_effect=["\n".join(json.dumps(page) for page in pages), manifest.decode()]) as execute:
                    receipt = PRODUCER.acquire_version_allocation(output.parent / "source", SOURCE, {})
                self.assertEqual(receipt, metadata["version_allocation"])
                self.assertEqual(receipt["allocated_base"], "0.6.26")
                self.assertEqual(execute.call_count, 2)
                self.assertTrue(execute.call_args.args[0][-1].endswith("/releases/assets/17"))
            release["assets"][0]["digest"] = "sha256:" + "0" * 64
            with patch.object(PRODUCER, "command", side_effect=["\n".join(json.dumps(page) for page in pages), manifest.decode()]):
                with self.assertRaisesRegex(ValueError, "manifest bytes differ"):
                    PRODUCER.acquire_version_allocation(output.parent / "source", SOURCE, {})

    def test_history_failure_never_falls_back_to_release_line(self):
        with tempfile.TemporaryDirectory() as temporary:
            source = source_fixture(Path(temporary))
            with patch.object(PRODUCER, "command", side_effect=ValueError("history unavailable")):
                with self.assertRaisesRegex(ValueError, "history unavailable"):
                    PRODUCER.acquire_version_allocation(source, SOURCE, {})
            for content in ("{}", "[{}]", '[[{"tag_name":"v0.6.25","draft":"false"}]]'):
                with self.subTest(content=content), patch.object(PRODUCER, "command", return_value=content):
                    with self.assertRaises(ValueError):
                        PRODUCER.acquire_version_allocation(source, SOURCE, {})

    def test_publisher_rejects_missing_or_tampered_allocation_before_network(self):
        for mutation in ("missing", "digest", "version"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                output = build_fixture(Path(temporary))
                path = output / "cli-candidate.json"
                metadata = json.loads(path.read_text())
                if mutation == "missing":
                    del metadata["version_allocation"]
                elif mutation == "digest":
                    metadata["version_allocation"]["history_sha256"] = "0" * 64
                else:
                    metadata["version"] = "0.6.25-staging.cli.sha" + SOURCE[:12]
                path.write_text(json.dumps(metadata))
                with self.assertRaises(ValueError), patch.object(PUBLISHER.SHARED, "run") as execute:
                    PUBLISHER.validate(output, SOURCE)
                execute.assert_not_called()

    def test_go_pin_integrity_guard_is_not_rebaselined_after_download(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = source_fixture(root)

            def execute(arguments, directory, environment, timeout=300):
                if arguments[1:] == ["env", "GOVERSION"]:
                    return PRODUCER.GO_VERSION
                if arguments[1:] == ["mod", "edit", "-json"]:
                    return '{"Replace":null}'
                if arguments[1:] == ["mod", "download", "all"]:
                    (source / "go.sum").write_text("changed")
                if arguments[1] == "build":
                    Path(arguments[arguments.index("-o") + 1]).write_bytes(b"fixture")
                return ""

            with patch.object(PRODUCER, "command", side_effect=execute), \
                    patch.object(PRODUCER, "verify_native_cli"):
                with self.assertRaisesRegex(ValueError, "Go module pins changed"):
                    PRODUCER.build(source, SOURCE, root / "work", root / "output", "go", allocation())

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
                    PRODUCER.build(source, SOURCE, root / "work", root / "out", "go", allocation())
            self.assertEqual(execute.call_count, 3)

    def test_stale_embed_manifest_fails_before_any_go_command(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = source_fixture(root)
            with patch.object(PRODUCER, "command", side_effect=ValueError("stale embed manifest")) as execute:
                with self.assertRaisesRegex(ValueError, "stale embed manifest"):
                    PRODUCER.build(source, SOURCE, root / "work", root / "out", "go", allocation())
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

    def test_candidate_toolchain_matches_reviewed_go_security_pin(self):
        repository = ROOT.parents[1]
        toolchain = next(line.split()[1] for line in (repository / "go.mod").read_text().splitlines()
                         if line.startswith("toolchain "))
        self.assertEqual(PRODUCER.GO_VERSION, toolchain)
        self.assertEqual(toolchain, "go1.26.10")
        workflow = (repository / ".github/workflows/cli-candidate.yml").read_text()
        self.assertIn("go-version: '1.26.10'", workflow)
        self.assertEqual(workflow.count("      - release/packaging/release-version.py"), 2)
        self.assertIn("GH_TOKEN: ${{ github.token }}", workflow)

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
                       "assets": [{"name": name, "state": "uploaded", **data} for name, data in assets.items()]}
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
        for state in ("absent", "partial", "draft", "published"):
            with self.subTest(state=state), tempfile.TemporaryDirectory() as temporary:
                output = build_fixture(Path(temporary))
                tag, assets = PUBLISHER.validate(output, SOURCE)
                release = {"id": 123, "tag_name": tag, "target_commitish": SOURCE, "prerelease": True,
                           "draft": state != "published", "html_url": "https://github.com/example/candidate",
                           "assets": [] if state in ("absent", "partial") else
                           [{"name": name, "state": "uploaded", **data} for name, data in assets.items()]}
                exists = state != "absent"
                calls = []

                def execute(arguments):
                    nonlocal exists
                    calls.append(arguments)
                    if "--method" in arguments:
                        self.assertFalse(exists)
                        self.assertIn("draft=true", arguments)
                        self.assertIn("prerelease=true", arguments)
                        self.assertIn("make_latest=false", arguments)
                        self.assertIn("target_commitish=" + SOURCE, arguments)
                        exists = True
                        return json.dumps({"id": 123, "tag_name": tag})
                    if arguments[1:3] == ["release", "upload"]:
                        name = Path(arguments[4]).name
                        self.assertFalse(any(item["name"] == name for item in release["assets"]))
                        release["assets"].append({"name": name, "state": "uploaded", **assets[name]})
                        return ""
                    if arguments[1:3] == ["release", "edit"]:
                        self.assertIn("--latest=false", arguments)
                        release["draft"] = False
                        return ""
                    if "graphql" in arguments:
                        reference = None if release["draft"] else {"target": {"__typename": "Commit", "oid": SOURCE}}
                        return json.dumps({"data": {"repository": {"ref": reference}}})
                    endpoint = next((item for item in arguments if item.startswith("repos/")), "")
                    self.assertIn("--jq", arguments)
                    if endpoint.endswith("/releases?per_page=100&page=1"):
                        return json.dumps({"count": int(exists), "matches":
                            [{"id": 123, "tag_name": tag}] if exists else []})
                    if endpoint.endswith("/releases/123"):
                        return json.dumps(release)
                    self.fail("Unexpected publication command")

                environment = {"GITHUB_REF": "refs/heads/dev", "GITHUB_EVENT_NAME": "push",
                               "GITHUB_REPOSITORY": PUBLISHER.REPOSITORY, "GITHUB_SHA": SOURCE}
                with patch.dict(os.environ, environment, clear=True), \
                        patch.object(PUBLISHER.SHARED, "run", side_effect=execute), patch("builtins.print"):
                    PUBLISHER.publish(output)
                self.assertEqual(sum("--method" in call for call in calls), int(state == "absent"))
                self.assertEqual(sum(call[1:3] == ["release", "edit"] for call in calls), int(state != "published"))
                self.assertEqual(sum(call[1:3] == ["release", "upload"] for call in calls),
                                 len(assets) if state in ("absent", "partial") else 0)
                self.assertFalse(any("--clobber" in call or "delete" in call for call in calls))
                if state == "absent":
                    creation = next(index for index, call in enumerate(calls) if "--method" in call)
                    self.assertIn("/releases/123", " ".join(calls[creation + 1]))


if __name__ == "__main__":
    unittest.main()
