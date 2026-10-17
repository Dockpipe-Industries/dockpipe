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
