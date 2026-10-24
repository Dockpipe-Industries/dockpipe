"""Immutable SDK transactions survive unrelated release volume and partial outcomes."""
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import test_publish_launcher_sdk as launcher_tests
import test_publish_display_sdk as display_tests
import test_cli_candidate as cli_tests

SOURCE = 'a' * 40
ASSETS = {'fixture': {'size': 2, 'digest': 'sha256:aa'}}


class Remote:
    def __init__(self, shared, tag, state='absent'):
        self.shared = shared
        self.tag = tag
        self.exists = state != 'absent'
        self.release = {'id': 123, 'tag_name': tag, 'target_commitish': SOURCE,
                        'prerelease': True, 'draft': state != 'published',
                        'html_url': 'https://github.com/example/candidate',
                        'assets': [] if state in ('absent', 'partial') else
                        [{'name': name, 'state': 'uploaded', **data} for name, data in ASSETS.items()]}
        self.calls = []
        self.lose_create = False
        self.lose_upload = False

    def execute(self, arguments):
        self.calls.append(arguments)
        if 'graphql' in arguments:
            reference = None if self.release['draft'] else {'target': {'__typename': 'Commit', 'oid': SOURCE}}
            return json.dumps({'data': {'repository': {'ref': reference}}})
        if arguments[1:3] == ['release', 'upload']:
            name = Path(arguments[4]).name
            if any(item['name'] == name for item in self.release['assets']):
                raise AssertionError('Attempted to replace existing bytes')
            self.release['assets'].append({'name': name, 'state': 'uploaded', **ASSETS[name]})
            if self.lose_upload:
                self.lose_upload = False
                raise ValueError('Ambiguous upload response')
            return ''
        if arguments[1:3] == ['release', 'edit']:
            self.release['draft'] = False
            return ''
        if '--method' in arguments:
            if self.exists:
                raise ValueError('HTTP 422 existing tag')
            self.exists = True
            if self.lose_create:
                self.lose_create = False
                raise ValueError('Ambiguous create response')
            return json.dumps({'id': 123, 'tag_name': self.tag})
        endpoint = next((item for item in arguments if item.startswith('repos/')), '')
        if '/releases?' in endpoint:
            # Broad output would exceed the unchanged 2MiB cap. Projection must
            # happen in gh before Python captures it, not after loading JSON.
            if '--jq' not in arguments:
                return 'irrelevant release bodies' * (1 << 20)
            if not self.exists:
                return json.dumps({'count': 0, 'matches': []})
            return json.dumps({
                'count': 100,
                'matches': [{'id': 123, 'tag_name': self.tag}],
            })
        if endpoint.endswith('/releases/123'):
            return json.dumps(self.release)
        raise AssertionError('Unexpected SDK operation: ' + repr(arguments))


class SDKTransactionTests(unittest.TestCase):
    def invoke(self, module, shared, remote, root):
        environment = {'GITHUB_REPOSITORY': shared.REPOSITORY, 'GITHUB_REF': 'refs/heads/dev',
                       'GITHUB_EVENT_NAME': 'push', 'GITHUB_SHA': SOURCE}
        with patch.dict(shared.os.environ, environment, clear=True), \
                patch.object(module, 'validate', return_value=(remote.tag, ASSETS)
                             if module is cli_tests.PUBLISHER else ASSETS), \
                patch.object(shared, 'run', side_effect=remote.execute), patch('builtins.print'):
            if module is display_tests.PUBLISHER:
                module.publish(root, 'darwin-arm64')
            else:
                module.publish(root)

    def publishers(self):
        yield launcher_tests.publisher, launcher_tests.publisher, 'launcher-sdk-1.1.0-staging.' + SOURCE
        yield display_tests.PUBLISHER, display_tests.PUBLISHER.SHARED, 'display-sdk-1.0.0-darwin-arm64-staging.' + SOURCE
        yield cli_tests.PUBLISHER, cli_tests.PUBLISHER.SHARED, 'v0.6.26-staging.cli.sha' + SOURCE[:12]

    def test_all_publishers_create_by_id_and_resume_only_missing_assets(self):
        for module, shared, tag in self.publishers():
            for state in ('absent', 'partial', 'draft', 'published'):
                with self.subTest(module=module.__name__, state=state), tempfile.TemporaryDirectory() as temporary:
                    remote = Remote(shared, tag, state)
                    self.invoke(module, shared, remote, Path(temporary))
                    self.assertFalse(remote.release['draft'])
                    creations = [args for args in remote.calls if '--method' in args]
                    self.assertEqual(len(creations), int(state == 'absent'))
                    uploads = [args for args in remote.calls if args[1:3] == ['release', 'upload']]
                    self.assertEqual(len(uploads), int(state in ('absent', 'partial')))
                    if creations:
                        index = remote.calls.index(creations[0])
                        self.assertIn('/releases/123', ' '.join(remote.calls[index + 1]))
                    self.assertFalse(any('--clobber' in args or 'delete' in args for args in remote.calls))

    def test_ambiguous_create_or_upload_stops_then_resumes_without_duplicate_bytes(self):
        for module, shared, tag in self.publishers():
            for lost in ('lose_create', 'lose_upload'):
                with self.subTest(module=module.__name__, lost=lost), tempfile.TemporaryDirectory() as temporary:
                    remote = Remote(shared, tag)
                    setattr(remote, lost, True)
                    with self.assertRaisesRegex(ValueError, 'Ambiguous'):
                        self.invoke(module, shared, remote, Path(temporary))
                    self.assertTrue(remote.release['draft'])
                    self.invoke(module, shared, remote, Path(temporary))
                    self.assertEqual(sum('--method' in args for args in remote.calls), 1)
                    self.assertEqual(sum(args[1:3] == ['release', 'upload'] for args in remote.calls), 1)

    def test_published_partial_or_conflicting_assets_never_upload_or_publish(self):
        for module, shared, tag in self.publishers():
            for changed in ('missing', 'digest', 'extra'):
                with self.subTest(module=module.__name__, changed=changed), tempfile.TemporaryDirectory() as temporary:
                    remote = Remote(shared, tag, 'published')
                    if changed == 'missing':
                        remote.release['assets'] = []
                    elif changed == 'digest':
                        remote.release['assets'][0]['digest'] = 'sha256:wrong'
                    else:
                        remote.release['assets'].append({'name': 'other'})
                    with self.assertRaises(ValueError):
                        self.invoke(module, shared, remote, Path(temporary))
                    self.assertFalse(any(args[1] == 'release' or '--method' in args for args in remote.calls))

    def test_concurrent_publication_cannot_extend_an_incomplete_published_release(self):
        shared = launcher_tests.publisher
        tag = 'candidate'
        assets = {name: {'size': 2, 'digest': 'sha256:aa'} for name in ('first', 'second')}
        draft = {'id': 123, 'tag_name': tag, 'target_commitish': SOURCE,
                 'draft': True, 'prerelease': True, 'assets': []}
        published = {**draft, 'draft': False, 'assets': [
            {'name': 'first', 'state': 'uploaded', **assets['first']}]}
        with patch.object(shared, 'run', return_value='') as run, \
                patch.object(shared, 'read_release', return_value=published):
            with self.assertRaisesRegex(ValueError, 'inventory differs'):
                shared.reconcile_assets(Path('fixture'), draft, tag, SOURCE, assets)
            self.assertEqual(run.call_count, 1)
            self.assertEqual(Path(run.call_args.args[0][4]).name, 'first')

    def test_large_irrelevant_inventory_is_projected_before_bounded_capture(self):
        shared = launcher_tests.publisher
        from types import SimpleNamespace
        response = {'id': 123, 'tag_name': 'candidate', 'draft': True}
        calls = []
        inventory = [{'id': index + 1000, 'tag_name': 'unrelated-' + str(index),
                      'body': 'irrelevant body' * 2500} for index in range(99)]
        inventory.append({'id': 123, 'tag_name': 'candidate', 'body': 'selected body'})
        raw_inventory = json.dumps(inventory)
        self.assertGreater(len(raw_inventory), 2 << 20)

        def execute(arguments, **kwargs):
            calls.append(arguments)
            if '--jq' not in arguments:
                return SimpleNamespace(returncode=0, stdout=raw_inventory, stderr='')
            query = arguments[arguments.index('--jq') + 1]
            if '/releases?' in ' '.join(arguments):
                self.assertIn('select(.tag_name=="candidate")', query)
                output = {'count': len(inventory), 'matches': [
                    {key: item[key] for key in ('id', 'tag_name')}
                    for item in inventory if item['tag_name'] == 'candidate']}
            else:
                self.assertNotIn('body', query)
                self.assertIn('digest', query)
                output = response
            return SimpleNamespace(returncode=0, stdout=json.dumps(output), stderr='')

        with patch.object(shared.subprocess, 'run', side_effect=execute):
            self.assertEqual(shared.find_release('candidate'), response)
        self.assertEqual(len(calls), 2)
        with patch.object(shared.subprocess, 'run', return_value=SimpleNamespace(
                returncode=0, stdout='x' * ((2 << 20) + 1), stderr='')):
            with self.assertRaisesRegex(ValueError, 'exceeds metadata capacity'):
                shared.run(['gh', 'api', 'unexpected-large-selected-response'])

    def test_changed_create_id_or_tag_never_reconciles_assets(self):
        shared = launcher_tests.publisher
        for created in ({'id': 0, 'tag_name': 'candidate'}, {'id': 123, 'tag_name': 'other'}):
            with self.subTest(created=created), patch.object(shared, 'run', return_value=json.dumps(created)) as run:
                with self.assertRaises(ValueError):
                    shared.create_release('candidate', SOURCE, 'title', 'notes')
                self.assertEqual(run.call_count, 1)


if __name__ == '__main__':
    unittest.main()
