package infrastructure

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/domain/invocation"
)

// Host cleanup after kind: host scripts (RunHostScript):
//
// When DOCKPIPE_RUN_ID is set (normal host run with a workdir), cleanup is run-scoped only:
// workdir/bin/.dockpipe/runs/<runID>.container holds the Docker container name for this invocation.
// Matching entries under workdir/bin/.dockpipe/cleanup/docker-* and the legacy session_container file
// are removed without stopping other containers.
//
// When DOCKPIPE_RUN_ID is empty (no host run registry), legacy mode scans workdir/bin/.dockpipe/cleanup/docker-*
// and workdir/bin/.dockpipe/cursor-dev/session_container — each file is one line: a container name to stop.
//
// Templates register resources by writing those files; the Go runner applies cleanup when the bash
// child exits (trap, normal return, or defer if the process died without removing markers).

var (
	hostCleanupDirRel         = filepath.Join(DockpipeDirRel, "cleanup")
	cursorDevSessionLegacyRel = filepath.Join(DockpipeDirRel, "cursor-dev", "session_container")
	hostCleanupExecCommandFn  = exec.CommandContext
)

func envGet(env []string, key string) string {
	key = domain.NativeEnvKey(key)
	for i := len(env) - 1; i >= 0; i-- {
		candidate, value, found := strings.Cut(env[i], "=")
		if found && domain.NativeEnvKey(candidate) == key {
			return value
		}
	}
	return ""
}

func hostCleanupSkip(env []string) bool {
	v := strings.TrimSpace(strings.ToLower(envGet(env, "DOCKPIPE_SKIP_HOST_CLEANUP")))
	return v == "1" || v == "true" || v == "yes"
}

// ApplyHostCleanup stops Docker containers registered for this host run.
// With DOCKPIPE_RUN_ID, only runs/<id>.container is used (not a global cleanup/ sweep).
// Call from RunHostScript defer so dockpipe tears down host-started resources the script tracked.
func ApplyHostCleanup(env []string) {
	if err := ApplyHostCleanupWithAdmission(NativeExecutionAdmission{}, env); err != nil {
		fmt.Fprintf(os.Stderr, "[dockpipe] Host cleanup failed: %v\n", err)
	}
}

// ApplyHostCleanupWithAdmission freezes the cleanup scope before reading any
// markers. Existing run-scoped/legacy ownership and best-effort cleanup remain
// in the executor; admission cannot expand the registered resource selection.
func ApplyHostCleanupWithAdmission(admission NativeExecutionAdmission, env []string) error {
	_, err := invokeNativeExecution(admission, "dockpipe.containers.cleanup", env,
		func(_ context.Context, input []string) (invocation.Prepared[struct{}], error) {
			if err := ValidateNativeExecutionStrings(input); err != nil {
				return invocation.Prepared[struct{}]{}, err
			}
			owned := append([]string(nil), input...)
			environment := os.Environ()
			prepared := invocation.Prepared[struct{}]{Execute: func(ctx context.Context) (struct{}, error) {
				return struct{}{}, applyHostCleanup(ctx, owned, environment)
			}}
			if admission.Authorize != nil {
				prepared.Digest = NativeExecutionDigest(owned, environment)
			}
			return prepared, nil
		})
	return err
}

func applyHostCleanup(parent context.Context, env, environment []string) error {
	if hostCleanupSkip(env) {
		return nil
	}
	wd := strings.TrimSpace(envGet(env, "DOCKPIPE_WORKDIR"))
	if wd == "" {
		return nil
	}
	wdAbs, err := absHostWorkdir(wd)
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()

	runID := strings.TrimSpace(envGet(env, "DOCKPIPE_RUN_ID"))
	if runID != "" {
		if !isValidHostRunID(runID) {
			return nil
		}
		applyRunScopedHostCleanup(ctx, wdAbs, runID, environment)
		return ctx.Err()
	}

	applyLegacyHostCleanupSweep(ctx, wdAbs, environment)
	return ctx.Err()
}

// isValidHostRunID rejects path segments and spoofed env (expected: 8 hex chars from BeginHostRun).
func isValidHostRunID(runID string) bool {
	if len(runID) != 8 {
		return false
	}
	for _, c := range runID {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

func applyRunScopedHostCleanup(ctx context.Context, wdAbs, runID string, environment []string) {
	pidSidecar := filepath.Join(HostRunsDir(wdAbs), runID+".pid")
	if stopped := tryKillProcessAndRemoveMarker(pidSidecar); stopped {
		LogOperationResult(os.Stderr, OperationResult{
			Unit:       "host.cleanup.process",
			Status:     OperationStatusDone,
			DurationMs: 0,
			IDs: map[string]string{
				"marker": filepath.Base(pidSidecar),
				"result": "stopped",
				"scope":  "run",
			},
		})
	}
	sidecar := filepath.Join(HostRunsDir(wdAbs), runID+".container")
	b, err := os.ReadFile(sidecar)
	if err != nil {
		return
	}
	cn := strings.TrimSpace(string(b))
	if cn == "" {
		_ = os.Remove(sidecar)
		return
	}
	marker := sidecar
	if stopped := tryStopDockerAndRemoveMarker(ctx, cn, marker, environment); stopped {
		logHostCleanupContainerResult(cn, "run", "sidecar", "stopped", "")
	}
	removeCleanupMarkersForContainerName(wdAbs, cn)
}

func tryKillProcessAndRemoveMarker(marker string) bool {
	b, err := os.ReadFile(marker)
	if err != nil {
		return false
	}
	var pid int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(b)), "%d", &pid); err != nil || pid <= 0 {
		_ = os.Remove(marker)
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(marker)
		return false
	}
	_ = proc.Kill()
	_ = os.Remove(marker)
	return true
}

// removeCleanupMarkersForContainerName deletes docker-* and legacy session_container files whose
// single-line content matches cn (after the container was stopped via the run sidecar).
func removeCleanupMarkersForContainerName(wdAbs, cn string) {
	if cn == "" {
		return
	}
	dir := filepath.Join(wdAbs, hostCleanupDirRel)
	ents, err := os.ReadDir(dir)
	if err == nil {
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasPrefix(name, "docker-") {
				continue
			}
			p := filepath.Join(dir, name)
			b, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			if strings.TrimSpace(string(b)) == cn {
				_ = os.Remove(p)
			}
		}
	}
	leg := filepath.Join(wdAbs, cursorDevSessionLegacyRel)
	if b, err := os.ReadFile(leg); err == nil && strings.TrimSpace(string(b)) == cn {
		_ = os.Remove(leg)
	}
}

func applyLegacyHostCleanupSweep(ctx context.Context, wdAbs string, environment []string) {
	dir := filepath.Join(wdAbs, hostCleanupDirRel)
	ents, err := os.ReadDir(dir)
	if err == nil {
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasPrefix(name, "docker-") {
				continue
			}
			p := filepath.Join(dir, name)
			b, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			cn := strings.TrimSpace(string(b))
			if stopped := tryStopDockerAndRemoveMarker(ctx, cn, p, environment); stopped {
				logHostCleanupContainerResult(cn, "legacy", "cleanup_marker", "stopped", "")
			}
		}
	}

	leg := filepath.Join(wdAbs, cursorDevSessionLegacyRel)
	b, err := os.ReadFile(leg)
	if err != nil {
		return
	}
	cn := strings.TrimSpace(string(b))
	if stopped := tryStopDockerAndRemoveMarker(ctx, cn, leg, environment); stopped {
		logHostCleanupContainerResult(cn, "legacy", "legacy_marker", "stopped", "")
	}
}

func logHostCleanupContainerResult(name, scope, markerType, result, skipReason string) {
	ids := map[string]string{
		"container": strings.TrimSpace(name),
		"result":    strings.TrimSpace(result),
	}
	if value := strings.TrimSpace(scope); value != "" {
		ids["scope"] = value
	}
	if value := strings.TrimSpace(markerType); value != "" {
		ids["marker_type"] = value
	}
	if value := strings.TrimSpace(skipReason); value != "" {
		ids["skip_reason"] = value
	}
	LogOperationResult(os.Stderr, OperationResult{
		Unit:       "host.cleanup.container",
		Status:     OperationStatusDone,
		DurationMs: 0,
		IDs:        ids,
	})
}

func tryStopDockerAndRemoveMarker(ctx context.Context, name, markerPath string, environment []string) bool {
	if name == "" || !isSafeDockerContainerName(name) {
		_ = os.Remove(markerPath)
		return false
	}
	provider, err := resolveDockerCommandFromEnvironment(environment)
	if err != nil {
		_ = os.Remove(markerPath)
		return false
	}
	run := func(args ...string) error {
		cmd := hostCleanupExecCommandFn(ctx, provider, args...)
		cmd.Env = dockerCommandEnv(environment, provider)
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	if err := run("stop", "-t", "10", name); err == nil {
		_ = os.Remove(markerPath)
		return true
	}
	if err := run("inspect", name); err != nil {
		_ = os.Remove(markerPath)
		return false
	}
	_ = run("kill", name)
	_ = run("stop", "-t", "2", name)
	if err := run("inspect", name); err != nil {
		_ = os.Remove(markerPath)
		return true
	}
	logHostCleanupContainerResult(name, "", "", "skipped", "stop_failed")
	_ = os.Remove(markerPath)
	return false
}

func isSafeDockerContainerName(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !isDockerNameCharFirst(r) {
				return false
			}
			continue
		}
		if isDockerNameCharFirst(r) || r == '.' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func isDockerNameCharFirst(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}
