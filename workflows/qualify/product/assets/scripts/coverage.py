"""Feature/platform accounting; unavailable and human-pending checks never pass."""
from collections import Counter

from contracts import case_key, require


def applicable(selector, platform):
    return selector == "all" or selector in platform["selectors"]


def expected_cases(registry, target):
    platform = next(value for value in registry["platforms"] if value["id"] == target["platform"])
    result = []
    for feature in registry["features"]:
        if not applicable(feature["selector"], platform):
            continue
        for case in feature["cases"]:
            result.append({"id": feature["id"], "scenario": case["scenario"],
                           "layer": case["layer"], "required_assertions": case["required_assertions"],
                           "critical": feature["critical"], "requires": feature.get("requires", [])})
    return result


def missing_case(expected, reason, status="not_run"):
    return {"id": expected["id"], "scenario": expected["scenario"], "layer": expected["layer"],
            "critical": expected["critical"], "status": status, "reason": reason,
            "assertions": [], "evidence": []}


def target_summary(target, expected, observed):
    require(len(expected) == len({case_key(value) for value in expected}), "Duplicate expected case")
    rows = []
    for case in expected:
        row = dict(observed.get(case_key(case), missing_case(case, "No driver produced this required observation")))
        row["critical"] = case["critical"]
        rows.append(row)
    counts = Counter(value["status"] for value in rows)
    critical = [value for value in rows if value["critical"]]
    critical_passed = sum(value["status"] == "passed" for value in critical)
    passed = counts["passed"]
    total = len(rows)
    coverage = passed / total if total else 0.0
    qualified = total > 0 and coverage >= 0.9 and critical_passed == len(critical) and not counts["failed"]
    layers = {}
    for layer in sorted({value["layer"] for value in rows}):
        selected = [value for value in rows if value["layer"] == layer]
        layers[layer] = {"required": len(selected),
                         "passed": sum(value["status"] == "passed" for value in selected)}
    status = "incomplete"
    if counts["failed"]:
        status = "failed"
    elif qualified:
        status = "qualified"
    return {"target": {key: target[key] for key in ("id", "platform", "kind", "artifact_sha256")},
            "status": status,
            "required": total, "passed": passed, "coverage": coverage,
            "critical_required": len(critical), "critical_passed": critical_passed,
            "counts": dict(counts), "layers": layers, "cases": rows}


def overall_summary(registry, summaries):
    required = sum(value["required"] for value in summaries)
    passed = sum(value["passed"] for value in summaries)
    selected_profiles = {value["target"]["platform"] for value in summaries}
    missing_profiles = [value["id"] for value in registry["platforms"]
                        if value["id"] not in selected_profiles]
    selected_qualified = bool(summaries) and all(value["status"] == "qualified" for value in summaries)
    return {"selected_targets_qualified": selected_qualified,
            "full_product_qualified": selected_qualified and not missing_profiles,
            "selected_case_coverage": passed / required if required else 0.0,
            "required": required, "passed": passed,
            "unconfigured_platform_profiles": missing_profiles,
            "note": "Selected-target coverage is not full-product or bug-detection coverage. "
                    "Component, installed, guided and human layers are reported separately."}
