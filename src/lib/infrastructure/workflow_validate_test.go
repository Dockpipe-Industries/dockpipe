package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestResolveWorkflowYAMLPath_relativeFromSubdir(t *testing.T) {
	root := t.TempDir()
	wfDir := filepath.Join(root, "workflows", "demo")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(wfDir, "config.yml")
	if err := os.WriteFile(cfg, []byte("name: demo\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "nested", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_REPO_ROOT", root)
	t.Chdir(sub)

	got, err := ResolveWorkflowYAMLPath("workflows/demo/config.yml")
	if err != nil {
		t.Fatalf("ResolveWorkflowYAMLPath: %v", err)
	}
	want, err := filepath.Abs(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveWorkflowYAMLPath_defaultSingleWorkflow(t *testing.T) {
	root := t.TempDir()
	wfDir := filepath.Join(root, "workflows", "solo")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(wfDir, "config.yml")
	if err := os.WriteFile(cfg, []byte("name: solo\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_REPO_ROOT", root)
	t.Chdir(root)

	got, err := ResolveWorkflowYAMLPath("")
	if err != nil {
		t.Fatalf("ResolveWorkflowYAMLPath: %v", err)
	}
	want, err := filepath.Abs(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestValidateResolvedWorkflowYAML_AcceptsAsyncGroup(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "config.yml")
	yml := `name: async-demo
steps:
  - group:
      mode: async
      tasks:
        - id: a
          cmd: echo a
          outputs: a.env
        - id: b
          cmd: echo b
          outputs: b.env
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateResolvedWorkflowYAML(cfg); err != nil {
		t.Fatalf("expected async group to validate, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_AcceptsWorkflowView(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "config.yml")
	yml := `name: view-demo
types:
  - ./models/ViewDemo.pipe
view:
  entry:
    type: choice
    field: General.Mode
    title: Demo Mode
    options:
      - value: existing
        label: Use Existing
        pages: [existing]
      - value: create
        label: Create New
        next: create
  pages:
    - id: existing
      title: Existing
      sections:
        - id: image
          title: Image
          fields:
            - Storage.Disk
    - id: create
      title: Create
      sections:
        - id: create-image
          title: New Image
          fields:
            - Storage.Disk
            - Storage.DiskSize
steps: []
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateResolvedWorkflowYAML(cfg); err != nil {
		t.Fatalf("expected view contract to validate, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_AcceptsVMInputsShape(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "config.yml")
	yml := `name: vm-demo
runtime: vm
resolver: qemu
inputs:
  General.ExecMode: powershell
  Advanced.KeepAlive: true
  Advanced.SyncHostPath:
    from: DOCKPIPE_UH_HOST_CONTEXT
steps:
  - id: guest-session
    vm:
      mounts:
        - host: C:\src\repo
          guest: C:\uh
        - host: C:\tmp\artifacts
          guest: C:\artifacts
      guest_path: C:\uh
      interactive_debug: true
      keepalive: true
      keepalive_seconds: "28800"
    cmd: hostname
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateResolvedWorkflowYAML(cfg); err != nil {
		t.Fatalf("expected vm + inputs shape to validate, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_RejectsConflictingVMInteractiveModes(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "config.yml")
	yml := `name: bad-vm
steps:
  - runtime: vm
    resolver: qemu
    vm:
      interactive_debug: true
      interactive_ssh: true
    cmd: hostname
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateResolvedWorkflowYAML(cfg)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected mutually exclusive vm interactive mode error, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_RejectsUnknownStepKey(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "config.yml")
	yml := `name: bad-step
steps:
  - cmd: echo hi
    runtimee: dockerimage
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateResolvedWorkflowYAML(cfg)
	if err == nil || !strings.Contains(err.Error(), "additionalProperties") {
		t.Fatalf("expected schema additionalProperties error, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_RejectsUnknownViewKey(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "config.yml")
	yml := `name: bad-view
view:
  pages:
    - id: source
      title: Source
      weird: nope
steps: []
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateResolvedWorkflowYAML(cfg)
	if err == nil || !strings.Contains(err.Error(), "additionalProperties") {
		t.Fatalf("expected schema additionalProperties error for bad view key, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_RejectsHostBuiltinWithoutHostKind(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "config.yml")
	yml := `name: bad-host-builtin
compose:
  file: assets/compose/docker-compose.yml
steps:
  - host_builtin: compose_up
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateResolvedWorkflowYAML(cfg)
	if err == nil || !strings.Contains(err.Error(), "requires kind: host") {
		t.Fatalf("expected host_builtin kind error, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_RejectsWorkflowWithoutPackage(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "config.yml")
	yml := `name: bad-nesting
steps:
  - workflow: child-name
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateResolvedWorkflowYAML(cfg)
	if err == nil || !strings.Contains(err.Error(), "requires package: <namespace>") {
		t.Fatalf("expected packaged workflow package error, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_RejectsTopLevelSingleFlowFieldsWithSteps(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "config.yml")
	yml := `name: mixed-shape
run: scripts/setup.sh
steps:
  - cmd: echo hi
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateResolvedWorkflowYAML(cfg)
	if err == nil || !strings.Contains(err.Error(), "top-level run") {
		t.Fatalf("expected top-level run with steps error, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_RejectsPlainIsBlockingFalse(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "config.yml")
	yml := `name: mixed-async
steps:
  - id: a
    cmd: echo hi
    is_blocking: false
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateResolvedWorkflowYAML(cfg)
	if err == nil || !strings.Contains(err.Error(), "is_blocking: false is no longer supported on plain steps") {
		t.Fatalf("expected plain is_blocking false error, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_RejectsHostStepSecurity(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "config.yml")
	yml := `name: bad-host-security
steps:
  - kind: host
    cmd: echo hi
    security:
      profile: secure-default
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateResolvedWorkflowYAML(cfg)
	if err == nil || !strings.Contains(err.Error(), "does not use security") {
		t.Fatalf("expected host security rejection, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_RejectsResolverOwnedScriptWithoutExplicitDependency(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DOCKPIPE_REPO_ROOT", root)
	if err := os.WriteFile(filepath.Join(root, domain.DockpipeProjectConfigFileName), []byte(`{
  "schema": 1,
  "compile": {
    "workflows": ["workflows", "packages"]
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	resolverDir := filepath.Join(root, "packages", "dorkpipe", "resolvers", "dorkpipe", "profile")
	if err := os.MkdirAll(resolverDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgDir := filepath.Join(root, "workflows", "demo")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(cfgDir, "config.yml")
	yml := `name: demo
steps:
  - kind: host
    run: scripts/dorkpipe/dev-stack.sh
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateResolvedWorkflowYAML(cfg)
	if err == nil || !strings.Contains(err.Error(), "require an explicit resolver dependency") {
		t.Fatalf("expected explicit resolver dependency error, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_AcceptsResolverOwnedScriptWithRequiresResolvers(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DOCKPIPE_REPO_ROOT", root)
	if err := os.WriteFile(filepath.Join(root, domain.DockpipeProjectConfigFileName), []byte(`{
  "schema": 1,
  "compile": {
    "workflows": ["workflows", "packages"]
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	resolverDir := filepath.Join(root, "packages", "dorkpipe", "resolvers", "dorkpipe", "profile")
	if err := os.MkdirAll(resolverDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgDir := filepath.Join(root, "workflows", "demo")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "package.yml"), []byte(`schema: 1
name: demo
version: 0.1.0
kind: workflow
requires_resolvers: [dorkpipe]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(cfgDir, "config.yml")
	yml := `name: demo
steps:
  - kind: host
    run: scripts/dorkpipe/dev-stack.sh
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateResolvedWorkflowYAML(cfg); err != nil {
		t.Fatalf("expected requires_resolvers to satisfy logical resolver script dependency, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_AcceptsProjectScriptOverrideWithoutResolverDependency(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DOCKPIPE_REPO_ROOT", root)
	if err := os.WriteFile(filepath.Join(root, domain.DockpipeProjectConfigFileName), []byte(`{
  "schema": 1,
  "compile": {
    "workflows": ["workflows", "packages"]
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	resolverDir := filepath.Join(root, "packages", "dorkpipe", "resolvers", "dorkpipe", "profile")
	if err := os.MkdirAll(resolverDir, 0o755); err != nil {
		t.Fatal(err)
	}
	overridePath := filepath.Join(root, "scripts", "dorkpipe")
	if err := os.MkdirAll(overridePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(overridePath, "dev-stack.sh"), []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgDir := filepath.Join(root, "workflows", "demo")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(cfgDir, "config.yml")
	yml := `name: demo
steps:
  - kind: host
    run: scripts/dorkpipe/dev-stack.sh
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateResolvedWorkflowYAML(cfg); err != nil {
		t.Fatalf("expected project script override to bypass resolver dependency check, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_DeclaredWorkflowResolverClosure(t *testing.T) {
	cases := []struct {
		name           string
		parentDepends  string
		parentRequires string
		aliasDepends   string
		aliasRequires  string
		wantError      string
	}{
		{name: "transitive resolver", parentDepends: "alias", aliasRequires: "tool"},
		{name: "direct resolver before workflow dependency exists", parentDepends: "missing", parentRequires: "tool"},
		{name: "undeclared resolver", parentDepends: "alias", wantError: "require an explicit resolver dependency"},
		{name: "unrelated installed resolver", parentDepends: "other", aliasRequires: "tool", wantError: "require an explicit resolver dependency"},
		{name: "missing workflow dependency", parentDepends: "missing", wantError: "workflow dependency"},
		{name: "workflow dependency cycle", parentDepends: "alias", aliasDepends: "consumer", aliasRequires: "tool", wantError: "workflow dependency cycle"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("DOCKPIPE_REPO_ROOT", root)
			config := `{"schema":1,"compile":{"workflows":["workflows","packages"]}}`
			if err := os.WriteFile(filepath.Join(root, domain.DockpipeProjectConfigFileName), []byte(config), 0o644); err != nil {
				t.Fatal(err)
			}
			resolverProfile := filepath.Join(root, "packages", "tool", "resolvers", "tool", "profile")
			if err := os.MkdirAll(resolverProfile, 0o755); err != nil {
				t.Fatal(err)
			}
			writeWorkflow := func(name, depends, requires, run string) string {
				t.Helper()
				dir := filepath.Join(root, "workflows", name)
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				manifest := "schema: 1\nname: " + name + "\nversion: 0.1.0\nkind: workflow\n"
				if depends != "" {
					manifest += "depends: [" + depends + "]\n"
				}
				if requires != "" {
					manifest += "requires_resolvers: [" + requires + "]\n"
				}
				if err := os.WriteFile(filepath.Join(dir, "package.yml"), []byte(manifest), 0o644); err != nil {
					t.Fatal(err)
				}
				workflow := "name: " + name + "\nsteps:\n  - kind: host\n    run: " + run + "\n"
				path := filepath.Join(dir, "config.yml")
				if err := os.WriteFile(path, []byte(workflow), 0o644); err != nil {
					t.Fatal(err)
				}
				return path
			}
			writeWorkflow("alias", tc.aliasDepends, tc.aliasRequires, "scripts/core/noop.sh")
			writeWorkflow("other", "", "other", "scripts/core/noop.sh")
			path := writeWorkflow("consumer", tc.parentDepends, tc.parentRequires, "scripts/tool/run.sh")
			err := ValidateResolvedWorkflowYAML(path)
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("declared transitive resolver rejected: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("expected %q, got %v", tc.wantError, err)
			}
		})
	}
}
