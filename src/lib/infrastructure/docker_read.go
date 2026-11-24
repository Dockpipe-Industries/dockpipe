package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
	"dockpipe/src/lib/infrastructure/process"
)

// ContainerReadRequest selects one exact Docker object. The adapter,
// not the request, chooses the executable, endpoint, arguments and log tail.
type ContainerReadRequest struct {
	Action string `json:"action"`
	Kind   string `json:"kind"`
	Object string `json:"object"`
}

type ContainerReadResult struct {
	Schema  string          `json:"schema"`
	Action  string          `json:"action"`
	Kind    string          `json:"kind"`
	Object  string          `json:"object"`
	Inspect json.RawMessage `json:"inspect,omitempty"`
	Logs    string          `json:"logs,omitempty"`
}

// Detailed reads use the same 512-KiB output budget as inventory. This allows
// complete inspection of one object or a 200-line diagnostic tail while keeping
// even JSON-escaped logs comfortably inside the remote feature response budget.
const maxDockerReadBytes = 512 * 1024

type dockerReadOutput struct {
	mu        sync.Mutex
	buffer    bytes.Buffer
	exhausted bool
}

func (output *dockerReadOutput) Write(data []byte) (int, error) {
	output.mu.Lock()
	defer output.mu.Unlock()
	if len(data) > maxDockerReadBytes-output.buffer.Len() {
		output.exhausted = true
		return 0, errors.New("detail response from Docker exceeds 512 KiB")
	}
	return output.buffer.Write(data)
}

func ValidateContainerReadRequest(request ContainerReadRequest) error {
	if request.Action != "inspect" && request.Action != "logs" {
		return errors.New("unsupported container read action")
	}
	switch request.Kind {
	case "container", "network":
		if !isFullContainerID(request.Object) {
			return errors.New("an exact full object ID is required for Docker inspection")
		}
	case "volume":
		if !isSafeDockerContainerName(request.Object) {
			return errors.New("an exact volume name is required for Docker inspection")
		}
	default:
		return errors.New("unsupported Docker object kind")
	}
	if request.Action == "logs" && request.Kind != "container" {
		return errors.New("a container is required to read Docker logs")
	}
	return nil
}

func ReadContainer(ctx context.Context, request ContainerReadRequest) (ContainerReadResult, error) {
	target, identity := nativeinvocation.Local()
	return ReadContainerWithAuthority(ctx, request, target, identity, nil)
}

func ReadContainerWithAuthority(ctx context.Context, request ContainerReadRequest, target invocation.Target, identity invocation.Identity, authorize invocation.Authorize) (ContainerReadResult, error) {
	if err := ValidateContainerReadRequest(request); err != nil {
		return ContainerReadResult{}, err
	}
	id := invocation.ID{Capability: "dockpipe.containers", Action: request.Action, Version: "1"}
	definition := invocation.Definition{ID: id, Target: target, RequireAuthorization: authorize != nil || target.Kind == invocation.Remote}
	native, err := invocation.New(definition, func(_ context.Context, owned ContainerReadRequest) (invocation.Prepared[ContainerReadResult], error) {
		environment := os.Environ()
		prepared := invocation.Prepared[ContainerReadResult]{Execute: func(ctx context.Context) (ContainerReadResult, error) {
			ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			arguments := []string{owned.Kind, "inspect", owned.Object}
			if owned.Action == "logs" {
				arguments = []string{"container", "logs", "--tail", "200", owned.Object}
			}
			name, err := resolveDockerCommandFromEnvironment(environment)
			if err != nil {
				return ContainerReadResult{}, err
			}
			command := exec.CommandContext(ctx, name, arguments...)
			command.Env = dockerCommandEnv(environment, name)
			command.WaitDelay = time.Second
			output := &dockerReadOutput{}
			command.Stdout = output
			command.Stderr = io.Discard
			if owned.Action == "logs" {
				command.Stderr = output
			}
			if err := process.Run(command); err != nil {
				if ctx.Err() != nil {
					return ContainerReadResult{}, ctx.Err()
				}
				return ContainerReadResult{}, errors.New("the detail query to Docker failed or exceeded its 512-KiB response limit")
			}
			result := ContainerReadResult{Schema: "dockpipe.containers/v1", Action: owned.Action, Kind: owned.Kind, Object: owned.Object}
			if owned.Action == "logs" {
				result.Logs = output.buffer.String()
				return result, nil
			}
			var rows []json.RawMessage
			if json.Unmarshal(output.buffer.Bytes(), &rows) != nil || len(rows) != 1 {
				return ContainerReadResult{}, errors.New("invalid Docker inspect response")
			}
			var object map[string]json.RawMessage
			if json.Unmarshal(rows[0], &object) != nil || object == nil {
				return ContainerReadResult{}, errors.New("invalid Docker inspect object")
			}
			identityField := "Id"
			if owned.Kind == "volume" {
				identityField = "Name"
			}
			var actual string
			if json.Unmarshal(object[identityField], &actual) != nil || actual != owned.Object {
				return ContainerReadResult{}, errors.New("object identity returned by Docker inspect differed from the request")
			}
			result.Inspect = append(json.RawMessage(nil), output.buffer.Bytes()...)
			return result, nil
		}}
		if definition.RequireAuthorization {
			prepared.Digest = NativeExecutionDigest([]string{owned.Action, owned.Kind, owned.Object}, environment)
		}
		return prepared, nil
	}, authorize)
	if err != nil {
		return ContainerReadResult{}, err
	}
	return native.Invoke(ctx, invocation.Call{Contract: id, Target: target, Identity: identity}, request)
}
