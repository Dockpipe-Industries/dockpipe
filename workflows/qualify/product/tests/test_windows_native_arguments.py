"""Pure argv snapshot contracts; no process discovery, GUI or guest operations."""
import importlib.util
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[1] / 'platforms/windows'
specification = importlib.util.spec_from_file_location('windows_native_arguments', ROOT / 'native_arguments.py')
NATIVE = importlib.util.module_from_spec(specification)
specification.loader.exec_module(NATIVE)


def arguments(ssh='tcp:127.0.0.1:3865-:22', agent='tcp:127.0.0.1:34617-:47831'):
    return ['/pinned/qemu-system-x86_64', '-netdev', f'user,id=net0,hostfwd={ssh},hostfwd={agent}',
            '-device', 'e1000e,netdev=net0', '-drive', 'file=/private/path with spaces/disk.qcow2,format=qcow2']


class WindowsNativeArgumentTests(unittest.TestCase):
    def test_separate_following_device_argument_is_not_part_of_netdev(self):
        result = NATIVE.validate_forwarding(arguments(), 3865)
        self.assertEqual(result['ssh'], {'host_address':'127.0.0.1','host_port':3865,'guest_port':22})
        self.assertEqual(result['agent'], {'host_address':'127.0.0.1','host_port':34617,'guest_port':47831})

    def test_dynamic_agent_port_is_observed_instead_of_previous_run_constant(self):
        for selected in (38425, 34617, 1, 65535):
            with self.subTest(port=selected):
                result = NATIVE.validate_forwarding(arguments(agent=f'tcp:127.0.0.1:{selected}-:47831'), 3865)
                self.assertEqual(result['agent']['host_port'], selected)

    def test_exact_expected_ssh_host_port_remains_required(self):
        with self.assertRaises(ValueError):
            NATIVE.validate_forwarding(arguments(), 3866)
        for expected in (True, '3865', 0, -1, 65536):
            with self.subTest(expected=expected), self.assertRaises(ValueError):
                NATIVE.validate_forwarding(arguments(), expected)

    def test_wildcard_nonloopback_and_unknown_protocols_are_refused(self):
        for forward in ('tcp::34617-:47831', 'tcp:0.0.0.0:34617-:47831',
                        'tcp:192.0.2.1:34617-:47831', 'tcp:127.0.0.2:34617-:47831',
                        'udp:127.0.0.1:34617-:47831', 'sctp:127.0.0.1:34617-:47831',
                        'tcp:[::1]:34617-:47831', 'tcp:127.0.0.1:34617-10.0.2.15:47831'):
            with self.subTest(forward=forward), self.assertRaises(ValueError):
                NATIVE.validate_forwarding(arguments(agent=forward), 3865)

    def test_port_bounds_malformed_ports_and_unknown_destinations_are_refused(self):
        for forward in ('tcp:127.0.0.1:0-:47831', 'tcp:127.0.0.1:65536-:47831',
                        'tcp:127.0.0.1:-1-:47831', 'tcp:127.0.0.1:034617-:47831',
                        'tcp:127.0.0.1:34617-:0', 'tcp:127.0.0.1:34617-:65536',
                        'tcp:127.0.0.1:34617-:47832', 'tcp:127.0.0.1:34617-:47831 -device e1000e'):
            with self.subTest(forward=forward), self.assertRaises(ValueError):
                NATIVE.validate_forwarding(arguments(agent=forward), 3865)

    def test_extra_or_duplicate_forwards_are_refused_across_all_netdev_arguments(self):
        original = arguments()
        cases = [original[:2] + [original[2] + ',hostfwd=tcp:127.0.0.1:8080-:80'] + original[3:],
                 original + ['-netdev', 'user,id=other,hostfwd=tcp:127.0.0.1:8080-:80'],
                 arguments(agent='tcp:127.0.0.1:3865-:47831'),
                 arguments(agent='tcp:127.0.0.1:34617-:22')]
        for selected in cases:
            with self.subTest(argv=selected), self.assertRaises(ValueError):
                NATIVE.validate_forwarding(selected, 3865)

    def test_additional_socket_tap_or_user_backend_is_refused(self):
        for backend in ('socket,id=public,listen=:12345', 'tap,id=public,ifname=tap0', 'user,id=extra'):
            with self.subTest(backend=backend), self.assertRaises(ValueError):
                NATIVE.validate_forwarding(arguments() + ['-netdev', backend], 3865)
        replacement = arguments()
        replacement[2] = replacement[2].replace('user,', 'socket,', 1)
        with self.assertRaises(ValueError):
            NATIVE.validate_forwarding(replacement, 3865)

    def test_missing_or_alternative_forwarding_is_refused(self):
        cases = [['/qemu', '-netdev'], ['/qemu', '-netdev', '-device', 'e1000e'],
                 ['/qemu', '-netdev', 'user,id=net0,hostfwd=tcp:127.0.0.1:3865-:22'],
                 arguments() + ['-nic', 'user,hostfwd=tcp:127.0.0.1:8080-:80'],
                 arguments() + ['-netdev', 'user,id=other,guestfwd=tcp:10.0.2.100:80-cmd:helper']]
        for selected in cases:
            with self.subTest(argv=selected), self.assertRaises(ValueError):
                NATIVE.validate_forwarding(selected, 3865)

    def test_argv_must_be_elementwise_and_bounded(self):
        for selected in (' '.join(arguments()), [], [None], ['/qemu', '\0'], ['/qemu', 'x' * 65537],
                         ['/qemu'] * 4097):
            with self.subTest(kind=type(selected).__name__), self.assertRaises(ValueError):
                NATIVE.validate_forwarding(selected, 3865)

    def test_public_lifecycle_exposes_same_parser_without_automatic_process_claims(self):
        specification = importlib.util.spec_from_file_location('public_windows_lifecycle', ROOT / 'lifecycle.py')
        lifecycle = importlib.util.module_from_spec(specification)
        specification.loader.exec_module(lifecycle)
        self.assertEqual(lifecycle.NATIVE_ARGUMENTS.validate_forwarding(arguments(), 3865),
                         NATIVE.validate_forwarding(arguments(), 3865))


if __name__ == '__main__':
    unittest.main()
