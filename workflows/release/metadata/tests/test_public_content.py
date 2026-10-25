import copy
import importlib.util
import json
import io
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).resolve().parents[4] / 'workflows/release/metadata/assets/scripts/check-public-content.py'
spec = importlib.util.spec_from_file_location('public_content', SCRIPT)
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)


# Compose reviewed public names so this scanner's own lexical source gate
# does not confuse these Qt names with operator environment identifiers.
QT_QUICK_CONTROLS_PLUGIN = 'qtquickcontrols' + '2implplugin'
QT_QUICK_TEMPLATES_LIBRARY = 'libqtquicktemplates' + '2plugin'
QT_WAYLAND_TEXT_INPUT_SOURCE = 'qwaylandtextinputmanager' + 'v3'


class PublicContentTests(unittest.TestCase):
    def test_requires_operator_targets_and_allows_placeholders(self):
        examples = [
            'R2_BUCKET: ""',
            '"environment_id": "your-production-environment-id"',
            'R2_ENDPOINT_URL=https://your-account-id.r2.cloudflarestorage.com',
            'repository: https://github.com/Dockpipe-Industries/dockpipe',
        ]
        for example in examples:
            with self.subTest(example=example):
                self.assertEqual(checker.findings('workflows/example/config.yml', example), [])
        for private in [
            'R2_BUCKET: "dockpipe"',
            'REPOSITORY = "Dockpipe-Industries/dockpipe"',
            'https://' + 'a' * 32 + '.r2.cloudflarestorage.com',
            '"environment_id": "' + 'a2' * 13 + '"',
            '/home/' + 'private-operator/project',
        ]:
            with self.subTest(private=private):
                self.assertTrue(checker.findings('workflows/example/config.yml', private))

    def test_optional_store_absence_empty_and_empty_manifest_are_valid(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / 'stores'
            self.assertEqual(checker.optional_store_archives(root), [])
            root.mkdir()
            self.assertEqual(checker.optional_store_archives(root), [])
            store = root / 'linux-amd64'
            store.mkdir()
            (store / 'packages-store-manifest.json').write_text(json.dumps({
                'schema': 1, 'packages': {'resolvers': [], 'workflows': []}}))
            self.assertEqual(checker.optional_store_archives(root), [])
            (root / 'unlisted.txt').write_text('unexpected')
            with self.assertRaisesRegex(ValueError, 'complete flat manifests'):
                checker.optional_store_archives(root)

    def test_optional_store_still_checks_source_and_rejects_corrupt_selected_bytes(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            with patch.object(checker, 'tracked_entries', return_value=[
                    ('workflows/example/config.yml', b'R2_BUCKET: "dockpipe"')]), \
                    patch('sys.argv', ['check', '--root', str(root), '--store-root', str(root / 'absent')]):
                self.assertEqual(checker.main(), 1)
            store = root / 'selected'
            store.mkdir()
            name = 'dockpipe-resolver-example-1.0.0.tar.gz'
            (store / name).write_bytes(b'corrupt selected archive')
            (store / 'packages-store-manifest.json').write_text(json.dumps({
                'schema': 1, 'packages': {'resolvers': [{
                    'name': 'example', 'version': '1.0.0', 'tarball': name, 'sha256': '0' * 64}],
                    'workflows': []}}))
            with self.assertRaises(ValueError):
                checker.optional_store_archives(store)

    def test_scans_package_contents_without_extracting(self):
        with tempfile.TemporaryDirectory() as temporary:
            archive = Path(temporary) / 'example.tar.gz'
            payload = b'R2_BUCKET: "dockpipe"\n'
            with tarfile.open(archive, 'w:gz') as stream:
                member = tarfile.TarInfo('workflows/example/config.yml')
                member.size = len(payload)
                stream.addfile(member, io.BytesIO(payload))
            entries = list(checker.archive_entries(archive))
            self.assertEqual(len(entries), 1)
            self.assertTrue(checker.findings(entries[0][0], entries[0][1].decode()))
            self.assertFalse((Path(temporary) / 'workflows').exists())

    def test_archive_vendor_scope_preserves_authored_configuration_checks(self):
        root = 'resolvers/example/assets/tooling/bin/'
        vendor_files = [
            root + '.deps/aws/aws/dist/awscli/examples/example.rst',
            root + '.deps/node/runtime/share/doc/example.md',
            root + '.deps/go/go/test/example.go',
            root + '.deps/wrangler/node_modules/example/index.js',
            root + '.deps/azure/azure/cli/example.py',
        ]
        authored_files = [
            root + 'aws',
            root + '.deps/aws/source.json',
            root + '.deps/azure/source.json',
            root + '.deps/wrangler/package-lock.json',
            root + '.deps/unknown/config.json',
            'workflows/example/config.yml',
            'workflows/example/assets/.deps/go/go/config.json',
            root + '.deps/go/go/../../config.json',
        ]
        payload = b'"environment_id": "' + b'a2' * 13 + b'"\n'
        with tempfile.TemporaryDirectory() as temporary:
            archive = Path(temporary) / 'bundled-tools.tar.gz'
            with tarfile.open(archive, 'w:gz') as stream:
                for name in vendor_files + authored_files:
                    member = tarfile.TarInfo(name)
                    member.size = len(payload)
                    stream.addfile(member, io.BytesIO(payload))
            entries = list(checker.archive_entries(archive))
            self.assertEqual([path for path, _ in entries],
                             ['packages/' + path for path in authored_files])
            for path, data in entries:
                self.assertTrue(checker.findings(path, data.decode()), path)

        # A tracked file cannot gain an exemption by imitating a vendor path.
        for path in vendor_files:
            self.assertTrue(checker.findings('packages/' + path, payload.decode()), path)


class QtSpdxContentTests(unittest.TestCase):
    def document(self, module):
        return {
            "spdxVersion": "SPDX-2.3",
            "SPDXID": "SPDXRef-DOCUMENT",
            "dataLicense": "CC0-1.0",
            "name": module + "-6.10.3",
            "documentNamespace": "https://qt.io/spdxdocs/" + module + "-upstream-build",
            "creationInfo": {"creators": ["Organization: TheQtCompany", "Tool: Qt Build System"]},
            "packages": [],
            "files": [],
            "relationships": [],
        }

    def path(self, module):
        return "packages/resolvers/cockpit/assets/licenses/qt/" + module + "-6.10.3.spdx.json"

    def findings(self, module, document, **options):
        return checker.findings(self.path(module), json.dumps(document, **options))

    def test_reviewed_component_fields_are_not_environment_ids(self):
        document = self.document("qtdeclarative")
        identifier = 'SPDXRef-Package-qtdeclarative-' + QT_QUICK_CONTROLS_PLUGIN + '-bdf0db835863'
        document["packages"] = [{
            "SPDXID": identifier,
            "name": QT_QUICK_CONTROLS_PLUGIN,
            "comment": '\nCMake target name: ' + QT_QUICK_CONTROLS_PLUGIN + '\n',
            "externalRefs": [{
                "referenceCategory": "PACKAGE_MANAGER",
                "referenceType": "purl",
                "referenceLocator": 'pkg:github/qt/qtdeclarative@3a714efe?library_name=' + QT_QUICK_CONTROLS_PLUGIN + '#src/quickcontrolsimpl',
            }],
        }]
        document["files"] = [{
            "SPDXID": 'SPDXRef-PackagedFile-qt-plugin-' + QT_QUICK_CONTROLS_PLUGIN + '-bdf0db835863',
            "fileName": './qml/QtQuick/Templates/' + QT_QUICK_TEMPLATES_LIBRARY + '.so',
        }]
        document["relationships"] = [{"spdxElementId": identifier, "relatedSpdxElement": identifier}]
        original = copy.deepcopy(document)
        for options in [{}, {"indent": 4}]:
            self.assertEqual(self.findings("qtdeclarative", document, **options), [])
        self.assertEqual(document, original)

        wayland = self.document("qtwayland")
        wayland["relationships"] = [{
            "relationshipType": "GENERATED_FROM",
            "comment": '/src_dir/qtwayland/src/compositor/extensions/' + QT_WAYLAND_TEXT_INPUT_SOURCE + '.' + extension,
        } for extension in ["cpp", "h"]]
        self.assertEqual(self.findings("qtwayland", wayland), [])
        wayland["relationships"][0]["relationshipType"] = "OTHER"
        self.assertTrue(self.findings("qtwayland", wayland))

    def root_build_package(self):
        return {
            "name": "qtbase",
            "SPDXID": "SPDXRef-Package-qtbase-18caa008d90e",
            "supplier": "Organization: TheQtCompany",
            "downloadLocation": "git://code.qt.io/qt/qtbase.git@7ddbc87d",
            "comment": "Built by CMake 3.30.5\nqtbase was configured with:\n    " + checker.QT_INSTALL_FLAG,
        }

    def test_only_exact_upstream_install_flag_in_root_build_comment_is_allowed(self):
        document = self.document("qtbase")
        document["packages"] = [self.root_build_package()]
        self.assertEqual(self.findings("qtbase", document), [])
        for suffix in ["-private", "/private", ' ' + '/home/' + 'private-operator/project', ' ' + '/home/' + 'qt/work/install']:
            changed = copy.deepcopy(document)
            changed["packages"][0]["comment"] += suffix
            with self.subTest(suffix=suffix):
                self.assertTrue(self.findings("qtbase", changed))
        for field, value in [("name", "another-package"), ("supplier", "Organization: AnotherVendor"),
                             ("SPDXID", 42), ("downloadLocation", "https://example.invalid/qtbase")]:
            changed = copy.deepcopy(document)
            changed["packages"][0][field] = value
            with self.subTest(field=field):
                self.assertTrue(self.findings("qtbase", changed))
        document["creationInfo"]["comment"] = document["packages"][0]["comment"]
        self.assertTrue(self.findings("qtbase", document))

    def test_other_matches_on_same_physical_line_remain_findings(self):
        document = self.document("qtdeclarative")
        document["packages"] = [{
            "SPDXID": 'SPDXRef-Package-' + QT_QUICK_CONTROLS_PLUGIN + '-bdf0db835863-' + "a2" * 13,
            "name": QT_QUICK_CONTROLS_PLUGIN,
            "environment_id": "a2" * 13,
            "account_id": "a" * 32,
            "endpoint": "https://" + "a" * 32 + ".r2.cloudflarestorage.com",
            "private_path": '/home/' + 'private-operator/project',
            "repository": "R2_BUCKET: dockpipe",
        }]
        problems = self.findings("qtdeclarative", document)
        self.assertEqual({line for line, _ in problems}, {1})
        self.assertEqual({reason for _, reason in problems}, {
            "personal home path; use a documented example path",
            "account-specific R2 endpoint; require operator configuration",
            "opaque environment identifier; use an explicit placeholder",
            "organization-specific deployment default",
        })

    def test_unknown_fields_and_unknown_component_names_are_not_exempt(self):
        for field, value in [("operator", QT_QUICK_CONTROLS_PLUGIN),
                             ("name", "a2" * 13),
                             ("comment", QT_QUICK_CONTROLS_PLUGIN),
                             ("fileName", './other/' + QT_QUICK_TEMPLATES_LIBRARY + '.so')]:
            document = self.document("qtdeclarative")
            document["packages"] = [{field: value}]
            with self.subTest(field=field, value=value):
                self.assertTrue(self.findings("qtdeclarative", document))

    def test_unadmitted_or_malformed_documents_fail_closed(self):
        original = self.document("qtdeclarative")
        original["packages"] = [{"name": QT_QUICK_CONTROLS_PLUGIN}]
        for field, value in [("spdxVersion", "SPDX-2.2"), ("dataLicense", "unknown"),
                             ("name", "qtbase-6.10.3"), ("documentNamespace", "https://example.invalid/spdx")]:
            changed = copy.deepcopy(original)
            changed[field] = value
            with self.subTest(field=field):
                self.assertTrue(self.findings("qtdeclarative", changed))
        changed = copy.deepcopy(original)
        changed["creationInfo"]["creators"] = ["Organization: AnotherVendor", "Tool: Qt Build System"]
        self.assertTrue(self.findings("qtdeclarative", changed))
        text = json.dumps(original)
        for path in ["packages/resolvers/cockpit/assets/licenses/qt/fake.spdx.json",
                     "packages/resolvers/cockpit/assets/licenses/qt/../qt/qtdeclarative-6.10.3.spdx.json",
                     "packages/other/qtdeclarative-6.10.3.spdx.json"]:
            self.assertTrue(checker.findings(path, text), path)
        self.assertTrue(checker.findings(self.path("qtdeclarative"), text[:-1]))
        nonstandard = text[:-1] + ', "unknown": NaN}'
        self.assertTrue(checker.findings(self.path("qtdeclarative"), nonstandard))
        duplicate = text[:-1] + ', "name": "qtdeclarative-6.10.3"}'
        self.assertTrue(checker.findings(self.path("qtdeclarative"), duplicate))

    def test_findings_keep_original_line_numbers(self):
        document = self.document("qtbase")
        document["packages"] = [self.root_build_package()]
        document["operator"] = '/home/' + 'private-operator/project'
        text = json.dumps(document, indent=4)
        expected = next(index for index, line in enumerate(text.splitlines(), 1) if '"operator"' in line)
        self.assertEqual(checker.findings(self.path("qtbase"), text),
                         [(expected, "personal home path; use a documented example path")])


    def test_resource_exhaustion_preserves_original_scan_and_diagnostics(self):
        document = self.document("qtdeclarative")
        document["packages"] = [{"name": QT_QUICK_CONTROLS_PLUGIN}]
        document["operator"] = '/home/' + 'private-operator/project'
        nested = {"value": "leaf"}
        for _ in range(8):
            nested = {"nested": nested}
        document["nested"] = nested
        document["wide"] = [0] * 100
        text = json.dumps(document, indent=4)
        original = checker.findings("untrusted.json", text)
        self.assertEqual(len(original), 2)
        for name, limit in [("MAX_QT_SPDX_CHARACTERS", len(text) - 1),
                            ("MAX_QT_SPDX_NODES", 50), ("MAX_QT_SPDX_DEPTH", 5)]:
            with self.subTest(limit=name), patch.object(checker, name, limit):
                self.assertEqual(checker.qt_spdx_scan_text(self.path("qtdeclarative"), text), text)
                self.assertEqual(checker.findings(self.path("qtdeclarative"), text), original)
