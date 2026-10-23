#!/usr/bin/env python3
"""Keep the application installer at its managed Applications location."""
import argparse
from pathlib import Path
import plistlib


def configure_components(path):
    with path.open("rb") as stream:
        components = plistlib.load(stream)
    if not isinstance(components, list):
        raise ValueError("Installer components must be an array")
    applications = [component for component in components
                    if isinstance(component, dict)
                    and component.get("RootRelativeBundlePath") == "Applications/DockPipe.app"]
    if len(applications) != 1:
        raise ValueError("Installer must contain exactly one DockPipe application component")
    applications[0]["BundleIsRelocatable"] = False
    with path.open("wb") as stream:
        plistlib.dump(components, stream, sort_keys=False)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("components", type=Path)
    arguments = parser.parse_args()
    configure_components(arguments.components)
