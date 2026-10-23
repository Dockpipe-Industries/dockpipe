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

// CheckProjectDockpipeVersion enforces the project's exact executable identity.
// Development builds do not bypass a repository pin.
func CheckProjectDockpipeVersion(config *DockpipeProjectConfig, current string) error {
	if config == nil || config.DockpipeVersion == nil {
		return nil
	}
	required := *config.DockpipeVersion
	if err := ValidateExactDockpipeVersion(required); err != nil {
		return err
	}
	if strings.TrimPrefix(current, "v") != required {
		return fmt.Errorf("project requires Dockpipe %s exactly; running %s. Install that release and retry", required, current)
	}
	return nil
}
