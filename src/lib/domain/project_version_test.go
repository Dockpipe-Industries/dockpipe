package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestProjectDockpipeVersionAcceptsOnlyExactCompleteSemver(t *testing.T) {
	for _, version := range []string{"0.6.24", "1.0.0-rc.1", "1.0.0+build.7"} {
		if err := ValidateExactDockpipeVersion(version); err != nil {
			t.Errorf("valid pin %q: %v", version, err)
		}
	}
	for _, version := range []string{"", "latest", "dev", "v0.6.24", "0.6", "^0.6.24", ">=0.6.24", " 0.6.24", "01.6.24", "1.0.0-01"} {
		t.Run(version, func(t *testing.T) {
			root := t.TempDir()
			data, err := json.Marshal(map[string]string{"dockpipe_version": version})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, DockpipeProjectConfigFileName), data, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadDockpipeProjectConfig(root); err == nil {
				t.Errorf("invalid pin %q accepted", version)
			}
		})
	}
}

func TestProjectDockpipeVersionRequiresExactBuildIdentity(t *testing.T) {
	required := "1.2.3+build.1"
	config := &DockpipeProjectConfig{DockpipeVersion: &required}
	for _, current := range []string{required, "v" + required} {
		if err := CheckProjectDockpipeVersion(config, current); err != nil {
			t.Errorf("matching identity: %v", err)
		}
	}
	for _, current := range []string{"1.2.4", "1.2.2", "1.2.3", "dev", "dev+hash", "1.2.3+build.2"} {
		if err := CheckProjectDockpipeVersion(config, current); err == nil {
			t.Errorf("mismatched identity %q accepted", current)
		}
	}
	if err := CheckProjectDockpipeVersion(&DockpipeProjectConfig{}, "dev"); err != nil {
		t.Fatalf("absent pin changes behavior: %v", err)
	}
}

func TestProjectDockpipeVersionRejectsNonStringPins(t *testing.T) {
	for _, value := range []string{"null", "123", "true", "[]", "{}"} {
		root := t.TempDir()
		data := []byte(`{"dockpipe_version":` + value + `}`)
		if err := os.WriteFile(filepath.Join(root, DockpipeProjectConfigFileName), data, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadDockpipeProjectConfig(root); err == nil {
			t.Errorf("non-string pin %s accepted", value)
		}
	}
}
