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

func TestGenerateRejectsExistingPublicKeyWithoutCreatingPrivateKey(t *testing.T) {
	root := t.TempDir()
	publicPath := root + "/guest.pub"
	if err := os.WriteFile(publicPath, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(root, "guest"); err == nil {
		t.Fatal("expected existing public-key rejection")
	}
	if _, err := os.Stat(root + "/guest.key"); !os.IsNotExist(err) {
		t.Fatal("private key was created after public-key collision")
	}
	b, _ := os.ReadFile(publicPath)
	if string(b) != "existing" {
		t.Fatal("existing public key was replaced")
	}
}
