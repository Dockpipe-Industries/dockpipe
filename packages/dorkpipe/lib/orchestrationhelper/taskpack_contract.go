package orchestrationhelper

import (
	"encoding/json"
	"errors"
	"fmt"
	pathpkg "path"
	"sort"
	"strings"
)

type compiledExecutableContract struct {
	Plan             map[string]any `json:"plan"`
	TaskGraph        map[string]any `json:"task_graph"`
	TaskArtifacts    []any          `json:"task_artifacts"`
	ProposalMetadata map[string]any `json:"proposal_metadata"`
}

func compileExecutableContract(packageDefaults, repoTaskPack map[string]any, proposal *parsedPlannerProposal) (*compiledExecutableContract, error) {
	var proposalDeclaration map[string]any
	if proposal != nil {
		proposalDeclaration = proposal.Declaration
	}
	normalized, err := normalizeTaskPackContract(packageDefaults, repoTaskPack, proposalDeclaration)
	if err != nil {
		return nil, err
	}

	rolesByID := map[string]map[string]any{}
	for _, rawRole := range listValue(normalized["roles"]) {
		role := mapValue(rawRole)
		rolesByID[stringValue(role["id"])] = role
	}
	taskArtifacts := make([]any, 0, len(listValue(normalized["tasks"])))
	graphTasks := make([]any, 0, len(listValue(normalized["tasks"])))
	for _, rawTask := range listValue(normalized["tasks"]) {
		task := mapValue(rawTask)
		artifact := map[string]any{}
		for _, key := range []string{"id", "agent", "goal", "brief", "context", "constraints", "expected_output", "prompt", "claims", "citations", "max_cloud_tokens", "depends_on", "materialize_outputs"} {
			if value, present := task[key]; present {
				artifact[key] = value
			}
		}
		role := rolesByID[stringValue(task["agent"])]
		for _, key := range []string{"role", "worker", "worker_type", "work_mode", "worker_policy", "model", "model_policy", "accessible_paths", "authority"} {
			if value, present := role[key]; present {
				artifact[key] = value
			}
		}
		roleConstraints := stringList(role["constraints"])
		artifact["constraints"] = anySlice(appendStableContractStrings(roleConstraints, stringList(task["constraints"])...))
		effectiveAccess := mapValue(normalized["access"])
		if len(role) > 0 {
			effectiveAccess = mapValue(role["access"])
		}
		if taskAccess, present := mapDeclaration(task["access"]); present {
			effectiveAccess = intersectContractAccess(effectiveAccess, taskAccess)
		}
		artifact["access"] = effectiveAccess
		if _, present := artifact["max_cloud_tokens"]; !present {
			if roleBudget := intFromAny(role["max_cloud_tokens"]); roleBudget > 0 {
				artifact["max_cloud_tokens"] = roleBudget
			} else if runBudget := intFromAny(mapValue(normalized["cloud_budget"])["max_task_tokens"]); runBudget > 0 {
				artifact["max_cloud_tokens"] = runBudget
			}
		}
		if stringValue(artifact["role"]) == "" && stringValue(artifact["agent"]) != "" {
			artifact["role"] = artifact["agent"]
		}
		if len(mapValue(artifact["context"])) == 0 {
			artifact["context"] = map[string]any{}
		}
		worker := fallbackString(strings.TrimSpace(stringValue(artifact["worker"])), "auto")
		model := mapValue(artifact["model"])
		lane := map[string]any{
			"lane_id":       "compiled." + worker,
			"provider":      worker,
			"resolver_hint": worker,
			"model":         stringValue(model["model"]),
			"available":     true,
		}
		artifact["base_id"] = artifact["id"]
		artifact["comparison"] = map[string]any{"enabled": false}
		artifact["resolver_hint"] = worker
		artifact["requested_resolver_hint"] = worker
		artifact["lane"] = lane
		artifact["output_path"] = inferTaskOutputPath(artifact)
		artifact["context_paths"] = anySlice(stringList(mapValue(artifact["context"])["required_artifacts"]))
		artifact["startup_prompt"] = normalized["startup_prompt"]
		artifact["include_agents_md"] = normalized["include_agents_md"]
		taskArtifacts = append(taskArtifacts, artifact)
		graphTasks = append(graphTasks, map[string]any{
			"id":                  artifact["id"],
			"base_task_id":        artifact["id"],
			"agent":               artifact["agent"],
			"depends_on":          artifact["depends_on"],
			"resolver_hint":       artifact["resolver_hint"],
			"lane_id":             lane["lane_id"],
			"provider":            lane["provider"],
			"model":               lane["model"],
			"worker_type":         fallbackString(stringValue(artifact["worker_type"]), "analysis"),
			"output_path":         artifact["output_path"],
			"materialize_outputs": artifact["materialize_outputs"],
		})
	}

	planCfg := mapValue(normalized["plan"])
	concurrency := mapValue(contractOrchestration(packageDefaults)["concurrency"])
	plan := map[string]any{
		"goal":              stringValue(planCfg["goal"]),
		"request":           mapValue(normalized["request"]),
		"plan":              planCfg,
		"merge":             mapValue(normalized["merge"]),
		"verify":            mapValue(normalized["verify"]),
		"startup_prompt":    normalized["startup_prompt"],
		"include_agents_md": normalized["include_agents_md"],
		"constraints":       normalized["constraints"],
		"required_outputs":  normalized["required_outputs"],
		"access":            normalized["access"],
		"cloud_budget":      normalized["cloud_budget"],
		"roles":             normalized["roles"],
		"concurrency":       concurrency,
		"approval_required": normalized["approval_required"],
		"publish":           normalized["publish"],
		"sync":              normalized["sync"],
		"apply": map[string]any{
			"require_approval":   normalized["approval_required"],
			"target_root":        normalized["apply_target"],
			"required_artifacts": normalized["required_outputs"],
		},
	}
	return &compiledExecutableContract{
		Plan:             plan,
		TaskGraph:        map[string]any{"concurrency": concurrency, "tasks": graphTasks},
		TaskArtifacts:    taskArtifacts,
		ProposalMetadata: normalizedProposalMetadata(proposal, normalized),
	}, nil
}

func normalizedProposalMetadata(proposal *parsedPlannerProposal, normalized map[string]any) map[string]any {
	metadata := map[string]any{
		"present":             false,
		"source_format":       "none",
		"selected_graph":      false,
		"role_ids":            []any{},
		"constraints":         []any{},
		"task_ids":            []any{},
		"dependencies":        []any{},
		"materialize_outputs": []any{},
	}
	if proposal == nil {
		return metadata
	}
	metadata["present"] = true
	metadata["source_format"] = proposal.Format
	metadata["selected_graph"] = true
	roleIDs := []string{}
	for id := range mapValue(contractOrchestration(proposal.Declaration)["agents"]) {
		roleIDs = append(roleIDs, id)
	}
	sort.Strings(roleIDs)
	metadata["role_ids"] = anySlice(roleIDs)
	metadata["constraints"] = anySlice(contractConstraints(proposal.Declaration))
	taskIDs := []string{}
	dependencies := []any{}
	outputs := []any{}
	for _, rawTask := range listValue(normalized["tasks"]) {
		task := mapValue(rawTask)
		id := stringValue(task["id"])
		taskIDs = append(taskIDs, id)
		dependencies = append(dependencies, map[string]any{"task_id": id, "depends_on": task["depends_on"]})
		for _, rawOutput := range listValue(task["materialize_outputs"]) {
			outputs = append(outputs, map[string]any{"task_id": id, "path": stringValue(mapValue(rawOutput)["path"])})
		}
	}
	metadata["task_ids"] = anySlice(taskIDs)
	metadata["dependencies"] = dependencies
	metadata["materialize_outputs"] = outputs
	return metadata
}

// normalizeTaskPackContract compiles package defaults, one loaded repository task-pack agent
// declaration, and an optional per-run planner proposal into a deterministic, data-only contract.
// It deliberately does not materialize artifacts or execute tasks; the two-phase compiler owns
// those later boundaries.
func normalizeTaskPackContract(packageDefaults, repoTaskPack, proposal map[string]any) (map[string]any, error) {
	layers := []struct {
		name string
		data map[string]any
	}{
		{name: "package", data: packageDefaults},
		{name: "repo", data: repoTaskPack},
		{name: "proposal", data: proposal},
	}
	problems := []string{}

	packageAccess := normalizedContractAccess(contractAccess(packageDefaults), "package.access", &problems)
	effectiveAccess := copyMap(packageAccess)
	for _, layer := range layers[1:] {
		if len(layer.data) == 0 {
			continue
		}
		effectiveAccess = narrowContractAccess(effectiveAccess, contractAccess(layer.data), layer.name, &problems)
	}

	packageBudget := normalizedContractBudget(contractBudget(packageDefaults), "package.cloud_budget", &problems)
	effectiveBudget := copyMap(packageBudget)
	for _, layer := range layers[1:] {
		if len(layer.data) == 0 {
			continue
		}
		effectiveBudget = narrowContractBudget(effectiveBudget, contractBudget(layer.data), layer.name, &problems)
	}

	for _, layer := range layers[1:] {
		if len(layer.data) == 0 {
			continue
		}
		if contractBool(layer.data, "publish") {
			problems = append(problems, layer.name+".publish cannot enable package-owned publish")
		}
		if contractBool(layer.data, "sync") {
			problems = append(problems, layer.name+".sync cannot enable package-owned sync")
		}
		if value, ok := contractApply(layer.data)["require_approval"]; ok && !boolAny(value) {
			problems = append(problems, layer.name+".apply.require_approval cannot disable mandatory package approval")
		}
	}

	soft := map[string]any{}
	for _, key := range []string{"startup_prompt", "include_agents_md"} {
		for _, layer := range layers {
			if value, ok := layer.data[key]; ok && isContractScalar(value) {
				soft[key] = value
			}
		}
	}
	for _, key := range []string{"request", "plan", "merge", "verify"} {
		merged := map[string]any{}
		for _, layer := range layers {
			mergeContractScalars(merged, mapValue(contractOrchestration(layer.data)[key]))
		}
		soft[key] = merged
	}

	constraints := []string{}
	for _, layer := range layers {
		constraints = appendStableContractStrings(constraints, contractConstraints(layer.data)...)
	}

	baseFloor := []string{}
	for _, layer := range layers[:2] {
		for _, rawPath := range stringList(contractApply(layer.data)["required_artifacts"]) {
			path, err := normalizeContractOutputPath(rawPath)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s.apply.required_artifacts output path %q is invalid: %v", layer.name, rawPath, err))
				continue
			}
			baseFloor = appendStableContractStrings(baseFloor, path)
		}
	}
	requiredOutputs := append([]string{}, baseFloor...)
	proposalApply := contractApply(proposal)
	if rawProposalFloor, present := proposalApply["required_artifacts"]; present {
		proposalFloor := []string{}
		for _, rawPath := range stringList(rawProposalFloor) {
			path, err := normalizeContractOutputPath(rawPath)
			if err != nil {
				problems = append(problems, fmt.Sprintf("proposal.apply.required_artifacts output path %q is invalid: %v", rawPath, err))
				continue
			}
			proposalFloor = appendStableContractStrings(proposalFloor, path)
		}
		for _, floor := range baseFloor {
			if !containsString(proposalFloor, floor) {
				problems = append(problems, fmt.Sprintf("proposal.apply.required_artifacts cannot remove or rename required output %q", floor))
			}
		}
		requiredOutputs = appendStableContractStrings(requiredOutputs, proposalFloor...)
	}

	applyTarget := strings.TrimSpace(stringValue(contractApply(packageDefaults)["target_root"]))
	if repoTarget := strings.TrimSpace(stringValue(contractApply(repoTaskPack)["target_root"])); repoTarget != "" {
		applyTarget = repoTarget
	}
	if proposalTarget := strings.TrimSpace(stringValue(proposalApply["target_root"])); proposalTarget != "" && proposalTarget != applyTarget {
		problems = append(problems, fmt.Sprintf("proposal.apply.target_root %q cannot change repo-selected target %q", proposalTarget, applyTarget))
	}

	roles := normalizeContractRoles(packageDefaults, repoTaskPack, proposal, packageAccess, effectiveAccess, effectiveBudget, &problems)
	taskLayerName, rawTasks := selectContractTasks(packageDefaults, repoTaskPack, proposal)
	tasks := normalizeContractTasks(rawTasks, taskLayerName, effectiveAccess, effectiveBudget, &problems)
	if maxTasks := intFromAny(effectiveBudget["max_tasks"]); maxTasks > 0 && len(tasks) > maxTasks {
		problems = append(problems, fmt.Sprintf("%s.tasks count %d exceeds cloud_budget.max_tasks ceiling %d", taskLayerName, len(tasks), maxTasks))
	}
	validateContractTaskRoles(tasks, roles, taskLayerName, &problems)
	validateContractGraph(tasks, requiredOutputs, &problems)

	if len(problems) > 0 {
		return nil, fmt.Errorf("invalid normalized orchestration contract: %s", strings.Join(problems, "; "))
	}

	return map[string]any{
		"startup_prompt":    soft["startup_prompt"],
		"include_agents_md": soft["include_agents_md"],
		"request":           soft["request"],
		"plan":              soft["plan"],
		"merge":             soft["merge"],
		"verify":            soft["verify"],
		"access":            effectiveAccess,
		"cloud_budget":      effectiveBudget,
		"constraints":       anySlice(constraints),
		"required_outputs":  anySlice(requiredOutputs),
		"apply_target":      applyTarget,
		"approval_required": true,
		"publish":           false,
		"sync":              false,
		"roles":             roles,
		"tasks":             tasks,
	}, nil
}

func contractOrchestration(layer map[string]any) map[string]any {
	return mapValue(layer["orchestration"])
}

func contractAccess(layer map[string]any) map[string]any {
	if access, ok := mapDeclaration(layer["access"]); ok {
		return access
	}
	return mapValue(contractOrchestration(layer)["access"])
}

func contractBudget(layer map[string]any) map[string]any {
	if budget, ok := mapDeclaration(layer["cloud_budget"]); ok {
		return budget
	}
	return mapValue(contractOrchestration(layer)["cloud_budget"])
}

func contractApply(layer map[string]any) map[string]any {
	return mapValue(contractOrchestration(layer)["apply"])
}

func contractBool(layer map[string]any, key string) bool {
	if value, ok := layer[key]; ok {
		return boolAny(value)
	}
	return boolAny(contractOrchestration(layer)[key])
}

func contractConstraints(layer map[string]any) []string {
	out := []string{}
	out = appendStableContractStrings(out, stringList(layer["constraints"])...)
	orchestration := contractOrchestration(layer)
	out = appendStableContractStrings(out, stringList(orchestration["constraints"])...)
	out = appendStableContractStrings(out, stringList(mapValue(orchestration["plan"])["constraints"])...)
	return out
}

func isContractScalar(value any) bool {
	switch value.(type) {
	case nil, string, bool, int, int64, float64, json.Number:
		return true
	default:
		return false
	}
}

func mergeContractScalars(target, source map[string]any) {
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if key == "constraints" || !isContractScalar(source[key]) {
			continue
		}
		target[key] = source[key]
	}
}

func appendStableContractStrings(values []string, additions ...string) []string {
	for _, addition := range additions {
		addition = strings.TrimSpace(addition)
		if addition != "" && !containsString(values, addition) {
			values = append(values, addition)
		}
	}
	return values
}

func normalizedContractAccess(access map[string]any, field string, problems *[]string) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"read", "write", "deny"} {
		paths := []string{}
		for _, rawPath := range stringList(access[key]) {
			path, err := normalizeContractAuthorityPath(rawPath)
			if err != nil {
				*problems = append(*problems, fmt.Sprintf("%s.%s path %q is invalid: %v", field, key, rawPath, err))
				continue
			}
			paths = appendStableContractStrings(paths, path)
		}
		out[key] = anySlice(paths)
	}
	return out
}

func narrowContractAccess(current, requested map[string]any, layer string, problems *[]string) map[string]any {
	out := copyMap(current)
	if removed := stringList(requested["remove_deny"]); len(removed) > 0 {
		for _, path := range removed {
			*problems = append(*problems, fmt.Sprintf("%s.access.remove_deny cannot remove package deny rule %q", layer, path))
		}
	}
	for _, key := range []string{"read", "write"} {
		raw, present := requested[key]
		if !present {
			continue
		}
		ceiling := stringList(current[key])
		narrowed := []string{}
		for _, rawPath := range stringList(raw) {
			path, err := normalizeContractAuthorityPath(rawPath)
			if err != nil {
				*problems = append(*problems, fmt.Sprintf("%s.access.%s path %q is invalid: %v", layer, key, rawPath, err))
				continue
			}
			if !contractPathWithinAny(path, ceiling) {
				*problems = append(*problems, fmt.Sprintf("%s.access.%s path %q is outside the current package authority ceiling", layer, key, path))
				continue
			}
			narrowed = appendStableContractStrings(narrowed, path)
		}
		out[key] = anySlice(narrowed)
	}
	denied := stringList(current["deny"])
	for _, rawPath := range stringList(requested["deny"]) {
		path, err := normalizeContractAuthorityPath(rawPath)
		if err != nil {
			*problems = append(*problems, fmt.Sprintf("%s.access.deny path %q is invalid: %v", layer, rawPath, err))
			continue
		}
		denied = appendStableContractStrings(denied, path)
	}
	out["deny"] = anySlice(denied)
	return out
}

func normalizeContractAuthorityPath(raw string) (string, error) {
	value := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if value == "" {
		return "", errors.New("path is empty")
	}
	cleaned := pathpkg.Clean(value)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", errors.New("path escapes its authority root")
	}
	return cleaned, nil
}

func contractPathWithinAny(candidate string, ceilings []string) bool {
	for _, ceiling := range ceilings {
		if ceiling == "." && candidate != ".." && !strings.HasPrefix(candidate, "../") && !strings.HasPrefix(candidate, "/") {
			return true
		}
		if candidate == ceiling || ceiling == "/" || strings.HasPrefix(candidate, strings.TrimSuffix(ceiling, "/")+"/") {
			return true
		}
	}
	return false
}

func intersectContractAccess(packageRoleAccess, runAccess map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"read", "write"} {
		paths := []string{}
		for _, runPath := range stringList(runAccess[key]) {
			for _, rolePath := range stringList(packageRoleAccess[key]) {
				switch {
				case contractPathWithinAny(runPath, []string{rolePath}):
					paths = appendStableContractStrings(paths, runPath)
				case contractPathWithinAny(rolePath, []string{runPath}):
					paths = appendStableContractStrings(paths, rolePath)
				}
			}
		}
		out[key] = anySlice(paths)
	}
	denied := appendStableContractStrings(nil, stringList(packageRoleAccess["deny"])...)
	denied = appendStableContractStrings(denied, stringList(runAccess["deny"])...)
	out["deny"] = anySlice(denied)
	return out
}

func normalizedContractBudget(budget map[string]any, field string, problems *[]string) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"max_total_tokens", "max_task_tokens", "max_tasks"} {
		if raw, ok := budget[key]; ok {
			value := intFromAny(raw)
			if value < 0 {
				*problems = append(*problems, fmt.Sprintf("%s.%s must be non-negative", field, key))
				continue
			}
			out[key] = value
		}
	}
	return out
}

func narrowContractBudget(current, requested map[string]any, layer string, problems *[]string) map[string]any {
	out := copyMap(current)
	for _, key := range []string{"max_total_tokens", "max_task_tokens", "max_tasks"} {
		raw, present := requested[key]
		if !present {
			continue
		}
		value := intFromAny(raw)
		ceilingRaw, governed := current[key]
		ceiling := intFromAny(ceilingRaw)
		if value < 0 {
			*problems = append(*problems, fmt.Sprintf("%s.cloud_budget.%s must be non-negative", layer, key))
			continue
		}
		if !governed || value > ceiling {
			*problems = append(*problems, fmt.Sprintf("%s.cloud_budget.%s value %d exceeds package ceiling %d", layer, key, value, ceiling))
			continue
		}
		out[key] = value
	}
	return out
}

func normalizeContractRoles(packageDefaults, repoTaskPack, proposal map[string]any, packageAccess, runAccess, budget map[string]any, problems *[]string) []any {
	layers := []struct {
		name  string
		roles map[string]any
	}{
		{name: "package", roles: mapValue(contractOrchestration(packageDefaults)["agents"])},
		{name: "repo", roles: mapValue(contractOrchestration(repoTaskPack)["agents"])},
		{name: "proposal", roles: mapValue(contractOrchestration(proposal)["agents"])},
	}
	ids := []string{}
	for _, layer := range layers {
		for id := range layer.roles {
			ids = appendStableContractStrings(ids, id)
		}
	}
	sort.Strings(ids)
	out := make([]any, 0, len(ids))
	hardKeys := map[string]bool{"access": true, "authority": true, "max_cloud_tokens": true, "cloud_budget": true, "require_approval": true, "publish": true, "sync": true}
	for _, id := range ids {
		role := map[string]any{"id": id}
		constraints := []string{}
		packageRole := mapValue(layers[0].roles[id])
		roleAccess := copyMap(packageAccess)
		if access, ok := mapDeclaration(packageRole["access"]); ok {
			roleAccess = narrowContractAccess(roleAccess, access, fmt.Sprintf("package.roles[%q]", id), problems)
		}
		roleAccess = intersectContractAccess(roleAccess, runAccess)
		roleBudget := intFromAny(budget["max_task_tokens"])
		if raw, ok := packageRole["max_cloud_tokens"]; ok && intFromAny(raw) < roleBudget {
			roleBudget = intFromAny(raw)
		}
		for _, layer := range layers {
			definition := mapValue(layer.roles[id])
			if len(definition) == 0 {
				continue
			}
			keys := make([]string, 0, len(definition))
			for key := range definition {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				if key == "constraints" || hardKeys[key] {
					continue
				}
				role[key] = definition[key]
			}
			constraints = appendStableContractStrings(constraints, stringList(definition["constraints"])...)
			if layer.name == "package" {
				if authority, ok := definition["authority"]; ok {
					role["authority"] = authority
				}
				continue
			}
			if access, ok := mapDeclaration(definition["access"]); ok {
				roleAccess = narrowContractAccess(roleAccess, access, fmt.Sprintf("%s.roles[%q]", layer.name, id), problems)
			}
			if raw, ok := definition["max_cloud_tokens"]; ok {
				value := intFromAny(raw)
				if value > roleBudget {
					*problems = append(*problems, fmt.Sprintf("%s.roles[%q].max_cloud_tokens value %d exceeds package ceiling %d", layer.name, id, value, roleBudget))
				} else {
					roleBudget = value
				}
			}
			if authority, ok := definition["authority"]; ok && mustJSON(authority, nil) != mustJSON(role["authority"], nil) {
				*problems = append(*problems, fmt.Sprintf("%s.roles[%q].authority cannot replace package authority", layer.name, id))
			}
			for _, key := range []string{"require_approval", "publish", "sync"} {
				if boolAny(definition[key]) || (key == "require_approval" && definition[key] != nil && !boolAny(definition[key])) {
					*problems = append(*problems, fmt.Sprintf("%s.roles[%q].%s is package-owned authority", layer.name, id, key))
				}
			}
		}
		role["constraints"] = anySlice(constraints)
		role["access"] = roleAccess
		if roleBudget > 0 {
			role["max_cloud_tokens"] = roleBudget
		}
		out = append(out, role)
	}
	return out
}

func selectContractTasks(packageDefaults, repoTaskPack, proposal map[string]any) (string, []any) {
	if tasks, present := contractOrchestration(proposal)["tasks"]; present {
		return "proposal", listValue(tasks)
	}
	if tasks, present := contractOrchestration(repoTaskPack)["tasks"]; present {
		return "repo", listValue(tasks)
	}
	return "package", listValue(contractOrchestration(packageDefaults)["tasks"])
}

func normalizeContractTasks(rawTasks []any, layer string, access, budget map[string]any, problems *[]string) []any {
	out := make([]any, 0, len(rawTasks))
	maxTaskTokens := intFromAny(budget["max_task_tokens"])
	for index, raw := range rawTasks {
		source := mapValue(raw)
		id := strings.TrimSpace(stringValue(source["id"]))
		field := fmt.Sprintf("%s.tasks[%d]", layer, index)
		if id != "" {
			field = fmt.Sprintf("%s.tasks[%q]", layer, id)
		}
		task := copyMap(source)
		task["id"] = id
		dependsOn := appendStableContractStrings(nil, stringList(source["depends_on"])...)
		task["depends_on"] = anySlice(dependsOn)
		constraints := appendStableContractStrings(nil, stringList(source["constraints"])...)
		task["constraints"] = anySlice(constraints)

		outputs := []any{}
		for _, rawOutput := range listValue(source["materialize_outputs"]) {
			output := mapValue(rawOutput)
			rawPath := stringValue(output["path"])
			if len(output) == 0 {
				rawPath = stringValue(rawOutput)
				output = map[string]any{}
			}
			path, err := normalizeContractOutputPath(rawPath)
			if err != nil {
				*problems = append(*problems, fmt.Sprintf("%s.materialize_outputs output path %q is invalid: %v", field, rawPath, err))
				continue
			}
			output = copyMap(output)
			output["path"] = path
			outputs = append(outputs, output)
		}
		task["materialize_outputs"] = outputs

		if taskAccess, ok := mapDeclaration(source["access"]); ok {
			task["access"] = narrowContractAccess(access, taskAccess, field, problems)
		}
		if raw, present := source["max_cloud_tokens"]; present {
			value := intFromAny(raw)
			if maxTaskTokens == 0 || value > maxTaskTokens {
				*problems = append(*problems, fmt.Sprintf("%s.max_cloud_tokens value %d exceeds package ceiling %d", field, value, maxTaskTokens))
			}
		}
		if contractBool(source, "publish") {
			*problems = append(*problems, field+".publish cannot enable package-owned publish")
		}
		if contractBool(source, "sync") {
			*problems = append(*problems, field+".sync cannot enable package-owned sync")
		}
		out = append(out, task)
	}
	return out
}

func validateContractTaskRoles(tasks, roles []any, layer string, problems *[]string) {
	rolesByID := map[string]map[string]any{}
	for _, rawRole := range roles {
		role := mapValue(rawRole)
		rolesByID[stringValue(role["id"])] = role
	}
	for _, rawTask := range tasks {
		task := mapValue(rawTask)
		agentID := strings.TrimSpace(stringValue(task["agent"]))
		if agentID == "" {
			continue
		}
		role, exists := rolesByID[agentID]
		if !exists {
			*problems = append(*problems, fmt.Sprintf("%s.tasks[%q].agent references unknown normalized role %q", layer, stringValue(task["id"]), agentID))
			continue
		}
		taskBudget := intFromAny(task["max_cloud_tokens"])
		roleBudget := intFromAny(role["max_cloud_tokens"])
		if taskBudget > 0 && roleBudget > 0 && taskBudget > roleBudget {
			*problems = append(*problems, fmt.Sprintf("%s.tasks[%q].max_cloud_tokens value %d exceeds normalized role %q ceiling %d", layer, stringValue(task["id"]), taskBudget, agentID, roleBudget))
		}
	}
}

func normalizeContractOutputPath(raw string) (string, error) {
	value := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if value == "" {
		return "", errors.New("path is empty")
	}
	if strings.HasPrefix(value, "/") || strings.Contains(value, ":") {
		return "", errors.New("path must be relative")
	}
	cleaned := pathpkg.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", errors.New("path escapes the materialized output root")
	}
	return cleaned, nil
}

func validateContractGraph(tasks []any, requiredOutputs []string, problems *[]string) {
	firstTask := map[string]int{}
	orderedIDs := []string{}
	for index, raw := range tasks {
		task := mapValue(raw)
		id := stringValue(task["id"])
		if id == "" {
			*problems = append(*problems, fmt.Sprintf("tasks[%d].id is required", index))
			continue
		}
		if first, duplicate := firstTask[id]; duplicate {
			*problems = append(*problems, fmt.Sprintf("tasks[%q].id is duplicate (first declared at index %d)", id, first))
			continue
		}
		firstTask[id] = index
		orderedIDs = append(orderedIDs, id)
	}
	for _, raw := range tasks {
		task := mapValue(raw)
		id := stringValue(task["id"])
		for _, dependency := range stringList(task["depends_on"]) {
			if _, ok := firstTask[dependency]; !ok {
				*problems = append(*problems, fmt.Sprintf("tasks[%q].depends_on dependency %q does not exist", id, dependency))
			}
		}
	}

	state := map[string]int{}
	stack := []string{}
	cycleSeen := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		state[id] = 1
		stack = append(stack, id)
		task := mapValue(tasks[firstTask[id]])
		for _, dependency := range stringList(task["depends_on"]) {
			if _, exists := firstTask[dependency]; !exists {
				continue
			}
			if state[dependency] == 0 {
				visit(dependency)
				continue
			}
			if state[dependency] == 1 {
				start := 0
				for stack[start] != dependency {
					start++
				}
				cycle := append(append([]string{}, stack[start:]...), dependency)
				signature := strings.Join(cycle, " -> ")
				if !cycleSeen[signature] {
					cycleSeen[signature] = true
					*problems = append(*problems, fmt.Sprintf("tasks[%q].depends_on dependency %q creates cycle %s", id, dependency, signature))
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = 2
	}
	for _, id := range orderedIDs {
		if state[id] == 0 {
			visit(id)
		}
	}

	producers := map[string]string{}
	for _, raw := range tasks {
		task := mapValue(raw)
		id := stringValue(task["id"])
		for _, rawOutput := range listValue(task["materialize_outputs"]) {
			path := stringValue(mapValue(rawOutput)["path"])
			if first, exists := producers[path]; exists {
				*problems = append(*problems, fmt.Sprintf("materialized output %q has duplicate producers tasks[%q] and tasks[%q]", path, first, id))
				continue
			}
			producers[path] = id
		}
	}
	for _, path := range requiredOutputs {
		if _, ok := producers[path]; !ok {
			*problems = append(*problems, fmt.Sprintf("required output floor %q has no producer", path))
		}
	}
}
