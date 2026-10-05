package secretenv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/secretenv"
	"dockpipe/src/lib/infrastructure"
)

func fixture(t *testing.T, script string) string {
	t.Helper()
	root := t.TempDir()
	resolvers, err := infrastructure.PackagesResolversDir(root)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(resolvers, "test-secrets")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "profile"), []byte("DOCKPIPE_SECRET_ENVIRONMENT_RESOLVE=resolve.sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "resolve.sh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestResolverSuppressesSecretsAndRequiresExactBindings(t *testing.T) {
	request := contract.Request{Schema: contract.Schema, Bindings: map[string]string{"TOKEN": "remote"}}
	for _, script := range []string{
		"echo LEAK_SENTINEL >&2; exit 1\n",
		"echo LEAK_SENTINEL\n",
		`printf '%s' '{"schema":"dockpipe.secret-environment/v1","values":{"OTHER":"LEAK_SENTINEL"}}'`,
	} {
		values, err := Resolve(context.Background(), fixture(t, script), "test-secrets", request)
		if err == nil || len(values) != 0 || strings.Contains(err.Error(), "LEAK_SENTINEL") {
			t.Fatal("invalid provider response escaped")
		}
	}
	values, err := Resolve(context.Background(), fixture(t, `printf '%s' '{"schema":"dockpipe.secret-environment/v1","values":{"TOKEN":"line1\nline2"}}'`), "test-secrets", request)
	if err != nil || values["TOKEN"] != "line1\nline2" {
		t.Fatalf("multiline value did not round trip: %v", err)
	}
}

func TestResolverCancellation(t *testing.T) {
	root := fixture(t, "exec sleep 30\n")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := Resolve(ctx, root, "test-secrets", contract.Request{Schema: contract.Schema, Bindings: map[string]string{"TOKEN": "key"}})
	if err != context.DeadlineExceeded {
		t.Fatalf("cancellation not propagated: %v", err)
	}
}

func TestResolverCancellationStopsDescendants(t *testing.T) {
	root := fixture(t, "(sleep 0.3; printf done > child-finished) >/dev/null 2>&1 &\nwait\n")
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	_, err := Resolve(ctx, root, "test-secrets", contract.Request{Schema: contract.Schema, Bindings: map[string]string{"TOKEN": "key"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected resolution result: %v", err)
	}
	marker := filepath.Join(root, "child-finished")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("child finished before cancellation")
	}
	time.Sleep(500 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("resolver descendant survived cancellation: %v", err)
	}
}
