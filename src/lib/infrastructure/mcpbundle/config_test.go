package mcpbundle

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func configuredFixture(t *testing.T, field map[string]any, argument string) (string, string) {
	return fixture(t, "", func(m map[string]any) {
		m["user_config"] = map[string]any{"workspace": field}
		m["server"].(map[string]any)["mcp_config"].(map[string]any)["args"] = []string{"--workspace", argument}
	})
}
func TestExplicitDirectoryConfiguration(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "project")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	file, sum := configuredFixture(t, map[string]any{"type": "directory", "title": "Workspace", "required": true}, "${user_config.workspace}")
	bindings := Bindings{root, map[string]string{"workspace": child}}
	installed, err := InstallConfigured(context.Background(), file, sum, "linux", "amd64", t.TempDir(), bindings)
	if err != nil {
		t.Fatal(err)
	}
	canonical, _ := filepath.EvalSymlinks(child)
	if installed.Inspection.Arguments[1] != canonical || installed.Inspection.ConfigurationDigest == "" || installed.Started {
		t.Fatal("directory binding not retained", installed)
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(installed.Directory), "receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	var receipt Inspection
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.ConfigurationDigest != installed.Inspection.ConfigurationDigest || receipt.Arguments[1] != canonical {
		t.Fatal("receipt lost effective configuration")
	}
	for _, bad := range []Bindings{{}, {root, map[string]string{"workspace": t.TempDir()}}, {root, map[string]string{"workspace": "relative"}}, {root, map[string]string{"workspace": child, "secret": "hidden"}}} {
		if _, err := InspectConfigured(context.Background(), file, sum, "linux", "amd64", bad); err == nil {
			t.Fatal("invalid explicit binding accepted")
		}
	}
}
func TestDirectoryBindingsRejectSecretsDefaultsAndInterpolation(t *testing.T) {
	root := t.TempDir()
	bindings := Bindings{root, map[string]string{"workspace": root}}
	fields := []map[string]any{{"type": "directory", "required": true, "sensitive": true}, {"type": "directory", "required": true, "default": "/"}, {"type": "directory", "required": true, "multiple": true}, {"type": "string", "required": true}}
	for _, field := range fields {
		file, sum := configuredFixture(t, field, "${user_config.workspace}")
		if _, err := InspectConfigured(context.Background(), file, sum, "linux", "amd64", bindings); err == nil {
			t.Fatal("unsupported field accepted")
		}
	}
	for _, arg := range []string{"${HOME}", "--workspace=${user_config.workspace}", "${user_config.unknown}", "${user_config.workspace}/.."} {
		file, sum := configuredFixture(t, map[string]any{"type": "directory", "required": true}, arg)
		if _, err := InspectConfigured(context.Background(), file, sum, "linux", "amd64", bindings); err == nil {
			t.Fatal("unsupported interpolation accepted")
		}
	}
}
func TestDirectorySymlinkEscape(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "escape")
	if err := os.Symlink(t.TempDir(), link); err != nil {
		t.Skip("symlink unavailable")
	}
	file, sum := configuredFixture(t, map[string]any{"type": "directory", "required": true}, "${user_config.workspace}")
	if _, err := InspectConfigured(context.Background(), file, sum, "linux", "amd64", Bindings{root, map[string]string{"workspace": link}}); err == nil {
		t.Fatal("symlink escaped root")
	}
}
