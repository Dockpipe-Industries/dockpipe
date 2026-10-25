package runtimeassets

import "embed"

// Container resources are a fixed set of generic engine recipes, not authored
// workflows or package implementations.
//
//go:embed containers/base-dev.Dockerfile containers/dev-tools.fragment containers/entrypoint.sh
var containerResources embed.FS

// ContainerFiles returns the complete build context for a builtin development
// image. The heavier image extends the shared recipe directly, so it does not
// depend on another local image tag being built beforehand.
func ContainerFiles(name string) (map[string][]byte, bool, error) {
	if name != "base-dev" && name != "dev" {
		return nil, false, nil
	}
	dockerfile, err := containerResources.ReadFile("containers/base-dev.Dockerfile")
	if err != nil {
		return nil, true, err
	}
	if name == "dev" {
		fragment, err := containerResources.ReadFile("containers/dev-tools.fragment")
		if err != nil {
			return nil, true, err
		}
		dockerfile = append(append(dockerfile, '\n'), fragment...)
	}
	entrypoint, err := ContainerEntrypoint()
	if err != nil {
		return nil, true, err
	}
	return map[string][]byte{"Dockerfile": dockerfile, "assets/entrypoint.sh": entrypoint}, true, nil
}

// ContainerEntrypoint exports the generic command/action/signal adapter.
func ContainerEntrypoint() ([]byte, error) {
	return containerResources.ReadFile("containers/entrypoint.sh")
}
