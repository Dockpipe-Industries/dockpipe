package vscode

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

func (catalog *Catalog) Resolve(ctx context.Context, id string, values Values) (*Plan, error) {
	return catalog.resolve(ctx, id, values, false)
}

func (catalog *Catalog) resolve(ctx context.Context, id string, values Values, allowMissing bool) (*Plan, error) {
	var selected *Entry
	for index := range catalog.Entries {
		if catalog.Entries[index].ID == id {
			selected = &catalog.Entries[index]
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("VS Code entry %q was not found", id)
	}
	if selected.UnsupportedReason != "" {
		return nil, fmt.Errorf("%s", selected.UnsupportedReason)
	}
	state := resolution{catalog: catalog, values: values, allowMissing: allowMissing, visiting: make(map[string]bool), completed: make(map[string]bool), plan: &Plan{Entry: *selected, Commands: []Command{}}}
	state.plan.Entry.Inputs = nil
	if err := state.appendEntry(ctx, selected); err != nil {
		return nil, err
	}
	return state.plan, nil
}

type resolution struct {
	catalog      *Catalog
	values       Values
	allowMissing bool
	visiting     map[string]bool
	completed    map[string]bool
	plan         *Plan
}

func (state *resolution) appendEntry(ctx context.Context, entry *Entry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if state.visiting[entry.ID] {
		return fmt.Errorf("VS Code dependency cycle at %q", entry.Name)
	}
	if state.completed[entry.ID] {
		return nil
	}
	if entry.UnsupportedReason != "" {
		return fmt.Errorf("%s: %s", entry.Name, entry.UnsupportedReason)
	}
	state.visiting[entry.ID] = true
	defer delete(state.visiting, entry.ID)
	raw := state.effective(entry)
	if entry.Kind == "launch" {
		if stringValue(raw["request"]) != "launch" {
			return fmt.Errorf("%s: only launch requests can run without an editor debugger", entry.Name)
		}
		for _, field := range []string{"postDebugTask", "envFile", "pipeTransport", "debugServer", "serverReadyAction", "setupCommands", "miDebuggerArgs", "customLaunchSetupCommands", "runtimeVersion", "launchSettingsProfile", "launchSettingsFilePath"} {
			if raw[field] != nil {
				return fmt.Errorf("%s: %s requires editor/debugger integration", entry.Name, field)
			}
		}
		if task := stringValue(raw["preLaunchTask"]); task != "" {
			if err := state.appendTask(ctx, task); err != nil {
				return err
			}
		}
	} else {
		if boolValue(raw["isBackground"]) {
			return fmt.Errorf("%s: background readiness/problem matchers require editor integration", entry.Name)
		}
		dependencies, err := stringList(raw["dependsOn"])
		if err != nil {
			return fmt.Errorf("%s: dependsOn %w", entry.Name, err)
		}
		if len(dependencies) > 1 && stringValue(raw["dependsOrder"]) != "sequence" {
			return fmt.Errorf("%s: parallel task dependencies are not supported yet; use dependsOrder sequence", entry.Name)
		}
		for _, task := range dependencies {
			if err := state.appendTask(ctx, task); err != nil {
				return err
			}
		}
	}
	command, err := state.command(entry, raw)
	if err != nil {
		return err
	}
	if command.Executable != "" {
		if len(state.plan.Commands) >= MaxCommands {
			return fmt.Errorf("VS Code plan exceeds %d commands", MaxCommands)
		}
		state.plan.Commands = append(state.plan.Commands, command)
	}
	state.completed[entry.ID] = true
	return nil
}

func (state *resolution) appendTask(ctx context.Context, label string) error {
	task := state.catalog.tasks[label]
	if task == nil {
		return fmt.Errorf("VS Code dependency task %q was not found", label)
	}
	return state.appendEntry(ctx, task)
}

func (state *resolution) effective(entry *Entry) map[string]any {
	result := make(map[string]any)
	if entry.Kind == "task" {
		for key, value := range entry.document.root {
			if key != "tasks" && key != "inputs" {
				result[key] = value
			}
		}
		state.mergePlatform(result, entry.document.root)
	}
	for key, value := range entry.raw {
		if key == "options" {
			result[key] = mergeObjects(result[key], value)
		} else {
			result[key] = value
		}
	}
	state.mergePlatform(result, entry.raw)
	return result
}

func (state *resolution) mergePlatform(result map[string]any, source map[string]any) {
	platform := state.catalog.options.GOOS
	if platform == "darwin" {
		platform = "osx"
	}
	if override, ok := source[platform].(map[string]any); ok {
		for key, value := range override {
			if key == "options" {
				result[key] = mergeObjects(result[key], value)
			} else {
				result[key] = value
			}
		}
	}
}

func mergeObjects(base any, override any) map[string]any {
	result := make(map[string]any)
	if object, ok := base.(map[string]any); ok {
		for key, value := range object {
			result[key] = value
		}
	}
	if object, ok := override.(map[string]any); ok {
		for key, value := range object {
			if key == "env" {
				environment := make(map[string]any)
				if previous, ok := result[key].(map[string]any); ok {
					for name, item := range previous {
						environment[name] = item
					}
				}
				if additions, ok := value.(map[string]any); ok {
					for name, item := range additions {
						environment[name] = item
					}
				}
				result[key] = environment
			} else {
				result[key] = value
			}
		}
	}
	return result
}

func (state *resolution) command(entry *Entry, raw map[string]any) (Command, error) {
	command := Command{Name: entry.Name, CWD: state.catalog.workspace, Env: make(map[string]string)}
	if state.values.Symbolic {
		command.CWD = "${workspaceFolder}"
	}
	var arguments any
	var options map[string]any
	if entry.Kind == "task" {
		kind := stringValue(raw["type"])
		if kind != "" && kind != "shell" && kind != "process" {
			return command, fmt.Errorf("%s: task type %q requires a VS Code extension", entry.Name, kind)
		}
		command.Executable = stringValue(raw["command"])
		if raw["command"] != nil && command.Executable == "" {
			return command, fmt.Errorf("%s: command objects require editor-specific shell quoting", entry.Name)
		}
		command.Shell = kind != "process"
		arguments = raw["args"]
		options, _ = raw["options"].(map[string]any)
		dependencies, dependencyErr := stringList(raw["dependsOn"])
		if dependencyErr != nil {
			return command, dependencyErr
		}
		if command.Executable == "" && len(dependencies) == 0 {
			return command, fmt.Errorf("%s: task has no command or dependencies", entry.Name)
		}
	} else {
		kind := stringValue(raw["type"])
		program := stringValue(raw["program"])
		command.Executable = program
		arguments = raw["args"]
		options = raw
		runtime := stringValue(raw["runtimeExecutable"])
		runtimeArguments, err := stringList(raw["runtimeArgs"])
		if err != nil {
			return command, fmt.Errorf("%s: runtimeArgs %w", entry.Name, err)
		}
		switch kind {
		case "node", "pwa-node":
			if runtime == "" {
				runtime = "node"
			}
			command.Executable = runtime
			command.Args = append(command.Args, runtimeArguments...)
			if program != "" {
				command.Args = append(command.Args, program)
			}
			if program == "" && runtime == "node" {
				return command, fmt.Errorf("%s: Node launch needs program", entry.Name)
			}
		case "python", "debugpy":
			runtime = stringValue(raw["python"])
			if runtime == "" {
				runtime = stringValue(raw["pythonPath"])
			}
			if runtime == "" {
				runtime = "python3"
				if state.catalog.options.GOOS == "windows" {
					runtime = "python"
				}
			}
			command.Executable = runtime
			if module := stringValue(raw["module"]); module != "" {
				command.Args = append(command.Args, "-m", module)
			} else if program != "" {
				command.Args = append(command.Args, program)
			} else {
				return command, fmt.Errorf("%s: Python launch needs program or module", entry.Name)
			}
		case "coreclr", "clr":
			if strings.EqualFold(filepath.Ext(program), ".dll") {
				command.Executable = "dotnet"
				command.Args = append(command.Args, program)
			}
		case "cppdbg", "cppvsdbg":
		default:
			return command, fmt.Errorf("%s: launch type %q requires an unsupported debug adapter", entry.Name, kind)
		}
		if command.Executable == "" {
			return command, fmt.Errorf("%s: launch has no executable program", entry.Name)
		}
		state.plan.Warnings = append(state.plan.Warnings, entry.Name+": runs as an application without debugger, breakpoints, or debug-adapter hooks")
	}
	args, err := stringList(arguments)
	if err != nil {
		return command, fmt.Errorf("%s: args %w", entry.Name, err)
	}
	command.Args = append(command.Args, args...)
	if options != nil {
		if cwd := stringValue(options["cwd"]); cwd != "" {
			command.CWD = cwd
		}
		if environment, ok := options["env"].(map[string]any); ok {
			for name, value := range environment {
				text, ok := value.(string)
				if !ok {
					return command, fmt.Errorf("%s: env %s must be a string", entry.Name, name)
				}
				if !environmentName.MatchString(name) {
					return command, fmt.Errorf("%s: invalid environment variable name", entry.Name)
				}
				command.Env[name] = text
			}
		} else if options["env"] != nil {
			return command, fmt.Errorf("%s: env must be an object", entry.Name)
		}
		if shell, ok := options["shell"].(map[string]any); ok {
			command.ShellExecutable = stringValue(shell["executable"])
			command.ShellArgs, err = stringList(shell["args"])
			if err != nil {
				return command, fmt.Errorf("%s: shell args %w", entry.Name, err)
			}
		}
	}
	fields := []*string{&command.Executable, &command.CWD, &command.ShellExecutable}
	for index := range command.Args {
		fields = append(fields, &command.Args[index])
	}
	for index := range command.ShellArgs {
		fields = append(fields, &command.ShellArgs[index])
	}
	for _, field := range fields {
		*field, err = state.expand(entry, *field)
		if err != nil {
			return command, err
		}
	}
	for name, value := range command.Env {
		command.Env[name], err = state.expand(entry, value)
		if err != nil {
			return command, err
		}
	}
	if !filepath.IsAbs(command.CWD) && !strings.HasPrefix(command.CWD, "${") {
		if state.values.Symbolic {
			command.CWD = "${workspaceFolder}/" + command.CWD
		} else {
			command.CWD = filepath.Join(state.catalog.workspace, command.CWD)
		}
	}
	return command, nil
}

func stringList(value any) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	if text, ok := value.(string); ok {
		return []string{text}, nil
	}
	array, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("must be a string or array of strings")
	}
	result := make([]string, 0, len(array))
	for _, item := range array {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("must contain only strings; quoted argument objects are not supported")
		}
		result = append(result, text)
	}
	return result, nil
}
