package infrastructure

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"dockpipe/src/lib/domain/workflow"
)

type workflowControlManifest struct {
	Schema            int    `json:"schema"`
	Profile           string `json:"profile"`
	DefinitionSHA256  string `json:"definition_sha256"`
	SDKManifestSHA256 string `json:"sdk_manifest_sha256"`
	SourceSHA256      string `json:"source_sha256"`
	BinarySHA256      string `json:"binary_sha256"`
	BinarySize        int64  `json:"binary_size"`
	OS                string `json:"os"`
	Architecture      string `json:"architecture"`
	BodyLine          int    `json:"body_line"`
	Sanitizers        bool   `json:"sanitizers"`
}

func controlCacheDirectory(workdir, identity string) (string, error) {
	cache, err := StateInternalCacheDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "workflow-control", "v1", runtime.GOOS+"-"+runtime.GOARCH, identity), nil
}

func controlBinaryName() string {
	if runtime.GOOS == "windows" {
		return "control.exe"
	}
	return "control"
}

func controlFileDigest(path string, limit int64) (string, int64, error) {
	file, err := openControlRegular(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", 0, err
	}
	link, linkErr := os.Lstat(path)
	if linkErr != nil || !info.Mode().IsRegular() || !link.Mode().IsRegular() || !os.SameFile(info, link) || info.Size() > limit {
		return "", 0, fmt.Errorf("control artifact must be a bounded regular file")
	}
	hash := sha256.New()
	size, err := io.Copy(hash, io.LimitReader(file, limit+1))
	if err != nil || size > limit || size != info.Size() {
		return "", 0, fmt.Errorf("control artifact changed or exceeded its byte limit")
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func validControlDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, ch := range value {
		if !(ch >= '0' && ch <= '9') && !(ch >= 'a' && ch <= 'f') {
			return false
		}
	}
	return true
}

func loadControlArtifact(workdir string, definition *workflow.Workflow, workflowRoots ...string) (string, workflowControlManifest, error) {
	var manifest workflowControlManifest
	identity, err := controlDefinitionIdentity(definition)
	if err != nil {
		return "", manifest, err
	}
	root, err := controlCacheDirectory(workdir, identity)
	if err != nil {
		return "", manifest, err
	}
	selected, err := readControlMetadata(filepath.Join(root, "selected.json"), 256)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			for _, workflowRoot := range workflowRoots {
				if workflowRoot == "" {
					continue
				}
				directory := filepath.Join(workflowRoot, controlBundleRelative(identity))
				if _, statErr := os.Lstat(filepath.Join(directory, "manifest.json")); statErr == nil {
					return validateControlArtifactDirectory(directory, definition, identity)
				} else if !errors.Is(statErr, os.ErrNotExist) {
					return "", manifest, statErr
				}
			}
		}
		return "", manifest, fmt.Errorf("workflow control build_required: explicitly build this workflow first")
	}
	if len(selected) > 256 {
		return "", manifest, fmt.Errorf("invalid control artifact selection")
	}
	var selection struct {
		Artifact string `json:"artifact"`
	}
	if err := json.Unmarshal(selected, &selection); err != nil || !strings.HasPrefix(selection.Artifact, "artifact-") || strings.ContainsAny(selection.Artifact, "/\\") || len(selection.Artifact) > 80 {
		return "", manifest, fmt.Errorf("invalid control artifact selection")
	}
	artifact := filepath.Join(root, selection.Artifact)
	return validateControlArtifactDirectory(artifact, definition, identity)
}

func validateControlArtifactDirectory(artifact string, definition *workflow.Workflow, identity string) (string, workflowControlManifest, error) {
	var manifest workflowControlManifest
	encoded, err := readControlMetadata(filepath.Join(artifact, "manifest.json"), 8192)
	if err != nil || len(encoded) > 8192 {
		return "", manifest, fmt.Errorf("invalid control artifact manifest")
	}
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return "", manifest, fmt.Errorf("invalid control artifact manifest: %w", err)
	}
	source, _, _ := controlSource(definition)
	sourceDigest := sha256.Sum256([]byte(source))
	if manifest.Schema != 1 || manifest.Profile != workflowControlProfile || manifest.DefinitionSHA256 != identity ||
		!validControlDigest(manifest.SDKManifestSHA256) || manifest.OS != runtime.GOOS || manifest.Architecture != runtime.GOARCH ||
		manifest.SourceSHA256 != hex.EncodeToString(sourceDigest[:]) || !validControlDigest(manifest.BinarySHA256) {
		return "", manifest, fmt.Errorf("workflow control build_required: artifact identity does not match the workflow")
	}
	binary := filepath.Join(artifact, controlBinaryName())
	hash, size, err := controlFileDigest(binary, 256*1024*1024)
	if err != nil || hash != manifest.BinarySHA256 || size != manifest.BinarySize {
		return "", manifest, fmt.Errorf("workflow control artifact binary integrity failed")
	}
	return binary, manifest, nil
}

// ValidateWorkflowControlArtifact admits execution without compiling or repairing.
func ValidateWorkflowControlArtifact(workdir string, definition *workflow.Workflow, workflowRoots ...string) error {
	if definition == nil || definition.Control == nil {
		return nil
	}
	_, _, err := loadControlArtifact(workdir, definition, workflowRoots...)
	return err
}

func readControlMetadata(path string, maximum int64) ([]byte, error) {
	file, err := openControlRegular(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	link, linkErr := os.Lstat(path)
	if err != nil || linkErr != nil || !info.Mode().IsRegular() || !link.Mode().IsRegular() || !os.SameFile(info, link) || info.Size() > maximum {
		return nil, fmt.Errorf("control metadata must be a bounded regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maximum {
		return nil, fmt.Errorf("control metadata exceeds its byte bound")
	}
	return data, nil
}

func controlBundleRelative(identity string) string {
	return filepath.Join("assets", "dockpipe-control", runtime.GOOS+"-"+runtime.GOARCH, identity)
}

// ExportWorkflowControlArtifact stages an already-built verified artifact. It
// never invokes a compiler from package compilation or workflow execution.
func ExportWorkflowControlArtifact(workdir string, definition *workflow.Workflow, destination string) error {
	if definition == nil || definition.Control == nil {
		return nil
	}
	binary, manifest, err := loadControlArtifact(workdir, definition)
	if err != nil {
		return err
	}
	root := filepath.Join(destination, controlBundleRelative(manifest.DefinitionSHA256))
	if _, err := os.Lstat(root); err == nil {
		_, existing, err := validateControlArtifactDirectory(root, definition, manifest.DefinitionSHA256)
		if err != nil {
			return err
		}
		if existing.BinarySHA256 != manifest.BinarySHA256 || existing.SDKManifestSHA256 != manifest.SDKManifestSHA256 || existing.Sanitizers != manifest.Sanitizers {
			return fmt.Errorf("staged workflow control differs from the selected build")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	if err := copyControlBinary(binary, filepath.Join(root, controlBinaryName())); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "manifest.json"), encoded, 0o600)
}
