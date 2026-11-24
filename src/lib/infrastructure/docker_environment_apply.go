package infrastructure

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
	"dockpipe/src/lib/infrastructure/process"
	"dockpipe/src/lib/infrastructure/telemetry"
	"dockpipe/src/lib/model/managedenvironment"
)

const dockerEnvironmentSettingsLabel = "com.dockpipe.environment.settings"

type environmentDocker struct {
	executable string
	env        []string
	endpoint   string
}

type DockerEnvironmentApplyRequest struct {
	Name       string
	InstanceID string
	Settings   managedenvironment.Container
}

func localDockerEnvironment(ctx context.Context, environment []string) (environmentDocker, error) {
	executable, err := resolveDockerCommandFromEnvironment(environment)
	if err != nil {
		return environmentDocker{}, errors.New("unable to use Docker; install or start Docker on this computer and try again")
	}
	endpoint := strings.TrimSpace(envGet(environment, "DOCKER_HOST"))
	docker := environmentDocker{executable: executable, env: environment}
	if endpoint == "" || envGet(environment, "DOCKER_CONTEXT") != "" {
		result, err := docker.run(ctx, "context", "inspect", "--format", "{{.Endpoints.docker.Host}}")
		if err != nil {
			return environmentDocker{}, err
		}
		endpoint = strings.TrimSpace(string(result))
	}
	if !localDockerEnvironmentEndpoint(endpoint) {
		return environmentDocker{}, errors.New("container environments require a Docker engine on this computer")
	}

	docker.endpoint = endpoint
	docker.env = make([]string, 0, len(environment))
	for _, entry := range environment {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(key) {
		case "DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_TLS", "DOCKER_TLS_VERIFY":
			continue
		}
		docker.env = append(docker.env, entry)
	}
	return docker, nil
}

func (docker environmentDocker) run(ctx context.Context, arguments ...string) ([]byte, error) {
	if docker.endpoint != "" {
		arguments = append([]string{"--host", docker.endpoint}, arguments...)
	}
	command := exec.CommandContext(ctx, docker.executable, arguments...)
	command.Env = dockerCommandEnv(docker.env, docker.executable)
	command.WaitDelay = time.Second
	output := &inventoryOutput{}
	diagnostic := &boundedTelemetryOutput{}
	command.Stdout, command.Stderr = output, diagnostic
	if err := process.RunWithScope(ctx, command); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		message := strings.TrimSpace(diagnostic.String())
		if message != "" {
			return nil, fmt.Errorf("error from Docker: %s", message)
		}
		return nil, fmt.Errorf("command failed in Docker: %w", err)
	}
	return output.buffer.Bytes(), nil
}

// ApplyDockerEnvironment creates at most one container per definition on the
// selected local engine, then starts only an exact matching container.
func ApplyDockerEnvironment(ctx context.Context, name, instanceID string, settings managedenvironment.Container) (string, error) {
	target, identity := nativeinvocation.Local()
	request := DockerEnvironmentApplyRequest{Name: name, InstanceID: instanceID, Settings: settings}
	return ApplyDockerEnvironmentWithAuthority(ctx, request, target, identity, nil)
}

// ApplyDockerEnvironmentWithAuthority binds a container definition and the
// executing host's Docker environment before contacting its local engine.
func ApplyDockerEnvironmentWithAuthority(ctx context.Context, request DockerEnvironmentApplyRequest, target invocation.Target,
	identity invocation.Identity, authorize invocation.Authorize) (string, error) {
	id := invocation.ID{Capability: "dockpipe.environments", Action: "container-apply", Version: "1"}
	definition := invocation.Definition{
		ID: id, Target: target, RequireAuthorization: authorize != nil || target.Kind == invocation.Remote,
	}
	native, err := invocation.New(definition, func(_ context.Context, request DockerEnvironmentApplyRequest) (invocation.Prepared[string], error) {
		if !managedenvironment.ValidName(request.Name) || len(request.InstanceID) != 32 ||
			strings.Trim(request.InstanceID, "0123456789abcdef") != "" {
			return invocation.Prepared[string]{}, errors.New("invalid container environment identity")
		}
		if err := request.Settings.Validate(); err != nil {
			return invocation.Prepared[string]{}, err
		}

		environment := os.Environ()
		arguments := dockerEnvironmentArguments(request.InstanceID, request.Settings)
		prepared := invocation.Prepared[string]{Execute: func(ctx context.Context) (string, error) {
			return applyDockerEnvironment(ctx, request.Name, request.InstanceID, arguments, environment)
		}}
		if definition.RequireAuthorization {
			prepared.Digest = NativeExecutionDigest([]string{request.Name, request.InstanceID}, arguments, environment)
		}
		return prepared, nil
	}, authorize)
	if err != nil {
		return "", err
	}

	return native.Invoke(ctx, invocation.Call{Contract: id, Target: target, Identity: identity}, request)
}

func applyDockerEnvironment(ctx context.Context, name, instanceID string, arguments, environment []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	docker, err := localDockerEnvironment(ctx, environment)
	if err != nil {
		return "", err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(arguments, "\x00"))))
	container, err := docker.observe(ctx, instanceID)
	if err != nil {
		return "", err
	}
	if container.ID == "" {
		container, err = docker.create(ctx, name, instanceID, digest, arguments)
		if err != nil {
			return "", err
		}
	}

	format := `{{index .Config.Labels "com.dockpipe.environment.instance"}}|{{index .Config.Labels "com.dockpipe.environment.settings"}}|{{.State.Running}}`
	result, err := docker.run(ctx, "container", "inspect", "--format", format, container.ID)
	if err != nil {
		return "", err
	}
	state := strings.TrimSpace(string(result))
	switch state {
	case instanceID + "|" + digest + "|true":
		return container.ID, nil
	case instanceID + "|" + digest + "|false":
		_, err := docker.run(ctx, "container", "start", container.ID)
		return container.ID, err
	default:
		return "", errors.New("the existing container has different settings; save a new environment to create another container with these settings")
	}
}

func (docker environmentDocker) create(ctx context.Context, name, instanceID, digest string, arguments []string) (telemetry.DockerContainer, error) {
	create := []string{"container", "create", "--name", "dockpipe-" + name + "-" + instanceID,
		"--label", dockerEnvironmentInstanceLabel + "=" + instanceID,
		"--label", dockerEnvironmentSettingsLabel + "=" + digest}
	output, err := docker.run(ctx, append(create, arguments...)...)
	if err != nil {
		// A concurrent apply may have created the deterministic name first.
		existing, observeErr := docker.observe(ctx, instanceID)
		if observeErr == nil && existing.ID != "" {
			return existing, nil
		}
		return telemetry.DockerContainer{}, err
	}
	containerID := strings.TrimSpace(string(output))
	if !isFullContainerID(containerID) {
		return telemetry.DockerContainer{}, errors.New("no exact container ID returned by Docker after creation")
	}
	return telemetry.DockerContainer{ID: containerID}, nil
}

func dockerEnvironmentArguments(instanceID string, settings managedenvironment.Container) []string {
	network := "bridge"
	if settings.Network == "none" {
		network = "none"
	}
	restart := "no"
	if settings.Restart == "unless-stopped" {
		restart = settings.Restart
	}
	arguments := []string{"--platform", "linux", "--network", network, "--restart", restart,
		"--security-opt", "no-new-privileges", "--init"}
	if settings.Security == "restricted" {
		arguments = append(arguments, "--cap-drop", "ALL")
	}
	if settings.User == "non-root" {
		arguments = append(arguments, "--user", "65534:65534")
	}
	if settings.ReadOnly {
		arguments = append(arguments, "--read-only")
	}
	if settings.Publish != nil {
		mapping := "127.0.0.1:" + strconv.Itoa(int(settings.Publish.Host)) + ":" + strconv.Itoa(int(settings.Publish.Container))
		arguments = append(arguments, "--publish", mapping)
	}
	if settings.DataPath != "" {
		arguments = append(arguments, "--mount", "type=volume,source=dockpipe-"+instanceID+",target="+settings.DataPath)
	}
	if settings.Startup == "keep-running" {
		return append(arguments, "--entrypoint", "/bin/sleep", "--", settings.Image, "2147483647")
	}
	return append(arguments, "--", settings.Image)
}
