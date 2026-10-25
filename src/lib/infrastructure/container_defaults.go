package infrastructure

import "dockpipe/src/lib/runtimeassets"

// BuiltinContainerBuild materializes the selected generic container context only
// when requested. Unknown names remain available for package or registry lookup.
func BuiltinContainerBuild(workdir, name string) (image, directory, context string, ok bool, err error) {
	files, ok, err := runtimeassets.ContainerFiles(name)
	if err != nil || !ok {
		return "", "", "", ok, err
	}
	directory, err = materializeLibraryResources(workdir, "container-"+name, files)
	if err != nil {
		return "", "", "", true, err
	}
	return "dockpipe-" + name, directory, directory, true, nil
}
