package vscode

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

func (state *resolution) expand(entry *Entry, text string) (string, error) {
	if strings.IndexByte(text, 0) >= 0 {
		return "", fmt.Errorf("%s: command field contains a NUL byte", entry.Name)
	}
	var output strings.Builder
	for {
		start := strings.Index(text, "${")
		if start < 0 {
			output.WriteString(text)
			break
		}
		output.WriteString(text[:start])
		end := strings.IndexByte(text[start+2:], '}')
		if end < 0 {
			return "", fmt.Errorf("%s: unterminated variable", entry.Name)
		}
		end += start + 2
		variable := text[start+2 : end]
		value, err := state.variable(entry, variable)
		if err != nil {
			return "", err
		}
		if strings.IndexByte(value, 0) >= 0 {
			return "", fmt.Errorf("%s: variable contains a NUL byte", entry.Name)
		}
		output.WriteString(value)
		text = text[end+1:]
	}
	return output.String(), nil
}

var environmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func (state *resolution) variable(entry *Entry, name string) (string, error) {
	if strings.HasPrefix(name, "env:") && !environmentName.MatchString(strings.TrimPrefix(name, "env:")) {
		return "", fmt.Errorf("%s: invalid environment variable reference", entry.Name)
	}
	if state.values.Symbolic {
		switch name {
		case "workspaceFolder", "workspaceRoot", "workspaceFolderBasename", "workspaceRootFolderName", "cwd", "userHome", "pathSeparator", "/":
			return "${" + name + "}", nil
		}
		if strings.HasPrefix(name, "env:") || strings.HasPrefix(name, "config:") {
			return "${" + name + "}", nil
		}
	}
	switch name {
	case "workspaceFolder", "workspaceRoot":
		return state.catalog.workspace, nil
	case "workspaceFolderBasename", "workspaceRootFolderName":
		return filepath.Base(state.catalog.workspace), nil
	case "cwd":
		return state.catalog.workspace, nil
	case "pathSeparator", "/":
		if state.catalog.options.GOOS == "windows" {
			return "\\", nil
		}
		return "/", nil
	case "userHome":
		key := "HOME"
		if state.catalog.options.GOOS == "windows" {
			key = "USERPROFILE"
		}
		return state.catalog.options.Environment[key], nil
	}
	if strings.HasPrefix(name, "env:") {
		return state.catalog.options.Environment[strings.TrimPrefix(name, "env:")], nil
	}
	if strings.HasPrefix(name, "config:") {
		if value, ok := state.values.Configuration[strings.TrimPrefix(name, "config:")]; ok {
			return value, nil
		}
		if state.allowMissing {
			return "${" + name + "}", nil
		}
		return "", fmt.Errorf("%s: provide a value for %s; editor settings are not evaluated", entry.Name, name)
	}
	if strings.HasPrefix(name, "input:") {
		id := strings.TrimPrefix(name, "input:")
		var definition *Input
		for index := range entry.document.inputs {
			if entry.document.inputs[index].ID == id {
				definition = &entry.document.inputs[index]
				break
			}
		}
		if definition == nil {
			return "", fmt.Errorf("%s: input %q is not defined", entry.Name, id)
		}
		recorded := false
		for _, input := range state.plan.Entry.Inputs {
			if input.ID == id {
				if input.Type != definition.Type || input.Password != definition.Password || strings.Join(input.Options, "\x00") != strings.Join(definition.Options, "\x00") {
					return "", fmt.Errorf("%s: input %q has incompatible task and launch definitions", entry.Name, id)
				}
				recorded = true
				break
			}
		}
		if !recorded {
			state.plan.Entry.Inputs = append(state.plan.Entry.Inputs, *definition)
		}
		if definition.Type != "promptString" && definition.Type != "pickString" {
			return "", fmt.Errorf("%s: input %q requires a VS Code command provider", entry.Name, id)
		}
		value, supplied := state.values.Inputs[id]
		if state.values.Symbolic {
			return "${" + name + "}", nil
		}
		if !supplied {
			if state.allowMissing {
				return "${" + name + "}", nil
			}
			return "", fmt.Errorf("%s: provide input %q", entry.Name, id)
		}
		if definition.Type == "pickString" {
			found := false
			for _, option := range definition.Options {
				if option == value {
					found = true
					break
				}
			}
			if !found {
				return "", fmt.Errorf("%s: input %q is not one of the declared options", entry.Name, id)
			}
		}
		return value, nil
	}
	return "", fmt.Errorf("%s: variable ${%s} needs an editor context or extension", entry.Name, name)
}

// InputEnvName maps arbitrary input identifiers onto collision-resistant process environment keys.
func InputEnvName(id string) string {
	digest := sha256.Sum256([]byte(id))
	return "DOCKPIPE_VSCODE_INPUT_" + strings.ToUpper(hex.EncodeToString(digest[:8]))
}
