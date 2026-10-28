package infrastructure

import (
	"bytes"
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

func TestProjectConfigurationSchemaMinimumMatchesRuntime(t *testing.T) {
	data, err := os.ReadFile("schema/dockpipe.config.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	mirror, err := os.ReadFile("../../../tooling/vscode-extensions/dockpipe-language-support/schemas/dockpipe.config.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, mirror) {
		t.Fatal("project schema mirrors differ")
	}
	schema, err := jsonschema.CompileString("dockpipe.config.schema.json", string(data))
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []interface{}{nil, true, 123, "", "1.2.3", "1.2.3-rc.1", "1.2.3+build.1", "v1.2.3", "1.2", "dev", "latest", ">=1.2.3", "1.2.3-01", " 1.2.3"} {
		schemaValid := schema.Validate(map[string]interface{}{"min_dockpipe_version": value}) == nil
		minimum, isString := value.(string)
		config := &domain.DockpipeProjectConfig{MinDockpipeVersion: &minimum}
		runtimeValid := isString && domain.ValidateDockpipeProjectConfig(config) == nil
		if schemaValid != runtimeValid {
			t.Errorf("minimum %v: schema=%v runtime=%v", value, schemaValid, runtimeValid)
		}
	}
}
