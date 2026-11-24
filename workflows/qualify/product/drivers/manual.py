"""Produce human check packets and admit explicit, individually evidenced observations."""
from pathlib import Path
import argparse
import datetime
import shutil
import sys


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "assets/scripts"))
from contracts import (case_key, contained_file, digest_file, read_json, require,
                       validate_receipt, write_json)


def run(request, configuration, output):
    require(set(configuration) <= {"record"}, "Unknown human driver configuration")
    require(all(case["layer"] == "human" for case in request["cases"]),
            "Manual observations qualify only the human layer")
    output.mkdir(mode=0o700, parents=True)
    packet = {key: request[key] for key in ("run_id", "plan_sha256", "registry_sha256", "target", "cases")}
    packet.update(schema="dockpipe.product-integration.human-packet/v1", instructions=[
        "Use the selected installed artifact and isolated generated fixtures only.",
        "Perform every requested assertion and record the actual result, detail and timestamp individually.",
        "Record exact VM passthrough device or physical host identity without secrets or personal data.",
        "Attach bounded evidence of actual observations; do not attach unrelated screen or clipboard content.",
        "Missing hardware or permission remains not_run/manual_pending. Never infer a pass from this packet.",
        "Report cleanup and retain original references when effects are uncertain.",
    ])
    write_json(output / "human-packet.json", packet)
    record_path = configuration.get("record")
    observations = {}
    if record_path is not None:
        record_path = Path(record_path)
        require(record_path.is_absolute(), "Human record must select an absolute private file")
        record = read_json(record_path)
        require(record.get("schema") == "dockpipe.product-integration.human-record/v1", "Unknown human record schema")
        for key in ("run_id", "plan_sha256", "registry_sha256", "target"):
            require(record.get(key) == request[key], "Human record changed its captured " + key)
        require(record.get("observer_role") == "operator", "Human evidence requires an explicit operator observation")
        observed_at = datetime.datetime.fromisoformat(record["observed_at"])
        require(observed_at.tzinfo is not None, "Human observation requires a timezone")
        requested = {case_key(case) for case in request["cases"]}
        cases = record.get("cases")
        require(isinstance(cases, list) and len(cases) <= len(requested), "Unexpected human observation count")
        total_bytes = 0
        for case in cases:
            key = case_key(case)
            require(key in requested and key not in observations, "Unexpected or duplicate human observation")
            require(isinstance(case.get("reason"), str) and case["reason"].strip(), "Human observation needs an actual result description")
            for assertion in case.get("assertions", []):
                require(isinstance(assertion.get("detail"), str) and assertion["detail"].strip(),
                        "Each human assertion needs an observation detail")
            evidence = []
            for index, item in enumerate(case.get("evidence", [])):
                source = contained_file(record_path.parent, item["path"])
                require(digest_file(source) == item["sha256"], "Human observation evidence changed")
                total_bytes += source.stat().st_size
                require(total_bytes <= 512 * 1024 * 1024, "Human evidence exceeds its aggregate 512 MiB bound")
                relative = "evidence/" + str(len(observations)) + "-" + str(index) + ".bin"
                destination = output / relative
                destination.parent.mkdir(mode=0o700, exist_ok=True)
                shutil.copyfile(source, destination)
                destination.chmod(0o600)
                require(digest_file(destination) == item["sha256"], "Copied human evidence changed")
                evidence.append({"path": relative, "sha256": item["sha256"]})
            row = dict(case, evidence=evidence)
            observations[key] = row
        write_json(output / "observer.json", {"observer_role": "operator", "observed_at": record["observed_at"],
                                              "record_sha256": digest_file(record_path)})
    rows = []
    for case in request["cases"]:
        row = observations.get(case_key(case))
        if row is None:
            row = {key: case[key] for key in ("id", "scenario", "layer")}
            row.update(status="manual_pending", reason="Human check packet created; actual operator observations are pending",
                       assertions=[], evidence=[])
        rows.append(row)
    receipt = {key: request[key] for key in ("run_id", "plan_sha256", "registry_sha256", "target")}
    receipt.update(schema="dockpipe.product-integration.lane/v1", cases=rows,
                   observation_method="operator-recorded; evidence semantics require human review")
    validate_receipt(receipt, request, output)
    write_json(output / "result.json", receipt)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--request", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    arguments = parser.parse_args()
    require(arguments.output.is_absolute() and not arguments.output.exists(), "Choose fresh absolute output")
    request = read_json(arguments.request)
    require(request.get("schema") == "dockpipe.product-integration.request/v1", "Unknown request schema")
    run(request, request["configuration"], arguments.output)


if __name__ == "__main__":
    main()
