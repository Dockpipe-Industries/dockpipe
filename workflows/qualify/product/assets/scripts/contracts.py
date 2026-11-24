"""Strict, bounded inputs and evidence admission for local product qualification."""
from pathlib import Path, PurePosixPath
import hashlib
import json
import os
import re
import stat


MAX_JSON_BYTES = 4 * 1024 * 1024
MAX_EVIDENCE_BYTES = 512 * 1024 * 1024
DIGEST = re.compile(r"[0-9a-f]{64}\Z")
IDENTIFIER = re.compile(r"[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}\Z")
STATUSES = {"passed", "failed", "blocked", "not_run", "manual_pending"}
LAYERS = {"component", "installed_e2e", "guided_ui", "human"}
SCENARIOS = {"happy", "failure", "cancellation", "restart-recovery"}
SELECTORS = {"all", "all-cli", "docker-capable", "desktop-provider", "native-desktop",
             "desktop", "editor-host", "linux-x11", "linux-viewer", "linux-to-darwin",
             "darwin-sender", "contained", "vm-host", "package-upgrade"}
BASELINE_KINDS = {"vm": "vm", "container": "container", "wsl2": "local", "manual-vm": "vm"}


class ContractError(ValueError):
    pass


def require(condition, message):
    if not condition:
        raise ContractError(message)


def matches(pattern, value):
    return isinstance(value, str) and pattern.fullmatch(value) is not None


def read_json_document(path):
    path = Path(path)
    information = path.lstat()
    require(stat.S_ISREG(information.st_mode) and information.st_size <= MAX_JSON_BYTES,
            "Expected a bounded regular JSON input: " + str(path))
    flags = os.O_RDONLY | getattr(os, "O_BINARY", 0) | getattr(os, "O_NOFOLLOW", 0) | getattr(os, "O_NONBLOCK", 0)
    descriptor = os.open(path, flags)
    with os.fdopen(descriptor, "rb") as source:
        opened = os.fstat(source.fileno())
        require(stat.S_ISREG(opened.st_mode) and os.path.samestat(information, opened),
                "JSON input identity changed before reading")
        data = source.read(MAX_JSON_BYTES + 1)
        after = os.fstat(source.fileno())
    require(len(data) <= MAX_JSON_BYTES, "JSON input exceeds its byte bound")
    require((opened.st_size, opened.st_mtime_ns) == (after.st_size, after.st_mtime_ns),
            "JSON input changed while reading")

    def unique_members(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, "Duplicate JSON member: " + key)
            result[key] = value
        return result

    def reject_constant(value):
        raise ContractError("Non-JSON numeric literal: " + value)

    value = json.loads(data, object_pairs_hook=unique_members, parse_constant=reject_constant)
    return value, data


def read_json(path):
    value, _data = read_json_document(path)
    return value


def digest_file(path, maximum=MAX_EVIDENCE_BYTES):
    path = Path(path)
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_size <= maximum,
            "Expected a bounded regular evidence file: " + str(path))
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    after = path.lstat()
    require((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns)
            == (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns),
            "Evidence changed while hashing: " + str(path))
    return digest.hexdigest()


def contained_file(root, relative):
    require(isinstance(relative, str) and "\\" not in relative,
            "Evidence paths must use relative forward-slash components")
    member = PurePosixPath(relative)
    require(relative and not member.is_absolute() and ".." not in member.parts
            and str(member) == relative, "Noncanonical evidence path")
    root = Path(root).resolve(strict=True)
    selected = root.joinpath(*member.parts)
    resolved = selected.resolve(strict=True)
    require(resolved.is_relative_to(root) and resolved == selected,
            "Evidence must remain within its owned output without links")
    require(selected.is_file(), "Evidence file is unavailable")
    return selected


def write_json(path, value):
    path = Path(path)
    data = (json.dumps(value, indent=2, sort_keys=True, allow_nan=False) + "\n").encode()
    require(len(data) <= MAX_JSON_BYTES, "Result metadata exceeds its bound")
    with path.open("xb") as output:
        path.chmod(0o600)
        output.write(data)


def case_key(value):
    return value["id"], value["scenario"], value["layer"]


def validate_registry(registry):
    require(isinstance(registry, dict), "Registry must be an object")
    require(registry.get("schema") == "dockpipe.product-integration.registry/v1",
            "Unknown product registry schema")
    features = registry.get("features")
    require(isinstance(features, list) and 1 <= len(features) <= 512,
            "Registry feature count is invalid")
    feature_ids = set()
    platforms = registry.get("platforms")
    require(isinstance(platforms, list) and 1 <= len(platforms) <= 64, "Registry platform count is invalid")
    platform_ids = set()
    for platform in platforms:
        require(isinstance(platform, dict) and isinstance(platform.get("id"), str)
                and 1 <= len(platform["id"]) <= 128 and platform["id"] not in platform_ids
                and platform.get("group") in {"native-desktop", "contained", "cli-only"},
                "Invalid or duplicate platform profile")
        platform_ids.add(platform["id"])
        selectors = platform.get("selectors")
        require(isinstance(selectors, list) and selectors
                and all(isinstance(value, str) and value in SELECTORS for value in selectors)
                and len(selectors) == len(set(selectors)), "Platform profile needs explicit unique selectors")

        baseline = platform.get("baseline")
        if isinstance(baseline, dict) and "display_backend" in baseline:
            require("desktop" in selectors and baseline["display_backend"] in ("x11", "wayland"),
                    "Display backend must be x11 or wayland on a desktop profile")
        if isinstance(baseline, dict) and "image_digest" in baseline:
            image = baseline["image_digest"]
            require(baseline.get("environment") == "container" and isinstance(image, str)
                    and image.startswith("sha256:") and matches(DIGEST, image.removeprefix("sha256:")),
                    "Container image pin must be a full SHA-256 identity")

        if "package-upgrade" in selectors:
            require(isinstance(baseline, dict), "Upgrade profile needs a platform baseline")
            fields = ["os", "release", "architecture", "environment"]
            if "desktop" in selectors:
                fields.append("desktop")
            if baseline.get("environment") == "wsl2":
                fields.append("host")
            require(all(isinstance(baseline.get(field), str)
                        and 1 <= len(baseline[field].strip()) <= 128 for field in fields),
                    "Platform baseline fields must be nonempty bounded strings")
            require(baseline["architecture"] in {"amd64", "arm64"}
                    and baseline["environment"] in BASELINE_KINDS,
                    "Invalid baseline architecture or environment")
    for feature in features:
        require(isinstance(feature, dict), "Feature must be an object")
        identifier = feature.get("id", "")
        require(matches(IDENTIFIER, identifier) and identifier not in feature_ids,
                "Registry feature identity is invalid or duplicated")
        feature_ids.add(identifier)
        require(feature.get("selector") in SELECTORS, "Unknown platform selector")
        requirements = feature.get("requires", [])
        require(isinstance(requirements, list) and len(requirements) <= 64
                and all(matches(IDENTIFIER, value) for value in requirements), "Invalid feature capabilities")
        require(type(feature.get("critical")) is bool, "Critical classification must be explicit")
        require(isinstance(feature.get("cases"), list) and feature["cases"],
                "Every feature needs explicit scenarios")
        seen = set()
        for case in feature["cases"]:
            require(isinstance(case, dict) and isinstance(case.get("scenario"), str)
                    and isinstance(case.get("layer"), str), "Registry case must declare string scenario and layer")
            key = case.get("scenario"), case.get("layer")
            require(key not in seen and case.get("layer") in LAYERS and case.get("scenario") in SCENARIOS,
                    "Registry scenario/layer is invalid or duplicated")
            seen.add(key)
            assertions = case.get("required_assertions")
            require(isinstance(assertions, list) and assertions
                    and all(isinstance(value, str) and IDENTIFIER.fullmatch(value) for value in assertions)
                    and len(assertions) == len(set(assertions)),
                    "Every case needs unique, named assertions")
    policy = registry.get("policy", {})
    require(isinstance(policy, dict), "Registry policy must be an object")
    require(policy.get("local_only") is True and policy.get("minimum_coverage") == 0.9
            and policy.get("critical_coverage") == 1.0
            and policy.get("unavailable_counts_as_unpassed") is True,
            "Qualification policy must retain the declared coverage thresholds")
    return registry


def validate_plan(plan, registry):
    require(isinstance(plan, dict), "Run plan must be an object")
    require(plan.get("schema") == "dockpipe.product-integration.plan/v1", "Unknown run plan schema")
    require(matches(IDENTIFIER, plan.get("run_id")), "A stable run_id is required")
    targets = plan.get("targets")
    require(isinstance(targets, list) and 1 <= len(targets) <= 64, "Explicit selected targets are required")
    profiles = {value["id"]: value for value in registry["platforms"]}
    identifiers = set()
    for target in targets:
        require(isinstance(target, dict), "Target must be an object")
        require(matches(IDENTIFIER, target.get("id")) and target["id"] not in identifiers,
                "Target identity is invalid or duplicated")
        identifiers.add(target["id"])
        require(target.get("platform") in profiles, "Target platform is absent from the registry")
        require(target.get("kind") in {"local", "vm", "remote", "flatpak", "container"},
                "Target execution kind must be explicit")
        profile = profiles[target["platform"]]
        if "package-upgrade" in profile["selectors"]:
            require(target["kind"] == BASELINE_KINDS[profile["baseline"]["environment"]],
                    "Target execution kind differs from its pinned platform baseline")
        require(matches(DIGEST, target.get("artifact_sha256")),
                "Pin each selected installed product artifact")
        capabilities = target.get("capabilities", [])
        require(isinstance(capabilities, list) and len(capabilities) <= 64
                and all(matches(IDENTIFIER, value) for value in capabilities)
                and len(capabilities) == len(set(capabilities)),
                "Capabilities must be a bounded list of unique names")
        require(isinstance(target.get("drivers", []), list) and len(target.get("drivers", [])) <= 64,
                "Driver selection exceeds its bound")
        driver_ids = set()
        feature_ids = {feature["id"] for feature in registry["features"]}
        for driver in target.get("drivers", []):
            require(isinstance(driver, dict), "Driver must be an object")
            require(matches(IDENTIFIER, driver.get("id")) and driver["id"] not in driver_ids,
                    "Invalid or duplicate driver identity")
            driver_ids.add(driver["id"])
            features = driver.get("features")
            require(isinstance(features, list) and features
                    and all(isinstance(value, str) and value in feature_ids for value in features)
                    and len(features) == len(set(features)),
                    "Drivers must declare their feature ownership")
            require(matches(DIGEST, driver.get("sha256")), "Driver source must be pinned")
            require(isinstance(driver.get("configuration", {}), dict), "Driver configuration must be an object")
            for key in ("scenarios", "layers"):
                if key not in driver:
                    continue
                values = driver[key]
                require(isinstance(values, list) and values
                        and all(matches(IDENTIFIER, value) for value in values)
                        and len(values) == len(set(values)), "Invalid driver " + key)
            require(all(value in LAYERS for value in driver.get("layers", [])), "Unknown driver layer")
            require(all(value in SCENARIOS for value in driver.get("scenarios", [])), "Unknown driver scenario")
            dependencies = driver.get("dependencies", [])
            require(isinstance(dependencies, list) and len(dependencies) <= 64,
                    "Driver dependency count exceeds its bound")
            dependency_paths = set()
            for dependency in dependencies:
                require(isinstance(dependency, dict)
                        and set(dependency) == {"path", "sha256"}
                        and isinstance(dependency["path"], str)
                        and dependency["path"] not in dependency_paths
                        and matches(DIGEST, dependency["sha256"]), "Invalid driver dependency pin")
                dependency_paths.add(dependency["path"])
            timeout = driver.get("timeout_seconds", 1200)
            require(type(timeout) is int and 1 <= timeout <= 14400, "Driver deadline exceeds four hours")
    return plan


def validate_receipt(receipt, request, output):
    require(isinstance(receipt, dict), "Lane result must be an object")
    require(receipt.get("schema") == "dockpipe.product-integration.lane/v1", "Unknown lane result schema")
    for key in ("run_id", "plan_sha256", "registry_sha256", "target"):
        require(receipt.get(key) == request[key], "Lane result changed its captured " + key)
    requested = {case_key(value): value for value in request["cases"]}
    cases = receipt.get("cases")
    require(isinstance(cases, list) and len(cases) <= len(requested), "Unexpected lane result count")
    observed = {}
    for case in cases:
        require(isinstance(case, dict) and all(isinstance(case.get(key), str) for key in ("id", "scenario", "layer")),
                "Lane case must declare string identity, scenario and layer")
        key = case_key(case)
        require(key in requested and key not in observed, "Unrequested or duplicate case result")
        require(isinstance(case.get("status"), str) and case["status"] in STATUSES, "Unknown case status")
        require(isinstance(case.get("reason", ""), str) and len(case.get("reason", "")) <= 4096,
                "Case reason exceeds its bound")
        expected_assertions = set(requested[key]["required_assertions"])
        assertions = case.get("assertions", [])
        require(isinstance(assertions, list) and len(assertions) <= len(expected_assertions),
                "Unexpected assertion count")
        actual_assertions = {}
        for assertion in assertions:
            require(isinstance(assertion, dict) and isinstance(assertion.get("id"), str),
                    "Assertion must declare a string identity")
            identifier = assertion.get("id")
            require(identifier in expected_assertions and identifier not in actual_assertions,
                    "Unrequested or duplicate assertion")
            require(isinstance(assertion.get("status"), str) and assertion["status"] in {"passed", "failed"}, "Invalid assertion result")
            require(isinstance(assertion.get("detail", ""), str)
                    and len(assertion.get("detail", "")) <= 4096, "Assertion detail exceeds its bound")
            actual_assertions[identifier] = assertion["status"]
        evidence = case.get("evidence", [])
        require(isinstance(evidence, list) and len(evidence) <= 64, "Evidence count exceeds its bound")
        for item in evidence:
            require(isinstance(item, dict), "Evidence entry must be an object")
            require(matches(DIGEST, item.get("sha256")), "Invalid evidence digest")
            path = contained_file(output, item.get("path"))
            require(digest_file(path) == item["sha256"], "Evidence digest mismatch")
        if case["status"] == "passed":
            require(evidence and set(actual_assertions) == expected_assertions
                    and all(value == "passed" for value in actual_assertions.values()),
                    "A pass needs every named assertion and actual hashed evidence")
        observed[key] = case
    return observed
