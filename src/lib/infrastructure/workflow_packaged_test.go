package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePackagedWorkflowConfigPath(t *testing.T) {
	repo := t.TempDir()
	projectCfg := `{"schema":1,"compile":{"workflows":["vendor/my-workflows"]}}`
	if err := os.WriteFile(filepath.Join(repo, "dockpipe.config.json"), []byte(projectCfg), 0o644); err != nil {
		t.Fatal(err)
	}
	wfDir := filepath.Join(repo, "vendor", "my-workflows", "mywf")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `name: mywf
namespace: dockpipe.demo
steps: []
`
	if err := os.WriteFile(filepath.Join(wfDir, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolvePackagedWorkflowConfigPath(repo, repo, "mywf", "dockpipe.demo")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(wfDir, "config.yml") {
		t.Fatalf("got %s want %s", got, filepath.Join(wfDir, "config.yml"))
	}
	_, err = ResolvePackagedWorkflowConfigPath(repo, repo, "mywf", "wrong.ns")
	if err == nil {
		t.Fatal("expected error for wrong namespace")
	}
}

func TestNormalizeRuntimeProfileName(t *testing.T) {
	if got := NormalizeRuntimeProfileName("kubepod"); got != "dockerimage" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeRuntimeProfileName("cli"); got != "dockerimage" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeRuntimeProfileName("docker"); got != "dockerimage" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeRuntimeProfileName("dockerimage"); got != "dockerimage" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeRuntimeProfileName("vmimage"); got != "vmimage" {
		t.Fatalf("got %q", got)
	}
}

func TestResolvePackagedWorkflowIgnoresUndeclaredTrees(t *testing.T) {
	repo := t.TempDir()
	for _, tree := range []string{"templates", "bundle/workflows", "src/core"} {
		directory := filepath.Join(repo, tree, "retired-workflow")
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "config.yml"), []byte("name: retired-workflow\nnamespace: example.test\nsteps: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := ResolvePackagedWorkflowConfigPath(repo, repo, "retired-workflow", "example.test"); err == nil {
		t.Fatalf("unexpected undeclared workflow %q", got)
	}
}
