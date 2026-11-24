package application

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"dockpipe/src/lib/model/managedenvironment"
)

const maxEnvironmentDefinitionEntries = 2048
const environmentDefinitionsSchema = "dockpipe.environment.definitions/v1"

type environmentDefinitionEntry struct {
	Scope      string                         `json:"scope"`
	Path       string                         `json:"path"`
	InstanceID string                         `json:"instance_id,omitempty"`
	Runtime    *environmentRuntimeStatus      `json:"runtime,omitempty"`
	Definition *managedenvironment.Definition `json:"definition,omitempty"`
	Error      string                         `json:"error,omitempty"`
	Reason     string                         `json:"reason,omitempty"`
}

func cmdEnvironmentDefinitions(args []string) error {
	flags := flag.NewFlagSet("environment definitions", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() { fmt.Print(environmentUsage) }
	var files []string
	observe := flags.Bool("observe", false, "Read process status for saved environments")
	flags.Func("file", "Include a definition at an explicit path", func(path string) error {
		if path == "" || len(files) >= maxEnvironmentDefinitionEntries {
			return errors.New("a definition path is required and the list is bounded")
		}
		files = append(files, path)
		return nil
	})
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("environment definitions requires --file for explicit paths")
	}

	entries, err := collectEnvironmentDefinitions(files)
	if err != nil {
		return err
	}
	if *observe {
		observeEnvironmentDefinitions(entries)
	}
	document := struct {
		Schema      string                       `json:"schema"`
		Definitions []environmentDefinitionEntry `json:"definitions"`
	}{Schema: environmentDefinitionsSchema, Definitions: entries}

	return json.NewEncoder(os.Stdout).Encode(document)
}

func observeEnvironmentDefinitions(entries []environmentDefinitionEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	indices := make(chan int)
	var workers sync.WaitGroup
	for range min(4, len(entries)) {
		workers.Go(func() {
			for index := range indices {
				entry := &entries[index]
				observation := observeEnvironmentRuntime(ctx, entry.Definition.Kind, entry.InstanceID)
				if observation.Reason != "no-runtime-record" {
					entry.Runtime = &observation
				}
			}
		})
	}

observe:
	for index := range entries {
		if entries[index].InstanceID == "" {
			continue
		}
		select {
		case indices <- index:
		case <-ctx.Done():
			break observe
		}
	}
	close(indices)
	workers.Wait()
}

func collectEnvironmentDefinitions(files []string) ([]environmentDefinitionEntry, error) {
	directory, err := personalEnvironmentDirectory()
	if err != nil {
		return nil, err
	}
	children, err := readPersonalEnvironmentChildren(directory)
	if err != nil {
		return nil, err
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].Name() < children[j].Name()
	})

	entries := make([]environmentDefinitionEntry, 0)
	seen := make(map[string]bool)
	for _, child := range children {
		name, validExtension := strings.CutSuffix(child.Name(), ".yaml")
		if !validExtension || !managedenvironment.ValidName(name) || child.IsDir() {
			continue
		}
		path := filepath.Join(directory, child.Name())
		entries = append(entries, inspectEnvironmentDefinition("personal", path, name))
		seen[path] = true
	}
	for _, file := range files {
		path, err := filepath.Abs(file)
		if err != nil {
			return nil, err
		}
		if seen[path] {
			continue
		}
		entries = append(entries, inspectEnvironmentDefinition("explicit", path, ""))
		seen[path] = true
	}

	return entries, nil
}

func readPersonalEnvironmentChildren(directory string) ([]os.DirEntry, error) {
	file, err := os.Open(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	children, err := file.ReadDir(maxEnvironmentDefinitionEntries + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(children) > maxEnvironmentDefinitionEntries {
		return nil, errors.New("environment definitions directory exceeds the entry limit")
	}

	return children, nil
}

func inspectEnvironmentDefinition(scope, path, expectedName string) environmentDefinitionEntry {
	entry := environmentDefinitionEntry{Scope: scope, Path: path}
	definition, err := readEnvironment(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		entry.Error = "missing"
	case errors.Is(err, managedenvironment.ErrKubernetesPodUnsupported):
		entry.Error = "invalid"
		entry.Reason = "unsupported-kind"
	case err != nil:
		entry.Error = "invalid"
	case expectedName != "" && expectedName != definition.Name:
		entry.Error = "name-mismatch"
	default:
		entry.Definition = &definition
		entry.InstanceID = observedEnvironmentInstanceID(path)
	}

	return entry
}

func observedEnvironmentInstanceID(path string) string {
	recordPath, err := environmentInstanceRecordPath(path)
	if err != nil {
		return ""
	}
	record, err := readEnvironmentInstance(recordPath)
	if err != nil {
		return ""
	}

	return record.ID
}
