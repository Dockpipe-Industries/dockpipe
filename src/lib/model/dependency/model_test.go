package dependency

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSpecYAMLShape(t *testing.T) {
	required := true
	want := "host:\n    - id: git\n      command: git\n      description: version control\n      required: true\n      install:\n        windows: winget install Git.Git\n        macos: brew install git\n        linux: install git\n        deb: apt-get install git\n"

	got, err := yaml.Marshal(DependencySpec{Host: []HostDependency{{
		ID:          "git",
		Command:     "git",
		Description: "version control",
		Required:    &required,
		Install: HostDependencyInstallHint{
			Windows: "winget install Git.Git",
			MacOS:   "brew install git",
			Linux:   "install git",
			Deb:     "apt-get install git",
		},
	}}})
	if err != nil {
		t.Fatalf("marshal dependency spec: %v", err)
	}
	if string(got) != want {
		t.Fatalf("dependency YAML changed:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestSpecYAMLRoundTrip(t *testing.T) {
	const authored = "host:\n  - id: node\n    command: node\n    install:\n      windows: winget install OpenJS.NodeJS\n"

	var got DependencySpec
	if err := yaml.Unmarshal([]byte(authored), &got); err != nil {
		t.Fatalf("unmarshal dependency spec: %v", err)
	}
	if len(got.Host) != 1 || got.Host[0].ID != "node" || got.Host[0].Install.Windows != "winget install OpenJS.NodeJS" {
		t.Fatalf("unexpected dependency shape: %#v", got)
	}
}
