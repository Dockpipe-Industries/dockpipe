"""Offline shared-store ownership and immutable metadata composition checks."""
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


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
        path.symlink_to(self.baseline_root / self.other['tarball'])
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


if __name__ == "__main__":
    unittest.main()
