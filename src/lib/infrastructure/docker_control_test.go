package infrastructure

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
)

func TestContainerControlRejectsUnknownActionsAndAmbiguousIDs(t *testing.T) {
	for _, request := range []ContainerControlRequest{
		{Action: "exec", Container: strings.Repeat("a", 64)},
		{Action: "stop", Container: "abc123"},
		{Action: "restart", Container: "--all"},
	} {
		if _, err := ControlContainer(context.Background(), request); err == nil {
			t.Fatal("invalid container control accepted")
		}
	}
}

func TestContainerControlAuthorizationPrecedesExecutableLookup(t *testing.T) {
	t.Setenv("DOCKPIPE_DOCKER_BIN", filepath.Join(t.TempDir(), "missing-docker"))
	denied := errors.New("container control denied")
	target, identity := nativeinvocation.Local()
	request := ContainerControlRequest{Action: "stop", Container: strings.Repeat("a", 64)}
	_, err := ControlContainerWithAuthority(context.Background(), request, target, identity, func(_ context.Context, binding invocation.Binding) error {
		if binding.Contract.Action != "stop" || len(binding.Digest) != 64 {
			t.Fatal("container control lacks exact binding")
		}
		return denied
	})
	if !errors.Is(err, denied) {
		t.Fatalf("denial lost: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ControlContainer(ctx, request); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled control reached executable lookup: %v", err)
	}
}
