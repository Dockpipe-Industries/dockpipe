package domain

import "strings"

// PackageSourceKind identifies the layout of an explicit local package source.
type PackageSourceKind string

// Keep these constants untyped so existing callers may continue assigning them
// directly to exported string fields without a conversion.
const (
	PackageSourceKindStore      = "store"
	PackageSourceKindTarballDir = "tarball_dir"
)

// NormalizePackageSourceKind returns the effective package source kind.
// An omitted kind defaults to store; unknown values remain available for validation.
func NormalizePackageSourceKind(value string) PackageSourceKind {
	kind := PackageSourceKind(strings.ToLower(strings.TrimSpace(value)))
	if kind == "" {
		return PackageSourceKindStore
	}
	return kind
}

// IsValid reports whether kind names a supported local package source layout.
func (kind PackageSourceKind) IsValid() bool {
	switch kind {
	case PackageSourceKindStore, PackageSourceKindTarballDir:
		return true
	default:
		return false
	}
}
