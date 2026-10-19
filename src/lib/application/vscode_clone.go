package application

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/vscode"
	"gopkg.in/yaml.v3"
)

var clonedWorkflowName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func cmdWorkflowClone(args []string) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Println("dockpipe workflow clone <workflow-id> [--workdir <path>] [--name <name>] [--format json|text]")
		return nil
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: dockpipe workflow clone <workflow-id> [--workdir <path>] [--name <name>] [--format json|text]")
	}
	id := args[0]
	workdir, err := os.Getwd()
	if err != nil {
		return err
	}
	name, format := "imported-"+strings.TrimPrefix(id, "vscode-"), "text"
	for index := 1; index < len(args); index++ {
		if args[index] == "--help" || args[index] == "-h" {
			fmt.Println("dockpipe workflow clone <workflow-id> [--workdir <path>] [--name <name>] [--format json|text]")
			return nil
		}
		if index+1 >= len(args) {
			return fmt.Errorf("%s requires a value", args[index])
		}
		switch args[index] {
		case "--workdir":
			workdir = args[index+1]
		case "--name":
			name = args[index+1]
		case "--format":
			format = args[index+1]
		default:
			return fmt.Errorf("unknown clone option %q", args[index])
		}
		index++
	}
	if format != "json" && format != "text" {
		return fmt.Errorf("unknown --format %q (use json or text)", format)
	}
	path, err := cloneVSCodeWorkflow(workdir, id, name)
	if err != nil {
		return err
	}
	if format == "json" {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"workflow_id": name, "config_path": path})
	}
	fmt.Fprintln(os.Stdout, path)
	return nil
}

func cloneVSCodeWorkflow(workdir, id, name string) (string, error) {
	if !isVSCodeWorkflow(id) {
		return "", fmt.Errorf("workflow clone currently accepts VS Code task and launch IDs from workflow list")
	}
	if !clonedWorkflowName.MatchString(name) || isVSCodeWorkflow(name) {
		return "", fmt.Errorf("clone name must be one directory name of up to 128 letters, digits, dots, underscores or hyphens")
	}
	plan, workspace, err := loadVSCodePlan(workdir, id)
	if err != nil {
		return "", err
	}
	workflow, err := lowerVSCodeWorkflow(plan)
	if err != nil {
		return "", err
	}
	workflow.Name = name
	if len(plan.Entry.Inputs) > 0 {
		workflow.Types = []string{"ImportedInputs.pipe"}
	}
	if err := domain.ValidateLoadedWorkflow(workflow); err != nil {
		return "", err
	}
	content, err := yaml.Marshal(workflow)
	if err != nil {
		return "", err
	}
	root := infrastructure.WorkflowsRootDir(workspace)
	if info, err := os.Lstat(root); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("workflow destination must not be a symbolic link")
	}
	directory := filepath.Join(root, name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	// Exclusive directory creation keeps repeat clones and concurrent requests
	// from overwriting authored work, including a directory containing assets.
	if err := os.Mkdir(directory, 0o755); err != nil {
		return "", fmt.Errorf("create clone (choose another --name if it exists): %w", err)
	}
	inputPath := filepath.Join(directory, "ImportedInputs.pipe")
	if len(plan.Entry.Inputs) > 0 {
		if err := writeClonedWorkflowFile(inputPath, []byte(clonedVSCodeInputTypes(plan.Entry.Inputs))); err != nil {
			_ = os.Remove(directory)
			return "", err
		}
	}
	path := filepath.Join(directory, "config.yml")
	if err := writeClonedWorkflowFile(path, content); err != nil {
		if len(plan.Entry.Inputs) > 0 {
			_ = os.Remove(inputPath)
		}
		_ = os.Remove(directory)
		return "", err
	}
	return path, nil
}

func writeClonedWorkflowFile(path string, content []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(content)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	return nil
}

func clonedVSCodeInputTypes(inputs []vscode.Input) string {
	var source strings.Builder
	source.WriteString("public Class ImportedInputs\n{\n")
	for index, input := range inputs {
		attributes := map[string]string{
			"DisplayName": input.ID,
			"EnvName":     vscode.InputEnvName(input.ID),
		}
		if input.Password {
			attributes["Password"] = "true"
			attributes["Secret"] = "true"
		}
		if len(input.Options) > 0 {
			options, _ := json.Marshal(input.Options)
			attributes["Options"] = string(options)
		}
		names := make([]string, 0, len(attributes))
		for name := range attributes {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			value, _ := json.Marshal(attributes[name])
			fmt.Fprintf(&source, "    [%s = %s]\n", name, value)
		}
		fmt.Fprintf(&source, "    public string Input%d;\n\n", index+1)
	}
	source.WriteString("}\n")
	return source.String()
}
