package dependencylifecycle

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dockpipe/src/lib/domain/dependency"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/process"
)

// WorkflowProvider reuses resolver discovery and the current CLI's ordinary
// workflow admission/execution. It does not resolve tools or implement installers.
type WorkflowProvider struct {
	Executable  string
	Diagnostics io.Writer
}

func (provider WorkflowProvider) Inspect(ctx context.Context, workdir, resolver, machine string) (dependency.Inventory, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	var empty dependency.Inventory
	if !filepath.IsAbs(provider.Executable) {
		return empty, errors.New("dependency inspection requires an explicit absolute Dockpipe executable")
	}
	if !dependency.ValidLifecycleIdentifier(resolver) || !dependency.ValidLifecycleIdentifier(machine) {
		return empty, errors.New("dependency resolver and machine must be bounded identifiers")
	}
	profilePath, err := infrastructure.ResolveResolverFilePath(workdir, resolver)
	if err != nil {
		return empty, err
	}
	profile, err := infrastructure.LoadResolverFile(profilePath)
	if err != nil {
		return empty, err
	}
	if profile["DOCKPIPE_DEPENDENCY_LIFECYCLE_PROTOCOL"] != dependency.InventorySchema {
		return empty, errors.New("resolver does not support the dependency inventory protocol")
	}
	workflow, err := containedWorkflow(filepath.Dir(profilePath), profile["DOCKPIPE_DEPENDENCY_LIFECYCLE_WORKFLOW"])
	if err != nil {
		return empty, err
	}
	metadata, err := infrastructure.DescribeResolver(profilePath, resolver)
	if err != nil || metadata.Version == "" {
		return empty, errors.New("dependency lifecycle requires versioned resolver metadata")
	}
	root, err := infrastructure.DurableStateRoot()
	if err != nil {
		return empty, err
	}
	if err := infrastructure.PreparePrivateDirectory(root); err != nil {
		return empty, err
	}
	root, err = infrastructure.PreparePrivateStateSubdirectory(root, "dependency-lifecycle")
	if err != nil {
		return empty, err
	}
	runRoot, err := os.MkdirTemp(root, "inspect-")
	if err != nil {
		return empty, err
	}
	defer os.RemoveAll(runRoot)
	if err := infrastructure.ValidatePrivatePath(runRoot, true); err != nil {
		return empty, err
	}
	receipt := filepath.Join(runRoot, "inventory.json")
	arguments := []string{"--workflow-file", workflow, "--workdir", workdir,
		"--env", "DOCKPIPE_DEPENDENCY_OUTPUT=" + receipt,
		"--env", "DOCKPIPE_DEPENDENCY_MACHINE=" + machine,
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
		if ctx.Err() != nil {
			return empty, ctx.Err()
		}
		return empty, fmt.Errorf("dependency inspection workflow failed: %w", err)
	}
	if err := infrastructure.ValidatePrivatePath(receipt, false); err != nil {
		return empty, fmt.Errorf("dependency receipt is not private: %w", err)
	}
	inventory, err := ReadInventory(receipt)
	if err != nil {
		return empty, err
	}
	if err := inventory.Validate(time.Now()); err != nil {
		return empty, err
	}
	if inventory.Provider.ID != resolver || inventory.Provider.Version != metadata.Version || inventory.Machine != machine {
		return empty, errors.New("dependency receipt does not match the selected resolver, version and target")
	}
	return inventory, nil
}

func containedWorkflow(root, relative string) (string, error) {
	if relative == "" || len(relative) > 4096 || filepath.IsAbs(relative) || strings.ContainsAny(relative, "\x00\r\n") {
		return "", errors.New("resolver dependency lifecycle workflow must be package-relative")
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	path, err := filepath.EvalSymlinks(filepath.Join(root, relative))
	if err != nil {
		return "", err
	}
	contained, err := filepath.Rel(root, path)
	if err != nil || contained == ".." || strings.HasPrefix(contained, ".."+string(filepath.Separator)) {
		return "", errors.New("dependency workflow escapes the selected resolver")
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("dependency workflow is not a regular file")
	}
	return path, nil
}

func inventoryEnvironment(base []string, root, executable string) []string {
	result := make([]string, 0, len(base)+3)
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(key) {
		case "DOCKPIPE_DEPENDENCY_REQUEST", "DOCKPIPE_DEPENDENCY_OPERATION", "DOCKPIPE_ARTIFACT_ROOT", "DOCKPIPE_OUTPUT_ROOT", "DOCKPIPE_DEPENDENCY_OUTPUT", "DOCKPIPE_DEPENDENCY_MACHINE", "DOCKPIPE_BIN":
			continue
		}
		result = append(result, entry)
	}
	return append(result, "DOCKPIPE_ARTIFACT_ROOT="+root, "DOCKPIPE_OUTPUT_ROOT="+root, "DOCKPIPE_BIN="+executable)
}

type boundedDiagnostics struct {
	mutex     sync.Mutex
	writer    io.Writer
	remaining int
}

func (output *boundedDiagnostics) Write(data []byte) (int, error) {
	output.mutex.Lock()
	defer output.mutex.Unlock()
	if len(data) > output.remaining {
		return 0, errors.New("dependency workflow diagnostics exceeded 64 KiB; output is truncated")
	}
	output.remaining -= len(data)
	return output.writer.Write(data)
}
