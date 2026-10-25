#!/usr/bin/env python3
"""Run the released CLI outside its checkout with a real native host workflow."""
from pathlib import Path
import importlib.util
import os
import subprocess
import sys
import tempfile

binary = str(Path(sys.argv[1]).resolve())
with tempfile.TemporaryDirectory(prefix="dockpipe-release-smoke-") as temporary:
    root = Path(temporary)
    environment = {key: value for key, value in os.environ.items()
                   if not key.startswith('DOCKPIPE_') or key == 'DOCKPIPE_HOST_BASH_BIN'}
    environment.update(XDG_STATE_HOME=str(root / 'state'), XDG_CACHE_HOME=str(root / 'cache'),
                       DOCKPIPE_GLOBAL_ROOT=str(root / 'global'), DOCKPIPE_SYSTEM_ROOT=str(root / 'system'))
    if sys.platform == 'win32':
        shared = Path(__file__).resolve().parents[4] / 'shared/run.py'
        specification = importlib.util.spec_from_file_location('workflow_host_tools', shared)
        host_tools = importlib.util.module_from_spec(specification)
        specification.loader.exec_module(host_tools)
        _, environment = host_tools.host_command(['bash'], environment, sys.platform)
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
