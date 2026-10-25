package environment

import (
	"runtime"
	"strings"
)

// MergeIfUnset adds keys from src into dst only when dst does not already have the key,
// or when the existing value is blank. Empty inherited env vars should not block workflow
// defaults, .env values, or typed input source mappings.
func MergeIfUnset(dst map[string]string, src map[string]string) {
	for k, v := range src {
		if cur, ok := dst[k]; !ok || strings.TrimSpace(cur) == "" {
			dst[k] = v
		}
	}
}

// EnvSliceToMap parses KEY=VAL from a slice (e.g. --env).
func EnvSliceToMap(lines []string) map[string]string {
	m := make(map[string]string)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if ok {
			m[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return m
}

// EnvironToMap converts KEY=VAL lines (e.g. os.Environ()) to a map.
func EnvironToMap(environ []string) map[string]string {
	return environToMap(environ, runtime.GOOS)
}

// NativeEnvKey canonicalizes host environment keys. Windows environment names
// are case-insensitive; Unix environment names remain case-sensitive.
func NativeEnvKey(key string) string {
	return nativeEnvKey(key, runtime.GOOS)
}

func nativeEnvKey(key, operatingSystem string) string {
	if operatingSystem == "windows" {
		return strings.ToUpper(key)
	}
	return key
}

func environToMap(environ []string, operatingSystem string) map[string]string {
	m := make(map[string]string)
	for _, e := range environ {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			m[nativeEnvKey(k, operatingSystem)] = v
		}
	}
	return m
}

// EnvMapToSlice converts a map to KEY=VAL lines for subprocess env (order undefined).
func EnvMapToSlice(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}
