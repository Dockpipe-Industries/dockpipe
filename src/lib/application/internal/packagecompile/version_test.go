package packagecompile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestCompileChildrenTrackOwnerVersionWithoutSourceChanges(t *testing.T) {
	for _, kind := range []string{"workflow", "resolver"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			owner := filepath.Join(root, "independent")
			source := filepath.Join(owner, "children", "child")
			if err := os.MkdirAll(source, 0o755); err != nil {
				t.Fatal(err)
			}
			write := func(path, content string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(root, "VERSION"), "9.0.0\n")
			write(filepath.Join(source, "config.yml"), "name: child\nsteps: []\n")
			write(filepath.Join(source, "package.yml"), "schema: 1\nname: child\nkind: "+kind+"\n")
			destination, err := infrastructure.PackagesWorkflowsDir(root)
			if kind == "resolver" {
				destination, err = infrastructure.PackagesResolversDir(root)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(destination, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, version := range []string{"1.2.3", "1.2.4"} {
				write(filepath.Join(owner, "package.yml"), "schema: 1\nname: owner\nkind: bundle\nversion: "+version+"\n")
				if kind == "workflow" {
					err = compileWorkflowOne(root, source, "child", false)
				} else {
					err = compileSingleResolverDir(root, destination, source, "child", "", "9.0.0", false)
				}
				if err != nil {
					t.Fatal(err)
				}
				archive := filepath.Join(destination, fmt.Sprintf("dockpipe-%s-child-%s.tar.gz", kind, version))
				manifest, err := packagebuild.ReadFileFromTarGz(archive, kind+"s/child/package.yml")
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(manifest), "version: "+version) {
					t.Fatalf("wrong inherited version: %s", manifest)
				}
				// A future archive timestamp proves version, not mtime, causes rebuild.
				future := time.Now().Add(time.Hour)
				if err := os.Chtimes(archive, future, future); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
