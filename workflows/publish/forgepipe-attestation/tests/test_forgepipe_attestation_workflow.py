"""Keep manual SDK attestation privileges outside reusable release builds."""
from pathlib import Path
import unittest

import yaml




def workflow(name):
    path = Path(__file__).resolve().parents[4] / '.github/workflows' / name
    return yaml.load(path.read_text(), Loader=yaml.BaseLoader)


class ForgePipeAttestationWorkflowTests(unittest.TestCase):
    def test_manual_attestation_preserves_exact_event_tag_and_ref_gate(self):
        document = workflow('forgepipe-sdk-attestation.yml')
        self.assertEqual(set(document['on']), {'workflow_dispatch'})
        tag = document['on']['workflow_dispatch']['inputs']['forgepipe_sdk_tag']
        self.assertEqual(tag, {
            'description': 'Attest only the frozen, audited ForgePipe SDK prerelease selected by its committed lock',
            'required': 'false',
            'default': '',
            'type': 'string',
        })
        self.assertEqual(document['permissions'], {'contents': 'read'})
        self.assertEqual(set(document['jobs']), {'attest-forgepipe-sdk'})
        job = document['jobs']['attest-forgepipe-sdk']
        self.assertEqual(job['permissions'], {
            'contents': 'read',
            'id-token': 'write',
            'attestations': 'write',
        })
        expected = (
            "github.event_name == 'workflow_dispatch' && inputs.forgepipe_sdk_tag != '' "
            "&& (github.ref == 'refs/heads/js/dev' || github.ref == 'refs/heads/staging')"
        )
        self.assertEqual(job['if'], expected)
        for event in ('workflow_dispatch', 'push', 'pull_request'):
            for tag_value in ('', 'fixture-frozen-tag'):
                for ref in ('refs/heads/js/dev', 'refs/heads/staging', 'refs/heads/dev',
                            'refs/heads/master', 'refs/tags/staging'):
                    with self.subTest(event=event, tag=tag_value, ref=ref):
                        expression = job['if']
                        for name, value in {
                            'github.event_name': event,
                            'inputs.forgepipe_sdk_tag': tag_value,
                            'github.ref': ref,
                        }.items():
                            expression = expression.replace(name, repr(value))
                        expression = expression.replace('&&', ' and ').replace('||', ' or ')
                        allowed = eval(expression, {'__builtins__': {}}, {})
                        self.assertEqual(allowed, event == 'workflow_dispatch' and bool(tag_value)
                                         and ref in ('refs/heads/js/dev', 'refs/heads/staging'))

    def test_audit_still_precedes_pinned_attestation_without_release_publication(self):
        job = workflow('forgepipe-sdk-attestation.yml')['jobs']['attest-forgepipe-sdk']
        checkout, audit, attest = job['steps']
        self.assertEqual(checkout['uses'], 'actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1')
        self.assertEqual(checkout['with']['persist-credentials'], 'false')
        self.assertEqual(audit['env'], {'FORGEPIPE_SDK_TAG': '${{ inputs.forgepipe_sdk_tag }}'})
        self.assertIn('workflows/publish/forgepipe-attestation/config.yml', audit['run'])
        owner = yaml.safe_load((Path(__file__).resolve().parents[4] / 'workflows/publish/forgepipe-attestation/config.yml').read_text())
        self.assertIn('workflows/publish/forgepipe-attestation/assets/scripts/attest-forgepipe.py', owner['steps'][0]['cmd'])
        self.assertIn('--lock workflows/publish/forgepipe-attestation/assets/forgepipe-sdk.lock.json', audit['run'])
        self.assertIn('--exports workflows/publish/forgepipe-attestation/assets/forgepipe-public-exports.txt', audit['run'])
        self.assertIn('--tag "$FORGEPIPE_SDK_TAG"', audit['run'])
        self.assertEqual(attest['uses'],
                         'actions/attest-build-provenance@43d14bc2b83dec42d39ecae14e916627a18bb661')
        self.assertEqual(set(attest['with']), {'subject-path'})
        self.assertNotIn('environment', job)
        self.assertNotIn('gh release', yaml.dump(job))

    def test_reusable_release_requests_fit_the_staging_caller_without_attestation_write(self):
        caller = workflow('ci.yml')['jobs']['staging-release']
        self.assertEqual(caller['uses'], './.github/workflows/release.yml')
        self.assertEqual(caller['permissions'], {'contents': 'write', 'attestations': 'read'})
        release = workflow('release.yml')
        self.assertNotIn('forgepipe_sdk_tag', release['on']['workflow_dispatch']['inputs'])
        self.assertNotIn('attest-forgepipe-sdk', release['jobs'])
        levels = {'none': 0, 'read': 1, 'write': 2}
        for name, job in release['jobs'].items():
            requested = job.get('permissions', release['permissions'])
            for permission, access in requested.items():
                with self.subTest(job=name, permission=permission):
                    self.assertLessEqual(levels[access],
                                         levels[caller['permissions'].get(permission, 'none')])
            self.assertNotEqual(requested.get('attestations'), 'write', name)
            self.assertNotEqual(requested.get('id-token'), 'write', name)


if __name__ == '__main__':
    unittest.main()
