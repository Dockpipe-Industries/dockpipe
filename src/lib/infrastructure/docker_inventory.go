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

// Serializing Docker's formatter root also evaluates its Size method, which can
// enable costly filesystem scans. Request only the fields in our inventory DTOs.
const containerInventoryFormat = `{"ID":{{json .ID}},"Names":{{json .Names}},"State":{{json .State}},"Status":{{json .Status}},"Image":{{json .Image}},"Ports":{{json .Ports}},"RunningFor":{{json .RunningFor}}}`
const networkInventoryFormat = `{"ID":{{json .ID}},"Name":{{json .Name}},"Driver":{{json .Driver}},"Scope":{{json .Scope}}}`
const volumeInventoryFormat = `{"Name":{{json .Name}},"Driver":{{json .Driver}}}`

func (output *inventoryOutput) Write(data []byte) (int, error) {
	if len(data) > 512*1024-output.Len() {
		return 0, errors.New("docker inventory exceeds 512 KiB")
	}
	return output.Buffer.Write(data)
}

func dockerInventoryCommand(ctx context.Context, args ...string) ([]byte, error) {
	return runDockerInventoryCommand(ctx, false, args...)
}

func localDockerInventoryCommand(ctx context.Context, args ...string) ([]byte, error) {
	return runDockerInventoryCommand(ctx, true, args...)
}

func dockerInventoryProcess(ctx context.Context, local bool, args ...string) *exec.Cmd {
	commandName := dockerCommandName()
	if local {
		args = append([]string{"--host=unix:///var/run/docker.sock", "--tls=false"}, args...)
	}
	command := exec.CommandContext(ctx, commandName, args...)
	command.Env = dockerCommandEnv(os.Environ(), commandName)
	if local {
		environment := make([]string, 0, len(command.Env))
		for _, entry := range command.Env {
			name, _, _ := strings.Cut(entry, "=")
			if !strings.HasPrefix(strings.ToUpper(name), "DOCKER_") {
				environment = append(environment, entry)
			}
		}
		command.Env = environment
	}
	command.WaitDelay = 500 * time.Millisecond
	return command
}

func runDockerInventoryCommand(ctx context.Context, local bool, args ...string) ([]byte, error) {
	command := dockerInventoryProcess(ctx, local, args...)
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
	return collectDockerInventory(ctx, dockerInventoryCommand)
}

// CollectLocalDockerInventory pins inventory to the local system Unix daemon,
// independent of user context, endpoint environment and TLS preferences.
func CollectLocalDockerInventory(ctx context.Context) telemetry.DockerSnapshot {
	return collectDockerInventory(ctx, localDockerInventoryCommand)
}

func collectDockerInventory(ctx context.Context, query func(context.Context, ...string) ([]byte, error)) telemetry.DockerSnapshot {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	result := telemetry.DockerSnapshot{Containers: []telemetry.DockerContainer{}, Networks: []telemetry.DockerNetwork{},
		Volumes: []telemetry.DockerVolume{}, Topology: []telemetry.DockerTopology{}, Notices: []string{}}
	containers, err := query(ctx, "container", "ls", "--all", "--no-trunc", "--size=false", "--format", containerInventoryFormat)
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
	networks, networkErr := query(ctx, "network", "ls", "--no-trunc", "--format", networkInventoryFormat)
	var truncated bool
	if networkErr == nil {
		result.Networks, truncated, networkErr = inventoryRows[telemetry.DockerNetwork](networks)
		result.Truncated = result.Truncated || truncated
	}
	if networkErr != nil {
		result.Notices = append(result.Notices, "Network inventory unavailable")
	}
	volumes, volumeErr := query(ctx, "volume", "ls", "--format", volumeInventoryFormat)
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
	topology, topologyErr := query(ctx, args...)
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
