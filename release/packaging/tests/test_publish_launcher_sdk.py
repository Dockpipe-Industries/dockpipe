import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location(
    'publisher', Path(__file__).resolve().parents[1] / 'publish-launcher-sdk.py')
publisher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(publisher)


class CandidateBoundaryTests(unittest.TestCase):
    def test_production_and_manual_calls_fail_before_network(self):
        for ref, event in [('refs/heads/master', 'push'), ('refs/heads/dev', 'workflow_dispatch')]:
            environment = {
                'GITHUB_REPOSITORY': publisher.REPOSITORY,
                'GITHUB_REF': ref,
                'GITHUB_EVENT_NAME': event,
                'GITHUB_SHA': 'a' * 40,
            }
            with patch.dict(publisher.os.environ, environment, clear=True), \
                    patch.object(publisher, 'run') as run:
                with self.assertRaises(ValueError):
                    publisher.publish(Path('/not-read'))
                run.assert_not_called()

    def test_remote_mutation_is_rejected(self):
        assets = {'a': {'size': 2, 'digest': 'sha256:aaa'}}
        release = {
            'tag_name': 'candidate',
            'target_commitish': 'a' * 40,
            'prerelease': True,
            'assets': [{'name': 'a', 'size': 2, 'digest': 'sha256:aaa'}],
        }
        publisher.verify_release(release, 'candidate', 'a' * 40, assets)
        release['assets'][0]['digest'] = 'sha256:bbb'
        with self.assertRaises(ValueError):
            publisher.verify_release(release, 'candidate', 'a' * 40, assets)


if __name__ == "__main__":
    unittest.main()
