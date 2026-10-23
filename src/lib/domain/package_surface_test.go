package domain

import "testing"

func TestPackageSurfaceManifestPathIsPackageRelative(t *testing.T) {
	for _, value := range []string{"assets/ui/surface.json", "surface.json"} {
		if err := ValidatePackageManifest(&PackageManifest{Surface: value}); err != nil {
			t.Fatalf("valid surface path %q: %v", value, err)
		}
	}
	for _, value := range []string{".", "..", "../surface.json", "/surface.json", "assets/../surface.json", `C:\surface.json`, "surface\n.json"} {
		if err := ValidatePackageManifest(&PackageManifest{Surface: value}); err == nil {
			t.Fatalf("invalid surface path accepted: %q", value)
		}
	}
}
