package remote

import (
	"errors"
	"path/filepath"
	"strings"
	"time"
)

type Result struct {
	Status       string            `json:"status"`
	ExitCode     int               `json:"exit_code"`
	Log          string            `json:"log"`
	LogTruncated bool              `json:"log_truncated"`
	Artifacts    map[string][]byte `json:"artifacts,omitempty"`
	StartedAt    time.Time         `json:"started_at"`
	FinishedAt   time.Time         `json:"finished_at"`
	OS           string            `json:"os"`
	Architecture string            `json:"architecture"`
}

func (r Result) Validate() error {
	switch r.Status {
	case "success", "failure", "cancelled", "unknown":
	default:
		return errors.New("invalid result status")
	}
	if len(r.Log) > MaxLog || len(r.Artifacts) > 32 || (r.Status == "success" && r.ExitCode != 0) {
		return errors.New("invalid result bounds or exit status")
	}
	total := 0
	for name, data := range r.Artifacts {
		if !SafeRelative(name) {
			return errors.New("invalid artifact path")
		}
		total += len(data)
	}
	if total > MaxArtifacts {
		return errors.New("artifact size limit exceeded")
	}
	return nil
}

// SafeRelative accepts canonical relative paths in the remote wire contract.
func SafeRelative(path string) bool {
	return path != "" && !strings.ContainsAny(path, "\\:\x00\r\n") && filepath.IsLocal(path) && filepath.ToSlash(filepath.Clean(path)) == path && path != "."
}
