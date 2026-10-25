package domain

import "dockpipe/src/lib/domain/packagecontract"

type PackageManifest = packagecontract.PackageManifest

type PackageImageSpec = packagecontract.PackageImageSpec

type PackageScriptContract = packagecontract.PackageScriptContract

type PackageStateSpec = packagecontract.PackageStateSpec

type PackageBuildSpec = packagecontract.PackageBuildSpec

type PackageSourceBuildSpec = packagecontract.PackageSourceBuildSpec

type PackageTestSpec = packagecontract.PackageTestSpec

func ParsePackageManifest(path string) (*PackageManifest, error) {
	return packagecontract.ParsePackageManifest(path)
}

func ValidatePackageManifest(m *PackageManifest) error {
	return packagecontract.ValidatePackageManifest(m)
}

func ValidatePackageStateSpec(spec *PackageStateSpec) error {
	return packagecontract.ValidatePackageStateSpec(spec)
}

func NormalizePackageManifestYAMLAliases(m *PackageManifest) {
	packagecontract.NormalizePackageManifestYAMLAliases(m)
}

func ValidateProvider(s string) error {
	return packagecontract.ValidateProvider(s)
}

func ValidateCapabilityID(s string) error {
	return packagecontract.ValidateCapabilityID(s)
}

func ValidatePrimitive(s string) error {
	return packagecontract.ValidatePrimitive(s)
}

func ValidatePackageVersion(s string) error {
	return packagecontract.ValidatePackageVersion(s)
}

func ValidatePackageImageSpec(img *PackageImageSpec) error {
	return packagecontract.ValidatePackageImageSpec(img)
}

func ValidateScriptContractInjectable(s string) error {
	return packagecontract.ValidateScriptContractInjectable(s)
}

func ValidatePackageBuildSpec(spec *PackageBuildSpec) error {
	return packagecontract.ValidatePackageBuildSpec(spec)
}

func ValidatePackageTestSpec(test *PackageTestSpec) error {
	return packagecontract.ValidatePackageTestSpec(test)
}

func ValidatePackageSurfacePath(value string) error {
	return packagecontract.ValidatePackageSurfacePath(value)
}
