package application

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/infrastructure"
)

const scopeUsageText = `dockpipe scope — resolve workflow/package scope objects and paths

Usage:
  dockpipe scope [--workdir <path>]
  dockpipe scope <scope> [path ...] [--workdir <path>]
  dockpipe scope --package <name> [path ...] [--workdir <path>]
  dockpipe scope workflow <name> [path ...] [--workdir <path>]
  dockpipe scope resolver <name> <field> [--workdir <path>]

Built-in scopes:
  source, repo, workdir
  artifacts, artifact, output

With no positional scope, prints the current workflow scope object as JSON.
Custom scope names resolve under the current workflow output root:
  <output-root>/scopes/<scope>

With --package and no path, prints the package scope object as JSON. With path
segments, prints a path under that package scope.

Workflow scope paths resolve under a named workflow artifact root. Use this when a
workflow needs to read artifacts from another workflow.

Resolver scope fields are read from the resolver profile:
  auth-dir, container-auth-dir, auth-mount-mode, config-file, container-config-file

Examples:
  dockpipe scope
  dockpipe scope source src
  dockpipe scope artifacts providers/codex/result.json
  dockpipe scope name.123213
  dockpipe scope workflow docs.orchestrate orchestrate
  dockpipe scope --package dorkpipe
  dockpipe scope --package dorkpipe training metrics.jsonl
  dockpipe scope resolver codex auth-dir
`

func cmdScope(args []string) error {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Print(scopeUsageText)
		return nil
	}
	parsed, err := parseScopeArgs(args)
	if err != nil {
		return err
	}
	if parsed.packageName != "" {
		obj, err := packageScopeObject(parsed.workdir, parsed.packageName)
		if err != nil {
			return err
		}
		if len(parsed.suffix) == 0 {
			return printScopeObject(obj)
		}
		if len(parsed.suffix) == 1 && parsed.suffix[0] == "." {
			fmt.Println(obj.Root)
			return nil
		}
		joined, err := infrastructure.JoinStatePath(obj.Root, filepath.ToSlash(filepath.Join(parsed.suffix...)))
		if err != nil {
			return err
		}
		fmt.Println(joined)
		return nil
	}
	if parsed.scope == "" {
		obj, err := workflowScopeObject(parsed.workdir)
		if err != nil {
			return err
		}
		return printScopeObject(obj)
	}
	if normalizeGetField(parsed.scope) == "resolver" {
		value, err := resolverScopeValue(parsed.workdir, parsed.suffix)
		if err != nil {
			return err
		}
		fmt.Println(value)
		return nil
	}
	if normalizeGetField(parsed.scope) == "workflow" {
		value, err := workflowScopePath(parsed.workdir, parsed.suffix)
		if err != nil {
			return err
		}
		fmt.Println(value)
		return nil
	}
	root, err := resolveScopeRoot(parsed.scope, parsed.workdir)
	if err != nil {
		return err
	}
	parts := append([]string{root}, parsed.suffix...)
	fmt.Println(filepath.Clean(filepath.Join(parts...)))
	return nil
}

type scopeArgs struct {
	scope       string
	packageName string
	workdir     string
	suffix      []string
}

type scopeObject struct {
	Kind         string `json:"kind"`
	Name         string `json:"name,omitempty"`
	Scope        string `json:"scope"`
	Root         string `json:"root"`
	DockpipeBin  string `json:"dockpipe_bin,omitempty"`
	SourceRoot   string `json:"source_root,omitempty"`
	ArtifactRoot string `json:"artifact_root,omitempty"`
	EventLog     string `json:"event_log,omitempty"`
	EventIndex   string `json:"event_index,omitempty"`
	OutputRoot   string `json:"output_root,omitempty"`
	StateRoot    string `json:"state_root,omitempty"`
	Workdir      string `json:"workdir"`
}

func printScopeObject(obj scopeObject) error {
	b, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func parseScopeArgs(args []string) (scopeArgs, error) {
	parsed := scopeArgs{}
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--workdir":
			if i+1 >= len(args) {
				return parsed, fmt.Errorf("--workdir requires a path")
			}
			parsed.workdir = strings.TrimSpace(args[i+1])
			i++
		case "--package":
			if i+1 >= len(args) {
				return parsed, fmt.Errorf("--package requires a name")
			}
			parsed.packageName = strings.TrimSpace(args[i+1])
			i++
		case "-h", "--help":
			return parsed, fmt.Errorf("usage: dockpipe scope [<scope> [path ...] | --package <name> [path ...]] [--workdir <path>]")
		default:
			positionals = append(positionals, args[i])
		}
	}
	if parsed.packageName == "" && len(positionals) > 0 {
		parsed.scope = strings.TrimSpace(positionals[0])
		parsed.suffix = positionals[1:]
	} else {
		parsed.suffix = positionals
	}
	workdir, err := resolveSDKWorkdir(parsed.workdir)
	if err != nil {
		return parsed, err
	}
	parsed.workdir = workdir
	return parsed, nil
}

func workflowScopeObject(workdir string) (scopeObject, error) {
	outputRoot, err := resolveOutputRoot(workdir)
	if err != nil {
		return scopeObject{}, err
	}
	stateRoot, err := infrastructure.StateRoot(workdir)
	if err != nil {
		return scopeObject{}, err
	}
	artifactRoot := strings.TrimSpace(os.Getenv("DOCKPIPE_ARTIFACT_ROOT"))
	if artifactRoot == "" {
		artifactRoot, err = workflowArtifactRoot(workdir, strings.TrimSpace(os.Getenv("DOCKPIPE_WORKFLOW_NAME")))
		if err != nil {
			return scopeObject{}, err
		}
	}
	eventLog, err := resolveEventLogPath(workdir)
	if err != nil {
		return scopeObject{}, err
	}
	eventIndex, err := resolveEventIndexPath(workdir)
	if err != nil {
		return scopeObject{}, err
	}
	sourceRoot := firstNonEmpty(strings.TrimSpace(os.Getenv("DOCKPIPE_SOURCE_ROOT")), workdir)
	name := strings.TrimSpace(os.Getenv("DOCKPIPE_WORKFLOW_NAME"))
	dockpipeBin, err := resolveDockpipeBinForSDK(workdir)
	if err != nil {
		return scopeObject{}, err
	}
	return scopeObject{
		Kind:         "workflow",
		Name:         name,
		Scope:        sanitizeWorkflowStateScope(name),
		Root:         outputRoot,
		DockpipeBin:  dockpipeBin,
		SourceRoot:   sourceRoot,
		ArtifactRoot: artifactRoot,
		EventLog:     eventLog,
		EventIndex:   eventIndex,
		OutputRoot:   outputRoot,
		StateRoot:    filepath.Join(stateRoot, "workflows", sanitizeWorkflowStateScope(name)),
		Workdir:      workdir,
	}, nil
}

func packageScopeObject(workdir, packageName string) (scopeObject, error) {
	scope := infrastructure.SanitizePackageStateScope(packageName)
	status, err := infrastructure.PreparePackageStateDir(workdir, strings.TrimSpace(packageName))
	if err != nil {
		return scopeObject{}, err
	}
	if status.LegacyDiverged {
		fmt.Fprintf(os.Stderr, "[dockpipe] package state: durable state is authoritative; legacy %q has diverged and was not merged\n", packageName)
	}
	dockpipeBin, err := resolveDockpipeBinForSDK(workdir)
	if err != nil {
		return scopeObject{}, err
	}
	return scopeObject{
		Kind:        "package",
		Name:        packageName,
		Scope:       scope,
		Root:        status.Dir,
		DockpipeBin: dockpipeBin,
		StateRoot:   status.Dir,
		Workdir:     workdir,
	}, nil
}

func resolveScopeRoot(scope, workdir string) (string, error) {
	normalized := normalizeGetField(scope)
	switch normalized {
	case "source", "repo", "workdir":
		return firstNonEmpty(strings.TrimSpace(os.Getenv("DOCKPIPE_SOURCE_ROOT")), workdir), nil
	case "artifacts", "artifact", "output":
		return resolveOutputRoot(workdir)
	default:
		outputRoot, err := resolveOutputRoot(workdir)
		if err != nil {
			return "", err
		}
		return filepath.Join(outputRoot, "scopes", sanitizeNamedScope(scope)), nil
	}
}

func resolverScopeValue(workdir string, args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: dockpipe scope resolver <name> <auth-dir|container-auth-dir|auth-mount-mode|config-file|container-config-file>")
	}
	resolverName := strings.TrimSpace(args[0])
	field := normalizeGetField(args[1])
	profilePath, err := infrastructure.ResolveResolverFilePath(workdir, resolverName)
	if err != nil {
		return "", err
	}
	profile, err := infrastructure.LoadResolverFile(profilePath)
	if err != nil {
		return "", err
	}
	switch field {
	case "auth_dir":
		return resolveResolverHostPath(profile, "DOCKPIPE_RESOLVER_AUTH_DIR_ENV", "DOCKPIPE_RESOLVER_AUTH_DIR")
	case "container_auth_dir":
		return strings.TrimSpace(profile["DOCKPIPE_RESOLVER_CONTAINER_AUTH_DIR"]), nil
	case "auth_mount_mode":
		return firstNonEmpty(strings.TrimSpace(profile["DOCKPIPE_RESOLVER_AUTH_MOUNT_MODE"]), "rw"), nil
	case "config_file":
		return resolveResolverHostPath(profile, "DOCKPIPE_RESOLVER_CONFIG_FILE_ENV", "DOCKPIPE_RESOLVER_CONFIG_FILE")
	case "container_config_file":
		return strings.TrimSpace(profile["DOCKPIPE_RESOLVER_CONTAINER_CONFIG_FILE"]), nil
	default:
		return "", fmt.Errorf("unknown resolver scope field %q", args[1])
	}
}

func workflowScopePath(workdir string, args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: dockpipe scope workflow <name> [path ...]")
	}
	workflowName := strings.TrimSpace(args[0])
	root, err := workflowArtifactRoot(workdir, workflowName)
	if err != nil {
		return "", err
	}
	parts := append([]string{root}, args[1:]...)
	return filepath.Clean(filepath.Join(parts...)), nil
}

func resolveResolverHostPath(profile map[string]string, envKey, defaultKey string) (string, error) {
	if envName := strings.TrimSpace(profile[envKey]); envName != "" {
		if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
			return value, nil
		}
	}
	value := strings.TrimSpace(profile[defaultKey])
	if value == "" {
		return "", nil
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value), nil
	}
	home := strings.TrimSpace(os.Getenv("HOME"))
	if home == "" {
		if userHome, err := os.UserHomeDir(); err == nil {
			home = strings.TrimSpace(userHome)
		}
	}
	if home == "" {
		return "", fmt.Errorf("user home directory is not set")
	}
	return filepath.Join(home, filepath.Clean(value)), nil
}

func resolveOutputRoot(workdir string) (string, error) {
	v := strings.TrimSpace(os.Getenv("DOCKPIPE_OUTPUT_ROOT"))
	if v == "" {
		v = strings.TrimSpace(os.Getenv("DOCKPIPE_ARTIFACT_ROOT"))
	}
	if v != "" {
		return v, nil
	}
	return workflowArtifactRoot(workdir, strings.TrimSpace(os.Getenv("DOCKPIPE_WORKFLOW_NAME")))
}

func sanitizeNamedScope(scope string) string {
	scope = strings.TrimSpace(scope)
	var b strings.Builder
	lastDash := false
	for _, r := range scope {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "default"
	}
	return out
}
