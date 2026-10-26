package workflowinputs

import (
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	pipelangmaterialize "github.com/Dockpipe-Industries/pipelang/sdk/materialize"
	"github.com/Dockpipe-Industries/pipelang/src/lib/pipelang"
)

// Discover reports source-selection errors before collecting fields. Once sources
// are selected, individual type-entry failures do not discard other valid fields.
func Discover(configPath, projectRoot string, workflow *domain.Workflow, step *domain.Step) Discovery {
	entries, err := workflowTypeEntries(configPath, projectRoot, workflow, step)
	if err != nil {
		return Discovery{Diagnostics: []error{err}}
	}
	if len(entries) == 0 {
		return Discovery{}
	}
	return discoverFields(entries, workflow.Vars)
}

func discoverFields(entries []typeEntry, workflowVars map[string]string) Discovery {
	defaultsByClass := map[string]map[string]string{}
	seen := map[string]struct{}{}
	filesByRoot := map[string]map[string][]byte{}
	progByRoot := map[string]*pipelang.Program{}
	var result Discovery

	for _, entry := range entries {
		filePath, typeRef, err := parseTypeSpec(entry.ModuleRoot, entry.Spec)
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, err)
			continue
		}
		typeRoot := filepath.Dir(filePath)
		files, ok := filesByRoot[typeRoot]
		if !ok {
			files, _, err = pipelangmaterialize.ReadFilesUnder(typeRoot)
			if err != nil {
				result.Diagnostics = append(result.Diagnostics, err)
				continue
			}
			if len(files) == 0 {
				continue
			}
			filesByRoot[typeRoot] = files
		}
		prog, ok := progByRoot[typeRoot]
		if !ok {
			prog, err = mergePipeLangProgram(files)
			if err != nil {
				result.Diagnostics = append(result.Diagnostics, err)
				continue
			}
			progByRoot[typeRoot] = prog
		}
		shape := findTypeShape(prog, typeRef)
		if shape == nil {
			continue
		}
		docsByType := fieldDocsByType(files)
		className := shape.ClassName
		if className == "" {
			className, err = pipelangmaterialize.InferEntryClassFromTypeRef(files, typeRef)
		}
		classDefaults := map[string]string{}
		if className != "" && err == nil {
			if cached, ok := defaultsByClass[className]; ok {
				classDefaults = cached
			} else {
				classDefaults = literalClassDefaults(prog, className)
				defaultsByClass[className] = classDefaults
			}
		}
		envPrefix := inferredEnvPrefix(shape.ClassName, typeRef)
		shapeName := strings.TrimSpace(typeRef)
		if strings.TrimSpace(shape.ClassName) != "" {
			shapeName = strings.TrimSpace(shape.ClassName)
		}
		for _, field := range shape.Fields {
			record := buildField(prog, field, envPrefix, shapeName, docsByType, classDefaults, workflowVars, seen, 0)
			if record == nil {
				continue
			}
			result.Fields = append(result.Fields, *record)
		}
	}
	return result
}
