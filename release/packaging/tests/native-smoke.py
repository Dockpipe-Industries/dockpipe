#!/usr/bin/env python3
"""Run the released CLI outside its checkout with a real native host workflow."""
from pathlib import Path
import os
import subprocess
import sys
import tempfile

binary = str(Path(sys.argv[1]).resolve())
with tempfile.TemporaryDirectory(prefix="dockpipe-release-smoke-") as temporary:
    root = Path(temporary)
    environment = dict(os.environ, XDG_STATE_HOME=str(root / "state"), XDG_CACHE_HOME=str(root / "cache"))
    workflow = root / "workflow.yml"
    workflow.write_text(
        "name: release-smoke\ndocker_preflight: false\nsteps:\n"
        "  - id: native\n    kind: host\n    cwd: repo\n"
        "    run: smoke.sh\n"
    )
    script = root / "smoke.sh"
    script.write_text("#!/usr/bin/env bash\nset -euo pipefail\nprintf release-ok > release-result.txt\n")
    script.chmod(0o755)
    subprocess.run([binary, "--version"], cwd=root, check=True, timeout=30)
    subprocess.run([binary, "--workflow-file", str(workflow), "--workdir", str(root)],
                   cwd=root, env=environment, check=True, timeout=90)
    assert (root / "release-result.txt").read_text() == "release-ok"
print("PASS: native binary and host workflow outside source checkout")
