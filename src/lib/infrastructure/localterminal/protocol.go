// Package localterminal implements a private, ephemeral terminal stream. It
// connects a native local caller to an installed native provider through the
// shared terminal admission boundary, without a daemon or persistent receipts.
package localterminal

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"dockpipe/src/lib/infrastructure/terminalsession"
)

const (
	Schema         = "dockpipe.local-terminal/v1"
	MaxStartLine   = 2 * 1024 * 1024
	MaxCommandLine = 96 * 1024
	MaxOutputChunk = 32 * 1024
	MaxID          = 1<<53 - 1
)

var ErrProtocol = errors.New("invalid local terminal protocol")

type Command struct {
	Schema  string                   `json:"schema"`
	Type    string                   `json:"type"`
	ID      uint64                   `json:"id"`
	Profile *terminalsession.Profile `json:"profile,omitempty"`
	Columns int                      `json:"columns,omitempty"`
	Rows    int                      `json:"rows,omitempty"`
	Data    []byte                   `json:"data,omitempty"`
}

type Event struct {
	Schema    string `json:"schema"`
	Type      string `json:"type"`
	ID        uint64 `json:"id"`
	SessionID string `json:"session_id,omitempty"`
	Bytes     int    `json:"bytes,omitempty"`
	Data      []byte `json:"data,omitempty"`
	Cursor    uint64 `json:"cursor,omitempty"`
	Oldest    uint64 `json:"oldest,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
	ExitCode  int    `json:"exit_code,omitempty"`
	Cleanup   string `json:"cleanup,omitempty"`
	Code      string `json:"code,omitempty"`
}

func DecodeCommand(data []byte, start bool) (Command, error) {
	var command Command
	limit := MaxCommandLine
	if start {
		limit = MaxStartLine
	}
	if len(data) > limit || decode(data, &command) != nil || command.Schema != Schema || command.ID == 0 || command.ID > MaxID {
		return command, ErrProtocol
	}
	if start {
		if command.Type != "start" || command.ID != 1 || command.Profile == nil || !validSize(command.Columns, command.Rows) || len(command.Data) != 0 || !exactFields(data, "schema", "type", "id", "profile", "columns", "rows") {
			return command, ErrProtocol
		}
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(data, &fields)
		if !exactFields(fields["profile"], "id", "executable", "args", "env", "dir", "route", "environment_mode") {
			return command, ErrProtocol
		}
		if ValidateProfile(*command.Profile) != nil {
			return command, ErrProtocol
		}
		return command, nil
	}
	if command.Profile != nil {
		return command, ErrProtocol
	}
	switch command.Type {
	case "input":
		if len(command.Data) == 0 || len(command.Data) > terminalsession.MaxInput || command.Columns != 0 || command.Rows != 0 || !exactFields(data, "schema", "type", "id", "data") {
			return command, ErrProtocol
		}
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(data, &fields)
		var encoded string
		_ = json.Unmarshal(fields["data"], &encoded)
		if encoded != base64.StdEncoding.EncodeToString(command.Data) {
			return command, ErrProtocol
		}
	case "resize":
		if !validSize(command.Columns, command.Rows) || len(command.Data) != 0 || !exactFields(data, "schema", "type", "id", "columns", "rows") {
			return command, ErrProtocol
		}
	case "close":
		if command.Columns != 0 || command.Rows != 0 || len(command.Data) != 0 || !exactFields(data, "schema", "type", "id") {
			return command, ErrProtocol
		}
	default:
		return command, ErrProtocol
	}
	return command, nil
}

func exactFields(data []byte, names ...string) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || len(fields) != len(names) {
		return false
	}
	for _, name := range names {
		value, exists := fields[name]
		if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return false
		}
	}
	return true
}

func validSize(columns, rows int) bool {
	return columns >= 1 && columns <= terminalsession.MaxDimension && rows >= 1 && rows <= terminalsession.MaxDimension
}

// Reject duplicate/case-aliased keys, unknown fields, invalid UTF-8, excessive
// nesting and trailing documents. Errors never include supplied field values.
func decode(data []byte, target any) error {
	if !utf8.Valid(data) {
		return ErrProtocol
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var visit func(int) error
	visit = func(depth int) error {
		if depth > 8 {
			return ErrProtocol
		}
		token, err := decoder.Token()
		if err != nil {
			return ErrProtocol
		}
		delimiter, container := token.(json.Delim)
		if !container {
			return nil
		}
		switch delimiter {
		case '{':
			seen := make(map[string]bool)
			for decoder.More() {
				token, err := decoder.Token()
				key, ok := token.(string)
				if err != nil || !ok || seen[key] || key != strings.ToLower(key) {
					return ErrProtocol
				}
				seen[key] = true
				if err := visit(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := visit(depth + 1); err != nil {
					return err
				}
			}
		default:
			return ErrProtocol
		}
		_, err = decoder.Token()
		return err
	}
	if err := visit(0); err != nil {
		return ErrProtocol
	}
	if _, err := decoder.Token(); err != io.EOF {
		return ErrProtocol
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		return ErrProtocol
	}
	return nil
}
