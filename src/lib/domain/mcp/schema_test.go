package mcp

import (
	"encoding/json"
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
