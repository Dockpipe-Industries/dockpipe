package domain

import "testing"

func TestPackageSourceKindConstantsMatchWireVocabulary(t *testing.T) {
	var store string = PackageSourceKindStore
	var tarballDir string = PackageSourceKindTarballDir
	if store != "store" || tarballDir != "tarball_dir" {
		t.Fatalf("package source kinds = %q, %q", store, tarballDir)
	}
}

func TestPackageSourceKindWireFieldRemainsString(t *testing.T) {
	var kind string = (DockpipePackageSourceConfig{}).Kind
	_ = kind
}

func TestNormalizePackageSourceKindPreservesCompatibility(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  PackageSourceKind
		valid bool
	}{
		{name: "default", want: PackageSourceKindStore, valid: true},
		{name: "trimmed store", value: " STORE ", want: PackageSourceKindStore, valid: true},
		{name: "tarball directory", value: " TarBall_Dir ", want: PackageSourceKindTarballDir, valid: true},
		{name: "unknown preserved", value: " Future_Source ", want: "future_source", valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := NormalizePackageSourceKind(test.value)
			if got != test.want {
				t.Fatalf("NormalizePackageSourceKind(%q) = %q, want %q", test.value, got, test.want)
			}
			if got.IsValid() != test.valid {
				t.Fatalf("PackageSourceKind(%q).IsValid() = %t, want %t", got, got.IsValid(), test.valid)
			}
		})
	}
}

func TestPackageSourceKindValidationDoesNotMutateWireValue(t *testing.T) {
	sources := []DockpipePackageSourceConfig{{Kind: " STORE ", Path: "packages"}}
	cfg := DockpipeProjectConfig{Packages: DockpipePackagesConfig{Sources: &sources}}
	if err := ValidateDockpipeProjectConfig(&cfg); err != nil {
		t.Fatal(err)
	}
	if sources[0].Kind != " STORE " {
		t.Fatalf("validation mutated package source kind to %q", sources[0].Kind)
	}
}
