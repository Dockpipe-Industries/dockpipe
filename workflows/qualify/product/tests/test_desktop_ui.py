import importlib.util
from pathlib import Path
from types import SimpleNamespace
import unittest


SOURCE = Path(__file__).resolve().parents[1] / "platforms/linux/ui.py"
SPEC = importlib.util.spec_from_file_location("desktop_ui", SOURCE)
UI = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(UI)


class Node:
    def __init__(self, pid, children=()):
        self.pid = pid
        self.children = children

    def get_process_id(self):
        return self.pid

    def __iter__(self):
        return iter(self.children)


class DesktopUITests(unittest.TestCase):
    def test_transient_null_children_do_not_hide_valid_owned_controls(self):
        control = Node(17)
        owned = Node(17, (None, control, None))
        other = Node(21, (Node(21),))
        registry = SimpleNamespace(getDesktop=lambda index: [owned, other])

        self.assertEqual(list(UI.nodes(registry, 17)), [(owned, 0), (control, 1)])


if __name__ == "__main__":
    unittest.main()
