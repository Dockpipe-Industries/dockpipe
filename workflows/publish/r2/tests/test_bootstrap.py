#!/usr/bin/env python3
"""Offline release-lock rejection and exact package/SDK installation checks."""
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import tarfile
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location("r2_bootstrap", Path(__file__).resolve().parents[1] / "assets/scripts/bootstrap.py")
bootstrap = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(bootstrap)


class BootstrapTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        (self.root / "dockpipe.config.json").write_text(json.dumps({"packages": {"sources": [{"kind": "store", "path": bootstrap.PROJECT_STORE}]}}))
        self.package = self.root / "package"
        self.package.mkdir()
        (self.package / "package.yml").write_text("version: 0.1.1\n")
        (self.package / "config.yml").write_text("name: dockpipe.cloudflare.r2bulk\nsteps: []\n")
        requirements = self.package / "requirements.lock"
        requirements.write_text("qualified requirements fixture\n")
        self.lock = {"schema": 1, "package": bootstrap.PACKAGE_NAME, "version": "0.1.1",
                     "manifest_url": "https://github.com/Dockpipe-Industries/dockpipe-packages/releases/download/r2bulk-0.1.1/packages-store-manifest.json",
                     "manifest_sha256": "c" * 64,
                     "archive_sha256": "a" * 64,
                     "requirements_lock_sha256": hashlib.sha256(requirements.read_bytes()).hexdigest()}
        self.lock_path = self.root / "lock.json"
        self.lock_path.write_text(json.dumps(self.lock))

    def test_rejects_unpinned_or_unofficial_release_before_install(self):
        cases = (("version", "latest"), ("archive_sha256", ""),
                 ("requirements_lock_sha256", "bad"), ("manifest_sha256", "bad"),
                 ("manifest_url", "https://example.invalid/catalog.json"),
                 ("manifest_url", "https://github.com/Dockpipe-Industries/dockpipe-packages/releases/download/latest/packages-store-manifest.json"))
        for field, value in cases:
            self.lock_path.write_text(json.dumps(dict(self.lock, **{field: value})))
            with self.subTest(field=field, value=value), patch.object(bootstrap, "fetch_public") as command:
                with self.assertRaises(ValueError):
                    bootstrap.install(self.lock_path, self.root / "runtime", Path("/cli"), self.root)
                command.assert_not_called()

    def test_exact_archive_and_hash_locked_requirements_are_used(self):
        calls = []

        def run(command, **kwargs):
            calls.append(command)
            if command[1:3] == ["-m", "venv"]:
                python = Path(command[3]) / "bin/python"
                python.parent.mkdir(parents=True)
                python.touch()

        with patch.object(bootstrap, "release_archive", return_value=self.package) as install, patch.object(bootstrap.subprocess, "run", side_effect=run):
            python = bootstrap.install(self.lock_path, self.root / "runtime", Path("/cli"), self.root)
            self.assertTrue(python.is_file())
            self.assertEqual(install.call_args.args, (self.lock, self.root / "runtime"))
            requirements = python.parent.parent / "requirements.lock"
            self.assertEqual(requirements.read_bytes(), (self.package / "requirements.lock").read_bytes())
            self.assertIn([str(python), "-m", "pip", "install", "--require-hashes", "--no-deps", "-r", str(requirements)], calls)
            calls.clear()
            bootstrap.install(self.lock_path, self.root / "runtime", Path("/cli"), self.root)
            self.assertEqual(calls[0], [str(python), "-m", "pip", "check"])
            self.assertTrue(calls[1][1].endswith("verify-sdk.py"))

    def test_returned_interpreter_preserves_virtual_environment_symlink(self):
        runtime = self.root / "runtime" / self.lock["requirements_lock_sha256"]
        python = runtime / "bin/python"
        python.parent.mkdir(parents=True)
        python.symlink_to(Path(bootstrap.sys.executable))
        (runtime / "qualified.json").write_text(json.dumps({"requirements_lock_sha256": self.lock["requirements_lock_sha256"]}))
        with patch.object(bootstrap, "release_archive", return_value=self.package), patch.object(bootstrap.subprocess, "run"):
            selected = bootstrap.install(self.lock_path, self.root / "runtime", Path("/cli"), self.root)
        self.assertEqual(selected, python)
        self.assertTrue(selected.is_symlink())

    def test_rejects_sdk_hash_and_package_version_mismatch(self):
        for path, replacement in ((self.package / "requirements.lock", "corrupted"), (self.package / "package.yml", "version: 0.1.2\n")):
            original = path.read_text()
            path.write_text(replacement)
            with self.subTest(path=path), patch.object(bootstrap, "release_archive", return_value=self.package), patch.object(bootstrap.subprocess, "run") as run:
                with self.assertRaises(ValueError):
                    bootstrap.install(self.lock_path, self.root / "runtime", Path("/cli"), self.root)
                run.assert_not_called()
            path.write_text(original)

    def write_archive(self, members=None):
        archive = self.root / "package.tar.gz"
        if members is None:
            members = [(name, (self.package / name).read_bytes(), tarfile.REGTYPE)
                       for name in (*bootstrap.PACKAGE_FILES, "config.yml")]
        with tarfile.open(archive, "w:gz") as bundle:
            for name, data, member_type in members:
                member = tarfile.TarInfo("workflows/" + bootstrap.PACKAGE_NAME + "/" + name)
                member.type = member_type
                member.size = len(data) if member_type == tarfile.REGTYPE else 0
                bundle.addfile(member, io.BytesIO(data) if member.isfile() else None)
        self.lock["archive_sha256"] = hashlib.sha256(archive.read_bytes()).hexdigest()
        self.lock_path.write_text(json.dumps(self.lock))
        return archive

    def test_real_archive_install_uses_only_bounded_descriptor_and_runtime_sdk_lock(self):
        archive = self.write_archive()

        def run(command, **kwargs):
            if command[1:3] == ["-m", "venv"]:
                python = Path(command[3]) / "bin/python"
                python.parent.mkdir(parents=True)
                python.touch()

        with patch.object(bootstrap, "release_archive", return_value=archive), patch.object(bootstrap.subprocess, "run", side_effect=run):
            selected = bootstrap.install(self.lock_path, self.root / "runtime", Path("/cli"), self.root)
        self.assertTrue(selected.is_file())
        self.assertEqual((selected.parent.parent / "requirements.lock").read_bytes(),
                         (self.package / "requirements.lock").read_bytes())
        self.assertFalse((selected.parent.parent / "package.yml").exists())
        self.assertFalse((self.root / "workflows").exists())

    def test_archive_hash_version_and_sdk_hash_mismatch_fail_before_runtime_commands(self):
        for field, replacement in (("archive_sha256", "b" * 64), ("version", "0.1.2"),
                                   ("requirements_lock_sha256", "b" * 64)):
            archive = self.write_archive()
            original = self.lock[field]
            self.lock_path.write_text(json.dumps(dict(self.lock, **{field: replacement})))
            with self.subTest(field=field), patch.object(bootstrap, "release_archive", return_value=archive), patch.object(bootstrap.subprocess, "run") as run:
                with self.assertRaises(ValueError):
                    bootstrap.install(self.lock_path, self.root / "runtime", Path("/cli"), self.root)
                run.assert_not_called()
            self.lock[field] = original

    def test_archive_rejects_missing_duplicate_link_and_oversized_required_members(self):
        descriptor = ("package.yml", b"version: 0.1.1\n", tarfile.REGTYPE)
        requirements = ("requirements.lock", (self.package / "requirements.lock").read_bytes(), tarfile.REGTYPE)
        cases = [
            [descriptor],
            [descriptor, requirements, requirements],
            [descriptor, ("requirements.lock", b"", tarfile.SYMTYPE)],
            [("package.yml", b"x" * (bootstrap.PACKAGE_FILES["package.yml"] + 1), tarfile.REGTYPE), requirements],
            [descriptor, ("../requirements.lock", requirements[1], tarfile.REGTYPE)],
        ]
        for members in cases:
            archive = self.write_archive(members)
            with self.subTest(members=[member[0] for member in members]), patch.object(bootstrap, "release_archive", return_value=archive), patch.object(bootstrap.subprocess, "run") as run:
                with self.assertRaises(ValueError):
                    bootstrap.install(self.lock_path, self.root / "runtime", Path("/cli"), self.root)
                run.assert_not_called()

    def test_missing_venv_support_has_explicit_dependency_error(self):
        with patch.object(bootstrap, "release_archive", return_value=self.package), patch.object(bootstrap.importlib.util, "find_spec", return_value=None), patch.object(bootstrap.subprocess, "run") as run:
            with self.assertRaisesRegex(RuntimeError, "venv and ensurepip"):
                bootstrap.install(self.lock_path, self.root / "runtime", Path("/cli"), self.root)
            run.assert_not_called()

    def public_fixture(self):
        archive = self.write_archive()
        basename = "dockpipe-workflow-" + bootstrap.PACKAGE_NAME + "-0.1.1.tar.gz"
        entry = {"name": bootstrap.PACKAGE_NAME, "version": "0.1.1", "tarball": basename,
                 "sha256": self.lock["archive_sha256"]}
        manifest = {"schema": 1, "packages": {"workflows": [entry]}}
        manifest_bytes = json.dumps(manifest).encode()
        self.lock["manifest_sha256"] = hashlib.sha256(manifest_bytes).hexdigest()
        self.lock_path.write_text(json.dumps(self.lock))
        return manifest, manifest_bytes, archive.read_bytes(), basename

    def test_declared_public_archive_is_verified_cached_and_reused_without_replacement(self):
        manifest, manifest_bytes, archive_bytes, basename = self.public_fixture()
        runtime = self.root / "public-runtime"
        with patch.object(bootstrap, "fetch_public", side_effect=[manifest_bytes, archive_bytes]) as fetch:
            archive = bootstrap.release_archive(self.lock, runtime)
        self.assertEqual(archive, runtime / "package-store/workflows" / basename)
        self.assertEqual(archive.read_bytes(), archive_bytes)
        self.assertEqual(fetch.call_args_list[1].args,
                         (self.lock["manifest_url"].rsplit("/", 1)[0] + "/" + basename, bootstrap.ARCHIVE_LIMIT))
        with patch.object(bootstrap, "fetch_public", return_value=manifest_bytes) as fetch:
            self.assertEqual(bootstrap.release_archive(self.lock, runtime), archive)
            self.assertEqual(fetch.call_count, 1)
        archive.write_bytes(b"preexisting conflicting bytes")
        with patch.object(bootstrap, "fetch_public", return_value=manifest_bytes) as fetch:
            with self.assertRaisesRegex(ValueError, "archive differs"):
                bootstrap.release_archive(self.lock, runtime)
            self.assertEqual(fetch.call_count, 1)
        self.assertEqual(archive.read_bytes(), b"preexisting conflicting bytes")

    def test_manifest_and_archive_hashes_are_required_before_store_write(self):
        manifest, manifest_bytes, archive_bytes, basename = self.public_fixture()
        runtime = self.root / "public-runtime"
        with patch.object(bootstrap, "fetch_public", return_value=manifest_bytes + b" "):
            with self.assertRaisesRegex(ValueError, "manifest differs"):
                bootstrap.release_archive(self.lock, runtime)
        with patch.object(bootstrap, "fetch_public", side_effect=[manifest_bytes, archive_bytes + b"corrupt"]):
            with self.assertRaisesRegex(ValueError, "archive differs"):
                bootstrap.release_archive(self.lock, runtime)
        self.assertFalse((runtime / "package-store/workflows" / basename).exists())

    def test_manifest_rejects_wrong_identity_extra_packages_and_noncanonical_archive(self):
        manifest, manifest_bytes, archive_bytes, basename = self.public_fixture()
        entry = manifest["packages"]["workflows"][0]
        cases = [
            {"workflows": [dict(entry, name="other")]},
            {"workflows": [dict(entry, version="0.1.2")]},
            {"workflows": [dict(entry, sha256="d" * 64)]},
            {"workflows": [dict(entry, tarball="../" + basename)]},
            {"workflows": [entry, entry]},
            {"workflows": [entry], "resolvers": [entry]},
        ]
        for packages in cases:
            payload = json.dumps({"schema": 1, "packages": packages}).encode()
            lock = dict(self.lock, manifest_sha256=hashlib.sha256(payload).hexdigest())
            with self.subTest(packages=packages), patch.object(bootstrap, "fetch_public", return_value=payload) as fetch:
                with self.assertRaises(ValueError):
                    bootstrap.release_archive(lock, self.root / "public-runtime")
                self.assertEqual(fetch.call_count, 1)

    def test_public_transport_rejects_bad_redirect_hosts_schemes_credentials_and_fourth_redirect(self):
        invalid = ["http://github.com/file", "https://example.invalid/file",
                   "https://github.com.evil.invalid/file", "https://token@github.com/file",
                   "https://github.com:8443/file"]
        request = bootstrap.Request(self.lock["manifest_url"])
        for url in invalid:
            with self.subTest(url=url), self.assertRaises(ValueError):
                bootstrap.PublicReleaseRedirects().redirect_request(request, None, 302, "", {}, url)
        redirects = bootstrap.PublicReleaseRedirects()
        for host in ("github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com"):
            redirected = redirects.redirect_request(request, None, 302, "", {}, "https://" + host + "/file?signature=public")
            self.assertNotIn("Authorization", redirected.headers)
        with self.assertRaisesRegex(ValueError, "three redirects"):
            redirects.redirect_request(request, None, 302, "", {}, "https://github.com/file")

    def test_public_fetch_has_timeout_byte_limit_and_no_authorization_headers(self):
        response = io.BytesIO(b"too many bytes")
        response.geturl = lambda: self.lock["manifest_url"]
        with patch.object(bootstrap, "build_opener") as opener:
            opener.return_value.open.return_value = response
            with self.assertRaisesRegex(ValueError, "byte limit"):
                bootstrap.fetch_public(self.lock["manifest_url"], 4)
            request = opener.return_value.open.call_args.args[0]
            self.assertNotIn("Authorization", request.headers)
            self.assertEqual(opener.return_value.open.call_args.kwargs, {"timeout": 30})

    def test_bootstrap_exports_only_venv_path_for_declared_project_store(self):
        binary = self.root / "dockpipe"
        binary.touch()
        runtime = self.root / "runtime"
        selected = runtime / "sdk/bin/python"
        outputs = self.root / "bootstrap.env"
        environment = {"DOCKPIPE_BIN": str(binary), "DOCKPIPE_STEP_OUTPUTS_FILE": str(outputs)}
        with patch.dict(bootstrap.os.environ, environment), patch.object(bootstrap.subprocess, "check_output", return_value=str(runtime)), patch.object(bootstrap, "install", return_value=selected):
            bootstrap.main()
        values = dict(line.split("=", 1) for line in outputs.read_text().splitlines())
        self.assertEqual(values["R2_BULK_PYTHON"], str(selected))
        self.assertNotIn("DOCKPIPE_PACKAGES_ROOT", values)

    def test_declared_project_store_materialization_reuses_only_matching_inventory(self):
        archive = self.write_archive()
        store = bootstrap.declared_store(self.root)
        destination = bootstrap.materialize_archive(archive, store, self.lock["archive_sha256"])
        self.assertEqual(destination, self.root / bootstrap.PROJECT_STORE / "workflows" / bootstrap.PACKAGE_NAME)
        self.assertEqual((destination / "config.yml").read_bytes(), (self.package / "config.yml").read_bytes())
        receipt = json.loads((destination / bootstrap.MATERIALIZATION_RECEIPT).read_text())
        self.assertEqual(receipt["archive_sha256"], self.lock["archive_sha256"])
        self.assertEqual(bootstrap.materialize_archive(archive, store, self.lock["archive_sha256"]), destination)
        (destination / "config.yml").write_text("conflicting cached bytes")
        with self.assertRaisesRegex(ValueError, "Conflicting materialized"):
            bootstrap.materialize_archive(archive, store, self.lock["archive_sha256"])
        self.assertEqual((destination / "config.yml").read_text(), "conflicting cached bytes")

    def test_project_store_must_be_declared_and_archive_members_are_strict(self):
        config = self.root / "dockpipe.config.json"
        config.write_text(json.dumps({"packages": {"sources": []}, "unrelated": "preserved"}))
        with self.assertRaisesRegex(ValueError, "Declare packages.sources"):
            bootstrap.declared_store(self.root)
        self.assertEqual(json.loads(config.read_text())["unrelated"], "preserved")
        cases = [
            [("../escape", b"bad", tarfile.REGTYPE)],
            [("link", b"", tarfile.SYMTYPE)],
            [("config.yml", b"one", tarfile.REGTYPE), ("config.yml", b"two", tarfile.REGTYPE)],
        ]
        for members in cases:
            archive = self.write_archive(members)
            with self.subTest(members=[member[0] for member in members]), self.assertRaises(ValueError):
                bootstrap.materialize_archive(archive, self.root / "isolated-store", self.lock["archive_sha256"])
        self.assertFalse((self.root / "escape").exists())


if __name__ == "__main__":
    unittest.main()
