package application

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"dockpipe/src/lib/model/managedenvironment"
)

func TestEnvironmentDefinitionScopesAndExclusiveCreate(t *testing.T) {
	user := t.TempDir()
	project := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", user)
	definition := managedenvironment.Definition{
		Schema: managedenvironment.Schema,
		Name:   "dev-vm",
		Kind:   "vm",
		Provision: &managedenvironment.Workflow{
			Workflow: "linux-vm",
			Package:  "vm",
		},
	}

	options := []string{"generate", "--name", definition.Name, "--kind", definition.Kind,
		"--workflow", definition.Provision.Workflow, "--package", definition.Provision.Package}
	if err := cmdEnvironment(append(append([]string(nil), options...), "--personal"), nil); err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(project, "dev-vm.yaml")
	if err := cmdEnvironment(append(options, "--file", shared), nil); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{filepath.Join(user, "environments", "dev-vm.yaml"), shared} {
		got, err := readEnvironment(path)
		if err != nil || !reflect.DeepEqual(got, definition) {
			t.Fatalf("read %s = %#v, %v", path, got, err)
		}
		if err := saveNewEnvironment(path, definition); !errors.Is(err, os.ErrExist) {
			t.Fatalf("overwriting an existing definition returned %v", err)
		}
	}
	if _, err := personalEnvironmentPath("../shared", false); err == nil {
		t.Fatal("personal path accepted an environment name with directory traversal")
	}
}

func TestContainerEnvironmentPresetGeneratesSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dev-shell.yaml")
	args := []string{"generate", "--name", "dev-shell", "--kind", "container",
		"--preset", "ubuntu", "--network", "bridge", "--security", "restricted",
		"--user", "non-root", "--read-only", "--host-port", "8080", "--container-port", "80",
		"--data-path", "/data", "--file", path}
	if err := cmdEnvironment(args, nil); err != nil {
		t.Fatal(err)
	}

	definition, err := readEnvironment(path)
	if err != nil {
		t.Fatal(err)
	}
	want := &managedenvironment.Container{
		Image: "ubuntu:24.04", Startup: "keep-running", Network: "bridge", Security: "restricted",
		User: "non-root", ReadOnly: true, Publish: &managedenvironment.LocalPort{Host: 8080, Container: 80},
		DataPath: "/data",
	}
	if definition.Provision != nil || !reflect.DeepEqual(definition.Container, want) {
		t.Fatalf("generated unexpected settings: %#v", definition)
	}
}

func TestContainerEnvironmentRejectsIncompatibleSettings(t *testing.T) {
	for _, options := range [][]string{
		{"--preset", "unknown"},
		{"--preset", "ubuntu", "--image", "alpine:3.22"},
		{"--preset", "ubuntu", "--workflow", "setup"},
		{"--image", "alpine:3.22", "--network", "none", "--host-port", "8080", "--container-port", "80"},
		{"--image", "alpine:3.22", "--host-port", "0"},
		{"--image", "--privileged"},
		{"--image", "alpine:3.22", "--data-path", "/data,source=/"},
	} {
		args := append([]string{"generate", "--name", "dev-shell", "--kind", "container"}, options...)
		if err := cmdEnvironment(args, nil); err == nil {
			t.Errorf("accepted incompatible or malformed options %v", options)
		}
	}
}

func TestEnvironmentDefinitionRejectsUnexpectedInput(t *testing.T) {
	const valid = "schema: dockpipe.environment/v1\nname: dev-vm\nkind: vm\nprovision:\n  workflow: linux-vm\n"
	for _, test := range []struct{ name, content string }{
		{"unknown field", valid + "credentials: plaintext\n"},
		{"duplicate field", valid + "name: other\n"},
		{"extra document", valid + "---\n" + valid},
		{"oversized", valid + strings.Repeat(" ", maxEnvironmentDefinitionBytes)},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "environment.yaml")
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := readEnvironment(path); err == nil {
				t.Fatal("unexpected definition was accepted")
			}
		})
	}
}

func TestEnvironmentRejectsKubernetesPodBeforeProvisioning(t *testing.T) {
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	for _, kind := range []string{"kubepod", "kube-pod", "kubernetes-pod"} {
		t.Run(kind, func(t *testing.T) {
			directory := t.TempDir()
			generated := filepath.Join(directory, "generated.yaml")
			for _, command := range []string{"generate", "create"} {
				err := cmdEnvironment([]string{command, "--name", "dev-pod",
					"--kind", kind, "--workflow", "unused", "--file", generated}, nil)
				if !errors.Is(err, managedenvironment.ErrKubernetesPodUnsupported) {
					t.Fatalf("%s returned %v", command, err)
				}
			}
			if _, err := os.Stat(generated); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unsupported kind should leave no definition file, got %v", err)
			}

			saved := filepath.Join(directory, "dev-pod.yaml")
			content := "schema: dockpipe.environment/v1\nname: dev-pod\nkind: " + kind +
				"\nprovision:\n  workflow: unused\n"
			if err := os.WriteFile(saved, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			for _, command := range []string{"apply", "show"} {
				err := cmdEnvironment([]string{command, "--file", saved}, nil)
				if !errors.Is(err, managedenvironment.ErrKubernetesPodUnsupported) {
					t.Fatalf("%s returned %v", command, err)
				}
			}

			entry := inspectEnvironmentDefinition("explicit", saved, "")
			if entry.Error != "invalid" || entry.Reason != "unsupported-kind" ||
				entry.Definition != nil || entry.InstanceID != "" {
				t.Fatalf("unexpected inventory entry: %#v", entry)
			}
		})
	}
}

func TestEnvironmentDefinitionsCollectBothScopesWithoutHidingInvalidFiles(t *testing.T) {
	user := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", user)
	personal, err := personalEnvironmentPath("vm", true)
	if err != nil {
		t.Fatal(err)
	}
	definition := managedenvironment.Definition{
		Schema:    managedenvironment.Schema,
		Name:      "vm",
		Kind:      "vm",
		Provision: &managedenvironment.Workflow{Workflow: "linux-vm"},
		Storage:   &managedenvironment.Storage{Profile: "test-image"},
	}
	explicit := filepath.Join(t.TempDir(), "env.yml")
	for _, path := range []string{personal, explicit} {
		if err := saveNewEnvironment(path, definition); err != nil {
			t.Fatal(err)
		}
	}
	mismatched := filepath.Join(filepath.Dir(personal), "other.yaml")
	if err := saveNewEnvironment(mismatched, definition); err != nil {
		t.Fatal(err)
	}

	missing := filepath.Join(filepath.Dir(explicit), "missing.yaml")
	entries, err := collectEnvironmentDefinitions([]string{personal, explicit, explicit, missing})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected four distinct entries, got %d", len(entries))
	}
	if entries[0].Error != "name-mismatch" || entries[0].Definition != nil ||
		entries[3].Error != "missing" || entries[3].Definition != nil {
		t.Fatalf("expected per-file failures, got %#v", entries)
	}
	if entries[1].Scope != "personal" || entries[1].Path != personal ||
		entries[2].Scope != "explicit" || entries[2].Path != explicit {
		t.Fatalf("wrong scopes or order: %#v", entries)
	}
	if entries[1].InstanceID != "" || entries[2].InstanceID != "" {
		t.Fatal("reading saved definitions assigned a VM instance")
	}
	for _, entry := range entries[1:3] {
		if entry.Definition == nil || entry.Definition.Storage == nil ||
			entry.Definition.Storage.Profile != definition.Storage.Profile {
			t.Fatalf("expected the shared definition model, got %#v", entry)
		}
	}
	instance, _, err := environmentInstanceForDefinition(explicit)
	if err != nil {
		t.Fatal(err)
	}
	entries, err = collectEnvironmentDefinitions([]string{explicit})
	if err != nil || len(entries) != 3 || entries[2].InstanceID != instance.ID || entries[1].InstanceID != "" {
		t.Fatalf("definition inventory did not expose the scoped instance identity: %#v, %v", entries, err)
	}
}
