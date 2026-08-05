package keymaterial

import (
	"os"
	"testing"
)

func TestGeneratePerInstanceOwnerOnlyKey(t *testing.T) {
	root := t.TempDir()
	pair, err := Generate(root, "controller")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(pair.PrivatePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("private key mode = %o", info.Mode().Perm())
	}
	if _, err := Generate(root, "controller"); err == nil {
		t.Fatal("expected existing per-instance key rejection")
	}
	if _, err := Generate("relative", "bad"); err == nil {
		t.Fatal("expected relative key root rejection")
	}
}
