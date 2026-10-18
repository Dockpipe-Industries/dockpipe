package packagebuild

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/infrastructure/archivecodec"

	"golang.org/x/mod/semver"
)

// MatchesVersionedTarball requires the complete package name and a complete
// semantic version. A prefix glob for "codex" must not match "codex-pav".
func MatchesVersionedTarball(path, kind, packageName string) bool {
	if kind != "workflow" && kind != "resolver" {
		return false
	}
	filename := filepath.Base(path)
	prefix := "dockpipe-" + kind + "-" + SafeTarballToken(packageName) + "-"
	stem, ok := archivecodec.StripExtension(filename)
	if !ok || !strings.HasPrefix(stem, prefix) {
		return false
	}
	version := strings.TrimPrefix(stem, prefix)
	normalized := "v" + strings.TrimPrefix(version, "v")
	core := strings.SplitN(strings.SplitN(normalized, "+", 2)[0], "-", 2)[0]
	return semver.IsValid(normalized) && strings.Count(core, ".") == 2
}

// IsPackageArchivePath recognizes a complete package archive filename without
// depending on whether its decoder provider is currently installed.
func IsPackageArchivePath(path, kind string) bool {
	_, _, ok := PackageArchiveIdentity(path, kind)
	return ok
}

// PackageArchiveIdentity returns the name and version encoded by a complete
// archive filename. It does not require the decoder provider to be installed.
func PackageArchiveIdentity(path, kind string) (string, string, bool) {
	filename := filepath.Base(path)
	stem, ok := archivecodec.StripExtension(filename)
	if !ok {
		return "", "", false
	}
	prefix := "dockpipe-" + kind + "-"
	if kind == "core" {
		version := strings.TrimPrefix(stem, prefix)
		return "core", version, strings.HasPrefix(stem, prefix) && validVersion(version)
	}
	if kind != "workflow" && kind != "resolver" || !strings.HasPrefix(stem, prefix) {
		return "", "", false
	}
	remainder := strings.TrimPrefix(stem, prefix)
	for separator := strings.LastIndexByte(remainder, '-'); separator > 0; separator = strings.LastIndexByte(remainder[:separator], '-') {
		name := remainder[:separator]
		version := remainder[separator+1:]
		if name == SafeTarballToken(name) && validVersion(version) {
			return name, version, true
		}
	}
	return "", "", false
}

func validVersion(version string) bool {
	normalized := "v" + strings.TrimPrefix(version, "v")
	core := strings.SplitN(strings.SplitN(normalized, "+", 2)[0], "-", 2)[0]
	return semver.IsValid(normalized) && strings.Count(core, ".") == 2
}

// ListPackageArchivePaths lists regular, identity-shaped files in one store dir.
// For the same archive stem, gzip precedes zstd, then opaque codec IDs.
func ListPackageArchivePaths(root, kind string) ([]string, error) {
	if kind != "core" && kind != "workflow" && kind != "resolver" {
		return nil, nil
	}
	matches, err := filepath.Glob(filepath.Join(root, "dockpipe-"+kind+"-*.tar.*"))
	if err != nil {
		return nil, err
	}
	var archives []string
	for _, candidate := range matches {
		if !IsPackageArchivePath(candidate, kind) {
			continue
		}
		info, err := os.Lstat(candidate)
		if err != nil {
			return nil, err
		}
		if info.Mode().IsRegular() {
			archives = append(archives, candidate)
		}
	}
	sort.Slice(archives, func(i, j int) bool {
		left, _ := archivecodec.StripExtension(filepath.Base(archives[i]))
		right, _ := archivecodec.StripExtension(filepath.Base(archives[j]))
		if left != right {
			return left < right
		}
		return ArchiveCodecPriority(archives[i]) < ArchiveCodecPriority(archives[j])
	})
	return archives, nil
}

func ArchiveCodecPriority(path string) string {
	codec, err := archivecodec.CodecForPath(path)
	if err != nil {
		return "3~"
	}
	switch codec {
	case "gzip":
		return "0"
	case "zstd":
		return "1"
	default:
		return "2" + codec
	}
}
