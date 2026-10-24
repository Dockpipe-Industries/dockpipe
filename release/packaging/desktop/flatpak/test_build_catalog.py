"""Compiled Flatpak catalog archive integrity boundaries."""

import hashlib
import importlib.util
import io
import subprocess
import shutil
import tempfile
import tarfile
import unittest
from pathlib import Path
from unittest.mock import patch


DIRECTORY = Path(__file__).resolve().parent


def load_module(name, filename):
    specification = importlib.util.spec_from_file_location(name, DIRECTORY / filename)
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


catalog = load_module("build_catalog", "build-catalog.py")
tools = load_module("prepare_package_tools", "prepare-package-tools.py")


class CompiledArchiveChecksumTests(unittest.TestCase):
    def test_compiled_archive_has_string_digest_with_separate_capacity(self):
        with tempfile.TemporaryDirectory() as temporary:
            archive = Path(temporary) / "compiled.tar.gz"
            archive.write_bytes(b"catalog archive")

            with patch.object(tools, "MAX_TOOL_ARCHIVE_BYTES", 3):
                with self.assertRaisesRegex(ValueError, "Optional tool archive exceeds capacity"):
                    tools.checksum(archive)
                with patch.object(catalog, "MAX_COMPILED_ARCHIVE_BYTES", 32):
                    digest = catalog.compiled_archive_checksum(archive)

            self.assertIsInstance(digest, str)
            self.assertEqual(digest, hashlib.sha256(b"catalog archive").hexdigest())

    def test_compiled_archive_rejects_growth_after_open(self):
        with tempfile.TemporaryDirectory() as temporary:
            archive = Path(temporary) / "compiled.tar.gz"
            archive.write_bytes(b"abc")

            original_fdopen = catalog.os.fdopen

            class GrowingArchive:
                def __init__(self, opened):
                    self.opened = opened
                    self.first_read = True

                def __enter__(self):
                    return self

                def __exit__(self, exception_type, exception, traceback):
                    self.opened.close()

                def fileno(self):
                    return self.opened.fileno()

                def read(self, size):
                    chunk = self.opened.read(size)
                    if self.first_read:
                        self.first_read = False
                        with archive.open("ab") as writer:
                            writer.write(b"d")
                    return chunk

            def open_growing_archive(descriptor, mode):
                return GrowingArchive(original_fdopen(descriptor, mode))

            with patch.object(catalog, "MAX_COMPILED_ARCHIVE_BYTES", 3):
                with patch.object(catalog.os, "fdopen", side_effect=open_growing_archive):
                    with self.assertRaisesRegex(ValueError, "Compiled package archive exceeds capacity"):
                        catalog.compiled_archive_checksum(archive)

    def test_compiled_archive_rejects_symlink(self):
        with tempfile.TemporaryDirectory() as temporary:
            archive = Path(temporary) / "compiled.tar.gz"
            archive.write_bytes(b"archive")
            linked = Path(temporary) / "linked.tar.gz"
            linked.symlink_to(archive)

            with self.assertRaises(OSError):
                catalog.compiled_archive_checksum(linked)


class PinnedDependencyTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.core = self.root / "core"
        self.core.mkdir()
        self.archive("core", "core", "name: core\n")

    def archive(self, kind, name, manifest, nested=None):
        directory = self.core if kind == "core" else self.root
        filename = directory / f"dockpipe-{kind}-{name}-0.6.1.tar.gz"
        prefix = "core" if kind == "core" else f"{kind}s/{name}"
        members = [(f"{prefix}/package.yml", manifest)]
        if nested is not None:
            members.append((f"{prefix}/tests/fixture/package.yml", nested))
        with tarfile.open(filename, "w:gz") as archive:
            for path, text in members:
                payload = text.encode()
                member = tarfile.TarInfo(path)
                member.size = len(payload)
                archive.addfile(member, io.BytesIO(payload))
        return filename

    def test_historical_pinned_workflow_reports_real_missing_packages_and_resolver(self):
        # Exact dependency fields from the pinned Linux 0.6.16 store's
        # secretstore-r2-publish-test 0.6.1 archive (SHA256 61475c36...).
        workflow = self.archive("workflow", "secretstore-r2-publish-test", """schema: 1
kind: workflow
name: secretstore-r2-publish-test
version: 0.6.1
depends: [cloud.storage, secrets]
requires_resolvers: [onepassword, r2]
""")
        onepassword = self.archive("resolver", "onepassword", "name: onepassword\n")

        with self.assertRaisesRegex(ValueError, "Incomplete pinned catalog") as failure:
            catalog.validate_package_dependencies([workflow, onepassword], self.core)

        message = str(failure.exception)
        self.assertIn("requires missing package cloud.storage", message)
        self.assertIn("requires missing package secrets", message)
        self.assertIn("requires missing resolver r2", message)
        self.assertNotIn("missing resolver onepassword", message)

    def test_present_dependencies_and_shipped_core_are_retained(self):
        workflow = self.archive("workflow", "consumer", """name: consumer
depends: [core, helper]
requires_resolvers: [backend]
""")
        helper = self.archive("workflow", "helper", "name: helper\n")
        backend = self.archive("resolver", "backend", "name: backend\n")

        catalog.validate_package_dependencies([workflow, helper, backend], self.core)
        with self.assertRaisesRegex(ValueError, "missing package helper"):
            catalog.validate_package_dependencies([workflow, backend], self.core)

    def test_nested_fixture_manifest_does_not_become_catalog_package_or_dependency(self):
        workflow = self.archive("workflow", "consumer", "name: consumer\n",
                                nested="name: fixture\ndepends: [not-installed]\n")
        catalog.validate_package_dependencies([workflow], self.core)

        other = self.archive("workflow", "other", "name: other\ndepends: [fixture]\n")
        with self.assertRaisesRegex(ValueError, "missing package fixture"):
            catalog.validate_package_dependencies([workflow, other], self.core)

    def test_required_resolver_cannot_be_satisfied_by_workflow_with_same_name(self):
        consumer = self.archive("workflow", "consumer", "name: consumer\nrequires_resolvers: [backend]\n")
        backend = self.archive("workflow", "backend", "name: backend\n")
        with self.assertRaisesRegex(ValueError, "missing resolver backend"):
            catalog.validate_package_dependencies([consumer, backend], self.core)

    def test_dependency_types_manifest_size_and_core_selection_are_bounded(self):
        invalid = self.archive("workflow", "consumer", "name: consumer\ndepends: arbitrary-string\n")
        with self.assertRaisesRegex(ValueError, "Invalid depends"):
            catalog.validate_package_dependencies([invalid], self.core)
        oversized = self.archive("workflow", "consumer", "name: consumer\n#" + "x" * 65536)
        with self.assertRaisesRegex(ValueError, "Invalid package manifest"):
            catalog.validate_package_dependencies([oversized], self.core)
        self.archive("core", "second", "name: second\n")
        with self.assertRaisesRegex(ValueError, "one selected compiled core"):
            catalog.validate_package_dependencies([], self.core)


class RuntimeDependencyTests(unittest.TestCase):
    def test_runtime_timeout_is_not_replaced_and_qualification_requires_it(self):
        execute = subprocess.run
        bash = shutil.which("bash")
        self.assertIsNotNone(bash)
        for available in (False, True):
            with self.subTest(available=available), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                package = root / "workflows" / "bounded-operation"
                package.mkdir(parents=True)
                metadata = {
                    "name": "bounded-operation",
                    "dependencies": {"host": [{"id": "deadline", "command": "timeout", "required": False}]},
                }
                (package / "config.yml").write_text(catalog.yaml.safe_dump(metadata))
                commands = catalog.adapt_metadata(package, "fixture")
                bundles, host_commands = catalog.copy_tools(
                    package, {"tools": [], "host_commands": []}, commands, root / "unused-tools")
                self.assertEqual(bundles, set())
                self.assertEqual(host_commands, set())
                self.assertEqual(list((package / "assets/tooling/bin").iterdir()), [])
                adapted = catalog.yaml.safe_load((package / "config.yml").read_text())
                self.assertEqual(adapted["dependencies"], metadata["dependencies"])
                runtime_bin = root / "runtime-bin"
                runtime_bin.mkdir()
                if available:
                    executable = runtime_bin / "timeout"
                    executable.write_text("#!/bin/sh\nexit 0\n")
                    executable.chmod(0o755)

                def invoke_runtime(arguments, check, timeout):
                    self.assertIn("--runtime", arguments)
                    workflow_path = Path(arguments[arguments.index("--workflow-file") + 1])
                    workflow = catalog.yaml.safe_load(workflow_path.read_text())
                    script = package / workflow["steps"][0]["run"]
                    # Emulate only Flatpak's marker. The generated script's actual
                    # command lookup must fail with no timeout and pass with it.
                    wrapper = 'test() { [[ "$1" == -f && "$2" == /.flatpak-info ]]; }; source "$1"'
                    result = execute([bash, "-c", wrapper, "qualification", str(script)],
                                     env={"PATH": str(runtime_bin)}, capture_output=True, timeout=5)
                    if check and result.returncode:
                        raise subprocess.CalledProcessError(result.returncode, arguments)
                    return result

                with patch.object(catalog.subprocess, "run", side_effect=invoke_runtime):
                    if available:
                        catalog.qualify(root / "app", root / "stage", package,
                                        commands, bundles, host_commands)
                    else:
                        with self.assertRaises(subprocess.CalledProcessError):
                            catalog.qualify(root / "app", root / "stage", package,
                                            commands, bundles, host_commands)

    def test_unknown_dependency_still_has_no_implicit_host_fallback(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            with self.assertRaisesRegex(ValueError, "Dependency has no Flatpak implementation"):
                catalog.copy_tools(root, {"tools": [], "host_commands": []},
                                   {"undeclared-external-tool"}, root / "tools")


if __name__ == "__main__":
    unittest.main()
