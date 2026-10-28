package projectmodel

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProjectConfigJSONShape(t *testing.T) {
	workflows := []string{"workflows", "packages"}
	vaultTemplate := ".env.vault.template"
	opInjectTemplate := ".env.op.template"
	vault := "op"
	notes := "references only"
	tarballDir := "release/artifacts"
	namespace := "example"
	registryURLs := []string{"https://packages.example.com"}
	sources := []DockpipePackageSourceConfig{{Kind: "store", Path: "vendor/packages"}}

	got, err := json.Marshal(DockpipeProjectConfig{
		Schema: 1,
		Compile: DockpipeCompileConfig{
			Workflows: &workflows,
		},
		Secrets: DockpipeSecretsConfig{
			VaultTemplate:    &vaultTemplate,
			OpInjectTemplate: &opInjectTemplate,
			Vault:            &vault,
			Notes:            &notes,
		},
		Packages: DockpipePackagesConfig{
			TarballDir:   &tarballDir,
			Namespace:    &namespace,
			RegistryURLs: &registryURLs,
			Sources:      &sources,
		},
	})
	if err != nil {
		t.Fatalf("marshal project config: %v", err)
	}
	const want = `{"schema":1,"compile":{"workflows":["workflows","packages"]},"secrets":{"vault_template":".env.vault.template","op_inject_template":".env.op.template","vault":"op","notes":"references only"},"packages":{"tarball_dir":"release/artifacts","namespace":"example","registry_urls":["https://packages.example.com"],"sources":[{"kind":"store","path":"vendor/packages"}]}}`
	if string(got) != want {
		t.Fatalf("project config JSON changed:\nwant: %s\ngot:  %s", want, got)
	}
}

func TestProjectConfigJSONRoundTripPreservesEmptyListsAndIgnoresUnknownKeys(t *testing.T) {
	const authored = `{"schema":1,"future_top_level":true,"compile":{"workflows":[],"future_compile_key":true},"packages":{"registry_urls":[],"sources":[]}}`

	var got DockpipeProjectConfig
	if err := json.Unmarshal([]byte(authored), &got); err != nil {
		t.Fatalf("unmarshal project config: %v", err)
	}
	if got.Compile.Workflows == nil || len(*got.Compile.Workflows) != 0 {
		t.Fatalf("compile.workflows did not preserve explicit empty list: %#v", got.Compile.Workflows)
	}
	if got.Packages.RegistryURLs == nil || len(*got.Packages.RegistryURLs) != 0 {
		t.Fatalf("packages.registry_urls did not preserve explicit empty list: %#v", got.Packages.RegistryURLs)
	}
	if got.Packages.Sources == nil || len(*got.Packages.Sources) != 0 {
		t.Fatalf("packages.sources did not preserve explicit empty list: %#v", got.Packages.Sources)
	}
}

func TestCompileConfigRejectsRetiredKeysWithExactErrors(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{name: "core_from", json: `{"core_from":"legacy/core"}`, want: "compile.core_from is retired; runtime defaults are built in and optional assets belong to packages"},
		{name: "resolvers", json: `{"resolvers":["extra"]}`, want: "compile.resolvers is not supported; use compile.workflows"},
		{name: "bundles", json: `{"workflows":["canonical"],"bundles":["legacy"]}`, want: "compile.bundles is not supported; use compile.workflows"},
		{name: "both", json: `{"bundles":[],"resolvers":[]}`, want: "compile.resolvers is not supported; use compile.workflows"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got DockpipeCompileConfig
			err := json.Unmarshal([]byte(test.json), &got)
			if err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestCompileConfigInvalidJSONPreservesDecoderError(t *testing.T) {
	var got DockpipeCompileConfig
	err := json.Unmarshal([]byte(`{"workflows":`), &got)
	if err == nil || !strings.Contains(err.Error(), "unexpected end of JSON input") {
		t.Fatalf("unexpected invalid JSON error: %v", err)
	}
}

func TestProjectConfigVersionDeclarationsRoundTrip(t *testing.T) {
	const authored = `{"dockpipe_version":"1.2.3+build.1","min_dockpipe_version":"1.2.3-rc.1"}`
	var config DockpipeProjectConfig
	if err := json.Unmarshal([]byte(authored), &config); err != nil {
		t.Fatal(err)
	}
	if config.DockpipeVersion == nil || *config.DockpipeVersion != "1.2.3+build.1" {
		t.Fatal("lost exact identity")
	}
	if config.MinDockpipeVersion == nil || *config.MinDockpipeVersion != "1.2.3-rc.1" {
		t.Fatal("lost minimum")
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	var decoded DockpipeProjectConfig
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if *decoded.DockpipeVersion != *config.DockpipeVersion || *decoded.MinDockpipeVersion != *config.MinDockpipeVersion {
		t.Fatal("version declarations changed during round trip")
	}
}
