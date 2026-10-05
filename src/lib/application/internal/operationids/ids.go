// Package operationids builds the stable identifiers attached to application operation results.
package operationids

import (
	"os"
	"path/filepath"
	"strings"
)

// Build returns the common project and optional workflow identifiers for an operation.
func Build(workdir, workflow string) map[string]string {
	ids := map[string]string{
		"project": filepath.Base(strings.TrimRight(filepath.Clean(workdir), string(os.PathSeparator))),
	}
	if strings.TrimSpace(workflow) != "" {
		ids["workflow"] = strings.TrimSpace(workflow)
	}
	return ids
}

// Merge combines identifier maps, trimming and omitting empty keys and values.
func Merge(base map[string]string, extra map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(extra))
	for key, value := range base {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	for key, value := range extra {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
