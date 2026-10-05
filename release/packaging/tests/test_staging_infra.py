#!/usr/bin/env python3
"""Exercise the real staging wrapper and package pipeline with a fake provider CLI."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

REPOSITORY = Path(__file__).resolve().parents[3]
MODULE = Path("packages/cloud/storage/resolvers/r2/dockpipe.cloudflare.r2infra")
WRAPPER = REPOSITORY / "workflows/package/package-store-staging-infra/assets/scripts/plan-staging.sh"


class StagingInfraTests(unittest.TestCase):
    def test_production_settings_cannot_redirect_staging_plan(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            module = root / MODULE
            shutil.copytree(REPOSITORY / MODULE, module, ignore=shutil.ignore_patterns(".terraform", "*.tfstate*", ".pipelang"))
            (module / "terraform/production.auto.tfvars").write_text('bucket_name="production"')
            (module / "terraform/terraform.tfstate").write_text('protected production state')
            tools = root / "tools"
            tools.mkdir()
            cli = tools / "dockpipe"
            pipeline = REPOSITORY / "packages/terraform/resolvers/terraform-core/assets/scripts/terraform-pipeline.sh"
            cli.write_text('''#!/usr/bin/env bash
set -euo pipefail
case "$1" in
  scope)
    if [[ "${4:-}" == backend.hcl ]]; then
      echo "$DOCKPIPE_WORKDIR/artifacts/backend.hcl"
    else
      echo "$DOCKPIPE_WORKDIR/artifacts"
    fi
    ;;
  sdk) cat <<'SDK'
dockpipe_sdk() {
  case "$1" in
    init-script)
      ROOT="$DOCKPIPE_WORKDIR"
      SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[1]}")" && pwd)"
      WF_NS=staging-test ;;
    source) source "$TEST_PIPELINE" ;;
    die) echo "$2" >&2; exit 1 ;;
  esac
}
SDK
  ;;
  *) exit 1 ;;
esac
''')
            cli.chmod(0o755)
            terraform = tools / "terraform"
            terraform.write_text('''#!/usr/bin/env python3
import json, os, pathlib, sys
record = {"args": sys.argv[1:], "cwd": os.getcwd(), "env": {
    key: value for key, value in os.environ.items()
    if key.startswith(("TF_VAR_", "DOCKPIPE_TF_", "TF_CLI_ARGS", "TF_WORKSPACE"))}}
for arg in sys.argv:
    if arg.startswith("-backend-config="):
        record["backend"] = pathlib.Path(arg.split("=", 1)[1]).read_text()
with open(os.environ["TEST_LOG"], "a") as stream:
    stream.write(json.dumps(record) + "\\n")
if sys.argv[1] == "plan":
    pathlib.Path("staging.tfplan").write_text("fake provider plan")
''')
            terraform.chmod(0o755)
            log = root / "commands.jsonl"
            env = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ["PATH"],
                       DOCKPIPE_WORKDIR=str(root), DOCKPIPE_BIN=str(cli), TEST_LOG=str(log), TEST_PIPELINE=str(pipeline),
                       CLOUDFLARE_ACCOUNT_ID="a" * 32, CLOUDFLARE_API_TOKEN="test-token",
                       R2_STATE_ACCESS_KEY_ID="test-state", R2_STATE_SECRET_ACCESS_KEY="test-state-secret",
                       TF_VAR_zone_id="b" * 32, TF_VAR_public_hostname="packages.dockpipe.com",
                       TF_VAR_bucket_name="dockpipe", TF_VAR_enable_cache_rules="true", TF_VAR_enable_waf_baseline="true",
                       DOCKPIPE_TF_STATE_KEY="state/terraform.tfstate", DOCKPIPE_TF_STATE_BUCKET="production-state",
                       DOCKPIPE_TF_IMPORT_ARGS="unwanted import", DOCKPIPE_TF_WORKSPACE="production",
                       DOCKPIPE_TF_REMOTE_BACKEND_FILE="production.hcl", TF_WORKSPACE="production",
                       R2_TERRAFORM_COMMANDS="apply", TF_CLI_ARGS_plan="-destroy", DOCKPIPE_TF_COMMANDS="plan")
            result = subprocess.run(["bash", str(WRAPPER)], env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            calls = [json.loads(line) for line in log.read_text().splitlines()]
            self.assertEqual([call["args"][0] for call in calls], ["init", "validate", "plan"])
            self.assertIn('key    = "state/package-store-staging/terraform.tfstate"', calls[0]["backend"])
            self.assertIn('bucket = "dockpipe-tfstate"', calls[0]["backend"])
            for call in calls:
                environment = call["env"]
                self.assertEqual(environment["TF_VAR_bucket_name"], "dockpipe-staging")
                self.assertEqual(environment["TF_VAR_public_hostname"], "packages.staging.dockpipe.com")
                self.assertEqual(environment["TF_VAR_enable_cache_rules"], "false")
                self.assertEqual(environment["TF_VAR_enable_waf_baseline"], "false")
                self.assertNotIn("TF_WORKSPACE", environment)
                self.assertNotIn("TF_CLI_ARGS_plan", environment)
                self.assertNotEqual(Path(call["cwd"]), module / "terraform")
                self.assertFalse((Path(call["cwd"]) / "production.auto.tfvars").exists())
                self.assertFalse((Path(call["cwd"]) / "terraform.tfstate").exists())
            self.assertEqual((module / "terraform/terraform.tfstate").read_text(), "protected production state")
            self.assertTrue((Path(calls[-1]["cwd"]) / "staging.tfplan").is_file())
            log.unlink()
            result = subprocess.run(["bash", str(WRAPPER)], env=dict(env, DOCKPIPE_TF_COMMANDS="apply"), capture_output=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(log.exists())
