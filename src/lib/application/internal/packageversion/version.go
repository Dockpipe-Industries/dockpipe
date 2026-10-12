// Package packageversion resolves the version used for authored package output.
package packageversion

import (
	"os"
	"path/filepath"
	"strings"
)

const Default = "0.0.0"

// Authored returns the trimmed checkout VERSION or the package default when unavailable.
func Authored(workdir string) string {
	workdir = strings.TrimSpace(workdir)
	if workdir != "" {
		if value, err := os.ReadFile(filepath.Join(workdir, "VERSION")); err == nil {
			if version := strings.TrimSpace(string(value)); version != "" {
				return version
			}
		}
	}
	return Default
}
