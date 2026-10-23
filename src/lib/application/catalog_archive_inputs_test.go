package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestListCatalogWorkflowsReadsTypedInputsFromInstalledArchive(t *testing.T) {
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "dockpipe.config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	stage := t.TempDir()
	modelDirectory := filepath.Join(stage, "models")
	if err := os.MkdirAll(modelDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	config := "name: archived-inputs\ntypes: [models/DisplayConfig.pipe]\nsteps: []\n"
	if err := os.WriteFile(filepath.Join(stage, "config.yml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	model := `public Class DisplayConfig
{
    [DisplayName = "Width"]
    [EnvName = "DISPLAY_WIDTH"]
    public string Width = "1920";
}
`
	if err := os.WriteFile(filepath.Join(modelDirectory, "DisplayConfig.pipe"), []byte(model), 0o644); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(project, "bin", ".dockpipe", "internal", "packages", "workflows")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(store, "dockpipe-workflow-archived-inputs-1.0.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(stage, archive, "workflows/archived-inputs"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(stage); err != nil {
		t.Fatal(err)
	}

	workflows, err := listCatalogWorkflows(project, project)
	if err != nil {
		t.Fatal(err)
	}
	if len(workflows) != 1 || !strings.HasPrefix(workflows[0].ConfigPath, "tar://") {
		t.Fatalf("expected one archive-backed workflow, got %#v", workflows)
	}
	inputs := workflows[0].Inputs
	if len(inputs) != 1 || inputs[0].FieldName != "Width" || inputs[0].EnvName != "DISPLAY_WIDTH" || inputs[0].DefaultValue != "1920" {
		t.Fatalf("installed archive lost its typed input contract: %#v", inputs)
	}
}
