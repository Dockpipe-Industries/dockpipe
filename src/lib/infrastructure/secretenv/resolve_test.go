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

func loginFixture(t *testing.T, loginScript string, succeeds bool) string {
	t.Helper()
	script := `printf '%s' '{"schema":"dockpipe.secret-environment/v1","error":"authentication_required"}'; exit 1`
	if succeeds {
		script = `if [ -f authenticated ]; then
  printf '%s' '{"schema":"dockpipe.secret-environment/v1","values":{"TOKEN":"fixture"}}'
else
  printf '%s' '{"schema":"dockpipe.secret-environment/v1","error":"authentication_required"}'
  exit 1
fi`
	}
	root := fixture(t, script)
	resolvers, err := infrastructure.PackagesResolversDir(root)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(resolvers, "test-secrets")
	profile := "DOCKPIPE_SECRET_ENVIRONMENT_RESOLVE=resolve.sh\nDOCKPIPE_SECRET_ENVIRONMENT_AUTHENTICATE=login.sh\n"
	if err := os.WriteFile(filepath.Join(directory, "profile"), []byte(profile), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "login.sh"), []byte(loginScript), 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func setInteractive(t *testing.T, enabled bool) {
	t.Helper()
	original := interactiveLogin
	interactiveLogin = func() bool { return enabled }
	t.Cleanup(func() { interactiveLogin = original })
}

func TestLoginResolvesAfterOneInteractiveAttempt(t *testing.T) {
	setInteractive(t, true)
	request := contract.Request{Schema: contract.Schema, Bindings: map[string]string{"TOKEN": "key"}}
	for _, succeeds := range []bool{true, false} {
		root := loginFixture(t, "printf attempt >> attempts\ntouch authenticated", succeeds)
		values, err := Resolve(context.Background(), root, "test-secrets", request)
		if succeeds && (err != nil || values["TOKEN"] != "fixture") {
			t.Fatalf("resolution did not resume: %v", err)
		}
		if !succeeds && !errors.Is(err, contract.ErrAuthenticationRequired) {
			t.Fatalf("login failure not preserved: %v", err)
		}
		attempts, err := os.ReadFile(filepath.Join(root, "attempts"))
		if err != nil || string(attempts) != "attempt" {
			t.Fatal("authentication looped or failed to run")
		}
	}
}

func TestNonInteractiveResolutionNeverLaunchesLogin(t *testing.T) {
	setInteractive(t, false)
	root := loginFixture(t, "touch unexpected-login", true)
	_, err := Resolve(context.Background(), root, "test-secrets", contract.Request{Schema: contract.Schema, Bindings: map[string]string{"TOKEN": "key"}})
	if !errors.Is(err, contract.ErrAuthenticationRequired) {
		t.Fatalf("missing actionable authentication error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "unexpected-login")); !os.IsNotExist(err) {
		t.Fatal("unattended authentication opened login")
	}
}

func TestLoginCancellationStopsBeforeResolution(t *testing.T) {
	setInteractive(t, true)
	root := loginFixture(t, "exec sleep 30", true)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := Resolve(ctx, root, "test-secrets", contract.Request{Schema: contract.Schema, Bindings: map[string]string{"TOKEN": "key"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("login cancellation not propagated: %v", err)
	}
}

func TestAuthenticationHookCannotEscapePackage(t *testing.T) {
	setInteractive(t, true)
	root := loginFixture(t, "exit 0", true)
	resolvers, err := infrastructure.PackagesResolversDir(root)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(resolvers, "test-secrets")
	if err := os.Remove(filepath.Join(directory, "login.sh")); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "outside.sh")
	if err := os.WriteFile(out, []byte("touch unexpected-login"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(out, filepath.Join(directory, "login.sh")); err != nil {
		t.Skip(err)
	}
	_, err = Resolve(context.Background(), root, "test-secrets", contract.Request{Schema: contract.Schema, Bindings: map[string]string{"TOKEN": "key"}})
	if err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("escaping authentication hook accepted: %v", err)
	}
}

func TestLoginRequiresTerminalOutsideCI(t *testing.T) {
	for _, ci := range []string{"true", "1", "yes", "custom"} {
		if loginAllowed(ci, true, true) {
			t.Fatal("CI allowed interactive login")
		}
	}
	if loginAllowed("", false, true) || loginAllowed("", true, false) || !loginAllowed("", true, true) {
		t.Fatal("terminal login policy is incorrect")
	}
}
