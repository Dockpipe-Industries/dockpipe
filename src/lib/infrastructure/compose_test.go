package infrastructure

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/domain/invocation"
)

func withComposeSeams(t *testing.T) {
	t.Helper()
	fakeDocker := filepath.Join(t.TempDir(), "docker.exe")
	if err := os.WriteFile(fakeDocker, nil, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_DOCKER_BIN", fakeDocker)
	old := composeExecCommandFn
	t.Cleanup(func() {
		composeExecCommandFn = old
	})
}

func successfulComposeTestCommand() *exec.Cmd {
	return successfulComposeTestCommandContext(context.Background())
}

func successfulComposeTestCommandContext(ctx context.Context) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/c", "exit 0")
	}
	if shell := os.Getenv("SHELL"); shell != "" {
		return exec.CommandContext(ctx, shell, "-c", "exit 0")
	}
	return exec.CommandContext(ctx, "sh", "-c", "exit 0")
}

func TestRunComposeLifecycleUpBuildsArgs(t *testing.T) {
	withComposeSeams(t)
	var gotName string
	var gotArgs []string
	composeExecCommandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return successfulComposeTestCommandContext(ctx)
	}
	err := RunComposeLifecycle(ComposeLifecycleOpts{
		Action:           "up",
		File:             "/wf/assets/compose/docker-compose.yml",
		Project:          "dockpipe-dev",
		ProjectDirectory: "/repo",
		Services:         []string{"proxy", "mcp"},
		Env:              []string{"MCP_HTTP_URL=http://127.0.0.1:8766", "DATABASE_URL=postgres://local"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotName != os.Getenv("DOCKPIPE_DOCKER_BIN") {
		t.Fatalf("expected docker command, got %q", gotName)
	}
	joined := strings.Join(gotArgs, " ")
	for _, want := range []string{
		"compose",
		"-p dockpipe-dev",
		"-f /wf/assets/compose/docker-compose.yml",
		"--project-directory /repo",
		"up -d --remove-orphans",
		"proxy",
		"mcp",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected args to contain %q, got %s", want, joined)
		}
	}
	merged := strings.Join(mergeComposeProcessEnv([]string{"MCP_HTTP_URL=http://127.0.0.1:8766", "DATABASE_URL=postgres://local"}), "\n")
	for _, want := range []string{"MCP_HTTP_URL=http://127.0.0.1:8766", "DATABASE_URL=postgres://local"} {
		if !strings.Contains(merged, want) {
			t.Fatalf("expected merged env to contain %q, got %s", want, merged)
		}
	}
}

func TestRunComposeLifecycleDefaultsProjectDirectoryFromFile(t *testing.T) {
	withComposeSeams(t)
	var gotArgs []string
	composeExecCommandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		return successfulComposeTestCommandContext(ctx)
	}
	if err := RunComposeLifecycle(ComposeLifecycleOpts{
		Action: "ps",
		File:   "/wf/assets/compose/docker-compose.yml",
	}); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(gotArgs, " ")
	wantProjectDir := "--project-directory " + filepath.FromSlash("/wf/assets/compose")
	if !strings.Contains(joined, wantProjectDir) {
		t.Fatalf("expected default project directory from compose file, got %s", joined)
	}
	if !strings.Contains(joined, " ps") {
		t.Fatalf("expected ps action, got %s", joined)
	}
}

func TestRunComposeLifecycleRejectsUnknownAction(t *testing.T) {
	withComposeSeams(t)
	if err := RunComposeLifecycle(ComposeLifecycleOpts{
		Action: "logs",
		File:   "/wf/assets/compose/docker-compose.yml",
	}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestMergeComposeProcessEnvOverridesKeys(t *testing.T) {
	t.Setenv("MCP_HTTP_URL", "http://old")
	merged := strings.Join(mergeComposeProcessEnv([]string{"MCP_HTTP_URL=http://new", "NEW_KEY=value"}), "\n")
	for _, want := range []string{"MCP_HTTP_URL=http://new", "NEW_KEY=value"} {
		if !strings.Contains(merged, want) {
			t.Fatalf("expected merged env to contain %q, got %s", want, merged)
		}
	}
}

func TestComposeAuthorityCapturesSelectionBeforeProcessEffects(t *testing.T) {
	withComposeSeams(t)
	calls := 0
	var executed *exec.Cmd
	composeExecCommandFn = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		calls++
		if strings.Contains(strings.Join(args, " "), "changed") {
			t.Fatalf("authorizer changed owned service selection: %q", args)
		}
		executed = successfulComposeTestCommandContext(ctx)
		return executed
	}
	options := ComposeLifecycleOpts{Action: "up", File: "/selected/compose.yml", Services: []string{"selected"}, Env: []string{"MODE=selected"}}
	denied := errors.New("compose denied")
	admission := NativeExecutionAdmission{Authorize: func(context.Context, invocation.Binding) error { return denied }}
	if err := RunComposeLifecycleWithAdmission(admission, options); !errors.Is(err, denied) || calls != 0 {
		t.Fatalf("compose denial lost: calls=%d, error=%v", calls, err)
	}
	admission.Authorize = func(_ context.Context, binding invocation.Binding) error {
		if len(binding.Digest) != 64 {
			t.Fatal("compose selection is not exactly bound")
		}
		options.Services[0] = "changed"
		options.Env[0] = "MODE=changed"
		return nil
	}
	if err := RunComposeLifecycleWithAdmission(admission, options); err != nil || calls != 1 {
		t.Fatalf("owned compose failed: calls=%d, error=%v", calls, err)
	}
	if !strings.Contains(strings.Join(executed.Env, "\n"), "MODE=selected") || strings.Contains(strings.Join(executed.Env, "\n"), "MODE=changed") {
		t.Fatal("authorizer changed owned Compose environment")
	}
	options.Services = []string{"--privileged"}
	if err := RunComposeLifecycleWithAdmission(NativeExecutionAdmission{}, options); err == nil || calls != 1 {
		t.Fatal("service option injection reached process construction")
	}
}

func TestComposeCancellationAndFailuresRemainFailures(t *testing.T) {
	withComposeSeams(t)
	options := ComposeLifecycleOpts{Action: "up", File: "/selected/compose.yml"}
	composeExecCommandFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return helperExitCommandContext(ctx, 7)
	}
	if err := RunComposeLifecycle(options); err == nil {
		t.Fatal("failed compose became success")
	}
	if runtime.GOOS == "windows" {
		t.Skip("POSIX process tree cancellation fixture")
	}
	composeExecCommandFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "sleep 30")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := RunComposeLifecycleWithAdmission(NativeExecutionAdmission{Context: ctx}, options); !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 2*time.Second {
		t.Fatalf("compose process tree escaped cancellation: %v", err)
	}
}
