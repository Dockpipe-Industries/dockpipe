//go:build linux

package osauth

import (
	"path/filepath"
	"testing"
)

func TestSecurityDirectoryIgnoresProjectAndEnvironmentOverrides(t *testing.T) {
	before, err := securityDirectory()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"HOME", "XDG_STATE_HOME", "XDG_DATA_HOME", "DOCKPIPE_GLOBAL_ROOT", "DOCKPIPE_REMOTE_STATE"} {
		t.Setenv(name, "/tmp/untrusted-project-security-root")
	}
	after, err := securityDirectory()
	if err != nil || after != before || !filepath.IsAbs(after) {
		t.Fatalf("security owner followed project environment: %q %v", after, err)
	}
}
