package packagecompile

import (
	"fmt"
	"os"
	"strings"

	"dockpipe/src/lib/domain"
	"gopkg.in/yaml.v3"
)

// mergeCompiledCatalogMetadata fills gaps from config.yml in the staged manifest.
// Explicit package metadata wins; source files and install identity stay unchanged.
func mergeCompiledCatalogMetadata(manifestPath, configPath string) error {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var fields map[string]any
	if err := yaml.Unmarshal(content, &fields); err != nil {
		return err
	}
	if fields == nil {
		fields = make(map[string]any)
	}
	var authored domain.PackageManifest
	if err := yaml.Unmarshal(content, &authored); err != nil {
		return err
	}
	if err := domain.ValidateCatalogMetadata(authored.CatalogType, authored.Categories); err != nil {
		return fmt.Errorf("package catalog metadata: %w", err)
	}
	var workflow domain.Workflow
	config, err := os.ReadFile(configPath)
	if err == nil {
		if err := yaml.Unmarshal(config, &workflow); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	fill := func(key, value string) {
		existing, _ := fields[key].(string)
		if strings.TrimSpace(existing) == "" && strings.TrimSpace(value) != "" {
			fields[key] = value
		}
	}
	fill("title", workflow.Title)
	name, _ := fields["name"].(string)
	fill("title", name)
	fill("description", workflow.Description)
	fill("catalog_type", workflow.CatalogType)
	if workflow.Category == "app" {
		fill("catalog_type", "app")
	}
	kind, _ := fields["kind"].(string)
	switch kind {
	case "resolver":
		fill("catalog_type", "integration")
	case "workflow", "core":
		fill("catalog_type", kind)
	}
	if _, exists := fields["categories"]; !exists && len(workflow.Categories) > 0 {
		fields["categories"] = workflow.Categories
	}
	content, err = yaml.Marshal(fields)
	if err != nil {
		return err
	}
	var manifest domain.PackageManifest
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return err
	}
	if err := domain.ValidatePackageManifest(&manifest); err != nil {
		return fmt.Errorf("package catalog metadata: %w", err)
	}
	return os.WriteFile(manifestPath, content, 0o644)
}
