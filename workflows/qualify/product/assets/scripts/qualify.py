#!/usr/bin/env python3
"""Run explicit local product lanes and report honest per-platform coverage."""
from pathlib import Path
import argparse
import datetime
import hashlib
import json
import os
import signal
import sys
import tempfile

from contracts import (ContractError, case_key, contained_file, digest_file, read_json, read_json_document,
                       require, validate_plan, validate_receipt, validate_registry, write_json)
from coverage import expected_cases, missing_case, overall_summary, target_summary
from process import run_driver
from remote_matrix import remote_matrix
from remote_work_plan import remote_work_plan


OWNER = Path(__file__).resolve().parents[2]
REPOSITORY = OWNER.parents[2]
MAX_RUN_CASES = 8192
MAX_RESULT_METADATA_BYTES = 32 * 1024 * 1024


def timestamp():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def interrupt_run(_number, _frame):
    raise KeyboardInterrupt


def replace_report(path, value):
    """Replace only this invocation's report after a fully written private snapshot."""
    data = json.dumps(value, indent=2, sort_keys=True, allow_nan=False) + "\n"
    require(len(data.encode()) <= MAX_RESULT_METADATA_BYTES, "Qualification report exceeds its 32 MiB metadata bound")
    descriptor, temporary = tempfile.mkstemp(prefix=".report-", dir=path.parent)
    try:
        with os.fdopen(descriptor, "w") as destination:
            destination.write(data)
            destination.flush()
            os.fsync(destination.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def runner_identity():
    directory = Path(__file__).resolve().parent
    return {"python": sys.version, "executable": sys.executable,
            "sources": {name: digest_file(directory / name)
                        for name in ("qualify.py", "contracts.py", "coverage.py", "process.py", "remote_matrix.py",
                                     "remote_work_plan.py")}}


def selected_cases(driver, cases):
    return [case for case in cases if case["id"] in driver["features"]
            and case["scenario"] in driver.get("scenarios", [case["scenario"]])
            and case["layer"] in driver.get("layers", [case["layer"]])]


def selected_source(reference):
    require(isinstance(reference, str) and reference, "Source reference must be a nonempty string")
    roots = {"product": OWNER, "repo": REPOSITORY}
    if "://" not in reference:
        return contained_file(OWNER, reference)
    kind, relative = reference.split("://", 1)
    require(kind in roots, "Unknown source root selector")
    return contained_file(roots[kind], relative)


def source_pins(driver):
    entrypoint = selected_source(driver["entrypoint"])
    require(entrypoint.suffix == ".py" and digest_file(entrypoint) == driver["sha256"],
            "Selected driver source differs from its explicit pin")
    pins = [(entrypoint, driver["sha256"])]
    for dependency in driver.get("dependencies", []):
        path = selected_source(dependency["path"])
        require(digest_file(path) == dependency["sha256"], "Driver dependency differs from its pin")
        pins.append((path, dependency["sha256"]))
    return entrypoint, pins


def emit_event(stream, level, event, **details):
    value = {"time": timestamp(), "level": level, "event": event, **details}
    stream.write(json.dumps(value, sort_keys=True) + "\n")
    stream.flush()
    if level == "WARNING":
        print("WARNING: " + event + ": " + details.get("reason", ""), file=sys.stderr, flush=True)


def admit_sources(plan, registry):
    count = 0
    for target in plan["targets"]:
        cases = expected_cases(registry, target)
        count += len(cases)
        seen = set()
        for driver in target.get("drivers", []):
            source_pins(driver)
            matches = selected_cases(driver, cases)
            require(matches, "Driver selection matches no applicable registry cases")
            for case in matches:
                key = case_key(case)
                require(key not in seen, "Two drivers claim the same target/scenario/layer")
                seen.add(key)
    require(count <= MAX_RUN_CASES, "Selected run exceeds its explicit case-count bound")


def run_target(target, registry, plan_digest, registry_digest, plan, output, events, resume=False,
               metadata_budget=None):
    if metadata_budget is None:
        metadata_budget = {"remaining": MAX_RESULT_METADATA_BYTES}
    expected = expected_cases(registry, target)
    observed = {}
    warned = set()
    capabilities = set(target.get("capabilities", []))
    if target["kind"] in {"vm", "flatpak", "container"}:
        capabilities.discard("bare_metal")
    for case in expected:
        missing = sorted(set(case["requires"]) - capabilities)
        if not missing:
            continue
        reason = "Unavailable required capabilities on " + target["platform"] + ": " + ", ".join(missing)
        observed[case_key(case)] = missing_case(case, reason)
        warned.add(case_key(case))
        emit_event(events, "WARNING", "required-check-not-run", target=target["id"],
                   feature=case["id"], scenario=case["scenario"], reason=reason)
    for driver in target.get("drivers", []):
        cases = [case for case in selected_cases(driver, expected) if case_key(case) not in observed]
        if not cases:
            continue
        invocation = output / target["id"] / driver["id"]
        lane_output = invocation / "output"
        request = {"schema": "dockpipe.product-integration.request/v1", "run_id": plan["run_id"],
                   "plan_sha256": plan_digest, "registry_sha256": registry_digest,
                   "target": {key: target[key] for key in ("id", "platform", "kind", "artifact_sha256")},
                   "configuration": driver.get("configuration", {}),
                   "cases": [{key: case[key] for key in ("id", "scenario", "layer", "required_assertions")}
                             for case in cases]}
        request_path = invocation / "request.json"
        entrypoint, pins = source_pins(driver)
        command = [sys.executable, str(entrypoint), "--request", str(request_path), "--output", str(lane_output)]
        existing = invocation.exists()
        execution = {"exit_code": None, "failure": "Driver did not complete"}
        try:
            if existing:
                require(resume and not invocation.is_symlink(), "Driver output already exists")
                require(read_json(request_path) == request, "Resume request differs from captured invocation")
                execution = read_json(invocation / "process.json")
            else:
                invocation.mkdir(mode=0o700, parents=True)
                write_json(request_path, request)
                emit_event(events, "INFO", "driver-started", target=target["id"], driver=driver["id"])
                execution = run_driver(command, invocation, driver.get("timeout_seconds", 1200), dict(os.environ))
                write_json(invocation / "process.json", execution)
            require(not execution["failure"], execution["failure"])
            require(all(digest_file(path) == digest for path, digest in pins),
                    "Driver source changed during the observation")
            receipt_path = lane_output / "result.json"
            metadata_budget["remaining"] -= receipt_path.lstat().st_size
            require(metadata_budget["remaining"] >= 0, "Lane receipts exceed the run's 32 MiB metadata bound")
            receipt = read_json(receipt_path)
            rows = validate_receipt(receipt, request, lane_output)
            if execution["exit_code"] != 0:
                require(not any(row["status"] == "passed" for row in rows.values()),
                        "A failing driver process cannot contribute passing coverage")
            for key, row in rows.items():
                row["evidence_root"] = lane_output.relative_to(output).as_posix()
                observed[key] = row
            if existing:
                emit_event(events, "INFO", "driver-receipt-revalidated", target=target["id"], driver=driver["id"])
        except (ContractError, OSError, ValueError, KeyError, TypeError) as error:
            reason = str(error)
            if existing:
                reason += "; retained invocation was not replayed; owning driver must reconcile its exact resource identities"
            for case in cases:
                observed[case_key(case)] = missing_case(case, reason, "failed")
            emit_event(events, "WARNING", "driver-result-rejected", target=target["id"],
                       driver=driver["id"], reason=reason)
        emit_event(events, "INFO", "driver-finished", target=target["id"], driver=driver["id"],
                   exit_code=execution["exit_code"])
    summary = target_summary(target, expected, observed)
    for case in summary["cases"]:
        if case["status"] in {"blocked", "not_run", "manual_pending"} and case_key(case) not in warned:
            emit_event(events, "WARNING", "required-check-not-run", target=target["id"],
                       feature=case["id"], scenario=case["scenario"], reason=case["reason"])
    return summary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--registry", type=Path, default=OWNER / "registry.json")
    parser.add_argument("--plan", type=Path)
    parser.add_argument("--output", type=Path)
    listing = parser.add_mutually_exclusive_group()
    listing.add_argument("--list", action="store_true", help="List coverage without launching any test")
    listing.add_argument("--remote-matrix", action="store_true",
                         help="List installed remote feature and role combinations")
    listing.add_argument("--remote-work-plan", action="store_true",
                         help="Group remaining remote checks by platform environments")
    parser.add_argument("--remote-observations", type=Path, action="append", default=[],
                        help="Private installed observations for remote role coverage (repeatable)")
    parser.add_argument("--resume", action="store_true",
                        help="Revalidate completed lanes; never replay an existing incomplete invocation")
    arguments = parser.parse_args()
    registry, registry_bytes = read_json_document(arguments.registry)
    registry = validate_registry(registry)
    role_matrix = None
    if arguments.remote_matrix or arguments.remote_work_plan or (not arguments.list and "remote_qualification" in registry):
        role_matrix = remote_matrix(registry, arguments.remote_observations)
    if arguments.remote_matrix:
        print(json.dumps(role_matrix, indent=2))
        return 0
    if arguments.remote_work_plan:
        print(json.dumps(remote_work_plan(registry, role_matrix), indent=2))
        return 0
    if arguments.list:
        print(json.dumps({"platforms": registry["platforms"], "features": registry["features"]}, indent=2))
        return 0
    if arguments.plan is None or arguments.output is None:
        parser.error("--plan and a fresh absolute --output are required")
    plan, plan_bytes = read_json_document(arguments.plan)
    plan = validate_plan(plan, registry)
    admit_sources(plan, registry)
    output = arguments.output
    require(output.is_absolute() and not output.is_symlink(), "Choose an absolute output directory without links")
    plan_digest = hashlib.sha256(plan_bytes).hexdigest()
    registry_digest = hashlib.sha256(registry_bytes).hexdigest()
    identity = runner_identity()
    if arguments.resume:
        require(output.is_dir() and digest_file(output / "plan.json") == plan_digest
                and digest_file(output / "registry.json") == registry_digest
                and read_json(output / "runner.json") == identity,
                "Resume requires the identical captured plan, registry and runner")
    else:
        require(not output.exists(), "Choose a fresh invocation-owned output directory")
        output.mkdir(mode=0o700, parents=True)
        # Preserve exact source bytes so resume binds formatting as well as values.
        for data, name in ((plan_bytes, "plan.json"), (registry_bytes, "registry.json")):
            destination = output / name
            destination.write_bytes(data)
            destination.chmod(0o600)
        write_json(output / "runner.json", identity)
    summaries = []
    with (output / "events.jsonl").open("a" if arguments.resume else "x") as events:
        (output / "events.jsonl").chmod(0o600)
        emit_event(events, "INFO", "qualification-started", run_id=plan["run_id"],
                   policy="opt-in-local-no-ci")
        interrupted = False
        metadata_budget = {"remaining": MAX_RESULT_METADATA_BYTES}
        try:
            for target in plan["targets"]:
                summaries.append(run_target(target, registry, plan_digest, registry_digest, plan, output, events,
                                            arguments.resume, metadata_budget))
        except KeyboardInterrupt:
            interrupted = True
            emit_event(events, "WARNING", "qualification-interrupted",
                       reason="Owned driver process stopped; external effects require owning-driver reconciliation")
            finished = {summary["target"]["id"] for summary in summaries}
            for target in plan["targets"]:
                if target["id"] in finished:
                    continue
                expected = expected_cases(registry, target)
                rows = {case_key(case): missing_case(case, "Run interrupted; retained resources require reconciliation")
                        for case in expected}
                summaries.append(target_summary(target, expected, rows))
        overall = overall_summary(registry, summaries)
        if role_matrix is not None:
            remote_counts = role_matrix["counts"]
            overall["remote_role_coverage"] = remote_counts
            overall["full_product_qualified"] = (
                overall["full_product_qualified"] and remote_counts["passed"] > 0
                and not any(remote_counts[status] for status in ("failed", "blocked", "unrun"))
            )
        for profile in overall["unconfigured_platform_profiles"]:
            emit_event(events, "WARNING", "platform-profile-not-run", reason=profile)
        report = {"schema": "dockpipe.product-integration.report/v1", "run_id": plan["run_id"],
                  "plan_sha256": plan_digest, "registry_sha256": registry_digest,
                  "runner": identity, "overall": overall, "targets": summaries, "finished_at": timestamp()}
        require(runner_identity() == identity, "Runner sources changed during qualification; retained evidence requires review")
        if role_matrix is not None:
            matrix_path = output / "remote-matrix.json"
            replace_report(matrix_path, role_matrix)
            report["remote_matrix"] = {"path": matrix_path.name, "sha256": digest_file(matrix_path)}

        report["interrupted"] = interrupted
        replace_report(output / "result.json", report)
        emit_event(events, "INFO", "qualification-finished", **overall)
    print(json.dumps(overall, indent=2))
    if interrupted:
        return 130
    if (any(value["counts"].get("failed", 0) for value in summaries)
            or (role_matrix is not None and role_matrix["counts"]["failed"])):
        return 1
    return 0 if overall["full_product_qualified"] else 2


if __name__ == "__main__":
    signal.signal(signal.SIGTERM, interrupt_run)
    try:
        raise SystemExit(main())
    except (ContractError, OSError, ValueError, KeyError, TypeError) as error:
        print("Product qualification input/result failure: " + str(error), file=sys.stderr)
        raise SystemExit(1)
