package application

import "dockpipe/src/lib/application/internal/packageversion"

func authoredPackageVersion(workdir string) string {
	return packageversion.Authored(workdir)
}
