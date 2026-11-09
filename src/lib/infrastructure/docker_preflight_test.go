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

func TestDockerDoctorCheckUsesHook(t *testing.T) {
	old := dockerPreflightHook
	defer func() { dockerPreflightHook = old }()

	dockerPreflightHook = func(_ *os.File) error { return nil }
	t.Setenv("DOCKPIPE_SKIP_DOCKER_PREFLIGHT", "")
	if err := DockerDoctorCheck(nil); err != nil {
		t.Fatalf("hook: %v", err)
	}
}

func TestDockerDoctorCheckUsesConfiguredDockerBinary(t *testing.T) {
	oldHook := dockerPreflightHook
	oldExec := execCommandContextFn
	t.Cleanup(func() {
		dockerPreflightHook = oldHook
		execCommandContextFn = oldExec
	})
	dockerPreflightHook = nil
	fakeDocker := filepath.Join(t.TempDir(), "docker-test")
	if err := os.WriteFile(fakeDocker, []byte(""), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	t.Setenv("DOCKPIPE_DOCKER_BIN", fakeDocker)
	var gotName string
	execCommandContextFn = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		gotName = name
		return helperExitCommandContext(ctx, 0)
	}
	if err := DockerDoctorCheck(nil); err != nil {
		t.Fatalf("DockerDoctorCheck: %v", err)
	}
	if gotName != fakeDocker {
		t.Fatalf("expected configured docker binary %q, got %q", fakeDocker, gotName)
	}
}

func TestDockerPreflightAdmissionPrecedesHookAndExecutableDiscovery(t *testing.T) {
	oldHook := dockerPreflightHook
	t.Cleanup(func() { dockerPreflightHook = oldHook })
	calls := 0
	dockerPreflightHook = func(*os.File) error {
		calls++
		return nil
	}
	denied := errors.New("preflight denied")
	admission := NativeExecutionAdmission{Authorize: func(_ context.Context, binding invocation.Binding) error {
		if len(binding.Digest) != 64 {
			t.Fatal("preflight selection is not exactly bound")
		}
		return denied
	}}
	if err := DockerDoctorCheckWithAdmission(admission, nil); !errors.Is(err, denied) || calls != 0 {
		t.Fatalf("denied preflight reached hook: calls=%d, error=%v", calls, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := DockerDoctorCheckWithAdmission(NativeExecutionAdmission{Context: ctx}, nil); !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatalf("cancelled preflight reached hook: calls=%d, error=%v", calls, err)
	}
}

func TestDockerPreflightDiagnosticBoundAndProcessCancellation(t *testing.T) {
	oldHook := dockerPreflightHook
	t.Cleanup(func() { dockerPreflightHook = oldHook })
	dockerPreflightHook = nil
	dockerReadFixture(t, "head -c 524289 /dev/zero\n")
	if err := DockerDoctorCheck(nil); err == nil || !strings.Contains(err.Error(), "diagnostic limit") {
		t.Fatalf("oversized preflight output accepted: %v", err)
	}
	dockerReadFixture(t, "sleep 30\n")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := DockerDoctorCheckWithAdmission(NativeExecutionAdmission{Context: ctx}, nil); !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 2*time.Second {
		t.Fatalf("preflight process tree escaped cancellation: %v", err)
	}
}
