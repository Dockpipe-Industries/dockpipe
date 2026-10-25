package domain

import "dockpipe/src/lib/domain/packagecontract"

func ValidateCatalogMetadata(catalogType string, categories []string) error {
	return packagecontract.ValidateCatalogMetadata(catalogType, categories)
}
