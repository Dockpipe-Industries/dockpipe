#!/usr/bin/env python3
"""Check every exact requirements pin against this active SDK interpreter."""
from importlib.metadata import version
from pathlib import Path
import re
import sys


def verify(requirements):
    pins = dict(re.findall(r"(?m)^([A-Za-z0-9_.-]+)==([^\s;\\]+)", requirements.read_text()))
    if not pins:
        raise ValueError("The SDK requirements lock must contain exact version pins")
    for name, expected in pins.items():
        actual = version(name)
        if actual != expected:
            raise ValueError(f"SDK dependency {name} differs from its lock: {actual} != {expected}")


if __name__ == "__main__":
    verify(Path(sys.argv[1]))
