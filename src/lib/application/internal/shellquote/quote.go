// Package shellquote owns shell-specific single-argument quoting.
package shellquote

import "strings"

// POSIX quotes one value for a POSIX-compatible shell.
func POSIX(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
