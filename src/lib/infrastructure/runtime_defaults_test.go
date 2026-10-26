package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuiltinRuntimeDefaultsWithoutAssetTree(t *testing.T) {
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	t.Setenv("DOCKPIPE_SYSTEM_ROOT", t.TempDir())
	for _, name := range []string{"dockerfile", "dockerimage", "package", "vm", "vmimage", "docker", "cli"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			values, err := LoadIsolationProfile(root, name, "")
			if err != nil {
				t.Fatal(err)
			}
			want := NormalizeRuntimeProfileName(name)
			if name == "vm" {
				want = "vmimage"
			}
			if values["DOCKPIPE_RUNTIME_SUBSTRATE"] != want {
				t.Fatalf("defaults = %#v, want substrate %q", values, want)
			}
			if _, exists := values["DOCKPIPE_RUNTIME_HOST_SCRIPT"]; exists {
				t.Fatal("builtin defaults must not select a package execution adapter")
			}
			if want == "vmimage" && values["DOCKPIPE_RUNTIME_HOST_REQUIRES_DOCKER"] != "0" {
				t.Fatal("VM substrate unexpectedly requires Docker")
			}
			if _, err := ResolveRuntimeFilePath(root, name); err == nil {
				t.Fatal("builtin defaults must not pretend to have a profile file")
			}
		})
	}
}

func TestRuntimeDefaultsOverlayOrder(t *testing.T) {
	root := t.TempDir()
	writeRuntimeFile(t, root, "dockerimage/profile", "DOCKPIPE_RUNTIME_SUBSTRATE=custom\nDOCKPIPE_RUNTIME_CMD=runtime\n")
	writeResolverFile(t, root, "adapter/profile", "DOCKPIPE_RUNTIME_CMD=resolver\nDOCKPIPE_RESOLVER_HOST_ISOLATE=assets/runner.sh\n")
	values, err := LoadIsolationProfile(root, "dockerimage", "adapter")
	if err != nil {
		t.Fatal(err)
	}
	if values["DOCKPIPE_RUNTIME_SUBSTRATE"] != "custom" || values["DOCKPIPE_RUNTIME_CMD"] != "resolver" {
		t.Fatalf("overlay order = %#v", values)
	}
	if values["DOCKPIPE_RESOLVER_HOST_ISOLATE"] != "assets/runner.sh" {
		t.Fatalf("resolver adapter missing: %#v", values)
	}
}

func TestRuntimeProfileLookup(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	t.Setenv("DOCKPIPE_SYSTEM_ROOT", t.TempDir())
	store := t.TempDir()
	t.Setenv("DOCKPIPE_PACKAGES_ROOT", store)
	installed := filepath.Join(store, "runtimes", "custom", "profile")
	if err := os.MkdirAll(filepath.Dir(installed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installed, []byte("DOCKPIPE_RUNTIME_CMD=installed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRuntimeFile(t, root, "custom", "DOCKPIPE_RUNTIME_CMD=project\n")
	for _, selection := range []struct{ name, want string }{
		{"custom", installed},
		{"runtimes/custom", filepath.Join(root, "runtimes", "custom")},
		{filepath.Dir(installed), installed},
	} {
		got, err := ResolveRuntimeFilePath(root, selection.name)
		if err != nil || got != selection.want {
			t.Fatalf("ResolveRuntimeFilePath(%q) = %q, %v; want %q", selection.name, got, err, selection.want)
		}
	}
}
