package xdg

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDefaultsAndOverrides(t *testing.T) {
	home := t.TempDir()
	p, err := Resolve(home, map[string]string{"XDG_RUNTIME_DIR": filepath.Join(home, "run")})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(p.Images, "/.cache/dockpipe/vm/images") || !strings.HasSuffix(p.Instances, "/.local/state/dockpipe/vm/instances") {
		t.Fatalf("unexpected defaults: %+v", p)
	}
	if strings.Contains(p.Images+p.Instances+p.Evidence, "bin/.dockpipe") {
		t.Fatalf("checkout fallback leaked into XDG layout: %+v", p)
	}
	override := filepath.Join(home, "custom")
	p, err = Resolve(home, map[string]string{
		"XDG_CACHE_HOME": override + "/cache", "XDG_STATE_HOME": override + "/state",
		"XDG_CONFIG_HOME": override + "/config", "XDG_RUNTIME_DIR": override + "/run",
	})
	if err != nil || p.Config != override+"/config/dockpipe/vm" {
		t.Fatalf("override resolution failed: %+v, %v", p, err)
	}
}

func TestResolveRejectsMissingRuntimeAndRelativeRoots(t *testing.T) {
	if _, err := Resolve(t.TempDir(), nil); err == nil {
		t.Fatal("expected missing XDG_RUNTIME_DIR rejection")
	}
	if _, err := Resolve(t.TempDir(), map[string]string{"XDG_RUNTIME_DIR": "relative"}); err == nil {
		t.Fatal("expected relative runtime rejection")
	}
}
