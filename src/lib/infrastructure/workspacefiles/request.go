// Package workspacefiles owns bounded workspace file operations. Views and
// remote transports consume the same operations; neither defines file policy.
package workspacefiles

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const Schema = "dockpipe.files/v1"

// These bounds match the existing built-in text editor and graph directory page.
const MaxFileBytes = 1 << 20
const MaxPageEntries = 150
const MaxPathBytes = 32768

var (
	ErrInvalidRequest = errors.New("invalid workspace file request")
	ErrOutsideRoot    = errors.New("workspace paths cannot traverse links, Git metadata, or another root")
	ErrNotRegular     = errors.New("the selected item is not a regular file")
	ErrChanged        = errors.New("the selected file or directory changed; refresh before retrying")
	ErrLimit          = errors.New("the selected file exceeds the 1 MiB editor limit")
)

// Root selection is trusted composition, not a field a remote request can change.
// Content is the exact byte representation, including BOM and line endings.
type Request struct {
	Schema      string `json:"schema"`
	Action      string `json:"action"`
	Path        string `json:"path"`
	Destination string `json:"destination,omitempty"`
	Revision    string `json:"revision,omitempty"`
	Content     []byte `json:"content,omitempty"`
	After       string `json:"after,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Hidden      bool   `json:"hidden,omitempty"`
}

type Entry struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	SizeBytes int64  `json:"size_bytes,omitempty,string"`
}

type Result struct {
	Schema   string  `json:"schema"`
	Path     string  `json:"path"`
	Revision string  `json:"revision,omitempty"`
	Content  []byte  `json:"content,omitempty"`
	Entries  []Entry `json:"entries,omitempty"`
	More     bool    `json:"more,omitempty"`
	After    string  `json:"after,omitempty"`
}

func validPath(value string, directory bool) bool {
	if len(value) == 0 || len(value) > MaxPathBytes || !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return false
	}
	if value == "." {
		return directory
	}
	if !fs.ValidPath(value) || strings.Contains(value, "\\") || filepath.IsAbs(value) || filepath.VolumeName(value) != "" {
		return false
	}
	for _, component := range strings.Split(value, "/") {
		if strings.EqualFold(component, ".git") {
			return false
		}
	}
	return true
}

func validRevision(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

func (request Request) validate() error {
	if request.Schema != Schema {
		return ErrInvalidRequest
	}
	switch request.Action {
	case "list", "read", "write", "create", "rename", "delete", "mkdir":
	default:
		return ErrInvalidRequest
	}
	if !validPath(request.Path, request.Action == "list") {
		return ErrOutsideRoot
	}
	if request.Action == "rename" {
		if !validPath(request.Destination, false) || request.Destination == request.Path {
			return ErrOutsideRoot
		}
	} else if request.Destination != "" {
		return ErrInvalidRequest
	}
	if len(request.Content) > MaxFileBytes {
		return ErrLimit
	}
	if request.Action != "write" && request.Action != "create" && len(request.Content) != 0 {
		return ErrInvalidRequest
	}
	if request.Action == "write" || request.Action == "rename" || request.Action == "delete" {
		if !validRevision(request.Revision) {
			return ErrInvalidRequest
		}
	} else if request.Action == "list" {
		if request.Revision != "" && !validRevision(request.Revision) {
			return ErrInvalidRequest
		}
	} else if request.Revision != "" {
		return ErrInvalidRequest
	}
	if request.Action == "list" {
		if request.Limit < 1 || request.Limit > MaxPageEntries || len(request.After) > MaxPathBytes || !utf8.ValidString(request.After) {
			return ErrInvalidRequest
		}
		if request.After != "" && (!validPath(request.After, false) || strings.Contains(request.After, "/")) {
			return ErrInvalidRequest
		}
	} else if request.Limit != 0 || request.After != "" || request.Hidden {
		return ErrInvalidRequest
	}
	return nil
}
