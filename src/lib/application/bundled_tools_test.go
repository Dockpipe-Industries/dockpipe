package application

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestBundledToolsSatisfyPreflightAndExecuteInPackageScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	root := t.TempDir()
	tools := filepath.Join(root, "assets", "tooling", "bin")
	if err := os.MkdirAll(tools, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(root, "package.yml"):      "schema: 1\nname: bundled-example\nkind: workflow\nversion: 1.0.0\n",
		filepath.Join(root, "config.yml"):       "name: bundled-example\n",
		filepath.Join(tools, "fixture-helper"):  "#!/bin/sh\nprintf 'bundled-tool-ok'\n",
		filepath.Join(root, "assets", "run.sh"): "#!/bin/sh\nfixture-helper\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	wf := &domain.Workflow{Dependencies: domain.DependencySpec{
		Host: []domain.HostDependency{{ID: "fixture-helper"}},
	}}
	if err := checkWorkflowHostDependencies(wf, root, filepath.Join(root, "config.yml"), nil); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "assets", "run.sh")
	command := exec.Command("/bin/sh", script)
	command.Env = envSliceWithScriptContext(os.Environ(), script)
	output, err := command.CombinedOutput()
	if err != nil || string(output) != "bundled-tool-ok" {
		t.Fatalf("package helper execution: %q, %v", output, err)
	}
	if _, err := bundledDependencyCommandPath(tools, "../fixture-helper"); err == nil {
		t.Fatal("accepted dependency path traversal")
	}
	if err := os.Chmod(filepath.Join(tools, "fixture-helper"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkWorkflowHostDependencies(wf, root, filepath.Join(root, "config.yml"), nil); err == nil {
		t.Fatal("non-executable file satisfied dependency preflight")
	}
}

func TestFlatpakDependenciesNeverOfferNativeInstallers(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Flatpak is a Linux execution environment")
	}
	t.Setenv("FLATPAK_ID", "org.example.Fixture")
	if got := currentDependencyPlatform(); got != "flatpak" {
		t.Fatalf("execution platform = %q", got)
	}
	if !dependencyPlatformsSupportCurrent([]string{"flatpak"}) || dependencyPlatformsSupportCurrent([]string{"linux", "deb"}) {
		t.Fatal("Flatpak must require explicit platform support")
	}
	dep := domain.HostDependency{Install: domain.HostDependencyInstallHint{
		Linux: "native-installer", Deb: "apt-get install tool",
	}}
	if hint := installCommandForCurrentPlatform(dep); hint != "" {
		t.Fatalf("offered native installer inside Flatpak: %q", hint)
	}
	if err := domain.ValidatePlatformList("platforms", []string{"flatpak"}); err != nil {
		t.Fatal(err)
	}
}
