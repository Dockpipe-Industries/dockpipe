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
    "infisical": ("infisical", {"project_id": "test-project", "environment": "prod",
                              "profile": "smoke-test", "domain": "https://vault.example/api"}),
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
if name == "infisical" and sys.argv[1:3] == ["login", "status"]:
    needs_login = os.environ.get("FAKE_NEEDS_LOGIN") == "1" and not Path("logged-in").exists()
    print(json.dumps({"sessions": [] if needs_login else [{"status": "authenticated"}]}))
    sys.exit(1 if needs_login else 0)
elif name == "infisical" and sys.argv[1] == "login":
    print("SECRET_SENTINEL", file=sys.stderr)
    print("To complete your login: https://vault.example/login?callback_port=54321", file=sys.stderr)
    Path("logged-in").write_text("authenticated")
elif name == "aws":
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
    environment.pop("INFISICAL_TOKEN", None)
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

        if provider == "infisical" and os.name != "nt":
            import pty
            import select

            (tools / "xdg-open").write_text("#!/bin/sh\nexit 0\n")
            (tools / "xdg-open").chmod(0o755)
            environment.update(FAKE_NEEDS_LOGIN="1", FAKE_SECRET_MISSING="0", CI="true")
            result = subprocess.run(command, cwd=root, env=environment, capture_output=True, text=True, timeout=60)
            assert result.returncode != 0 and "authentication required" in result.stderr
            assert not (root / "logged-in").exists(), "CI started a browser login"

            environment["CI"] = "false"
            master, slave = pty.openpty()
            try:
                result = subprocess.run(command, cwd=root, env=environment, stdin=slave, stderr=slave,
                                        stdout=subprocess.PIPE, text=True, timeout=60)
                # Keep the slave open until buffered output is drained: macOS can
                # discard unread terminal output when the last slave is closed.
                # The child has exited, so stop once no buffered bytes are ready.
                diagnostic = bytearray()
                while select.select([master], [], [], 0)[0]:
                    chunk = os.read(master, 4096)
                    if not chunk:
                        break
                    diagnostic.extend(chunk)
                assert result.returncode == 0, diagnostic.decode()
                assert b"SECRET_SENTINEL" not in diagnostic and "SECRET_SENTINEL" not in result.stdout
                assert b"https://vault.example/login?callback_port=54321" in diagnostic
                assert (root / "logged-in").exists() and (root / "result").read_text() == "passed"
            finally:
                os.close(slave)
                os.close(master)
            environment.pop("FAKE_NEEDS_LOGIN")
            print("PASS: Infisical terminal login resumes workflow; CI never logs in; login secrets withheld")
