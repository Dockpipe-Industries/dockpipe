package packagecatalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/packagebuild"
	"gopkg.in/yaml.v3"
)

// One interactive install has a 2 GiB staging budget, matching the archive
// expansion budget. Package/depth limits bound network fan-out and traversal
// stack use within the CLI's ten-minute install deadline.
const maxInstallPackages = 128
const maxInstallDepth = 64
const maxInstallBytes int64 = 2 << 30

// InstallWithDependencies verifies the entire dependency closure in a private
// staging store before publishing dependencies first and the requested package last.
// Dependency references never choose another remote or execute package scripts.
func (c *Client) InstallWithDependencies(ctx context.Context, catalog Catalog, requested Entry, storeRoot string) (installedPath string, resultErr error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(storeRoot, 0o755); err != nil {
		return "", err
	}
	staging, err := os.MkdirTemp(storeRoot, ".install-")
	if err != nil {
		return "", err
	}
	defer func() {
		if err := os.RemoveAll(staging); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("package staging cleanup failed: %w", err))
		}
	}()
	active := make(map[string]bool)
	completed := make(map[string]bool)
	var order []Entry
	var totalBytes int64
	var visit func(Entry, int) error
	visit = func(entry Entry, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		key := entry.Kind + "/" + entry.Name
		if active[key] {
			return fmt.Errorf("package dependency cycle at %s", key)
		}
		if completed[key] {
			return nil
		}
		if depth > maxInstallDepth || len(active)+len(completed) >= maxInstallPackages {
			return fmt.Errorf("package dependency closure exceeds installation limits")
		}
		active[key] = true
		defer delete(active, key)
		if totalBytes >= maxInstallBytes || entry.SizeBytes > maxInstallBytes-totalBytes {
			return fmt.Errorf("package dependency downloads exceed installation size limit")
		}
		installed, err := c.Install(ctx, catalog, entry, staging)
		if err != nil {
			return fmt.Errorf("prepare %s: %w", key, err)
		}
		info, err := os.Stat(installed)
		if err != nil {
			return err
		}
		totalBytes += info.Size()
		if totalBytes > maxInstallBytes {
			return fmt.Errorf("package dependency downloads exceed installation size limit")
		}
		// Install has already verified the archive checksum, paths, manifest size
		// and identity. Read only that verified package's dependency declarations.
		prefix := "core"
		if entry.Kind != "core" {
			prefix = entry.Kind + "s/" + entry.Name
		}
		data, err := packagebuild.ReadFileFromTarGz(installed, prefix+"/package.yml")
		if err != nil {
			return err
		}
		var manifest domain.PackageManifest
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			return err
		}
		if len(manifest.Depends)+len(manifest.RequiresResolvers) > maxInstallPackages {
			return fmt.Errorf("package %s declares too many dependencies", key)
		}
		resolverNames := make(map[string]bool)
		for _, name := range manifest.RequiresResolvers {
			resolverNames[strings.TrimSpace(name)] = true
		}
		dependencies := append(append([]string(nil), manifest.Depends...), manifest.RequiresResolvers...)
		for _, name := range dependencies {
			name = strings.TrimSpace(name)
			dependency, err := resolveInstallDependency(catalog, name, resolverNames[name])
			if err != nil {
				return fmt.Errorf("package %s: %w", key, err)
			}
			if err := visit(dependency, depth+1); err != nil {
				return err
			}
		}
		completed[key] = true
		order = append(order, entry)
		return nil
	}
	if err := visit(requested, 0); err != nil {
		return "", err
	}
	root, err := os.OpenRoot(storeRoot)
	if err != nil {
		return "", err
	}
	defer root.Close()
	for index, entry := range order {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("installation stopped after publishing %d dependency packages: %w", index, err)
		}
		subdir := "core"
		if entry.Kind != "core" {
			subdir = entry.Kind + "s"
		}
		if err := root.MkdirAll(subdir, 0o755); err != nil {
			return "", fmt.Errorf("installation stopped after publishing %d dependency packages: %w", index, err)
		}
		destination := filepath.Join(subdir, entry.Tarball)
		if err := root.Rename(filepath.Join(filepath.Base(staging), destination), destination); err != nil {
			return "", fmt.Errorf("installation stopped after publishing %d dependency packages: %w", index, err)
		}
	}
	subdir := "core"
	if requested.Kind != "core" {
		subdir = requested.Kind + "s"
	}
	return filepath.Join(storeRoot, subdir, requested.Tarball), nil
}

func resolveInstallDependency(catalog Catalog, name string, resolverOnly bool) (Entry, error) {
	var matches []Entry
	for _, entry := range catalog.Packages {
		if entry.Name == name && (!resolverOnly || entry.Kind == "resolver") {
			matches = append(matches, entry)
		}
	}
	if name == "" || len(matches) == 0 {
		return Entry{}, fmt.Errorf("required package %q is missing from this platform's catalog; refresh the catalog or contact the publisher", name)
	}
	if len(matches) != 1 {
		return Entry{}, fmt.Errorf("dependency %q matches multiple package kinds; the publisher must identify resolver dependencies with requires_resolvers", name)
	}
	return matches[0], nil
}
