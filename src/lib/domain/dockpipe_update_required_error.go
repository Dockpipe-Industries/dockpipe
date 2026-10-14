package domain

import "fmt"

// DockpipeUpdateRequiredError preserves the compatibility failure for UI callers.
type DockpipeUpdateRequiredError struct {
	Package string
	Minimum string
	Current string
}

func (e *DockpipeUpdateRequiredError) Error() string {
	return fmt.Sprintf("package %q requires Dockpipe %s or newer; this executable is %s. Update Dockpipe, including the desktop app if you use it, then retry", e.Package, e.Minimum, e.Current)
}
