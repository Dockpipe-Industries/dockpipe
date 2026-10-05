//go:build linux || darwin

package remotecmd

import (
	"path/filepath"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestInitializationDoesNotReplaceCredentials(t *testing.T) {
	root := filepath.Join(t.TempDir(), "remote")
	first, err := initialize(root, "127.0.0.1:47831")
	if err != nil {
		t.Fatal(err)
	}
	second, err := initialize(root, "127.0.0.1:47831")
	if err != nil || second.Token != first.Token {
		t.Fatalf("retry changed identity: %v", err)
	}
	if _, err := initialize(root, "0.0.0.0:47831"); err == nil {
		t.Fatal("accepted public plain HTTP listener")
	}
	if err := infrastructure.ValidatePrivatePath(filepath.Join(root, "operator.json"), false); err != nil {
		t.Fatal("operator file is not private")
	}
}
