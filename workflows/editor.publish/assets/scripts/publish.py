#!/usr/bin/env python3
"""Publish exactly the qualified, reviewed VSIX without rebuilding it."""
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
    parser.add_argument("--artifact-dir", type=Path, required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--sha256", required=True)
    options = parser.parse_args(json.loads(os.environ.get("DOCKPIPE_ARGS_JSON", "[]")))
    artifact = contract.qualify_artifact(options.artifact_dir.resolve(strict=True), options.version, options.sha256)
    token = os.environ.get("VSCE_PAT", "")
    if not token:
        raise ValueError("Configure the VSCE_PAT environment secret for the reviewed publisher")
    environment = contract.clean_environment()
    with tempfile.TemporaryDirectory(prefix="editor-publish-", dir=contract.scope("build")) as temporary:
        staging = contract.stage(Path(temporary))
        node, tool = contract.install_tools(staging, environment)
        environment["VSCE_PAT"] = token
        contract.run([node, tool, "publish", "--packagePath", artifact], staging, environment)


if __name__ == "__main__":
    main()
