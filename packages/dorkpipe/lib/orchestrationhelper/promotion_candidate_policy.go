package orchestrationhelper

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

func validatePromotionMetadata(proposal *parsedPlannerProposal, metadata map[string]any, identity softwareDevPromotionIdentity) error {
	if !boolAny(metadata["present"]) || !boolAny(metadata["selected_graph"]) {
		return errors.New("promotion metadata does not identify a successfully selected proposal graph")
	}
	if stringValue(metadata["source_format"]) != proposal.Format {
		return errors.New("promotion metadata source format does not match the raw proposal")
	}
	taskPack := mapValue(metadata["task_pack"])
	if stringValue(taskPack["path"]) != identity.TaskPackPath || stringValue(taskPack["step_id"]) != identity.StepID {
		return errors.New("promotion metadata task-pack identity does not match the selected repo surface")
	}

	orchestration := contractOrchestration(proposal.Declaration)
	roleIDs := []string{}
	for id := range mapValue(orchestration["agents"]) {
		roleIDs = append(roleIDs, id)
	}
	sort.Strings(roleIDs)
	taskIDs := []string{}
	dependencies := []any{}
	outputs := []any{}
	for _, rawTask := range listValue(orchestration["tasks"]) {
		task := mapValue(rawTask)
		id := stringValue(task["id"])
		taskIDs = append(taskIDs, id)
		dependencies = append(dependencies, map[string]any{"task_id": id, "depends_on": anySlice(stringList(task["depends_on"]))})
		for _, rawOutput := range listValue(task["materialize_outputs"]) {
			outputs = append(outputs, map[string]any{"task_id": id, "path": stringValue(mapValue(rawOutput)["path"])})
		}
	}
	expected := map[string]any{
		"role_ids":            anySlice(roleIDs),
		"constraints":         anySlice(contractConstraints(proposal.Declaration)),
		"task_ids":            anySlice(taskIDs),
		"dependencies":        dependencies,
		"materialize_outputs": outputs,
	}
	for _, key := range []string{"role_ids", "constraints", "task_ids", "dependencies", "materialize_outputs"} {
		if mustJSON(metadata[key], []any{}) != mustJSON(expected[key], []any{}) {
			return fmt.Errorf("promotion metadata %s does not match the selected proposal graph", key)
		}
	}
	return nil
}

func validatePromotionVerificationArtifact(verification map[string]any) error {
	for _, key := range []string{"status", "failure_class"} {
		value, ok := verification[key].(string)
		if !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("promotion verification %s must be a non-empty string", key)
		}
	}
	issues, ok := proposalArray(verification["issues"])
	if !ok {
		return errors.New("promotion verification issues must be an array of strings")
	}
	for _, issue := range issues {
		if _, ok := issue.(string); !ok {
			return errors.New("promotion verification issues must be an array of strings")
		}
	}
	valueBar, ok := mapDeclaration(verification["value_bar"])
	if !ok {
		return errors.New("promotion verification value_bar must be an object")
	}
	if verdict, ok := valueBar["verdict"].(string); !ok || strings.TrimSpace(verdict) == "" {
		return errors.New("promotion verification value_bar.verdict must be a non-empty string")
	}
	switch valueBar["overall_score"].(type) {
	case int, int64, float32, float64, json.Number:
	default:
		return errors.New("promotion verification value_bar.overall_score must be a number")
	}
	baseline, ok := mapDeclaration(verification["direct_worker_baseline"])
	if !ok {
		return errors.New("promotion verification direct_worker_baseline must be an object")
	}
	if verdict, ok := baseline["verdict"].(string); !ok || strings.TrimSpace(verdict) == "" {
		return errors.New("promotion verification direct_worker_baseline.verdict must be a non-empty string")
	}
	return nil
}

func evaluateSoftwareDevPromotion(repoTaskPack, proposal, metadata, verification map[string]any, identity softwareDevPromotionIdentity, rawRelativePath string) map[string]any {
	delta, meaningful := promotionSoftLayerDelta(repoTaskPack, proposal)
	reasons := []any{}
	status := strings.TrimSpace(stringValue(verification["status"]))
	if status != "pass" {
		reasons = append(reasons, promotionReason("verification_not_passed", "Verification status must be pass."))
	}
	if len(stringList(verification["issues"])) > 0 {
		reasons = append(reasons, promotionReason("verification_has_issues", "Verification issues must be empty."))
	}
	if failureClass := strings.TrimSpace(stringValue(verification["failure_class"])); failureClass != "none" {
		reasons = append(reasons, promotionReason("verification_failure_class", "Verification failure_class must be none."))
	}
	valueBar := mapValue(verification["value_bar"])
	valueVerdict := stringValue(valueBar["verdict"])
	if valueVerdict != "strong_orchestration_value" && valueVerdict != "mixed_orchestration_value" {
		reasons = append(reasons, promotionReason("weak_or_missing_value_bar", "Value-bar evidence must be mixed or strong."))
	}
	baseline := mapValue(verification["direct_worker_baseline"])
	baselineVerdict := stringValue(baseline["verdict"])
	if baselineVerdict != "orchestration_adds_value" && baselineVerdict != "direct_worker_likely_sufficient" {
		reasons = append(reasons, promotionReason("lower_or_missing_direct_worker_baseline", "Direct-worker evidence must not prefer the direct worker."))
	}
	if !meaningful {
		reasons = append(reasons, promotionReason("no_reusable_soft_layer_delta", "The proposal adds no meaningful reusable soft-layer guidance."))
	}
	eligibilityStatus := "eligible"
	if len(reasons) > 0 {
		eligibilityStatus = "ineligible"
	} else {
		reasons = append(reasons,
			promotionReason("selected_proposal_graph_confirmed", "Proposal metadata matches the selected compiled graph."),
			promotionReason("verification_and_value_bar_passed", "Verification, value-bar, and direct-worker gates passed."),
			promotionReason("reusable_soft_layer_delta_found", "At least one reusable soft-layer delta is reviewable."),
		)
	}

	targets := []any{
		map[string]any{
			"kind":    "selected_task_pack_step",
			"path":    identity.TaskPackPath,
			"step_id": identity.StepID,
		},
	}
	if len(mapValue(delta["roles"])) > 0 && identity.SiblingRoles != "" {
		targets = append(targets, map[string]any{
			"kind": "owned_sibling_agents",
			"path": identity.SiblingRoles,
		})
	}

	return map[string]any{
		"contract_version": "software.dev.promotion-candidate/v1",
		"eligibility": map[string]any{
			"status":  eligibilityStatus,
			"reasons": reasons,
		},
		"source_evidence": map[string]any{
			"proposal": map[string]any{
				"raw":            rawRelativePath,
				"normalized":     "proposal/normalized.json",
				"metadata":       "proposal/metadata.json",
				"source_format":  metadata["source_format"],
				"selected_graph": metadata["selected_graph"],
			},
			"verification": map[string]any{
				"path":                    "verify/result.json",
				"status":                  status,
				"failure_class":           verification["failure_class"],
				"value_bar_verdict":       valueVerdict,
				"value_bar_overall_score": valueBar["overall_score"],
				"direct_worker_verdict":   baselineVerdict,
			},
		},
		"mutable_surface": map[string]any{
			"task_pack_path": identity.TaskPackPath,
			"step_id":        identity.StepID,
		},
		"repo_owned_target_surfaces":     targets,
		"promotable_soft_layer_delta":    delta,
		"excluded_session_only_fields":   promotionSessionExclusions(),
		"excluded_hard_authority_fields": promotionHardAuthorityExclusions(),
		"workspace_mutation": map[string]any{
			"performed":    false,
			"confirmation": "Evaluation wrote only proposal/promotion-candidate.json under the existing run artifact root.",
		},
	}
}

func promotionSoftLayerDelta(repoTaskPack, proposal map[string]any) (map[string]any, bool) {
	repoOrchestration := contractOrchestration(repoTaskPack)
	proposalOrchestration := contractOrchestration(proposal)
	meaningful := false
	step := map[string]any{}
	if startup := strings.TrimSpace(stringValue(proposal["startup_prompt"])); startup != "" && startup != strings.TrimSpace(stringValue(repoTaskPack["startup_prompt"])) {
		step["startup_prompt"] = startup
		meaningful = true
	}
	rootConstraints := promotionStringAdditions(stringList(repoTaskPack["constraints"]), stringList(proposal["constraints"]))
	if len(rootConstraints) > 0 {
		step["constraints"] = anySlice(rootConstraints)
		meaningful = true
	}

	orchestration := map[string]any{}
	orchestrationConstraints := promotionStringAdditions(stringList(repoOrchestration["constraints"]), stringList(proposalOrchestration["constraints"]))
	if len(orchestrationConstraints) > 0 {
		orchestration["constraints"] = anySlice(orchestrationConstraints)
		meaningful = true
	}
	plan := promotionPlanDelta(mapValue(repoOrchestration["plan"]), mapValue(proposalOrchestration["plan"]))
	if len(plan) > 0 {
		orchestration["plan"] = plan
		meaningful = true
	}
	for _, key := range []string{"merge", "verify"} {
		difference := promotionScalarMapDelta(mapValue(repoOrchestration[key]), mapValue(proposalOrchestration[key]))
		if len(difference) > 0 {
			orchestration[key] = difference
			meaningful = true
		}
	}
	required := promotionStringAdditions(
		stringList(mapValue(repoOrchestration["apply"])["required_artifacts"]),
		stringList(mapValue(proposalOrchestration["apply"])["required_artifacts"]),
	)
	if len(required) > 0 {
		orchestration["apply"] = map[string]any{"required_artifacts": anySlice(required)}
		meaningful = true
	}
	if len(orchestration) > 0 {
		step["orchestration"] = orchestration
	}

	roles := map[string]any{}
	repoRoles := mapValue(repoOrchestration["agents"])
	proposalRoles := mapValue(proposalOrchestration["agents"])
	roleIDs := make([]string, 0, len(proposalRoles))
	for id := range proposalRoles {
		roleIDs = append(roleIDs, id)
	}
	sort.Strings(roleIDs)
	for _, id := range roleIDs {
		proposalRole := mapValue(proposalRoles[id])
		repoRole := mapValue(repoRoles[id])
		roleDelta := map[string]any{}
		if guidance := strings.TrimSpace(stringValue(proposalRole["role"])); guidance != "" && guidance != strings.TrimSpace(stringValue(repoRole["role"])) {
			roleDelta["role"] = guidance
		}
		constraints := promotionStringAdditions(stringList(repoRole["constraints"]), stringList(proposalRole["constraints"]))
		if len(constraints) > 0 {
			roleDelta["constraints"] = anySlice(constraints)
		}
		if len(roleDelta) > 0 {
			roles[id] = roleDelta
			meaningful = true
		}
	}

	return map[string]any{
		"task_pack_step": step,
		"roles":          roles,
	}, meaningful
}

func promotionPlanDelta(repo, proposal map[string]any) map[string]any {
	out := map[string]any{}
	if goal := strings.TrimSpace(stringValue(proposal["goal"])); goal != "" && goal != strings.TrimSpace(stringValue(repo["goal"])) {
		out["goal"] = goal
	}
	steps := promotionStringAdditions(stringList(repo["steps"]), stringList(proposal["steps"]))
	if len(steps) > 0 {
		out["steps"] = anySlice(steps)
	}
	constraints := promotionStringAdditions(stringList(repo["constraints"]), stringList(proposal["constraints"]))
	if len(constraints) > 0 {
		out["constraints"] = anySlice(constraints)
	}
	return out
}

func promotionScalarMapDelta(repo, proposal map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range proposal {
		if isContractScalar(value) && mustJSON(value, nil) != mustJSON(repo[key], nil) {
			out[key] = value
		}
	}
	return out
}

func promotionStringAdditions(existing, proposed []string) []string {
	out := []string{}
	for _, value := range proposed {
		value = strings.TrimSpace(value)
		if value != "" && !containsString(existing, value) {
			out = appendStableContractStrings(out, value)
		}
	}
	return out
}

func promotionReason(code, message string) map[string]any {
	return map[string]any{"code": code, "message": message}
}

func promotionSessionExclusions() []any {
	return []any{
		map[string]any{"fields": []any{"orchestration.request"}, "reason": "The exact request is per-run context."},
		map[string]any{"fields": []any{"orchestration.tasks", "orchestration.tasks[].goal", "orchestration.tasks[].brief", "orchestration.tasks[].prompt"}, "reason": "Exact task decomposition and task prompts are session-only."},
		map[string]any{"fields": []any{"orchestration.tasks[].depends_on"}, "reason": "The exact dependency graph is session-only."},
		map[string]any{"fields": []any{"orchestration.agents[].worker", "orchestration.agents[].worker_type", "orchestration.agents[].work_mode", "orchestration.agents[].worker_policy", "orchestration.agents[].model", "orchestration.agents[].model_policy"}, "reason": "Selected workers, providers, models, and lanes are session-only."},
		map[string]any{"fields": []any{"orchestration.tasks[].materialize_outputs"}, "reason": "One-off inferred outputs remain session-only; only explicit required-artifact floor additions may be proposed."},
		map[string]any{"fields": []any{"verification.recommended_rerun_tasks", "verification.root_cause_task", "verification.next_action", "verification.issues"}, "reason": "Repair plans and validation failures are session-only evidence."},
	}
}

func promotionHardAuthorityExclusions() []any {
	return []any{
		map[string]any{"fields": []any{"access", "orchestration.access", "orchestration.agents[].access", "orchestration.agents[].accessible_paths"}, "reason": "Mounts and access policy are hard runtime authority."},
		map[string]any{"fields": []any{"access.deny", "orchestration.access.deny", "orchestration.agents[].access.deny", "remove_deny"}, "reason": "Deny-rule changes are never promotable."},
		map[string]any{"fields": []any{"auth", "secrets"}, "reason": "Authentication and secrets are never promotable."},
		map[string]any{"fields": []any{"cloud_budget", "max_cloud_tokens", "orchestration.cloud_budget", "orchestration.agents[].cloud_budget", "orchestration.agents[].max_cloud_tokens"}, "reason": "Budgets and token ceilings are package-owned authority."},
		map[string]any{"fields": []any{"orchestration.apply.require_approval", "orchestration.agents[].require_approval"}, "reason": "Approval settings are package-owned authority."},
		map[string]any{"fields": []any{"orchestration.apply.target_root"}, "reason": "The repo-selected apply target cannot be changed by promotion."},
		map[string]any{"fields": []any{"publish", "sync", "orchestration.publish", "orchestration.sync", "orchestration.agents[].publish", "orchestration.agents[].sync"}, "reason": "Publish and sync behavior are package-owned authority."},
		map[string]any{"fields": []any{"destructive_action_policy"}, "reason": "Destructive-action policy is never promotable."},
	}
}
