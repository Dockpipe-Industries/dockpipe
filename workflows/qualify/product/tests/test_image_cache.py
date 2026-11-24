"""Regression checks for content-addressed snapshots and bounded decompression."""
from hashlib import sha256
import json
from pathlib import Path
import sys
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch


FIXTURE = Path(__file__).resolve().parents[1] / "fixtures/image-cache"
sys.path.insert(0, str(FIXTURE))
from chunk_codecs import gzip_decode, gzip_encode
from prepare import prepare


class SnapshotContracts(unittest.TestCase):
    def test_next_snapshot_reuses_unchanged_chunk(self):
        with TemporaryDirectory() as scratch:
            root = Path(scratch)
            pairs = []
            for name, change in (("baseline", b"B"), ("next", b"C")):
                image = (b"A" * 1024) + (change * 1024)
                base = root / (name + ".qcow2")
                base.write_bytes(image)
                base.chmod(0o400)
                profile = root / (name + ".json")
                profile.write_text(json.dumps({
                    "schema": "dockpipe.qualification-image.v1", "name": name,
                    "image": {"format": "qcow2", "bytes": len(image),
                              "sha256": sha256(image).hexdigest()},
                }))
                pairs.append((profile, base))

            output = root / "snapshots"
            with patch("prepare.CHUNK_BYTES", 1024), patch("prepare.verify_standalone_base"):
                prepare(pairs, output, "gzip")

            baseline = json.loads((output / "baseline.json").read_text())["storage"]["chunks"]
            next_snapshot = json.loads((output / "next.json").read_text())["storage"]["chunks"]
            self.assertEqual(baseline[0]["stored_sha256"], next_snapshot[0]["stored_sha256"])
            self.assertNotEqual(baseline[1]["stored_sha256"], next_snapshot[1]["stored_sha256"])
            self.assertEqual(len(list((output / "chunks").iterdir())), 3)
            for reference in next_snapshot:
                self.assertEqual(reference["codec"], "gzip")
                self.assertLess(reference["stored_bytes"], reference["bytes"])

    def test_gzip_decoder_rejects_output_larger_than_manifest(self):
        with TemporaryDirectory() as scratch:
            part = Path(scratch) / "payload"
            part.write_bytes(gzip_encode(b"A" * 1024))
            with self.assertRaisesRegex(ValueError, "oversized gzip chunk"):
                gzip_decode(part, 256)
            self.assertEqual(gzip_decode(part, 1024), b"A" * 1024)


if __name__ == "__main__":
    unittest.main()
