//go:build !linux && !darwin

package cloudflare

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSetupRejectsUnsupportedPlatformBeforeProviderCommands(t *testing.T) {
	root := t.TempDir()
	setup := Setup{
		State:      filepath.Join(root, "edge"),
		Output:     filepath.Join(root, "edge.json"),
		Home:       root,
		Executable: filepath.Join(root, "cloudflared"),
		Hostname:   "bench.example.com",
		Origin:     "http://127.0.0.1:47831",
		Run: func(context.Context, string, []string, bool) error {
			t.Fatal("unsupported setup invoked the provider")
			return nil
		},
	}
	for attempt := 0; attempt < 2; attempt++ {
		err := setup.Execute(context.Background())
		if err == nil || err.Error() != "remote nodes currently require Linux or macOS" {
			t.Fatalf("expected unsupported-platform rejection, got %v", err)
		}
	}
	if _, err := os.Stat(setup.Output); !os.IsNotExist(err) {
		t.Fatalf("unsupported setup published an edge: %v", err)
	}
	entries, err := os.ReadDir(setup.State)
	if err != nil || len(entries) != 0 {
		t.Fatalf("unsupported setup wrote recovery state: %v, %v", entries, err)
	}
}
