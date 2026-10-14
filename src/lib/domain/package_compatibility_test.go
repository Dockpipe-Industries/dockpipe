package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestPackageEngineMinimum(t *testing.T) {
	for _, test := range []struct {
		current string
		minimum string
		allowed bool
	}{
		{"0.6.13", "0.6.14", false},
		{"0.6.14", "0.6.14", true},
		{"0.6.15", "0.6.14", true},
		{"0.6.9", "0.6.14", false},
		{"0.6.14-rc.1", "0.6.14", false},
		{"0.6.14+build.5", "v0.6.14", true},
		{"0.6.14-rc.10", "0.6.14-rc.2", true},
		{"dev+abc.dirty", "0.6.14", true},
		{"unknown", "0.6.14", false},
		{"0.6.13", "", true},
	} {
		t.Run(test.current+"/"+test.minimum, func(t *testing.T) {
			err := CheckPackageEngineVersion("example", test.minimum, test.current)
			if (err == nil) != test.allowed {
				t.Fatalf("allowed=%v, error=%v", test.allowed, err)
			}
		})
	}
	err := CheckPackageEngineVersion("example", "0.6.14", "0.6.13")
	var update *DockpipeUpdateRequiredError
	if !errors.As(err, &update) || !strings.Contains(err.Error(), "Update Dockpipe, including the desktop app") {
		t.Fatalf("missing typed update guidance: %v", err)
	}
}

func TestPackageEngineMinimumRejectsInvalidMetadata(t *testing.T) {
	for _, minimum := range []string{"0.6", ">=0.6.14", "latest", "0.6.14-01", "0.06.14", strings.Repeat("1", 100)} {
		if err := ValidatePackageManifest(&PackageManifest{MinDockpipeVersion: minimum}); err == nil {
			t.Fatalf("accepted invalid minimum %q", minimum)
		}
	}
}
