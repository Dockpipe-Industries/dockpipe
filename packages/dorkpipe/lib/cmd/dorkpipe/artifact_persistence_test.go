package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreparedBundleRequiresBothArtifacts(t *testing.T) {
	for _, blocked := range []string{"artifact.json", "patch.diff", ""} {
		t.Run(blocked, func(t *testing.T) {
			root, directory := t.TempDir(), t.TempDir()
			if blocked != "" {
				if err := os.Mkdir(filepath.Join(directory, blocked), 0700); err != nil {
					t.Fatal(err)
				}
			}
			artifact := &editModelArtifact{
				Summary: "Create file", TargetFiles: []string{"new.txt"},
				Patch:        "diff --git a/new.txt b/new.txt\nnew file mode 100644\n--- /dev/null\n+++ b/new.txt\n@@ -0,0 +1 @@\n+hello\n",
				CreatedFiles: map[string]string{"new.txt": "hello\n"},
			}
			prepared, patch, err := writePreparedArtifactBundle(root, directory, artifact, "")
			if blocked != "" {
				if err == nil || prepared != nil || patch != "" {
					t.Fatalf("incomplete bundle reported ready: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			loaded, _, err := loadPreparedArtifact(directory)
			if err != nil || loaded.Patch != prepared.Patch {
				t.Fatalf("ready bundle cannot be loaded: %v", err)
			}
		})
	}
}

func TestWriteJSONReportsEncodingFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact.json")
	if err := writeJSON(path, make(chan int)); err == nil {
		t.Fatal("unsupported value reported persisted")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("encoding failure created an artifact: %v", err)
	}
}
