package application

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = old })
	fn()
	_ = w.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	os.Stderr = old
	return string(b)
}

// TestCmdInitFromBlankScaffold produces a minimal workflow YAML when --from blank.
func TestCmdInitFromBlankScaffold(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdInit([]string{"blankflow", "--from", "blank"}); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(project, "workflows", "blankflow", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "blankflow") {
		t.Fatalf("expected workflow name in config, got:\n%s", string(b))
	}
}

// TestCmdInitBareCreatesStarterWorkflow ensures bare init seeds workflows/example/ for first-run discoverability.
func TestCmdInitBareCreatesStarterWorkflow(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdInit([]string{}); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(project, "workflows", "example", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := domain.ParseWorkflowYAML(b)
	if err != nil {
		t.Fatal(err)
	}
	if workflow.Name != "example" || workflow.DockerPreflight == nil || *workflow.DockerPreflight {
		t.Fatalf("expected a host-only starter with Docker preflight disabled: %+v", workflow)
	}
	if len(workflow.Steps) != 1 || !workflow.Steps[0].IsHostStep() || workflow.Steps[0].Cmd == "" {
		t.Fatalf("expected one runnable host step: %+v", workflow.Steps)
	}
	if len(workflow.Types) != 0 || len(workflow.Dependencies.Host) != 0 {
		t.Fatalf("starter must not depend on copied models or external tools: %+v", workflow)
	}
	if _, err := os.Stat(filepath.Join(project, "workflows", "example", "models")); !os.IsNotExist(err) {
		t.Fatalf("basic init must not copy optional example models: %v", err)
	}
}

// TestCmdInitFromRunTemplate copies an existing bundled workflow template by name.
func TestCmdInitFromRunTemplate(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdInit([]string{"myrun", "--from", "run"}); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(project, "workflows", "myrun", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "myrun") {
		t.Fatalf("expected patched name myrun in config, got:\n%s", s)
	}
}

func TestCmdInitFlagsPreferRuntimeAndResolverWithoutDefaultFields(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdInit([]string{"simple", "--resolver", "codex", "--runtime", "dockerimage"}); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(project, "workflows", "simple", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "resolver: codex") {
		t.Fatalf("expected resolver field, got:\n%s", s)
	}
	if !strings.Contains(s, "runtime: dockerimage") {
		t.Fatalf("expected runtime field, got:\n%s", s)
	}
	if strings.Contains(s, "default_resolver:") {
		t.Fatalf("did not expect default_resolver in new init output, got:\n%s", s)
	}
	if strings.Contains(s, "default_runtime:") {
		t.Fatalf("did not expect default_runtime in new init output, got:\n%s", s)
	}
}

// TestCmdInitDoesNotCreateGitDir ensures init never bootstraps a git repository in the project tree.
func TestCmdInitDoesNotCreateGitDir(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdInit([]string{}); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project, ".git")); err == nil {
		t.Fatal("init must not create .git (no clone/bootstrap)")
	}
	if _, err := os.Stat(filepath.Join(project, ".env.vault.template.example")); !os.IsNotExist(err) {
		t.Fatalf("init must not create vault scaffolding without opt-in: %v", err)
	}
}

func TestCmdInitSeedsDerivedPackagesNamespace(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	parent := t.TempDir()
	project := filepath.Join(parent, "uh-workflows")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdInit([]string{}); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(project, domain.DockpipeProjectConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	var cfg domain.DockpipeProjectConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatalf("parse project config: %v\n%s", err, string(b))
	}
	if cfg.Packages.Namespace == nil || *cfg.Packages.Namespace != "uh-workflows" {
		t.Fatalf("expected derived packages.namespace uh-workflows, got %+v", cfg.Packages.Namespace)
	}
}

func TestInitScaffoldDoesNotRequestVault(t *testing.T) {
	project := t.TempDir()
	if err := ensureProjectScaffold(project); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_VAULT_INJECT", "1")
	oldLook := opLookPathFn
	t.Cleanup(func() { opLookPathFn = oldLook })
	opLookPathFn = func(string) (string, error) {
		t.Fatal("a fresh project must not look for a vault provider")
		return "", nil
	}
	output := captureStderr(t, func() {
		workflow := &domain.Workflow{Name: "hello"}
		if err := mergeOpInjectFromProjectIfEnabled(map[string]string{}, &CliOpts{Workdir: project}, "", project, workflow); err != nil {
			t.Fatalf("fresh project secret setup: %v", err)
		}
	})
	if output != "" {
		t.Fatalf("fresh project must not warn about vault setup: %s", output)
	}
}

func TestInitScaffoldPreservesExplicitVaultConfiguration(t *testing.T) {
	project := t.TempDir()
	files := map[string]string{
		domain.DockpipeProjectConfigFileName: `{"schema":1,"secrets":{"vault_template":".env.vault.template","vault":"op"}}`,
		".env.vault.template":                "TOKEN=op://example/item/token\n",
		".env.vault.template.example":        "# My existing example\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(project, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := ensureProjectScaffold(project); err != nil {
		t.Fatal(err)
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(project, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Fatalf("init changed existing %s", name)
		}
	}
}

// TestCmdInitDoesNotCopyLegacyTemplateTree verifies dockpipe init no longer copies templates/core.
func TestCmdInitDoesNotCopyLegacyTemplateTree(t *testing.T) {
	repoRoot := testRepoRoot(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdInit([]string{}); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}
	for _, p := range []string{
		filepath.Join(project, "templates"),
		filepath.Join(project, "templates", "core"),
		filepath.Join(project, "scripts"),
		filepath.Join(project, "images"),
	} {
		if _, err := os.Stat(p); err == nil {
			t.Fatalf("did not expect legacy scaffold path %q", p)
		}
	}
}

func TestCmdInitBareDoesNotWarnForEmptyWorkflowsSubdirs(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, "workflows", "github", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	stderr := captureStderr(t, func() {
		if err := cmdInit([]string{}); err != nil {
			t.Fatalf("cmdInit: %v", err)
		}
	})
	for _, want := range []string{"unit=init.project", "unit=init.workflow", "status=done", "workflow=example"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected init stderr to contain %q, got:\n%s", want, stderr)
		}
	}
	if strings.Contains(stderr, "has no Dockpipe workflow folders") {
		t.Fatalf("did not expect workflows warning for empty directory tree, got:\n%s", stderr)
	}
}

func TestCmdInitBareWarnsForNonDockpipeWorkflowFiles(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "workflows", "ci", "deploy.yml"), "name: deploy\n", 0o644)
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	stderr := captureStderr(t, func() {
		if err := cmdInit([]string{}); err != nil {
			t.Fatalf("cmdInit: %v", err)
		}
	})
	if !strings.Contains(stderr, "has no Dockpipe workflow folders") {
		t.Fatalf("expected workflows warning for non-Dockpipe workflow files, got:\n%s", stderr)
	}
}

func TestInitBuiltinScaffoldsDoNotMaterializeBundledAssets(t *testing.T) {
	for _, test := range []struct {
		name       string
		args       []string
		workflow   string
		standalone bool
	}{
		{name: "project", workflow: "example"},
		{name: "named blank", args: []string{"blank"}, workflow: "blank"},
		{name: "explicit blank", args: []string{"blank", "--from", "blank"}, workflow: "blank"},
		{name: "explicit starter", args: []string{"starter", "--from", "init"}, workflow: "starter"},
		{name: "standalone starter", args: []string{"init", "standalone"}, workflow: "standalone", standalone: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			project := t.TempDir()
			cacheBlocker := filepath.Join(t.TempDir(), "not-a-directory")
			writeFile(t, cacheBlocker, "blocked bundle cache", 0o644)
			t.Setenv("DOCKPIPE_REPO_ROOT", "")
			t.Setenv("DOCKPIPE_BUNDLED_CACHE", cacheBlocker)
			oldWorkingDirectory, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir(project); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chdir(oldWorkingDirectory) })
			command := "init"
			config := filepath.Join(project, "workflows", test.workflow, "config.yml")
			if test.standalone {
				command = "template"
				config = filepath.Join(project, test.workflow, "config.yml")
			}
			if err := Run(append([]string{command}, test.args...), nil); err != nil {
				t.Fatalf("asset-free CLI init: %v", err)
			}
			if err := infrastructure.ValidateResolvedWorkflowYAML(config); err != nil {
				t.Fatalf("generated workflow failed schema validation: %v", err)
			}
			cacheContents, err := os.ReadFile(cacheBlocker)
			if err != nil || string(cacheContents) != "blocked bundle cache" {
				t.Fatalf("asset-free scaffold changed the blocked cache: %v, %q", err, cacheContents)
			}
		})
	}
}

func TestCmdInitPreservesExistingStarterOnRepeat(t *testing.T) {
	project := t.TempDir()
	oldWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWorkingDirectory) })
	if err := cmdInit(nil); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(project, "workflows", "example", "config.yml")
	const edited = "name: example\ndescription: User-owned edit\n"
	writeFile(t, config, edited, 0o644)
	if err := cmdInit(nil); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(config)
	if err != nil || string(actual) != edited {
		t.Fatalf("repeat init overwrote starter: %v, %q", err, actual)
	}
	if err := cmdInit([]string{"example"}); err == nil {
		t.Fatal("named init must reject an existing workflow directory")
	}
}

func TestCmdInitCopiesLocalSourceWithoutBundledAssets(t *testing.T) {
	project := t.TempDir()
	source := t.TempDir()
	writeFile(t, filepath.Join(source, "config.yml"), "name: source\ndescription: User template\n", 0o644)
	writeFile(t, filepath.Join(source, "assets", "preserved.txt"), "user-owned source bytes\n", 0o644)
	cacheBlocker := filepath.Join(t.TempDir(), "not-a-directory")
	writeFile(t, cacheBlocker, "blocked bundle cache", 0o644)
	t.Setenv("DOCKPIPE_REPO_ROOT", "")
	t.Setenv("DOCKPIPE_BUNDLED_CACHE", cacheBlocker)
	oldWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWorkingDirectory) })
	if err := cmdInit([]string{"copied", "--from", source, "--runtime", "local", "--resolver", "tool", "--strategy", "wrap"}); err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(filepath.Join(project, "workflows", "copied", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := domain.ParseWorkflowYAML(config)
	if err != nil {
		t.Fatal(err)
	}
	if workflow.Name != "copied" || workflow.Runtime != "local" || workflow.Resolver != "tool" || workflow.Strategy != "wrap" {
		t.Fatalf("local copy did not preserve naming/flag behavior: %+v", workflow)
	}
	actual, err := os.ReadFile(filepath.Join(project, "workflows", "copied", "assets", "preserved.txt"))
	if err != nil || string(actual) != "user-owned source bytes\n" {
		t.Fatalf("local source content changed: %v, %q", err, actual)
	}
}

func TestInitScaffoldReportsWorkflowsDirectoryFailure(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "workflows"), "existing ordinary file", 0o644)
	if err := ensureProjectScaffold(project); err == nil {
		t.Fatal("creating workflows over an existing file must report failure")
	}
	if _, err := os.Stat(filepath.Join(project, domain.DockpipeProjectConfigFileName)); !os.IsNotExist(err) {
		t.Fatalf("failed scaffold must not continue writing metadata: %v", err)
	}
}
