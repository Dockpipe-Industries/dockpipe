//go:build linux || darwin

package cloudflare

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func TestBrowserSetupRecoveryAndRuntimeCredentialSeparation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	if err := remoteio.PrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	setup := Setup{State: filepath.Join(root, "edge"), Output: filepath.Join(root, "edge.json"), Home: home, Executable: filepath.Join(root, "cloudflared"), Hostname: "bench.example.com", Origin: "http://127.0.0.1:47831"}
	var progress bytes.Buffer
	setup.Progress = &progress
	var calls [][]string
	setup.Run = func(ctx context.Context, executable string, args []string, interactive bool) error {
		calls = append(calls, append([]string{}, args...))
		switch {
		case reflect.DeepEqual(args, []string{"tunnel", "login"}):
			if !interactive {
				t.Fatal("login must use the real browser flow")
			}
			if err := remoteio.PrivateDirectory(filepath.Join(home, ".cloudflared")); err != nil {
				return err
			}
			return remoteio.WritePrivate(filepath.Join(home, ".cloudflared", "cert.pem"), "account-secret")
		case len(args) == 7 && args[3] == "create":
			if interactive || args[4] != "--credentials-file" {
				t.Fatal(args)
			}
			return remoteio.WritePrivate(args[5], map[string]string{"TunnelID": "01234567-89ab-cdef-0123-456789abcdef", "TunnelSecret": "worker-secret"})
		case len(args) == 7 && args[3] == "route":
			if strings.Contains(strings.Join(args, " "), "overwrite") {
				t.Fatal("DNS overwrite requested")
			}
			return nil
		default:
			t.Fatalf("unexpected cloudflared invocation %v", args)
			return nil
		}
	}
	if err := setup.Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := setup.Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(progress.String(), "login URL") || !strings.Contains(progress.String(), "browser login is not needed") {
		t.Fatal("missing login and reuse guidance")
	}
	if len(calls) != 3 {
		t.Fatalf("setup replayed mutations: %v", calls)
	}
	var edge contract.EdgeConfig
	if err := remoteio.ReadPrivate(setup.Output, &edge); err != nil {
		t.Fatal(err)
	}
	if edge.Endpoint != "https://bench.example.com" || strings.Contains(strings.Join(edge.Arguments, " "), "cert.pem") {
		t.Fatalf("runtime contains management credential: %+v", edge)
	}
	for _, path := range []string{setup.Output, filepath.Join(setup.State, "tunnel-config.json")} {
		raw, _ := os.ReadFile(path)
		if strings.Contains(string(raw), "worker-secret") || strings.Contains(string(raw), "account-secret") {
			t.Fatal("credential leaked into configuration")
		}
	}
}

func TestUnknownTunnelCreateIsNotRetried(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	if err := remoteio.PrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	if err := remoteio.PrivateDirectory(filepath.Join(home, ".cloudflared")); err != nil {
		t.Fatal(err)
	}
	if err := remoteio.WritePrivate(filepath.Join(home, ".cloudflared", "cert.pem"), "cert"); err != nil {
		t.Fatal(err)
	}
	count := 0
	setup := Setup{State: filepath.Join(root, "edge"), Output: filepath.Join(root, "edge.json"), Home: home, Executable: filepath.Join(root, "cloudflared"), Hostname: "bench.example.com", Origin: "http://127.0.0.1:47831"}
	setup.Run = func(context.Context, string, []string, bool) error { count++; return errors.New("response lost") }
	if err := setup.Execute(context.Background()); err == nil {
		t.Fatal("failed create passed")
	}
	if err := setup.Execute(context.Background()); err == nil {
		t.Fatal("unknown create was retried")
	}
	if count != 1 {
		t.Fatalf("repeated provider mutation %d times", count)
	}
	if _, err := os.Stat(setup.Output); !os.IsNotExist(err) {
		t.Fatal("published incomplete setup")
	}
}

func TestLoginRunnerPreservesBrowserSessionWithoutCredentials(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "browser-opened")
	browser := filepath.Join(root, "browser")
	// Simulate cloudflared's browser handoff with the inherited desktop session.
	script := "#!/bin/sh\n[ \"$XAUTHORITY\" = expected-xauth ] || exit 11\n[ \"$XDG_CURRENT_DESKTOP\" = expected-desktop ] || exit 12\n[ -z \"$CLOUDFLARE_API_TOKEN\" ] || exit 13\nprintf opened > \"$1\"\n"
	if err := os.WriteFile(browser, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(root, "cloudflared")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexec \"$BROWSER\" \"$1\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BROWSER", browser)
	t.Setenv("XAUTHORITY", "expected-xauth")
	t.Setenv("XDG_CURRENT_DESKTOP", "expected-desktop")
	t.Setenv("CLOUDFLARE_API_TOKEN", "must-not-inherit")
	if err := runCloudflared(context.Background(), fake, []string{marker}, true); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "opened" {
		t.Fatalf("browser handoff failed: %q %v", data, err)
	}
}
