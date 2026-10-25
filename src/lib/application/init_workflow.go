package application

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"

	"gopkg.in/yaml.v3"
)

const dockpipeProjectReadme = `# Dockpipe project

- **workflows/** — Default home for named workflows (**config.yml** per folder); **dockpipe init &lt;name&gt;** creates **workflows/&lt;name&gt;/** (override with **--workflows-dir** or **DOCKPIPE_WORKFLOWS_DIR**).
- **Compiled packages** — Workflows and resolvers come from declared project sources or installed packages. Runtime defaults are built into the library.
- **templates/&lt;name&gt;/** — Legacy named workflows; still resolved if **workflows/** does not define the same name.
- **Secrets (optional)** — No vault is configured by default. Add a named secret environment or vault template only when your workflow needs it; see **docs/runtime/vault.md** in the Dockpipe documentation.
- **dockpipe.config.json** (optional) — Repo-root JSON: **compile** source lists and optional **secrets** (**vault_template** preferred; **op_inject_template** is legacy). Omit to use built-in compile defaults when you add a config file later.
`

// ensureProjectScaffold creates the minimal project scaffold and root metadata files.
// Package-owned tools remain in declared sources or installed packages.
func ensureProjectScaffold(projectDir string) error {
	if err := os.MkdirAll(filepath.Join(projectDir, "workflows"), 0o755); err != nil {
		return err
	}
	readme := filepath.Join(projectDir, "README.md")
	if _, err := os.Stat(readme); os.IsNotExist(err) {
		if err := os.WriteFile(readme, []byte(dockpipeProjectReadme), 0o644); err != nil {
			return err
		}
	}
	dc := filepath.Join(projectDir, domain.DockpipeProjectConfigFileName)
	if _, err := os.Stat(dc); os.IsNotExist(err) {
		b, err := domain.DefaultDockpipeProjectConfigBytes()
		if err != nil {
			return fmt.Errorf("%s: %w", domain.DockpipeProjectConfigFileName, err)
		}
		if derived := deriveInitNamespace(projectDir); derived != "" {
			var cfg domain.DockpipeProjectConfig
			if err := json.Unmarshal(b, &cfg); err != nil {
				return fmt.Errorf("%s: parse default scaffold: %w", domain.DockpipeProjectConfigFileName, err)
			}
			cfg.Packages.Namespace = &derived
			b, err = json.MarshalIndent(cfg, "", "  ")
			if err != nil {
				return fmt.Errorf("%s: write default scaffold: %w", domain.DockpipeProjectConfigFileName, err)
			}
		}
		if err := os.WriteFile(dc, append(b, '\n'), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func deriveInitNamespace(projectDir string) string {
	name := strings.TrimSpace(filepath.Base(projectDir))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return ""
	}
	name = strings.ToLower(name)
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		keep := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if keep {
			if b.Len() == 0 && r >= '0' && r <= '9' {
				continue
			}
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if err := domain.ValidateNamespace(out); err != nil {
		return ""
	}
	return out
}

// localWorkflowSourceDir resolves named local templates for init --from.
// Existing templates/<name> directories remain supported.
func localWorkflowSourceDir(repoRoot, name string) string {
	legacy := filepath.Join(repoRoot, "templates", name)
	if st, err := os.Stat(legacy); err == nil && st.IsDir() {
		return legacy
	}
	return filepath.Join(infrastructure.WorkflowsRootDir(repoRoot), name)
}

// initScaffoldWorkflow builds the library-owned starters without asset discovery.
// Other sources remain directory copies through the normal workflow lookup.
func initScaffoldWorkflow(name, source string) *domain.Workflow {
	workflow := &domain.Workflow{
		Name:        name,
		Description: "Dockpipe workflow — edit me.",
	}
	switch source {
	case "blank":
		return workflow
	case "init":
		dockerPreflight := false
		workflow.Title = "Getting started example"
		workflow.Description = "A host workflow using the operating system's default shell. Customize its steps for your project."
		workflow.DockerPreflight = &dockerPreflight
		workflow.Steps = []domain.Step{
			{
				ID:   "hello",
				Kind: "host",
				Cmd:  `echo "Hello from Dockpipe"`,
			},
		}
		return workflow
	default:
		return nil
	}
}

func resolveInitFromSource(from string) (string, error) {
	from = strings.TrimSpace(from)
	if from == "" {
		return "", fmt.Errorf("empty --from")
	}
	if strings.Contains(from, "://") {
		return "", fmt.Errorf("--from must be a template source (blank, init, a local template name, or local directory path), not a URL")
	}
	// Explicit filesystem paths do not require package discovery.
	if filepath.IsAbs(from) || strings.ContainsAny(from, `/\`) || strings.HasPrefix(from, ".") {
		abs, err := filepath.Abs(from)
		if err != nil {
			return "", err
		}
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			return abs, nil
		}
	}
	repoRoot, err := infrastructure.RepoRoot()
	if err != nil {
		return "", err
	}
	source := localWorkflowSourceDir(repoRoot, from)
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		return source, nil
	}
	if path := infrastructure.FindNestedWorkflowDirByLeafName(repoRoot, from); path != "" {
		return path, nil
	}
	abs, err := filepath.Abs(from)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(abs); err == nil && info.IsDir() {
		return abs, nil
	}
	return "", fmt.Errorf("unknown --from source %q — use blank, init, a declared workflow name, or a local workflow directory", from)
}

func writeWorkflowYAML(path string, wf *domain.Workflow) error {
	data, err := yaml.Marshal(wf)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func patchWorkflowConfigName(cfgPath, name string) error {
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	wf, err := domain.ParseWorkflowYAML(b)
	if err != nil {
		return fmt.Errorf("parse %s: %w", cfgPath, err)
	}
	wf.Name = name
	return writeWorkflowYAML(cfgPath, wf)
}

func applyInitWorkflowFlags(cfgPath, resolver, runtime, strategy string) error {
	resolver = strings.TrimSpace(resolver)
	runtime = strings.TrimSpace(runtime)
	strategy = strings.TrimSpace(strategy)
	if resolver == "" && runtime == "" && strategy == "" {
		return nil
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	wf, err := domain.ParseWorkflowYAML(b)
	if err != nil {
		return fmt.Errorf("parse %s: %w", cfgPath, err)
	}
	if resolver != "" {
		wf.Resolver = resolver
	}
	if runtime != "" {
		wf.Runtime = runtime
	}
	if strategy != "" {
		wf.Strategy = strategy
		if wf.Strategies == nil {
			wf.Strategies = []string{strategy}
		} else if !stringSliceContains(wf.Strategies, strategy) {
			wf.Strategies = append(wf.Strategies, strategy)
		}
	}
	return writeWorkflowYAML(cfgPath, wf)
}

func stringSliceContains(ss []string, s string) bool {
	for _, x := range ss {
		if strings.TrimSpace(x) == s {
			return true
		}
	}
	return false
}

func maybeWarnUnusedWorkflowsRoot(projectDir string) {
	root := infrastructure.WorkflowsRootDir(projectDir)
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return
	}
	if infrastructure.WorkflowsDirHasDockpipeWorkflow(root) {
		return
	}
	if !infrastructure.WorkflowsDirHasFiles(root) {
		return
	}
	rel := root
	if r, err := filepath.Rel(projectDir, root); err == nil && !strings.HasPrefix(r, "..") {
		rel = r
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] warning: %q exists but has no Dockpipe workflow folders (no <name>/config.yml). "+
		"This path is often used for GitHub Actions or other tools. New workflows will still be created here. "+
		"To use a different directory set DOCKPIPE_WORKFLOWS_DIR or run: dockpipe init <name> --workflows-dir <path>\n", rel)
}

func ensureDefaultStarterWorkflow(projectDir string) error {
	wfRoot := infrastructure.WorkflowsRootDir(projectDir)
	if infrastructure.WorkflowsDirHasDockpipeWorkflow(wfRoot) {
		return nil
	}
	return createNamedWorkflow(projectDir, "example", "init", "", "", "")
}

func createNamedWorkflow(projectDir, name, fromSource, resolver, runtime, strategy string) error {
	maybeWarnUnusedWorkflowsRoot(projectDir)
	wfBase := infrastructure.WorkflowsRootDir(projectDir)
	td := filepath.Join(wfBase, name)
	if st, err := os.Stat(td); err == nil && st.IsDir() {
		return fmt.Errorf("workflow directory already exists: %s", td)
	}
	legacy := filepath.Join(projectDir, "templates", name)
	if st, err := os.Stat(legacy); err == nil && st.IsDir() {
		return fmt.Errorf("workflow %q already exists at %s (legacy templates/ layout); remove or rename before creating under workflows/", name, legacy)
	}

	workflow := initScaffoldWorkflow(name, fromSource)
	var sourceDirectory string
	if workflow == nil {
		var err error
		sourceDirectory, err = resolveInitFromSource(fromSource)
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(td, 0o755); err != nil {
		return err
	}
	cfgPath := filepath.Join(td, "config.yml")
	showRel := td
	if r, err := filepath.Rel(projectDir, td); err == nil && !strings.HasPrefix(r, "..") {
		showRel = r
	}
	return infrastructure.RunOperationWithOptions(
		os.Stderr,
		"init.workflow",
		"Creating workflow…",
		map[string]string{
			"workflow": name,
			"path":     showRel,
			"source":   fromSource,
		},
		infrastructure.OperationOptions{Spinner: false},
		func() error {
			if workflow != nil {
				if err := writeWorkflowYAML(cfgPath, workflow); err != nil {
					return err
				}
			} else {
				if err := copyDir(sourceDirectory, td); err != nil {
					return err
				}
				if err := patchWorkflowConfigName(cfgPath, name); err != nil {
					return err
				}
			}
			return applyInitWorkflowFlags(cfgPath, resolver, runtime, strategy)
		},
	)
}
