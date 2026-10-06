package remote

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/domain"
	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
)

// BuildBundle snapshots selected workflow sources and explicit inputs only.
// Dependencies are unpacked installable package directories, not host tools.
func BuildBundle(workdir, workflow string, includes, dependencies, artifacts []string) (*contract.Bundle, error) {
	workdir, err := filepath.Abs(workdir)
	if err != nil {
		return nil, err
	}
	if filepath.IsAbs(workflow) {
		workflow, err = filepath.Rel(workdir, workflow)
		if err != nil {
			return nil, err
		}
	}
	workflow = filepath.ToSlash(workflow)
	if !contract.SafeBundlePath(workflow) || strings.HasPrefix(workflow, "store/") {
		return nil, fmt.Errorf("workflow must be a safe relative source path inside --workdir")
	}
	builder := bundleBuilder{bundle: contract.Bundle{WorkflowFile: workflow, Artifacts: artifacts}, seen: map[string]bool{}}
	selected := filepath.Dir(workflow)
	if selected == "." {
		selected = workflow
	}
	for _, source := range append([]string{selected}, includes...) {
		if !contract.SafeBundlePath(filepath.ToSlash(source)) || source == "store" || strings.HasPrefix(source, "store/") {
			return nil, fmt.Errorf("include must be a relative source path; store/ is reserved: %s", source)
		}
		if err := builder.addTree(filepath.Join(workdir, source), filepath.ToSlash(source)); err != nil {
			return nil, err
		}
	}
	manifests := map[string]*domain.PackageManifest{}
	for _, directory := range dependencies {
		if !filepath.IsAbs(directory) {
			directory = filepath.Join(workdir, directory)
		}
		if err := infrastructure.ValidateUnlinkedPath(directory); err != nil {
			return nil, err
		}
		manifestPath := filepath.Join(directory, "package.yml")
		if err := infrastructure.ValidateUnlinkedPath(manifestPath); err != nil {
			return nil, err
		}
		info, err := os.Lstat(manifestPath)
		if err != nil || !info.Mode().IsRegular() || info.Size() > contract.MaxBundleBytes {
			return nil, fmt.Errorf("dependency manifest must be a bounded regular file")
		}
		manifest, err := domain.ParsePackageManifest(manifestPath)
		if err != nil {
			return nil, err
		}
		if !safePackageName(manifest.Name) || manifests[manifest.Name] != nil {
			return nil, fmt.Errorf("dependency names must be unique bounded identifiers")
		}
		var category string
		switch manifest.Kind {
		case "workflow":
			category = "workflows"
		case "resolver":
			category = "resolvers"
		case "assets":
			category = "assets"
		default:
			return nil, fmt.Errorf("dependency %s must be an unpacked workflow, resolver, or assets package", manifest.Name)
		}
		manifests[manifest.Name] = manifest
		if err := builder.addTree(directory, "store/"+category+"/"+manifest.Name); err != nil {
			return nil, err
		}
	}
	for name, manifest := range manifests {
		for _, dependency := range manifest.Depends {
			if manifests[dependency] == nil {
				return nil, fmt.Errorf("package %s requires %s; add its unpacked directory with --dependency", name, dependency)
			}
		}
	}
	sort.Slice(builder.bundle.Files, func(i, j int) bool { return builder.bundle.Files[i].Path < builder.bundle.Files[j].Path })
	if err := builder.bundle.Validate(); err != nil {
		return nil, err
	}
	if err := validateBundlePackages(builder.bundle); err != nil {
		return nil, err
	}
	return &builder.bundle, nil
}

type bundleBuilder struct {
	bundle contract.Bundle
	seen   map[string]bool
	total  int
}

func (builder *bundleBuilder) addTree(source, destination string) error {
	if err := infrastructure.ValidateUnlinkedPath(source); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, current)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(filepath.Join(destination, relative))
		if !contract.SafeBundlePath(name) {
			return fmt.Errorf("refusing private/generated delivery path %q; select a clean source tree", name)
		}
		if entry.IsDir() {
			return nil
		}
		if builder.seen[name] {
			return nil
		}
		if len(builder.bundle.Files) >= contract.MaxBundleFiles {
			return fmt.Errorf("delivery exceeds %d files", contract.MaxBundleFiles)
		}
		if err := infrastructure.ValidateUnlinkedPath(current); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > int64(contract.MaxBundleBytes-builder.total) {
			return fmt.Errorf("delivery source %q is not a bounded regular file", name)
		}
		file, err := os.OpenInRoot(filepath.Dir(current), filepath.Base(current))
		if err != nil {
			return err
		}
		defer file.Close()
		opened, err := file.Stat()
		if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
			return fmt.Errorf("delivery source changed during snapshot")
		}
		data, err := io.ReadAll(io.LimitReader(file, int64(contract.MaxBundleBytes-builder.total)+1))
		if err != nil {
			return err
		}
		builder.total += len(data)
		if builder.total > contract.MaxBundleBytes {
			return fmt.Errorf("delivery exceeds 8 MiB")
		}
		builder.seen[name] = true
		builder.bundle.Files = append(builder.bundle.Files, contract.BundleFile{Path: name, Data: data, Executable: info.Mode().Perm()&0o111 != 0})
		return nil
	})
}
