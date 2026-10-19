"""Exercise SDK identity, archive safety and publication failures without network."""
import copy
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import subprocess
import unittest
from unittest.mock import patch


SCRIPT = Path(__file__).resolve().parents[1] / "assets/scripts/prepare-forgepipe-sdk.py"
SPEC = importlib.util.spec_from_file_location("cockpit_sdk", SCRIPT)
SDK = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SDK)


def fixture_lock():
    base = "https://github.com/Dockpipe-Industries/dockpipe/releases/download/forgepipe-sdk-v0.1.0-test/"
    return {
        "schema": 1, "version": "0.1.0", "platform": "linux-amd64", "qt_version": "6.10.3",
        "source_repository": SDK.REPOSITORY, "source_sha": "a" * 40, "public_redistribution": True,
        "artifact": {"url": base + "ForgePipe-SDK-0.1.0-Linux.tar.gz", "sha256": "b" * 64, "size_bytes": 100},
        "metadata": {"url": base + "sdk-release.json", "sha256": "c" * 64, "size_bytes": 200},
    }


def write_archive(path, extra=None):
    root = "ForgePipe-SDK-0.1.0-Linux/"
    files = {
        "lib/cmake/ForgePipe/ForgePipeConfig.cmake": b"# verified SDK configuration\n",
        "share/forgepipe/LICENSE": b"license\n",
        "share/forgepipe/NOTICE": b"notice\n",
        "share/forgepipe/SOURCE-PROVENANCE.json": b"{}\n",
    }
    with tarfile.open(path, "w:gz") as archive:
        member = tarfile.TarInfo(root)
        member.type = tarfile.DIRTYPE
        archive.addfile(member)
        for name, payload in files.items():
            member = tarfile.TarInfo(root + name)
            member.size = len(payload)
            archive.addfile(member, io.BytesIO(payload))
        if extra:
            archive.addfile(extra)


class SDKTests(unittest.TestCase):
    def test_attestation_requires_the_independent_release_workflow(self):
        with patch.object(SDK.subprocess, "run") as command:
            SDK.verify_attestation(Path("candidate.tar.gz"))
        arguments = command.call_args.args[0]
        self.assertEqual(arguments, ["gh", "attestation", "verify", "candidate.tar.gz", "--repo",
                                    "Dockpipe-Industries/dockpipe", "--signer-workflow",
                                    "Dockpipe-Industries/dockpipe/.github/workflows/release.yml"])
        self.assertTrue(command.call_args.kwargs["check"])
        with patch.object(SDK.subprocess, "run", side_effect=subprocess.CalledProcessError(1, arguments)):
            with self.assertRaises(subprocess.CalledProcessError):
                SDK.verify_attestation(Path("candidate.tar.gz"))

    def test_lock_rejects_unqualified_source_and_abi(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "lock.json"
            path.write_text(json.dumps(fixture_lock()))
            self.assertEqual(SDK.load_lock(path)["qt_version"], "6.10.3")
            for key, value in (("public_redistribution", False), ("source_sha", None),
                               ("qt_version", "6.10.2"), ("platform", "linux-arm64")):
                lock = fixture_lock()
                lock[key] = value
                path.write_text(json.dumps(lock))
                with self.subTest(key=key), self.assertRaises(ValueError):
                    SDK.load_lock(path)

    def test_metadata_binds_archive_source_and_public_authority(self):
        lock = fixture_lock()
        metadata = {key: value for key, value in lock.items() if key not in ("schema", "artifact", "metadata")}
        metadata["archive"] = {"name": "ForgePipe-SDK-0.1.0-Linux.tar.gz", **lock["artifact"]}
        SDK.verify_metadata(metadata, lock)
        for key, value in (("source_sha", "d" * 40), ("public_redistribution", False), ("qt_version", "6.2.4")):
            changed = copy.deepcopy(metadata)
            changed[key] = value
            with self.subTest(key=key), self.assertRaises(ValueError):
                SDK.verify_metadata(changed, lock)
        metadata["archive"]["sha256"] = "0" * 64
        with self.assertRaises(ValueError):
            SDK.verify_metadata(metadata, lock)

    def test_extracts_payload_and_rejects_escaping_or_special_members(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            archive = root / "sdk.tar.gz"
            write_archive(archive)
            sdk = SDK.unpack(archive, root / "good", "0.1.0")
            self.assertEqual((sdk / "share/forgepipe/LICENSE").read_bytes(), b"license\n")
            for name, kind, link in (("../outside", tarfile.REGTYPE, ""),
                                     ("ForgePipe-SDK-0.1.0-Linux/lib/escape", tarfile.SYMTYPE, "../../outside"),
                                     ("ForgePipe-SDK-0.1.0-Linux/device", tarfile.FIFOTYPE, "")):
                member = tarfile.TarInfo(name)
                member.type = kind
                member.linkname = link
                write_archive(archive, member)
                with self.subTest(name=name), tempfile.TemporaryDirectory(dir=root) as output:
                    with self.assertRaises(ValueError):
                        SDK.unpack(archive, Path(output), "0.1.0")
            self.assertFalse((root / "outside").exists())

    def test_download_checks_exact_size_and_hash(self):
        class Response(io.BytesIO):
            url = "https://release-assets.githubusercontent.com/fixture"

        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            payload = b"pinned SDK bytes"
            pin = {"url": fixture_lock()["artifact"]["url"], "size_bytes": len(payload),
                   "sha256": hashlib.sha256(payload).hexdigest()}
            with patch.object(SDK.urllib.request, "urlopen", return_value=Response(payload)):
                SDK.fetch(pin, root / "good")
            self.assertEqual((root / "good").read_bytes(), payload)
            for name, data in (("truncated", payload[:-1]), ("excess", payload + b"x"), ("corrupt", b"x" * len(payload))):
                with self.subTest(name=name), patch.object(SDK.urllib.request, "urlopen", return_value=Response(data)):
                    with self.assertRaises(ValueError):
                        SDK.fetch(pin, root / name)

    def test_failed_attestation_cleans_staging_without_publishing_sdk(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / "sdk"

            def supply_metadata(pin, destination):
                destination.write_text("{}")

            with patch.object(SDK, "fetch", side_effect=supply_metadata), \
                    patch.object(SDK, "verify_attestation", side_effect=ValueError("untrusted publisher")), \
                    patch.object(SDK.platform, "system", return_value="Linux"), \
                    patch.object(SDK.platform, "machine", return_value="x86_64"):
                with self.assertRaisesRegex(ValueError, "untrusted publisher"):
                    SDK.prepare(fixture_lock(), output)
            self.assertFalse(output.exists())
            self.assertEqual(list(output.parent.iterdir()), [])


if __name__ == "__main__":
    unittest.main()
