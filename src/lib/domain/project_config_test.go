package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultDockpipeProjectConfigBytesRoundTrip(t *testing.T) {
	b, err := DefaultDockpipeProjectConfigBytes()
	if err != nil {
		t.Fatal(err)
	}
	var c DockpipeProjectConfig
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatalf("default JSON: %v\n%s", err, string(b))
	}
	if c.Schema != 1 {
		t.Fatalf("schema: %d", c.Schema)
	}
	if c.Compile.Workflows == nil || len(*c.Compile.Workflows) < 1 {
		t.Fatal("expected compile.workflows")
	}
	if c.Secrets.VaultTemplate == nil || *c.Secrets.VaultTemplate == "" {
		t.Fatal("expected secrets.vault_template in default")
	}
}

func TestLoadDockpipeProjectConfigCanonicalCompileWorkflows(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, DockpipeProjectConfigFileName)
	if err := os.WriteFile(p, []byte(`{"schema":1,"compile":{"workflows":["workflows","packages"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadDockpipeProjectConfig(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if c.Compile.Workflows == nil || len(*c.Compile.Workflows) != 2 {
		t.Fatalf("compile.workflows = %v", c.Compile.Workflows)
	}
}

func TestLoadDockpipeProjectConfigRejectsCompileResolvers(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, DockpipeProjectConfigFileName)
	if err := os.WriteFile(p, []byte(`{"schema":1,"compile":{"resolvers":["extra"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadDockpipeProjectConfig(tmp)
	if err == nil {
		t.Fatal("expected compile.resolvers rejection")
	}
	if got := err.Error(); !strings.Contains(got, "compile.resolvers") || !strings.Contains(got, "compile.workflows") {
		t.Fatalf("error should name old key and replacement: %v", err)
	}
}

func TestLoadDockpipeProjectConfigRejectsCompileBundles(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, DockpipeProjectConfigFileName)
	if err := os.WriteFile(p, []byte(`{"schema":1,"compile":{"bundles":["legacy"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadDockpipeProjectConfig(tmp)
	if err == nil {
		t.Fatal("expected compile.bundles rejection")
	}
	if got := err.Error(); !strings.Contains(got, "compile.bundles") || !strings.Contains(got, "compile.workflows") {
		t.Fatalf("error should name old key and replacement: %v", err)
	}
}

func TestLoadDockpipeProjectConfigRejectsCompileBundlesWhenWorkflowsAlsoPresent(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, DockpipeProjectConfigFileName)
	if err := os.WriteFile(p, []byte(`{"schema":1,"compile":{"workflows":["canonical"],"bundles":["legacy"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadDockpipeProjectConfig(tmp)
	if err == nil {
		t.Fatal("expected compile.bundles rejection instead of precedence or merging")
	}
	if got := err.Error(); !strings.Contains(got, "compile.bundles") || !strings.Contains(got, "compile.workflows") {
		t.Fatalf("both-key error should name rejected key and replacement: %v", err)
	}
}

func TestLoadDockpipeProjectConfigStillIgnoresOtherUnknownKeys(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, DockpipeProjectConfigFileName)
	if err := os.WriteFile(p, []byte(`{"schema":1,"future_top_level":true,"compile":{"workflows":["workflows"],"future_compile_key":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadDockpipeProjectConfig(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if c.Compile.Workflows == nil || len(*c.Compile.Workflows) != 1 {
		t.Fatalf("compile.workflows = %v", c.Compile.Workflows)
	}
}

func TestLoadDockpipeProjectConfigMissing(t *testing.T) {
	c, err := LoadDockpipeProjectConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if c != nil {
		t.Fatal("expected nil when file missing")
	}
}

func TestLoadDockpipeProjectConfigInvalidVault(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, DockpipeProjectConfigFileName)
	if err := os.WriteFile(p, []byte(`{"schema":1,"secrets":{"vault":"bogus"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadDockpipeProjectConfig(tmp)
	if err == nil {
		t.Fatal("expected error for invalid secrets.vault")
	}
}

func TestLoadDockpipeProjectConfigInvalidPackageSourceKind(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, DockpipeProjectConfigFileName)
	if err := os.WriteFile(p, []byte(`{"schema":1,"packages":{"sources":[{"kind":"bogus","path":"x"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadDockpipeProjectConfig(tmp)
	if err == nil {
		t.Fatal("expected error for invalid packages.sources kind")
	}
}

func TestResolveVaultTemplatePathPrecedence(t *testing.T) {
	root := t.TempDir()
	vault := ".env.vault.template"
	legacy := ".env.op.template"
	cfg := &DockpipeProjectConfig{
		Secrets: DockpipeSecretsConfig{
			VaultTemplate:    &vault,
			OpInjectTemplate: &legacy,
		},
	}
	got, ok := ResolveVaultTemplatePath(cfg, root)
	if !ok {
		t.Fatal("expected ok")
	}
	want := filepath.Join(root, vault)
	if got != want {
		t.Fatalf("got %q want %q (vault_template should win)", got, want)
	}
	cfg2 := &DockpipeProjectConfig{
		Secrets: DockpipeSecretsConfig{
			OpInjectTemplate: &legacy,
		},
	}
	got2, ok2 := ResolveVaultTemplatePath(cfg2, root)
	if !ok2 {
		t.Fatal("expected ok for legacy only")
	}
	if got2 != filepath.Join(root, legacy) {
		t.Fatalf("got %q", got2)
	}
}

func TestFindProjectRootWithDockpipeConfig(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, DockpipeProjectConfigFileName)
	if err := os.WriteFile(cfgPath, []byte(`{"schema":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	got, err := FindProjectRootWithDockpipeConfig(sub)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
