//go:build !linux && !darwin

package remotecmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitializationRejectsUnsupportedPlatformWithoutCredentials(t *testing.T) {
	root := filepath.Join(t.TempDir(), "remote")
	_, err := initialize(root, "127.0.0.1:47831")
	if err == nil || err.Error() != "remote nodes currently require Linux or macOS" {
		t.Fatalf("expected unsupported-platform rejection, got %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("unsupported initialization wrote credentials or state: %v, %v", entries, err)
	}
}
