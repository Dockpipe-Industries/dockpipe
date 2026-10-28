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

func TestProjectDockpipeMinimumUsesSemverPrecedence(t *testing.T) {
	for _, test := range []struct {
		minimum string
		current string
		allowed bool
	}{
		{"0.6.11", "0.6.10", false},
		{"0.6.11", "0.6.11", true},
		{"0.6.11", "0.6.33", true},
		{"1.2.3", "1.2.3-rc.1", false},
		{"1.2.3", "1.2.4-rc.1", true},
		{"1.2.3-rc.2", "1.2.3-rc.1", false},
		{"1.2.3-rc.2", "1.2.3-rc.10", true},
		{"1.2.3-rc.2", "1.2.3", true},
		{"1.2.3+build.1", "v1.2.3+build.2", true},
		{"1.2.3", "dev", false},
		{"1.2.3", "dev+hash", false},
		{"1.2.3", "1.3", false},
	} {
		t.Run(test.minimum+"/"+test.current, func(t *testing.T) {
			config := &DockpipeProjectConfig{MinDockpipeVersion: &test.minimum}
			err := CheckProjectDockpipeVersion(config, test.current)
			if (err == nil) != test.allowed {
				t.Fatalf("allowed=%v, got %v", test.allowed, err)
			}
		})
	}
}

func TestProjectDockpipeMinimumRejectsInvalidDeclarations(t *testing.T) {
	for _, value := range []string{`null`, `123`, `true`, `[]`, `{}`, `""`, `"v1.2.3"`, `" 1.2.3"`, `"1.2.3 "`, `"latest"`, `"dev"`, `"1.2"`, `">=1.2.3"`, `"01.2.3"`, `"1.2.3-01"`} {
		t.Run(value, func(t *testing.T) {
			var config DockpipeProjectConfig
			err := json.Unmarshal([]byte(`{"min_dockpipe_version":`+value+`}`), &config)
			if err == nil {
				err = ValidateDockpipeProjectConfig(&config)
			}
			if err == nil {
				t.Fatal("invalid minimum accepted")
			}
		})
	}
}

func TestProjectDockpipeExactAndMinimumMustAgree(t *testing.T) {
	minimum := "1.2.3"
	for _, test := range []struct {
		exact string
		valid bool
	}{
		{"1.2.2", false},
		{"1.2.3-rc.1", false},
		{"1.2.3", true},
		{"1.2.3+build.1", true},
		{"1.2.4", true},
	} {
		config := &DockpipeProjectConfig{DockpipeVersion: &test.exact, MinDockpipeVersion: &minimum}
		if err := ValidateDockpipeProjectConfig(config); (err == nil) != test.valid {
			t.Errorf("exact %s valid=%v: %v", test.exact, test.valid, err)
		}
		if err := CheckProjectDockpipeVersion(config, test.exact); (err == nil) != test.valid {
			t.Errorf("runtime exact %s valid=%v: %v", test.exact, test.valid, err)
		}
	}
}

func TestProjectDockpipeExactPrereleaseIdentity(t *testing.T) {
	exact := "1.2.3-rc.1+build.1"
	config := &DockpipeProjectConfig{DockpipeVersion: &exact}
	for _, current := range []string{exact, "1.2.3-rc.2+build.1", "1.2.3-rc.1+build.2", "1.2.3+build.1"} {
		if err := CheckProjectDockpipeVersion(config, current); (err == nil) != (current == exact) {
			t.Errorf("exact prerelease identity %q: %v", current, err)
		}
	}
}
