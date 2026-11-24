package infrastructure

import (
	"errors"
	"path/filepath"
)

// ResolveResolverHook admits a script only within the selected resolver directory.
func ResolveResolverHook(profilePath, declared string) (string, error) {
	script := filepath.FromSlash(declared)
	if !filepath.IsLocal(script) || script == "." {
		return "", errors.New("resolver hook must declare a local script")
	}
	directory := filepath.Dir(profilePath)
	root, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return "", errors.New("resolver directory is unavailable")
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(directory, script))
	if err != nil {
		return "", errors.New("resolver script is unavailable")
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == "." || !filepath.IsLocal(relative) {
		return "", errors.New("resolver script escapes its package")
	}
	return resolved, nil
}
