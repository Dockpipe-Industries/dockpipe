"""Thin bridge to the owning normal installed Flatpak GUI qualification."""
from pathlib import Path
import importlib.util

OWNER = (Path(__file__).resolve().parents[4]
         / 'workflows/build/flatpak/assets/scripts/installed-remote-product.py')


def main():
    specification = importlib.util.spec_from_file_location('owning_installed_remote_product', OWNER)
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    module.main()


if __name__ == '__main__':
    main()
