package application

import (
	"encoding/json"
	"testing"

	"dockpipe/src/lib/application/internal/workflowinputs"
)

func TestCatalogInputProjectionPreservesJSONAndOwnsControls(t *testing.T) {
	fields := []workflowinputs.Field{
		{FieldName: "Tags", EnvName: "TAGS", Type: "List<string>", ElementType: "string"},
		{FieldName: "Settings", Type: "Settings", Attributes: map[string]string{"displayname": "Options"}, Children: []workflowinputs.Field{
			{FieldName: "Name", EnvName: "NAME", Type: "string", DefaultValue: "default"},
		}},
		{FieldName: "Entries", Type: "List<Entry>", ElementType: "Entry", Attributes: map[string]string{"control": "custom"}, Children: []workflowinputs.Field{
			{FieldName: "Value", EnvName: "ENTRY_VALUE", Type: "string"},
		}},
	}
	projected := catalogWorkflowInputRecords(fields)
	encoded, err := json.Marshal(projected)
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"field_name":"Tags","env_name":"TAGS","type":"List\u003cstring\u003e","element_type":"string","attributes":{"control":"list"}},{"field_name":"Settings","env_name":"","type":"Settings","attributes":{"control":"object","displayname":"Options"},"children":[{"field_name":"Name","env_name":"NAME","type":"string","default_value":"default"}]},{"field_name":"Entries","env_name":"","type":"List\u003cEntry\u003e","element_type":"Entry","attributes":{"control":"custom"},"children":[{"field_name":"Value","env_name":"ENTRY_VALUE","type":"string"}]}]`
	if string(encoded) != want {
		t.Fatalf("catalog JSON changed:\n%s\nwant:\n%s", encoded, want)
	}
	if _, exists := fields[1].Attributes["control"]; exists {
		t.Fatal("catalog projection mutated shared metadata")
	}
	projected[2].Attributes["control"] = "changed"
	if fields[2].Attributes["control"] != "custom" {
		t.Fatal("catalog projection aliases metadata attributes")
	}
}
