"""Offline shared-store ownership and immutable metadata composition checks."""
import copy
import ctypes
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import types
import unittest
from unittest import mock


SCRIPT = Path(__file__).resolve().parents[1] / "compose-package-store.py"
spec = importlib.util.spec_from_file_location("compose_package_store", SCRIPT)
composer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(composer)


def artifact(name, kind="workflow", version="1.0.0", **metadata):
    prefix = "dockpipe-core-" if kind == "core" else f"dockpipe-{kind}-{name}-"
    return {"name": name, "version": version, "tarball": f"{prefix}{version}.tar.gz",
            "sha256": "a" * 64, "size_bytes": 100, **metadata}


def store(workflows=None, resolvers=None, **metadata):
    return {"schema": 1, "platform": "linux-amd64", "packages": {
        "workflows": workflows or [], "resolvers": resolvers or []}, **metadata}


class CompositionTests(unittest.TestCase):
    def setUp(self):
        self.base = store([artifact("other", hidden=True), artifact("owned")],
                          [artifact("shared", "resolver")], provenance={"release": "base"})
        self.base["packages"]["core"] = artifact("core", "core")
        self.candidate = store([artifact("owned", version="2.0.0", hidden=True, title="New")])
        self.owned = [["workflow", "owned"]]

    def test_preserves_core_unrelated_metadata_and_replaces_exact_identity(self):
        before = copy.deepcopy(self.base)
        output = composer.compose(self.base, self.candidate, self.owned)
        self.assertEqual(output["packages"]["core"], self.base["packages"]["core"])
        self.assertEqual(output["packages"]["resolvers"], self.base["packages"]["resolvers"])
        self.assertEqual(output["provenance"], {"release": "base"})
        self.assertEqual(output["packages"]["workflows"], [self.base["packages"]["workflows"][0], self.candidate["packages"]["workflows"][0]])
        output["packages"]["core"]["name"] = "changed"
        self.assertEqual(self.base, before)

    def test_deterministic_order_and_addition_with_trusted_ownership(self):
        self.candidate["packages"]["workflows"].append(artifact("added"))
        self.owned.append(["workflow", "added"])
        first = composer.compose(self.base, self.candidate, self.owned)
        self.base["packages"]["workflows"].reverse()
        self.candidate["packages"]["workflows"].reverse()
        self.assertEqual(first, composer.compose(self.base, self.candidate, list(reversed(self.owned))))
        self.assertEqual([entry["name"] for entry in first["packages"]["workflows"]], ["added", "other", "owned"])

    def test_forged_ownership_in_candidate_does_not_authorize_replacement(self):
        self.candidate["owned"] = [["workflow", "other"]]
        self.candidate["packages"]["workflows"].append(artifact("other", version="2.0.0"))
        with self.assertRaisesRegex(ValueError, "trusted ownership"):
            composer.compose(self.base, self.candidate, self.owned)

    def test_requires_complete_owned_set_and_rejects_duplicates(self):
        for owned in [self.owned + [["resolver", "missing"]], self.owned * 2, [["core", "core"]]]:
            with self.subTest(owned=owned), self.assertRaises(ValueError):
                composer.compose(self.base, self.candidate, owned)
        for document in [self.base, self.candidate]:
            document["packages"]["workflows"].append(copy.deepcopy(document["packages"]["workflows"][0]))
            with self.assertRaisesRegex(ValueError, "Duplicate"):
                composer.compose(self.base, self.candidate, self.owned)
            document["packages"]["workflows"].pop()

    def test_rejects_core_and_platform_or_schema_mismatch(self):
        for key, value in [("platform", "darwin-arm64"), ("platform", ""), ("schema", True), ("schema", 2)]:
            with self.subTest(key=key, value=value):
                candidate = copy.deepcopy(self.candidate)
                candidate[key] = value
                with self.assertRaises(ValueError):
                    composer.compose(self.base, candidate, self.owned)
        self.candidate["packages"]["core"] = None
        with self.assertRaisesRegex(ValueError, "core"):
            composer.compose(self.base, self.candidate, self.owned)

    def test_rejects_path_traversal_and_malformed_metadata(self):
        mutations = [("tarball", "../archive.tar.gz"), ("tarball", "/absolute.tar.gz"),
                     ("tarball", "https://host/archive.tar.gz"), ("tarball", "sub/archive.tar.gz"),
                     ("name", "../owned"), ("version", "1/2"), ("sha256", "g" * 64),
                     ("sha256", "a" * 63), ("size_bytes", -1), ("size_bytes", True),
                     ("size_bytes", 1.5), ("hidden", "true"), ("min_dockpipe_version", "bad")]
        for key, value in mutations:
            with self.subTest(key=key, value=value):
                candidate = copy.deepcopy(self.candidate)
                candidate["packages"]["workflows"][0][key] = value
                with self.assertRaises(ValueError):
                    composer.compose(self.base, candidate, self.owned)

    def test_preserves_complete_semantic_version_minimum_metadata(self):
        # Minimum-version normalization belongs to the existing runtime contract,
        # independently of the producer's exact dockpipe_version pin grammar.
        # The runtime minimum accepts an optional v prefix and SemVer suffixes.
        accepted = [
            "", "0.6.26", "0.6.26-0", "1.2.3-rc.1", "1.2.3-0.3.7",
            "1.2.3-x.7.z.92", "1.2.3-alpha-beta", "1.2.3--",
            "1.2.3+build.001", "1.2.3-rc.1+build.001", "v1.2.3-rc.1+build.5",
        ]
        for minimum in accepted:
            with self.subTest(minimum=minimum):
                candidate = copy.deepcopy(self.candidate)
                candidate["packages"]["workflows"][0]["min_dockpipe_version"] = minimum
                before = copy.deepcopy(candidate)
                output = composer.compose(self.base, candidate, self.owned)
                entry = next(entry for entry in output["packages"]["workflows"]
                             if entry["name"] == "owned")
                self.assertEqual(entry["min_dockpipe_version"], minimum)
                self.assertEqual(candidate, before)

    def test_rejects_malformed_or_injected_minimum_versions(self):
        rejected = [
            None, True, 1, [], {}, "latest", "dev", "0.6", "1", "1.2.3.4",
            ">=1.2.3", "^1.2.3", "01.2.3", "1.02.3", "1.2.03", "1.2.3-01",
            "1.2.3-alpha.01", "1.2.3-", "1.2.3+", "1.2.3-alpha..1",
            "1.2.3+build..1", "1.2.3+one+two", "V1.2.3", "vv1.2.3",
            "1.2.3/../../file", "1.2.3;echo bad", "$(command)",
            "1.2.3\ncommand", "1.2.3\r\n", " 1.2.3", "1.2.3 ", "1.2.3\x00",
            "1.2.3-\u03b1", "\u0661.2.3",
        ]
        for minimum in rejected:
            with self.subTest(minimum=minimum):
                candidate = copy.deepcopy(self.candidate)
                candidate["packages"]["workflows"][0]["min_dockpipe_version"] = minimum
                with self.assertRaisesRegex(ValueError, "minimum DockPipe version"):
                    composer.compose(self.base, candidate, self.owned)

    def test_rejects_reusing_archive_name_with_different_bytes(self):
        self.candidate = store([artifact("owned", sha256="b" * 64)])
        with self.assertRaisesRegex(ValueError, "immutable archive"):
            composer.compose(self.base, self.candidate, self.owned)
        self.candidate = store([artifact("owned")])
        composer.compose(self.base, self.candidate, self.owned)

    def test_rejects_tarball_collision_with_unrelated_identity(self):
        self.candidate["packages"]["workflows"][0]["tarball"] = self.base["packages"]["workflows"][0]["tarball"]
        with self.assertRaises(ValueError):
            composer.compose(self.base, self.candidate, self.owned)

        # Name/version delimiters can yield identical canonical filenames even
        # when both individual identities pass token and filename validation.
        base = store([artifact("owned-extra", version="1.0.0")])
        candidate = store([artifact("owned", version="extra-1.0.0")])
        with self.assertRaisesRegex(ValueError, "immutable archive"):
            composer.compose(base, candidate, self.owned)

    def test_legacy_baseline_size_can_be_absent_but_candidate_requires_it(self):
        del self.base["packages"]["workflows"][0]["size_bytes"]
        composer.compose(self.base, self.candidate, self.owned)
        del self.candidate["packages"]["workflows"][0]["size_bytes"]
        with self.assertRaisesRegex(ValueError, "size_bytes"):
            composer.compose(self.base, self.candidate, self.owned)

    def test_cli_fresh_output_and_bounded_input(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for name, data in [("base", self.base), ("candidate", self.candidate), ("owned", self.owned)]:
                (root / name).write_text(json.dumps(data))
            command = [sys.executable, str(SCRIPT), "--base", str(root / "base"),
                       "--candidate", str(root / "candidate"), "--owned", str(root / "owned"), "--out", str(root / "out")]
            first = subprocess.run(command, capture_output=True, text=True)
            self.assertEqual(first.returncode, 0, first.stderr)
            expected = (root / "out").read_bytes()
            second = subprocess.run(command, capture_output=True, text=True)
            self.assertNotEqual(second.returncode, 0)
            self.assertEqual((root / "out").read_bytes(), expected)
            (root / "base").write_bytes(b" " * (composer.MAX_MANIFEST_BYTES + 1))
            with self.assertRaisesRegex(ValueError, "8 MiB"):
                composer.read_json(root / "base")
            (root / "base").write_text('{"schema": 1, "schema": 2}')
            with self.assertRaisesRegex(ValueError, "Duplicate JSON"):
                composer.read_json(root / "base")


class ArchiveAssemblyTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.baseline_root = self.root / 'baseline'
        self.candidate_root = self.root / 'candidate'
        self.baseline_root.mkdir()
        self.candidate_root.mkdir()
        self.output = self.root / 'assembled'
        self.core = self.write_artifact(self.baseline_root, 'core', b'core bytes', kind='core')
        self.other = self.write_artifact(self.baseline_root, 'other', b'unrelated bytes')
        self.owned = self.write_artifact(self.candidate_root, 'owned', b'candidate bytes', version='2.0.0')
        self.base = store([self.other])
        self.base['packages']['core'] = self.core
        self.candidate = store([self.owned])
        self.budget = sum(entry['size_bytes'] for entry in [self.core, self.other, self.owned])

    def write_artifact(self, root, name, data, **metadata):
        entry = artifact(name, **metadata)
        entry['size_bytes'] = len(data)
        entry['sha256'] = hashlib.sha256(data).hexdigest()
        (root / entry['tarball']).write_bytes(data)
        return entry

    def assemble(self, budget=None):
        return composer.assemble(self.base, self.candidate, [['workflow', 'owned']],
                                 self.baseline_root, self.candidate_root, self.output,
                                 self.budget if budget is None else budget)

    def test_complete_store_retains_core_and_unrelated_verified_bytes(self):
        manifest = self.assemble()
        self.assertEqual(json.loads((self.output / 'packages-store-manifest.json').read_text()), manifest)
        expected = {'packages-store-manifest.json', self.core['tarball'], self.other['tarball'], self.owned['tarball']}
        self.assertEqual({path.name for path in self.output.iterdir()}, expected)
        self.assertEqual((self.output / self.other['tarball']).read_bytes(), b'unrelated bytes')

    def test_archive_budget_boundary_and_cleanup(self):
        with self.assertRaisesRegex(ValueError, 'byte budget'):
            self.assemble(self.budget - 1)
        self.assertFalse(self.output.exists())
        self.assertEqual(list(self.root.glob('.store-assembly-*')), [])
        self.assemble(self.budget)

    def test_bad_baseline_or_candidate_bytes_never_publish_output(self):
        for source, entry in [(self.baseline_root, self.other), (self.candidate_root, self.owned)]:
            with self.subTest(source=source):
                path = source / entry['tarball']
                original = path.read_bytes()
                path.write_bytes(b'x' * len(original))
                with self.assertRaisesRegex(ValueError, 'checksum'):
                    self.assemble()
                self.assertFalse(self.output.exists())
                path.write_bytes(original)

    def test_missing_or_linked_archive_never_publish_output(self):
        path = self.candidate_root / self.owned['tarball']
        original = path.read_bytes()
        path.unlink()
        with self.assertRaises(FileNotFoundError):
            self.assemble()
        try:
            path.symlink_to(self.baseline_root / self.other['tarball'])
        except OSError as error:
            if os.name == 'nt' and error.winerror == 1314:
                self.skipTest('Windows symbolic-link privilege is unavailable')
            raise
        with self.assertRaisesRegex(ValueError, 'link'):
            self.assemble()
        path.unlink()
        path.write_bytes(original)
        self.assemble()

    def test_existing_output_is_preserved(self):
        self.output.mkdir()
        marker = self.output / 'preserve'
        marker.write_bytes(b'user bytes')
        with self.assertRaisesRegex(ValueError, 'fresh output'):
            self.assemble()
        self.assertEqual(marker.read_bytes(), b'user bytes')


class SecureArchiveOpenTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.archive = self.root / 'archive'
        self.archive.write_bytes(b'verified regular archive')

    def test_regular_archive_is_readable_and_descriptor_is_closed(self):
        with composer.open_archive(self.archive) as reader:
            descriptor = reader.fileno()
            self.assertEqual(reader.read(), b'verified regular archive')
            self.assertFalse(os.get_inheritable(descriptor))
        with self.assertRaises(OSError):
            os.fstat(descriptor)

    def test_directory_is_rejected(self):
        with self.assertRaisesRegex(ValueError, 'regular'):
            with composer.open_archive(self.root):
                self.fail('Directory was exposed as an archive')

    def test_links_are_rejected_even_when_the_target_bytes_match(self):
        link = self.root / 'link'
        try:
            link.symlink_to(self.archive)
        except OSError as error:
            if os.name == 'nt' and error.winerror == 1314:
                self.skipTest('Windows symbolic-link privilege is unavailable')
            raise
        with self.assertRaisesRegex(ValueError, 'link'):
            with composer.open_archive(link):
                self.fail('Link was exposed as an archive')

    def test_descriptor_is_closed_when_post_open_validation_fails(self):
        original = composer.archive_descriptor
        opened = []
        def capture_descriptor(path):
            descriptor = original(path)
            opened.append(descriptor)
            return descriptor
        with mock.patch.object(composer, 'archive_descriptor', side_effect=capture_descriptor), \
                mock.patch.object(composer, 'reject_archive_links', side_effect=[None, ValueError('changed link')]):
            with self.assertRaisesRegex(ValueError, 'changed link'):
                with composer.open_archive(self.archive):
                    self.fail('Changed path was exposed')
        self.assertEqual(len(opened), 1)
        with self.assertRaises(OSError):
            os.fstat(opened[0])

    @unittest.skipUnless(os.name == 'posix', 'POSIX no-follow acquisition')
    def test_link_swap_during_open_fails_without_following(self):
        original = composer.archive_descriptor
        def replace_before_open(path):
            path.unlink()
            path.symlink_to(self.root / 'target')
            return original(path)
        (self.root / 'target').write_bytes(b'verified regular archive')
        with mock.patch.object(composer, 'archive_descriptor', side_effect=replace_before_open):
            with self.assertRaises(OSError):
                with composer.open_archive(self.archive):
                    self.fail('Racing symbolic link was followed')

    @unittest.skipUnless(os.name == 'posix', 'POSIX nonblocking acquisition')
    def test_fifo_is_rejected_without_waiting_for_a_writer(self):
        self.archive.unlink()
        os.mkfifo(self.archive)
        with self.assertRaisesRegex(ValueError, 'regular'):
            with composer.open_archive(self.archive):
                self.fail('FIFO was exposed as an archive')

    @unittest.skipUnless(os.name == 'posix', 'POSIX security capability check')
    def test_missing_no_follow_support_fails_closed(self):
        with mock.patch.object(os, 'O_NOFOLLOW'):
            del os.O_NOFOLLOW
            with self.assertRaisesRegex(ValueError, 'requires no-follow'):
                composer.archive_descriptor(self.archive)

    def test_unsupported_platform_fails_closed(self):
        with mock.patch.object(os, 'name', 'unsupported'):
            with self.assertRaisesRegex(ValueError, 'unsupported'):
                composer.archive_descriptor(self.archive)

    @unittest.skipUnless(os.name == 'nt', 'Windows native reparse-handle rejection')
    def test_native_windows_reparse_handle_is_rejected_without_path_precheck(self):
        link = self.root / 'native-link'
        try:
            link.symlink_to(self.archive)
        except OSError as error:
            if error.winerror == 1314:
                self.skipTest('Windows symbolic-link privilege is unavailable')
            raise
        with self.assertRaisesRegex(ValueError, 'reparse link'):
            composer.windows_archive_descriptor(link)

    @unittest.skipUnless(os.name == 'nt', 'Windows native sharing protection')
    def test_native_windows_handle_denies_write_and_delete_until_closed(self):
        with composer.open_archive(self.archive) as reader:
            with self.assertRaises(OSError):
                self.archive.write_bytes(b'changed')
            with self.assertRaises(OSError):
                self.archive.unlink()
            self.assertEqual(reader.read(), b'verified regular archive')
        self.archive.write_bytes(b'after close')
        self.archive.unlink()

    @unittest.skipUnless(os.name == 'nt', 'Windows native junction rejection')
    def test_native_windows_parent_junction_is_rejected(self):
        junction = self.root / 'junction'
        result = subprocess.run(['cmd', '/c', 'mklink', '/J', str(junction), str(self.root)],
                                capture_output=True, text=True, timeout=10)
        if result.returncode:
            self.skipTest('Windows directory junction creation is unavailable')
        try:
            with self.assertRaisesRegex(ValueError, 'reparse link'):
                with composer.open_archive(junction / self.archive.name):
                    self.fail('Parent junction was followed')
        finally:
            junction.rmdir()


class WindowsArchiveHandleTests(unittest.TestCase):
    """Exercise Win32 validation and ownership even on the POSIX development host."""
    def setUp(self):
        self.kernel = types.SimpleNamespace(
            CreateFileW=mock.Mock(return_value=0x100000001),
            GetFileType=mock.Mock(return_value=1),
            GetFileInformationByHandleEx=mock.Mock(side_effect=self.file_information),
            CloseHandle=mock.Mock(return_value=1),
        )
        self.attributes = 0x80
        self.reparse_tag = 0
        self.crt = types.SimpleNamespace(open_osfhandle=mock.Mock(return_value=42))
        self.patches = [
            mock.patch.object(ctypes, 'WinDLL', return_value=self.kernel, create=True),
            mock.patch.dict(sys.modules, {'msvcrt': self.crt}),
            mock.patch.object(os, 'O_BINARY', 0x8000, create=True),
            mock.patch.object(os, 'O_NOINHERIT', 0x80, create=True),
            mock.patch.object(ctypes, 'get_last_error', return_value=5, create=True),
            mock.patch.object(ctypes, 'WinError', return_value=OSError('Win32 error'), create=True),
        ]
        for patch in self.patches:
            patch.start()
            self.addCleanup(patch.stop)

    def file_information(self, handle, information_class, pointer, size):
        self.assertEqual(information_class, 9)
        self.assertEqual(size, 8)
        pointer._obj.attributes = self.attributes
        pointer._obj.reparse_tag = self.reparse_tag
        return 1

    def test_secure_flags_and_full_width_handle_transfer(self):
        self.assertEqual(composer.windows_archive_descriptor('archive'), 42)
        args = self.kernel.CreateFileW.call_args.args
        self.assertEqual(args[1:3], (0x80000000, 0x1))
        self.assertEqual(args[4:6], (3, 0x00200000 | 0x02000000))
        self.assertIs(self.kernel.CreateFileW.restype, ctypes.c_void_p)
        self.crt.open_osfhandle.assert_called_once_with(0x100000001, os.O_RDONLY | os.O_BINARY | os.O_NOINHERIT)
        self.kernel.CloseHandle.assert_not_called()

    def test_non_disk_directory_device_and_reparse_handles_are_closed(self):
        scenarios = [(2, 0x80, 0), (1, 0x10, 0), (1, 0x40, 0),
                     (1, 0x400, 0), (1, 0x80, 0xA000000C)]
        for file_type, attributes, tag in scenarios:
            with self.subTest(file_type=file_type, attributes=attributes, tag=tag):
                self.kernel.GetFileType.return_value = file_type
                self.attributes = attributes
                self.reparse_tag = tag
                self.kernel.CloseHandle.reset_mock()
                with self.assertRaises(ValueError):
                    composer.windows_archive_descriptor('archive')
                self.kernel.CloseHandle.assert_called_once_with(0x100000001)
                self.crt.open_osfhandle.assert_not_called()

    def test_attribute_query_and_crt_transfer_failures_close_native_handle(self):
        self.kernel.GetFileInformationByHandleEx.side_effect = None
        self.kernel.GetFileInformationByHandleEx.return_value = 0
        with self.assertRaisesRegex(OSError, 'Win32'):
            composer.windows_archive_descriptor('archive')
        self.kernel.CloseHandle.assert_called_once_with(0x100000001)
        self.kernel.CloseHandle.reset_mock()
        self.kernel.GetFileInformationByHandleEx.side_effect = self.file_information
        self.crt.open_osfhandle.side_effect = OSError('CRT transfer failed')
        with self.assertRaisesRegex(OSError, 'CRT transfer'):
            composer.windows_archive_descriptor('archive')
        self.kernel.CloseHandle.assert_called_once_with(0x100000001)

    def test_invalid_native_handle_is_never_transferred_or_closed(self):
        self.kernel.CreateFileW.return_value = ctypes.c_void_p(-1).value
        with self.assertRaisesRegex(OSError, 'Win32'):
            composer.windows_archive_descriptor('archive')
        self.kernel.CloseHandle.assert_not_called()
        self.crt.open_osfhandle.assert_not_called()

    def test_missing_windows_api_fails_closed(self):
        del self.kernel.GetFileInformationByHandleEx
        with self.assertRaisesRegex(ValueError, 'secure file API'):
            composer.windows_archive_descriptor('archive')
        self.kernel.CreateFileW.assert_not_called()


if __name__ == "__main__":
    unittest.main()
