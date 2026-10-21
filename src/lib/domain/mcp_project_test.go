package domain

import (
	"encoding/json"
	"testing"
)

func TestMCPProjectValidation(t *testing.T) {
	var config DockpipeProjectConfig
	if err := json.Unmarshal([]byte(`{"schema":1,"mcp":{"servers":{"example":{"transport":"stdio","local":{"executable":"/opt/example/bin/server","working_directory":"/opt/example","runtime":"host","platform":"linux/amd64"}}}}}`), &config); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDockpipeProjectConfig(&config); err != nil {
		t.Fatal(err)
	}
	server := config.MCP.Servers["example"]
	server.Local.SecretEnvironment = "not-configured"
	config.MCP.Servers["example"] = server
	if err := ValidateDockpipeProjectConfig(&config); err == nil {
		t.Fatal("missing secret environment accepted")
	}
}
