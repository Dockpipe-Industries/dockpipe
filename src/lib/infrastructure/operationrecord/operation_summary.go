package operationrecord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const MaxOperationSummaries = 200
const operationSummaryBytes = 64 * 1024

// OperationSummary is an allowlist for the live inspector. Free-form messages,
// errors and ID maps can contain execution output and are intentionally absent.
type OperationSummary struct {
	Timestamp  string `json:"ts"`
	Unit       string `json:"unit"`
	Status     string `json:"status"`
	DurationMS *int64 `json:"duration_ms,omitempty"`
}

func ReadOperationSummaries(path string) ([]OperationSummary, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("operation log is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	offset := max(int64(0), info.Size()-operationSummaryBytes)
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, operationSummaryBytes))
	if err != nil {
		return nil, err
	}
	lines := bytes.Split(data, []byte{'\n'})
	if offset > 0 && len(lines) > 0 {
		lines = lines[1:]
	}
	summaries := make([]OperationSummary, 0, MaxOperationSummaries)
	start := max(0, len(lines)-MaxOperationSummaries-1)
	for _, line := range lines[start:] {
		var event OperationEvent
		if json.Unmarshal(line, &event) != nil || event.Schema != OperationEventSchemaV1 {
			continue
		}
		// Unit/status are protocol identifiers, not arbitrary log strings.
		if !safeOperationIdentifier(event.Unit) || !safeOperationIdentifier(event.Status) || len(event.Timestamp) > 40 {
			continue
		}
		summaries = append(summaries, OperationSummary{Timestamp: event.Timestamp, Unit: event.Unit, Status: event.Status, DurationMS: event.DurationMs})
		if len(summaries) == MaxOperationSummaries {
			break
		}
	}
	return summaries, nil
}

func safeOperationIdentifier(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z') && !(char >= '0' && char <= '9') && char != '.' && char != '_' && char != '-' {
			return false
		}
	}
	return true
}
