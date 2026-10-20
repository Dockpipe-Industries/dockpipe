package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/vscode"
)

var importedWorkflowID = regexp.MustCompile(`^vscode-(task|launch)-[0-9a-f]{16}$`)
var importedInputEnvironment = regexp.MustCompile(`^DOCKPIPE_VSCODE_INPUT_[0-9A-F]{16}$`)

func isVSCodeWorkflow(id string) bool {
	return importedWorkflowID.MatchString(id)
}

// Editor inputs are strings, including significant surrounding whitespace.
// Preserve their CLI values without changing the older generic --env trimming
// contract. The same reserved keys remain valid in independent native clones.
func mergeVSCodeInputEnv(environment map[string]string, lines, overrides []string) {
	locked := lockedKeys(overrides)
	for _, line := range lines {
		key, value, found := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if found && importedInputEnvironment.MatchString(key) && !locked[key] {
			environment[key] = value
		}
	}
}

// Find the nearest editor workspace without walking sibling projects or reading
// user-level VS Code settings. A Dockpipe project boundary also ends the search.
func vscodeWorkspace(workdir string) (string, error) {
	absolute, err := filepath.Abs(workdir)
	if err != nil {
		return "", err
	}
	for directory := absolute; ; directory = filepath.Dir(directory) {
		for _, name := range []string{"tasks.json", "launch.json"} {
			if _, err := os.Stat(filepath.Join(directory, ".vscode", name)); err == nil {
				return directory, nil
			}
		}
		for _, boundary := range []string{domain.DockpipeProjectConfigFileName, ".git"} {
			if _, err := os.Lstat(filepath.Join(directory, boundary)); err == nil {
				return directory, nil
			}
		}
		if filepath.Dir(directory) == directory {
			return absolute, nil
		}
	}
}

func discoverVSCodeWorkflows(workdir string) (*vscode.Catalog, string, error) {
	workspace, err := vscodeWorkspace(workdir)
	if err != nil {
		return nil, "", err
	}
	catalog, err := vscode.Discover(context.Background(), workspace, vscode.Options{})
	return catalog, workspace, err
}

func catalogVSCodeWorkflows(workdir string) ([]catalogWorkflowRecord, error) {
	catalog, _, err := discoverVSCodeWorkflows(workdir)
	if err != nil {
		return nil, err
	}
	for _, diagnostic := range catalog.Diagnostics {
		fmt.Fprintf(os.Stderr, "[dockpipe] VS Code %s: %s\n", diagnostic.SourceFile, diagnostic.Message)
	}
	entries := make([]catalogWorkflowRecord, 0, len(catalog.Entries))
	for _, entry := range catalog.Entries {
		if entry.Supported {
			plan, resolveErr := catalog.Resolve(context.Background(), entry.ID, vscode.Values{Symbolic: true})
			if resolveErr == nil {
				_, resolveErr = lowerVSCodeWorkflow(plan)
			}
			if resolveErr != nil {
				entry.Supported = false
				entry.UnsupportedReason = resolveErr.Error()
			}
		}
		category := "workflow"
		description := "Run directly from VS Code tasks.json."
		var warnings []string
		if entry.Kind == "launch" {
			category = "app"
			description = "Run from VS Code launch.json without a debugger."
			warnings = []string{"Runs the application without a debugger; breakpoints and debugger extensions are not provided."}
		}
		record := catalogWorkflowRecord{
			WorkflowID:        entry.ID,
			DisplayName:       entry.Name,
			Category:          category,
			Description:       description,
			Source:            "vscode",
			SourceKind:        entry.Kind,
			SourcePath:        entry.SourceFile,
			SourceName:        entry.Name,
			Cloneable:         entry.Supported,
			Supported:         &entry.Supported,
			UnsupportedReason: entry.UnsupportedReason,
			Warnings:          warnings,
		}
		for _, input := range entry.Inputs {
			attributes := map[string]string{"displayname": input.ID}
			if input.Password {
				attributes["secret"] = "true"
				attributes["password"] = "true"
			}
			if len(input.Options) > 0 {
				options, _ := json.Marshal(input.Options)
				attributes["options"] = string(options)
			}
			defaultValue := input.Default
			if input.Password {
				defaultValue = ""
			}
			record.Inputs = append(record.Inputs, catalogWorkflowInputRecord{
				FieldName:    input.ID,
				EnvName:      vscode.InputEnvName(input.ID),
				Type:         "string",
				Description:  input.Description,
				DefaultValue: defaultValue,
				Attributes:   attributes,
			})
		}
		entries = append(entries, record)
	}
	return entries, nil
}

func loadVSCodeWorkflow(workdir, id string) (*domain.Workflow, string, error) {
	plan, workspace, err := loadVSCodePlan(workdir, id)
	if err != nil {
		return nil, "", err
	}
	workflow, err := lowerVSCodeWorkflow(plan)
	return workflow, workspace, err
}

func loadVSCodePlan(workdir, id string) (*vscode.Plan, string, error) {
	catalog, workspace, err := discoverVSCodeWorkflows(workdir)
	if err != nil {
		return nil, "", err
	}
	plan, err := catalog.Resolve(context.Background(), id, vscode.Values{Symbolic: true})
	if err != nil {
		return nil, "", err
	}
	return plan, workspace, nil
}
