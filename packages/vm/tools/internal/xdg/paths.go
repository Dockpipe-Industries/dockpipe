package xdg

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Paths is the package-owned VM layout. It deliberately has no checkout or
// bin/.dockpipe fallback.
type Paths struct {
	Images    string
	Instances string
	Evidence  string
	Config    string
	Runtime   string
}

func Resolve(home string, env map[string]string) (Paths, error) {
	home = strings.TrimSpace(home)
	if home == "" || !filepath.IsAbs(home) {
		return Paths{}, fmt.Errorf("home must be an absolute path")
	}
	cache, err := root(env["XDG_CACHE_HOME"], filepath.Join(home, ".cache"), "XDG_CACHE_HOME")
	if err != nil {
		return Paths{}, err
	}
	state, err := root(env["XDG_STATE_HOME"], filepath.Join(home, ".local", "state"), "XDG_STATE_HOME")
	if err != nil {
		return Paths{}, err
	}
	config, err := root(env["XDG_CONFIG_HOME"], filepath.Join(home, ".config"), "XDG_CONFIG_HOME")
	if err != nil {
		return Paths{}, err
	}
	runtimeRoot := strings.TrimSpace(env["XDG_RUNTIME_DIR"])
	if runtimeRoot == "" {
		return Paths{}, fmt.Errorf("XDG_RUNTIME_DIR is required")
	}
	runtimeRoot, err = root(runtimeRoot, "", "XDG_RUNTIME_DIR")
	if err != nil {
		return Paths{}, err
	}
	return Paths{
		Images:    filepath.Join(cache, "dockpipe", "vm", "images"),
		Instances: filepath.Join(state, "dockpipe", "vm", "instances"),
		Evidence:  filepath.Join(state, "dockpipe", "evidence"),
		Config:    filepath.Join(config, "dockpipe", "vm"),
		Runtime:   filepath.Join(runtimeRoot, "dockpipe", "vm"),
	}, nil
}

func root(value, fallback, name string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	if !filepath.IsAbs(value) {
		return "", fmt.Errorf("%s must be absolute", name)
	}
	return filepath.Clean(value), nil
}
