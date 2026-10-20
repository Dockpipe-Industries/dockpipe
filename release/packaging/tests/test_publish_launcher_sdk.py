import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location(
    'publisher', Path(__file__).resolve().parents[1] / 'publish-launcher-sdk.py')
publisher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(publisher)


class CandidateBoundaryTests(unittest.TestCase):
    def test_existing_tag_must_resolve_to_the_qualified_commit(self):
        commit = {'__typename': 'Commit', 'oid': 'a' * 40}
        publisher.verify_tag({'target': commit}, 'a' * 40)
        publisher.verify_tag({'target': {'__typename': 'Tag', 'target': commit}}, 'a' * 40)
        publisher.verify_tag(None, 'a' * 40, allow_missing=True)
        for reference in [None, {'target': {'__typename': 'Commit', 'oid': 'b' * 40}},
                          {'target': {'__typename': 'Tree', 'oid': 'a' * 40}}]:
            with self.assertRaises(ValueError):
                publisher.verify_tag(reference, 'a' * 40)

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
