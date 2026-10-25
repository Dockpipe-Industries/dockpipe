package domain

import "dockpipe/src/lib/domain/packagecontract"

func ValidateMinimumDockpipeVersion(minimum string) error {
	return packagecontract.ValidateMinimumDockpipeVersion(minimum)
}

func CheckPackageEngineVersion(name, minimum, current string) error {
	return packagecontract.CheckPackageEngineVersion(name, minimum, current)
}
