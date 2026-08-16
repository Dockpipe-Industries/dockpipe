package application

import "dockpipe/src/lib/application/internal/packageversion"

const defaultPackageVersion = packageversion.Default

func authoredPackageVersion(workdir string) string {
	return packageversion.Authored(workdir)
}
