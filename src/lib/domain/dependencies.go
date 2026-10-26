package domain

import "dockpipe/src/lib/domain/dependency"

type DependencySpec = dependency.DependencySpec

type HostDependency = dependency.HostDependency

type HostDependencyInstallHint = dependency.HostDependencyInstallHint

func ValidateDependencySpec(fieldPrefix string, deps DependencySpec) error {
	return dependency.ValidateDependencySpec(fieldPrefix, deps)
}

func ValidatePlatformList(fieldPrefix string, platforms []string) error {
	return dependency.ValidatePlatformList(fieldPrefix, platforms)
}
