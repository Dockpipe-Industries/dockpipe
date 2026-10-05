"""Keep runtime CI aligned with release qualification and compiler containment."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import yaml


REPOSITORY = Path(__file__).resolve().parents[3]


class RuntimeCITests(unittest.TestCase):
    def test_selector_excludes_only_separate_compiler_campaigns(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            fake_go = root / "go"
            fake_go.write_text(
                "#!/usr/bin/env bash\n"
                "set -euo pipefail\n"
                "if [[ $1 == list ]]; then\n"
                "  printf '%s\\n' dockpipe/src/cmd dockpipe/src/lib/infrastructure "
                "dockpipe/src/lib/pipelang dockpipe/src/lib/pipelang/backend "
                "dockpipe/src/lib/applicationir dockpipe/tests/pipelangcompat "
                "dockpipe/tests/containedexec dockpipe/tests/containedexec/transcript\n"
                "else\n"
                "  printf '%s\\n' \"$@\" > \"$TEST_GO_ARGUMENTS\"\n"
                "fi\n"
            )
            fake_go.chmod(0o755)
            arguments = root / "arguments"
            environment = dict(os.environ, PATH=f"{root}:{os.environ['PATH']}",
                               TEST_GO_ARGUMENTS=str(arguments))
            subprocess.run(["bash", str(REPOSITORY / "release/packaging/test-runtime.sh")],
                           env=environment, check=True, capture_output=True, text=True)
            self.assertEqual(arguments.read_text().splitlines(),
                             ["test", "dockpipe/src/cmd", "dockpipe/src/lib/infrastructure"])

    def test_host_and_nested_workflow_use_release_selector_and_pinned_tools(self):
        ci = yaml.load((REPOSITORY / ".github/workflows/ci.yml").read_text(), Loader=yaml.BaseLoader)
        nested = yaml.load((REPOSITORY / "workflows/ci/test/config.yml").read_text(), Loader=yaml.BaseLoader)
        selector = "bash release/packaging/test-runtime.sh"
        for job in ("test", "test-windows"):
            commands = [step.get("run", "") for step in ci["jobs"][job]["steps"]]
            self.assertIn(selector, commands)
            self.assertNotIn("go test ./...", "\n".join(commands))
        scan = next(step["cmd"] for step in nested["steps"] if step["id"] == "scan")
        self.assertIn(selector, scan)
        self.assertNotIn("go test ./...", scan)
        host = "\n".join(step.get("run", "") for step in ci["jobs"]["test"]["steps"])
        for tool in ("honnef.co/go/tools/cmd/staticcheck@v0.7.0",
                     "golang.org/x/vuln/cmd/govulncheck@v1.7.0",
                     "github.com/securego/gosec/v2/cmd/gosec@v2.29.0"):
            self.assertIn(f"go install {tool}", host)
            self.assertIn(f"go install {tool}", scan)
        self.assertEqual(ci["env"]["GOTOOLCHAIN"], "local")
        self.assertEqual(nested["vars"]["GOTOOLCHAIN"], "local")

    def test_json_vulnerability_findings_fail_the_host_gate(self):
        ci = yaml.load((REPOSITORY / ".github/workflows/ci.yml").read_text(), Loader=yaml.BaseLoader)
        scan = next(step["run"] for step in ci["jobs"]["test"]["steps"]
                    if step.get("name") == "govulncheck + gosec + DorkPipe CI signal bundle")
        commands = scan[scan.index("set +e"):scan.index('"$GOBIN/gosec"')]
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            scanner = root / "govulncheck"
            scanner.write_text(
                "#!/usr/bin/env bash\n"
                "if [[ $1 == -format ]]; then\n"
                "  echo '{\"finding\":{}}'\n"
                "  exit 0\n"
                "fi\n"
                "cat >/dev/null\n"
                "exit 3\n"
            )
            scanner.chmod(0o755)
            environment = dict(os.environ, GOBIN=str(root), CI_RAW_DIR=str(root))
            result = subprocess.run(["bash", "-c", commands + 'exit "$VC"'],
                                    env=environment, capture_output=True, text=True)
            self.assertEqual(result.returncode, 3, result.stderr)

    def test_release_and_ci_use_the_security_patched_toolchain(self):
        for filename in ("ci.yml", "release.yml"):
            workflow = yaml.load((REPOSITORY / ".github/workflows" / filename).read_text(), Loader=yaml.BaseLoader)
            for job in workflow["jobs"].values():
                for step in job.get("steps", []):
                    if step.get("uses", "").startswith("actions/setup-go@"):
                        self.assertEqual(step["with"]["go-version"], "1.25.13")
        for filename in ("go.mod", "packages/dorkpipe-mcp/go.mod"):
            self.assertIn("toolchain go1.25.13", (REPOSITORY / filename).read_text())

    def test_codeql_retains_go_and_actions_coverage(self):
        workflow = yaml.load((REPOSITORY / ".github/workflows/codeql.yml").read_text(), Loader=yaml.BaseLoader)
        job = workflow["jobs"]["codeql"]
        self.assertEqual(job["strategy"]["matrix"]["include"],
                         [{"language": "go", "build-mode": "autobuild"},
                          {"language": "actions", "build-mode": "none"}])
        analyze = next(step for step in job["steps"] if step.get("uses", "").startswith("github/codeql-action/analyze@"))
        self.assertEqual(analyze["with"]["category"], "/language:${{ matrix.language }}")


if __name__ == "__main__":
    unittest.main()
