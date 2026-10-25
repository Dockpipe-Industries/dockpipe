package domain

import "dockpipe/src/lib/domain/namespace"

func IsReservedNamespace(s string) bool {
	return namespace.IsReservedNamespace(s)
}

func ValidateNamespace(s string) error {
	return namespace.ValidateNamespace(s)
}
