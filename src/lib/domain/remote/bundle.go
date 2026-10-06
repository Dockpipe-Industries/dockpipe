package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"strings"
)

const MaxBundleBytes = 8 << 20
const MaxBundleFiles = 1024

// Bundle carries only selected source files. It has no archive entries, links,
// ownership metadata, environment, executable override, or worker-side paths.
type Bundle struct {
	WorkflowFile string       `json:"workflow_file"`
	Files        []BundleFile `json:"files"`
	Artifacts    []string     `json:"artifacts,omitempty"`
}

type BundleFile struct {
	Path       string `json:"path"`
	Data       []byte `json:"data"`
	Executable bool   `json:"executable,omitempty"`
}

type SubmitRequest struct {
	Submission
	Bundle *Bundle `json:"bundle,omitempty"`
}

// DeliveryPermission is worker-local authority to execute operator-supplied
// code as the worker user. It is deliberately separate from installed profiles.
type DeliveryPermission struct {
	TimeoutSeconds int `json:"timeout_seconds"`
}

func (b Bundle) Validate() error {
	if !SafeBundlePath(b.WorkflowFile) || len(b.Files) == 0 || len(b.Files) > MaxBundleFiles || len(b.Artifacts) > 32 {
		return errors.New("invalid delivery manifest bounds")
	}
	seen := map[string]bool{}
	total := 0
	for _, file := range b.Files {
		key := strings.ToLower(file.Path)
		if !SafeBundlePath(file.Path) || seen[key] {
			return errors.New("unsafe or duplicate delivery file")
		}
		seen[key] = true
		total += len(file.Data)
		if total > MaxBundleBytes {
			return errors.New("delivery exceeds 8 MiB")
		}
	}
	for _, file := range b.Files {
		for parent := path.Dir(file.Path); parent != "."; parent = path.Dir(parent) {
			if seen[strings.ToLower(parent)] {
				return errors.New("delivery file conflicts with a directory")
			}
		}
	}
	found := false
	for _, file := range b.Files {
		found = found || file.Path == b.WorkflowFile
	}
	if !found {
		return errors.New("delivery is missing its selected workflow")
	}
	for _, artifact := range b.Artifacts {
		if !SafeRelative(artifact) || len(artifact) > 512 {
			return errors.New("invalid delivery artifact path")
		}
	}
	return nil
}

// Reject private/generated paths rather than silently sending a partial tree.
// This is a filename guard, not a secret scanner: authors must review source.
func SafeBundlePath(value string) bool {
	if !SafeRelative(value) || len(value) > 512 {
		return false
	}
	for _, component := range strings.Split(strings.ToLower(value), "/") {
		if strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".pem") || strings.HasSuffix(component, ".key") || strings.HasSuffix(component, ".p12") || strings.HasSuffix(component, ".pfx") {
			return false
		}
		switch component {
		case "dockpipe.config.json", "credentials", "credentials.json", "id_rsa", "id_ed25519", "node_modules":
			return false
		}
		for _, character := range component {
			if character < 32 || character > 126 {
				return false
			}
		}
		if strings.HasSuffix(component, " ") || strings.HasSuffix(component, ".") {
			return false
		}
	}
	return true
}

func (b Bundle) Digest() (string, error) {
	if err := b.Validate(); err != nil {
		return "", err
	}
	data, err := json.Marshal(b)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}
