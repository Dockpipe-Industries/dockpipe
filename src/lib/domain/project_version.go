package domain

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

// ValidateExactDockpipeVersion accepts a complete SemVer without a v prefix.
func ValidateExactDockpipeVersion(version string) error {
	core := strings.SplitN(strings.SplitN(version, "+", 2)[0], "-", 2)[0]
	if !semver.IsValid("v"+version) || strings.Count(core, ".") != 2 {
		return fmt.Errorf("dockpipe_version must be an exact complete semantic version without a v prefix, such as 0.6.24; ranges and tags are not supported")
	}
	return nil
}

// ValidateProjectDockpipeVersions validates independent identity and compatibility contracts.
func ValidateProjectDockpipeVersions(config *DockpipeProjectConfig) error {
	if config == nil {
		return nil
	}
	if config.DockpipeVersion != nil {
		if err := ValidateExactDockpipeVersion(*config.DockpipeVersion); err != nil {
			return err
		}
	}
	if config.MinDockpipeVersion != nil {
		minimum := *config.MinDockpipeVersion
		if minimum == "" || minimum != strings.TrimSpace(minimum) || strings.HasPrefix(minimum, "v") {
			return fmt.Errorf("min_dockpipe_version must be a complete semantic version without a v prefix, such as 0.6.24")
		}
		if err := ValidateMinimumDockpipeVersion(minimum); err != nil {
			return err
		}
		if config.DockpipeVersion != nil && semver.Compare("v"+*config.DockpipeVersion, "v"+minimum) < 0 {
			return fmt.Errorf("dockpipe_version %s contradicts min_dockpipe_version %s", *config.DockpipeVersion, minimum)
		}
	}
	return nil
}

// CheckProjectDockpipeVersion enforces exact identity and/or a SemVer lower bound.
// Explicit project constraints require a versioned executable, including development use.
func CheckProjectDockpipeVersion(config *DockpipeProjectConfig, current string) error {
	if err := ValidateProjectDockpipeVersions(config); err != nil {
		return err
	}
	if config == nil {
		return nil
	}
	if config.DockpipeVersion != nil && strings.TrimPrefix(current, "v") != *config.DockpipeVersion {
		return fmt.Errorf("project requires Dockpipe %s exactly; running %s. Install that release and retry", *config.DockpipeVersion, current)
	}
	if config.MinDockpipeVersion != nil {
		version := strings.TrimPrefix(current, "v")
		if err := ValidateExactDockpipeVersion(version); err != nil {
			return fmt.Errorf("cannot check project min_dockpipe_version %s: running Dockpipe version %q is unversioned or invalid; use a versioned executable", *config.MinDockpipeVersion, current)
		}
		if semver.Compare("v"+version, "v"+*config.MinDockpipeVersion) < 0 {
			return fmt.Errorf("project requires Dockpipe %s or newer; running %s. Update Dockpipe and retry", *config.MinDockpipeVersion, current)
		}
	}
	return nil
}
