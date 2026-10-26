"""Check source ownership and build references without compiling the GUI."""
from pathlib import Path
import re
import unittest


APP_ROOT = Path(__file__).resolve().parents[1]
SOURCE_ROOT = APP_ROOT / "src"


class SourceLayoutTests(unittest.TestCase):
    def test_local_header_names_are_unambiguous(self):
        headers = {}
        for path in SOURCE_ROOT.rglob("*.h"):
            self.assertNotIn(path.name, headers, f"ambiguous bare include: {path.name}")
            headers[path.name] = path

    def test_domain_does_not_depend_on_effects_or_widgets(self):
        headers = {path.name: path for path in SOURCE_ROOT.rglob("*.h")}
        forbidden_qt = {"QWidget", "QDialog", "QProcess", "QFile", "QSaveFile", "QNetworkAccessManager"}
        for path in SOURCE_ROOT.rglob("*"):
            if not path.is_file() or "domain" not in path.parts:
                continue
            for header in re.findall(r'^#include\s+[<"]([^>"]+)[>"]', path.read_text(), re.MULTILINE):
                self.assertNotIn(header, forbidden_qt, f"effectful domain include: {path}")
                if header in headers:
                    dependency = headers[header]
                    self.assertTrue(
                        "domain" in dependency.parts or "contracts" in dependency.parts,
                        f"domain dependency crosses into an adapter or view: {path} -> {dependency}",
                    )

    def test_explicit_cmake_source_paths_exist(self):
        files = [APP_ROOT / "CMakeLists.txt", *sorted((APP_ROOT / "cmake").glob("*.cmake")),
                 APP_ROOT / "display-sdk" / "CMakeLists.txt"]
        for file in files:
            for relative in re.findall(r'\bsrc/[\w/.-]+\.(?:cpp|mm|h)\b', file.read_text()):
                self.assertTrue((APP_ROOT / relative).is_file(), f"missing CMake source: {file}: {relative}")

    def test_every_source_directory_is_explicitly_available(self):
        layout = (APP_ROOT / "cmake" / "AppSourceLayout.cmake").read_text()
        for path in SOURCE_ROOT.rglob("*.h"):
            relative = path.parent.relative_to(SOURCE_ROOT).as_posix()
            self.assertIn('${DOCKPIPE_APP_SOURCE_ROOT}/' + relative + '"', layout)

    def test_display_sdk_headers_are_available_to_build_tree_consumers(self):
        sdk_root = APP_ROOT / "display-sdk"
        cmake = (sdk_root / "CMakeLists.txt").read_text()
        headers = re.findall(r'set\(\w+_header "([^"]+)"\)', cmake)
        include_paths = re.findall(r'\$<BUILD_INTERFACE:([^>]+)>', cmake)
        self.assertTrue(headers, "Display SDK must declare its public headers")
        self.assertTrue(include_paths, "Display SDK must expose build-tree includes")

        include_directories = {
            Path(path.replace("${CMAKE_CURRENT_SOURCE_DIR}", str(sdk_root))).resolve()
            for path in include_paths
        }
        for header in headers:
            path = Path(header.replace("${CMAKE_CURRENT_SOURCE_DIR}", str(sdk_root))).resolve()
            self.assertIn(path.parent, include_directories, f"SDK header is unavailable: {path.name}")


if __name__ == "__main__":
    unittest.main()
