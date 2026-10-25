package infrastructure

// runtimeDefaults describes engine substrate behavior without an authored asset tree.
// Execution adapters are supplied by the selected runtime or resolver package.
type runtimeDefaults struct {
	substrate          string
	hostRequiresDocker *bool
}

func builtinRuntimeDefaults(name string) (runtimeDefaults, bool) {
	switch NormalizeRuntimeProfileName(name) {
	case "dockerfile":
		return runtimeDefaults{substrate: "dockerfile"}, true
	case "dockerimage":
		return runtimeDefaults{substrate: "dockerimage"}, true
	case "package":
		return runtimeDefaults{substrate: "package"}, true
	case "vm", "vmimage":
		requiresDocker := false
		return runtimeDefaults{substrate: "vmimage", hostRequiresDocker: &requiresDocker}, true
	default:
		return runtimeDefaults{}, false
	}
}

func (defaults runtimeDefaults) assignments() map[string]string {
	values := map[string]string{"DOCKPIPE_RUNTIME_SUBSTRATE": defaults.substrate}
	if defaults.hostRequiresDocker != nil {
		value := "0"
		if *defaults.hostRequiresDocker {
			value = "1"
		}
		values["DOCKPIPE_RUNTIME_HOST_REQUIRES_DOCKER"] = value
	}
	return values
}

// BuiltinRuntimeProfileNames lists the substrate selectors supported without
// installing a profile tree. Aliases remain visible for existing callers.
func BuiltinRuntimeProfileNames() []string {
	return []string{"dockerfile", "dockerimage", "package", "vm", "vmimage"}
}
