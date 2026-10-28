package remotecmd

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundleIdentifierOutputLimit(t *testing.T) {
	output := &bundleIdentifierOutput{}
	if _, err := io.Copy(output, strings.NewReader(strings.Repeat("a", 255)+"\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := output.Write([]byte("x")); err == nil {
		t.Fatal("accepted oversized identifier output")
	}
}

func TestServiceBundleIdentifierReadsNativePlists(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	contents := filepath.Join(root, "Example.app", "Contents")
	if err := os.MkdirAll(filepath.Join(contents, "Helpers"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(contents, "Info.plist")
	data := `<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>org.example.test</string></dict></plist>`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(contents, "Helpers", "dockpipe")
	for _, format := range []string{"xml1", "binary1"} {
		if err := exec.Command("/usr/bin/plutil", "-convert", format, path).Run(); err != nil {
			t.Fatal(err)
		}
		identifier, err := serviceBundleIdentifier(context.Background(), executable)
		if err != nil || identifier != "org.example.test" {
			t.Fatalf("format %s: identifier %q, error %v", format, identifier, err)
		}
	}
	if err := os.WriteFile(path, []byte("invalid plist"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := serviceBundleIdentifier(context.Background(), executable); err == nil {
		t.Fatal("bundled executable silently accepted invalid app metadata")
	}
	identifier, err := serviceBundleIdentifier(context.Background(), "/usr/local/bin/dockpipe")
	if err != nil || identifier != "" {
		t.Fatal("standalone CLI incorrectly requires application metadata")
	}
}

func TestServiceBundleIdentifierCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := serviceBundleIdentifier(ctx, "/Applications/Example.app/Contents/Helpers/dockpipe")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
