package remotecmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNetworkInputRequiresBoundedNamedPrivateFile(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "intent.json")
	if err := os.WriteFile(path, []byte(`{"schema":"dockpipe.remote.network/v1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readNetworkInput(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if _, err := readNetworkInput(context.Background(), "-"); err == nil {
		t.Fatal("stdin accepted despite lifetime ownership")
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readNetworkInput(context.Background(), path); err == nil {
		t.Fatal("shared request accepted")
	}
	if err := os.WriteFile(path, make([]byte, 4097), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readNetworkInput(context.Background(), path); err == nil {
		t.Fatal("oversized request accepted")
	}
}
