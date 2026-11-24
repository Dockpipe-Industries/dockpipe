"""Exercise package authoring through one pinned portable CLI and private fixtures."""
from pathlib import Path
import argparse
import os
import platform
import shutil
import sys
import tarfile


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "assets/scripts"))
from contracts import digest_file, read_json, require, write_json
from portable import command


def archive_asset(archive, member, expected):
    with tarfile.open(archive, "r:gz") as contents:
        information = contents.getmember(member)
        if not information.isfile() or information.size > 1024 * 1024:
            return False
        with contents.extractfile(information) as stream:
            return stream.read(1024 * 1024 + 1) == expected.encode()


def create_sources(workspace):
    files = {
        "resolvers/fixture-tool/package.yml": "schema: 1\nkind: resolver\nname: fixture-tool\nversion: 1.0.0\nnamespace: author-fixture\n",
        "resolvers/fixture-tool/profile": "PRODUCT_RESOLVER_FIXTURE=compiled-tool\n",
        "child/package.yml": (
            "schema: 1\nkind: workflow\nname: author-child\nversion: 1.0.0\nnamespace: author-fixture\n"
            "allow_clone: true\ntest:\n  script: tests.sh\n"),
        "child/config.yml": (
            "name: author-child\nnamespace: author-fixture\nvault: none\ndocker_preflight: false\n"
            "types:\n  - AuthoringFixture.pipe\n"
            "vars:\n  PRODUCT_VALUE: declared-default\nsteps:\n"
            "  - id: child\n    kind: host\n    cwd: repo\n    run: child.sh\n"),
        "child/child.sh": "set -eu\nprintf '%s' \"$PRODUCT_VALUE\" > child-result.txt\n",
        "child/AuthoringFixture.pipe": (
            "public Class AuthoringFixture\n{\n"
            "    [EnvName = \"PRODUCT_VALUE\"]\n"
            "    public string Value = \"declared-default\";\n}\n"),
        "child/tests.sh": "set -eu\nprintf '%s\\n' package-owned-test-executed\n",
        "parent/package.yml": (
            "schema: 1\nkind: workflow\nname: author-parent\nversion: 1.0.0\nnamespace: author-fixture\nallow_clone: false\n"),
        "parent/config.yml": (
            "name: author-parent\nvault: none\ndocker_preflight: false\nsteps:\n"
            "  - id: invoke-child\n    workflow: author-child\n    package: author-fixture\n"
            "    inputs:\n      Value: supplied-value\n"),
    }
    source = workspace / "sources"
    for relative, text in files.items():
        destination = source / relative
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_text(text)
    return source


def run(request, output):
    target = request["target"]
    require(platform.system() == "Linux" and target["platform"] == "linux-portable"
            and target["kind"] == "local", "Package authoring requires an explicitly selected local Linux portable CLI")
    configuration = request["configuration"]
    require(set(configuration) == {"artifact"}, "Package authoring requires exactly one pinned artifact")
    artifact = Path(configuration["artifact"])
    require(artifact.is_absolute() and digest_file(artifact) == target["artifact_sha256"],
            "Selected package authoring CLI differs from its pin")
    output.mkdir(mode=0o700, parents=True)
    cli = output / "dockpipe"
    shutil.copyfile(artifact, cli)
    cli.chmod(0o700)
    require(digest_file(cli) == target["artifact_sha256"], "Copied package authoring CLI differs from its pin")
    home = output / "home"
    home.mkdir(mode=0o700)
    environment = {name: os.environ[name] for name in ("PATH", "LANG", "LC_ALL", "TZ") if name in os.environ}
    environment.update(HOME=str(home), DOCKPIPE_VAULT_INJECT="0", DOCKPIPE_INTERACTIVE="0",
                       DOCKPIPE_PACKAGES_ROOT=str(output / "store"))
    for name in ("config", "cache", "data", "state"):
        directory = home / name
        directory.mkdir(mode=0o700)
        environment["XDG_" + name.upper() + "_HOME"] = str(directory)
    workspace = output / "workspace"
    workspace.mkdir(mode=0o700)
    project = workspace / "dockpipe.config.json"
    project.write_text('{"compile":{"workflows":["sources"]}}\n')
    source = create_sources(workspace)
    operations = {}

    def invoke(name, arguments):
        operations[name] = command(cli, arguments, workspace, environment, output, name)
        return operations[name]

    invoke("resolver-compile", ["package", "compile", "resolvers", "--from", str(source / "resolvers")])
    for name in ("child", "parent"):
        invoke(name + "-compile", ["package", "compile", "workflow", "--from", str(source / name)])
    invoke("package-test", ["package", "test", "--only", "author-child"])
    tested = operations["package-test"] and "package-owned-test-executed" in (
        output / "package-test/driver.stdout").read_text()
    # The runtime must resolve installed archives, with no authored source fallback.
    project.write_text("{}\n")
    invoke("inventory", ["package", "list", "--format", "json"])
    inventory = read_json(output / "inventory/driver.stdout") if operations["inventory"] else {}
    packages = inventory.get("packages", [])
    compiled = {}
    for kind, name in (("resolver", "fixture-tool"), ("workflow", "author-child"), ("workflow", "author-parent")):
        candidates = [Path(row["path"]) for row in packages if row.get("kind") == kind and row.get("name") == name]
        if len(candidates) == 1 and candidates[0].resolve().is_relative_to(output):
            compiled[name] = candidates[0]
    compiled_ok = len(compiled) == 3 and all(operations[name] for name in
        ("resolver-compile", "child-compile", "parent-compile", "inventory"))
    if compiled_ok:
        compiled_ok = archive_asset(compiled["fixture-tool"], "resolvers/fixture-tool/profile",
                                    "PRODUCT_RESOLVER_FIXTURE=compiled-tool\n")
    marker = workspace / "child-result.txt"
    results = []
    for name in ("run-first", "run-repeat"):
        marker.unlink(missing_ok=True)
        succeeded = invoke(name, ["--workflow", "author-parent", "--package", "author-parent"])
        results.append(succeeded and marker.is_file() and marker.read_bytes() == b"supplied-value")
    clone = workspace / "cloned-child"
    clone_allowed = invoke("clone-allowed", ["clone", "author-child", "--to", str(clone)])
    clone_allowed = clone_allowed and (clone / "child.sh").is_file()
    clone_allowed = clone_allowed and (clone / "child.sh").read_bytes() == (source / "child/child.sh").read_bytes()
    archive_digests = {name: digest_file(path) for name, path in compiled.items()}
    denied = workspace / "cloned-parent"
    invoke("clone-denied", ["clone", "author-parent", "--to", str(denied)])
    refusal = read_json(output / "clone-denied/process.json")
    diagnostic = (output / "clone-denied/driver.stderr").read_text()
    refused = type(refusal["exit_code"]) is int and refusal["exit_code"] == 1 and not refusal["failure"]
    refused = refused and "does not allow cloning" in diagnostic and "allow_clone" in diagnostic
    unchanged = not denied.exists() and archive_digests == {name: digest_file(path) for name, path in compiled.items()}
    invoke("export", ["package", "build", "store", "--out", str(output / "exported"), "--only", "workflows"])
    observed = {
        "resolver-and-workflow-compiled": compiled_ok and operations["export"],
        "packaged-child-ran": all(results),
        "inputs-deterministic": all(results) and tested and clone_allowed,
    }
    facts = {"operations": operations, "package_owned_test_observed": tested,
             "child_markers_exact": results, "authored_sources_excluded_from_runtime_roots": True,
             "allowed_clone_exact_asset": clone_allowed, "forbidden_clone_refused": refused,
             "forbidden_clone_preserved_archives": unchanged,
             "installed_executable_sha256": digest_file(cli), "system": platform.system()}
    write_json(output / "observations.json", facts)
    paths = [output / "observations.json", project]
    paths.extend(path for path in source.rglob("*") if path.is_file())
    paths.extend(compiled.values())
    for name in operations:
        paths.extend(sorted((output / name).iterdir()))
    if marker.is_file():
        paths.append(marker)
    if (clone / "child.sh").is_file():
        paths.append(clone / "child.sh")
    evidence = [{"path": path.relative_to(output).as_posix(), "sha256": digest_file(path)} for path in paths]
    rows = []
    for case in request["cases"]:
        row = {name: case[name] for name in ("id", "scenario", "layer")}
        row.update(status="blocked", reason="Package authoring driver does not implement this case",
                   assertions=[], evidence=[])
        if case["id"] == "packages.authoring" and case["scenario"] == "happy" and case["layer"] == "installed_e2e":
            require(set(case["required_assertions"]) == set(observed), "Package authoring assertion contract changed")
            row.update(status="passed" if all(observed.values()) else "failed",
                       reason="Actual resolver/workflow archives, package-owned test, exported store and repeated packaged-child input readback",
                       assertions=[{"id": name, "status": "passed" if observed[name] else "failed",
                                    "detail": "See actual CLI logs, archive identities and independent child marker readback"}
                                   for name in case["required_assertions"]], evidence=evidence)
        elif case["id"] == "packages.authoring" and case["scenario"] == "failure" and case["layer"] == "component":
            failures = {"expected-failure-observed": refused, "no-unowned-effects": unchanged,
                        "actionable-result": refused}
            require(set(case["required_assertions"]) == set(failures), "Package refusal assertion contract changed")
            row.update(status="passed" if all(failures.values()) else "failed",
                       reason="Actual compiled package allow_clone:false refusal; exact exit/diagnostic and unchanged archives/no target",
                       assertions=[{"id": name, "status": "passed" if failures[name] else "failed",
                                    "detail": "Restricted clone must fail normally without changing the selected archives or creating its target"}
                                   for name in case["required_assertions"]], evidence=evidence)
        rows.append(row)
    receipt = {name: request[name] for name in ("run_id", "plan_sha256", "registry_sha256", "target")}
    receipt.update(schema="dockpipe.product-integration.lane/v1", cases=rows, observed=facts)
    write_json(output / "result.json", receipt)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--request", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    arguments = parser.parse_args()
    require(arguments.output.is_absolute() and not arguments.output.exists(), "Select fresh absolute output")
    request = read_json(arguments.request)
    require(request.get("schema") == "dockpipe.product-integration.request/v1", "Unknown request schema")
    run(request, arguments.output)


if __name__ == "__main__":
    main()
