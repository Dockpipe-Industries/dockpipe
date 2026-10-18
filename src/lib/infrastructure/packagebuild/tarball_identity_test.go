package packagebuild

import "testing"

func TestMatchesVersionedTarballRequiresExactIdentity(t *testing.T) {
	for _, kind := range []string{"workflow", "resolver"} {
		for _, version := range []string{"0.6.4", "0.6.4-staging.123.1.abcdef"} {
			path := "dockpipe-" + kind + "-dorkpipe-" + version + ".tar.gz"
			if !MatchesVersionedTarball(path, kind, "dorkpipe") {
				t.Fatalf("missing exact %s archive %s", kind, path)
			}
			if MatchesVersionedTarball(path, kind, "dork") {
				t.Fatalf("accepted colliding %s archive %s", kind, path)
			}
		}
		for _, filename := range []string{
			"dockpipe-" + kind + "-dorkpipe-self-analysis-0.6.4.tar.gz",
			"dockpipe-" + kind + "-dorkpipe-invalid.tar.gz",
			"dockpipe-" + kind + "-dorkpipe-0.6.tar.gz",
		} {
			if MatchesVersionedTarball(filename, kind, "dorkpipe") {
				t.Fatalf("accepted wrong %s archive %s", kind, filename)
			}
		}
	}
}

func TestPackageArchiveIdentityAcceptsFormatsWithoutCodecProvider(t *testing.T) {
	for _, extension := range []string{".tar.gz", ".tar.zst", ".tar.nucleon"} {
		filename := "dockpipe-workflow-docs-review-1.2.3-staging.4" + extension
		name, version, ok := PackageArchiveIdentity(filename, "workflow")
		if !ok || name != "docs-review" || version != "1.2.3-staging.4" {
			t.Fatalf("identity for %s: %q, %q, %t", filename, name, version, ok)
		}
		if !MatchesVersionedTarball(filename, "workflow", "docs-review") {
			t.Fatalf("missing exact identity for %s", filename)
		}
	}
	for _, filename := range []string{
		"dockpipe-workflow-docs-review-1.2.tar.zst",
		"dockpipe-workflow-docs-review-1.2.3.tar.GZ",
		"dockpipe-workflow-docs-review-1.2.3.tar.gz.sha256",
	} {
		if IsPackageArchivePath(filename, "workflow") {
			t.Fatalf("accepted malformed archive filename %q", filename)
		}
	}
}
