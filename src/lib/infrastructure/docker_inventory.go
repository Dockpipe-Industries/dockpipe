package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"dockpipe/src/lib/infrastructure/process"
	"dockpipe/src/lib/infrastructure/telemetry"
)

type inventoryOutput struct{ bytes.Buffer }

func (output *inventoryOutput) Write(data []byte) (int, error) {
	if len(data) > 512*1024-output.Len() {
		return 0, errors.New("docker inventory exceeds 512 KiB")
	}
	return output.Buffer.Write(data)
}

func dockerInventoryCommand(ctx context.Context, args ...string) ([]byte, error) {
	commandName := dockerCommandName()
	command := exec.CommandContext(ctx, commandName, args...)
	command.Env = dockerCommandEnv(os.Environ(), commandName)
	command.WaitDelay = 500 * time.Millisecond
	output := &inventoryOutput{}
	command.Stdout = output
	command.Stderr = io.Discard
	if err := process.Run(command); err != nil {
		return nil, errors.New("docker inventory unavailable or query limit exceeded")
	}
	return output.Bytes(), nil
}

func inventoryRows[T any](data []byte) ([]T, bool, error) {
	rows := []T{}
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if len(rows) == telemetry.MaxDockerObjects {
			return rows, true, nil
		}
		var row T
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, false, errors.New("invalid Docker inventory response")
		}
		rows = append(rows, row)
	}
	return rows, false, nil
}

func CollectDockerInventory(ctx context.Context) telemetry.DockerSnapshot {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	result := telemetry.DockerSnapshot{Containers: []telemetry.DockerContainer{}, Networks: []telemetry.DockerNetwork{},
		Volumes: []telemetry.DockerVolume{}, Topology: []telemetry.DockerTopology{}, Notices: []string{}}
	containers, err := dockerInventoryCommand(ctx, "container", "ls", "--all", "--no-trunc", "--format", "{{json .}}")
	if err != nil {
		result.Notices = append(result.Notices, err.Error())
		return result
	}
	result.Containers, result.Truncated, err = inventoryRows[telemetry.DockerContainer](containers)
	if err != nil {
		result.Notices = append(result.Notices, err.Error())
		return result
	}
	result.Available = true
	networks, networkErr := dockerInventoryCommand(ctx, "network", "ls", "--no-trunc", "--format", "{{json .}}")
	var truncated bool
	if networkErr == nil {
		result.Networks, truncated, networkErr = inventoryRows[telemetry.DockerNetwork](networks)
		result.Truncated = result.Truncated || truncated
	}
	if networkErr != nil {
		result.Notices = append(result.Notices, "Network inventory unavailable")
	}
	volumes, volumeErr := dockerInventoryCommand(ctx, "volume", "ls", "--format", "{{json .}}")
	if volumeErr == nil {
		result.Volumes, truncated, volumeErr = inventoryRows[telemetry.DockerVolume](volumes)
		result.Truncated = result.Truncated || truncated
	}
	if volumeErr != nil {
		result.Notices = append(result.Notices, "Volume inventory unavailable")
	}
	if len(result.Containers) == 0 {
		return result
	}
	format := `{"Id":{{json .Id}},"Name":{{json .Name}},"Labels":{"com.docker.compose.project":{{json (index .Config.Labels "com.docker.compose.project")}},"com.docker.compose.service":{{json (index .Config.Labels "com.docker.compose.service")}},"com.docker.compose.depends_on":{{json (index .Config.Labels "com.docker.compose.depends_on")}}},"Networks":{{json .NetworkSettings.Networks}},"Mounts":{{json .Mounts}}}`
	args := []string{"container", "inspect", "--format", format}
	for _, container := range result.Containers {
		if len(container.ID) != 64 || strings.Trim(container.ID, "0123456789abcdef") != "" {
			result.Notices = append(result.Notices, "Container identity unavailable; topology omitted")
			return result
		}
		args = append(args, container.ID)
	}
	topology, topologyErr := dockerInventoryCommand(ctx, args...)
	if topologyErr == nil {
		result.Topology, truncated, topologyErr = inventoryRows[telemetry.DockerTopology](topology)
		result.Truncated = result.Truncated || truncated
	}
	if topologyErr != nil {
		result.Notices = append(result.Notices, "Container topology unavailable")
	}
	for index := range result.Topology {
		labels := result.Topology[index].Labels
		allowed := map[string]string{}
		for _, key := range []string{"com.docker.compose.project", "com.docker.compose.service", "com.docker.compose.depends_on"} {
			if value, ok := labels[key]; ok {
				allowed[key] = value
			}
		}
		result.Topology[index].Labels = allowed
	}
	return result
}
