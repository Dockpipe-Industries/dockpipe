#!/usr/bin/env python3
"""Test and package this repository's editor through its locked toolchain."""
import argparse
import json
import os
from pathlib import Path
import sys
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "editor/shared"))
import contract


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=Path)
    options = parser.parse_args(json.loads(os.environ.get("DOCKPIPE_ARGS_JSON", "[]")))
    output = options.out or contract.scope("vsix")
    if not output.is_absolute():
        parser.error("--out must select an absolute artifact directory")
    output.mkdir(parents=True, exist_ok=True)
    current = contract.manifest()
    filename = current["name"] + "-" + current["version"] + ".vsix"
    if (output / filename).exists() or (output / "extension.json").exists():
        raise ValueError("Use a fresh artifact directory; qualified VSIX files are immutable")
    if current["name"] == "dockpipe-language-support":
        for name in ("dockpipe.config.schema.json", "workflow.schema.json"):
            if (contract.EXTENSION / "schemas" / name).read_bytes() != (contract.ROOT / "src/lib/infrastructure/schema" / name).read_bytes():
                raise ValueError("Editor schema must match the actual runtime source: " + name)
    environment = contract.clean_environment()
    contract.run([sys.executable, contract.ROOT / "workflows/editor/shared/test_contract.py"], contract.ROOT, environment, 60)
    with tempfile.TemporaryDirectory(prefix="editor-build-", dir=contract.scope("build")) as temporary:
        temporary = Path(temporary)
        staging = contract.stage(temporary)
        node, tool = contract.install_tools(staging, environment)
        contract.run([node, "--check", "extension.js"], staging, environment, 60)
        contract.run([node, "extension.test.js"], staging, environment, 60)
        candidate = temporary / filename
        contract.run([node, tool, "package", "--no-dependencies", "--out", candidate], staging, environment)
        identity = contract.inspect_vsix(candidate, current["version"])
        receipt = {"schema": 1, "identity": identity, "version": current["version"], "filename": filename,
                   "bytes": candidate.stat().st_size, "sha256": contract.digest(candidate),
                   "source_inventory": contract.inventory(), "source_revision": os.environ.get("GITHUB_SHA"),
                   "qualification": "tests-and-vsce-package-passed", "reproducible_bytes_claimed": False}
        candidate.rename(output / filename)
        (output / "extension.json").write_text(json.dumps(receipt, indent=2) + "\n")
    if os.environ.get("GITHUB_OUTPUT"):
        with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as target:
            target.write("artifact_directory=" + str(output) + "\n")
    print(json.dumps({key: receipt[key] for key in ("identity", "version", "filename", "sha256")}))


if __name__ == "__main__":
    main()
