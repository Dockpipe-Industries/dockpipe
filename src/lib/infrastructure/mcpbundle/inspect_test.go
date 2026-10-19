package mcpbundle

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T, extra string, change func(map[string]any)) (string, string) {
	t.Helper()
	document := map[string]any{"manifest_version": "0.3", "name": "neutral-fixture", "version": "1.2.3", "description": "Test fixture", "author": map[string]any{"name": "Test"}, "compatibility": map[string]any{"platforms": []string{"linux"}}, "server": map[string]any{"type": "binary", "entry_point": "bin/server", "mcp_config": map[string]any{"command": "${__dirname}/bin/server", "args": []string{"--stdio"}}}}
	if change != nil {
		change(document)
	}
	filename := filepath.Join(t.TempDir(), "fixture.mcpb")
	f, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	bundle := zip.NewWriter(f)
	metadata, _ := json.Marshal(document)
	entry, _ := bundle.Create("manifest.json")
	entry.Write(metadata)
	native := make([]byte, 64)
	copy(native, []byte{0x7f, 'E', 'L', 'F', 2, 1})
	native[18] = 62
	entry, _ = bundle.Create("bin/server")
	entry.Write(native)
	if extra != "" {
		entry, err = bundle.Create(extra)
		if err != nil {
			t.Fatal(err)
		}
		entry.Write([]byte("extra"))
	}
	if err := bundle.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	data, _ := os.ReadFile(filename)
	sum := sha256.Sum256(data)
	return filename, hex.EncodeToString(sum[:])
}
func TestInspectBinaryAndRejectUnsafeContracts(t *testing.T) {
	file, sum := fixture(t, "", nil)
	result, err := Inspect(context.Background(), file, sum, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "neutral-fixture" || result.EntryPoint != "bin/server" || result.Executed || len(result.Arguments) != 1 || result.Arguments[0] != "--stdio" {
		t.Fatalf("incorrect inspection: %+v", result)
	}
	for _, name := range []string{"../outside", "/absolute", "bin\\escape", "bin/CON.txt", "BIN/SERVER", "bin/server/child", "bin/trailing.", "manifest.json"} {
		t.Run(name, func(t *testing.T) {
			file, sum := fixture(t, name, nil)
			if _, err := Inspect(context.Background(), file, sum, "linux", "amd64"); err == nil {
				t.Fatal("unsafe member accepted")
			}
		})
	}
	for _, target := range []struct{ platform, arch string }{{"linux", "arm64"}, {"darwin", "amd64"}, {"win32", "amd64"}} {
		if _, err := Inspect(context.Background(), file, sum, target.platform, target.arch); err == nil {
			t.Fatal("wrong target accepted")
		}
	}
	if _, err := Inspect(context.Background(), file, "0000000000000000000000000000000000000000000000000000000000000000", "linux", "amd64"); err == nil {
		t.Fatal("wrong digest accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Inspect(ctx, file, sum, "linux", "amd64"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
func TestUnsupportedLaunchConfiguration(t *testing.T) {
	changes := []func(map[string]any){
		func(m map[string]any) { m["user_config"] = map[string]any{"token": map[string]any{"type": "string"}} },
		func(m map[string]any) { m["server"].(map[string]any)["type"] = "node" },
		func(m map[string]any) {
			m["server"].(map[string]any)["mcp_config"].(map[string]any)["env"] = map[string]string{"TOKEN": "secret"}
		},
		func(m map[string]any) {
			m["server"].(map[string]any)["mcp_config"].(map[string]any)["command"] = "external"
		},
		func(m map[string]any) {
			m["server"].(map[string]any)["mcp_config"].(map[string]any)["args"] = []string{"${user_config.workspace}"}
		},
	}
	for i, change := range changes {
		file, sum := fixture(t, "", change)
		if _, err := Inspect(context.Background(), file, sum, "linux", "amd64"); err == nil {
			t.Fatalf("unsupported configuration %d accepted", i)
		}
	}
}
