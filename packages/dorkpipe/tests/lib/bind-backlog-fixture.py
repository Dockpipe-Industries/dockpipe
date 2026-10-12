#!/usr/bin/env python3
"""Bind temporary shell-test fixtures to the current synthetic artifact chain."""
import hashlib
import json
from pathlib import Path
import sys


def fingerprint(value):
    encoded = json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False)
    # Match encoding/json's canonical map encoding and HTML escaping.
    for character in ("&", "<", ">", "\u2028", "\u2029"):
        encoded = encoded.replace(character, f"\\u{ord(character):04x}")
    return "sha256:" + hashlib.sha256(encoded.encode()).hexdigest()


def bind_fixture(artifact_root, fixture_path):
    def read(name):
        return json.loads((artifact_root / name).read_text())

    payload = json.loads(fixture_path.read_text())
    request = read("remote-request.json")
    bindings = {
        "request_fingerprint": request["request_fingerprint"],
        "required_validation_fingerprint": fingerprint({
            "required_validation": request["required_validation"],
        }),
        "validation_inputs_fingerprint": request["validation_input_manifest"]["fingerprint"],
    }
    whole_artifacts = {
        "compatibility_fingerprint": "remote-adapter-compatibility.json",
        "completion_candidate_fingerprint": "completion-candidate.json",
        "remote_status_fingerprint": "remote-status.json",
        "remote_diff_fingerprint": "remote-diff.json",
        "remote_result_fingerprint": "remote-result.json",
        "validation_receipt_fingerprint": "validation-receipt.json",
        "patch_boundary_fingerprint": "patch-boundary.json",
        "patch_application_fingerprint": "patch-application.json",
    }
    stored_fingerprints = {
        "dispatch_fingerprint": ("remote-task.json", "dispatch_fingerprint"),
        "validation_execution_fingerprint": ("validation-execution.json", "artifact_fingerprint"),
        "semantic_review_fingerprint": ("semantic-review-decision.json", "artifact_fingerprint"),
        "readiness_fingerprint": ("ready-for-review.json", "artifact_fingerprint"),
    }
    for field, name in whole_artifacts.items():
        if field in payload:
            bindings[field] = fingerprint(read(name))
    for field, (name, key) in stored_fingerprints.items():
        if field in payload:
            bindings[field] = read(name)[key]
    if "consumer_preimage_fingerprint" in payload:
        bindings["consumer_preimage_fingerprint"] = read("patch-application.json")["preimage_manifest"]["fingerprint"]
    for field in payload:
        if field in bindings:
            payload[field] = bindings[field]
    fixture_path.write_text(json.dumps(payload, indent=2) + "\n")


if __name__ == "__main__":
    bind_fixture(Path(sys.argv[1]), Path(sys.argv[2]))
