package remotecmd

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceBundleInfoPath(t *testing.T) {
	for _, directory := range []string{"Helpers", "MacOS"} {
		executable := filepath.FromSlash("/Applications/Dock Pipe.app/Contents/" + directory + "/dockpipe")
		if path := serviceBundleInfoPath(executable); path != filepath.FromSlash("/Applications/Dock Pipe.app/Contents/Info.plist") {
			t.Fatalf("incorrect app metadata path: %q", path)
		}
	}
	for _, executable := range []string{"/usr/local/bin/dockpipe", "/tmp/Contents/Helpers/dockpipe", "/tmp/Test.app/Other/Helpers/dockpipe", "/tmp/Test.app/Contents/Resources/dockpipe"} {
		if path := serviceBundleInfoPath(executable); path != "" {
			t.Fatalf("standalone executable attributed to app: %q", path)
		}
	}
}

func TestServiceBundleIdentifierValidation(t *testing.T) {
	for _, identifier := range []string{"com.dockpipe.launcher", "org.example.Test-App"} {
		if err := validateServiceBundleIdentifier(identifier); err != nil {
			t.Fatal(err)
		}
	}
	for _, identifier := range []string{"", "dockpipe", "com.example\n", "com.<example>", strings.Repeat("a", 256) + ".app"} {
		if err := validateServiceBundleIdentifier(identifier); err == nil {
			t.Fatalf("accepted invalid identifier %q", identifier)
		}
	}
}

func TestLaunchAgentAssociatesOnlyEnclosingApplication(t *testing.T) {
	arguments := []string{"/Applications/Example.app/Contents/Helpers/dockpipe", "remote", "worker"}
	for _, identifier := range []string{"com.dockpipe.launcher", "org.example.development"} {
		manifest := launchAgent("test", arguments, "/tmp/worker.log", identifier)
		if !strings.Contains(manifest, "<key>AssociatedBundleIdentifiers</key><array><string>"+identifier+"</string></array>") {
			t.Fatal("enclosing application association missing")
		}
	}
	if strings.Contains(launchAgent("test", arguments, "/tmp/worker.log", ""), "AssociatedBundleIdentifiers") {
		t.Fatal("standalone CLI claims app association")
	}
}
