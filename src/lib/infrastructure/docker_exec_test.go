package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/domain/invocation"
)

func execTestStreams(t *testing.T) (*os.File, *os.File, *os.File) {
	t.Helper()
	files := make([]*os.File, 0, 3)
	for _, name := range []string{"input", "output", "errors"} {
		file, err := os.Create(filepath.Join(t.TempDir(), name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = file.Close() })
		files = append(files, file)
	}
	return files[0], files[1], files[2]
}

func TestContainerExecRejectsInvalidSelectionAndDeniedAuthorityBeforeEffects(t *testing.T) {
	dockerReadFixture(t, `printf unexpected > "$CONTAINER_EXEC_MARKER"`)
	marker := filepath.Join(t.TempDir(), "started")
	t.Setenv("CONTAINER_EXEC_MARKER", marker)
	stdin, stdout, stderr := execTestStreams(t)
	id := strings.Repeat("a", 64)
	for _, request := range []ContainerExecRequest{
		{Container: "short", Shell: "/bin/sh"},
		{Container: "--privileged", Shell: "/bin/sh"},
		{Container: id, Shell: "/usr/bin/python"},
		{Container: id, Shell: "/bin/sh -c code"},
	} {
		if err := ExecLocalContainerWithAdmission(NativeExecutionAdmission{}, request, stdin, stdout, stderr); err == nil {
			t.Fatalf("invalid container session accepted: %+v", request)
		}
	}
	denied := errors.New("container shell denied")
	admission := NativeExecutionAdmission{Authorize: func(_ context.Context, binding invocation.Binding) error {
		if binding.Contract.Capability != "dockpipe.containers" || binding.Contract.Action != "exec" || len(binding.Digest) != 64 {
			t.Fatal("interactive session lacks typed authority binding")
		}
		return denied
	}}
	request := ContainerExecRequest{Container: id, Shell: "/bin/sh"}
	if err := ExecLocalContainerWithAdmission(admission, request, stdin, stdout, stderr); !errors.Is(err, denied) {
		t.Fatalf("denied exec became another outcome: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ExecLocalContainerWithAdmission(NativeExecutionAdmission{Context: ctx}, request, stdin, stdout, stderr); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled exec reached provider: %v", err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rejected exec performed process effects: %v", err)
	}
}

func TestContainerExecFreezesProviderEnvironmentAndInheritsStreams(t *testing.T) {
	id := strings.Repeat("a", 64)
	dockerReadFixture(t, `test "$#" = 5 && test "$1:$2:$3:$4:$5" = 'container:exec:-i:`+id+`:/bin/bash' || exit 8
read line
printf 'input:%s endpoint:%s' "$line" "$DOCKER_HOST"
printf diagnostic >&2
`)
	stdin, stdout, stderr := execTestStreams(t)
	if _, err := stdin.WriteString("interactive fixture\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := stdin.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKER_HOST", "selected-endpoint")
	admission := NativeExecutionAdmission{Authorize: func(context.Context, invocation.Binding) error {
		t.Setenv("DOCKPIPE_DOCKER_BIN", filepath.Join(t.TempDir(), "other-docker"))
		t.Setenv("DOCKER_HOST", "changed-endpoint")
		return nil
	}}
	if err := ExecLocalContainerWithAdmission(admission, ContainerExecRequest{Container: id, Shell: "/bin/bash"}, stdin, stdout, stderr); err != nil {
		t.Fatal(err)
	}
	output, err := os.ReadFile(stdout.Name())
	if err != nil || string(output) != "input:interactive fixture endpoint:selected-endpoint" {
		t.Fatalf("stream/provider selection changed: %q (%v)", output, err)
	}
	diagnostics, err := os.ReadFile(stderr.Name())
	if err != nil || string(diagnostics) != "diagnostic" {
		t.Fatalf("stderr was not inherited: %q (%v)", diagnostics, err)
	}
}

func TestContainerExecCancellationAndFailedExitRemainFailures(t *testing.T) {
	for _, script := range []string{"exit 7\n", "sleep 30\n"} {
		t.Run(script, func(t *testing.T) {
			dockerReadFixture(t, script)
			stdin, stdout, stderr := execTestStreams(t)
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			started := time.Now()
			err := ExecLocalContainerWithAdmission(NativeExecutionAdmission{Context: ctx}, ContainerExecRequest{Container: strings.Repeat("a", 64), Shell: "/bin/sh"}, stdin, stdout, stderr)
			if err == nil || time.Since(started) > 2*time.Second {
				t.Fatalf("failed exec escaped its lifecycle: %v", err)
			}
		})
	}
}
