package infrastructure

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func surfaceFixture(t *testing.T) (string, map[string]any) {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"snapshot.yml", "action.yml"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("name: fixture\nsteps: []\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root, map[string]any{
		"schema": 1, "id": "fixture.resources", "title": "Resources", "snapshot": "snapshot.yml",
		"actions": []any{map[string]any{"id": "open", "title": "Open", "scope": "resource", "workflow": "action.yml"}},
	}
}

func writeSurface(t *testing.T, root string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "surface.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestPackageSurfaceProjectsContainedFilesAndPublicWorkflow(t *testing.T) {
	root, descriptor := surfaceFixture(t)
	descriptor["actions"] = append(descriptor["actions"].([]any), map[string]any{"id": "create", "title": "Create", "scope": "surface", "workflow_id": "fixture-create"})
	writeSurface(t, root, descriptor)
	surface, err := LoadPackageSurface(root, "surface.json")
	if err != nil {
		t.Fatal(err)
	}
	if surface.Snapshot != filepath.Join(root, "snapshot.yml") || surface.Actions[0].Workflow != filepath.Join(root, "action.yml") || surface.Actions[1].WorkflowID != "fixture-create" {
		t.Fatalf("incorrect projection: %+v", surface)
	}
}

func TestPackageSurfaceRejectsMalformedContracts(t *testing.T) {
	mutations := map[string]func(map[string]any){
		"schema":           func(value map[string]any) { value["schema"] = 2 },
		"id":               func(value map[string]any) { value["id"] = "../escape" },
		"unknown field":    func(value map[string]any) { value["command"] = "execute" },
		"traversal":        func(value map[string]any) { value["snapshot"] = "../outside.yml" },
		"missing workflow": func(value map[string]any) { value["snapshot"] = "absent.yml" },
		"title capacity":   func(value map[string]any) { value["title"] = strings.Repeat("x", 161) },
		"ambiguous action": func(value map[string]any) { value["actions"].([]any)[0].(map[string]any)["workflow_id"] = "other" },
		"duplicate action": func(value map[string]any) {
			value["actions"] = append(value["actions"].([]any), value["actions"].([]any)[0])
		},
		"action capacity": func(value map[string]any) { value["actions"] = make([]any, MaxPackageSurfaceActions+1) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			root, value := surfaceFixture(t)
			mutate(value)
			writeSurface(t, root, value)
			if _, err := LoadPackageSurface(root, "surface.json"); err == nil {
				t.Fatal("expected invalid surface rejection")
			}
		})
	}
}

func TestPackageSurfaceRejectsLinkedEscapeAndOversizedInput(t *testing.T) {
	root, value := surfaceFixture(t)
	outside := filepath.Join(t.TempDir(), "outside.yml")
	if err := os.WriteFile(outside, []byte("name: outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked.yml")); err == nil {
		value["snapshot"] = "linked.yml"
		writeSurface(t, root, value)
		if _, err := LoadPackageSurface(root, "surface.json"); err == nil {
			t.Fatal("linked escape accepted")
		}
	}
	if err := os.WriteFile(filepath.Join(root, "surface.json"), []byte(strings.Repeat(" ", MaxPackageSurfaceBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPackageSurface(root, "surface.json"); err == nil {
		t.Fatal("oversized descriptor accepted")
	}
}

func TestPackageSurfaceEditorSchemaAcceptsOnlyOneWorkflowReference(t *testing.T) {
	data, err := os.ReadFile("schema/package-surface.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := jsonschema.CompileString("package-surface.schema.json", string(data))
	if err != nil {
		t.Fatal(err)
	}
	_, value := surfaceFixture(t)
	if err := schema.Validate(value); err != nil {
		t.Fatal(err)
	}
	value["actions"].([]any)[0].(map[string]any)["workflow_id"] = "ambiguous"
	if err := schema.Validate(value); err == nil {
		t.Fatal("editor accepted ambiguous workflow references")
	}
}

func TestPackageNativeDisplaySchema(t *testing.T) {
	data, err := os.ReadFile("schema/package-native-display.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := jsonschema.CompileString("package-native-display.schema.json", string(data))
	if err != nil {
		t.Fatal(err)
	}
	display := map[string]any{"kind": "native-window", "process_id": 42, "process_start_token": "boot:100"}
	if err := schema.Validate(display); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []any{0, -1, 1.5, 2147483648, "42"} {
		display["process_id"] = invalid
		if err := schema.Validate(display); err == nil {
			t.Fatalf("accepted invalid display process ID: %v", invalid)
		}
	}
	display["process_id"] = 42
	for _, invalid := range []string{"", "boot:100\n", strings.Repeat("x", 257)} {
		display["process_start_token"] = invalid
		if err := schema.Validate(display); err == nil {
			t.Fatal("accepted invalid display process token")
		}
	}
}

func TestPackageSurfaceResourceLaunchSchema(t *testing.T) {
	schema, err := jsonschema.Compile("schema/package-surface-resource.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	resource := map[string]any{
		"id": "fixture", "title": "Fixture", "state": "running",
		"details": map[string]any{}, "actions": []any{},
		"launch_id": "launch-123",
		"display":   map[string]any{"kind": "native-window", "process_id": 42, "process_start_token": "boot:100"},
	}
	if err := schema.Validate(resource); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []any{"", "unsafe launch", strings.Repeat("x", 129), 42} {
		resource["launch_id"] = invalid
		if err := schema.Validate(resource); err == nil {
			t.Fatal("accepted invalid resource launch correlation")
		}
	}
	delete(resource, "launch_id")
	if err := schema.Validate(resource); err != nil {
		t.Fatalf("legacy uncorrelated resource rejected: %v", err)
	}
}

func TestPackageSurfaceNativeFactoryContainedAndImmutable(t *testing.T) {
	root, descriptor := surfaceFixture(t)
	if err := os.WriteFile(filepath.Join(root, "display.so"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	descriptor["native_display"] = map[string]any{"interface": 1, "libraries": map[string]string{"linux": "display.so"}}
	writeSurface(t, root, descriptor)
	surface, err := LoadPackageSurface(root, "surface.json")
	if err != nil {
		t.Fatal(err)
	}
	if surface.NativeDisplay.PackageRoot != root || surface.NativeDisplay.Libraries["linux"] != filepath.Join(root, "display.so") {
		t.Fatalf("incorrect native library projection: %+v", surface.NativeDisplay)
	}
	for _, invalid := range []any{
		map[string]any{"interface": 2, "libraries": map[string]string{"linux": "display.so"}},
		map[string]any{"interface": 1, "libraries": map[string]string{"linux": "../outside.so"}},
		map[string]any{"interface": 1, "libraries": map[string]string{"unknown": "display.so"}},
		map[string]any{"interface": 1, "libraries": map[string]string{}},
		map[string]any{"interface": 1, "package_root": root, "libraries": map[string]string{"linux": "display.so"}},
	} {
		descriptor["native_display"] = invalid
		writeSurface(t, root, descriptor)
		if _, err := LoadPackageSurface(root, "surface.json"); err == nil {
			t.Fatalf("accepted invalid native factory: %+v", invalid)
		}
	}
}

func TestPackageSurfaceLaunchEnvironmentReferences(t *testing.T) {
	root, descriptor := surfaceFixture(t)
	asset := filepath.Join(root, "native-tool")
	if err := os.WriteFile(asset, []byte("fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	action := map[string]any{"id": "start", "title": "Start", "scope": "surface", "workflow_id": "fixture-start",
		"launch_environment": map[string]any{"linux": map[string]any{"TOOL_BIN": map[string]any{"asset": "native-tool"}, "REAL_BIN": map[string]any{"inherit": "TOOL_BIN"}}}}
	descriptor["actions"] = []any{action}
	writeSurface(t, root, descriptor)
	surface, err := LoadPackageSurface(root, "surface.json")
	if err != nil {
		t.Fatal(err)
	}
	if surface.PackageRoot != root || surface.Actions[0].LaunchEnvironment["linux"]["TOOL_BIN"].Asset != asset {
		t.Fatalf("incorrect contained action projection: %+v", surface)
	}
	for _, invalid := range []any{
		nil,
		map[string]any{},
		map[string]any{"linux": map[string]any{}},
		map[string]any{"linux": map[string]any{"TOOL_BIN": map[string]any{"asset": "../outside"}}},
		map[string]any{"linux": map[string]any{"BAD-NAME": map[string]any{"inherit": "TOOL_BIN"}}},
		map[string]any{"linux": map[string]any{"TOOL_BIN": map[string]any{"asset": "native-tool", "inherit": "SOURCE"}}},
		map[string]any{"linux": map[string]any{"TOOL_BIN": map[string]any{"value": "plaintext"}}},
		map[string]any{"unknown": map[string]any{"TOOL_BIN": map[string]any{"asset": "native-tool"}}},
	} {
		action["launch_environment"] = invalid
		writeSurface(t, root, descriptor)
		if _, err := LoadPackageSurface(root, "surface.json"); err == nil {
			t.Fatalf("accepted invalid action environment: %+v", invalid)
		}
	}
}
