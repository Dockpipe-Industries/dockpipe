// Package vscode adapts workspace-owned VS Code configuration without executing
// commands, evaluating editor extensions, or changing the source documents.
package vscode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// Limits bound a workspace catalog, not the lifetime or output of its processes.
const MaxDocumentBytes = 2 * 1024 * 1024
const MaxEntries = 256
const MaxCommands = 512

// Entry is a discoverable task workflow or launch application. Unsupported
// entries remain visible so a missing editor extension is never silently ignored.
type Entry struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	Kind              string  `json:"kind"`
	SourceFile        string  `json:"source_file"`
	SourceLabel       string  `json:"source_label"`
	Supported         bool    `json:"supported"`
	UnsupportedReason string  `json:"unsupported_reason,omitempty"`
	Inputs            []Input `json:"inputs,omitempty"`
	raw               map[string]any
	document          *document
}

type Input struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Default     string   `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	Password    bool     `json:"password,omitempty"`
}

type Diagnostic struct {
	SourceFile string `json:"source_file"`
	Message    string `json:"message"`
}

type Catalog struct {
	Entries     []Entry      `json:"entries"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	workspace   string
	options     Options
	tasks       map[string]*Entry
}

type Options struct {
	GOOS        string
	Environment map[string]string
}

type Values struct {
	// Symbolic validates variable references but retains their source spelling for portable export.
	// Callers must quote literals separately from these expressions; never interpolate them as code.
	Symbolic bool

	Inputs        map[string]string
	Configuration map[string]string
}

type Plan struct {
	Entry    Entry     `json:"entry"`
	Commands []Command `json:"commands"`
	Warnings []string  `json:"warnings,omitempty"`
}

// Command preserves process argv separately from shell command text. Shell
// commands require shell-specific quoting when the owning executor appends Args.
type Command struct {
	Name            string            `json:"name"`
	Executable      string            `json:"executable"`
	Args            []string          `json:"args,omitempty"`
	Shell           bool              `json:"shell"`
	ShellExecutable string            `json:"shell_executable,omitempty"`
	ShellArgs       []string          `json:"shell_args,omitempty"`
	CWD             string            `json:"cwd"`
	Env             map[string]string `json:"env,omitempty"`
}

type document struct {
	root   map[string]any
	inputs []Input
}

func Discover(ctx context.Context, workspace string, options Options) (*Catalog, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	if options.GOOS == "" {
		options.GOOS = runtime.GOOS
	}
	if options.Environment == nil {
		options.Environment = make(map[string]string)
		for _, value := range os.Environ() {
			for index, ch := range value {
				if ch == '=' {
					options.Environment[value[:index]] = value[index+1:]
					break
				}
			}
		}
	}
	catalog := &Catalog{Entries: []Entry{}, Diagnostics: []Diagnostic{}, workspace: absolute, options: options, tasks: make(map[string]*Entry)}
	for _, kind := range []string{"task", "launch"} {
		filename, collection, label := "tasks.json", "tasks", "label"
		if kind == "launch" {
			filename, collection, label = "launch.json", "configurations", "name"
		}
		source := filepath.Join(absolute, ".vscode", filename)
		doc, err := readDocument(ctx, source)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			catalog.Diagnostics = append(catalog.Diagnostics, Diagnostic{source, err.Error()})
			continue
		}
		values, ok := doc.root[collection].([]any)
		if !ok && doc.root[collection] != nil {
			catalog.Diagnostics = append(catalog.Diagnostics, Diagnostic{source, collection + " must be an array"})
			continue
		}
		if len(values) > MaxEntries || len(catalog.Entries)+len(values) > MaxEntries {
			return nil, fmt.Errorf("VS Code catalog exceeds %d entries", MaxEntries)
		}
		seen := make(map[string]bool)
		for _, value := range values {
			raw, ok := value.(map[string]any)
			if !ok {
				catalog.Diagnostics = append(catalog.Diagnostics, Diagnostic{source, "configuration must be an object"})
				continue
			}
			name := stringValue(raw[label])
			if name == "" {
				catalog.Diagnostics = append(catalog.Diagnostics, Diagnostic{source, "configuration is missing " + label})
				continue
			}
			digest := sha256.Sum256([]byte(kind + "\x00" + name))
			entry := Entry{ID: "vscode-" + kind + "-" + hex.EncodeToString(digest[:8]), Name: name, Kind: kind, SourceFile: source, SourceLabel: "VS Code", Inputs: doc.inputs, raw: raw, document: doc}
			if seen[name] {
				entry.UnsupportedReason = "duplicate " + label + ": " + name
			}
			seen[name] = true
			catalog.Entries = append(catalog.Entries, entry)
		}
	}
	byID := make(map[string]*Entry)
	for index := range catalog.Entries {
		entry := &catalog.Entries[index]
		if previous := byID[entry.ID]; previous != nil {
			previous.UnsupportedReason = "duplicate configuration name: " + entry.Name
			entry.UnsupportedReason = previous.UnsupportedReason
		}
		byID[entry.ID] = entry
	}
	// Take pointers only after the slice stops growing.
	for index := range catalog.Entries {
		entry := &catalog.Entries[index]
		if entry.Kind == "task" {
			if previous := catalog.tasks[entry.Name]; previous != nil {
				previous.UnsupportedReason = "duplicate task label: " + entry.Name
				entry.UnsupportedReason = previous.UnsupportedReason
			}
			catalog.tasks[entry.Name] = entry
		}
	}
	for index := range catalog.Entries {
		entry := &catalog.Entries[index]
		if entry.UnsupportedReason == "" {
			plan, err := catalog.resolve(ctx, entry.ID, Values{}, true)
			if err != nil {
				entry.UnsupportedReason = err.Error()
			} else {
				entry.Inputs = plan.Entry.Inputs
			}
		}
		entry.Supported = entry.UnsupportedReason == ""
	}
	return catalog, nil
}

func readDocument(ctx context.Context, path string) (*document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, MaxDocumentBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) > MaxDocumentBytes {
		return nil, fmt.Errorf("VS Code document exceeds %d bytes", MaxDocumentBytes)
	}
	normalized, err := normalizeJSONC(content)
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(normalized, &root); err != nil {
		return nil, fmt.Errorf("invalid VS Code JSONC: %w", err)
	}
	if root == nil {
		return nil, fmt.Errorf("VS Code document must be an object")
	}
	doc := &document{root: root}
	if entries, ok := root["inputs"].([]any); ok {
		if len(entries) > MaxEntries {
			return nil, fmt.Errorf("VS Code document exceeds %d input definitions", MaxEntries)
		}
		inputIDs := make(map[string]bool)
		for _, value := range entries {
			raw, ok := value.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("input must be an object")
			}
			input := Input{ID: stringValue(raw["id"]), Type: stringValue(raw["type"]), Description: stringValue(raw["description"]), Default: stringValue(raw["default"]), Password: boolValue(raw["password"])}
			if input.ID == "" {
				return nil, fmt.Errorf("input is missing id")
			}
			if inputIDs[input.ID] {
				return nil, fmt.Errorf("duplicate input id %q", input.ID)
			}
			inputIDs[input.ID] = true
			if options, ok := raw["options"].([]any); ok {
				if len(options) > MaxEntries {
					return nil, fmt.Errorf("input %q exceeds %d options", input.ID, MaxEntries)
				}
				for _, option := range options {
					if object, ok := option.(map[string]any); ok {
						input.Options = append(input.Options, stringValue(object["value"]))
					} else {
						input.Options = append(input.Options, stringValue(option))
					}
				}
			}
			doc.inputs = append(doc.inputs, input)
		}
	}
	return doc, nil
}

func stringValue(value any) string { result, _ := value.(string); return result }
func boolValue(value any) bool     { result, _ := value.(bool); return result }
