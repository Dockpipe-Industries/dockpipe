package containedexec

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func testToolchain(t *testing.T, version string) (string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	name := "go"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(root, "bin", name)
	if err := os.WriteFile(binary, []byte("test fixture, never executed"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte(version+"\ntime test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, binary
}

func TestToolchainRootUsesExplicitCampaignSelection(t *testing.T) {
	root, binary := testToolchain(t, runtime.Version())
	t.Setenv("PIPELANG_TEST_GO", binary)
	t.Setenv("GOROOT", t.TempDir())
	got, err := ToolchainRoot()
	if err != nil || got != root {
		t.Fatalf("selected root = %q, %v; want %q", got, err, root)
	}
}

func TestToolchainRootUsesGoTestEnvironment(t *testing.T) {
	root, _ := testToolchain(t, runtime.Version())
	t.Setenv("PIPELANG_TEST_GO", "")
	t.Setenv("GOROOT", root)
	got, err := ToolchainRoot()
	if err != nil || got != root {
		t.Fatalf("selected root = %q, %v; want %q", got, err, root)
	}
}

func TestToolchainRootRejectsMismatchAndRelativeSelection(t *testing.T) {
	_, binary := testToolchain(t, "go0.0.0")
	for _, selected := range []string{binary, "bin/go"} {
		t.Setenv("PIPELANG_TEST_GO", selected)
		if root, err := ToolchainRoot(); err == nil {
			t.Fatalf("accepted invalid selection %q: %q", selected, root)
		}
	}
}
