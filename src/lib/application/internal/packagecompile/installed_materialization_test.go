package packagecompile

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestInstalledMaterializationDoesNotRecompileArchive(t *testing.T) {
	for _, kind := range []string{"workflow", "resolver"} {
		t.Run(kind, func(t *testing.T) {
			workdir := t.TempDir()
			source := t.TempDir()
			if err := os.WriteFile(filepath.Join(source, "config.yml"), []byte("name: example\nsteps: []\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			manifest := fmt.Sprintf("schema: 1\nname: example\nkind: %s\nversion: 1.2.3\nnamespace: example\n", kind)
			if err := os.WriteFile(filepath.Join(source, "package.yml"), []byte(manifest), 0o644); err != nil {
				t.Fatal(err)
			}
			destination, err := infrastructure.PackagesWorkflowsDir(workdir)
			if kind == "resolver" {
				destination, err = infrastructure.PackagesResolversDir(workdir)
			}
			if err != nil {
				t.Fatal(err)
			}
			compile := func(from string, force bool) error {
				if kind == "workflow" {
					return compileWorkflowOne(workdir, from, "example", force)
				}
				return compileSingleResolverDir(workdir, destination, from, "example", "example", "1.2.3", force)
			}
			if err := compile(source, true); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(destination, fmt.Sprintf("dockpipe-%s-example-1.2.3.tar.gz", kind))
			before, err := os.ReadFile(archive)
			if err != nil {
				t.Fatal(err)
			}
			beforeInfo, err := os.Stat(archive)
			if err != nil {
				t.Fatal(err)
			}
			extraction, err := packagebuild.EnsureTarballExtractedCache(archive, infrastructure.TarballExtractCacheRoot(workdir))
			if err != nil {
				t.Fatal(err)
			}
			derived := filepath.Join(extraction, kind+"s", "example")
			future := time.Now().Add(time.Hour)
			if err := os.Chtimes(filepath.Join(derived, "config.yml"), future, future); err != nil {
				t.Fatal(err)
			}
			stale, err := infrastructure.SourceDirNewerThanPath(derived, archive)
			if err != nil || !stale {
				t.Fatalf("fixture did not reproduce extraction mtime: stale=%v err=%v", stale, err)
			}
			if err := compile(derived, false); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(archive)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("installed archive changed during automatic materialization compile: %v", err)
			}
			afterInfo, err := os.Stat(archive)
			if err != nil || !os.SameFile(beforeInfo, afterInfo) || !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
				t.Fatalf("installed archive was rewritten during automatic materialization compile: %v", err)
			}
			marker := filepath.Join(extraction, ".dockpipe-extracted")
			validMarker, err := os.ReadFile(marker)
			if err != nil {
				t.Fatal(err)
			}
			assertUnchanged := func() {
				t.Helper()
				current, err := os.ReadFile(archive)
				if err != nil || !bytes.Equal(before, current) {
					t.Fatalf("invalid cache provenance changed archive bytes: %v", err)
				}
				currentInfo, err := os.Stat(archive)
				if err != nil || !os.SameFile(beforeInfo, currentInfo) || !beforeInfo.ModTime().Equal(currentInfo.ModTime()) {
					t.Fatalf("invalid cache provenance rewrote archive: %v", err)
				}
			}
			for _, invalid := range []string{"missing", "corrupt", "oversized", "directory"} {
				if err := os.Remove(marker); err != nil {
					t.Fatal(err)
				}
				switch invalid {
				case "corrupt":
					err = os.WriteFile(marker, []byte("invalid digest"), 0o644)
				case "oversized":
					err = os.WriteFile(marker, make([]byte, 1000), 0o644)
				case "directory":
					err = os.Mkdir(marker, 0o755)
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := compile(derived, false); err == nil || !strings.Contains(err.Error(), "provenance") {
					t.Fatalf("%s extraction marker did not fail at provenance boundary: %v", invalid, err)
				}
				assertUnchanged()
				if invalid != "missing" {
					if err := os.Remove(marker); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(marker, validMarker, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			wrongExtraction := filepath.Join(filepath.Dir(extraction), "wrong-digest")
			if err := os.Rename(extraction, wrongExtraction); err != nil {
				t.Fatal(err)
			}
			if err := compile(filepath.Join(wrongExtraction, kind+"s", "example"), false); err == nil || !strings.Contains(err.Error(), "provenance") {
				t.Fatalf("wrong extraction digest did not fail at provenance boundary: %v", err)
			}
			assertUnchanged()
			if err := os.Rename(wrongExtraction, extraction); err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(derived, "config.yml")
			validConfig, err := os.ReadFile(config)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(config, []byte("steps: [malformed"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := compile(derived, false); err == nil {
				t.Fatal("materialized source bypassed workflow validation")
			}
			if err := os.WriteFile(config, validConfig, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(derived, "forced.txt"), []byte("explicit rebuild"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(marker); err != nil {
				t.Fatal(err)
			}
			if err := compile(derived, true); err != nil {
				t.Fatalf("explicit forced compilation failed: %v", err)
			}
			forced, err := packagebuild.ReadFileFromTarGz(archive, kind+"s/example/forced.txt")
			if err != nil || string(forced) != "explicit rebuild" {
				t.Fatalf("explicit force did not rebuild extracted source: %q, %v", forced, err)
			}

			// An explicit source edit still recompiles; the cache is not a blanket
			// exemption for independent authored directories.
			if err := os.WriteFile(filepath.Join(source, "asset.txt"), []byte("authored change"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(filepath.Join(source, "asset.txt"), future, future); err != nil {
				t.Fatal(err)
			}
			if err := compile(source, false); err != nil {
				t.Fatal(err)
			}
			authored, err := os.ReadFile(archive)
			if err != nil || bytes.Equal(before, authored) {
				t.Fatalf("authored change did not rebuild archive: %v", err)
			}
		})
	}
}

func TestSourceHiddenNamedWorkflowClosurePreservesInstalledArchive(t *testing.T) {
	workdir := t.TempDir()
	bundle := t.TempDir()
	t.Setenv("DOCKPIPE_REPO_ROOT", bundle)
	if err := os.WriteFile(filepath.Join(workdir, "dockpipe.config.json"), []byte(`{"schema":1,"packages":{"namespace":"example"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "config.yml"), []byte("name: example\nnamespace: example\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "package.yml"), []byte("schema: 1\nname: example\nkind: workflow\nversion: 1.2.3\nnamespace: example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := compileWorkflowOne(workdir, source, "example", true); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	start, err := infrastructure.WorkflowCompileStartDir(bundle, workdir, "example")
	if err != nil {
		t.Fatal(err)
	}
	workflows, err := infrastructure.PackagesWorkflowsDir(workdir)
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(workflows, "dockpipe-workflow-example-1.2.3.tar.gz")
	before, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	beforeInfo, err := os.Stat(archive)
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(start, "config.yml"), future, future); err != nil {
		t.Fatal(err)
	}
	if err := compileClosureForWorkflow(workdir, "example", false); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(archive)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("source-hidden workflow closure changed installed archive bytes: %v", err)
	}
	afterInfo, err := os.Stat(archive)
	if err != nil || !os.SameFile(beforeInfo, afterInfo) || !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
		t.Fatalf("source-hidden workflow closure rewrote installed archive: %v", err)
	}
}

func TestMaterializedSourceRejectsRebuildAndEscape(t *testing.T) {
	for _, kind := range []string{"workflow", "resolver"} {
		for _, scenario := range []string{"version-conflict", "invalid-archive", "invalid-config", "missing-archive", "symlink-escape"} {
			t.Run(kind+"/"+scenario, func(t *testing.T) {
				workdir := t.TempDir()
				source := t.TempDir()
				if err := os.WriteFile(filepath.Join(source, "config.yml"), []byte("name: example\nsteps: []\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				manifest := fmt.Sprintf("schema: 1\nname: example\nkind: %s\nversion: 1.2.3\nnamespace: example\n", kind)
				if err := os.WriteFile(filepath.Join(source, "package.yml"), []byte(manifest), 0o644); err != nil {
					t.Fatal(err)
				}
				destination, err := infrastructure.PackagesWorkflowsDir(workdir)
				if kind == "resolver" {
					destination, err = infrastructure.PackagesResolversDir(workdir)
				}
				if err != nil {
					t.Fatal(err)
				}
				compile := func(from string, force bool) error {
					if kind == "workflow" {
						return compileWorkflowOne(workdir, from, "example", force)
					}
					return compileSingleResolverDir(workdir, destination, from, "example", "example", "1.2.3", force)
				}
				if err := compile(source, true); err != nil {
					t.Fatal(err)
				}
				archive := filepath.Join(destination, fmt.Sprintf("dockpipe-%s-example-1.2.3.tar.gz", kind))
				if scenario == "invalid-config" {
					if err := os.WriteFile(filepath.Join(source, "config.yml"), []byte("steps: [malformed"), 0o644); err != nil {
						t.Fatal(err)
					}
					if _, err := packagebuild.WriteDirTarGzWithPrefix(source, archive, kind+"s/example"); err != nil {
						t.Fatal(err)
					}
				}
				extraction, err := packagebuild.EnsureTarballExtractedCache(archive, infrastructure.TarballExtractCacheRoot(workdir))
				if err != nil {
					t.Fatal(err)
				}
				derived := filepath.Join(extraction, kind+"s", "example")
				if scenario == "invalid-config" {
					// Keep the source parseable while the installed archive fails
					// its own validation; automatic execution must not repair it.
					if err := os.WriteFile(filepath.Join(derived, "config.yml"), []byte("name: example\nsteps: []\n"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				future := time.Now().Add(time.Hour)
				if err := os.Chtimes(filepath.Join(derived, "config.yml"), future, future); err != nil {
					t.Fatal(err)
				}
				observedArchive := archive
				switch scenario {
				case "version-conflict":
					observedArchive = strings.Replace(archive, "1.2.3.tar.gz", "1.2.4.tar.gz", 1)
					err = os.Rename(archive, observedArchive)
				case "invalid-archive":
					err = os.WriteFile(archive, []byte("invalid installed archive"), 0o644)
				case "missing-archive":
					observedArchive = filepath.Join(workdir, "held-original.tar.gz")
					err = os.Rename(archive, observedArchive)
				case "symlink-escape":
					outside := filepath.Join(workdir, "outside-cache")
					if err := os.Rename(derived, outside); err != nil {
						t.Fatal(err)
					}
					err = os.Symlink(outside, derived)
				}
				if err != nil {
					t.Fatal(err)
				}
				before, err := os.ReadFile(observedArchive)
				if err != nil {
					t.Fatal(err)
				}
				beforeInfo, err := os.Stat(observedArchive)
				if err != nil {
					t.Fatal(err)
				}
				if err := compile(derived, false); err == nil {
					t.Fatal("invalid materialized source admission succeeded")
				}
				after, err := os.ReadFile(observedArchive)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("rejected source changed installed input bytes: %v", err)
				}
				afterInfo, err := os.Stat(observedArchive)
				if err != nil || !os.SameFile(beforeInfo, afterInfo) || !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
					t.Fatalf("rejected source rewrote installed input: %v", err)
				}
				if scenario == "missing-archive" || scenario == "version-conflict" {
					if _, err := os.Stat(archive); !os.IsNotExist(err) {
						t.Fatalf("derived source recreated a missing installed archive: %v", err)
					}
				}
			})
		}
	}
}
