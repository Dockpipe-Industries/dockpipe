#!/usr/bin/env python3
"""Install an explicitly selected qualified VSIX in the local editor."""
import argparse
import json
import os
from pathlib import Path
import shutil
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "editor/shared"))
import contract


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--artifact-dir", type=Path, required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--sha256", required=True)
    parser.add_argument("--editor", default="code")
    options = parser.parse_args(json.loads(os.environ.get("DOCKPIPE_ARGS_JSON", "[]")))
    artifact = contract.qualify_artifact(options.artifact_dir.resolve(strict=True), options.version, options.sha256)
    executable = shutil.which(options.editor)
    if not executable:
        raise ValueError("The selected editor CLI is unavailable")
    contract.run([executable, "--install-extension", artifact], contract.ROOT, contract.clean_environment())


if __name__ == "__main__":
    main()
