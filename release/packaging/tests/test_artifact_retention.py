"""Check retention against actual upload steps and all release entry contexts."""
import itertools
from pathlib import Path
import unittest

import yaml

ROOT = Path(__file__).resolve().parents[3]


class ArtifactRetentionTests(unittest.TestCase):
    def test_retention_is_short_only_for_staging_release_uploads(self):
        expected = {
            ('release.yml', 'build-unix'): 3,
            ('release.yml', 'build-msi'): 3,
            ('release.yml', 'assemble'): 7,
            ('flatpak.yml', 'desktop'): 3,
        }
        seen = set()
        for path in (ROOT / '.github/workflows').glob('*.yml'):
            workflow = yaml.load(path.read_text(), Loader=yaml.BaseLoader)
            for job_name, job in workflow.get('jobs', {}).items():
                for step in job.get('steps', []):
                    if not step.get('uses', '').startswith('actions/upload-artifact@'):
                        continue
                    inputs = step['with']
                    name = inputs['name']
                    targeted = name.startswith('release-') or name.startswith('dockpipe-release-prepared-')
                    key = (path.name, job_name)
                    if key not in expected or not targeted:
                        # Security reports, bootstrap, standalone qualification and dry runs
                        # must not acquire the conditional staging policy.
                        self.assertNotIn('inputs.staging', inputs.get('retention-days', ''))
                        continue
                    seen.add(key)
                    expression = inputs['retention-days'].removeprefix('${{').removesuffix('}}')
                    for staging, event, ref in itertools.product(
                        (True, False), ('push', 'pull_request', 'workflow_dispatch'),
                        ('refs/heads/staging', 'refs/heads/master', 'refs/heads/dev', 'refs/heads/js/dev', 'refs/tags/staging')
                    ):
                        evaluated = expression
                        for token, value in {'inputs.staging': staging, 'github.event_name': event, 'github.ref': ref}.items():
                            evaluated = evaluated.replace(token, repr(value))
                        actual = eval(evaluated.replace('&&', ' and ').replace('||', ' or '), {'__builtins__': {}}, {})
                        short = staging and event == 'push' and ref == 'refs/heads/staging'
                        self.assertEqual(actual, expected[key] if short else 0, (key, staging, event, ref))
        self.assertEqual(seen, set(expected))

    def test_flatpak_staging_input_is_explicit_and_defaults_to_false(self):
        release = yaml.load((ROOT / '.github/workflows/release.yml').read_text(), Loader=yaml.BaseLoader)
        flatpak = yaml.load((ROOT / '.github/workflows/flatpak.yml').read_text(), Loader=yaml.BaseLoader)
        self.assertEqual(release['jobs']['build-flatpak']['with']['staging'], '${{ inputs.staging == true }}')
        value = flatpak['on']['workflow_call']['inputs']['staging']
        self.assertEqual(value['type'], 'boolean')
        self.assertEqual(value['default'], 'false')


if __name__ == '__main__':
    unittest.main()
