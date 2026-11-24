package nativehost

import (
	"errors"
	"strings"
)

func validDockerAction(action, id string) bool {
	return (action == "start" || action == "stop") && len(id) >= 12 && len(id) <= 64 && strings.Trim(id, "0123456789abcdefABCDEF") == ""
}

func ValidateDockerAction(action, id string) error {
	if !validDockerAction(action, id) {
		return errors.New("native Docker requires start or stop and a 12–64 digit hexadecimal container ID")
	}
	return nil
}

func failedDocker(action, id, reason string) DockerResult {
	return DockerResult{Schema: DockerActionSchema, Outcome: "Failure", Action: action, ID: id, Reason: reason}
}
