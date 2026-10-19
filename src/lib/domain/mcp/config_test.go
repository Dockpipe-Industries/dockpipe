package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	mcpmodel "dockpipe/src/lib/model/mcp"
)

func localServer() mcpmodel.Server {
	return mcpmodel.Server{Transport: "stdio", Local: &mcpmodel.Local{
		Executable: "/opt/example/bin/server", WorkingDirectory: "/opt/example", Runtime: "host", Platform: "linux/amd64",
		Arguments: []string{"--workspace", "/work/space with spaces"},
	}}
}
func TestConfiguredServers(t *testing.T) {
	environments := map[string]map[string]string{"selected": {"API_KEY": "vault-reference"}}
	remote := mcpmodel.Server{Transport: "streamable-http", Remote: &mcpmodel.Remote{
		URL: "https://mcp.example.test/api", SecretEnvironment: "selected", HeaderBindings: map[string]string{"Authorization": "API_KEY"},
	}}
	config := &mcpmodel.Config{Servers: map[string]mcpmodel.Server{"local": localServer(), "remote": remote}}
	if err := Validate(config, environments); err != nil {
		t.Fatal(err)
	}
	windows := localServer()
	windows.Local.Executable = `C:\Program Files\Example\server.exe`
	windows.Local.WorkingDirectory = `C:\Program Files\Example`
	windows.Local.Platform = "windows/arm64"
	config.Servers["windows"] = windows
	if err := Validate(config, environments); err != nil {
		t.Fatal(err)
	}
}
func TestInvalidLocalConfig(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*mcpmodel.Server)
	}{
		{"relative executable", func(s *mcpmodel.Server) { s.Local.Executable = "server" }},
		{"relative cwd", func(s *mcpmodel.Server) { s.Local.WorkingDirectory = "." }},
		{"wrong platform path", func(s *mcpmodel.Server) { s.Local.Platform = "windows/amd64" }},
		{"unknown platform", func(s *mcpmodel.Server) { s.Local.Platform = "beos/amd64" }},
		{"missing runtime", func(s *mcpmodel.Server) { s.Local.Runtime = "" }},
		{"NUL argument", func(s *mcpmodel.Server) { s.Local.Arguments = []string{"bad\x00argument"} }},
		{"argument budget", func(s *mcpmodel.Server) { s.Local.Arguments = make([]string, 129) }},
		{"byte budget", func(s *mcpmodel.Server) { s.Local.Arguments = []string{strings.Repeat("a", 8193)} }},
		{"mixed transport", func(s *mcpmodel.Server) { s.Remote = &mcpmodel.Remote{URL: "https://example.test"} }},
		{"missing environment", func(s *mcpmodel.Server) { s.Local.SecretEnvironment = "missing" }},
		{"mutable version", func(s *mcpmodel.Server) {
			s.Source = &mcpmodel.RegistrySource{Registry: mcpmodel.OfficialRegistry, Name: "org.example/server", Version: "latest"}
		}},
		{"other registry", func(s *mcpmodel.Server) {
			s.Source = &mcpmodel.RegistrySource{Registry: "https://other.test", Name: "org.example/server", Version: "1.0.0"}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := localServer()
			tt.mutate(&server)
			if Validate(&mcpmodel.Config{Servers: map[string]mcpmodel.Server{"sample": server}}, nil) == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}
func TestInvalidRemoteConfig(t *testing.T) {
	for _, address := range []string{"http://example.test/mcp", "file:///tmp/a", "https://user:password@example.test", "https://example.test?token=secret", "https://example.test#fragment", "https://example.test/{tenant}", "https://example.test?"} {
		if ValidateRemoteURL(address) == nil {
			t.Errorf("accepted unsafe URL %q", address)
		}
	}
	for _, bindings := range []map[string]string{
		{"Authorization": "literal-token"}, {"Host": "TOKEN"}, {"X-Key\r\nExtra": "TOKEN"}, {"X-Key": "TOKEN", "x-key": "TOKEN"},
	} {
		server := mcpmodel.Server{Transport: "streamable-http", Remote: &mcpmodel.Remote{URL: "https://example.test", SecretEnvironment: "selected", HeaderBindings: bindings}}
		if Validate(&mcpmodel.Config{Servers: map[string]mcpmodel.Server{"remote": server}}, map[string]map[string]string{"selected": {"TOKEN": "reference"}}) == nil {
			t.Fatal("unsafe binding accepted")
		}
	}
}
func TestNamesAndBudgets(t *testing.T) {
	for _, name := range []string{"core", "native", "dockpipe", "UPPER", "../other", ""} {
		if Validate(&mcpmodel.Config{Servers: map[string]mcpmodel.Server{name: localServer()}}, nil) == nil {
			t.Errorf("accepted name %q", name)
		}
	}
	servers := map[string]mcpmodel.Server{}
	for i := 0; i < 65; i++ {
		servers["s"+strings.Repeat("a", i)] = localServer()
	}
	if Validate(&mcpmodel.Config{Servers: servers}, nil) == nil {
		t.Fatal("server budget ignored")
	}
}
func TestUnknownSecurityFieldRejected(t *testing.T) {
	var config mcpmodel.Config
	if json.Unmarshal([]byte(`{"servers":{"x":{"transport":"stdio","local":{"executable":"/bin/x","env":{"SECRET":"plaintext"}}}}}`), &config) == nil {
		t.Fatal("unknown env field accepted")
	}
}
