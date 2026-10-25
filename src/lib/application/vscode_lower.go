package application

import (
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/vscode"
)

var importedEnvironmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var importedEnvironmentReference = regexp.MustCompile(`\$\{env:([A-Za-z_][A-Za-z0-9_]*)\}`)

// Lower into the existing host-step contract. No alternate executor or authored
// YAML fields are introduced, and values from the process environment are never
// serialized into a clone. Shell expansion happens only when the user runs it.
func lowerVSCodeWorkflow(plan *vscode.Plan) (*domain.Workflow, error) {
	workflow := &domain.Workflow{
		Name:        plan.Entry.ID,
		Title:       plan.Entry.Name,
		Description: "Imported from VS Code " + filepath.Base(plan.Entry.SourceFile) + ": " + plan.Entry.Name,
		Vars:        make(map[string]string),
		Platforms:   []string{vscodeWorkflowPlatform(runtime.GOOS)},
	}
	if plan.Entry.Kind == "launch" {
		workflow.Category = "app"
		workflow.Description += ". Runs without a debugger."
	}
	for _, input := range plan.Entry.Inputs {
		if !input.Password && input.Default != "" {
			workflow.Vars[vscode.InputEnvName(input.ID)] = input.Default
		}
	}
	for index, command := range plan.Commands {
		body, err := lowerVSCodeCommand(command)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", command.Name, err)
		}
		if index == 0 {
			body = importedInputChecks(plan.Entry.Inputs) + body
		}
		workflow.Steps = append(workflow.Steps, domain.Step{
			ID:   fmt.Sprintf("imported-%d", index+1),
			Kind: "host",
			Cmd:  body,
		})
	}
	if len(workflow.Steps) == 0 {
		return nil, fmt.Errorf("VS Code entry %q has no runnable commands", plan.Entry.Name)
	}
	return workflow, nil
}

// VS Code resolves host-specific commands before lowering. Keep that OS scope,
// but use the workflow schema's macOS spelling rather than Go's darwin value.
func vscodeWorkflowPlatform(goos string) string {
	if goos == "darwin" {
		return "macos"
	}
	return goos
}

func importedInputChecks(inputs []vscode.Input) string {
	var body strings.Builder
	for _, input := range inputs {
		name := vscode.InputEnvName(input.ID)
		body.WriteString(": \"${" + name + "?Provide " + name + " using --env}\"\n")
		if len(input.Options) == 0 {
			continue
		}
		body.WriteString("case \"$" + name + "\" in\n")
		for index, option := range input.Options {
			if index > 0 {
				body.WriteByte('|')
			}
			body.WriteString(quoteBashLiteral(option))
		}
		body.WriteString(") ;;\n*) printf '%s\\n' " + quoteBashLiteral("Invalid choice for VS Code input "+input.ID) + " >&2; exit 2 ;;\nesac\n")
	}
	return body.String()
}

func lowerVSCodeCommand(command vscode.Command) (string, error) {
	var body strings.Builder
	// VS Code substitutes ${env:...} from the parent environment, not from
	// options.env. Snapshot references before exporting task-specific values.
	values := append([]string{command.CWD, command.Executable, command.ShellExecutable}, command.Args...)
	values = append(values, command.ShellArgs...)
	for _, value := range command.Env {
		values = append(values, value)
	}
	references := make(map[string]bool)
	for _, value := range values {
		if strings.Contains(value, "${userHome}") {
			home := "HOME"
			if runtime.GOOS == "windows" {
				home = "USERPROFILE"
			}
			references[home] = true
		}
		for _, match := range importedEnvironmentReference.FindAllStringSubmatch(value, -1) {
			references[match[1]] = true
		}
	}
	names := make([]string, 0, len(references))
	for name := range references {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		body.WriteString(importedEnvironmentSnapshot(name) + `="${` + name + `-}"` + "\n")
	}
	body.WriteString("DOCKPIPE_VSCODE_WORKSPACE_ROOT=${DOCKPIPE_WORKDIR:?Set --workdir to the project workspace}\n")
	body.WriteString("DOCKPIPE_VSCODE_WORKSPACE_NAME=${DOCKPIPE_VSCODE_WORKSPACE_ROOT//\\\\//}\n")
	body.WriteString("DOCKPIPE_VSCODE_WORKSPACE_NAME=${DOCKPIPE_VSCODE_WORKSPACE_NAME##*/}\n")
	cwd, err := quoteImportedValue(command.CWD)
	if err != nil {
		return "", err
	}
	body.WriteString("set -e\ncd -- " + cwd + "\n")
	keys := make([]string, 0, len(command.Env))
	for key := range command.Env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !importedEnvironmentName.MatchString(key) || strings.HasPrefix(key, "DOCKPIPE_VSCODE_") {
			return "", fmt.Errorf("invalid environment name %q", key)
		}
		value, err := quoteImportedValue(command.Env[key])
		if err != nil {
			return "", err
		}
		body.WriteString("export " + key + "=" + value + "\n")
	}
	arguments := append([]string{command.Executable}, command.Args...)
	if command.Shell {
		shell := command.ShellExecutable
		shellArgs := command.ShellArgs
		if shell == "" {
			if runtime.GOOS == "windows" {
				shell, shellArgs = "powershell.exe", []string{"-NoProfile", "-Command"}
			} else {
				shell, shellArgs = "bash", []string{"-c"}
			}
		}
		if len(shellArgs) == 0 {
			switch importedShellName(shell) {
			case "powershell", "powershell.exe", "pwsh", "pwsh.exe":
				shellArgs = []string{"-NoProfile", "-Command"}
			case "bash", "bash.exe", "sh", "sh.exe", "dash", "zsh":
				shellArgs = []string{"-c"}
			case "cmd", "cmd.exe":
				shellArgs = []string{"/d", "/s", "/c"}
			default:
				return "", fmt.Errorf("shell %q needs explicit command arguments", shell)
			}
		}
		line, err := importedShellPreparation(command, shell)
		if err != nil {
			return "", err
		}
		body.WriteString(line)
		arguments = append([]string{shell}, shellArgs...)
	}
	for index, argument := range arguments {
		quoted, err := quoteImportedValue(argument)
		if err != nil {
			return "", err
		}
		if index > 0 {
			body.WriteByte(' ')
		}
		body.WriteString(quoted)
	}
	if command.Shell {
		body.WriteString(` "$DOCKPIPE_VSCODE_SHELL_LINE"`)
	}
	body.WriteByte('\n')
	return body.String(), nil
}

func importedShellPreparation(command vscode.Command, shell string) (string, error) {
	line, err := quoteImportedValue(command.Executable)
	if err != nil {
		return "", err
	}
	var body strings.Builder
	body.WriteString("DOCKPIPE_VSCODE_SHELL_LINE=" + line + "\n")
	name := importedShellName(shell)
	if len(command.Args) > 0 {
		if err := quoteImportedShellVariable(&body, "DOCKPIPE_VSCODE_SHELL_LINE", name); err != nil {
			return "", err
		}
		if name == "powershell" || name == "powershell.exe" || name == "pwsh" || name == "pwsh.exe" {
			body.WriteString("DOCKPIPE_VSCODE_SHELL_LINE=\"& $DOCKPIPE_VSCODE_SHELL_LINE\"\n")
		}
	}
	for _, argument := range command.Args {
		value, err := quoteImportedValue(argument)
		if err != nil {
			return "", err
		}
		body.WriteString("DOCKPIPE_VSCODE_SHELL_ARG=" + value + "\n")
		if err := quoteImportedShellVariable(&body, "DOCKPIPE_VSCODE_SHELL_ARG", name); err != nil {
			return "", err
		}
		body.WriteString("DOCKPIPE_VSCODE_SHELL_LINE+=\" $DOCKPIPE_VSCODE_SHELL_ARG\"\n")
	}
	return body.String(), nil
}

// The variable names are fixed by this adapter; user values are never evaluated
// as code by the outer shell. Inner-shell quoting happens after substitution.
func quoteImportedShellVariable(body *strings.Builder, variable, shell string) error {
	switch shell {
	case "powershell", "powershell.exe", "pwsh", "pwsh.exe":
		body.WriteString(variable + "=${" + variable + "//\\'/\\'\\'}\n")
		body.WriteString(variable + "=\"'$" + variable + "'\"\n")
	case "bash", "bash.exe", "zsh":
		body.WriteString("printf -v " + variable + " '%q' \"$" + variable + "\"\n")
	case "sh", "sh.exe", "dash":
		body.WriteString(variable + "=${" + variable + "//\\'/\\'\\\\\\'\\'}\n")
		body.WriteString(variable + "=\"'$" + variable + "'\"\n")
	case "cmd", "cmd.exe":
		return fmt.Errorf("cmd.exe shell arguments require editor-specific quoting; use a process task or PowerShell")
	default:
		return fmt.Errorf("shell %q needs an explicit argument quoting adapter; use a process task", shell)
	}
	return nil
}

func importedShellName(shell string) string {
	return strings.ToLower(filepath.Base(strings.ReplaceAll(shell, `\`, "/")))
}

func quoteBashLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func quoteImportedValue(value string) (string, error) {
	var result strings.Builder
	for {
		start := strings.Index(value, "${")
		if start < 0 {
			result.WriteString(quoteBashLiteral(value))
			return result.String(), nil
		}
		result.WriteString(quoteBashLiteral(value[:start]))
		end := strings.IndexByte(value[start+2:], '}')
		if end < 0 {
			return "", fmt.Errorf("unterminated imported variable")
		}
		end += start + 2
		variable := value[start+2 : end]
		switch {
		case variable == "workspaceFolder" || variable == "workspaceRoot" || variable == "cwd":
			result.WriteString(`"$DOCKPIPE_VSCODE_WORKSPACE_ROOT"`)
		case variable == "workspaceFolderBasename" || variable == "workspaceRootFolderName":
			result.WriteString(`"$DOCKPIPE_VSCODE_WORKSPACE_NAME"`)
		case variable == "userHome":
			if runtime.GOOS == "windows" {
				result.WriteString(`"${` + importedEnvironmentSnapshot("USERPROFILE") + `-}"`)
			} else {
				result.WriteString(`"${` + importedEnvironmentSnapshot("HOME") + `-}"`)
			}
		case strings.HasPrefix(variable, "env:"):
			name := strings.TrimPrefix(variable, "env:")
			if !importedEnvironmentName.MatchString(name) {
				return "", fmt.Errorf("invalid environment variable %q", name)
			}
			result.WriteString(`"${` + importedEnvironmentSnapshot(name) + `-}"`)
		case strings.HasPrefix(variable, "input:"):
			name := vscode.InputEnvName(strings.TrimPrefix(variable, "input:"))
			result.WriteString(`"${` + name + `?Provide ` + name + ` using --env}"`)
		default:
			return "", fmt.Errorf("${%s} cannot be converted without editor state", variable)
		}
		value = value[end+1:]
	}
}

func importedEnvironmentSnapshot(name string) string {
	return strings.Replace(vscode.InputEnvName(name), "_INPUT_", "_PARENT_ENV_", 1)
}
