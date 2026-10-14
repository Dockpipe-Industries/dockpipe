package domain

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

func normalizedEngineVersion(version string) string {
	return "v" + strings.TrimPrefix(strings.TrimSpace(version), "v")
}

// ValidateMinimumDockpipeVersion accepts an exact semantic version lower bound.
func ValidateMinimumDockpipeVersion(minimum string) error {
	if minimum == "" {
		return nil
	}
	version := normalizedEngineVersion(minimum)
	core := strings.SplitN(strings.SplitN(version, "+", 2)[0], "-", 2)[0]
	if !semver.IsValid(version) || strings.Count(core, ".") != 2 {
		return fmt.Errorf("min_dockpipe_version must be a complete semantic version, such as 0.6.14")
	}
	return nil
}

// CheckPackageEngineVersion is compatibility policy, not an authorization gate.
// Unversioned development builds use schema validation; releases enforce the bound.
func CheckPackageEngineVersion(name, minimum, current string) error {
	if err := ValidateMinimumDockpipeVersion(minimum); err != nil {
		return err
	}
	if minimum == "" || current == "dev" || strings.HasPrefix(current, "dev+") {
		return nil
	}
	if !semver.IsValid(normalizedEngineVersion(current)) {
		return fmt.Errorf("cannot check package %q compatibility: unrecognized Dockpipe version %q", name, current)
	}
	if semver.Compare(normalizedEngineVersion(current), normalizedEngineVersion(minimum)) < 0 {
		return &DockpipeUpdateRequiredError{Package: name, Minimum: minimum, Current: current}
	}
	return nil
}
