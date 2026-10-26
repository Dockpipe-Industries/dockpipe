"""Protect the reviewed publication body and its immutable media references."""
import importlib.util
from pathlib import Path
import tempfile
import unittest


SPEC = importlib.util.spec_from_file_location("prepare_article", Path(__file__).resolve().parents[4] / 'workflows/publish/release-article/assets/scripts/prepare-article.py')
ARTICLE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(ARTICLE)


class ArticleTests(unittest.TestCase):
    def test_approved_body_only_changes_publication_links(self):
        path = Path(__file__).resolve().parents[4] / 'release/articles/announcement.md'
        source = path.read_text()
        sha = "a" * 40
        prepared = ARTICLE.prepare(path, "Dockpipe-Industries/dockpipe", sha, "v0.6.7")
        self.assertEqual(prepared["title"], "Why we’re building Dockpipe")
        restored = prepared["body_markdown"].replace(
            f"https://raw.githubusercontent.com/Dockpipe-Industries/dockpipe/{sha}/release/articles/assets/",
            "assets/",
        ).replace("releases/tag/v0.6.7", "releases/latest")
        self.assertEqual(restored, source.split("\n", 1)[1].strip())
        self.assertIn(sha, prepared["main_image"])

    def test_rejects_mutable_source_and_staging_tag(self):
        path = Path(__file__).resolve().parents[4] / 'release/articles/announcement.md'
        for sha, tag in [("master", "v0.6.7"), ("a" * 40, "v0.6.7-staging.123")]:
            with self.subTest(sha=sha, tag=tag), self.assertRaises(ValueError):
                ARTICLE.prepare(path, "Dockpipe-Industries/dockpipe", sha, tag)

    def test_missing_or_external_image_stops_publication(self):
        with tempfile.TemporaryDirectory() as directory:
            article = Path(directory) / "article.md"
            for image in ["assets/missing.png", "../private.png", "https://example.com/image.png"]:
                article.write_text(f"# Reviewed title\n\n![image]({image})\n")
                with self.subTest(image=image), self.assertRaises(ValueError):
                    ARTICLE.prepare(article, "Dockpipe-Industries/dockpipe", "a" * 40, "v0.6.7")


if __name__ == "__main__":
    unittest.main()
