package manifest

import (
	"fmt"
	"path/filepath"
	"slices"
)

type TrialLayout struct {
	RunID        string
	Cohort       string
	MachineUUID  string
	InstanceRoot string
	OSDisk       string
	DataDisk     string
	AttemptRoot  string
	BoundaryRoot string
	Completed    bool
	Failed       bool
}

func (l TrialLayout) Validate() error {
	if !idPattern.MatchString(l.RunID) || !idPattern.MatchString(l.Cohort) || !uuidPattern.MatchString(l.MachineUUID) {
		return fmt.Errorf("run and cohort identities are required")
	}
	if !filepath.IsAbs(l.InstanceRoot) {
		return fmt.Errorf("instance root must be absolute")
	}
	wantPrefix := filepath.Join(filepath.Clean(l.InstanceRoot), l.RunID, l.Cohort)
	for label, path := range map[string]string{"os disk": l.OSDisk, "data disk": l.DataDisk, "attempt root": l.AttemptRoot, "boundary root": l.BoundaryRoot} {
		clean := filepath.Clean(path)
		rel, err := filepath.Rel(wantPrefix, clean)
		if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) || len(rel) >= 3 && rel[:3] == "../" {
			return fmt.Errorf("%s escapes exact run/cohort root", label)
		}
	}
	if l.OSDisk == l.DataDisk || l.AttemptRoot == l.BoundaryRoot {
		return fmt.Errorf("trial resources must be unique")
	}
	return nil
}

// CleanupPlan enumerates exact resources. It never removes them and refuses
// completed evidence roots, preserving failed and completed cohorts for review.
func (l TrialLayout) CleanupPlan(runID string, resources []string) ([]string, error) {
	if err := l.Validate(); err != nil {
		return nil, err
	}
	if l.Completed || l.Failed {
		return nil, fmt.Errorf("completed and failed cohort roots cannot be cleaned")
	}
	if runID != l.RunID {
		return nil, fmt.Errorf("cleanup run identity mismatch")
	}
	want := []string{l.OSDisk, l.DataDisk, l.AttemptRoot, l.BoundaryRoot}
	if !slices.Equal(resources, want) {
		return nil, fmt.Errorf("cleanup resources must exactly match the reviewed enumeration")
	}
	return slices.Clone(want), nil
}
