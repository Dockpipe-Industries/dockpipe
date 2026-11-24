"""Pure validation of forwarding in an independently admitted QEMU argv snapshot.

This does not discover a process, admit guest identity, or prove service readiness.
Never join argv: a following -device option is a separate argument, not netdev data.
"""
import re


MAXIMUM_ARGUMENTS = 4096
MAXIMUM_ARGUMENT_BYTES = 1 << 20
AGENT_GUEST_PORT = 47831


def port(value, label):
    if type(value) is not int or not 1 <= value <= 65535:
        raise ValueError(label + " must be an integer port from 1 to 65535")
    return value


def netdev_arguments(arguments):
    if not isinstance(arguments, (list, tuple)) or not 1 <= len(arguments) <= MAXIMUM_ARGUMENTS:
        raise ValueError("QEMU arguments must be a bounded argv sequence")
    if any(not isinstance(value, str) or not value or len(value) > 65536
           or any(character in value for character in "\0\r\n") for value in arguments):
        raise ValueError("QEMU argv contains an invalid element")
    if sum(len(value.encode("utf-8")) for value in arguments) > MAXIMUM_ARGUMENT_BYTES:
        raise ValueError("QEMU argv exceeds its byte bound")
    result = []
    index = 0
    while index < len(arguments):
        argument = arguments[index]
        if argument == "-netdev":
            index += 1
            if index >= len(arguments) or arguments[index].startswith("-"):
                raise ValueError("QEMU -netdev requires its own value argument")
            result.append(arguments[index])
        elif argument.startswith("-netdev="):
            result.append(argument[len("-netdev="):])
        elif argument in ("-net", "-nic") or argument.startswith(("-net=", "-nic=")):
            raise ValueError("Alternative network option syntax is outside this forwarding profile")
        index += 1
    if len(result) != 1:
        raise ValueError("Windows qualification requires exactly one user netdev")
    return result


def validate_forwarding(arguments, expected_ssh_host_port):
    """Require exactly SSH plus one dynamically allocated loopback Agent forward."""
    expected_ssh_host_port = port(expected_ssh_host_port, "Expected SSH host port")
    forwards = []
    for argument in netdev_arguments(arguments):
        fields = argument.split(",")
        if fields[0] != "user":
            raise ValueError("Windows qualification requires the user network backend")
        for field in fields[1:]:
            key, separator, value = field.partition("=")
            if not separator or not value or re.fullmatch(r"[a-z][a-z0-9_-]{0,63}", key) is None:
                raise ValueError("Malformed QEMU netdev option")
            if key == "guestfwd":
                raise ValueError("Guest forwarding is outside this forwarding profile")
            if key != "hostfwd":
                continue
            if fields[0] != "user":
                raise ValueError("Host forwarding requires the user network backend")
            match = re.fullmatch(r"tcp:127\.0\.0\.1:([1-9][0-9]{0,4})-:([1-9][0-9]{0,4})", value)
            if match is None:
                raise ValueError("Forward must be TCP on explicit IPv4 loopback with a default guest address")
            forwards.append({"host_address": "127.0.0.1", "host_port": port(int(match[1]), "Host port"),
                             "guest_port": port(int(match[2]), "Guest port")})
    if len(forwards) != 2 or len({item["host_port"] for item in forwards}) != 2:
        raise ValueError("Expected exactly two distinct host forwards")
    ssh = [item for item in forwards if item["guest_port"] == 22]
    agent = [item for item in forwards if item["guest_port"] == AGENT_GUEST_PORT]
    if len(ssh) != 1 or len(agent) != 1 or ssh[0]["host_port"] != expected_ssh_host_port:
        raise ValueError("Forward destinations or the selected SSH host port differ")
    return {"ssh": ssh[0], "agent": agent[0]}
