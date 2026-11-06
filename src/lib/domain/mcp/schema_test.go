package mcp

import (
	"bytes"
	"encoding/json"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"path/filepath"
	"testing"
)

func TestMCPAuthoredSchema(t *testing.T) {
	path, err := filepath.Abs("../../infrastructure/schema/mcp-config.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := jsonschema.Compile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		data  string
		valid bool
	}{
		{`{"servers":{"sample":{"transport":"stdio","local":{"executable":"/opt/server","working_directory":"/opt","runtime":"host","platform":"linux/amd64"}}}}`, true},
		{`{"servers":{"sample":{"transport":"streamable-http","remote":{"url":"https://example.test/mcp"}}}}`, true},
		{`{"servers":{"sample":{"transport":"sse","remote":{"url":"https://example.test/mcp"}}}}`, false},
		{`{"servers":{"sample":{"transport":"stdio","local":{"executable":"/opt/server","working_directory":"/opt","runtime":"host","platform":"linux/amd64","env":{"KEY":"value"}}}}}`, false},
		{`{"servers":{"sample":{"transport":"stdio","local":{"executable":"/opt/server","working_directory":"/opt","runtime":"host","platform":"linux/amd64"},"remote":{"url":"https://example.test"}}}}`, false},
	} {
		var value any
		if err := json.Unmarshal([]byte(test.data), &value); err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(value); (err == nil) != test.valid {
			t.Fatalf("schema acceptance mismatch: valid=%v err=%v", test.valid, err)
		}
	}
}

// Both authored entry points must reject malformed MCP configuration while
// retaining the project's current version fields and editor schema parity.
func TestMCPProjectSchemaAndEditorMirror(t *testing.T) {
	paths := []string{
		"../../infrastructure/schema/dockpipe.config.schema.json",
		"../../../../tooling/vscode-extensions/dockpipe-language-support/schemas/dockpipe.config.schema.json",
	}
	core, err := os.ReadFile("../../infrastructure/schema/mcp-config.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	editor, err := os.ReadFile("../../../../tooling/vscode-extensions/dockpipe-language-support/schemas/mcp-config.schema.json")
	if err != nil || !bytes.Equal(core, editor) {
		t.Fatalf("MCP editor schema differs: %v", err)
	}
	for _, filename := range paths {
		schema, err := jsonschema.Compile(filename)
		if err != nil {
			t.Fatal(err)
		}
		for _, transport := range []string{"streamable-http", "sse"} {
			value := map[string]any{
				"schema":               1,
				"dockpipe_version":     "1.2.3",
				"min_dockpipe_version": "1.2.0",
				"mcp": map[string]any{"servers": map[string]any{
					"example": map[string]any{
						"transport": transport,
						"remote":    map[string]any{"url": "https://example.test/mcp"},
					},
				}},
			}
			if err := schema.Validate(value); (err == nil) != (transport == "streamable-http") {
				t.Fatalf("%s accepted transport %s: %v", filename, transport, err)
			}
		}
	}
}
