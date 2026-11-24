"""SDK identity must come from the selected executable's exact module binding."""
from pathlib import Path
import sys
import json
import tempfile
import unittest


OWNER = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(OWNER / "drivers"))
import pipelang


class PipeLangIdentityTests(unittest.TestCase):
    module = "example.org/language"
    version = "v1.2.3"
    checksum = "h1:expected"
    row = "\tdep\texample.org/language\tv1.2.3\th1:expected\n"

    def observed(self, text):
        return pipelang.module_binding_observed(text, self.module, self.version, self.checksum)

    def test_exact_embedded_dependency_is_admitted(self):
        self.assertTrue(self.observed("fixture: go1.26\n" + self.row))

    def test_missing_duplicate_or_changed_identity_is_rejected(self):
        for text in ("", self.row + self.row, self.row.replace("v1.2.3", "v1.2.4"),
                     self.row.replace("h1:expected", "h1:different")):
            with self.subTest(text=text):
                self.assertFalse(self.observed(text))

    def test_local_or_versioned_replacement_cannot_claim_pinned_sdk(self):
        for replacement in ("\t=>\t/local/checkout\t(devel)\n", "\t=>\texample.org/fork\tv2.0.0\th1:other\n"):
            with self.subTest(replacement=replacement):
                self.assertFalse(self.observed(self.row + replacement))

    def test_unrelated_dependency_cannot_supply_missing_sdk_checksum(self):
        self.assertFalse(self.observed("\tdep\texample.org/language\tv1.2.3\n"
                                       "\tdep\texample.org/other\tv1.2.3\th1:expected\n"))

    def test_shared_qualification_routes_inspection_separately_from_language_operations(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            artifact = output / 'selected-cli'
            artifact.write_bytes(b'pinned generated artifact')
            go = output / 'selected-inspector'
            go.write_bytes(b'pinned generated inspector')
            workspace = output / 'workspace'
            workspace.mkdir()
            configuration = {'go': str(go), 'go_sha256': pipelang.digest_file(go),
                             'module': self.module, 'module_version': self.version, 'module_sum': self.checksum}
            inspected = []
            executed = []

            def failed_operation(tool, arguments, directory, environment, scope, name):
                logs = scope / name
                logs.mkdir()
                (logs / 'driver.stdout').write_text('{}')
                (logs / 'driver.stderr').write_text('generated unavailable fixture')
                (logs / 'process.json').write_text(json.dumps({'exit_code': 1, 'failure': ''}))
                return False

            def inspect(tool, arguments, *context):
                inspected.append((tool, arguments))
                return failed_operation(tool, arguments, *context)

            def execute(tool, arguments, *context):
                executed.append((tool, arguments))
                return failed_operation(tool, arguments, *context)

            observed, _, facts, _ = pipelang.qualify(
                '/app/bin/dockpipe', workspace, {}, output, configuration, pipelang.digest_file(artifact),
                execute=execute, inspect=inspect, inspection_artifact=artifact)
            self.assertEqual(inspected, [(go, ['version', '-m', str(artifact)])])
            self.assertEqual(len(executed), 7)
            self.assertTrue(all(tool == '/app/bin/dockpipe' and arguments[0] == 'pipelang'
                                for tool, arguments in executed))
            self.assertFalse(any(observed.values()))
            self.assertEqual(facts['inspected_executable_sha256'], pipelang.digest_file(artifact))

    def test_shared_qualification_rejects_an_inspection_mirror_with_another_digest(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            artifact = output / 'wrong-cli'
            artifact.write_bytes(b'another executable')
            with self.assertRaisesRegex(ValueError, 'Inspected CLI differs'):
                pipelang.qualify('/app/bin/dockpipe', output, {}, output, {'go': '/unused'},
                                 '0' * 64, inspection_artifact=artifact)

    def test_shared_case_selection_preserves_happy_and_failure_layers(self):
        happy = {'pinned-sdk-consumed': True}
        failure = {'expected-failure-observed': False}
        self.assertIs(pipelang.case_assertions({'id': 'sdk.pipelang', 'scenario': 'happy',
                                               'layer': 'installed_e2e'}, happy, failure), happy)
        self.assertIs(pipelang.case_assertions({'id': 'sdk.pipelang', 'scenario': 'failure',
                                               'layer': 'component'}, happy, failure), failure)
        self.assertIsNone(pipelang.case_assertions({'id': 'sdk.pipelang', 'scenario': 'failure',
                                                  'layer': 'installed_e2e'}, happy, failure))


if __name__ == "__main__":
    unittest.main()
