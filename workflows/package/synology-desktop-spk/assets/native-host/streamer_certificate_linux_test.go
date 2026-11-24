//go:build linux

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"dockpipe/workflows/internal/synology/endpointtls"
)

func TestLegacyStreamerCertificateMigrationPreservesContentsAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cert.pem")
	contents := []byte("existing public certificate\n")
	if err := os.WriteFile(path, contents, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := endpointtls.File(path, 0600, os.Geteuid()); err == nil {
		t.Fatal("fixture must reproduce the legacy readiness rejection")
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := normalizeStreamerCertificate(path); err != nil {
			t.Fatal(err)
		}
		actual, err := endpointtls.File(path, 0600, os.Geteuid())
		if err != nil || !bytes.Equal(actual, contents) {
			t.Fatal("migrated certificate changed or failed the gateway's file admission")
		}
	}
}

func TestStreamerCertificateMigrationAllowsFreshMissingCertificate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cert.pem")
	if err := normalizeStreamerCertificate(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("migration created a replacement certificate")
	}
}

func TestStreamerCertificateMigrationRejectsUnsafeEntries(t *testing.T) {
	for _, scenario := range []string{"group-writable", "world-writable", "symlink", "directory", "fifo"} {
		t.Run(scenario, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "cert.pem")
			target := filepath.Join(directory, "target.pem")
			contents := []byte("unchanged certificate\n")
			if err := os.WriteFile(target, contents, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(target, 0644); err != nil {
				t.Fatal(err)
			}
			var err error
			switch scenario {
			case "symlink":
				err = os.Symlink(target, path)
			case "directory":
				err = os.Mkdir(path, 0700)
			case "fifo":
				err = syscall.Mkfifo(path, 0600)
			default:
				err = os.WriteFile(path, contents, 0600)
				if err == nil {
					permissions := os.FileMode(0664)
					if scenario == "world-writable" {
						permissions = 0666
					}
					err = os.Chmod(path, permissions)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := normalizeStreamerCertificate(path); err == nil {
				t.Fatal("unsafe certificate entry was accepted")
			}
			after, err := os.Lstat(path)
			if err != nil || before.Mode() != after.Mode() {
				t.Fatal("rejected certificate metadata changed")
			}
			actual, err := endpointtls.File(target, 0644, os.Geteuid())
			if err != nil || !bytes.Equal(actual, contents) {
				t.Fatal("migration changed a symlink target or unrelated certificate")
			}
		})
	}
}
