package nativehost

import (
	"context"
	"errors"
	"os"
	"os/user"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Positive worker integration requires a real, unprivileged account. A mapped
// container UID without an NSS entry must exercise rejection instead of gaining
// a fabricated username, home directory or authority through the test fixture.
func workerIdentityForTest(t *testing.T) (Status, bool) {
	t.Helper()
	identity, identityErr := workerIdentity()
	if os.Getuid() == 0 {
		if identityErr == nil || identityErr.Error() != "native host worker requires an ordinary user" {
			t.Fatalf("root worker admission = %+v, %v", identity, identityErr)
		}
		return Status{}, false
	}
	account, lookupErr := user.LookupId(strconv.Itoa(os.Getuid()))
	if lookupErr != nil {
		var unknown user.UnknownUserIdError
		if !errors.As(lookupErr, &unknown) {
			t.Fatalf("unexpected native worker account lookup failure: %v", lookupErr)
		}
		if identityErr == nil || identityErr.Error() != "native host worker account unavailable" || identity.Schema != "" {
			t.Fatalf("nameless UID worker admission = %+v, %v", identity, identityErr)
		}
		return Status{}, false
	}
	if identityErr != nil {
		t.Fatal(identityErr)
	}
	if identity.Username != account.Username || identity.Home != account.HomeDir {
		t.Fatalf("worker identity differs from the actual account: %+v", identity)
	}
	return identity, true
}

func requireTestWorkerIdentity(t *testing.T) Status {
	t.Helper()
	identity, admitted := workerIdentityForTest(t)
	if !admitted {
		t.Skip("positive worker integration requires an ordinary UID with an actual account; rejection is covered by TestWorkerIdentityEnvironmentAdmission")
	}
	return identity
}

func TestWorkerIdentityEnvironmentAdmission(t *testing.T) {
	identity, admitted := workerIdentityForTest(t)
	if admitted {
		t.Logf("worker admitted with actual account %q", identity.Username)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ready := false
	options := WorkerOptions{
		Socket: "/tmp/native-host-unadmitted-worker.sock",
		Nonce:  strings.Repeat("a", 64),
		Ready: func() error {
			ready = true
			return nil
		},
	}
	_, expectedErr := workerIdentity()
	if err := RunWorker(ctx, options); err == nil || err.Error() != expectedErr.Error() {
		t.Fatalf("unadmitted worker startup error = %v, want %v", err, expectedErr)
	}
	if ready {
		t.Fatal("unadmitted worker announced readiness")
	}
	if err := DetachWorker(ctx, options); err == nil || err.Error() != expectedErr.Error() {
		t.Fatalf("unadmitted detached worker startup error = %v, want %v", err, expectedErr)
	}
	t.Logf("worker and detached startup reject the actual unsupported identity: %v", expectedErr)
}
