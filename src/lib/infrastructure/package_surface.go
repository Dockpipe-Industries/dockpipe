package infrastructure

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"dockpipe/src/lib/domain"
)

// Package surface v1 is a small interactive page: one snapshot and at most 32
// actions. Its descriptor is limited to 64 KiB; resource output has a separate
// viewer capacity contract. Catalog discovery never executes its workflows.
const MaxPackageSurfaceBytes = 64 * 1024
const MaxPackageSurfaceActions = 32

var surfaceEnvironmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)

var surfaceIdentifier = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

type PackageSurface struct {
	PackageRoot   string                 `json:"package_root,omitempty"`
	Schema        int                    `json:"schema"`
	ID            string                 `json:"id"`
	Title         string                 `json:"title"`
	Snapshot      string                 `json:"snapshot"`
	Actions       []PackageSurfaceAction `json:"actions"`
	NativeDisplay *PackageNativeDisplay  `json:"native_display,omitempty"`
}

// PackageNativeDisplay declares immutable installed libraries. Runtime resources
// supply public connection references only; they cannot select executable code.
type PackageNativeDisplay struct {
	Interface   int               `json:"interface"`
	Libraries   map[string]string `json:"libraries"`
	PackageRoot string            `json:"package_root,omitempty"`
}

type PackageSurfaceEnvironmentBinding struct {
	Asset   string `json:"asset,omitempty"`
	Inherit string `json:"inherit,omitempty"`
}

type PackageSurfaceAction struct {
	LaunchEnvironment map[string]map[string]PackageSurfaceEnvironmentBinding `json:"launch_environment,omitempty"`
	ID                string                                                 `json:"id"`
	Title             string                                                 `json:"title"`
	Scope             string                                                 `json:"scope"`
	Workflow          string                                                 `json:"workflow,omitempty"`
	WorkflowID        string                                                 `json:"workflow_id,omitempty"`
}

func containedSurfaceFile(root, relative string) (string, error) {
	if err := domain.ValidatePackageSurfacePath(relative); err != nil {
		return "", err
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	canonicalRoot, err = filepath.Abs(canonicalRoot)
	if err != nil {
		return "", err
	}
	candidate, err := filepath.EvalSymlinks(filepath.Join(canonicalRoot, filepath.FromSlash(relative)))
	if err != nil {
		return "", err
	}
	within, err := filepath.Rel(canonicalRoot, candidate)
	if err != nil || within == "." || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) || filepath.IsAbs(within) {
		return "", fmt.Errorf("surface file escapes its package")
	}
	info, err := os.Stat(candidate)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("surface file must be a regular file")
	}
	return candidate, nil
}

// LoadPackageSurface authenticates path ownership, validates the wire contract,
// and projects workflow files to absolute package-contained catalog references.
func LoadPackageSurface(root, descriptor string) (*PackageSurface, error) {
	filename, err := containedSurfaceFile(root, descriptor)
	if err != nil {
		return nil, err
	}
	source, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer source.Close()
	data, err := io.ReadAll(io.LimitReader(source, MaxPackageSurfaceBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxPackageSurfaceBytes {
		return nil, fmt.Errorf("surface descriptor exceeds 64 KiB")
	}
	var surface PackageSurface
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&surface); err != nil {
		return nil, fmt.Errorf("decode surface: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("surface descriptor contains trailing JSON")
	}
	var authoredFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &authoredFields); err != nil {
		return nil, err
	}
	if _, present := authoredFields["native_display"]; present && surface.NativeDisplay == nil {
		return nil, fmt.Errorf("native display must be an object")
	}
	if _, present := authoredFields["package_root"]; present {
		return nil, fmt.Errorf("surface package_root is a catalog projection only")
	}
	surface.PackageRoot, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	surface.PackageRoot, err = filepath.Abs(surface.PackageRoot)
	if err != nil {
		return nil, err
	}
	if surface.Schema != 1 || !surfaceIdentifier.MatchString(surface.ID) || !validSurfaceTitle(surface.Title) {
		return nil, fmt.Errorf("surface requires schema 1, a stable id and a title of 1 to 160 characters")
	}
	if surface.Actions == nil {
		return nil, fmt.Errorf("surface actions must be an array")
	}
	if len(surface.Actions) > MaxPackageSurfaceActions {
		return nil, fmt.Errorf("surface exceeds 32 actions")
	}
	surface.Snapshot, err = containedSurfaceFile(root, surface.Snapshot)
	if err != nil {
		return nil, fmt.Errorf("surface snapshot: %w", err)
	}
	if surface.NativeDisplay != nil {
		var authored struct {
			NativeDisplay map[string]json.RawMessage `json:"native_display"`
		}
		if err := json.Unmarshal(data, &authored); err != nil {
			return nil, err
		}
		if _, present := authored.NativeDisplay["package_root"]; present {
			return nil, fmt.Errorf("native display package_root is a catalog projection only")
		}
		display := surface.NativeDisplay
		if display.Interface != 1 || len(display.Libraries) == 0 || len(display.Libraries) > 3 || display.PackageRoot != "" {
			return nil, fmt.Errorf("native display requires interface 1 and platform libraries")
		}
		display.PackageRoot, err = filepath.EvalSymlinks(root)
		if err != nil {
			return nil, err
		}
		display.PackageRoot, err = filepath.Abs(display.PackageRoot)
		if err != nil {
			return nil, err
		}
		for platform, library := range display.Libraries {
			if platform != "linux" && platform != "windows" && platform != "macos" {
				return nil, fmt.Errorf("unsupported native display platform %q", platform)
			}
			display.Libraries[platform], err = containedSurfaceFile(root, library)
			if err != nil {
				return nil, fmt.Errorf("native display library: %w", err)
			}
		}
	}
	var authoredActions []map[string]json.RawMessage
	if err := json.Unmarshal(authoredFields["actions"], &authoredActions); err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	for index := range surface.Actions {
		action := &surface.Actions[index]
		if !surfaceIdentifier.MatchString(action.ID) || seen[action.ID] || !validSurfaceTitle(action.Title) ||
			(action.Scope != "surface" && action.Scope != "resource") || (action.Workflow == "") == (action.WorkflowID == "") {
			return nil, fmt.Errorf("surface actions require unique ids, titles, a scope and exactly one workflow reference")
		}
		if _, present := authoredActions[index]["launch_environment"]; present && len(action.LaunchEnvironment) == 0 {
			return nil, fmt.Errorf("launch environment must contain platform bindings")
		}
		if len(action.LaunchEnvironment) > 3 || (len(action.LaunchEnvironment) > 0 && (action.WorkflowID == "" || action.Scope != "surface")) {
			return nil, fmt.Errorf("launch environment requires a public surface workflow action")
		}
		for platform, bindings := range action.LaunchEnvironment {
			if (platform != "linux" && platform != "windows" && platform != "macos") || len(bindings) == 0 || len(bindings) > 16 {
				return nil, fmt.Errorf("launch environment requires known platforms and 1 to 16 bindings")
			}
			for name, binding := range bindings {
				if !surfaceEnvironmentName.MatchString(name) || (binding.Asset == "") == (binding.Inherit == "") {
					return nil, fmt.Errorf("launch environment requires valid names and exactly one asset or inherit reference")
				}
				if binding.Inherit != "" {
					if !surfaceEnvironmentName.MatchString(binding.Inherit) {
						return nil, fmt.Errorf("invalid inherited environment reference")
					}
				} else {
					binding.Asset, err = containedSurfaceFile(root, binding.Asset)
					if err != nil {
						return nil, fmt.Errorf("launch environment asset: %w", err)
					}
					info, err := os.Stat(binding.Asset)
					if err != nil {
						return nil, err
					}
					if runtime.GOOS != "windows" && platform != "windows" && info.Mode().Perm()&0111 == 0 {
						return nil, fmt.Errorf("launch environment asset must be executable")
					}
					bindings[name] = binding
				}
			}
		}
		seen[action.ID] = true
		if action.Workflow != "" {
			action.Workflow, err = containedSurfaceFile(root, action.Workflow)
			if err != nil {
				return nil, fmt.Errorf("surface action %s: %w", action.ID, err)
			}
		} else if !surfaceIdentifier.MatchString(action.WorkflowID) {
			return nil, fmt.Errorf("surface action requires a valid workflow_id")
		}
	}
	return &surface, nil
}

func validSurfaceTitle(title string) bool {
	return strings.TrimSpace(title) != "" && len([]rune(title)) <= 160 && !strings.ContainsAny(title, "\x00\r\n")
}
