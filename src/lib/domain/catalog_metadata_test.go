package domain

import (
	"strings"
	"testing"
)

func TestCatalogMetadataValidation(t *testing.T) {
	for _, catalogType := range []string{"", "app", "workflow", "integration", "core"} {
		if err := ValidateCatalogMetadata(catalogType, []string{"development", "ai"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, categories := range [][]string{{"AI"}, {"development", "development"}, {strings.Repeat("a", 65)}, make([]string, 17)} {
		if err := ValidateCatalogMetadata("app", categories); err == nil {
			t.Fatalf("accepted invalid categories: %q", categories)
		}
	}
	if err := ValidatePackageManifest(&PackageManifest{CatalogType: "resolver"}); err == nil {
		t.Fatal("installation kind must not be accepted as a catalog type")
	}
	workflow, err := ParseWorkflowYAML([]byte("name: editor\ntitle: Example editor\ncatalog_type: app\ncategories: [development]\nsteps: []\n"))
	if err != nil || workflow.Title != "Example editor" || workflow.CatalogType != "app" || len(workflow.Categories) != 1 {
		t.Fatalf("metadata was lost during workflow parsing: %+v, %v", workflow, err)
	}
	workflow.Categories = []string{"../invalid"}
	if err := ValidateLoadedWorkflow(workflow); err == nil {
		t.Fatal("workflow validation accepted invalid category")
	}
}
