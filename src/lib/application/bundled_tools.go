package application

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Package tools are scoped to the selected package, never a global search over
// installed packages. Their libraries must be resolved by the tools themselves.
func packageToolsDir(start string) string {
	root := nearestPackageRoot(start)
	if root == "" {
		return ""
	}
	dir := filepath.Join(root, "assets", "tooling", "bin")
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	return ""
}

func bundledDependencyCommandPath(toolsDir, command string) (string, error) {
	if toolsDir == "" || command == "" || strings.ContainsAny(command, `/\\`) {
		return "", os.ErrNotExist
	}
	names := []string{command}
	if runtime.GOOS == "windows" {
		names = append(names, command+".exe", command+".cmd", command+".bat")
	}
	for _, name := range names {
		path := filepath.Join(toolsDir, name)
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if runtime.GOOS == "windows" || info.Mode().Perm()&0o111 != 0 {
			return path, nil
		}
	}
	return "", os.ErrNotExist
}
