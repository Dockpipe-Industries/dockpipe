"""Check prebuilt identity and release publication boundaries without a registry."""

import hashlib
import importlib.util
import json
from pathlib import Path
import struct
import tempfile
import unittest
from unittest import mock
import yaml


ROOT = Path(__file__).resolve().parents[4]
SCRIPT = ROOT / "workflows/package/synology-desktop-spk/assets/scripts/prebuild.py"
SPECIFICATION = importlib.util.spec_from_file_location("synology_prebuild", SCRIPT)
PREBUILD = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(PREBUILD)


class PrebuiltImageTests(unittest.TestCase):
    def test_host_network_deployments_do_not_publish_or_mount_host_control_sockets(self):
        assets = ROOT / 'workflows/package/synology-desktop-spk/assets/desktop'
        for name in ('compose.yml', 'compose-prebuilt.yml'):
            with self.subTest(name=name):
                rendered = (assets / name).read_text().replace('@PULL_POLICY@', 'never')
                service = yaml.safe_load(rendered)['services']['dockpipe']
                self.assertEqual(service['network_mode'], 'host')
                self.assertNotIn('ports', service)
                self.assertNotIn('privileged', service)
                self.assertNotIn('pid', service)
                self.assertFalse(any('docker.sock' in volume for volume in service['volumes']))
                self.assertFalse(any('/etc' in volume or '@appconf' in volume for volume in service['volumes']))

    def test_invalid_native_gateway_fails_before_docker_or_output_creation(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            binaries = {}
            for architecture in ("amd64", "arm64"):
                content = bytearray(120)
                content[:7] = b"\x7fELF\x02\x01\x01"
                machine = PREBUILD.PACKAGING.CLI_PACKAGING.ARCHITECTURES[architecture][2]
                struct.pack_into("<HH", content, 16, 2, machine)
                struct.pack_into("<Q", content, 32, 64)
                struct.pack_into("<HH", content, 54, 56, 1)
                struct.pack_into("<I", content, 64, 1)
                binary = directory / architecture
                binary.write_bytes(content)
                binaries[architecture] = binary
            output = directory / "output"
            images = directory / "images"
            metadata = (directory / "cli.deb", directory / "desktop.deb", "1.2.3")
            with mock.patch.object(PREBUILD.PACKAGING.DESKTOP_PACKAGING, "validate_inputs", return_value=metadata), \
                    mock.patch.object(PREBUILD, "command") as execute:
                with self.assertRaisesRegex(ValueError, "architecture"):
                    PREBUILD.prebuild("1.2.3", "amd64", "a" * 40, "ghcr.io/example/desktop",
                                      binaries["amd64"], metadata[0], metadata[1], output, images,
                                      native_host_binary=binaries["arm64"])
                execute.assert_not_called()
            self.assertFalse(output.exists())
            self.assertFalse(images.exists())

    def test_embedded_container_probe_is_valid_python(self):
        compile(PREBUILD.QUALIFIER.PROBE, "desktop-startup-probe", "exec")

    def test_manifest_digest_is_over_exact_registry_manifest_bytes(self):
        config = "sha256:" + "a" * 64
        raw = json.dumps({"schemaVersion": 2, "config": {"digest": config}}).encode()
        digest, image_id = PREBUILD.manifest_identity(raw)
        self.assertEqual(digest, "sha256:" + hashlib.sha256(raw).hexdigest())
        self.assertEqual(image_id, config)

    def test_manifest_rejects_indexes_invalid_config_and_oversized_metadata(self):
        for raw in (b'[]', b'{"manifests":[]}', b'{"config":{"digest":"latest"}}', b' ' * (4 * 1024**2 + 1)):
            with self.subTest(raw=raw[:60]), self.assertRaises(ValueError):
                PREBUILD.manifest_identity(raw)

    def test_registry_writes_belong_to_publication_jobs_and_dry_runs_keep_artifacts(self):
        workflow = yaml.safe_load((ROOT / ".github/workflows/release.yml").read_text())
        jobs = workflow["jobs"]
        for name, job in jobs.items():
            if job.get("permissions", {}).get("packages") == "write":
                self.assertIn(name, {"publish", "publish-staging"})
        build = jobs["build-unix"]
        upload = next(step for step in build["steps"] if step.get("name") == "Retain verified Synology images for publication")
        self.assertTrue(upload["with"]["name"].startswith("synology-image-"))
        for name in ("publish", "publish-staging"):
            steps = jobs[name]["steps"]
            names = [step.get("name") for step in steps]
            self.assertLess(names.index("Publish exact desktop images and verify public access"), names.index("Create GitHub release"))
            self.assertIn("needs.meta.outputs.dry_run == 'false'", jobs[name]["if"])
        self.assertNotIn("packages", build["permissions"])
        package = next(step for step in build["steps"] if step.get("name") == "Package Synology desktop editions")
        command = package["run"]
        self.assertIn('CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build', command)
        self.assertIn('./workflows/package/synology-desktop-spk/assets/native-host', command)
        self.assertIn('--native-host-binary "$native_host_binary"', command)
        self.assertLess(command.index('go build'), command.index('synology-desktop-spk/prebuild.yml'))


if __name__ == "__main__":
    unittest.main()
