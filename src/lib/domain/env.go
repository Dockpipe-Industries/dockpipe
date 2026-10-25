package domain

import "dockpipe/src/lib/domain/environment"

// MergeIfUnset adds defaults without replacing an existing nonblank value.
func MergeIfUnset(dst map[string]string, src map[string]string) {
	environment.MergeIfUnset(dst, src)
}

// EnvSliceToMap parses KEY=VAL entries from command arguments.
func EnvSliceToMap(lines []string) map[string]string {
	return environment.EnvSliceToMap(lines)
}

// EnvironToMap converts inherited process environment entries to a map.
func EnvironToMap(environ []string) map[string]string {
	return environment.EnvironToMap(environ)
}

// NativeEnvKey canonicalizes a key for the host environment.
func NativeEnvKey(key string) string {
	return environment.NativeEnvKey(key)
}

// EnvMapToSlice converts environment values to subprocess entries.
func EnvMapToSlice(values map[string]string) []string {
	return environment.EnvMapToSlice(values)
}

// BranchPrefixForTemplate maps template/resolver flavor to git branch prefix.
func BranchPrefixForTemplate(t string) string {
	switch t {
	case "claude", "agent-dev":
		return "claude"
	case "codex":
		return "codex"
	default:
		return "dockpipe"
	}
}
