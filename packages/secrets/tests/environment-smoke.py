#!/usr/bin/env python3
"""Exercise the packaged resolver and CLI with fake provider processes, outside the checkout."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

binary = str(Path(sys.argv[1]).resolve())
store = str(Path(sys.argv[2]).resolve())
providers = {
    "onepassword": ("op", {"environment_id": "production-test"}),
    "infisical": ("infisical", {"project_id": "test-project", "environment": "prod"}),
    "aws-secretsmanager": ("aws", {"secret_id": "test-production"}),
    "azure-keyvault": ("az", {"vault_name": "test-production"}),
}
with tempfile.TemporaryDirectory(prefix="dockpipe-secrets-smoke-") as temporary:
    root = Path(temporary)
    tools = root / "tools"
    tools.mkdir()
    for name in ("op", "infisical", "aws", "az"):
        tool = tools / name
        tool.write_text('''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
name = Path(sys.argv[0]).name
missing = os.environ.get("FAKE_SECRET_MISSING") == "1"
value = "SECRET_SENTINEL\\nsecond line"
if name == "aws":
    print(json.dumps({"SecretString": json.dumps({} if missing else {"REMOTE_KEY": value, "UNBOUND": "private"})}))
elif name == "az":
    if missing:
        sys.exit(1)
    print(json.dumps({"value": value}))
else:
    environment = dict(os.environ, UNBOUND="private")
    if not missing:
        environment["REMOTE_KEY"] = value
    command = sys.argv[sys.argv.index("--") + 1:]
    os.execve(command[0], command, environment)
''')
        tool.chmod(0o755)
    (root / "workflow.yml").write_text(
        "name: secrets-smoke\nvault: environment\ndocker_preflight: false\nsteps:\n"
        "  - id: verify\n    kind: host\n    cwd: repo\n    run: check.sh\n"
    )
    (root / "check.sh").write_text("#!/usr/bin/env bash\nset -euo pipefail\npython3 check.py\n")
    (root / "check.sh").chmod(0o755)
    (root / "check.py").write_text(
        "import os\nfrom pathlib import Path\n"
        "assert os.environ['TOKEN'] == 'SECRET_SENTINEL\\nsecond line'\n"
        "assert 'UNBOUND' not in os.environ\n"
        "Path('result').write_text('passed')\n"
    )
    environment = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ["PATH"],
                       DOCKPIPE_VAULT_INJECT="1", DOCKPIPE_OP_INJECT="1", DOCKPIPE_COMPILE_DEPS="0",
                       DOCKPIPE_GLOBAL_ROOT=str(root / "global"), XDG_STATE_HOME=str(root / "state"),
                       XDG_CACHE_HOME=str(root / "cache"),
                       TOKEN="parent-fallback-forbidden", REMOTE_KEY="parent-fallback-forbidden")
    for provider, (_, parameters) in providers.items():
        config = {"schema": 1, "packages": {"sources": [{"kind": "tarball_dir", "path": store}]},
                  "secrets": {"environments": {"production": {"resolver": provider, "parameters": parameters,
                                                             "bindings": {"TOKEN": "REMOTE_KEY"}}}}}
        (root / "dockpipe.config.json").write_text(json.dumps(config))
        command = [binary, "--workflow-file", str(root / "workflow.yml"), "--secret-environment", "production"]
        for missing in (False, True):
            (root / "result").unlink(missing_ok=True)
            environment["FAKE_SECRET_MISSING"] = "1" if missing else "0"
            result = subprocess.run(command, cwd=root, env=environment, capture_output=True, text=True, timeout=60)
            if "SECRET_SENTINEL" in result.stdout + result.stderr:
                raise AssertionError("Secret appeared in workflow output")
            if missing:
                assert result.returncode != 0 and not (root / "result").exists(), "Missing secret did not stop steps"
            elif result.returncode != 0 or not (root / "result").exists():
                raise AssertionError(f"{provider} integration failed: {result.stderr}")
        print(f"PASS: {provider} packaged injection, multiline values, filtering, missing-value stop")
