package application

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestNestedWorkflowAdmissionPrecedesSetup(t *testing.T) {
	unsupported := "windows"
	if runtime.GOOS == "windows" {
		unsupported = "linux"
	}
	for _, declaration := range []struct {
		name      string
		yaml      string
		errorText string
	}{
		{"required", "dependencies:\n  host:\n    - command: dockpipe-admission-absent-tool\n", "missing required host dependencies"},
		{"platform", "platforms: [" + unsupported + "]\n", "does not support host platform"},
	} {
		for _, mode := range []string{"package", "step-resolver", "root-resolver"} {
			t.Run(declaration.name+"/"+mode, func(t *testing.T) {
				withRunSeams(t)
				withRunStepsSeams(t)
				root := writeAdmissionFixture(t, declaration.yaml)
				repoRootAppFn = func() (string, error) { return root, nil }
				setupCalled := false
				sourceHostScriptFn = func(string, []string) (map[string]string, error) {
					setupCalled = true
					return map[string]string{}, nil
				}
				sourceHostScriptAppFn = sourceHostScriptFn
				childCalled := false
				runHostCommandFn = func(string, []string) error {
					childCalled = true
					return nil
				}
				runStepsAppFn = func(runStepsOpts) error {
					childCalled = true
					return nil
				}
				var err error
				if mode == "root-resolver" {
					err = Run([]string{"--workdir", root, "--resolver", "delegate", "--pre-script", filepath.Join(root, "setup.sh"), "--", "unused"}, []string{})
				} else {
					o := baseRunStepsOpts()
					o.repoRoot, o.projectRoot, o.wfRoot = root, root, root
					o.opts.Workdir = root
					o.envMap["DOCKPIPE_WORKDIR"] = root
					step := domain.Step{Run: []string{"setup.sh"}}
					if mode == "package" {
						step.WorkflowName, step.Package = "child", "admission-fixture"
					} else {
						step.Resolver = "delegate"
					}
					o.wf.Steps = []domain.Step{step}
					err = runBlockingStep(&o, 0, 1, map[string]string{})
				}
				if err == nil || !strings.Contains(err.Error(), declaration.errorText) {
					t.Fatalf("expected %q, got %v", declaration.errorText, err)
				}
				if setupCalled || childCalled {
					t.Fatalf("admission ran too late: setup=%v child=%v", setupCalled, childCalled)
				}
			})
		}
	}
}

func writeAdmissionFixture(t *testing.T, declaration string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"dockpipe.config.json":       `{"schema":1,"compile":{"workflows":["workflows"]}}`,
		"workflows/child/config.yml": declaration + "name: child\nnamespace: admission-fixture\nsteps:\n  - kind: host\n    cmd: observe\n",
		"setup.sh":                   "#!/bin/sh\n",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeTestCoreResolver(t, root, "delegate", "DOCKPIPE_RESOLVER_WORKFLOW=child\n")
	return root
}

func TestPackagedWorkflowAdmissionAllowsOptionalToolAndLoadsOnce(t *testing.T) {
	withRunSeams(t)
	withRunStepsSeams(t)
	root := writeAdmissionFixture(t, "dependencies:\n  host:\n    - command: dockpipe-admission-absent-optional-tool\n      required: false\n")
	loads := 0
	load := loadWorkflowAppFn
	loadWorkflowAppFn = func(path string) (*domain.Workflow, error) {
		loads++
		return load(path)
	}
	setupCalled, childCalled := false, false
	sourceHostScriptFn = func(string, []string) (map[string]string, error) {
		setupCalled = true
		return map[string]string{"FROM_SETUP": "ready"}, nil
	}
	runHostCommandFn = func(_ string, environment []string) error {
		childCalled = true
		if got := domain.EnvSliceToMap(environment)["FROM_SETUP"]; got != "ready" {
			t.Fatalf("child lost setup environment: %q", got)
		}
		return nil
	}
	o := baseRunStepsOpts()
	o.repoRoot, o.projectRoot, o.wfRoot = root, root, root
	o.opts.Workdir = root
	o.envMap["DOCKPIPE_WORKDIR"] = root
	o.wf.Steps = []domain.Step{{WorkflowName: "child", Package: "admission-fixture", Run: []string{"setup.sh"}}}
	if err := runBlockingStep(&o, 0, 1, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if loads != 1 || !setupCalled || !childCalled {
		t.Fatalf("loads=%d setup=%v child=%v", loads, setupCalled, childCalled)
	}
}

func TestHostStepRejectsWorkflowDelegateBeforeDependencyAdmission(t *testing.T) {
	for _, mode := range []string{"existing", "created-by-setup"} {
		t.Run(mode, func(t *testing.T) {
			withRunSeams(t)
			withRunStepsSeams(t)
			root := writeAdmissionFixture(t, "dependencies:\n  host:\n    - command: fixture-tool\n      install:\n        windows: fixture-install\n        macos: fixture-install\n        linux: fixture-install\n")

			oldLookup := dependencyLookPathFn
			oldPowerShell := dependencyPowerShellLookupFn
			oldInstaller := dependencyRunShellFn
			t.Cleanup(func() {
				dependencyLookPathFn = oldLookup
				dependencyPowerShellLookupFn = oldPowerShell
				dependencyRunShellFn = oldInstaller
			})
			dependencyLookPathFn = func(string) (string, error) {
				return "", os.ErrNotExist
			}
			dependencyPowerShellLookupFn = dependencyLookPathFn
			installerCalls := 0
			dependencyRunShellFn = func(string) error {
				installerCalls++
				return nil
			}
			loads := 0
			load := loadWorkflowAppFn
			loadWorkflowAppFn = func(path string) (*domain.Workflow, error) {
				loads++
				return load(path)
			}

			resolver := "delegate"
			if mode == "created-by-setup" {
				resolver = "dynamic"
			}
			setupCalled := false
			runHostScriptFn = func(string, []string) error {
				setupCalled = true
				writeTestCoreResolver(t, root, resolver, "DOCKPIPE_RESOLVER_WORKFLOW=child\n")
				return nil
			}
			options := baseRunStepsOpts()
			options.repoRoot = root
			options.projectRoot = root
			options.wfRoot = root
			options.opts.Workdir = root
			options.opts.ApproveSystemChanges = true
			options.envMap["DOCKPIPE_WORKDIR"] = root
			// The host step inherits the workflow profile; an explicit step resolver
			// would already be rejected by structural workflow validation.
			options.wf.Resolver = resolver
			options.wf.Steps = []domain.Step{{Kind: "host", Run: []string{"setup.sh"}}}
			if err := domain.ValidateLoadedWorkflow(options.wf); err != nil {
				t.Fatalf("fixture must reach profile validation: %v", err)
			}

			err := runBlockingStep(&options, 0, 1, map[string]string{})
			if err == nil || !strings.Contains(err.Error(), "remove kind: host") {
				t.Errorf("expected incompatible step kind error, got %v", err)
			}
			if loads != 0 || installerCalls != 0 {
				t.Errorf("invalid host step reached dependency admission: child loads=%d installers=%d", loads, installerCalls)
			}
			if wantSetup := mode == "created-by-setup"; setupCalled != wantSetup {
				t.Errorf("setup called=%v, want %v", setupCalled, wantSetup)
			}
		})
	}
}
