package projectmodel

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProjectConfigJSONShape(t *testing.T) {
	coreFrom := "src/core"
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
			CoreFrom:  &coreFrom,
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
	const want = `{"schema":1,"compile":{"core_from":"src/core","workflows":["workflows","packages"]},"secrets":{"vault_template":".env.vault.template","op_inject_template":".env.op.template","vault":"op","notes":"references only"},"packages":{"tarball_dir":"release/artifacts","namespace":"example","registry_urls":["https://packages.example.com"],"sources":[{"kind":"store","path":"vendor/packages"}]}}`
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
