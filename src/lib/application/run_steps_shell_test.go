package application

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestHostShellRoutesBlockingAndParallelSteps(t *testing.T) {
	for _, parallel := range []bool{false, true} {
		name := "blocking"
		if parallel {
			name = "parallel"
		}
		t.Run(name, func(t *testing.T) {
			withRunStepsSeams(t)
			oldScript := runHostScriptWithShellFn
			oldCommand := runHostCommandWithShellFn
			t.Cleanup(func() {
				runHostScriptWithShellFn = oldScript
				runHostCommandWithShellFn = oldCommand
			})

			workdir := t.TempDir()
			for _, script := range []string{"setup.ps1", "extra.ps1", "strategy.sh"} {
				if err := os.WriteFile(filepath.Join(workdir, script), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			o := baseRunStepsOpts()
			o.wfRoot = workdir
			o.repoRoot = workdir
			o.projectRoot = workdir
			o.opts.Workdir = workdir
			o.wf.Name = "fixture"
			o.wf.Steps = []domain.Step{{Kind: "host", Shell: "pwsh", Run: domain.RunSpec{"setup.ps1"}, PreScript: "extra.ps1", Cmd: "Write-Output literal"}}
			o.firstStepExtra = []string{filepath.Join(workdir, "strategy.sh")}
			o.envMap["DOCKPIPE_WORKFLOW_NAME"] = "fixture"

			var scripts []string
			runHostScriptWithShellFn = func(path string, env []string, shell string) error {
				if shell != "pwsh" {
					t.Fatalf("shell = %q", shell)
				}
				scripts = append(scripts, filepath.Base(path))
				return nil
			}
			strategyCalls := 0
			runHostScriptFn = func(path string, env []string) error {
				if filepath.Base(path) != "strategy.sh" {
					t.Fatalf("step script escaped explicit shell routing: %s", path)
				}
				strategyCalls++
				return nil
			}
			commandCalls := 0
			expectedFailure := errors.New("fixture shell command failure")
			runHostCommandWithShellFn = func(command string, env []string, shell string) error {
				if shell != "pwsh" || command != "Write-Output literal" {
					t.Fatalf("shell/command = %q/%q", shell, command)
				}
				commandCalls++
				return expectedFailure
			}
			var err error
			if parallel {
				err = runParallelStepWorker(&o, 0, 1, 0, o.envMap, map[string]string{})
			} else {
				err = runBlockingStep(&o, 0, 1, map[string]string{})
			}
			if !errors.Is(err, expectedFailure) {
				t.Fatalf("explicit command failure was not propagated: %v", err)
			}
			if len(scripts) != 2 || scripts[0] != "setup.ps1" || scripts[1] != "extra.ps1" || strategyCalls != 1 || commandCalls != 1 {
				t.Fatalf("calls = scripts %v, strategy %d, command %d", scripts, strategyCalls, commandCalls)
			}
		})
	}
}

func TestHostShellNativeWorkflowAdmissionDoesNotRequireBash(t *testing.T) {
	withRunSeams(t)
	t.Setenv("PATH", t.TempDir())
	workdir := t.TempDir()
	repoRootAppFn = func() (string, error) { return workdir, nil }
	configPath := filepath.Join(workdir, "config.yml")
	config := "name: fixture\ndocker_preflight: false\nsteps:\n  - kind: host\n    shell: pwsh\n    cmd: Write-Output hello\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	called := false
	runStepsAppFn = func(options runStepsOpts) error {
		called = true
		if len(options.wf.Steps) != 1 || options.wf.Steps[0].Shell != "pwsh" {
			t.Fatalf("shell selection lost during workflow admission: %+v", options.wf.Steps)
		}
		return nil
	}
	if err := Run([]string{"--workflow-file", configPath, "--workdir", workdir}, os.Environ()); err != nil {
		t.Fatalf("native workflow was blocked before its owning shell boundary: %v", err)
	}
	if !called {
		t.Fatal("workflow did not reach step execution")
	}
}
