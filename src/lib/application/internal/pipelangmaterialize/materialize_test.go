package pipelangmaterialize

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestMaterializeRootsWritesArtifactsOutsideSource(t *testing.T) {
	source := filepath.Join(t.TempDir(), "workflow")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	pipe := `public Class DemoConfig
{
    public string Image = "nginx";
}
`
	if err := os.WriteFile(filepath.Join(source, "config.pipe"), []byte(pipe), 0o644); err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	count, err := MaterializeRoots([]string{source}, true, output)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("MaterializeRoots() count = %d, want 1", count)
	}
	if _, err := os.Stat(filepath.Join(source, ".pipelang")); !os.IsNotExist(err) {
		t.Fatalf("source-side output exists, err=%v", err)
	}
	found := false
	if err := filepath.WalkDir(output, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && filepath.Base(path) == "config.DemoConfig.workflow.yml" {
			found = true
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatalf("materialized workflow not found under %s", output)
	}
}

func TestInferEntryClassFromTypeRefResolvesInterfaceImplementation(t *testing.T) {
	files := map[string][]byte{
		"types.pipe": []byte(`public Interface Config { public string Image; }
public Class AppConfig : Config { public string Image = "nginx"; }
`),
	}
	got, err := InferEntryClassFromTypeRef(files, "Config")
	if err != nil {
		t.Fatal(err)
	}
	if got != "AppConfig" {
		t.Fatalf("InferEntryClassFromTypeRef() = %q", got)
	}
}
