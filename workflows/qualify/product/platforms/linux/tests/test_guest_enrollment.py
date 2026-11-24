"""Check run binding and grant cleanup for an owned Linux guest."""
from datetime import datetime, timedelta, timezone
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import Mock, patch


sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from driver import admit_configuration
from guest import Guest
from guest_enrollment import GuestEnrollment, expires


class GuestEnrollmentTests(unittest.TestCase):
    worker = {'path': '/fixture/dockpipe.deb', 'sha256': 'd' * 64}
    desktop = {'path': '/fixture/dockpipe-desktop.deb', 'sha256': 'c' * 64}
    metadata = [b'Package: dockpipe\nArchitecture: amd64\nVersion: 0.6.39\n',
                b'Package: dockpipe-desktop\nArchitecture: amd64\nVersion: 0.6.39\n']

    def setUp(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        self.root = Path(directory.name)
        self.host = self.root / 'host-state'
        self.host.mkdir(mode=0o700)
        self.write_private(self.host / 'worker.json', {
            'node': 'fixture-host', 'endpoint': 'https://fixture.example',
            'machine_key': 'host-key', 'broker_key': 'broker-key',
        })
        self.grant = {'id': 'a' * 64, 'host': 'fixture-host', 'run': 'fixture-run', 'guests': 1,
                      'broker_key': 'broker-key',
                      'expires_at': (datetime.now(timezone.utc) + timedelta(minutes=2)).isoformat()}
        grant_path = self.root / 'grant.json'
        self.write_private(grant_path, self.grant)
        self.selection = {'grant_file': str(grant_path), 'host_state': str(self.host),
                          'grant_sha256': hashlib.sha256(grant_path.read_bytes()).hexdigest()}
        self.enrollment_receipt = {'node': 'fixture-run-vm', 'grant': self.grant['id'],
                                   'expires_at': self.grant['expires_at']}
        claim = {'node': 'fixture-run-vm', 'grant': self.grant['id'], 'token_hash': 'b' * 64,
                 'public_key': 'guest-key', 'proof': 'guest-proof'}
        self.guest = Mock()
        self.guest.run.return_value = subprocess.CompletedProcess([], 0, stdout=json.dumps(claim).encode())
        self.guest.start.return_value.poll.return_value = None

    def write_private(self, path, value):
        path.write_text(json.dumps(value))
        path.chmod(0o600)

    def test_rejects_grant_for_another_run(self):
        with self.assertRaisesRegex(ValueError, 'exact qualification run'):
            GuestEnrollment(self.selection, 'another-run', '/fixture/dockpipe', self.root)

    def test_accepts_broker_nanosecond_utc_on_python_310(self):
        parsed = expires('2026-11-19T09:10:42.652657598Z')
        self.assertEqual(parsed, datetime(2026, 11, 19, 9, 10, 42, 652657, tzinfo=timezone.utc))

    def test_rejects_grant_that_changed_after_planning(self):
        self.selection['grant_sha256'] = 'b' * 64
        with self.assertRaisesRegex(ValueError, 'selected digest'):
            GuestEnrollment(self.selection, 'fixture-run', '/fixture/dockpipe', self.root)

    def test_rejects_shared_grant_even_when_it_matches_the_plan(self):
        grant_path = Path(self.selection['grant_file'])
        self.write_private(grant_path, {**self.grant, 'guests': 2})
        self.selection['grant_sha256'] = hashlib.sha256(grant_path.read_bytes()).hexdigest()

        with self.assertRaisesRegex(ValueError, 'dedicated one-guest grant'):
            GuestEnrollment(self.selection, 'fixture-run', '/fixture/dockpipe', self.root)

    def test_guided_driver_rejects_worker_deb_that_changed_after_planning(self):
        deb = self.root / 'dockpipe.deb'
        deb.write_bytes(b'changed')
        configuration = {'operation': 'guided-install', 'pins_file': '', 'pins_sha256': '',
                         'media_file': '', 'media_sha256': '', 'baseline_receipt': '',
                         'baseline_receipt_sha256': '', 'guest_enrollment': {
                             **self.selection, 'worker_deb': str(deb), 'worker_deb_sha256': 'b' * 64,
                             'desktop_deb': str(deb), 'desktop_deb_sha256': 'c' * 64}}

        with self.assertRaisesRegex(ValueError, 'worker_deb differs'):
            admit_configuration(configuration, self.root)

    def test_candidate_must_upgrade_baseline_before_package_transfer(self):
        self.guest.run.return_value = subprocess.CompletedProcess([], 1)

        with patch('guest.subprocess.check_output', side_effect=self.metadata):
            with self.assertRaisesRegex(ValueError, 'must upgrade'):
                Guest.install_enrollment_product(self.guest, [self.worker, self.desktop], self.root)

        self.guest.install_packages.assert_not_called()

    def test_mismatched_desktop_version_rejected_before_guest_mutation(self):
        metadata = [self.metadata[0], self.metadata[1].replace(b'0.6.39', b'0.6.38')]

        with patch('guest.subprocess.check_output', side_effect=metadata):
            with self.assertRaisesRegex(ValueError, 'same version'):
                Guest.install_enrollment_product(self.guest, [self.worker, self.desktop], self.root)

        self.guest.install_packages.assert_not_called()

    def test_installed_product_evidence_binds_debs_and_binaries(self):
        witness = (('e' * 64) + '  /usr/bin/dockpipe\n' + ('f' * 64)
                   + '  /usr/bin/dockpipe-launcher\ndockpipe 0.6.39\n').encode()
        self.guest.run.side_effect = [subprocess.CompletedProcess([], 0),
                                     subprocess.CompletedProcess([], 0, stdout=witness)]

        with patch('guest.subprocess.check_output', side_effect=self.metadata):
            receipt = Guest.install_enrollment_product(self.guest, [self.worker, self.desktop], self.root)

        self.guest.install_packages.assert_called_once()
        self.assertEqual(receipt, {'worker_deb_sha256': 'd' * 64, 'desktop_deb_sha256': 'c' * 64,
                                   'cli_sha256': 'e' * 64, 'desktop_sha256': 'f' * 64,
                                   'package_version': '0.6.39', 'version_output': 'dockpipe 0.6.39'})

    def test_enrolls_own_guest_and_releases_before_waiting_for_worker_exit(self):
        enrollment = GuestEnrollment(self.selection, 'fixture-run', '/fixture/dockpipe', self.root)

        def broker(command, *_arguments):
            if command == 'guest-enroll':
                return self.enrollment_receipt

            self.assertEqual(command, 'guest-release')
            self.guest.start.return_value.wait.assert_not_called()
            self.guest.start.return_value.poll.return_value = 0
            return {'grant': self.grant['id'], 'removed': ['fixture-run-vm'], 'retained': []}

        with patch.object(enrollment, 'call_host', side_effect=broker) as command:
            receipt = enrollment.start(self.guest)
            released = enrollment.stop()

        self.assertEqual(receipt['node'], 'fixture-run-vm')
        self.assertEqual(released['removed'], ['fixture-run-vm'])
        self.assertTrue(released['worker_transport_running_at_release'])
        self.assertTrue(released['worker_transport_exited'])
        guest_request = json.loads(self.guest.run.call_args.kwargs['input_data'])
        self.assertEqual(guest_request, {'endpoint': 'https://fixture.example', 'node': 'fixture-run-vm',
                                         'grant': {key: value for key, value in self.grant.items()
                                                   if key != 'guests'}})
        self.assertEqual(command.call_count, 2)
        self.guest.start.return_value.wait.assert_called_once_with(timeout=15)

    def test_loopback_broker_uses_a_forward_owned_by_the_worker_transport(self):
        path = self.host / 'worker.json'
        config = json.loads(path.read_text())
        config['endpoint'] = 'http://127.0.0.1:38125'
        self.write_private(path, config)
        enrollment = GuestEnrollment(self.selection, 'fixture-run', '/fixture/dockpipe', self.root)

        with patch.object(enrollment, 'call_host', return_value=self.enrollment_receipt):
            enrollment.start(self.guest)

        options = self.guest.start.call_args.args[1]
        self.assertEqual(options, ('-o', 'ExitOnForwardFailure=yes', '-R', '127.0.0.1:38125:127.0.0.1:38125'))

    def test_releases_exact_grant_after_uncertain_enrollment_response(self):
        enrollment = GuestEnrollment(self.selection, 'fixture-run', '/fixture/dockpipe', self.root)
        release = {'grant': self.grant['id'], 'removed': ['fixture-run-vm'], 'retained': []}

        with patch.object(enrollment, 'call_host', side_effect=[RuntimeError('response lost'), release]) as command:
            with self.assertRaisesRegex(RuntimeError, 'response lost'):
                enrollment.start(self.guest)
            self.assertEqual(enrollment.stop()['removed'], ['fixture-run-vm'])

        command.assert_called_with('guest-release', '--id', self.grant['id'])
        self.guest.start.assert_not_called()

    def test_early_worker_exit_fails_enrollment_and_releases_exact_node(self):
        enrollment = GuestEnrollment(self.selection, 'fixture-run', '/fixture/dockpipe', self.root)
        release = {'grant': self.grant['id'], 'removed': ['fixture-run-vm'], 'retained': []}
        self.guest.start.return_value.poll.return_value = 1

        with patch.object(enrollment, 'call_host', side_effect=[self.enrollment_receipt, release]) as command:
            with self.assertRaisesRegex(RuntimeError, 'worker for fixture-run-vm exited before release'):
                enrollment.start(self.guest)
            self.assertEqual(enrollment.stop()['removed'], ['fixture-run-vm'])

        command.assert_called_with('guest-release', '--id', self.grant['id'])

    def test_expired_grant_stops_gui_wait_even_with_a_running_transport(self):
        enrollment = GuestEnrollment(self.selection, 'fixture-run', '/fixture/dockpipe', self.root)
        with patch.object(enrollment, 'call_host', return_value=self.enrollment_receipt):
            enrollment.start(self.guest)

        past = datetime.now(timezone.utc) - timedelta(seconds=1)
        with patch('guest_enrollment.expires', return_value=past):
            with self.assertRaisesRegex(ValueError, 'guest grant has expired'):
                enrollment.require_running()
