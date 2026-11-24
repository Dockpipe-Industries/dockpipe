"""Expand installed remote qualification by role and by endpoint combination."""

from collections import Counter
import hashlib
from itertools import product

from contracts import DIGEST, IDENTIFIER, contained_file, digest_file, matches, read_json_document, require


ROLES = {
    "operator", "broker", "worker", "background-worker", "local-browser",
    "browser-viewer", "desktop-viewer", "desktop-sender", "rdp-viewer", "rdp-sender",
}
STATUSES = ("passed", "failed", "blocked", "unrun", "unsupported")
MANUAL_SERVER = "windows-server-2025-amd64"
OBSERVATION_SCHEMA = "dockpipe.product-integration.remote-observations/v1"
MAX_OBSERVATIONS = 8192
X11_REQUIRED = "Use an installed Linux GUI on an admitted X11 (xcb) display."
MANUAL_REQUIRED = "Windows Server requires the operator-provided VM and installed product."


def validate_remote_qualification(registry):
    configuration = registry.get("remote_qualification")
    if configuration is None:
        return

    require(isinstance(configuration, dict)
            and configuration.get("schema") == "dockpipe.product-integration.remote-qualification/v1",
            "Unknown remote qualification schema")
    features = configuration.get("features")
    require(isinstance(features, dict), "Remote qualification needs feature role definitions")
    expected = {feature["id"] for feature in registry["features"]
                if feature["id"].startswith("remote.") or feature["id"] == "browser.local"}
    expected.update(row["feature"] for row in registry["unsupported"] if row["feature"].startswith("remote."))
    require(set(features) == expected, "Remote qualification must cover the complete remote and Browser inventory")
    require(registry["platforms"][-1]["id"] == MANUAL_SERVER, "Windows Server must be the final platform profile")

    for feature, definition in features.items():
        require(isinstance(definition, dict), f"Invalid remote feature definition for {feature}")
        roles = definition.get("roles")
        services = definition.get("service_roles", [])
        assertions = definition.get("additional_assertions")
        service_assertions = definition.get("service_assertions", {})
        require(isinstance(roles, list) and 1 <= len(roles) <= 2 and isinstance(services, list)
                and len(services) <= 2 and all(isinstance(role, str) and role in ROLES for role in roles + services)
                and len(set(roles + services)) == len(roles + services), f"Invalid roles for {feature}")
        require(isinstance(assertions, list) and len(assertions) <= 64
                and all(matches(IDENTIFIER, assertion) for assertion in assertions)
                and len(set(assertions)) == len(assertions), f"Invalid installed assertions for {feature}")
        require(isinstance(service_assertions, dict) and set(service_assertions) <= set(services),
                f"Invalid service assertion roles for {feature}")
        for role, checks in service_assertions.items():
            require(isinstance(checks, list) and 1 <= len(checks) <= 64
                    and all(matches(IDENTIFIER, check) for check in checks)
                    and len(set(checks)) == len(checks), f"Invalid assertions for {feature} service {role}")


def role_availability(platform, role):
    identifier = platform["id"]
    selectors = set(platform["selectors"])
    desktop = "desktop" in selectors
    windows = identifier.startswith("windows-")
    darwin = identifier.startswith("darwin-")
    baseline = platform.get("baseline", {})
    backend = baseline.get("display_backend")
    x11 = desktop and not windows and not darwin and backend != "wayland"
    container = baseline.get("environment") == "container"

    if role in {"worker", "background-worker"}:
        supported = "all-cli" in selectors
    elif role == "operator":
        supported = desktop
    elif role == "local-browser":
        supported = desktop
    elif role == "broker":
        supported = not windows
    elif role in {"browser-viewer", "desktop-viewer"}:
        supported = x11
    elif role == "desktop-sender":
        supported = "darwin-sender" in selectors
    else:
        supported = False

    if not supported:
        if backend == "wayland" and role in {"browser-viewer", "desktop-viewer"}:
            return "unsupported", "The pinned desktop profile uses Wayland. " + X11_REQUIRED
        return "unsupported", "This platform profile does not provide the selected role."
    if identifier == MANUAL_SERVER:
        return "blocked", MANUAL_REQUIRED
    if container and role in {"broker", "background-worker"}:
        return "blocked", "This container fixture has no admitted native login and service session."
    if role in {"browser-viewer", "desktop-viewer"}:
        return "unrun", X11_REQUIRED
    if windows and role == "operator":
        return "blocked", "Native Windows broker authority fails closed. Establish a supported operator path before running."
    return "unrun", ""


def lane_key(feature, participants):
    return feature, tuple(sorted(participants.items()))


def feature_assertions(registry, feature, definition):
    existing = next((entry for entry in registry["features"] if entry["id"] == feature), None)
    required = []
    if existing is not None:
        for case in existing["cases"]:
            if case["layer"] == "installed_e2e":
                required.extend(case["required_assertions"])
    return list(dict.fromkeys(required + definition["additional_assertions"]))


def observation_index(paths):
    require(len(paths) <= MAX_OBSERVATIONS, "Too many remote observation documents")
    indexed = {}
    digests = {}
    for path in paths:
        document, data = read_json_document(path)
        require(isinstance(document, dict) and document.get("schema") == OBSERVATION_SCHEMA
                and isinstance(document.get("observations"), list), "Invalid remote observations document")
        observations = document["observations"]
        require(len(indexed) + len(observations) <= MAX_OBSERVATIONS, "Too many remote observations")
        document_digest = hashlib.sha256(data).hexdigest()
        index_document(indexed, digests, observations, path.parent, document_digest)

    return indexed


def index_document(indexed, digests, observations, evidence_root, document_digest):
    for record in observations:
        require(isinstance(record, dict), "Remote observation must be an object")
        feature = record.get("feature")
        participants = record.get("participants")
        status = record.get("status")
        require(isinstance(feature, str) and isinstance(participants, dict)
                and 1 <= len(participants) <= 2
                and all(role in ROLES and isinstance(value, str) for role, value in participants.items())
                and status in STATUSES[:-1], "Invalid remote observation feature, roles, or status")
        require(set(record) <= {"feature", "participants", "status", "reason", "layer",
                                "artifact_sha256", "evidence", "passed_assertions"},
                "Unknown fields in remote observation")

        key = lane_key(feature, participants)
        require(key not in indexed, "Duplicate remote feature and participant observation")
        if status in {"passed", "failed"} or "passed_assertions" in record:
            artifacts = record.get("artifact_sha256")
            require(record.get("layer") == "installed_e2e" and isinstance(artifacts, dict)
                    and set(artifacts) == set(participants)
                    and all(matches(DIGEST, value) for value in artifacts.values())
                    and "evidence" in record,
                    "Observed remote status needs installed artifact identities and evidence")
        if "evidence" in record:
            evidence = record["evidence"]
            require(isinstance(evidence, list) and 1 <= len(evidence) <= 64,
                    "Remote evidence must contain one to 64 files")
            for item in evidence:
                require(isinstance(item, dict) and matches(DIGEST, item.get("sha256")),
                        "Invalid remote evidence digest")
                path = contained_file(evidence_root, item.get("path"))
                if path not in digests:
                    digests[path] = digest_file(path)
                require(digests[path] == item["sha256"], "Remote evidence SHA-256 mismatch")
        if status in {"failed", "blocked"}:
            require(isinstance(record.get("reason"), str) and record["reason"].strip(),
                    "Failed and blocked remote observations need a reason")
        indexed[key] = record, document_digest


def remote_matrix(registry, observation_paths=()):
    validate_remote_qualification(registry)
    configuration = registry.get("remote_qualification")
    require(configuration is not None, "Registry has no remote qualification inventory")
    indexed = observation_index(observation_paths)
    roles = {}
    role_rows = []
    for platform in registry["platforms"]:
        identifier = platform["id"]
        for role in sorted(ROLES):
            status, reason = role_availability(platform, role)
            row = {"platform": identifier, "role": role, "status": status}
            if reason:
                row["reason"] = reason
            role_rows.append(row)
            if status != "unsupported":
                roles.setdefault(role, []).append(row)

    lanes = []
    unsupported = []
    for feature, definition in configuration["features"].items():
        common_assertions = feature_assertions(registry, feature, definition)
        service_assertions = definition.get("service_assertions", {})
        combinations = [definition["roles"]] + [[role] for role in definition.get("service_roles", [])]
        feature_roles = set(definition["roles"] + definition.get("service_roles", []))
        unsupported.extend({"feature": feature, **row} for row in role_rows
                           if row["role"] in feature_roles and row["status"] == "unsupported")
        for combination in combinations:
            additional = service_assertions.get(combination[0], []) if len(combination) == 1 else []
            required = list(dict.fromkeys(common_assertions + additional))
            for endpoints in product(*(roles.get(role, ()) for role in combination)):
                participants = {endpoint["role"]: endpoint["platform"] for endpoint in endpoints}
                key = lane_key(feature, participants)
                blockers = list(dict.fromkeys(endpoint["reason"] for endpoint in endpoints
                                              if endpoint["status"] == "blocked"))
                default_status = "blocked" if blockers else "unrun"
                lane = {"feature": feature, "participants": participants, "status": default_status,
                        "required_assertions": required}
                if blockers:
                    lane["reason"] = " ".join(blockers)

                observation = indexed.pop(key, None)
                if observation is not None:
                    record, document_digest = observation
                    passed = record.get("passed_assertions", [])
                    require(isinstance(passed, list) and all(isinstance(item, str) for item in passed)
                            and len(passed) == len(set(passed)) and set(passed) <= set(required),
                            "Remote assertions must belong to the exact combination")
                    if record["status"] == "passed":
                        require(set(passed) == set(required),
                                "Remote pass must cover all required assertions for the exact roles")
                    lane.pop("reason", None)
                    lane.update(record)
                    lane["observation_sha256"] = document_digest
                lanes.append(lane)

    require(not indexed, "Remote observations include unsupported or unknown feature-role combinations")
    lanes.sort(key=lambda row: MANUAL_SERVER in row["participants"].values())
    unsupported.sort(key=lambda row: row["platform"] == MANUAL_SERVER)
    counts = Counter(row["status"] for row in lanes)
    counts["unsupported"] = len(unsupported)
    return {"schema": "dockpipe.product-integration.remote-matrix/v1",
            "counts": {status: counts[status] for status in STATUSES},
            "roles": role_rows, "lanes": lanes, "unsupported": unsupported}
