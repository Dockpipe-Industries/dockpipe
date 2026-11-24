"""Execute one bounded accessibility action in an explicitly owned GUI session."""

import argparse
import importlib.util
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--session", required=True)
    parser.add_argument("--name", choices=["Start Windows VM"], required=True)
    parser.add_argument("--role", choices=["menu item"], required=True)
    parser.add_argument("--action", choices=["Press"], required=True)
    arguments = parser.parse_args()
    path = Path(__file__).parents[1] / "linux/ui.py"
    specification = importlib.util.spec_from_file_location("product_shared_ui", path)
    ui = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(ui)
    registry, session = ui.connect(arguments.session)
    node = ui.select(registry.Registry, session["gui_pid"], arguments.name, arguments.role)
    ui.action(node, arguments.action)


if __name__ == "__main__":
    main()
