"""Validate Qt catalogs and the app-owned extraction surface without network access."""

import argparse
from collections import Counter
from pathlib import Path
import re
import subprocess
import tempfile
import unittest
import xml.etree.ElementTree as ET


APP_ROOT = Path(__file__).resolve().parents[1]
CATALOGS = APP_ROOT / "translations"
LANGUAGES = {"en": 2, "es": 2, "fr": 2, "de": 2, "pt": 2, "it": 2,
             "nl": 2, "pl": 3, "ru": 3, "uk": 3, "tr": 1, "ar": 6,
             "hi": 2, "ja": 1, "ko": 1, "zh_CN": 1, "zh_TW": 1}
PLACEHOLDER = re.compile(r"%L?(?:[1-9][0-9]*|n)|%p%")
TAG = re.compile(r"</?[A-Za-z][^>]*>")
ACCELERATOR = re.compile(r"(?<!&)&(?![&\s])")
LUPDATE = None


def messages(path):
    root = ET.parse(path).getroot()
    result = {}
    for context in root.findall("context"):
        name = context.findtext("name")
        for message in context.findall("message"):
            key = (name, message.findtext("source", ""), message.findtext("comment", ""),
                   message.get("numerus", "no"))
            if key in result:
                raise ValueError(f"Duplicate message in {path.name}: {key}")
            result[key] = message
    return root, result


class TranslationCatalogTests(unittest.TestCase):
    def test_common_widget_literals_are_marked(self):
        widget = r"new\s+(?:QLabel|QPushButton|QCheckBox|QRadioButton|QAction)"
        method = r"\b(?:setText|setToolTip|setStatusTip|setWhatsThis|setWindowTitle|setPlaceholderText|setAccessibleName|addAction|addMenu|addItem|addTab)"
        pattern = re.compile(
            rf'(?:{widget}|{method})\s*\(\s*(?:QStringLiteral\s*\(\s*)?"((?:\\.|[^"\\])*)"')
        unmarked = []
        for path in (APP_ROOT / "src").rglob("*"):
            if path.suffix not in {".cpp", ".h", ".mm"}:
                continue
            content = path.read_text(encoding="utf-8")
            for match in pattern.finditer(content):
                visible = TAG.sub("", PLACEHOLDER.sub("", match.group(1)))
                if re.search(r"[A-Za-z]{2}", visible):
                    line = content.count("\n", 0, match.start()) + 1
                    unmarked.append(f"{path.relative_to(APP_ROOT)}:{line}: {match.group(1)}")
        self.assertEqual(unmarked, [], "Mark visible text with tr():\n" + "\n".join(unmarked))

    def test_complete_catalogs_preserve_runtime_tokens(self):
        _, english = messages(CATALOGS / "dockpipe_en.ts")
        self.assertGreater(len(english), 2000)
        for language, plural_count in LANGUAGES.items():
            with self.subTest(language=language):
                root, catalog = messages(CATALOGS / f"dockpipe_{language}.ts")
                self.assertEqual(root.get("language"), language)
                self.assertEqual(root.get("sourcelanguage"), "en")
                self.assertEqual(set(catalog), set(english))
                for key, message in catalog.items():
                    source = key[1]
                    translation = message.find("translation")
                    self.assertIsNotNone(translation, (language, key))
                    self.assertNotIn(translation.get("type"), {"unfinished", "vanished", "obsolete"},
                                     (language, key))
                    if message.get("numerus") == "yes":
                        values = [form.text or "" for form in translation.findall("numerusform")]
                        self.assertEqual(len(values), plural_count, (language, key))
                    else:
                        values = [translation.text or ""]
                    for value in values:
                        self.assertTrue(value.strip(), (language, key))
                        self.assertEqual(Counter(PLACEHOLDER.findall(source)),
                                         Counter(PLACEHOLDER.findall(value)), (language, key, value))
                        self.assertEqual(TAG.findall(source), TAG.findall(value), (language, key, value))
                        self.assertEqual(source.count("\n"), value.count("\n"), (language, key, value))
                        source_accelerators = len(ACCELERATOR.findall(source))
                        value_accelerators = len(ACCELERATOR.findall(value))
                        if source_accelerators:
                            self.assertEqual(source_accelerators, value_accelerators, (language, key, value))

    def test_source_extraction_matches_catalogs(self):
        if not LUPDATE:
            self.skipTest("Pass --lupdate with the selected Qt SDK tool to check extraction.")
        with tempfile.TemporaryDirectory(prefix="dockpipe-i18n-check-") as directory:
            extracted = Path(directory) / "extracted.ts"
            process = subprocess.run(
                [LUPDATE, str(APP_ROOT / "src"), "-extensions", "cpp,h,mm", "-no-obsolete",
                 "-locations", "none", "-ts", str(extracted)],
                capture_output=True, text=True, timeout=60, check=False)
            self.assertEqual(process.returncode, 0, process.stderr)
            self.assertNotIn("lacks Q_OBJECT", process.stderr)
            _, current = messages(extracted)
            _, english = messages(CATALOGS / "dockpipe_en.ts")
            self.assertEqual(set(current), set(english),
                             "Refresh and translate catalogs with dockpipe-translations-update.")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--lupdate")
    options, remaining = parser.parse_known_args()
    LUPDATE = options.lupdate
    unittest.main(argv=[__file__, *remaining])
