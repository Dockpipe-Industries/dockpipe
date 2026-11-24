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

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/model/managedenvironment"
)

func TestDockerProviderDiscoveryUsesCapturedEnvironment(t *testing.T) {
	directory := t.TempDir()
	name := "docker"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	selected := filepath.Join(directory, name)
	if err := os.WriteFile(selected, nil, 0755); err != nil {
		t.Fatal(err)
	}
	captured := []string{"PATH=" + directory, "PATHEXT=.exe"}
	t.Setenv("PATH", t.TempDir())
	t.Setenv("DOCKPIPE_DOCKER_BIN", filepath.Join(t.TempDir(), "other-docker"))
	actual, err := resolveDockerCommandFromEnvironment(captured)
	if err != nil || actual != selected {
		t.Fatalf("ambient environment redirected discovery: %q, %v", actual, err)
	}
	fallback, fallbackErr := resolveDockerCommandFromEnvironment(nil)
	actual, err = resolveDockerCommandFromEnvironment([]string{"PATH=."})
	if actual != fallback || errors.Is(err, exec.ErrNotFound) != errors.Is(fallbackErr, exec.ErrNotFound) {
		t.Fatalf("relative PATH changed Docker fallback: got %q (%v), want %q (%v)", actual, err, fallback, fallbackErr)
	}
}

func TestDockerEnvironmentApplyRequiresExactRemoteAuthorityBeforeDockerLookup(t *testing.T) {
	t.Setenv("DOCKPIPE_DOCKER_BIN", filepath.Join(t.TempDir(), "missing-docker"))
	request := DockerEnvironmentApplyRequest{
		Name:       "dev-shell",
		InstanceID: strings.Repeat("a", 32),
		Settings:   managedenvironment.Container{Image: "ubuntu:24.04"},
	}
	target := invocation.Target{
		Kind: invocation.Remote, ID: "paired-host",
		Fingerprint: strings.Repeat("b", 64), Generation: strings.Repeat("c", 64),
	}
	identity := invocation.Identity{Issuer: "fixture", Subject: "operator"}
	ctx := context.Background()
	_, err := ApplyDockerEnvironmentWithAuthority(ctx, request, target, identity, nil)
	if !errors.Is(err, invocation.ErrAuthorizationUnavailable) {
		t.Fatalf("remote apply without authority = %v", err)
	}

	denied := errors.New("apply denied")
	var digests []string
	authorize := func(_ context.Context, binding invocation.Binding) error {
		if binding.Call.Target != target || binding.Call.Identity != identity ||
			binding.Call.Contract.Capability != "dockpipe.environments" ||
			binding.Call.Contract.Action != "container-apply" || len(binding.Digest) != 64 {
			t.Fatalf("unexpected authorization binding: %#v", binding)
		}
		digests = append(digests, binding.Digest)
		return denied
	}

	_, err = ApplyDockerEnvironmentWithAuthority(ctx, request, target, identity, authorize)
	if !errors.Is(err, denied) {
		t.Fatalf("authority denial did not precede Docker discovery: %v", err)
	}
	request.Settings.ReadOnly = true
	_, err = ApplyDockerEnvironmentWithAuthority(ctx, request, target, identity, authorize)
	if !errors.Is(err, denied) || len(digests) != 2 || digests[0] == digests[1] {
		t.Fatalf("changed settings were not separately bound: %v, %v", err, digests)
	}
}
