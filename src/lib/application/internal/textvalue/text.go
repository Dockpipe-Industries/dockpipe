// Package textvalue provides small normalization helpers shared by application leaves.
package textvalue

import "strings"

// FirstNonEmpty returns the first non-empty trimmed value.
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// FirstNonBlank returns the first value containing non-whitespace bytes without
// changing the selected value.
func FirstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
