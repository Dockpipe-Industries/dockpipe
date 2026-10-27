package remotecmd

import (
	"context"
	"errors"
	"testing"

	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func TestBackgroundRemoteActionsDoNotOpenOSAuthentication(t *testing.T) {
	for _, ctx := range []context.Context{
		context.Background(),
		context.WithValue(context.Background(), remoteAuthenticationKey{}, false),
	} {
		if err := reauthenticateRemote(ctx); !errors.Is(err, remoteio.ErrOperatorLocked) {
			t.Fatalf("noninteractive operation attempted native authentication: %v", err)
		}
	}
}

func TestCancelledRemoteActionDoesNotOpenOSAuthentication(t *testing.T) {
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), remoteAuthenticationKey{}, true))
	cancel()
	if err := reauthenticateRemote(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled action opened native authentication: %v", err)
	}
}

func TestRemoteAuthenticationModeRejectsInvalidValue(t *testing.T) {
	if err := Run([]string{"nodes", "--auth", "sometimes"}, nil); err == nil {
		t.Fatal("invalid authentication mode accepted")
	}
}
