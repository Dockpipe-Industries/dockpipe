package nativehost

import (
	"encoding/json"
	"errors"
	"regexp"
)

const DockerQuerySchema = "dockpipe.native_host_docker_query.v1"
const maxDockerInspect = 256 << 10
const maxDockerLogs = 64 << 10
const maxDockerQuery = 900 << 10

var dockerVolumeName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,254}$`)

type DockerQueryResult struct {
	Schema        string          `json:"schema"`
	Outcome       string          `json:"outcome"`
	Kind          string          `json:"kind"`
	ID            string          `json:"id"`
	Inspect       json.RawMessage `json:"inspect,omitempty"`
	Logs          string          `json:"logs"`
	LogsError     string          `json:"logs_error,omitempty"`
	LogsTruncated bool            `json:"logs_truncated,omitempty"`
	Reason        string          `json:"reason,omitempty"`
}

func ValidateDockerQuery(kind, id string) error {
	if ((kind == "container" || kind == "network") && validDockerAction("start", id)) || (kind == "volume" && dockerVolumeName.MatchString(id)) {
		return nil
	}
	return errors.New("native Docker query requires a container/network hexadecimal ID or a valid volume name")
}

func failedDockerQuery(kind, id, reason string) DockerQueryResult {
	return DockerQueryResult{Schema: DockerQuerySchema, Outcome: "Failure", Kind: kind, ID: id, Reason: reason}
}

func validDockerQueryResult(value DockerQueryResult, kind, id string) bool {
	if value.Schema != DockerQuerySchema || value.Kind != kind || value.ID != id || ValidateDockerQuery(kind, id) != nil || len(value.Inspect) > maxDockerInspect || len(value.Logs) > maxDockerLogs {
		return false
	}
	if value.Outcome != "Success" && value.Outcome != "Failure" && value.Outcome != "Cancelled" {
		return false
	}
	switch value.Reason {
	case "", "docker_unavailable", "docker_query_failed", "docker_response_invalid", "docker_response_too_large", "operation_cancelled", "native_host_unavailable":
	default:
		return false
	}
	if value.LogsError != "" && value.LogsError != "docker_logs_unavailable" && value.LogsError != "docker_logs_invalid" {
		return false
	}
	if value.Outcome == "Success" {
		var object map[string]json.RawMessage
		if value.Reason != "" || json.Unmarshal(value.Inspect, &object) != nil || object == nil {
			return false
		}
	} else if value.Reason == "" || len(value.Inspect) != 0 || value.Logs != "" {
		return false
	}
	encoded, err := json.Marshal(value)
	return err == nil && len(encoded) <= maxDockerQuery
}
