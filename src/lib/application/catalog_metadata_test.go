package application

import (
	"testing"

	"dockpipe/src/lib/domain"
)

func TestCatalogWorkflowCategoryPrecedence(t *testing.T) {
	cases := []struct {
		name        string
		catalogType string
		category    string
		want        string
	}{
		{name: "legacy app", category: "app", want: "app"},
		{name: "new app", catalogType: "app", want: "app"},
		{name: "recategorized app", catalogType: "workflow", category: "app"},
		{name: "other legacy metadata", catalogType: "workflow", category: "demo", want: "demo"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			workflow := &domain.Workflow{CatalogType: test.catalogType, Category: test.category}
			if got := catalogWorkflowCategory(workflow); got != test.want {
				t.Fatalf("got category %q, want %q", got, test.want)
			}
		})
	}
}
