package workflowinputs

import (
	"fmt"
	"os"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

type boundField struct {
	Path         string
	EnvName      string
	DefaultValue string
	Description  string
}

func ResolveWorkflow(wf *domain.Workflow, wfConfig, projectRoot string, env map[string]string) (map[string]string, error) {
	if wf == nil || len(wf.Inputs) == 0 {
		return nil, nil
	}
	return resolveInputBindingsEnv(wf, wfConfig, projectRoot, nil, wf.Inputs, env)
}

func ResolveStep(wf *domain.Workflow, wfConfig, projectRoot string, step domain.Step, env map[string]string) (map[string]string, error) {
	if len(step.Inputs) == 0 {
		return nil, nil
	}
	return resolveInputBindingsEnv(wf, wfConfig, projectRoot, &step, step.Inputs, env)
}

func ResolveBindings(wf *domain.Workflow, wfConfig, projectRoot string, bindings map[string]domain.InputBinding, env map[string]string) (map[string]string, error) {
	if len(bindings) == 0 {
		return nil, nil
	}
	return resolveInputBindingsEnv(wf, wfConfig, projectRoot, nil, bindings, env)
}

func resolveInputBindingsEnv(wf *domain.Workflow, wfConfig, projectRoot string, step *domain.Step, bindings map[string]domain.InputBinding, env map[string]string) (map[string]string, error) {
	fields, err := workflowTypedInputFields(wf, wfConfig, projectRoot, step)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(bindings))
	for key, binding := range bindings {
		field, ok := fields[strings.TrimSpace(key)]
		if !ok {
			if step != nil {
				return nil, fmt.Errorf("step input %q does not match any typed field from workflow types", key)
			}
			return nil, fmt.Errorf("workflow input %q does not match any typed field from workflow types", key)
		}
		out[field.EnvName] = resolveInputBinding(binding, env, field.DefaultValue)
	}
	return out, nil
}

func resolveInputBinding(binding domain.InputBinding, env map[string]string, fallback string) string {
	if from := strings.TrimSpace(binding.From); from != "" {
		if env != nil {
			if v, ok := env[from]; ok && strings.TrimSpace(v) != "" {
				return v
			}
		}
		if strings.TrimSpace(binding.Value) != "" {
			return binding.Value
		}
		return fallback
	}
	if strings.TrimSpace(binding.Value) != "" {
		return binding.Value
	}
	return fallback
}

func workflowTypedInputFields(wf *domain.Workflow, wfConfig, projectRoot string, step *domain.Step) (map[string]boundField, error) {
	if wf == nil {
		return nil, nil
	}
	configPath, err := workflowTypedInputsConfigPath(wfConfig, projectRoot)
	if err != nil {
		return nil, err
	}
	// Preserve the existing execution contract: tolerate partial discovery and report
	// the established missing-types error when no usable fields remain.
	records := Discover(configPath, projectRoot, wf, step).Fields
	if len(records) == 0 {
		return nil, workflowTypedInputsMissingError(wf, step)
	}
	out := map[string]boundField{}
	flattenFields("", records, out)
	return out, nil
}

func workflowTypedInputsConfigPath(wfConfig, projectRoot string) (string, error) {
	cfgPath := strings.TrimSpace(wfConfig)
	if cfgPath == "" {
		return "", fmt.Errorf("workflow uses inputs: config path is empty")
	}
	projectRoot = strings.TrimSpace(projectRoot)
	if projectRoot == "" {
		projectRoot, _ = os.Getwd()
	}
	return infrastructure.WorkflowConfigOnDiskPath(projectRoot, cfgPath)
}

func workflowTypedInputsMissingError(wf *domain.Workflow, step *domain.Step) error {
	if wf == nil {
		return fmt.Errorf("workflow uses inputs: but no typed workflow inputs could be resolved")
	}
	if len(wf.Types) > 0 {
		return fmt.Errorf("workflow uses inputs but no typed workflow inputs could be resolved from types")
	}
	resolverName := strings.TrimSpace(wf.Resolver)
	runtimeName := strings.TrimSpace(wf.Runtime)
	if step != nil {
		if v := strings.TrimSpace(step.Resolver); v != "" {
			resolverName = v
		}
		if v := strings.TrimSpace(step.Runtime); v != "" {
			runtimeName = v
		}
	}
	if resolverName != "" || runtimeName != "" {
		return fmt.Errorf("workflow uses inputs: but no typed workflow inputs could be resolved from runtime=%q resolver=%q", runtimeName, resolverName)
	}
	return fmt.Errorf("workflow uses inputs: but has neither explicit types: nor a typed runtime/resolver config")
}

func flattenFields(prefix string, records []Field, out map[string]boundField) {
	for _, record := range records {
		name := strings.TrimSpace(record.FieldName)
		if name == "" {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		if envName := strings.TrimSpace(record.EnvName); envName != "" {
			out[path] = boundField{
				Path:         path,
				EnvName:      envName,
				DefaultValue: record.DefaultValue,
				Description:  record.Description,
			}
		}
		if len(record.Children) > 0 {
			flattenFields(path, record.Children, out)
		}
	}
}
