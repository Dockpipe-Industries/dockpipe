package infrastructure

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"dockpipe/src/lib/infrastructure/telemetry"
)

// ObserveDockerEnvironment reads a single explicitly associated container from
// the current local Docker engine. A missing or ambiguous label is unverified.
func ObserveDockerEnvironment(ctx context.Context, id string) (telemetry.DockerContainer, error) {
	if len(id) != 32 || strings.Trim(id, "0123456789abcdef") != "" {
		return telemetry.DockerContainer{}, errors.New("invalid environment instance identity")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	docker, err := localDockerEnvironment(ctx, os.Environ())
	if err != nil {
		return telemetry.DockerContainer{}, err
	}
	return docker.observe(ctx, id)
}

const dockerEnvironmentInstanceLabel = "com.dockpipe.environment.instance"

func (docker environmentDocker) observe(ctx context.Context, id string) (telemetry.DockerContainer, error) {
	output, err := docker.run(ctx, "container", "ls", "--all", "--no-trunc",
		"--filter", "label="+dockerEnvironmentInstanceLabel+"="+id, "--format", "{{json .}}")
	if err != nil {
		return telemetry.DockerContainer{}, err
	}
	containers, truncated, err := inventoryRows[telemetry.DockerContainer](output)
	if err != nil || truncated || len(containers) > 1 {
		return telemetry.DockerContainer{}, errors.New("ambiguous or unavailable Docker environment association")
	}
	if len(containers) == 0 {
		return telemetry.DockerContainer{}, nil
	}
	if !isFullContainerID(containers[0].ID) {
		return telemetry.DockerContainer{}, errors.New("no exact container identity for the Docker environment association")
	}

	return containers[0], nil
}

func localDockerEnvironmentEndpoint(endpoint string) bool {
	return strings.HasPrefix(endpoint, "unix:///") || strings.HasPrefix(endpoint, "npipe:////./pipe/")
}
