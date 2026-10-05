package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type parsedPlannerProposal struct {
	Format      string         `json:"format"`
	Declaration map[string]any `json:"declaration"`
}

// parsePlannerProposal accepts exactly one structured JSON or YAML document. It returns data only;
// the result is not executable until compileExecutableContract validates the complete contract.
func parsePlannerProposal(raw []byte) (*parsedPlannerProposal, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, errors.New("planner proposal is empty")
	}

	decoder := yaml.NewDecoder(bytes.NewReader(trimmed))
	document := yaml.Node{}
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("planner proposal is not valid JSON or YAML: %w", err)
	}
	extra := yaml.Node{}
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("planner proposal must contain exactly one structured document")
		}
		return nil, fmt.Errorf("planner proposal is not valid JSON or YAML: %w", err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("planner proposal root must be an object")
	}

	declaration := map[string]any{}
	if err := document.Content[0].Decode(&declaration); err != nil {
		return nil, fmt.Errorf("planner proposal is not valid JSON or YAML: %w", err)
	}
	if problems := validatePlannerProposal(declaration); len(problems) > 0 {
		return nil, fmt.Errorf("invalid planner proposal: %s", strings.Join(problems, "; "))
	}
	format := "yaml"
	if json.Valid(trimmed) {
		format = "json"
	}
	return &parsedPlannerProposal{Format: format, Declaration: declaration}, nil
}

func validatePlannerProposal(proposal map[string]any) []string {
	problems := []string{}
	appendUnknownProposalFields(proposal, "planner proposal root", []string{
		"startup_prompt", "include_agents_md", "access", "cloud_budget", "constraints", "publish", "sync", "orchestration",
	}, &problems)
	validateProposalOptionalString(proposal, "startup_prompt", "planner proposal.startup_prompt", &problems)
	validateProposalOptionalBool(proposal, "include_agents_md", "planner proposal.include_agents_md", &problems)
	validateProposalStringListField(proposal, "constraints", "planner proposal.constraints", &problems)
	validateProposalOptionalBool(proposal, "publish", "planner proposal.publish", &problems)
	validateProposalOptionalBool(proposal, "sync", "planner proposal.sync", &problems)
	validateProposalAccess(proposal["access"], "planner proposal.access", &problems)
	validateProposalBudget(proposal["cloud_budget"], "planner proposal.cloud_budget", &problems)

	orchestration, ok := mapDeclaration(proposal["orchestration"])
	if !ok {
		problems = append(problems, "planner proposal.orchestration is required")
		return problems
	}
	appendUnknownProposalFields(orchestration, "planner proposal.orchestration", []string{
		"request", "plan", "agents", "tasks", "merge", "verify", "apply", "access", "cloud_budget", "constraints", "publish", "sync",
	}, &problems)
	validateProposalRequest(orchestration["request"], "planner proposal.orchestration.request", &problems)
	validateProposalPlan(orchestration["plan"], "planner proposal.orchestration.plan", &problems)
	validateProposalRoles(orchestration["agents"], "planner proposal.orchestration.agents", &problems)
	validateProposalTasks(orchestration["tasks"], "planner proposal.orchestration.tasks", &problems)
	validateProposalScalarMap(orchestration["merge"], "planner proposal.orchestration.merge", &problems)
	validateProposalScalarMap(orchestration["verify"], "planner proposal.orchestration.verify", &problems)
	validateProposalApply(orchestration["apply"], "planner proposal.orchestration.apply", &problems)
	validateProposalAccess(orchestration["access"], "planner proposal.orchestration.access", &problems)
	validateProposalBudget(orchestration["cloud_budget"], "planner proposal.orchestration.cloud_budget", &problems)
	validateProposalStringListField(orchestration, "constraints", "planner proposal.orchestration.constraints", &problems)
	validateProposalOptionalBool(orchestration, "publish", "planner proposal.orchestration.publish", &problems)
	validateProposalOptionalBool(orchestration, "sync", "planner proposal.orchestration.sync", &problems)
	return problems
}

func appendUnknownProposalFields(value map[string]any, field string, allowed []string, problems *[]string) {
	allowedSet := map[string]bool{}
	for _, key := range allowed {
		allowedSet[key] = true
	}
	unknown := []string{}
	for key := range value {
		if !allowedSet[key] {
			unknown = append(unknown, key)
		}
	}
	sort.Strings(unknown)
	for _, key := range unknown {
		*problems = append(*problems, fmt.Sprintf("%s field %q is not allowed", field, key))
	}
}

func validateProposalOptionalString(value map[string]any, key, field string, problems *[]string) {
	raw, present := value[key]
	if !present {
		return
	}
	if _, ok := raw.(string); !ok {
		*problems = append(*problems, field+" must be a string")
	}
}

func validateProposalRequiredString(value map[string]any, key, field string, problems *[]string) {
	raw, present := value[key]
	text, ok := raw.(string)
	if !present || !ok || strings.TrimSpace(text) == "" {
		*problems = append(*problems, field+" is required")
	}
}

func validateProposalOptionalBool(value map[string]any, key, field string, problems *[]string) {
	raw, present := value[key]
	if !present {
		return
	}
	if _, ok := raw.(bool); !ok {
		*problems = append(*problems, field+" must be a boolean")
	}
}

func proposalArray(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case []string:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out, true
	default:
		return nil, false
	}
}

func validateProposalStringListField(value map[string]any, key, field string, problems *[]string) {
	raw, present := value[key]
	if !present {
		return
	}
	items, ok := proposalArray(raw)
	if !ok {
		*problems = append(*problems, field+" must be an array of strings")
		return
	}
	for _, item := range items {
		if _, ok := item.(string); !ok {
			*problems = append(*problems, field+" must be an array of strings")
			return
		}
	}
}

func proposalInteger(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case uint64:
		if typed <= uint64(^uint(0)>>1) {
			return int(typed), true
		}
	case float64:
		if typed == math.Trunc(typed) && typed >= 0 && typed <= float64(^uint(0)>>1) {
			return int(typed), true
		}
	}
	return 0, false
}

func validateProposalAccess(value any, field string, problems *[]string) {
	if value == nil {
		return
	}
	access, ok := mapDeclaration(value)
	if !ok {
		*problems = append(*problems, field+" must be an object")
		return
	}
	appendUnknownProposalFields(access, field, []string{"read", "write", "deny", "remove_deny"}, problems)
	for _, key := range []string{"read", "write", "deny", "remove_deny"} {
		validateProposalStringListField(access, key, field+"."+key, problems)
	}
}

func validateProposalBudget(value any, field string, problems *[]string) {
	if value == nil {
		return
	}
	budget, ok := mapDeclaration(value)
	if !ok {
		*problems = append(*problems, field+" must be an object")
		return
	}
	appendUnknownProposalFields(budget, field, []string{"max_total_tokens", "max_task_tokens", "max_tasks"}, problems)
	for _, key := range []string{"max_total_tokens", "max_task_tokens", "max_tasks"} {
		raw, present := budget[key]
		if !present {
			continue
		}
		if number, ok := proposalInteger(raw); !ok || number < 0 {
			*problems = append(*problems, field+"."+key+" must be a non-negative integer")
		}
	}
}

func validateProposalRequest(value any, field string, problems *[]string) {
	if value == nil {
		return
	}
	request, ok := mapDeclaration(value)
	if !ok {
		*problems = append(*problems, field+" must be an object")
		return
	}
	appendUnknownProposalFields(request, field, []string{"text"}, problems)
	validateProposalOptionalString(request, "text", field+".text", problems)
}

func validateProposalPlan(value any, field string, problems *[]string) {
	if value == nil {
		return
	}
	plan, ok := mapDeclaration(value)
	if !ok {
		*problems = append(*problems, field+" must be an object")
		return
	}
	appendUnknownProposalFields(plan, field, []string{"goal", "steps", "constraints"}, problems)
	validateProposalOptionalString(plan, "goal", field+".goal", problems)
	validateProposalStringListField(plan, "steps", field+".steps", problems)
	validateProposalStringListField(plan, "constraints", field+".constraints", problems)
}

func validateProposalRoles(value any, field string, problems *[]string) {
	if value == nil {
		return
	}
	roles, ok := mapDeclaration(value)
	if !ok {
		*problems = append(*problems, field+" must be an object")
		return
	}
	ids := make([]string, 0, len(roles))
	for id := range roles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		definition, ok := mapDeclaration(roles[id])
		roleField := fmt.Sprintf("%s[%q]", field, id)
		if strings.TrimSpace(id) == "" {
			*problems = append(*problems, field+" role id must not be empty")
		}
		if !ok {
			*problems = append(*problems, roleField+" must be an object")
			continue
		}
		appendUnknownProposalFields(definition, roleField, []string{
			"role", "worker", "worker_type", "work_mode", "worker_policy", "authority", "constraints", "accessible_paths", "access", "model", "model_policy", "max_cloud_tokens", "cloud_budget", "require_approval", "publish", "sync",
		}, problems)
		for _, key := range []string{"role", "worker", "worker_type", "work_mode"} {
			validateProposalOptionalString(definition, key, roleField+"."+key, problems)
		}
		for _, key := range []string{"worker_policy", "authority", "model", "model_policy"} {
			if raw, present := definition[key]; present {
				if _, ok := mapDeclaration(raw); !ok {
					*problems = append(*problems, roleField+"."+key+" must be an object")
				}
			}
		}
		validateProposalStringListField(definition, "constraints", roleField+".constraints", problems)
		validateProposalStringListField(definition, "accessible_paths", roleField+".accessible_paths", problems)
		validateProposalAccess(definition["access"], roleField+".access", problems)
		validateProposalBudget(definition["cloud_budget"], roleField+".cloud_budget", problems)
		validateProposalOptionalBool(definition, "require_approval", roleField+".require_approval", problems)
		validateProposalOptionalBool(definition, "publish", roleField+".publish", problems)
		validateProposalOptionalBool(definition, "sync", roleField+".sync", problems)
		if raw, present := definition["max_cloud_tokens"]; present {
			if number, ok := proposalInteger(raw); !ok || number < 0 {
				*problems = append(*problems, roleField+".max_cloud_tokens must be a non-negative integer")
			}
		}
	}
}

func validateProposalTasks(value any, field string, problems *[]string) {
	tasks, ok := proposalArray(value)
	if !ok {
		*problems = append(*problems, field+" must be an array")
		return
	}
	if len(tasks) == 0 {
		*problems = append(*problems, field+" must contain at least one task")
		return
	}
	for index, raw := range tasks {
		taskField := fmt.Sprintf("%s[%d]", field, index)
		task, ok := mapDeclaration(raw)
		if !ok {
			*problems = append(*problems, taskField+" must be an object")
			continue
		}
		appendUnknownProposalFields(task, taskField, []string{
			"id", "agent", "goal", "brief", "context", "expected_output", "prompt", "constraints", "depends_on", "claims", "citations", "max_cloud_tokens", "materialize_outputs",
		}, problems)
		validateProposalRequiredString(task, "id", taskField+".id", problems)
		for _, key := range []string{"agent", "goal", "brief", "expected_output", "prompt"} {
			validateProposalOptionalString(task, key, taskField+"."+key, problems)
		}
		for _, key := range []string{"constraints", "depends_on", "claims", "citations"} {
			validateProposalStringListField(task, key, taskField+"."+key, problems)
		}
		validateProposalTaskContext(task["context"], taskField+".context", problems)
		validateProposalMaterializeOutputs(task["materialize_outputs"], taskField+".materialize_outputs", problems)
		if raw, present := task["max_cloud_tokens"]; present {
			if number, ok := proposalInteger(raw); !ok || number < 0 {
				*problems = append(*problems, taskField+".max_cloud_tokens must be a non-negative integer")
			}
		}
	}
}

func validateProposalTaskContext(value any, field string, problems *[]string) {
	if value == nil {
		return
	}
	context, ok := mapDeclaration(value)
	if !ok {
		*problems = append(*problems, field+" must be an object")
		return
	}
	appendUnknownProposalFields(context, field, []string{"required_artifacts", "seed_paths", "source_roots"}, problems)
	for _, key := range []string{"required_artifacts", "seed_paths", "source_roots"} {
		validateProposalStringListField(context, key, field+"."+key, problems)
	}
}

func validateProposalMaterializeOutputs(value any, field string, problems *[]string) {
	if value == nil {
		return
	}
	outputs, ok := proposalArray(value)
	if !ok {
		*problems = append(*problems, field+" must be an array")
		return
	}
	for index, raw := range outputs {
		outputField := fmt.Sprintf("%s[%d]", field, index)
		output, ok := mapDeclaration(raw)
		if !ok {
			*problems = append(*problems, outputField+" must be an object")
			continue
		}
		appendUnknownProposalFields(output, outputField, []string{"id", "path"}, problems)
		validateProposalOptionalString(output, "id", outputField+".id", problems)
		validateProposalRequiredString(output, "path", outputField+".path", problems)
	}
}

func validateProposalScalarMap(value any, field string, problems *[]string) {
	if value == nil {
		return
	}
	data, ok := mapDeclaration(value)
	if !ok {
		*problems = append(*problems, field+" must be an object")
		return
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !isContractScalar(data[key]) {
			*problems = append(*problems, field+"."+key+" must be a scalar")
		}
	}
}

func validateProposalApply(value any, field string, problems *[]string) {
	if value == nil {
		return
	}
	apply, ok := mapDeclaration(value)
	if !ok {
		*problems = append(*problems, field+" must be an object")
		return
	}
	appendUnknownProposalFields(apply, field, []string{"require_approval", "target_root", "required_artifacts"}, problems)
	validateProposalOptionalBool(apply, "require_approval", field+".require_approval", problems)
	validateProposalOptionalString(apply, "target_root", field+".target_root", problems)
	validateProposalStringListField(apply, "required_artifacts", field+".required_artifacts", problems)
}

// compileExecutableContract is the authority boundary between parsed planner data and runnable
// artifacts. It constructs no partial graph or task artifact until normalization fully succeeds.
