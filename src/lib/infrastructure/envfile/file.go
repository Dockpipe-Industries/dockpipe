// Package envfile parses DockPipe dotenv-style environment files.
package envfile

import (
	"os"
)

// ParseFile reads KEY=VAL lines (dotenv-style). Skips comments and blanks.
func ParseFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseEnvReader(f)
}
