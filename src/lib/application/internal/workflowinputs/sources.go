package workflowinputs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"gopkg.in/yaml.v3"
)

type typeEntry struct {
	Spec       string
	ModuleRoot string
}

type workflowTypeMapDoc struct {
	Types []string `yaml:"types"`
}

func workflowTypeEntries(cfgPath, projectRoot string, wf *domain.Workflow, step *domain.Step) ([]typeEntry, error) {
	if wf == nil {
		return nil, nil
	}
	moduleRoot := filepath.Dir(cfgPath)
	if len(wf.Types) > 0 {
		out := make([]typeEntry, 0, len(wf.Types))
		for _, spec := range wf.Types {
			spec = strings.TrimSpace(spec)
			if spec == "" {
				continue
			}
			out = append(out, typeEntry{Spec: spec, ModuleRoot: moduleRoot})
		}
		return out, nil
	}

	projectRoot = strings.TrimSpace(projectRoot)
	if projectRoot == "" {
		var err error
		projectRoot, err = domain.FindProjectRootWithDockpipeConfig(moduleRoot)
		if err != nil {
			return nil, err
		}
	}
	runtimeName := strings.TrimSpace(wf.Runtime)
	resolverName := strings.TrimSpace(wf.Resolver)
	if step != nil {
		if v := strings.TrimSpace(step.Runtime); v != "" {
			runtimeName = v
		}
		if v := strings.TrimSpace(step.Resolver); v != "" {
			resolverName = v
		}
	}

	var out []typeEntry
	if runtimeName != "" {
		entries, err := typedEntriesFromRuntimeConfig(projectRoot, runtimeName)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}
	if resolverName != "" {
		entries, err := typedEntriesFromResolverConfig(projectRoot, resolverName)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}
	return dedupeTypeEntries(out), nil
}

func typedEntriesFromResolverConfig(projectRoot, resolverName string) ([]typeEntry, error) {
	profilePath, err := infrastructure.ResolveResolverFilePath(projectRoot, resolverName)
	if err != nil {
		return nil, nil
	}
	return typedEntriesFromAdjacentConfig(profilePath)
}

func typedEntriesFromRuntimeConfig(projectRoot, runtimeName string) ([]typeEntry, error) {
	profilePath, err := infrastructure.ResolveRuntimeFilePath(projectRoot, runtimeName)
	if err != nil {
		return nil, nil
	}
	return typedEntriesFromAdjacentConfig(profilePath)
}

func typedEntriesFromAdjacentConfig(profilePath string) ([]typeEntry, error) {
	dir := filepath.Dir(profilePath)
	for _, base := range []string{"types.yml", "config.yml"} {
		configPath := filepath.Join(dir, base)
		b, err := os.ReadFile(configPath)
		if err != nil {
			continue
		}
		var doc workflowTypeMapDoc
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return nil, fmt.Errorf("parse %s: %w", configPath, err)
		}
		moduleRoot := filepath.Dir(configPath)
		out := make([]typeEntry, 0, len(doc.Types))
		for _, spec := range doc.Types {
			spec = strings.TrimSpace(spec)
			if spec == "" {
				continue
			}
			out = append(out, typeEntry{Spec: spec, ModuleRoot: moduleRoot})
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	return nil, nil
}

func dedupeTypeEntries(in []typeEntry) []typeEntry {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]typeEntry, 0, len(in))
	for _, entry := range in {
		spec := strings.TrimSpace(entry.Spec)
		root := strings.TrimSpace(entry.ModuleRoot)
		if spec == "" || root == "" {
			continue
		}
		key := root + "\x00" + spec
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, typeEntry{Spec: spec, ModuleRoot: root})
	}
	return out
}
