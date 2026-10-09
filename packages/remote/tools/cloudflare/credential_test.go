//go:build linux || darwin

package cloudflare

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCredentialPermissions(t *testing.T) {
	for _, mode := range []os.FileMode{0o400, 0o600, 0o000, 0o200, 0o440, 0o644, 0o700} {
		t.Run(fmt.Sprintf("%04o", mode), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "credential.json")
			if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, mode); err != nil {
				t.Fatal(err)
			}
			err := privateCredential(path)
			allowed := mode == 0o400 || mode == 0o600
			if (err == nil) != allowed {
				t.Fatalf("mode %04o: %v", mode, err)
			}
			info, err := os.Lstat(path)
			if err != nil || info.Mode().Perm() != mode {
				t.Fatalf("validation changed permissions: %v", err)
			}
		})
	}
}

func TestCredentialRejectsLinkedPaths(t *testing.T) {
	root := t.TempDir()
	private := filepath.Join(root, "private")
	if err := os.Mkdir(private, 0o700); err != nil {
		t.Fatal(err)
	}
	credential := filepath.Join(private, "credential")
	if err := os.WriteFile(credential, []byte("fixture"), 0o400); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"file-link": credential, "directory-link": private} {
		link := filepath.Join(root, name)
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		if name == "directory-link" {
			link = filepath.Join(link, "credential")
		}
		if err := privateCredential(link); err == nil {
			t.Fatalf("accepted linked credential path: %s", name)
		}
	}
}
