package hostdependencies

import (
	"errors"
	"os"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestServicesKeepLookupAndInstallerDependenciesSeparate(t *testing.T) {
	t.Setenv("DOCKPIPE_APPROVE_PROMPTS", "0")
	installed := false
	installerCalls := 0
	adapters := missingToolAdapters()
	adapters.LookupCommand = func(string) (string, error) {
		if installed {
			return "/fixture/tool", nil
		}
		return "", os.ErrNotExist
	}
	adapters.RunInstaller = func(string) error {
		installerCalls++
		installed = true
		return nil
	}
	installing := New(adapters)

	// Replacing the composition inputs must not change a previously created service.
	adapters.LookupCommand = func(string) (string, error) {
		return "", os.ErrNotExist
	}
	adapters.RunInstaller = func(string) error {
		t.Fatal("an unapproved service ran an installer")
		return nil
	}
	declining := New(adapters)
	request := dependencyRequest(t.TempDir())
	request.ApproveSystemChanges = true
	err := installing.Check(request)
	if CurrentPlatform() == "flatpak" {
		if err == nil || !strings.Contains(err.Error(), "missing required host dependencies") {
			t.Fatalf("Flatpak must report the missing dependency: %v", err)
		}
		if installerCalls != 0 {
			t.Fatalf("Flatpak installer calls = %d, want 0", installerCalls)
		}
		// A tool becoming available must still use this service's captured lookup.
		installed = true
		if err := installing.Check(request); err != nil {
			t.Fatal(err)
		}
	} else {
		if err != nil {
			t.Fatal(err)
		}
		if installerCalls != 1 {
			t.Fatalf("installer calls = %d, want 1", installerCalls)
		}
	}
	request.ApproveSystemChanges = false
	if err := declining.Check(request); err == nil {
		t.Fatal("separate service inherited the successful lookup or approval")
	}
}

func TestServicePreservesInstallerFailure(t *testing.T) {
	installationFailure := errors.New("fixture installer failure")
	installerCalls := 0
	adapters := missingToolAdapters()
	adapters.RunInstaller = func(string) error {
		installerCalls++
		return installationFailure
	}
	request := dependencyRequest(t.TempDir())
	request.ApproveSystemChanges = true
	err := New(adapters).Check(request)
	if CurrentPlatform() == "flatpak" {
		if err == nil || !strings.Contains(err.Error(), "missing required host dependencies") {
			t.Fatalf("Flatpak must report the missing dependency: %v", err)
		}
		if installerCalls != 0 {
			t.Fatalf("Flatpak installer calls = %d, want 0", installerCalls)
		}
	} else {
		if !errors.Is(err, installationFailure) {
			t.Fatalf("installer failure was not preserved: %v", err)
		}
		if installerCalls != 1 {
			t.Fatalf("installer calls = %d, want 1", installerCalls)
		}
	}
}

func dependencyRequest(workdir string) Request {
	return Request{
		Workflow: &domain.Workflow{
			Dependencies: domain.DependencySpec{
				Host: []domain.HostDependency{{
					Command: "fixture-tool",
					Install: domain.HostDependencyInstallHint{
						Windows: "fixture-install",
						MacOS:   "fixture-install",
						Deb:     "fixture-install",
						Linux:   "fixture-install",
					},
				}},
			},
		},
		WorkflowRoot: workdir,
		Workdir:      workdir,
	}
}

func missingToolAdapters() Adapters {
	missing := func(string) (string, error) { return "", os.ErrNotExist }
	return Adapters{
		LookupCommand:    missing,
		LookupPowerShell: missing,
		RunInstaller:     func(string) error { return nil },
		PackageToolsDir:  func(string) string { return "" },
		BundledCommandPath: func(string, string) (string, error) {
			return "", os.ErrNotExist
		},
		TerminalAvailable: func() bool { return false },
	}
}
