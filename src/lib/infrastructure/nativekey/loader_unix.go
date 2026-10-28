//go:build (linux || darwin) && (amd64 || arm64)

package nativekey

import (
	"errors"
	"runtime"

	"github.com/ebitengine/purego"
)

func loadLibrary() (*library, error) {
	return loadFromPaths(systemLibraryPaths())
}

func systemLibraryPaths() []string {
	if runtime.GOOS == "darwin" {
		return []string{"/opt/homebrew/lib/libfido2.1.dylib", "/usr/local/lib/libfido2.1.dylib"}
	}
	multiarch := "x86_64-linux-gnu"
	if runtime.GOARCH == "arm64" {
		multiarch = "aarch64-linux-gnu"
	}
	return []string{
		"/usr/lib/" + multiarch + "/libfido2.so.1",
		"/lib/" + multiarch + "/libfido2.so.1",
		"/usr/lib64/libfido2.so.1",
		"/lib64/libfido2.so.1",
		"/usr/lib/libfido2.so.1",
		"/lib/libfido2.so.1",
	}
}

// Absolute system locations avoid loading a library from the working directory
// or a caller-supplied path. The successful handle remains mapped for process
// lifetime so no concurrent operation can use an unloaded function pointer.
func loadFromPaths(paths []string) (*library, error) {
	for _, path := range paths {
		handle, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			continue
		}
		lib := &library{}
		for _, binding := range lib.bindings() {
			symbol, err := purego.Dlsym(handle, binding.name)
			if err != nil {
				_ = purego.Dlclose(handle)
				return nil, errors.New("installed libfido2 lacks a required API; update libfido2 before using native security keys")
			}
			purego.RegisterFunc(binding.destination, symbol)
		}
		return lib, nil
	}
	return nil, errors.New("native security keys require libfido2 in a supported system library location")
}
