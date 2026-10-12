package containedexec

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// GoRoot resolves the selected installed toolchain instead of using the build
// machine's embedded GOROOT. Retained-binary campaigns pass their fingerprinted
// executable explicitly; ordinary go test exports its selected GOROOT.
// This only resolves paths. CombinedOutput and Measure still gate every child.
func GoRoot(t testing.TB) string {
	t.Helper()
	root, err := ToolchainRoot()
	if err != nil {
		t.Fatalf("resolve selected Go toolchain: %v", err)
	}
	return root
}

// ToolchainRoot is also used by the retained-artifact fingerprint so the bytes
// hashed for cache admission belong to the same toolchain used for compilation.
func ToolchainRoot() (string, error) {
	goBinary := os.Getenv("PIPELANG_TEST_GO")
	if goBinary == "" {
		if root := os.Getenv("GOROOT"); root != "" {
			name := "go"
			if runtime.GOOS == "windows" {
				name += ".exe"
			}
			goBinary = filepath.Join(root, "bin", name)
		} else {
			var err error
			goBinary, err = exec.LookPath("go")
			if err != nil {
				return "", err
			}
		}
	}
	if !filepath.IsAbs(goBinary) {
		return "", fmt.Errorf("selected Go executable must be absolute: %q", goBinary)
	}
	resolved, err := filepath.EvalSymlinks(goBinary)
	if err != nil {
		return "", fmt.Errorf("resolve selected Go executable: %w", err)
	}
	name := filepath.Base(resolved)
	if (name != "go" && name != "go.exe") || filepath.Base(filepath.Dir(resolved)) != "bin" {
		return "", fmt.Errorf("selected Go executable must belong to an installed toolchain: %q", resolved)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("selected Go executable is not a regular file: %q", resolved)
	}
	root := filepath.Dir(filepath.Dir(resolved))
	version, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		return "", fmt.Errorf("read selected Go version: %w", err)
	}
	firstLine, _, _ := strings.Cut(string(version), "\n")
	if strings.TrimSpace(firstLine) != runtime.Version() {
		return "", fmt.Errorf("selected Go toolchain %q differs from test binary %q", firstLine, runtime.Version())
	}
	return root, nil
}
