package dependencylifecycle

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/domain/dependency"
)

func TestDependencyReceiptBoundsAndStrictWire(t *testing.T) {
	root := t.TempDir()
	valid := dependency.Inventory{Schema: dependency.InventorySchema, Provider: dependency.ProviderIdentity{ID: "fixture", Version: "1"}, Machine: "host", ObservedAt: time.Now(), Observations: []dependency.Observation{}, Gaps: []string{}}
	data, _ := json.Marshal(valid)
	for _, test := range []struct {
		name     string
		data     []byte
		accepted bool
	}{
		{"valid", data, true},
		{"trailing", append(append([]byte{}, data...), []byte("{}")...), false},
		{"unknown", []byte(`{"unexpected":"field"}`), false},
		{"oversized", bytes.Repeat([]byte(" "), dependency.MaxInventoryBytes+1), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(root, test.name)
			if err := os.WriteFile(path, test.data, 0600); err != nil {
				t.Fatal(err)
			}
			_, err := ReadInventory(path)
			if (err == nil) != test.accepted {
				t.Fatalf("receipt accepted=%t: %v", err == nil, err)
			}
		})
	}
	link := filepath.Join(root, "linked")
	if err := os.Symlink(filepath.Join(root, "valid"), link); err == nil {
		if _, err := ReadInventory(link); err == nil {
			t.Fatal("linked evidence accepted")
		}
	}
}

func TestDependencyWorkflowCannotEscapeResolver(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside.yml")
	if err := os.WriteFile(outside, []byte("name: outside\n"), 0600); err != nil {
		t.Fatal(err)
	}
	resolver := filepath.Join(root, "resolver")
	if err := os.Mkdir(resolver, 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"", "../outside.yml", outside} {
		if _, err := containedWorkflow(resolver, path); err == nil {
			t.Fatalf("escaped workflow %q", path)
		}
	}
	if err := os.Symlink(outside, filepath.Join(resolver, "linked.yml")); err == nil {
		if _, err := containedWorkflow(resolver, "linked.yml"); err == nil {
			t.Fatal("linked escape accepted")
		}
	}
}

func TestDependencyDiagnosticCopyCannotBypassLimit(t *testing.T) {
	var retained bytes.Buffer
	output := &boundedDiagnostics{writer: &retained, remaining: 4}
	if _, err := io.Copy(output, io.LimitReader(strings.NewReader("12345"), 5)); err == nil || retained.Len() > 4 {
		t.Fatal("diagnostic cap bypassed")
	}
}

func TestDependencyEnvironmentRejectsCallerReceiptOverrides(t *testing.T) {
	result := inventoryEnvironment([]string{"PATH=/usr/bin", "DOCKPIPE_ARTIFACT_ROOT=/untrusted", "DOCKPIPE_OUTPUT_ROOT=/untrusted", "DOCKPIPE_DEPENDENCY_OUTPUT=/untrusted", "DOCKPIPE_BIN=/untrusted"}, "/private/run", "/trusted/cli")
	if strings.Contains(strings.Join(result, "\n"), "/untrusted") {
		t.Fatal("caller output or CLI override leaked")
	}
	provider := WorkflowProvider{}
	if _, err := provider.Inspect(context.Background(), t.TempDir(), "../resolver", "host"); err == nil {
		t.Fatal("unsafe resolver accepted")
	}
}
