package application

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScriptContextPropagatesSelectedPackageManifest(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "package.yml")
	if err := os.WriteFile(manifest, []byte("schema: 1\nname: maintained\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "assets", "scripts", "run.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	env := scriptContextEnv(script)
	if env["DOCKPIPE_PACKAGE_ROOT"] != root || env["DOCKPIPE_PACKAGE_MANIFEST"] != manifest {
		t.Fatalf("package context = %#v", env)
	}
}
