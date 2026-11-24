//go:build linux

package packageplatform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFlatpakDocumentPathNormalizesOnlyTheRuntimeAlias(t *testing.T) {
	runtimeDir := t.TempDir()
	alias := filepath.Join(runtimeDir, "doc")
	if err := os.Symlink("../../flatpak/doc", alias); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path string
		want string
	}{
		{alias + "/grant/project/package.yml", "/run/flatpak/doc/grant/project/package.yml"},
		{alias + "/grant/link/package.yml", "/run/flatpak/doc/grant/link/package.yml"},
		{alias + "-other/grant/package.yml", alias + "-other/grant/package.yml"},
		{alias + "/../package.yml", alias + "/../package.yml"},
		{"/run/flatpak/doc/grant/package.yml", "/run/flatpak/doc/grant/package.yml"},
	}
	for _, test := range tests {
		if got := flatpakDocumentPath(test.path, alias); got != test.want {
			t.Errorf("flatpakDocumentPath(%q) = %q, want %q", test.path, got, test.want)
		}
	}

	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/somewhere/else", alias); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(alias, "grant", "package.yml")
	if got := flatpakDocumentPath(path, alias); got != path {
		t.Errorf("rewrote unexpected link: got %q, want %q", got, path)
	}
}
