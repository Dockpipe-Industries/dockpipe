package packagecontract

import (
	"fmt"
	"regexp"
)

var catalogCategoryPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// ValidateCatalogMetadata checks discovery fields without changing package identity.
// Categories are bounded facets, not arbitrary documents or executable capabilities.
func ValidateCatalogMetadata(catalogType string, categories []string) error {
	switch catalogType {
	case "", "app", "workflow", "integration", "core":
	default:
		return fmt.Errorf("catalog_type must be app, workflow, integration, or core")
	}
	if len(categories) > 16 {
		return fmt.Errorf("categories must contain at most 16 entries")
	}
	seen := make(map[string]bool, len(categories))
	for _, category := range categories {
		if !catalogCategoryPattern.MatchString(category) || seen[category] {
			return fmt.Errorf("categories must be unique lowercase slugs of at most 64 characters: %q", category)
		}
		seen[category] = true
	}
	return nil
}
