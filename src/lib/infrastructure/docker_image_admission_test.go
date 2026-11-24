package infrastructure

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/domain/invocation"
)

func TestDockerImagesRejectBeforeExecutableDiscovery(t *testing.T) {
	withDockerSeams(t)
	dockerImageCommandFn = func(context.Context, string, ...string) *exec.Cmd {
		t.Fatal("rejected image operation reached command construction")
		return nil
	}
	denied := errors.New("image operation denied")
	admission := NativeExecutionAdmission{Authorize: func(_ context.Context, binding invocation.Binding) error {
		if len(binding.Digest) != 64 {
			t.Fatal("image selection is not exactly bound")
		}
		return denied
	}}
	for _, action := range []func() error{
		func() error { return DockerBuildWithAdmission(admission, "image", "/recipe", "/context") },
		func() error { return DockerPullWithAdmission(admission, "image") },
		func() error {
			_, err := DockerImageExistsWithAdmission(admission, "image")
			return err
		},
	} {
		if err := action(); !errors.Is(err, denied) {
			t.Fatalf("authorization error lost: %v", err)
		}
	}
	for _, image := range []string{"", "--help", "image\x00other"} {
		if err := DockerPull(image); err == nil {
			t.Fatal("invalid image accepted")
		}
	}
	if err := DockerBuild("image", "recipe", "--help"); err == nil {
		t.Fatal("option selected as build context")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := DockerPullWithAdmission(NativeExecutionAdmission{Context: ctx}, "image"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled image operation reached dispatch: %v", err)
	}
}

func TestDockerImageOperationsRetainCancellationAndExitSemantics(t *testing.T) {
	dockerReadFixture(t, "exit 9\n")
	if exists, err := DockerImageExists("image"); err != nil || exists {
		t.Fatalf("missing image changed contract: %v, %v", exists, err)
	}
	if err := DockerPull("image"); err == nil {
		t.Fatal("failed pull became success")
	}
	dockerReadFixture(t, "exec sleep 30\n")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	if exists, err := DockerImageExistsWithAdmission(NativeExecutionAdmission{Context: ctx}, "image"); !errors.Is(err, context.DeadlineExceeded) || exists || time.Since(started) > 2*time.Second {
		t.Fatalf("image cancellation became missing-image success: %v, %v", exists, err)
	}
}

func TestDockerVolumeReinitializationFailureStopsBeforeContainerRun(t *testing.T) {
	withDockerSeams(t)
	calls := 0
	execCommandFn = func(_ string, args ...string) *exec.Cmd {
		calls++
		if strings.Join(args, " ") != "volume rm selected-data" {
			t.Fatalf("container lifecycle continued after failed removal: %q", args)
		}
		return helperExitCommand(7)
	}
	directory := t.TempDir()
	input, err := os.Create(filepath.Join(directory, "input"))
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.Create(filepath.Join(directory, "output"))
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	code, err := RunContainer(RunOpts{Image: "image", WorkdirHost: directory, DataVolume: "selected-data", Reinit: true, Force: true, Stdin: input, Stdout: output, Stderr: output}, []string{"fixture"})
	if err == nil || code != 1 || calls != 1 {
		t.Fatalf("failed removal became success: code=%d, calls=%d, err=%v", code, calls, err)
	}
	text, err := os.ReadFile(output.Name())
	if err != nil || strings.Contains(string(text), "result=reinitialized") {
		t.Fatalf("false reinitialization receipt: %s (%v)", text, err)
	}
}

func TestHostCleanupAuthorityPrecedesMarkerIOAndFreezesScope(t *testing.T) {
	dockerProviderFixture(t)
	withDockerSeams(t)
	original := hostCleanupExecCommandFn
	t.Cleanup(func() { hostCleanupExecCommandFn = original })
	hostCleanupExecCommandFn = func(context.Context, string, ...string) *exec.Cmd {
		t.Fatal("denied cleanup reached process construction")
		return nil
	}
	root := t.TempDir()
	if err := os.MkdirAll(HostRunsDir(root), 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(HostRunsDir(root), "12345678.container")
	if err := os.WriteFile(marker, []byte("tracked"), 0600); err != nil {
		t.Fatal(err)
	}
	environment := []string{"DOCKPIPE_WORKDIR=" + root, "DOCKPIPE_RUN_ID=12345678"}
	denied := errors.New("cleanup denied")
	admission := NativeExecutionAdmission{Authorize: func(_ context.Context, binding invocation.Binding) error {
		if len(binding.Digest) != 64 {
			t.Fatal("cleanup scope is not bound")
		}
		return denied
	}}
	if err := ApplyHostCleanupWithAdmission(admission, environment); !errors.Is(err, denied) {
		t.Fatalf("cleanup denial lost: %v", err)
	}
	if actual, err := os.ReadFile(marker); err != nil || string(actual) != "tracked" {
		t.Fatalf("denied cleanup mutated marker: %q (%v)", actual, err)
	}
	// Authorizers cannot change the already captured cleanup workdir through a
	// caller-owned environment slice.
	calls := 0
	hostCleanupExecCommandFn = func(_ context.Context, _ string, args ...string) *exec.Cmd {
		calls++
		if strings.Join(args, " ") != "stop -t 10 tracked" {
			t.Fatalf("cleanup scope changed: %q", args)
		}
		return helperExitCommand(0)
	}
	admission.Authorize = func(context.Context, invocation.Binding) error {
		environment[0] = "DOCKPIPE_WORKDIR=" + filepath.Join(root, "other")
		return nil
	}
	if err := ApplyHostCleanupWithAdmission(admission, environment); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("captured cleanup scope was not executed: %d calls", calls)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("successful cleanup retained marker: %v", err)
	}
}

func TestDockerImageAuthorityCannotRedirectCapturedProviderOrEnvironment(t *testing.T) {
	withDockerSeams(t)
	selected := filepath.Join(t.TempDir(), "docker.exe")
	if err := os.WriteFile(selected, nil, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_DOCKER_BIN", selected)
	t.Setenv("DOCKER_HOST", "selected-endpoint")
	t.Setenv("DOCKER_CONTEXT", "selected-context")
	var command *exec.Cmd
	dockerImageCommandFn = func(ctx context.Context, name string, _ ...string) *exec.Cmd {
		if name != selected {
			t.Fatalf("authorizer redirected captured Docker executable: %q", name)
		}
		command = helperExitCommandContext(ctx, 0)
		return command
	}
	admission := NativeExecutionAdmission{Authorize: func(context.Context, invocation.Binding) error {
		t.Setenv("DOCKPIPE_DOCKER_BIN", filepath.Join(t.TempDir(), "other-docker"))
		t.Setenv("PATH", t.TempDir())
		t.Setenv("DOCKER_HOST", "changed-endpoint")
		t.Setenv("DOCKER_CONTEXT", "changed-context")
		return nil
	}}
	if err := DockerPullWithAdmission(admission, "image"); err != nil {
		t.Fatal(err)
	}
	if envGet(command.Env, "DOCKER_HOST") != "selected-endpoint" || envGet(command.Env, "DOCKER_CONTEXT") != "selected-context" {
		t.Fatal("authorizer redirected captured Docker endpoint or context")
	}
}

func dockerProviderFixture(t *testing.T) string {
	t.Helper()
	selected := filepath.Join(t.TempDir(), "docker.exe")
	if err := os.WriteFile(selected, nil, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_DOCKER_BIN", selected)
	return selected
}
