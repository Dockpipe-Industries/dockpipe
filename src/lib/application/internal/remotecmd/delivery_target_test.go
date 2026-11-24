package remotecmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCapturedDeliveryFlagsFailBeforeStateOrSourceAccess(t *testing.T) {
	for _, arguments := range [][]string{
		{"nodes", "--expected-fingerprint", strings.Repeat("a", 64), "--expected-generation", strings.Repeat("b", 64)},
		{"submit", "--node", "worker", "--workflow-file", "missing.yml", "--expected-fingerprint", strings.Repeat("a", 64)},
		{"submit", "--node", "worker", "--profile", "build", "--expected-fingerprint", strings.Repeat("a", 64), "--expected-generation", strings.Repeat("b", 64)},
		{"submit", "--node", "worker", "--workflow-file", "missing.yml", "--expected-root", "project"},
	} {
		state := filepath.Join(t.TempDir(), "uninitialized")
		arguments = append(arguments, "--state", state, "--auth", "never")
		if err := Run(arguments, nil); err == nil || !strings.Contains(err.Error(), "captured target flags") {
			t.Fatalf("accepted incomplete/unrelated target selection %v: %v", arguments, err)
		}
		if _, err := os.Stat(state); !os.IsNotExist(err) {
			t.Fatal("invalid captured target touched remote state", err)
		}
	}
}
