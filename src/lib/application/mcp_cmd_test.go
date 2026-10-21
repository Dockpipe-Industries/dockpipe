package application

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure/mcpregistry"
	mcpmodel "dockpipe/src/lib/model/mcp"
)

type registryStub struct {
	call    string
	name    string
	version string
	query   mcpregistry.Query
	err     error
}

func (r *registryStub) Search(ctx context.Context, q mcpregistry.Query) (mcpmodel.RegistryPage, error) {
	r.call = "search"
	r.query = q
	return mcpmodel.RegistryPage{Servers: []mcpmodel.RegistryRecord{}}, r.err
}
func (r *registryStub) Versions(ctx context.Context, name string) (mcpmodel.RegistryPage, error) {
	r.call = "versions"
	r.name = name
	return mcpmodel.RegistryPage{Servers: []mcpmodel.RegistryRecord{}}, r.err
}
func (r *registryStub) Version(ctx context.Context, name, version string) (mcpmodel.RegistryRecord, error) {
	r.call = "show"
	r.name = name
	r.version = version
	return mcpmodel.RegistryRecord{Name: name, Version: version}, r.err
}

func TestMCPRegistryDispatch(t *testing.T) {
	for _, args := range [][]string{
		{"registry", "search", "--search", "filesystem", "--limit", "2", "--cursor", "next"},
		{"registry", "versions", "--name", "org.example/server"},
		{"registry", "show", "--name", "org.example/server", "--version", "1.2.3"},
	} {
		var output bytes.Buffer
		client := &registryStub{}
		if err := runMCP(context.Background(), args, &output, client); err != nil {
			t.Fatal(err)
		}
		if client.call != args[1] || !json.Valid(output.Bytes()) {
			t.Fatal("incorrect dispatch/output", client, output.String())
		}
		if client.call == "search" && (client.query.Search != "filesystem" || client.query.Limit != 2 || client.query.Cursor != "next") {
			t.Fatal("query changed")
		}
		if client.call == "show" && client.version != "1.2.3" {
			t.Fatal("version changed")
		}
	}
}
func TestMCPInvalidCLIAndErrors(t *testing.T) {
	for _, args := range [][]string{{"registry"}, {"install", "x"}, {"registry", "launch"}, {"registry", "show"}, {"registry", "show", "--name", "org.example/server", "--cursor", "x"}, {"registry", "versions", "--name", "org.example/server", "--search", "x"}, {"registry", "search", "--name", "x"}, {"registry", "search", "--unknown"}} {
		var output bytes.Buffer
		client := &registryStub{}
		if runMCP(context.Background(), args, &output, client) == nil || client.call != "" {
			t.Fatal("invalid command dispatched", args)
		}
	}
	client := &registryStub{err: context.Canceled}
	var output bytes.Buffer
	if err := runMCP(context.Background(), []string{"registry", "search"}, &output, client); !errors.Is(err, context.Canceled) || output.Len() != 0 {
		t.Fatal("failed operation became successful output", err)
	}
}
func TestMCPProjectValidationNoResolution(t *testing.T) {
	dir := t.TempDir()
	config := `{"schema":1,"secrets":{"environments":{"remote":{"resolver":"this-must-never-execute","bindings":{"API_KEY":"private-secret-reference"}}}},"mcp":{"servers":{"example":{"transport":"streamable-http","remote":{"url":"https://never-connect.example.test/mcp","secret_environment":"remote","header_bindings":{"Authorization":"API_KEY"}}}}}}`
	if err := os.WriteFile(filepath.Join(dir, "dockpipe.config.json"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	client := &registryStub{}
	if err := runMCP(context.Background(), []string{"config", "validate", "--workdir", dir}, &output, client); err != nil {
		t.Fatal(err)
	}
	if client.call != "" || strings.Contains(output.String(), "secret-reference") || strings.Contains(output.String(), "never-connect") {
		t.Fatal("validation performed discovery or leaked configuration")
	}
	var result map[string]any
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["valid"] != true || result["servers_started"] != false || result["configured_servers"] != float64(1) {
		t.Fatal("incorrect validation result", result)
	}
	if err := os.WriteFile(filepath.Join(dir, "dockpipe.config.json"), []byte(strings.Replace(config, "streamable-http", "sse", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runMCP(context.Background(), []string{"config", "validate", "--workdir", dir}, &output, client); err == nil {
		t.Fatal("unsupported transport accepted")
	}
}

func TestMCPBundleRequiresExplicitSelection(t *testing.T) {
	for _, operation := range []string{"inspect", "install"} {
		var output bytes.Buffer
		client := &registryStub{}
		err := runMCP(context.Background(), []string{"bundle", operation, "--path", "missing.mcpb"}, &output, client)
		if err == nil || output.Len() != 0 || client.call != "" {
			t.Fatal("incomplete bundle request must not produce output or contact registry", err)
		}
	}
}

func TestMCPBundleCLIInstallAndInspect(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "dockpipe.config.json"), []byte(`{"schema":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "fixture.mcpb")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	bundle := zip.NewWriter(file)
	manifest := `{"manifest_version":"0.3","name":"neutral","version":"1.0.0","description":"fixture","author":{"name":"test"},"compatibility":{"platforms":["linux"]},"server":{"type":"binary","entry_point":"server","mcp_config":{"command":"${__dirname}/server","args":["--stdio"]}}}`
	writer, _ := bundle.Create("manifest.json")
	writer.Write([]byte(manifest))
	header := make([]byte, 64)
	copy(header, []byte{0x7f, 'E', 'L', 'F', 2, 1})
	header[18] = 62
	writer, _ = bundle.Create("server")
	writer.Write(header)
	bundle.Close()
	file.Close()
	data, _ := os.ReadFile(archive)
	sum := sha256.Sum256(data)
	client := &registryStub{}
	for _, operation := range []string{"inspect", "install"} {
		var output bytes.Buffer
		args := []string{"bundle", operation, "--path", archive, "--sha256", hex.EncodeToString(sum[:]), "--platform", "linux", "--arch", "amd64", "--workdir", root}
		if err := runMCP(context.Background(), args, &output, client); err != nil {
			t.Fatal(err)
		}
		var response map[string]any
		if err := json.Unmarshal(output.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if operation == "install" {
			if response["started"] != false {
				t.Fatal("install must not start")
			}
			executable, ok := response["executable"].(string)
			if !ok {
				t.Fatal("missing installed executable")
			}
			bytes, err := os.ReadFile(executable)
			if err != nil || len(bytes) != 64 {
				t.Fatal("wrong installed payload", err)
			}
		} else if response["executed"] != false || response["name"] != "neutral" {
			t.Fatal("wrong inspection", response)
		}
	}
	if client.call != "" {
		t.Fatal("local bundle operation accessed Registry")
	}
}
