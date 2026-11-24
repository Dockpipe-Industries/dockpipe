//go:build linux || darwin

package controlassets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// This tests the native build layout with a tiny consumer, independently of
// installed-SDK qualification of the generated PipeLang program.
func TestControlCMakeArtifactLayout(t *testing.T) {
	cmake, err := exec.LookPath("cmake")
	if err != nil {
		t.Skip("CMake is unavailable")
	}
	cxx, err := exec.LookPath("c++")
	if err != nil {
		t.Skip("C++ compiler is unavailable")
	}
	for _, generator := range []string{"Unix Makefiles", "Ninja Multi-Config"} {
		t.Run(generator, func(t *testing.T) {
			var buildTool string
			if generator == "Unix Makefiles" {
				buildTool, err = exec.LookPath("make")
			} else {
				buildTool = os.Getenv("DOCKPIPE_TEST_NINJA")
				if buildTool == "" {
					buildTool, err = exec.LookPath("ninja")
				}
			}
			if err != nil || buildTool == "" {
				t.Skip("generator build tool is unavailable")
			}

			root := t.TempDir()
			sdk := filepath.Join(root, "sdk")
			write := func(name string, data []byte, mode os.FileMode) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(name, data, mode); err != nil {
					t.Fatal(err)
				}
			}
			manifest := []byte("{}\n")
			digest := sha256.Sum256(manifest)
			manifestSHA := hex.EncodeToString(digest[:])
			write(filepath.Join(sdk, "share/pipelang-model/sdk-manifest.json"), manifest, 0o600)
			write(filepath.Join(sdk, "bin/pipelang-sdk-tool"), []byte("#!/bin/sh\nexit 0\n"), 0o700)
			config := `get_filename_component(PipeLangModel_SDK_PREFIX "${CMAKE_CURRENT_LIST_DIR}/../../.." ABSOLUTE)
set(PipeLangModel_MANIFEST_SHA256 "` + manifestSHA + `")
function(pipelang_add_model target)
    add_library(${target} INTERFACE)
endfunction()
`
			write(filepath.Join(sdk, "lib/cmake/PipeLangModel/PipeLangModelConfig.cmake"), []byte(config), 0o600)
			write(filepath.Join(sdk, "lib/cmake/PipeLangModel/PipeLangModelConfigVersion.cmake"), []byte("set(PACKAGE_VERSION 0.1.0)\nset(PACKAGE_VERSION_EXACT TRUE)\nset(PACKAGE_VERSION_COMPATIBLE TRUE)\n"), 0o600)
			asset, err := Files.ReadFile("native/CMakeLists.txt")
			if err != nil {
				t.Fatal(err)
			}
			write(filepath.Join(root, "CMakeLists.txt"), asset, 0o600)
			write(filepath.Join(root, "Main.cpp.in"), []byte("int main() { return 0; }\n"), 0o600)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			run := func(arguments ...string) {
				t.Helper()
				output, err := exec.CommandContext(ctx, cmake, arguments...).CombinedOutput()
				if err != nil {
					t.Fatalf("CMake failed: %v\n%s", err, output)
				}
			}
			build := filepath.Join(root, "build")
			run("-S", root, "-B", build, "-G", generator, "-DSDK_PREFIX="+sdk,
				"-DSDK_SHA256="+manifestSHA, "-DCMAKE_CXX_COMPILER="+cxx,
				"-DCMAKE_MAKE_PROGRAM="+buildTool, "-DCMAKE_BUILD_TYPE=Release")
			run("--build", build, "--config", "Release", "--target", "control", "--parallel", "1")
			binary := filepath.Join(build, "bin", "control")
			if output, err := exec.CommandContext(ctx, binary).CombinedOutput(); err != nil {
				t.Fatalf("built artifact at the admitted path failed: %v\n%s", err, output)
			}
		})
	}
}
