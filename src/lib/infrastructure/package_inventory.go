package infrastructure

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/archivecodec"
	"dockpipe/src/lib/infrastructure/packagebuild"

	"gopkg.in/yaml.v3"
)

// InstalledPackage describes an available local package, not a promise that it
// wins every workflow's resolution precedence.
type InstalledPackage struct {
	CatalogType string   `json:"catalog_type,omitempty"`
	Categories  []string `json:"categories,omitempty"`
	Name        string   `json:"name"`
	Title       string   `json:"title,omitempty"`
	Version     string   `json:"version"`
	Kind        string   `json:"kind"`
	Description string   `json:"description,omitempty"`
	Path        string   `json:"path"`
	Source      string   `json:"source"`
	Removable   bool     `json:"removable"`
	SizeBytes   int64    `json:"size_bytes,omitempty"`
}

type PackageInventory struct {
	Packages    []InstalledPackage `json:"packages"`
	Warnings    []string           `json:"warnings"`
	InstallRoot string             `json:"install_root"`
}

// ListInstalledPackages shares the runtime's project, configured, user, and
// system store roots. It never downloads or extracts packages.
func ListInstalledPackages(workdir string) (PackageInventory, error) {
	out := PackageInventory{Packages: []InstalledPackage{}, Warnings: []string{}}
	projectRoot, err := domain.FindProjectRootWithDockpipeConfig(workdir)
	if err != nil {
		return out, err
	}
	projectStore, err := PackagesRoot(workdir)
	if err != nil {
		return out, err
	}
	out.InstallRoot, err = GlobalPackagesRoot()
	if err != nil {
		return out, err
	}
	type source struct{ path, label string }
	sources := []source{{projectStore, "Project"}}
	for _, configured := range configuredPackageSources(projectRoot) {
		sources = append(sources, source{configured.path, "Configured"})
	}
	if cfg, err := domain.LoadDockpipeProjectConfig(projectRoot); err == nil && cfg != nil && cfg.Packages.TarballDir != nil && strings.TrimSpace(*cfg.Packages.TarballDir) != "" {
		sources = append(sources, source{resolveProjectConfigPath(projectRoot, *cfg.Packages.TarballDir), "Configured"})
	}
	sources = append(sources, source{out.InstallRoot, "User"})
	for _, system := range SystemPackagesRoots() {
		sources = append(sources, source{system, "System"})
	}
	if core, err := GlobalTemplatesCoreDir(); err == nil {
		sources = append(sources, source{core, "User"})
	}
	for _, core := range SystemTemplatesCoreDirs() {
		sources = append(sources, source{core, "System"})
	}
	seen := make(map[string]bool)
	for _, location := range sources {
		if canonical, err := filepath.EvalSymlinks(location.path); err == nil {
			location.path = canonical
		}
		err := filepath.WalkDir(location.path, func(filename string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if !os.IsNotExist(walkErr) {
					out.Warnings = append(out.Warnings, walkErr.Error())
				}
				return nil
			}
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			archive := packagebuild.IsPackageArchivePath(entry.Name(), "core") ||
				packagebuild.IsPackageArchivePath(entry.Name(), "workflow") ||
				packagebuild.IsPackageArchivePath(entry.Name(), "resolver")
			if !archive && entry.Name() != PackageManifestFilename {
				return nil
			}
			absolute, err := filepath.Abs(filename)
			if err != nil {
				return err
			}
			if seen[absolute] {
				return nil
			}
			seen[absolute] = true
			var manifest *domain.PackageManifest
			if archive {
				manifest, err = inventoryArchiveManifest(filename)
			} else {
				manifest, err = domain.ParsePackageManifest(filename)
			}
			if err != nil {
				out.Warnings = append(out.Warnings, fmt.Sprintf("%s: %v", filename, err))
				return nil
			}
			_, removalErr := userPackageArchivePath(out.InstallRoot, absolute)
			var archiveSize int64
			if archive {
				info, err := entry.Info()
				if err != nil {
					out.Warnings = append(out.Warnings, fmt.Sprintf("%s: %v", filename, err))
				} else {
					archiveSize = info.Size()
				}
			}
			out.Packages = append(out.Packages, InstalledPackage{
				Name: manifest.Name, Title: manifest.Title, Version: manifest.Version,
				CatalogType: manifest.CatalogType, Categories: manifest.Categories,
				Kind: manifest.Kind, Description: manifest.Description, Path: absolute, Source: location.label,
				SizeBytes: archiveSize,
				Removable: location.label == "User" && archive && removalErr == nil && (manifest.Kind == "workflow" || manifest.Kind == "resolver"),
			})
			return nil
		})
		if err != nil {
			return out, err
		}
	}
	sort.SliceStable(out.Packages, func(i, j int) bool { return out.Packages[i].Name < out.Packages[j].Name })
	return out, nil
}

func inventoryArchiveManifest(filename string) (*domain.PackageManifest, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return inventoryArchiveManifestReader(file, filename)
}

func inventoryArchiveManifestReader(reader io.Reader, filename string) (result *domain.PackageManifest, err error) {
	codec, err := archivecodec.CodecForPath(filename)
	if err != nil {
		return nil, err
	}
	decoded, err := archivecodec.Open(context.Background(), reader, codec, 2<<30)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, decoded.Close())
	}()
	archive := tar.NewReader(decoded)
	for count := 0; count < 100000; count++ {
		header, err := archive.Next()
		if err != nil {
			return nil, fmt.Errorf("read package metadata: %w", err)
		}
		parts := strings.Split(header.Name, "/")
		isManifest := header.Name == "core/package.yml" || (len(parts) == 3 && (parts[0] == "workflows" || parts[0] == "resolvers") && parts[2] == "package.yml")
		if !isManifest || header.Typeflag != tar.TypeReg {
			continue
		}
		if header.Size > 8<<20 {
			return nil, fmt.Errorf("package metadata exceeds size limit")
		}
		data, err := io.ReadAll(archive)
		if err != nil {
			return nil, err
		}
		var manifest domain.PackageManifest
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			return nil, err
		}
		domain.NormalizePackageManifestYAMLAliases(&manifest)
		if err := domain.ValidatePackageManifest(&manifest); err != nil {
			return nil, err
		}
		if manifest.Version == "" {
			prefix := "dockpipe-" + manifest.Kind + "-"
			if manifest.Kind != "core" {
				prefix += manifest.Name + "-"
			}
			basename := filepath.Base(filename)
			if stem, ok := archivecodec.StripExtension(basename); ok && strings.HasPrefix(stem, prefix) {
				version := strings.TrimPrefix(stem, prefix)
				if domain.ValidatePackageVersion(version) == nil {
					manifest.Version = version
				}
			}
		}
		return &manifest, nil
	}
	return nil, fmt.Errorf("package archive exceeds member limit")
}
