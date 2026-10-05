package infrastructure

import "dockpipe/src/lib/infrastructure/envfile"

// ParseEnvFile reads KEY=VAL lines (dotenv-style). Skips comments and blanks.
func ParseEnvFile(path string) (map[string]string, error) {
	return envfile.ParseFile(path)
}

// ParseEnvBytes parses KEY=VAL lines (dotenv-style) from UTF-8 bytes. Skips comments and blanks.
func ParseEnvBytes(data []byte) (map[string]string, error) {
	return envfile.ParseBytes(data)
}
