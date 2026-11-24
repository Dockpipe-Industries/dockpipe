"""Qualify package catalog/install/removal using a private trusted TLS fixture."""
from pathlib import Path
import argparse
import functools
import http.server
import os
import platform
import shutil
import ssl
import sys
import threading


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "assets/scripts"))
from contracts import digest_file, read_json, require, write_json
from portable import command


class FixtureServer(http.server.HTTPServer):
    def get_request(self):
        connection, address = super().get_request()
        connection.settimeout(5)
        try:
            return self.tls_context.wrap_socket(connection, server_side=True), address
        except BaseException:
            connection.close()
            raise


class FixtureHandler(http.server.SimpleHTTPRequestHandler):
    def do_GET(self):
        if len(self.server.requests) >= 64:
            self.send_error(503)
            return
        if self.path not in self.server.allowed_paths:
            self.send_error(404)
            return
        self.server.requests.append(self.path)
        super().do_GET()

    def log_message(self, format, *arguments):
        pass


def run(request, output):
    target = request["target"]
    require(platform.system() == "Linux" and target["platform"] == "linux-portable"
            and target["kind"] == "local", "Package lifecycle requires a selected local Linux portable CLI")
    configuration = request["configuration"]
    require(set(configuration) == {"artifact", "tls_certificate", "tls_key",
                                   "tls_certificate_sha256", "tls_key_sha256"},
            "Package lifecycle requires a pinned CLI and explicit private fixture TLS file references")
    artifact = Path(configuration["artifact"])
    certificate = Path(configuration["tls_certificate"])
    key = Path(configuration["tls_key"])
    for path in (artifact, certificate, key):
        require(path.is_absolute() and path.is_file() and not path.is_symlink(), "Select absolute regular fixture files")
    require(certificate.stat().st_size <= 65536 and key.stat().st_size <= 65536,
            "Fixture TLS files exceed their bounded size")
    require(key.stat().st_mode & 0o077 == 0, "Fixture TLS key must be private to its owner")
    require(digest_file(certificate) == configuration["tls_certificate_sha256"]
            and digest_file(key) == configuration["tls_key_sha256"], "Fixture TLS identity differs from its plan pins")
    require(digest_file(artifact) == target["artifact_sha256"], "Package lifecycle CLI differs from its pin")
    output.mkdir(mode=0o700, parents=True)
    cli = output / "dockpipe"
    shutil.copyfile(artifact, cli)
    cli.chmod(0o700)
    require(digest_file(cli) == target["artifact_sha256"], "Copied package lifecycle CLI differs from its pin")
    home = output / "home"
    home.mkdir(mode=0o700)
    environment = {name: os.environ[name] for name in ("PATH", "LANG", "LC_ALL", "TZ") if name in os.environ}
    environment.update(HOME=str(home), DOCKPIPE_VAULT_INJECT="0", DOCKPIPE_INTERACTIVE="0",
                       DOCKPIPE_GLOBAL_ROOT=str(output / "user-store"))
    for name in ("config", "cache", "data", "state"):
        directory = home / name
        directory.mkdir(mode=0o700)
        environment["XDG_" + name.upper() + "_HOME"] = str(directory)
    producer = output / "producer"
    consumer = output / "consumer"
    for directory in (producer, consumer):
        directory.mkdir(mode=0o700)
        (directory / "dockpipe.config.json").write_text("{}\n")
    producer_environment = dict(environment, DOCKPIPE_PACKAGES_ROOT=str(output / "compiled-store"))
    operations = {}

    def invoke(name, arguments, workspace=consumer, selected_environment=environment):
        operations[name] = command(cli, arguments, workspace, selected_environment, output, name)
        return operations[name]

    for name in ("remove", "keep", "reject"):
        identity = "lifecycle-" + name
        source = producer / identity
        source.mkdir()
        (source / "package.yml").write_text(
            "schema: 1\nkind: workflow\nname: " + identity + "\nversion: 1.0.0\nnamespace: lifecycle-fixture\n")
        (source / "config.yml").write_text(
            "name: " + identity + "\nvault: none\ndocker_preflight: false\nsteps:\n"
            "  - id: marker\n    kind: host\n    cwd: repo\n    run: marker.sh\n")
        (source / "marker.sh").write_text("set -eu\nprintf '%s' " + identity + " > " + identity + ".txt\n")
        invoke("compile-" + name, ["package", "compile", "workflow", "--from", str(source)],
               producer, producer_environment)
    site = output / "catalog-site"
    invoke("export", ["package", "build", "store", "--out", str(site), "--only", "workflows"],
           producer, producer_environment)
    require(all(operations.values()), "Private package fixture production failed; inspect retained CLI logs")
    manifest_path = site / "packages-store-manifest.json"
    manifest = read_json(manifest_path)
    entries = {row["name"]: row for row in manifest["packages"]["workflows"]}
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.load_cert_chain(certificate, key)
    server = FixtureServer(("127.0.0.1", 0), functools.partial(FixtureHandler, directory=str(site)))
    server.allowed_paths = {"/packages-store-manifest.json", *('/' + row["tarball"] for row in entries.values())}
    server.requests = []
    server.tls_context = context
    thread = threading.Thread(target=server.serve_forever, name="package-fixture-tls", daemon=True)
    environment["SSL_CERT_FILE"] = str(certificate)
    remote = "https://127.0.0.1:" + str(server.server_port) + "/packages-store-manifest.json"
    thread.start()
    installed = {}
    facts = {}
    try:
        invoke("remote-catalog", ["package", "catalog", "--remote", remote])
        for name in ("remove", "keep"):
            identity = "lifecycle-" + name
            invoke("install-" + name, ["package", "install", "--remote", remote, "--kind", "workflow",
                                      "--name", identity, "--sha256", entries[identity]["sha256"]])
            if operations["install-" + name]:
                path = Path(read_json(output / ("install-" + name) / "driver.stdout")["path"])
                require(path.is_absolute() and path.resolve().is_relative_to(output / "user-store"),
                        "Install receipt escaped the selected private user store")
                installed[name] = path
        invoke("installed-catalog", ["catalog", "list", "--format", "json"])
        catalog = read_json(output / "installed-catalog/driver.stdout") if operations["installed-catalog"] else {}
        catalog_names = {row.get("workflow_id") for row in catalog.get("workflows", [])}
        digests = {name: digest_file(path) for name, path in installed.items()}
        before_requests = list(server.requests)
        invoke("digest-refusal", ["package", "install", "--remote", remote, "--kind", "workflow",
                                  "--name", "lifecycle-reject", "--sha256", "0" * 64])
        refusal = read_json(output / "digest-refusal/process.json")
        diagnostic = (output / "digest-refusal/driver.stderr").read_text()
        unchanged = digests == {name: digest_file(path) for name, path in installed.items()}
        rejected_download = any(path.endswith(entries["lifecycle-reject"]["tarball"])
                                for path in server.requests[len(before_requests):])
        facts["checksum_refusal"] = (type(refusal["exit_code"]) is int and refusal["exit_code"] == 1
            and not refusal["failure"]
            and "selected package changed in the remote; refresh the catalog before installing" in diagnostic
            and unchanged and not rejected_download)
        removed = False
        if "remove" in installed:
            removed = invoke("uninstall", ["package", "uninstall", "--path", str(installed["remove"])])
            removed = removed and not installed["remove"].exists()
        kept = "keep" in installed and installed["keep"].is_file() and digest_file(installed["keep"]) == digests["keep"]
        executed = invoke("retained-execution", ["--workflow", "lifecycle-keep"])
        marker = consumer / "lifecycle-keep.txt"
        kept = kept and executed and marker.is_file() and marker.read_bytes() == b"lifecycle-keep"
        facts.update(selected_archives_match=len(installed) == 2 and all(
            digests[name] == entries["lifecycle-" + name]["sha256"] for name in installed),
            installed_catalog_owned={"lifecycle-remove", "lifecycle-keep"} <= catalog_names,
            selected_removed_other_retained=removed and kept,
            requests=list(server.requests), certificate_sha256=digest_file(certificate))
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=10)
    require(not thread.is_alive() and server.socket.fileno() == -1, "Owned fixture TLS server did not stop")
    require(digest_file(certificate) == configuration["tls_certificate_sha256"]
            and digest_file(key) == configuration["tls_key_sha256"], "Fixture TLS identity changed during execution")
    facts["fixture_server_stopped"] = True
    write_json(output / "observations.json", facts)
    proof_paths = [output / "observations.json", manifest_path]
    proof_paths.extend(path for path in producer.rglob("*") if path.is_file())
    for name in operations:
        proof_paths.extend(sorted((output / name).iterdir()))
    marker = consumer / "lifecycle-keep.txt"
    if marker.is_file():
        proof_paths.append(marker)
    evidence = [{"path": path.relative_to(output).as_posix(), "sha256": digest_file(path)} for path in proof_paths]
    rows = []
    for case in request["cases"]:
        row = {name: case[name] for name in ("id", "scenario", "layer")}
        row.update(status="blocked", reason="Package lifecycle driver does not implement this case", assertions=[], evidence=[])
        assertions = None
        if case["id"] == "packages.lifecycle" and case["scenario"] == "happy" and case["layer"] == "installed_e2e":
            assertions = {"fixture-installed": facts["selected_archives_match"],
                          "catalog-owned-surface": facts["installed_catalog_owned"],
                          "remove-preserves-other-package": facts["selected_removed_other_retained"]}
        elif case["id"] == "packages.lifecycle" and case["scenario"] == "failure" and case["layer"] == "component":
            assertions = {name: facts["checksum_refusal"] for name in
                          ("expected-failure-observed", "no-unowned-effects", "actionable-result")}
        if assertions is not None:
            require(set(case["required_assertions"]) == set(assertions), "Package lifecycle assertion contract changed")
            row.update(status="passed" if all(assertions.values()) else "failed",
                       reason="Actual TLS catalog and pinned install, private user store, exact optional removal and retained-package execution; checksum negative records normal refusal and no rejected archive download",
                       assertions=[{"id": name, "status": "passed" if assertions[name] else "failed",
                                    "detail": "See exact CLI logs, selected archive digests, fixture request ledger and marker"}
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
