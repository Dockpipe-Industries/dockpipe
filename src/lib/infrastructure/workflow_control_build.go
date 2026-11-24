package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"dockpipe/src/lib/controlassets"
	"dockpipe/src/lib/domain/workflow"
	ownedprocess "dockpipe/src/lib/infrastructure/process"
)

type WorkflowControlBuildOptions struct {
	Workdir         string
	ConfigPath      string
	Workflow        *workflow.Workflow
	SDKPrefix       string
	SDKSHA256       string
	CMake           string
	CXX             string
	ResourceProfile string
	Sanitizers      bool
}

type controlBuildOutput struct{ bytes.Buffer }

func (output *controlBuildOutput) Write(data []byte) (int, error) {
	count := len(data)
	remaining := 128*1024 - output.Len()
	if remaining > len(data) {
		remaining = len(data)
	}
	if remaining > 0 {
		_, _ = output.Buffer.Write(data[:remaining])
	}
	return count, nil
}

func controlBuildCommand(ctx context.Context, directory, logName string, arguments ...string) error {
	var output controlBuildOutput
	command := exec.CommandContext(ctx, arguments[0], arguments[1:]...)
	command.Dir = directory
	command.Stdout = &output
	command.Stderr = &output
	err := ownedprocess.Run(command)
	if writeErr := os.WriteFile(filepath.Join(directory, logName), output.Bytes(), 0o600); writeErr != nil {
		return writeErr
	}
	if err != nil {
		return fmt.Errorf("control build failed: %w\n%s", err, output.String())
	}
	return nil
}

// BuildWorkflowControl is an explicit build operation. Run never calls it.
func BuildWorkflowControl(ctx context.Context, options WorkflowControlBuildOptions) (string, error) {
	if err := workflow.ValidateLoadedWorkflow(options.Workflow); err != nil {
		return "", err
	}
	if options.Workflow == nil || options.Workflow.Control == nil {
		return "", fmt.Errorf("workflow has no control program")
	}
	if !filepath.IsAbs(options.SDKPrefix) || !validControlDigest(options.SDKSHA256) {
		return "", fmt.Errorf("select an absolute installed SDK and its exact manifest SHA256")
	}
	manifestPath := filepath.Join(options.SDKPrefix, "share", "pipelang-model", "sdk-manifest.json")
	digest, _, err := controlFileDigest(manifestPath, 16*1024*1024)
	if err != nil || digest != options.SDKSHA256 {
		return "", fmt.Errorf("selected PipeLang SDK manifest identity differs")
	}
	identity, err := controlDefinitionIdentity(options.Workflow)
	if err != nil {
		return "", err
	}
	root, err := controlCacheDirectory(options.Workdir, identity)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	scratchRoot := root
	if runtime.GOOS == "windows" {
		// CMake adds its own compiler-probe and generated-object directories.
		// Keep scratch in the project cache without the artifact's architecture
		// and definition digest prefixes, which can exhaust Win32 tool paths.
		cache, err := StateInternalCacheDir(options.Workdir)
		if err != nil {
			return "", err
		}
		scratchRoot = filepath.Join(cache, "control-build")
		if err := os.MkdirAll(scratchRoot, 0o700); err != nil {
			return "", err
		}
	}

	stage, err := os.MkdirTemp(scratchRoot, "build-")
	if err != nil {
		return "", err
	}
	for _, name := range []string{"Main.cpp.in", "CMakeLists.txt"} {
		data, err := controlassets.Files.ReadFile("native/" + name)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(stage, name), data, 0o600); err != nil {
			return "", err
		}
	}
	if err := os.Mkdir(filepath.Join(stage, "src"), 0o700); err != nil {
		return "", err
	}
	source, decoder, bodyLine := controlSource(options.Workflow)
	if err := os.WriteFile(filepath.Join(stage, "src", "Entry.pipe"), []byte(source), 0o600); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(stage, "InputDecode.inc"), []byte(decoder), 0o600); err != nil {
		return "", err
	}
	cmake := options.CMake
	if cmake == "" {
		cmake, err = exec.LookPath("cmake")
	}
	if err != nil || !filepath.IsAbs(cmake) {
		return "", fmt.Errorf("select an absolute CMake executable")
	}
	cxx := options.CXX
	if cxx == "" {
		cxx, err = exec.LookPath("c++")
	}
	if err != nil || !filepath.IsAbs(cxx) {
		return "", fmt.Errorf("select an absolute C++ compiler")
	}
	profile := options.ResourceProfile
	if profile == "" {
		profile = "system"
	}
	sanitizer := "OFF"
	if options.Sanitizers {
		sanitizer = "ON"
	}
	arguments := []string{cmake, "-S", stage, "-B", filepath.Join(stage, "build"),
		"-DSDK_PREFIX=" + options.SDKPrefix, "-DSDK_SHA256=" + options.SDKSHA256,
		"-DCMAKE_CXX_COMPILER=" + cxx, "-DPipeLangModel_CXX_RESOURCE_PROFILE=" + profile,
		"-DCMAKE_BUILD_TYPE=Release", "-DSANITIZERS=" + sanitizer}
	if err := controlBuildCommand(ctx, stage, "configure.log", arguments...); err != nil {
		return "", controlBuildDiagnostic(options, source, bodyLine, err)
	}
	if err := controlBuildCommand(ctx, stage, "build.log", cmake, "--build", filepath.Join(stage, "build"), "--config", "Release", "--target", "control", "--parallel", "1"); err != nil {
		return "", controlBuildDiagnostic(options, source, bodyLine, err)
	}
	binary := filepath.Join(stage, "build", "bin", controlBinaryName())
	hash, size, err := controlFileDigest(binary, 256*1024*1024)
	if err != nil {
		return "", err
	}
	sourceHash := sha256.Sum256([]byte(source))
	manifest := workflowControlManifest{Schema: 1, Profile: workflowControlProfile, DefinitionSHA256: identity,
		SDKManifestSHA256: options.SDKSHA256, SourceSHA256: hex.EncodeToString(sourceHash[:]), BinarySHA256: hash, BinarySize: size,
		OS: runtime.GOOS, Architecture: runtime.GOARCH, BodyLine: bodyLine, Sanitizers: options.Sanitizers}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	publication, err := os.MkdirTemp(root, "artifact-")
	if err != nil {
		return "", err
	}
	if err := copyControlBinary(binary, filepath.Join(publication, controlBinaryName())); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(publication, "manifest.json"), encoded, 0o600); err != nil {
		return "", err
	}
	// Versioned publications remain immutable. The selection file is the only
	// atomically replaced state; older validated artifacts are retained.
	selected, err := json.Marshal(struct {
		Artifact string `json:"artifact"`
	}{filepath.Base(publication)})
	if err != nil {
		return "", err
	}
	temporary, err := os.CreateTemp(root, ".selected-")
	if err != nil {
		return "", err
	}
	temporaryName := temporary.Name()
	if _, err := temporary.Write(selected); err != nil {
		temporary.Close()
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(temporaryName, filepath.Join(root, "selected.json")); err != nil {
		return "", err
	}
	return filepath.Join(publication, controlBinaryName()), nil
}

func copyControlBinary(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

var controlDiagnosticOffset = regexp.MustCompile(`"offset"\s*:\s*(\d+)`)

func controlBuildDiagnostic(options WorkflowControlBuildOptions, source string, bodyLine int, cause error) error {
	match := controlDiagnosticOffset.FindStringSubmatch(cause.Error())
	if len(match) != 2 {
		return cause
	}
	offset, err := strconv.Atoi(match[1])
	prefix := strings.SplitAfter(source, "\n")
	start := 0
	for _, line := range prefix[:bodyLine-1] {
		start += len(line)
	}
	if err != nil || offset < start || offset > start+len(options.Workflow.Control.Source) {
		return cause
	}
	line := options.Workflow.Control.SourceLine + strings.Count(source[start:offset], "\n")
	return fmt.Errorf("%s:%d: PipeLang control diagnostic: %w", options.ConfigPath, line, cause)
}
