package repotools

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPackageScopePassesThroughDurableCLIContract(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell executable")
	}
	base := t.TempDir()
	bin := filepath.Join(base, "dockpipe")
	script := `#!/bin/sh
case "$*" in
  *"--package maintained-owner child/file"*) printf '%s\n' '/durable/package/child/file' ;;
  *"--package maintained-owner"*) printf '%s\n' '{"kind":"package","name":"maintained-owner","scope":"package:maintained-owner","root":"/durable/package","state_root":"/durable/package","workdir":"/fixture"}' ;;
  *) exit 23 ;;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	sdk := SDK{DockpipeBin: bin, Workdir: filepath.Join(base, "checkout")}
	object, err := sdk.PackageScope("maintained-owner")
	if err != nil {
		t.Fatal(err)
	}
	if object.Root != "/durable/package" || object.StateRoot != object.Root {
		t.Fatalf("package scope did not preserve durable root: %+v", object)
	}
	path, err := sdk.PackageScopePath("maintained-owner", "child/file")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/durable/package/child/file" {
		t.Fatalf("package scope path = %q", path)
	}
}
