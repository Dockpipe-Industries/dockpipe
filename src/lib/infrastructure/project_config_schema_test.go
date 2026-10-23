package infrastructure

import (
	"os"
	"testing"

	"dockpipe/src/lib/domain"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func TestProjectConfigurationSchemaMatchesExactVersionValidation(t *testing.T) {
	data, err := os.ReadFile("schema/dockpipe.config.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := jsonschema.CompileString("dockpipe.config.schema.json", string(data))
	if err != nil {
		t.Fatal(err)
	}
	versions := []string{"0.6.24", "1.0.0-rc.1", "1.0.0+build.7", "", "v0.6.24", "latest", "dev", "0.6", ">=0.6.24", "01.6.24", "1.0.0-01"}
	for _, version := range versions {
		config := map[string]interface{}{"dockpipe_version": version}
		schemaValid := schema.Validate(config) == nil
		runtimeValid := domain.ValidateExactDockpipeVersion(version) == nil
		if schemaValid != runtimeValid {
			t.Errorf("editor/runtime disagree about %q: editor=%v runtime=%v", version, schemaValid, runtimeValid)
		}
	}
	if err := schema.Validate(map[string]interface{}{"compile": map[string]interface{}{"workflows": []interface{}{}}, "future_key": true}); err != nil {
		t.Fatalf("schema broke forward-compatible unpinned configuration: %v", err)
	}
}
