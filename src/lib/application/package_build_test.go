package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdPackageBuildCoreIsRetired(t *testing.T) {
	if err := cmdPackageBuild([]string{"core"}); err == nil {
		t.Fatal("retired core release producer must not remain available")
	}
}

func TestRunPackageBuildStoreAllowsEmptyStore(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(t.TempDir(), "store")
	stderr, err := captureResultStderr(t, func() error {
		return RunPackageBuildStoreFromFlags(root, output, "all", "9.9.9")
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"unit=package.build.store", "status=done", "slice=all", "count=0", "result=built", "packages-store-manifest.json"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected %q in stderr:\n%s", want, stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(output, "packages-store-manifest.json")); err != nil {
		t.Fatal(err)
	}
}
