package orchestrationhelper

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func compileSoftwareDevArtifacts(packageWorkflowPath, packageStepID, repoRoot, taskPackPath, taskPackStepID, artifactRoot, proposalPath string) error {
	rootPath, err := softwareDevArtifactRoot(artifactRoot)
	if err != nil {
		return err
	}
	artifactRoot = rootPath
	cleanupOnError := func(err error, rawProposal []byte) error {
		if len(rawProposal) > 0 {
			if recordErr := recordRejectedSoftwareDevProposal(artifactRoot, rawProposal); recordErr != nil {
				err = fmt.Errorf("%v; record rejected proposal: %w", err, recordErr)
			}
		}
		if cleanupErr := cleanupSoftwareDevExecutableArtifacts(artifactRoot); cleanupErr != nil {
			return fmt.Errorf("%v; clean executable artifacts: %w", err, cleanupErr)
		}
		return err
	}

	packageWorkflowAbs, err := filepath.Abs(packageWorkflowPath)
	if err != nil {
		return cleanupOnError(fmt.Errorf("invalid package workflow path: %w", err), nil)
	}
	packageDefaults, err := loadTaskPack(filepath.Dir(packageWorkflowAbs), filepath.Base(packageWorkflowAbs), packageStepID)
	if err != nil {
		return cleanupOnError(fmt.Errorf("load package software.dev contract: %w", err), nil)
	}
	packageDefaults["cloud_budget"] = map[string]any{
		"max_total_tokens": 60000,
		"max_task_tokens":  20000,
		"max_tasks":        8,
	}
	packageDefaults["constraints"] = []any{
		"execute only tasks in the successfully compiled graph",
		"preserve package access, budget, approval, apply, publish, and sync authority",
	}
	packageDefaults["publish"] = false
	packageDefaults["sync"] = false
	repoTaskPack, err := loadTaskPack(repoRoot, taskPackPath, taskPackStepID)
	if err != nil {
		return cleanupOnError(err, nil)
	}

	var proposal *parsedPlannerProposal
	var rawProposal []byte
	if strings.TrimSpace(proposalPath) != "" {
		rawProposal, err = os.ReadFile(proposalPath)
		if err != nil {
			return cleanupOnError(fmt.Errorf("planner proposal cannot be read: %w", err), nil)
		}
		proposal, err = parsePlannerProposal(rawProposal)
		if err != nil {
			return cleanupOnError(err, rawProposal)
		}
	}

	compiled, err := compileExecutableContract(packageDefaults, repoTaskPack, proposal)
	if err != nil {
		return cleanupOnError(err, rawProposal)
	}
	canonicalTaskPackPath := filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(taskPackPath))))
	compiled.ProposalMetadata["task_pack"] = map[string]any{
		"path":    canonicalTaskPackPath,
		"step_id": strings.TrimSpace(taskPackStepID),
	}
	if err := materializeSoftwareDevContract(artifactRoot, repoRoot, compiled, proposal, rawProposal); err != nil {
		return cleanupOnError(err, rawProposal)
	}
	return nil
}

func stageSoftwareDevProposal(repoRoot, repoRelativeProposal, target, artifactRoot string) error {
	raw, err := readRepoRelativeRegularFile(repoRoot, repoRelativeProposal)
	if err != nil {
		return err
	}
	if strings.TrimSpace(artifactRoot) == "" {
		return errors.New("software.dev artifact root is required when staging a proposal")
	}
	rootPath, err := softwareDevArtifactRoot(artifactRoot)
	if err != nil {
		return err
	}
	targetPath, err := filepath.Abs(target)
	if err != nil || !withinRoot(rootPath, targetPath) {
		return fmt.Errorf("software.dev staged proposal target escapes the run artifact root: %s", target)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	resolvedRoot, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		return fmt.Errorf("software.dev artifact root cannot be resolved: %w", err)
	}
	resolvedParent, err := filepath.EvalSymlinks(filepath.Dir(targetPath))
	if err != nil || !withinRoot(resolvedRoot, resolvedParent) {
		return fmt.Errorf("software.dev staged proposal target escapes the resolved run artifact root: %s", target)
	}
	if info, statErr := os.Lstat(targetPath); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("software.dev staged proposal target cannot be a symlink: %s", target)
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return statErr
	}
	return os.WriteFile(targetPath, raw, 0o644)
}

func readRepoRelativeRegularFile(repoRoot, relativePath string) ([]byte, error) {
	displayPath := strings.TrimSpace(relativePath)
	if displayPath == "" {
		return nil, errors.New("proposal fixture path is required")
	}
	if filepath.IsAbs(displayPath) || filepath.VolumeName(displayPath) != "" {
		return nil, fmt.Errorf("proposal fixture path %q must be relative to the consumer repo", displayPath)
	}
	rootPath, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("proposal fixture path %q has an invalid consumer repo root: %w", displayPath, err)
	}
	candidatePath, err := filepath.Abs(filepath.Join(rootPath, filepath.Clean(filepath.FromSlash(displayPath))))
	if err != nil || !withinRoot(rootPath, candidatePath) {
		return nil, fmt.Errorf("proposal fixture path %q escapes the consumer repo", displayPath)
	}
	resolvedRoot, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		return nil, fmt.Errorf("proposal fixture path %q has an invalid consumer repo root: %w", displayPath, err)
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidatePath)
	if err != nil {
		return nil, fmt.Errorf("proposal fixture path %q cannot be resolved: %w", displayPath, err)
	}
	if !withinRoot(resolvedRoot, resolvedCandidate) {
		return nil, fmt.Errorf("proposal fixture path %q escapes the consumer repo", displayPath)
	}
	info, err := os.Stat(resolvedCandidate)
	if err != nil {
		return nil, fmt.Errorf("proposal fixture path %q cannot be read: %w", displayPath, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("proposal fixture path %q is not a regular file", displayPath)
	}
	raw, err := os.ReadFile(resolvedCandidate)
	if err != nil {
		return nil, fmt.Errorf("proposal fixture path %q cannot be read: %w", displayPath, err)
	}
	return raw, nil
}

func materializeSoftwareDevContract(artifactRoot, repoRoot string, compiled *compiledExecutableContract, proposal *parsedPlannerProposal, rawProposal []byte) error {
	rootPath, err := softwareDevArtifactRoot(artifactRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(rootPath, 0o755); err != nil {
		return err
	}
	stageRoot, err := os.MkdirTemp(rootPath, ".software-dev-compile-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageRoot)

	if err := writeJSONFile(filepath.Join(stageRoot, "request.json"), mapValue(compiled.Plan["request"])); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(stageRoot, "plan.json"), compiled.Plan); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(stageRoot, "task-graph.json"), compiled.TaskGraph); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(stageRoot, "proposal", "metadata.json"), compiled.ProposalMetadata); err != nil {
		return err
	}
	if proposal != nil {
		if err := writeJSONFile(filepath.Join(stageRoot, "proposal", "normalized.json"), proposal.Declaration); err != nil {
			return err
		}
		extension := ".yaml"
		if proposal.Format == "json" {
			extension = ".json"
		}
		if err := os.WriteFile(filepath.Join(stageRoot, "proposal", "raw"+extension), rawProposal, 0o644); err != nil {
			return err
		}
	}
	for _, rawTask := range compiled.TaskArtifacts {
		task := mapValue(rawTask)
		taskID := stringValue(task["id"])
		if taskID == "" {
			return errors.New("compiled software.dev task artifact has no id")
		}
		taskDir := filepath.Join(stageRoot, "tasks", taskID)
		if err := writeJSONFile(filepath.Join(taskDir, "task.json"), task); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(taskDir, "prompt.md"), []byte(renderCompiledSoftwareDevPrompt(compiled.Plan, task, repoRoot)), 0o644); err != nil {
			return err
		}
	}

	for _, name := range []string{"request.json", "plan.json", "task-graph.json", "tasks", "proposal"} {
		target := filepath.Join(rootPath, name)
		if err := os.RemoveAll(target); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(stageRoot, name), target); err != nil {
			return err
		}
	}
	return nil
}

func renderCompiledSoftwareDevPrompt(plan, task map[string]any, repoRoot string) string {
	lines := []string{}
	if startup := strings.TrimSpace(stringValue(plan["startup_prompt"])); startup != "" {
		lines = append(lines, startup, "")
	}
	lines = append(lines,
		"You are one bounded worker in a governed software-development task graph.",
		"",
		"Task id: "+stringValue(task["id"]),
		"Agent role: "+fallbackString(stringValue(task["role"]), stringValue(task["agent"])),
		"Goal: "+stringValue(task["goal"]),
		"Expected output: "+stringValue(task["expected_output"]),
	)
	if brief := strings.TrimSpace(stringValue(task["brief"])); brief != "" {
		lines = append(lines, "", "Brief:", brief)
	}
	if prompt := strings.TrimSpace(stringValue(task["prompt"])); prompt != "" {
		lines = append(lines, "", "Task instructions:", prompt)
	}
	if constraints := stringList(task["constraints"]); len(constraints) > 0 {
		lines = append(lines, "", "Constraints:")
		for _, constraint := range constraints {
			lines = append(lines, "- "+constraint)
		}
	}
	if contextPaths := stringList(task["context_paths"]); len(contextPaths) > 0 {
		lines = append(lines, "", "Context briefing paths:")
		for _, contextPath := range contextPaths {
			lines = append(lines, "- "+contextPath)
		}
	}
	access := mapValue(task["access"])
	if len(stringList(access["read"]))+len(stringList(access["write"]))+len(stringList(access["deny"])) > 0 {
		lines = append(lines, "", "Access policy:")
		for _, mode := range []string{"read", "write", "deny"} {
			paths := stringList(access[mode])
			if len(paths) == 0 {
				continue
			}
			lines = append(lines, mode+":")
			for _, path := range paths {
				lines = append(lines, "- "+path)
			}
		}
	}
	if boolAny(plan["include_agents_md"]) {
		if rawAgents, err := os.ReadFile(filepath.Join(repoRoot, "AGENTS.md")); err == nil {
			lines = append(lines, "", "AGENTS.md context:", "", strings.TrimRight(string(rawAgents), "\n"))
		}
	}
	lines = append(lines,
		"",
		"Rules:",
		"- Execute only this compiled task and respect its dependencies.",
		"- Treat access, deny, budget, approval, apply, publish, and sync policy as fixed authority.",
		"- Return the requested artifact content directly without narrating commands or orchestration mechanics.",
	)
	if contract := renderMaterializeOutputContract(listValue(task["materialize_outputs"])); contract != "" {
		lines = append(lines, "", contract)
	}
	return strings.Join(lines, "\n") + "\n"
}

func cleanupSoftwareDevExecutableArtifacts(artifactRoot string) error {
	rootPath, err := softwareDevArtifactRoot(artifactRoot)
	if err != nil {
		return err
	}
	for _, name := range []string{"plan.json", "task-graph.json", "tasks"} {
		if err := os.RemoveAll(filepath.Join(rootPath, name)); err != nil {
			return err
		}
	}
	return nil
}

func recordRejectedSoftwareDevProposal(artifactRoot string, raw []byte) error {
	rootPath, err := softwareDevArtifactRoot(artifactRoot)
	if err != nil {
		return err
	}
	proposalDir := filepath.Join(rootPath, "proposal")
	if err := os.RemoveAll(proposalDir); err != nil {
		return err
	}
	if err := os.MkdirAll(proposalDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(proposalDir, "rejected.txt"), raw, 0o644)
}

func softwareDevArtifactRoot(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errors.New("software.dev artifact root is required")
	}
	rootPath, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("invalid software.dev artifact root: %w", err)
	}
	if filepath.Dir(rootPath) == rootPath {
		return "", fmt.Errorf("software.dev artifact root cannot be a filesystem root: %s", rootPath)
	}
	return rootPath, nil
}
