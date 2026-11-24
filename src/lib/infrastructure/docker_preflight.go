package infrastructure

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure/process"
)

// dockerPreflightHook, if non-nil, replaces the default Docker check (tests).
var dockerPreflightHook func(*os.File) error

var dockerPreflightOnce sync.Once
var dockerPreflightErr error

// EnsureDockerReachable runs a one-time lightweight check before the first container use in this process.
// It is skipped when DOCKPIPE_SKIP_DOCKER_PREFLIGHT=1 (application tests).
func EnsureDockerReachable(stderr *os.File) error {
	dockerPreflightOnce.Do(func() {
		dockerPreflightErr = admitDockerPreflight(NativeExecutionAdmission{}, stderr, false)
	})
	return dockerPreflightErr
}

// DockerDoctorCheck runs the same logic as EnsureDockerReachable but not memoized (for dockpipe doctor).
// It ignores DOCKPIPE_SKIP_DOCKER_PREFLIGHT so diagnostics always exercise Docker.
func DockerDoctorCheck(stderr *os.File) error {
	return DockerDoctorCheckWithAdmission(NativeExecutionAdmission{}, stderr)
}

func DockerDoctorCheckWithAdmission(admission NativeExecutionAdmission, stderr *os.File) error {
	return admitDockerPreflight(admission, stderr, true)
}

func admitDockerPreflight(admission NativeExecutionAdmission, stderr *os.File, ignoreSkipEnv bool) error {
	_, err := invokeNativeExecution(admission, "dockpipe.containers.preflight", ignoreSkipEnv,
		func(_ context.Context, owned bool) (invocation.Prepared[struct{}], error) {
			environment := os.Environ()
			prepared := invocation.Prepared[struct{}]{Execute: func(ctx context.Context) (struct{}, error) {
				return struct{}{}, checkDockerReachable(ctx, stderr, owned, environment)
			}}
			if admission.Authorize != nil {
				prepared.Digest = NativeExecutionDigest([]string{fmt.Sprint(owned), invocationFileName(stderr)}, environment)
			}
			return prepared, nil
		})
	return err
}

func checkDockerReachable(parent context.Context, stderr *os.File, ignoreSkipEnv bool, environment []string) error {
	if dockerPreflightHook != nil {
		return dockerPreflightHook(stderr)
	}
	if !ignoreSkipEnv && envGet(environment, "DOCKPIPE_SKIP_DOCKER_PREFLIGHT") == "1" {
		return nil
	}
	dockerCmd, err := resolveDockerCommandFromEnvironment(environment)
	if err != nil {
		return fmt.Errorf(
			"docker not found in PATH — install the Docker CLI and ensure the daemon is running\n" +
				"  https://docs.docker.com/get-docker/",
		)
	}

	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	cmd := execCommandContextFn(ctx, dockerCmd, "version")
	cmd.Env = dockerCommandEnv(environment, dockerCmd)
	output := &dockerReadOutput{}
	cmd.Stdout, cmd.Stderr = output, output
	cmd.WaitDelay = time.Second
	err = process.Run(cmd)
	if parent.Err() != nil {
		return parent.Err()
	}
	if output.exhausted {
		return fmt.Errorf("preflight output from Docker exceeds its 512-KiB diagnostic limit")
	}
	msg := strings.TrimSpace(output.buffer.String())
	if err == nil {
		return nil
	}

	var hint strings.Builder
	hint.WriteString("docker is not reachable")
	if ctx.Err() == context.DeadlineExceeded {
		hint.WriteString(" (timed out after 15s talking to the daemon)")
	}
	hint.WriteString(".\n")
	if msg != "" {
		hint.WriteString(msg)
		hint.WriteString("\n")
	}
	hint.WriteString(dockerDaemonHints())
	return fmt.Errorf("%s", strings.TrimSpace(hint.String()))
}

// execCommandContextFn is exec.CommandContext for tests.
var execCommandContextFn = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, arg...)
}

func dockerCommandName() string {
	if p, err := resolveDockerCommandPath(); err == nil {
		return p
	}
	return "docker"
}

func resolveDockerCommandPath() (string, error) {
	return resolveDockerCommandFromEnvironment(os.Environ())
}

func dockerDaemonHints() string {
	var b strings.Builder
	switch runtime.GOOS {
	case "darwin":
		b.WriteString("  • Start the Docker daemon or VM provider selected by your Docker context.\n")
	case "windows":
		b.WriteString("  • Start Docker Desktop. For WSL2: enable Docker Desktop → Settings → Resources → WSL integration.\n")
	default:
		b.WriteString("  • Linux: try `sudo systemctl start docker` (or your distro’s service name).\n")
		b.WriteString("  • Add your user to the `docker` group if you see permission errors on the socket: `sudo usermod -aG docker $USER` (log out/in).\n")
	}
	b.WriteString("  • Check `docker context ls` and any DOCKER_CONTEXT or DOCKER_HOST overrides.\n")
	b.WriteString("  • Docs: https://docs.docker.com/config/daemon/start/\n")
	return b.String()
}
