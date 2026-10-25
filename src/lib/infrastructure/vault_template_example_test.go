package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVaultTemplateExamplePreservesOperatorFile(t *testing.T) {
	root := t.TempDir()
	if err := WriteVaultTemplateExampleIfMissing(root); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, ".env.vault.template.example")
	example, err := os.ReadFile(destination)
	if err != nil || !strings.Contains(string(example), "op://YourVault/YourItem/token") {
		t.Fatalf("reference example = %q, %v", example, err)
	}
	original := []byte("operator-authored example\n")
	if err := os.WriteFile(destination, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteVaultTemplateExampleIfMissing(root); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(destination)
	if err != nil || string(actual) != string(original) {
		t.Fatalf("operator file changed: %q, %v", actual, err)
	}
}
