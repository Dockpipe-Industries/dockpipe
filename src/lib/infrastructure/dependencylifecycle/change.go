package dependencylifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"dockpipe/src/lib/domain/dependency"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/process"
)

func ReadChangeRequest(path string) (dependency.ChangeRequest, error) {
	var request dependency.ChangeRequest
	if err := readReceipt(path, &request); err != nil {
		return request, err
	}
	return request, request.Validate()
}

// Change invokes the existing workflow runner. The provider owns installation,
// locking, durable journals, postconditions and recovery; Core never guesses an
// installer or restarts a service. Ambiguous exits are never automatically retried.
func (provider WorkflowProvider) Change(ctx context.Context, workdir string, request dependency.ChangeRequest, operation, approval string) (dependency.ChangeResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	var result dependency.ChangeResult
	if err := request.Validate(); err != nil {
		return result, err
	}
	if operation != "plan" && operation != "apply" && operation != "recover" && operation != "recover-plan" && operation != "activate" && operation != "activation-plan" && operation != "activation-status" {
		return result, errors.New("unsupported dependency change operation")
	}
	if operation != "plan" && operation != "recover-plan" && operation != "activation-plan" && operation != "activation-status" && approval != request.ApprovalID(operation) {
		return result, errors.New("approval does not match this exact request and operation; run the dry-run first")
	}
	if !filepath.IsAbs(provider.Executable) {
		return result, errors.New("an absolute operator CLI is required")
	}
	profilePath, err := infrastructure.ResolveResolverFilePath(workdir, request.Resolver)
	if err != nil {
		return result, err
	}
	profile, err := infrastructure.LoadResolverFile(profilePath)
	if err != nil {
		return result, err
	}
	if profile["DOCKPIPE_DEPENDENCY_CHANGE_PROTOCOL"] != dependency.ChangeSchema {
		return result, errors.New("resolver does not support dependency changes")
	}
	metadata, err := infrastructure.DescribeResolver(profilePath, request.Resolver)
	if err != nil || metadata.Version != request.ProviderVersion {
		return result, errors.New("selected resolver version differs from the approved request")
	}
	workflow, err := containedWorkflow(filepath.Dir(profilePath), profile["DOCKPIPE_DEPENDENCY_CHANGE_WORKFLOW"])
	if err != nil {
		return result, err
	}
	root, err := infrastructure.DurableStateRoot()
	if err != nil {
		return result, err
	}
	if err := infrastructure.PreparePrivateDirectory(root); err != nil {
		return result, err
	}
	root, err = infrastructure.PreparePrivateStateSubdirectory(root, "dependency-lifecycle")
	if err != nil {
		return result, err
	}
	runRoot, err := os.MkdirTemp(root, "change-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(runRoot)
	requestPath := filepath.Join(runRoot, "request.json")
	data, err := json.Marshal(request)
	if err != nil {
		return result, err
	}
	if err := os.WriteFile(requestPath, data, 0600); err != nil {
		return result, err
	}
	outputPath := filepath.Join(runRoot, "result.json")
	arguments := []string{"--workflow-file", workflow, "--workdir", workdir,
		"--env", "DOCKPIPE_DEPENDENCY_REQUEST=" + requestPath,
		"--env", "DOCKPIPE_DEPENDENCY_OPERATION=" + operation,
		"--env", "DOCKPIPE_DEPENDENCY_OUTPUT=" + outputPath,
		"--env", "DOCKPIPE_ARTIFACT_ROOT=" + runRoot}
	command := exec.CommandContext(ctx, provider.Executable, arguments...)
	command.Dir = workdir
	command.Env = inventoryEnvironment(os.Environ(), runRoot, provider.Executable)
	command.WaitDelay = time.Second
	diagnostics := provider.Diagnostics
	if diagnostics == nil {
		diagnostics = io.Discard
	}
	output := &boundedDiagnostics{writer: diagnostics, remaining: 64 << 10}
	command.Stdout, command.Stderr = output, output
	if err := process.RunNested(command); err != nil {
		return result, fmt.Errorf("dependency workflow did not complete; inspect the provider journal before recovery (never retry automatically): %w", err)
	}
	if err := infrastructure.ValidatePrivatePath(outputPath, false); err != nil {
		return result, err
	}
	if err := readReceipt(outputPath, &result); err != nil {
		return result, err
	}
	if err := result.Validate(); err != nil {
		return result, err
	}
	if (operation == "apply" || operation == "recover" || operation == "activate") && result.Status == "ready" {
		return result, errors.New("provider returned a planning result for an execution operation")
	}
	if (operation == "plan" || operation == "recover-plan" || operation == "activation-plan") && (result.Status == "installed-activation-pending" || result.Status == "recovered-activation-pending" || result.Status == "active-healthy" || result.Status == "activation-recovered") {
		return result, errors.New("provider returned an execution result for a planning operation")
	}
	return result, nil
}
