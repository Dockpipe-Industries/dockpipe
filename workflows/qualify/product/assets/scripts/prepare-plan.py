#!/usr/bin/env python3
"""Prepare pinned Linux portable or installed package qualification suites."""
from pathlib import Path
import argparse
import re

from contracts import digest_file, require, validate_plan, validate_registry, read_json, write_json


OWNER = Path(__file__).resolve().parents[2]


def installed_package_plan(arguments, registry):
    profile = next((row for row in registry["platforms"] if row["id"] == arguments.package_profile), None)
    baseline = profile.get("baseline", {}) if profile is not None else {}
    image = baseline.get("image_digest")
    require(image is not None and "package-upgrade" in profile["selectors"],
            "Select a registered Linux package container profile with an image pin")

    artifacts = (arguments.package_baseline, arguments.package_candidate)
    require(all(path is not None and path.is_absolute() and "," not in str(path) for path in artifacts),
            "Select absolute baseline and candidate package files")
    versions = (arguments.baseline_version, arguments.expected_version)
    require(all(version is not None and re.fullmatch(r"[0-9]{1,8}(?:\.[0-9]{1,8}){2}", version)
                for version in versions), "Select two numeric package versions")
    prior_version, version = versions
    require(tuple(map(int, prior_version.split("."))) < tuple(map(int, version.split("."))),
            "Candidate package version must be newer than the baseline")
    prior_artifact, artifact = artifacts
    prior_digest, candidate_digest = map(digest_file, artifacts)
    require(prior_digest != candidate_digest, "Select distinct baseline and candidate package bytes")

    entrypoint = "drivers/package_upgrade.py"
    dependencies = ("assets/scripts/contracts.py", "assets/scripts/process.py",
                    "drivers/docker.py", "fixtures/package-upgrade/cli.sh")
    driver = {
        "id": "installed-package-suite",
        "entrypoint": entrypoint,
        "sha256": digest_file(OWNER / entrypoint),
        "dependencies": [{"path": path, "sha256": digest_file(OWNER / path)} for path in dependencies],
        "features": ["install.cli", "install.upgrade"],
        "scenarios": ["happy"],
        "layers": ["installed_e2e"],
        "timeout_seconds": 285,
        "configuration": {
            "image": image,
            "endpoint": "unix:///var/run/docker.sock",
            "artifact": str(artifact),
            "version": version,
            "prior_artifact": str(prior_artifact),
            "prior_version": prior_version,
            "prior_artifact_sha256": prior_digest,
        },
    }
    target_id = arguments.package_profile if arguments.target_id is None else arguments.target_id
    return {"schema": "dockpipe.product-integration.plan/v1", "run_id": arguments.run_id,
            "targets": [{"id": target_id,
                         "platform": arguments.package_profile, "kind": "container",
                         "artifact_sha256": candidate_digest, "drivers": [driver]}]}


def git_workspace_driver(artifact, manifest_path):
    require(manifest_path.is_absolute(), "Select an absolute owning Git fixture manifest")
    manifest_sha256 = digest_file(manifest_path)
    manifest = read_json(manifest_path)
    require(isinstance(manifest, dict) and manifest.get("schema") == "dockpipe.product-git-fixture-build/v1"
            and manifest.get("source_overlay_only") is True
            and manifest.get("configuration_fault_tests_passed") is True,
            "Select an owning manual Git fixture producer manifest with passing configuration checks")
    for name in ("binary", "git"):
        require(isinstance(manifest.get(name), str) and isinstance(manifest.get(name + "_sha256"), str),
                "Git fixture executable selections must contain paths and digests")
        selected = Path(manifest[name])
        require(selected.is_absolute() and digest_file(selected) == manifest.get(name + "_sha256"),
                "Selected Git fixture " + name + " differs from its producer pin")
    require(digest_file(manifest_path) == manifest_sha256, "Git fixture manifest changed during plan preparation")
    entrypoint = "drivers/git_workspace.py"
    dependencies = ("assets/scripts/contracts.py", "assets/scripts/process.py",
                    "fixtures/workspace-git/build.py", "fixtures/workspace-git/broker_test.go",
                    "fixtures/workspace-git/config.yml")
    return {"id": "git-workspace", "entrypoint": entrypoint, "sha256": digest_file(OWNER / entrypoint),
            "dependencies": [{"path": path, "sha256": digest_file(OWNER / path)} for path in dependencies],
            "features": ["git.workspace"], "scenarios": ["happy", "failure", "cancellation"],
            "layers": ["installed_e2e", "component"], "timeout_seconds": 150,
            "configuration": {"artifact": str(artifact), "fixture_manifest": str(manifest_path),
                              "fixture_manifest_sha256": manifest_sha256}}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    suite = parser.add_mutually_exclusive_group(required=True)
    suite.add_argument("--portable-artifact", type=Path)
    suite.add_argument("--package-profile", help="Registered Linux package container profile")
    parser.add_argument("--package-baseline", type=Path)
    parser.add_argument("--package-candidate", type=Path)
    parser.add_argument("--baseline-version", help="Exact older CLI version for the package suite")
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--target-id")
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--docker-image", help="Optional already-cached sha256 image ID for the offline container lane")
    parser.add_argument("--docker-detach", action="store_true",
                        help="Qualify real detached lifecycle on the local daemon using --docker-image")
    parser.add_argument("--expected-version", help="Exact CLI version, required for the installed package suite")
    parser.add_argument("--catalog-certificate", type=Path, help="Optional trusted loopback fixture TLS certificate")
    parser.add_argument("--catalog-key", type=Path, help="Private key for the selected loopback fixture certificate")
    parser.add_argument("--sdk-go", type=Path, help="Optional pinned Go inspector for the owning repository's PipeLang SDK binding")
    parser.add_argument("--git-fixture-manifest", type=Path,
                        help="Optional exact manifest from the manual generated Git fixture producer")
    parser.add_argument('--remote-expiry-plan', type=Path, help='Optional fresh explicitly reviewed View-only expiry plan')
    parser.add_argument('--remote-expiry-plan-sha256', help='Exact digest of that private expiry plan')
    parser.add_argument('--remote-expiry-execute-reviewed', action='store_true',
                        help='Explicit live selection; does not replace review of the exact private expiry plan')
    arguments = parser.parse_args()
    require(arguments.output.is_absolute() and not arguments.output.exists(), "Select a fresh absolute private plan file")
    registry = validate_registry(read_json(OWNER / "registry.json"))
    if arguments.package_profile is not None:
        portable_options = (arguments.docker_image, arguments.docker_detach, arguments.catalog_certificate,
                            arguments.catalog_key, arguments.sdk_go, arguments.git_fixture_manifest,
                            arguments.remote_expiry_plan, arguments.remote_expiry_plan_sha256,
                            arguments.remote_expiry_execute_reviewed)
        require(not any(portable_options), "Portable fixture options cannot select an installed package suite")
        write_json(arguments.output, validate_plan(installed_package_plan(arguments, registry), registry))
        print(str(arguments.output))
        return

    require(all(value is None for value in (arguments.package_baseline, arguments.package_candidate,
                                            arguments.baseline_version)),
            "Package inputs require --package-profile")
    require(arguments.portable_artifact.is_absolute(), "Select an absolute portable executable")
    require(not arguments.docker_detach or arguments.docker_image is not None, "Select --docker-image with --docker-detach")
    target_id = "linux-portable" if arguments.target_id is None else arguments.target_id
    entrypoint = "drivers/portable.py"
    dependencies = ["assets/scripts/contracts.py", "assets/scripts/process.py", "drivers/workspace_files.py"]
    plan = {"schema": "dockpipe.product-integration.plan/v1", "run_id": arguments.run_id,
            "targets": [{"id": target_id, "platform": "linux-portable", "kind": "local",
                         "artifact_sha256": digest_file(arguments.portable_artifact), "capabilities": [],
                         "drivers": [{"id": "portable", "entrypoint": entrypoint,
                                      "sha256": digest_file(OWNER / entrypoint),
                                      "dependencies": [{"path": path, "sha256": digest_file(OWNER / path)}
                                                       for path in dependencies],
                                      "features": ["install.cli", "init.scopes", "workflow.discovery",
                                                   "workflow.host", "files.workspace"],
                                      "scenarios": ["happy"], "layers": ["installed_e2e"],
                                      "configuration": {"artifact": str(arguments.portable_artifact)}}]}]}
    if arguments.expected_version is not None:
        plan["targets"][0]["drivers"][0]["configuration"]["expected_version"] = arguments.expected_version
    entrypoint = "drivers/package_authoring.py"
    plan["targets"][0]["drivers"].append({
        "id": "package-authoring", "entrypoint": entrypoint, "sha256": digest_file(OWNER / entrypoint),
        "dependencies": [{"path": path, "sha256": digest_file(OWNER / path)} for path in
                         ("assets/scripts/contracts.py", "assets/scripts/process.py",
                          "drivers/portable.py", "drivers/workspace_files.py")],
        "features": ["packages.authoring"], "scenarios": ["happy", "failure"],
        "layers": ["installed_e2e", "component"],
        "configuration": {"artifact": str(arguments.portable_artifact)}})
    entrypoint = "drivers/lifecycle.py"
    plan["targets"][0]["drivers"].append({
        "id": "observability", "entrypoint": entrypoint, "sha256": digest_file(OWNER / entrypoint),
        "dependencies": [{"path": path, "sha256": digest_file(OWNER / path)} for path in
                         ("assets/scripts/contracts.py", "assets/scripts/process.py")],
        "features": ["observability"], "scenarios": ["happy"], "layers": ["installed_e2e"],
        "timeout_seconds": 120, "configuration": {"artifact": str(arguments.portable_artifact)}})
    entrypoint = "drivers/lifecycle.py"
    plan["targets"][0]["drivers"].append({
        "id": "lifecycle-host", "entrypoint": entrypoint, "sha256": digest_file(OWNER / entrypoint),
        "dependencies": [{"path": path, "sha256": digest_file(OWNER / path)} for path in
                         ("assets/scripts/contracts.py", "assets/scripts/process.py")],
        "features": ["workflow.lifecycle"],
        "scenarios": (["failure", "cancellation", "restart-recovery"] if arguments.docker_detach
                      else ["happy", "failure", "cancellation", "restart-recovery"]),
        "layers": ["installed_e2e", "component"], "timeout_seconds": 120,
        "configuration": {"artifact": str(arguments.portable_artifact)}})
    entrypoint = "drivers/init_failure.py"
    plan["targets"][0]["drivers"].append({
        "id": "init-refusal", "entrypoint": entrypoint, "sha256": digest_file(OWNER / entrypoint),
        "dependencies": [{"path": path, "sha256": digest_file(OWNER / path)} for path in
                         ("assets/scripts/contracts.py", "assets/scripts/process.py")],
        "features": ["init.scopes"], "scenarios": ["failure"], "layers": ["component"],
        "configuration": {"artifact": str(arguments.portable_artifact)}})
    entrypoint = "drivers/host_failure.py"
    plan["targets"][0]["drivers"].append({
        "id": "host-failure", "entrypoint": entrypoint, "sha256": digest_file(OWNER / entrypoint),
        "dependencies": [{"path": path, "sha256": digest_file(OWNER / path)} for path in
                         ("assets/scripts/contracts.py", "assets/scripts/process.py")],
        "features": ["workflow.host"], "scenarios": ["failure"], "layers": ["component"],
        "configuration": {"artifact": str(arguments.portable_artifact)}})
    entrypoint = "drivers/host_cancel.py"
    plan["targets"][0]["drivers"].append({
        "id": "host-cancellation", "entrypoint": entrypoint, "sha256": digest_file(OWNER / entrypoint),
        "dependencies": [{"path": path, "sha256": digest_file(OWNER / path)} for path in
                         ("assets/scripts/contracts.py", "assets/scripts/process.py", "drivers/lifecycle.py")],
        "features": ["workflow.host"], "scenarios": ["cancellation"], "layers": ["component"],
        "configuration": {"artifact": str(arguments.portable_artifact)}})
    plan["targets"][0]["drivers"].append({
        "id": "host-recovery", "entrypoint": entrypoint, "sha256": digest_file(OWNER / entrypoint),
        "dependencies": [{"path": path, "sha256": digest_file(OWNER / path)} for path in
                         ("assets/scripts/contracts.py", "assets/scripts/process.py", "drivers/lifecycle.py")],
        "features": ["workflow.host"], "scenarios": ["restart-recovery"], "layers": ["component"],
        "configuration": {"artifact": str(arguments.portable_artifact)}})
    entrypoint = "drivers/policy_failure.py"
    plan["targets"][0]["drivers"].append({
        "id": "policy-refusal", "entrypoint": entrypoint, "sha256": digest_file(OWNER / entrypoint),
        "dependencies": [{"path": path, "sha256": digest_file(OWNER / path)} for path in
                         ("assets/scripts/contracts.py", "assets/scripts/process.py")],
        "features": ["workflow.containers"], "scenarios": ["failure"], "layers": ["component"],
        "timeout_seconds": 120, "configuration": {"artifact": str(arguments.portable_artifact)}})
    require((arguments.catalog_certificate is None) == (arguments.catalog_key is None),
            "Select both fixture TLS certificate and private key, or neither")
    if arguments.catalog_certificate is not None:
        require(arguments.catalog_certificate.is_absolute() and arguments.catalog_key.is_absolute(),
                "Select absolute fixture TLS file references")
        entrypoint = "drivers/package_lifecycle.py"
        plan["targets"][0]["drivers"].append({
            "id": "package-lifecycle", "entrypoint": entrypoint, "sha256": digest_file(OWNER / entrypoint),
            "dependencies": [{"path": path, "sha256": digest_file(OWNER / path)} for path in
                             ("assets/scripts/contracts.py", "assets/scripts/process.py",
                              "drivers/portable.py", "drivers/workspace_files.py")],
            "features": ["packages.lifecycle"], "scenarios": ["happy", "failure"],
            "layers": ["installed_e2e", "component"],
            "configuration": {"artifact": str(arguments.portable_artifact),
                              "tls_certificate": str(arguments.catalog_certificate),
                              "tls_key": str(arguments.catalog_key),
                              "tls_certificate_sha256": digest_file(arguments.catalog_certificate),
                              "tls_key_sha256": digest_file(arguments.catalog_key)}})
    if arguments.sdk_go is not None:
        require(arguments.sdk_go.is_absolute(), "Select an absolute Go build-info inspector")
        module = "github.com/Dockpipe-Industries/pipelang"
        repository = OWNER.parents[2]
        versions = re.findall(r"(?m)^\s*" + re.escape(module) + r"\s+(\S+)\s*$", (repository / "go.mod").read_text())
        require(len(versions) == 1, "Owning repository must pin one PipeLang module version")
        sums = []
        for line in (repository / "go.sum").read_text().splitlines():
            fields = line.split()
            if len(fields) == 3 and fields[:2] == [module, versions[0]]:
                sums.append(fields[2])
        require(len(sums) == 1, "Owning repository must pin one PipeLang module checksum")
        entrypoint = "drivers/pipelang.py"
        plan["targets"][0]["drivers"].append({
            "id": "pipelang", "entrypoint": entrypoint, "sha256": digest_file(OWNER / entrypoint),
            "dependencies": [{"path": path, "sha256": digest_file(OWNER / path)} for path in
                             ("assets/scripts/contracts.py", "assets/scripts/process.py",
                              "drivers/portable.py", "drivers/workspace_files.py")],
            "features": ["sdk.pipelang"], "scenarios": ["happy", "failure"],
            "layers": ["installed_e2e", "component"],
            "configuration": {"artifact": str(arguments.portable_artifact), "go": str(arguments.sdk_go),
                              "go_sha256": digest_file(arguments.sdk_go), "module": module,
                              "module_version": versions[0], "module_sum": sums[0]}})
    if arguments.git_fixture_manifest is not None:
        plan["targets"][0]["drivers"].append(git_workspace_driver(arguments.portable_artifact,
                                                                 arguments.git_fixture_manifest))
    if arguments.docker_image is not None:
        require(re.fullmatch("sha256:[0-9a-f]{64}", arguments.docker_image), "Select an immutable cached Docker image ID")
        entrypoint = "drivers/docker.py"
        plan["targets"][0]["drivers"].append({
            "id": "docker-offline", "entrypoint": entrypoint, "sha256": digest_file(OWNER / entrypoint),
            "dependencies": [{"path": path, "sha256": digest_file(OWNER / path)}
                             for path in ("assets/scripts/contracts.py", "assets/scripts/process.py")],
            "features": ["workflow.containers"], "scenarios": ["happy"], "layers": ["installed_e2e"],
            "configuration": {"artifact": str(arguments.portable_artifact), "image": arguments.docker_image,
                              "endpoint": "unix:///var/run/docker.sock"}})
    if arguments.docker_detach:
        entrypoint = "drivers/docker_detach.py"
        plan["targets"][0]["drivers"].append({
            "id": "docker-detach", "entrypoint": entrypoint, "sha256": digest_file(OWNER / entrypoint),
            "dependencies": [{"path": path, "sha256": digest_file(OWNER / path)} for path in
                             ("assets/scripts/contracts.py", "assets/scripts/process.py", "drivers/docker.py")],
            "features": ["workflow.lifecycle"], "scenarios": ["happy"], "layers": ["installed_e2e"],
            "timeout_seconds": 110,
            "configuration": {"artifact": str(arguments.portable_artifact), "image": arguments.docker_image,
                              "endpoint": "unix:///var/run/docker.sock"}})
    require((arguments.remote_expiry_plan is None) == (arguments.remote_expiry_plan_sha256 is None),
            'Select both the remote expiry plan and its exact digest')
    require(not arguments.remote_expiry_execute_reviewed or arguments.remote_expiry_plan is not None,
            'Reviewed execution requires an explicitly selected remote expiry plan')
    if arguments.remote_expiry_plan is not None:
        from remote_expiry_plan import prepare
        plan['targets'].append(prepare(arguments.remote_expiry_plan, arguments.remote_expiry_plan_sha256,
                                       arguments.run_id, digest_file(OWNER / 'registry.json'),
                                       arguments.remote_expiry_execute_reviewed))
    validate_plan(plan, registry)
    write_json(arguments.output, plan)
    print(str(arguments.output))


if __name__ == "__main__":
    main()
