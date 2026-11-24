package remotecmd

import (
	"context"
	"errors"
	"strings"
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

func TestRemoteTimeoutFlagsRequireSecurityPolicy(t *testing.T) {
	for _, flag := range []string{"--idle-seconds", "--absolute-seconds"} {
		err := Run([]string{"nodes", flag, "43200"}, nil)
		if err == nil || !strings.Contains(err.Error(), "supported only by remote security-policy") {
			t.Fatalf("policy-only flag %s did not fail before a remote operation: %v", flag, err)
		}
	}
}
