#!/usr/bin/env python3
"""Exercise release admission and deployment isolation from the authored workflow."""
import os
import re
from pathlib import Path
import subprocess
import tempfile
import unittest

import yaml


REPOSITORY = Path(__file__).resolve().parents[4]
WORKFLOW = Path(__file__).resolve().parents[4] / '.github/workflows/release.yml'

import importlib.util
_contract_spec = importlib.util.spec_from_file_location(
    'workflow_contract_testing', Path(__file__).resolve().parents[4] / 'workflows/shared/contract_testing.py')
_contracts = importlib.util.module_from_spec(_contract_spec)
_contract_spec.loader.exec_module(_contracts)


def expanded(command):
    return _contracts.expanded_run(Path(__file__).resolve().parents[4], command)



class ReleaseWorkflowTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workflow = yaml.load(WORKFLOW.read_text(), Loader=yaml.BaseLoader)
        cls.jobs = cls.workflow["jobs"]

    def condition(self, expression, ref, dry_run):
        """Evaluate the simple job guards with supplied GitHub context values."""
        expression = expression.removeprefix("${{").removesuffix("}}")
        context = {
            "github.ref": ref,
            "needs.meta.outputs.dry_run": dry_run,
            "vars.DEVTO_PUBLISH": "true",
            "vars.DEVTO_ARTICLE_ID": "test-article",
            "vars.DEVTO_ONE_TIME_POST": "false",
        }
        for name, value in context.items():
            expression = expression.replace(name, repr(value))
        expression = expression.replace("&&", " and ").replace("||", " or ")
        return eval(expression.strip(), {"__builtins__": {}}, {})

    def test_desktop_release_and_qualification_share_extension_capable_qt(self):
        native = yaml.load((Path(__file__).resolve().parents[4] / '.github/workflows/native-macos-symbols.yml').read_text(),
                           Loader=yaml.BaseLoader)
        native_scripts = {
            'owned-symbols': (REPOSITORY / 'workflows/qualify/native-desktop/assets/scripts/macos.sh').read_text(),
            'windows-terminal': (REPOSITORY / 'workflows/qualify/native-desktop/assets/scripts/windows.ps1').read_text(),
        }
        release_scripts = [expanded(step.get('run', '')) for job in self.jobs.values()
                           for step in job.get('steps', [])]
        installs = [script for script in [*release_scripts, *native_scripts.values()]
                    if '-m aqt install-qt' in script]
        for script in installs:
            self.assertIn('desktop 6.10.3 ', script)
            self.assertIn('/Qt/6.10.3/', script)
            self.assertIn('--modules qtwebengine qtwebchannel qtpositioning', script)
        self.assertEqual(len(installs), 4)

        developer = (Path(__file__).resolve().parents[4] / 'workflows/install/windows/assets/scripts/dev-install.ps1').read_text()
        self.assertIn('$SourceExe', developer)
        self.assertNotIn('go build', developer)
        self.assertNotIn('Stop-Process', developer)
        flatpak = (Path(__file__).resolve().parents[4] / 'workflows/build/flatpak/assets/scripts/build.sh').read_text()
        self.assertIn("runtime_branch=6.10", flatpak)
        self.assertIn("io.qt.qtwebengine.BaseApp", flatpak)
        for job_name in ("owned-symbols", "windows-terminal"):
            job = native["jobs"][job_name]
            scripts = native_scripts[job_name]
            if job_name == "windows-terminal":
                commands = [line.strip() for line in scripts.splitlines()]
                self.assertIn("-DBUILD_TESTING=ON", scripts)
                self.assertIn("cmake --build $build --config Release --parallel $Jobs", commands)
                ctest_commands = [line for line in commands if line.startswith("ctest ")]
                self.assertEqual(len(ctest_commands), 3)
                for command in ctest_commands:
                    self.assertIn('--no-tests=error', command)
                self.assertIn("-R '^terminal-session-win$'", ctest_commands[0])
                self.assertIn("-R '^desktop-app-window-win$'", ctest_commands[1])
                self.assertIn('--output-junit $embeddingReport', ctest_commands[1])
                self.assertIn('--timeout 120 --stop-on-failure', ctest_commands[2])
                self.assertIn('--require desktop-app-window-win', scripts)
                self.assertRegex(scripts, r"(?s)if \(\$LASTEXITCODE -ne 0\) \{\s+try \{\s+& ./src/dockpipe-app/tests/TraceWindowsFixtureLoader.ps1.*?throw 'Native embedding qualification failed\.'")
                self.assertIn("if ($qualificationExit -ne 0) { throw 'Native terminal qualification failed.' }", scripts)
                continue
            self.assertIn("browser-extensions-test", scripts)
            self.assertIn("browser-extension-package-test", scripts)
            self.assertIn("browser-extension-store-test", scripts)
            patterns = re.findall(r"-R ['\"]([^'\"]+)['\"]", scripts)
            for test_name in ("browser-extensions", "browser-extension-package", "browser-extension-store", "browser-extension-crx"):
                self.assertTrue(any(re.fullmatch(pattern, test_name) for pattern in patterns), test_name)
        macos_scripts = native_scripts["owned-symbols"]
        self.assertIn("terminal-widget-test", macos_scripts)
        macos_patterns = re.findall(r"-R ['\"]([^'\"]+)['\"]", macos_scripts)
        self.assertTrue(any(re.fullmatch(pattern, "terminal-widget") for pattern in macos_patterns))

    def test_metadata_admits_dry_runs_and_rejects_non_master_publication(self):
        metadata = next(step for step in self.jobs["meta"]["steps"] if step.get("id") == "m")
        refs = [
            "refs/heads/js/pipelang", "refs/heads/js/dev", "refs/heads/dev",
            "refs/heads/staging", "refs/heads/master", "refs/tags/master",
        ]
        with tempfile.TemporaryDirectory(prefix="dockpipe-workflow-test-") as temporary:
            output = Path(temporary) / "outputs"
            for ref in refs:
                for event, requested_dry_run in [
                    ("workflow_dispatch", "true"), ("workflow_dispatch", "false"),
                    ("workflow_dispatch", ""), ("push", ""),
                ]:
                    with self.subTest(ref=ref, event=event, dry_run=requested_dry_run):
                        output.write_text("")
                        dry_run = event == "workflow_dispatch" and requested_dry_run != "false"
                        allowed = dry_run or ref == "refs/heads/master"
                        environment = dict(os.environ, GITHUB_REF=ref, GITHUB_EVENT_NAME=event,
                                           GITHUB_OUTPUT=str(output), INPUT_VERSION="", RELEASE_ENABLED="true",
                                           INPUT_DRY_RUN=requested_dry_run, INPUT_BUILD_MSI="true")
                        result = subprocess.run(["bash", "-c", metadata["run"]], cwd=REPOSITORY,
                                                env=environment, text=True, capture_output=True)
                        self.assertEqual(result.returncode == 0, allowed, result.stdout + result.stderr)
                        if allowed:
                            self.assertIn(f"dry_run={str(dry_run).lower()}\n", output.read_text())
                        else:
                            self.assertIn("Production releases require master", result.stdout)
                            self.assertEqual(output.read_text(), "")

    def test_deployment_jobs_only_admit_explicit_production_on_master(self):
        for name in ("publish", "devto"):
            job = self.jobs[name]
            self.assertEqual(job["environment"], "release")
            for ref in ("refs/heads/js/pipelang", "refs/heads/js/dev", "refs/heads/dev",
                        "refs/heads/staging", "refs/heads/master", "refs/tags/master"):
                for dry_run in ("true", "false", ""):
                    with self.subTest(job=name, ref=ref, dry_run=dry_run):
                        self.assertEqual(self.condition(job["if"], ref, dry_run),
                                         ref == "refs/heads/master" and dry_run == "false")

    def test_artifact_verification_has_no_deployment_or_production_credentials(self):
        for name, job in self.jobs.items():
            if name in ("publish", "publish-staging", "devto"):
                continue
            self.assertNotIn("environment", job, name)
            self.assertNotIn("secrets.", yaml.dump(job), name)
            self.assertNotIn("write", job.get("permissions", {}).values(), name)

        self.assertNotIn("attest-forgepipe-sdk", self.jobs)
        self.assertNotIn("forgepipe_sdk_tag", self.workflow["on"]["workflow_dispatch"]["inputs"])

        assembly = self.jobs["assemble"]
        self.assertEqual(assembly["needs"], ["meta", "build-unix", "build-msi", "build-flatpak"])
        self.assertEqual(self.jobs["publish"]["needs"], ["meta", "assemble"])
        self.assertEqual(self.jobs["devto"]["needs"], ["meta", "publish"])
        self.assertIn("needs.build-unix.result == 'success'", assembly["if"])
        self.assertIn("needs.build-msi.result == 'success'", assembly["if"])
        self.assertIn("needs.build-msi.result == 'skipped'", assembly["if"])
        self.assertIn("needs.build-flatpak.result == 'success'", assembly["if"])
        self.assertEqual(self.jobs["build-flatpak"]["uses"], "./.github/workflows/flatpak.yml")
        flatpak = yaml.load((Path(__file__).resolve().parents[4] / '.github/workflows/flatpak.yml').read_text(), Loader=yaml.BaseLoader)
        self.assertNotIn("secrets", self.jobs["build-flatpak"])
        self.assertEqual(flatpak["permissions"], {"contents": "read"})
        for job in flatpak["jobs"].values():
            self.assertNotIn("environment", job)
            self.assertNotIn("secrets.", yaml.dump(job))

        dry_run_upload = next(step for step in assembly["steps"]
                              if step.get("name") == "Upload dry-run artifacts")
        self.assertTrue(self.condition(dry_run_upload["if"], "refs/heads/js/pipelang", "true"))
        self.assertFalse(self.condition(dry_run_upload["if"], "refs/heads/master", "false"))
        self.assertIn("release/apt/", dry_run_upload["with"]["path"])
        prepared = next(step for step in assembly["steps"]
                        if step.get("name") == "Upload prepared production artifacts")
        download = next(step for step in self.jobs["publish"]["steps"]
                        if step.get("uses", "").startswith("actions/download-artifact@"))
        self.assertEqual(prepared["with"]["name"], download["with"]["name"])
        self.assertEqual(prepared["with"]["path"].rstrip("/"), download["with"]["path"])

    def test_staging_admission_supports_enabled_forks_and_rejects_other_events(self):
        metadata = next(step for step in self.jobs["meta"]["steps"] if step.get("id") == "m")
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / "outputs"
            for event in ("push", "pull_request", "workflow_dispatch"):
                for ref in ("refs/heads/staging", "refs/heads/master", "refs/heads/dev"):
                    for repository, enabled in (("upstream/project", "true"), ("fork/project", "true"), ("fork/project", "false")):
                        output.write_text("")
                        environment = dict(os.environ, GITHUB_REF=ref, GITHUB_EVENT_NAME=event,
                                           GITHUB_REPOSITORY=repository, RELEASE_ENABLED=enabled, GITHUB_SHA="a" * 40,
                                           GITHUB_RUN_ID="1234", GITHUB_RUN_ATTEMPT="2",
                                           GITHUB_OUTPUT=str(output), INPUT_STAGING="true")
                        result = subprocess.run(["bash", "-c", metadata["run"]], cwd=REPOSITORY,
                                                env=environment, text=True, capture_output=True)
                        allowed = event == "push" and ref == "refs/heads/staging" and enabled == "true"
                        self.assertEqual(result.returncode == 0, allowed, result.stdout + result.stderr)
                        if allowed:
                            self.assertIn("staging_suffix=staging.1234.2.aaaaaaaaaaaa", output.read_text())
                            self.assertIn("dry_run=false", output.read_text())
                        else:
                            self.assertEqual(output.read_text(), "")

    def test_staging_call_waits_for_tests_and_publishes_only_prereleases(self):
        ci = yaml.load((Path(__file__).resolve().parents[4] / '.github/workflows/ci.yml').read_text(), Loader=yaml.BaseLoader)
        call = ci["jobs"]["staging-release"]
        self.assertEqual(call["needs"], ["test", "test-windows"])
        self.assertNotIn("cockpit-native", ci["jobs"])
        compatibility = yaml.load(
            (Path(__file__).resolve().parents[4] / '.github/workflows/cockpit-sdk-compatibility.yml').read_text(),
            Loader=yaml.BaseLoader)
        self.assertIn("workflow_dispatch", compatibility["on"])
        self.assertEqual(set(compatibility["jobs"]), {"cockpit-native"})
        native = compatibility["jobs"]["cockpit-native"]
        self.assertNotIn("continue-on-error", native)
        self.assertNotIn("if", native)
        for step in native["steps"]:
            self.assertNotIn("continue-on-error", step)
        self.assertEqual(native["runs-on"], "ubuntu-22.04")
        self.assertEqual(native["permissions"], {"contents": "read", "attestations": "read"})
        qt_install = next(step for step in native["steps"] if step.get("name") == "Install pinned Qt ABI")
        self.assertIn('DOCKPIPE_QT_ROOT=$RUNNER_TEMP/Qt/6.10.3/gcc_64', expanded(qt_install["run"]))
        self.assertNotIn("DOCKPIPE_QT_ROOT", native["env"])
        qualification = next(step for step in native["steps"]
                             if step.get("name") == "Verify public publisher and qualify installed Cockpit")
        script = expanded(qualification["run"])
        self.assertIn("--lock workflows/qualify/cockpit/assets/cockpit.lock.json", script)
        self.assertIn('cmake -S src/dockpipe-app', script)
        self.assertIn('-DDOCKPIPE_COCKPIT_ASSETS="$RUNNER_TEMP/cockpit-runtime/package/resolvers/cockpit/assets"', script)
        prepare = script.index("prepare-cockpit.py prepare")
        configure = script.index("cmake -S ")
        verify_compiler = script.index("prepare-cockpit.py verify-compiler")
        build = script.index("cmake --build ")
        qualify = script.index("ctest --test-dir ")
        self.assertLess(prepare, configure)
        self.assertLess(configure, verify_compiler)
        self.assertLess(verify_compiler, build)
        self.assertLess(build, qualify)
        self.assertIn('--store "$RUNNER_TEMP/cockpit-runtime/store"', script)
        self.assertIn("--target cockpit-package-rendering-test", script)
        self.assertIn("-R '^cockpit-package-rendering$' --no-tests=error", script)
        self.assertEqual(call["uses"], "./.github/workflows/release.yml")
        self.assertEqual(call["with"], {"staging": "true"})
        self.assertIn("github.event_name == 'push'", call["if"])
        self.assertIn("github.ref == 'refs/heads/staging'", call["if"])
        self.assertEqual(call["secrets"], "inherit")
        self.assertIn("vars.DOCKPIPE_RELEASE_ENABLED == 'true'", call["if"])
        job = self.jobs["publish-staging"]
        self.assertEqual(job["needs"], ["meta", "assemble"])
        self.assertEqual(job["environment"], "release-staging")
        self.assertIn("needs.meta.outputs.candidate != ''", job["if"])
        release = next(step for step in job["steps"] if step.get("uses", "").startswith("softprops/"))
        self.assertEqual(release["with"]["prerelease"], "true")
        self.assertEqual(release["with"]["make_latest"], "false")
        self.assertEqual(release["with"]["target_commitish"], "${{ github.sha }}")
        self.assertIn("Require the current staging head", [step.get("name") for step in job["steps"]])

    def test_release_tooling_contracts_run_before_the_ci_cli_build(self):
        ci = yaml.load((Path(__file__).resolve().parents[4] / '.github/workflows/ci.yml').read_text(), Loader=yaml.BaseLoader)
        steps = ci["jobs"]["test"]["steps"]
        by_name = {step.get("name"): index for index, step in enumerate(steps) if step.get("name")}
        install = by_name["Install build and shell-test dependencies"]
        contracts = by_name["Release tooling contracts before builds"]
        build = by_name["Build CLI (Bash tests use src/bin/dockpipe)"]
        self.assertLess(install, contracts)
        self.assertLess(contracts, build)
        for dependency in ("python3-yaml", "apt-utils", "gnupg"):
            self.assertIn(dependency, steps[install]["run"].split())
        self.assertEqual(steps[contracts]["run"],
                         "python3 workflows/shared/launch.py workflows/ci/workflow-contracts/config.yml")
        self.assertNotIn("if", steps[contracts])
        self.assertNotIn("continue-on-error", steps[contracts])
        release_gate = next(step for step in self.jobs["build-unix"]["steps"]
                            if step.get("name") == "Release tooling tests")
        self.assertIn("workflows/ci/workflow-contracts/config.yml", release_gate["run"])
        self.assertEqual(release_gate["if"], "matrix.platform == 'linux-amd64'")

    def test_manual_runs_default_to_dry_run_and_only_master_pushes_trigger(self):
        triggers = self.workflow["on"]
        self.assertEqual(triggers["push"]["branches"], ["master"])
        self.assertEqual(triggers["workflow_dispatch"]["inputs"]["dry_run"]["default"], "true")

    def test_runtime_release_does_not_fetch_optional_catalog_or_cockpit(self):
        steps = self.jobs["build-unix"]["steps"]
        build = next(step for step in steps
                     if step.get("name") == "Build native CLI and installers")
        self.assertIn("workflows/build/platform/config.yml", build["run"])
        for job_name in ("build-unix", "build-msi"):
            for step in self.jobs[job_name]["steps"]:
                script = expanded(step.get("run", ""))
                self.assertNotIn("prepare-cockpit.py", script)
                self.assertNotIn("--finalize-cockpit", script)
                self.assertNotIn("DOCKPIPE_QUALIFY_COCKPIT", step.get("env", {}))
        desktop = next(step for step in steps
                       if step.get("name") == "Build Linux desktop against Ubuntu 22.04")
        self.assertEqual(desktop["if"], "runner.os == 'Linux'")
        self.assertNotIn("continue-on-error", desktop)
        script = (Path(__file__).resolve().parents[4] / 'workflows/build/platform/assets/scripts/assemble.py').read_text()
        self.assertNotIn('ingest-pinned-stores.py', script)
        self.assertNotIn('pinned-stores.lock.json', script)
        self.assertIn("os.environ.get('DOCKPIPE_RELEASE_STORE')", script)
        self.assertIn("'verify-store', selected", script)
        self.assertNotIn('--core-only', script)

    def test_version_generation_uses_all_tags_and_fails_on_lookup_error(self):
        step = next(step for step in self.jobs["meta"]["steps"] if step.get("id") == "version")
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            gh = root / "gh"
            gh.write_text('#!/bin/sh\n[ "$2" = "--paginate" ] || exit 9\n'
                          'printf "v0.6.0-staging.123.1.aaaaaaaaaaaa\\nv0.6.3\\n"\n')
            gh.chmod(0o755)
            output = root / "outputs"
            env = dict(os.environ, PATH=f"{root}:{os.environ['PATH']}", RUNNER_TEMP=temporary,
                       GITHUB_OUTPUT=str(output), BASE_VERSION="0.6.0",
                       STAGING_SUFFIX="staging.456.2.bbbbbbbbbbbb", GITHUB_REPOSITORY="owner/repo")
            subprocess.run(["bash", "-c", step["run"]], cwd=REPOSITORY, env=env, check=True)
            self.assertIn("version=0.6.4\n", output.read_text())
            self.assertIn("candidate=0.6.4-staging.456.2.bbbbbbbbbbbb\n", output.read_text())
            output.unlink()
            gh.write_text('#!/bin/sh\nexit 1\n')
            failed = subprocess.run(["bash", "-c", step["run"]], cwd=REPOSITORY, env=env)
            self.assertNotEqual(failed.returncode, 0)
            self.assertFalse(output.exists())
        self.assertEqual(self.jobs["meta"]["outputs"]["version"], "${{ steps.version.outputs.version }}")
        concurrency = self.workflow["concurrency"]
        self.assertEqual(concurrency["cancel-in-progress"], "false")
        self.assertTrue(concurrency["group"].endswith("|| 'publish' }}"))

    def test_release_notes_are_generated_only_in_publication_jobs(self):
        for job_name in ("publish", "publish-staging"):
            job = self.jobs[job_name]
            step = next(step for step in job["steps"] if step.get("name") == "Create GitHub release")
            self.assertEqual(step["with"]["generate_release_notes"], "true")
            self.assertEqual(step["with"]["previous_tag"], "${{ needs.meta.outputs.previous_tag }}")
            self.assertEqual(step["with"]["body_path"], "release/artifacts/RELEASE_NOTES.md")
        self.assertEqual(self.jobs["assemble"]["permissions"]["contents"], "read")
        step = next(step for step in self.jobs["assemble"]["steps"]
                    if step.get("name") == "Prepare release notes body")
        self.assertIn("workflows/release/metadata/notes.yml", step["run"])
        self.assertNotIn("releasenotes/", WORKFLOW.read_text())

    def test_ship_gate_validates_version_without_a_notes_file(self):
        ci = yaml.load((REPOSITORY / '.github/workflows/ci.yml').read_text(), Loader=yaml.BaseLoader)
        gate = next(step for step in ci["jobs"]["test"]["steps"]
                    if step.get("name") == "Release version (PRs targeting master only)")
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for version, valid in [("0.6.29", True), ("1.0.0", True), ("0.06.0", False),
                                   ("1.2", False), ("1.2.3-rc1", False)]:
                with self.subTest(version=version):
                    (root / "VERSION").write_text(version + "\n")
                    result = subprocess.run(["bash", "-c", gate["run"]], cwd=root,
                                            capture_output=True, text=True)
                    self.assertEqual(result.returncode == 0, valid, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
