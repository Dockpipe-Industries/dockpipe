package packagemodel

import (
	"reflect"
	"testing"

	modeldependency "dockpipe/src/lib/model/dependency"

	"gopkg.in/yaml.v3"
)

func TestManifestYAMLShape(t *testing.T) {
	required := true
	want := "schema: 1\nname: demo\nversion: 1.2.3\ntitle: Demo\ndescription: package description\nauthor: ACME\nwebsite: https://example.com\nlicense: Apache-2.0\nicon: assets/icon.png\nartwork:\n    vscode: assets/vscode.png\nkind: workflow\nprovider: acme\ncapability: workflow.demo\nprimitive: workflow.legacy\nnamespace: example\ntags:\n    - demo\nkeywords:\n    - sample\nmin_dockpipe_version: 1.0.0\nrepository: https://example.com/repo\nprovides:\n    - demo\nrequires_capabilities:\n    - cli.demo\nrequires_primitives:\n    - cli.legacy\nrequires_resolvers:\n    - demo\nincludes_resolvers:\n    - demo-alt\ndepends:\n    - base\nplatforms:\n    - linux\ndependencies:\n    host:\n        - id: git\n          command: git\n          description: version control\n          required: true\n          install:\n            linux: install git\nallow_clone: true\ndistribution: source\nimage:\n    source: registry\n    ref: example/demo:1.2.3\n    pull_policy: if-missing\nscript_contract:\n    inject:\n        - workdir\npackage_state:\n    compatibility_import: package-owned\n    owner_ids:\n        - demo\nbuild:\n    source:\n        script: scripts/build.sh\ntest:\n    script: tests/run.sh\n"

	got, err := yaml.Marshal(PackageManifest{
		Schema:                           1,
		Name:                             "demo",
		Version:                          "1.2.3",
		Title:                            "Demo",
		Description:                      "package description",
		Author:                           "ACME",
		Website:                          "https://example.com",
		License:                          "Apache-2.0",
		Icon:                             "assets/icon.png",
		Artwork:                          map[string]string{"vscode": "assets/vscode.png"},
		Kind:                             "workflow",
		Provider:                         "acme",
		Capability:                       "workflow.demo",
		PrimitiveYAMLDeprecated:          "workflow.legacy",
		Namespace:                        "example",
		Tags:                             []string{"demo"},
		Keywords:                         []string{"sample"},
		MinDockpipeVersion:               "1.0.0",
		Repository:                       "https://example.com/repo",
		Provides:                         []string{"demo"},
		RequiresCapabilities:             []string{"cli.demo"},
		RequiresPrimitivesYAMLDeprecated: []string{"cli.legacy"},
		RequiresResolvers:                []string{"demo"},
		IncludesResolvers:                []string{"demo-alt"},
		Depends:                          []string{"base"},
		Platforms:                        []string{"linux"},
		Dependencies: modeldependency.DependencySpec{Host: []modeldependency.HostDependency{{
			ID:          "git",
			Command:     "git",
			Description: "version control",
			Required:    &required,
			Install:     modeldependency.HostDependencyInstallHint{Linux: "install git"},
		}}},
		AllowClone:   true,
		Distribution: "source",
		Image: PackageImageSpec{
			Source:     "registry",
			Ref:        "example/demo:1.2.3",
			PullPolicy: "if-missing",
		},
		ScriptContract: PackageScriptContract{Inject: []string{"workdir"}},
		PackageState: PackageStateSpec{
			CompatibilityImport: "package-owned",
			OwnerIDs:            []string{"demo"},
		},
		Build: PackageBuildSpec{Source: &PackageSourceBuildSpec{Script: "scripts/build.sh"}},
		Test:  PackageTestSpec{Script: "tests/run.sh"},
	})
	if err != nil {
		t.Fatalf("marshal package manifest: %v", err)
	}
	if string(got) != want {
		t.Fatalf("package manifest YAML changed:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestManifestYAMLRoundTrip(t *testing.T) {
	const authored = "schema: 1\nname: demo\nversion: 1.2.3\ndependencies:\n  host:\n    - id: git\n      command: git\npackage_state:\n  compatibility_import: package-owned\n  owner_ids: [demo]\nbuild:\n  source:\n    script: scripts/build.sh\ntest:\n  script: tests/run.sh\n"

	var first PackageManifest
	if err := yaml.Unmarshal([]byte(authored), &first); err != nil {
		t.Fatalf("unmarshal package manifest: %v", err)
	}
	encoded, err := yaml.Marshal(first)
	if err != nil {
		t.Fatalf("marshal package manifest: %v", err)
	}
	var second PackageManifest
	if err := yaml.Unmarshal(encoded, &second); err != nil {
		t.Fatalf("unmarshal round-trip package manifest: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("package manifest round trip changed shape:\nfirst: %#v\nsecond: %#v", first, second)
	}
	if len(second.Dependencies.Host) != 1 || second.Dependencies.Host[0].ID != "git" {
		t.Fatalf("nested dependency shape changed: %#v", second.Dependencies)
	}
}
