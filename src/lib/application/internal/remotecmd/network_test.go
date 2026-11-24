package remotecmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func TestNetworkInputRequiresBoundedNamedPrivateFile(t *testing.T) {
	root := privateConnectionDirectory(t)
	path := filepath.Join(root, "intent.json")
	if err := remoteio.WritePrivate(path, map[string]string{"schema": "dockpipe.remote.network/v1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := readNetworkInput(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if _, err := readNetworkInput(context.Background(), "-"); err == nil {
		t.Fatal("stdin accepted despite lifetime ownership")
	}

	shared := filepath.Join(t.TempDir(), "shared.json")
	if err := os.WriteFile(shared, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(shared, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readNetworkInput(context.Background(), shared); err == nil {
		t.Fatal("shared request accepted")
	}

	if err := remoteio.WritePrivate(path, strings.Repeat("a", 4097)); err != nil {
		t.Fatal(err)
	}
	if _, err := readNetworkInput(context.Background(), path); err == nil {
		t.Fatal("oversized request accepted")
	}
}
