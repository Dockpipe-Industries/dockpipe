"""A successful process without the product identity output must not pass."""
from pathlib import Path
import importlib.util
import sys
import unittest


OWNER = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(OWNER / "drivers"))
specification = importlib.util.spec_from_file_location("portable_identity_driver", OWNER / "drivers/portable.py")
driver = importlib.util.module_from_spec(specification)
specification.loader.exec_module(driver)


class PortableIdentityRegression(unittest.TestCase):
    def test_real_release_and_development_tokens_are_observable(self):
        for version in ("0.6.38\n", "0.6.38-dev", "0.6.38-staging.123+hash", "dev", "dev+hash.dirty"):
            with self.subTest(version=version):
                self.assertTrue(driver.version_observed(version))

    def test_empty_garbage_and_multiline_output_cannot_qualify(self):
        for version in ("", "   ", "everything passed", "0.6.38\nunexpected output", "01.6.38", "0.6.38-.."):
            with self.subTest(version=version):
                self.assertFalse(driver.version_observed(version))

    def test_operator_selected_exact_version_must_match(self):
        self.assertTrue(driver.version_observed("0.6.38\n", "0.6.38"))
        self.assertFalse(driver.version_observed("0.6.37\n", "0.6.38"))


if __name__ == "__main__":
    unittest.main()
