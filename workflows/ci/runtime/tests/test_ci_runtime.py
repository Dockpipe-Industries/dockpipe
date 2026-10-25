"""Keep runtime CI aligned with release qualification and compiler containment."""
import json
import os
import shlex
import sys
from pathlib import Path
import subprocess
import tempfile
import unittest

import yaml


REPOSITORY = Path(__file__).resolve().parents[4]

import importlib.util
_contract_spec = importlib.util.spec_from_file_location(
    'workflow_contract_testing', REPOSITORY / 'workflows/shared/contract_testing.py')
_contracts = importlib.util.module_from_spec(_contract_spec)
_contract_spec.loader.exec_module(_contracts)


def expanded(command):
    return _contracts.expanded_run(REPOSITORY, command)


class RuntimeCITests(unittest.TestCase):
    def test_selector_qualifies_every_owned_runtime_package(self):
        workflow = yaml.safe_load((REPOSITORY / 'workflows/ci/runtime/config.yml').read_text())
        command = shlex.split(workflow['steps'][0]['cmd'])
        self.assertEqual(command, ['python3', 'workflows/shared/run.py', 'python3',
                                   'workflows/ci/core/assets/scripts/test.py'])
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'test').write_text(
                'import json, os, pathlib, sys\n'
                'pathlib.Path(os.environ["TEST_GO_ARGUMENTS"]).write_text(json.dumps(sys.argv[1:]))\n'
            )
            arguments = root / 'arguments'
            environment = dict(os.environ, DOCKPIPE_TEST_GO=sys.executable,
                               DOCKPIPE_SOURCE_ROOT=str(root), TEST_GO_ARGUMENTS=str(arguments))
            for options in ([], ['-p', '2', './src/lib/...']):
                environment['DOCKPIPE_ARGS_JSON'] = json.dumps(options)
                subprocess.run([sys.executable, str(REPOSITORY / command[1]), sys.executable,
                                str(REPOSITORY / command[3])], cwd=root, env=environment,
                               check=True, capture_output=True, text=True)
                self.assertEqual(json.loads(arguments.read_text()), options or ['./...'])

    def test_host_and_nested_workflow_use_release_selector_and_pinned_tools(self):
        ci = yaml.load((Path(__file__).resolve().parents[4] / '.github/workflows/ci.yml').read_text(), Loader=yaml.BaseLoader)
        nested = yaml.load((Path(__file__).resolve().parents[4] / 'workflows/ci/test/config.yml').read_text(), Loader=yaml.BaseLoader)
        selector = "python3 workflows/shared/launch.py workflows/ci/runtime/config.yml"
        for job in ("test", "test-windows"):
            commands = [step.get("run", "") for step in ci["jobs"][job]["steps"]]
            self.assertIn(selector, commands)
            self.assertNotIn("go test ./...", "\n".join(commands))
        scan = next(step["cmd"] for step in nested["steps"] if step["id"] == "scan")
        self.assertIn("python3 workflows/ci/core/assets/scripts/test.py", scan)
        self.assertNotIn("go test ./...", scan)
        host = "\n".join(expanded(step.get("run", "")) for step in ci["jobs"]["test"]["steps"])
        for tool in ("honnef.co/go/tools/cmd/staticcheck@v0.7.0",
                     "golang.org/x/vuln/cmd/govulncheck@v1.7.0",
                     "github.com/securego/gosec/v2/cmd/gosec@v2.29.0"):
            self.assertIn(f"go install {tool}", host)
            self.assertIn(f"go install {tool}", scan)
        self.assertEqual(ci["env"]["GOTOOLCHAIN"], "local")
        self.assertEqual(nested["vars"]["GOTOOLCHAIN"], "local")

    def test_json_vulnerability_findings_fail_the_host_gate(self):
        ci = yaml.load((Path(__file__).resolve().parents[4] / '.github/workflows/ci.yml').read_text(), Loader=yaml.BaseLoader)
        scan = next(expanded(step["run"]) for step in ci["jobs"]["test"]["steps"]
                    if step.get("name") == "govulncheck + gosec")
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

    def test_security_reports_keep_scope_and_findings_on_failure(self):
        script = REPOSITORY / 'workflows/ci/repository/assets/scripts/security.sh'
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            tools = root / 'tools'
            tools.mkdir()
            binaries = root / 'go/bin'
            binaries.mkdir(parents=True)
            cli = root / 'src/bin/dockpipe'
            cli.parent.mkdir(parents=True)
            cli.write_text('#!/bin/sh\nprintf "%s\\n" "$TEST_CI_ARTIFACTS"\n')
            go = tools / 'go'
            go.write_text('#!/bin/sh\nif [ "$1" = env ]; then printf "%s\\n" "$TEST_GOPATH"; fi\n')
            vulnerability = binaries / 'govulncheck'
            vulnerability.write_text('#!/bin/sh\nif [ "$1" = -format ]; then printf "{}\\n"; else cat >/dev/null; fi\n')
            issue = {'file': 'src/lib/fixture.go', 'line': '12', 'rule_id': 'G104',
                     'severity': 'LOW', 'confidence': 'HIGH', 'details': 'Errors unhandled'}
            report = json.dumps({'Issues': [issue], 'Stats': {'found': 1}})
            security = binaries / 'gosec'
            security.write_text(
                '#!/bin/sh\n'
                'for option do\n'
                '  case "$option" in -out=*) output="${option#-out=}" ;; esac\n'
                'done\n'
                f"printf '%s\\n' '{report}' > \"$output\"\n"
                'exit 1\n'
            )
            for executable in (cli, go, vulnerability, security):
                executable.chmod(0o755)
            artifacts = root / 'scoped workflow artifacts/ci-raw'
            github_environment = root / 'github-environment'
            environment = dict(os.environ, PATH=f"{tools}:{os.environ['PATH']}",
                               DOCKPIPE_WORKDIR=str(root), TEST_CI_ARTIFACTS=str(artifacts),
                               TEST_GOPATH=str(root / 'go'), GITHUB_ENV=str(github_environment))
            result = subprocess.run(['bash', str(script)], cwd=root, env=environment,
                                    capture_output=True, text=True)
            self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
            self.assertIn('src/lib/fixture.go:12: G104 [LOW/HIGH] Errors unhandled', result.stdout)
            self.assertEqual(json.loads((artifacts / 'gosec.json').read_text())['Issues'], [issue])
            self.assertTrue((artifacts / 'govulncheck.json').is_file())
            self.assertEqual(github_environment.read_text(), f'DOCKPIPE_CI_RAW_DIR={artifacts}\n')

        ci = yaml.load((REPOSITORY / '.github/workflows/ci.yml').read_text(), Loader=yaml.BaseLoader)
        upload = next(step for step in ci['jobs']['test']['steps']
                      if step.get('name') == 'Upload CI scan receipts')
        self.assertEqual(upload['with']['path'], '${{ env.DOCKPIPE_CI_RAW_DIR }}')
        self.assertEqual(upload['with']['include-hidden-files'], 'true')
        self.assertIn('always()', upload['if'])

    def test_windows_release_and_worktree_entrypoints_keep_real_contracts(self):
        windows = yaml.safe_load((REPOSITORY / 'workflows/ci/runtime/regressions-release-windows.yml').read_text())
        regression = shlex.split(windows['steps'][0]['cmd'])
        self.assertEqual(regression, ['python3', 'workflows/shared/run.py', 'python3',
                                      'workflows/ci/core/assets/scripts/test.py'])
        unix = (REPOSITORY / 'workflows/ci/runtime/assets/scripts/regressions-release.sh').read_text()
        self.assertIn('python3 workflows/ci/core/assets/scripts/test.py', unix)
        self.assertIn('bash tests/run_tests.sh', unix)
        smoke = yaml.safe_load((REPOSITORY / 'workflows/ci/runtime/smoke-release-windows.yml').read_text())
        self.assertIn('python3 workflows/ci/runtime/assets/scripts/native-smoke.py src/bin/dockpipe.exe',
                      smoke['steps'][0]['cmd'])
        ci = yaml.safe_load((REPOSITORY / '.github/workflows/ci.yml').read_text())
        commands = [step.get('run', '') for step in ci['jobs']['test-windows']['steps']]
        self.assertIn('python3 workflows/shared/launch.py workflows/ci/repository/worktree.yml', commands)
        worktree = (REPOSITORY / 'workflows/ci/repository/assets/scripts/test-worktree.py').read_text()
        self.assertIn("'tests/unit-tests/test_clone_worktree_include.sh'", worktree)
        self.assertIn("'DOCKPIPE_TEST_DOCKPIPE_BIN'", worktree)
        self.assertIn("'scope', 'artifacts', 'worktree-cli'", worktree)
        self.assertIn("'./src/cmd'", worktree)

    def test_ci_integration_uses_own_workflow_and_unchanged_assertions(self):
        ci = yaml.safe_load((REPOSITORY / '.github/workflows/ci.yml').read_text())
        step = next(step for step in ci['jobs']['test']['steps']
                    if step.get('name') == 'Integration tests (Docker)')
        self.assertEqual(step['run'], 'python3 workflows/shared/launch.py workflows/ci/integration/config.yml')
        integration = yaml.safe_load((REPOSITORY / 'workflows/ci/integration/config.yml').read_text())
        self.assertEqual(integration['steps'][0]['cmd'],
                         'python3 workflows/shared/run.py bash tests/integration-tests/run.sh')
        self.assertTrue((REPOSITORY / 'tests/integration-tests/run.sh').is_file())

    def test_release_and_ci_use_the_security_patched_toolchain(self):
        workflow_names = (
            "ci.yml",
            "release.yml",
            "flatpak.yml",
            "native-macos-symbols.yml",
            "compiler-bootstrap.yml",
        )
        for filename in workflow_names:
            workflow = yaml.load((Path(__file__).resolve().parents[4] / '.github/workflows' / filename).read_text(), Loader=yaml.BaseLoader)
            for job in workflow["jobs"].values():
                for step in job.get("steps", []):
                    if step.get("uses", "").startswith("actions/setup-go@"):
                        self.assertEqual(step["with"]["go-version"], "1.26.10")
        self.assertIn("toolchain go1.26.10", (Path(__file__).resolve().parents[4] / 'go.mod').read_text())

        nested = yaml.load((Path(__file__).resolve().parents[4] / 'workflows/ci/test/config.yml').read_text(), Loader=yaml.BaseLoader)
        for step in nested["steps"]:
            self.assertEqual(step["isolate"], "golang:1.26.10-bookworm")
        lock = json.loads((Path(__file__).resolve().parents[4] / 'workflows/build/flatpak/assets/package-tools.lock.json').read_text())
        self.assertEqual(lock["tools"]["go"]["version"], "1.26.10")
        self.assertEqual(lock["tools"]["go"]["url"], "https://go.dev/dl/go1.26.10.linux-amd64.tar.gz")

    def test_codeql_retains_go_and_actions_coverage(self):
        workflow = yaml.load((Path(__file__).resolve().parents[4] / '.github/workflows/codeql.yml').read_text(), Loader=yaml.BaseLoader)
        job = workflow["jobs"]["codeql"]
        self.assertEqual(job["strategy"]["matrix"]["include"],
                         [{"language": "go", "build-mode": "autobuild"},
                          {"language": "actions", "build-mode": "none"}])
        analyze = next(step for step in job["steps"] if step.get("uses", "").startswith("github/codeql-action/analyze@"))
        self.assertEqual(analyze["with"]["category"], "/language:${{ matrix.language }}")


if __name__ == "__main__":
    unittest.main()
