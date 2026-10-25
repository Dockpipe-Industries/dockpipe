package domain

import "testing"

func TestWorkflowCatalogMetadataValidation(t *testing.T) {
	workflow, err := ParseWorkflowYAML([]byte("name: editor\ntitle: Example editor\ncatalog_type: app\ncategories: [development]\nsteps: []\n"))
	if err != nil || workflow.Title != "Example editor" || workflow.CatalogType != "app" || len(workflow.Categories) != 1 {
		t.Fatalf("metadata was lost during workflow parsing: %+v, %v", workflow, err)
	}
	workflow.Categories = []string{"../invalid"}
	if err := ValidateLoadedWorkflow(workflow); err == nil {
		t.Fatal("workflow validation accepted invalid category")
	}
}
