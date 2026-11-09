package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
	"dockpipe/src/lib/infrastructure/process"
)

// ContainerLifecycleReceipt binds only the provider/resource identity and its
// synchronous completion acknowledgment. It retains no Docker configuration.
type ContainerLifecycleReceipt struct {
	Container string `json:"container"`
	Provider  string `json:"provider"`
	Daemon    string `json:"daemon"`
	Completed bool   `json:"completed"`
}

type containerLifecycleKey struct{}
type containerLifecycleObserver func(ContainerLifecycleReceipt) error

func WithContainerLifecycle(ctx context.Context, observer func(ContainerLifecycleReceipt) error) context.Context {
	return context.WithValue(ctx, containerLifecycleKey{}, containerLifecycleObserver(observer))
}

func ContainerProviderBinding(name string, environment []string) string {
	selected := []string{name}
	for _, key := range []string{"DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_CONFIG", "DOCKER_CERT_PATH", "DOCKER_TLS_VERIFY", "DOCKER_API_VERSION"} {
		selected = append(selected, key+"="+envGet(environment, key))
	}
	data, _ := json.Marshal(selected)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func ContainerDaemonDigest(id string) string {
	digest := sha256.Sum256([]byte(id))
	return hex.EncodeToString(digest[:])
}

func containerProviderQuery(ctx context.Context, name string, environment []string, arguments ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, name, arguments...)
	command.Env = dockerCommandEnv(environment, name)
	var output dockerReadOutput
	command.Stdout = &output
	command.Stderr = io.Discard
	command.WaitDelay = time.Second
	if err := process.RunWithScope(ctx, command); err != nil {
		return nil, errors.New("Docker provider readback could not be confirmed")
	}
	return bytes.TrimSpace(output.buffer.Bytes()), nil
}

func containerDaemonIdentity(ctx context.Context, name string, environment []string) (string, error) {
	data, err := containerProviderQuery(ctx, name, environment, "info", "--format", "{{.ID}}")
	if err != nil || len(data) == 0 || len(data) > 256 || bytes.ContainsAny(data, "\r\n\x00") {
		return "", errors.New("Docker daemon identity could not be confirmed")
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func observeContainerStart(ctx context.Context, name string, environment []string, container string) (*ContainerLifecycleReceipt, error) {
	observer, exists := ctx.Value(containerLifecycleKey{}).(containerLifecycleObserver)
	if !exists {
		return nil, nil
	}
	daemon, err := containerDaemonIdentity(ctx, name, environment)
	if err != nil {
		return nil, err
	}
	receipt := ContainerLifecycleReceipt{Container: container, Provider: ContainerProviderBinding(name, environment), Daemon: daemon}
	if err := observer(receipt); err != nil {
		return nil, err
	}
	return &receipt, nil
}

func observeContainerCompletion(ctx context.Context, receipt *ContainerLifecycleReceipt) error {
	if receipt == nil {
		return nil
	}
	observer, exists := ctx.Value(containerLifecycleKey{}).(containerLifecycleObserver)
	if !exists {
		return nil
	}
	receipt.Completed = true
	return observer(*receipt)
}

// RecoverContainerLifecycle never repeats start/stop/restart. A successful
// original CLI response is a daemon completion receipt. Otherwise the operator
// must explicitly retire the exact resource: removal prevents a delayed old
// daemon request from starting that object again. A current inspect alone is
// not a fence for Docker's cancellation-independent restart handler.
func RecoverContainerLifecycle(ctx context.Context, receipt ContainerLifecycleReceipt, retire bool) error {
	target, identity := nativeinvocation.Local()
	return RecoverContainerLifecycleWithAuthority(ctx, receipt, retire, target, identity, nil)
}

func RecoverContainerLifecycleWithAuthority(ctx context.Context, receipt ContainerLifecycleReceipt, retire bool, target invocation.Target, identity invocation.Identity, authorize invocation.Authorize) error {
	action := "recover"
	if retire {
		action = "retire"
	}
	id := invocation.ID{Capability: "dockpipe.containers", Action: action, Version: "1"}
	definition := invocation.Definition{ID: id, Target: target, RequireAuthorization: target.Kind == invocation.Remote || authorize != nil}
	native, err := invocation.New(definition, func(_ context.Context, owned ContainerLifecycleReceipt) (invocation.Prepared[bool], error) {
		if !isFullContainerID(owned.Container) || len(owned.Provider) != 64 || len(owned.Daemon) != 64 {
			return invocation.Prepared[bool]{}, errors.New("invalid container recovery receipt")
		}
		data, _ := json.Marshal(struct {
			Receipt ContainerLifecycleReceipt
			Retire  bool
		}{owned, retire})
		digest := sha256.Sum256(data)
		return invocation.Prepared[bool]{Digest: hex.EncodeToString(digest[:]), Execute: func(ctx context.Context) (bool, error) {
			err := recoverContainerLifecycle(ctx, owned, retire)
			return err == nil, err
		}}, nil
	}, authorize)
	if err != nil {
		return err
	}
	_, err = native.Invoke(ctx, invocation.Call{Contract: id, Target: target, Identity: identity}, receipt)
	return err
}

func recoverContainerLifecycle(ctx context.Context, receipt ContainerLifecycleReceipt, retire bool) error {
	if !isFullContainerID(receipt.Container) || len(receipt.Provider) != 64 || len(receipt.Daemon) != 64 {
		return errors.New("the original Docker provider/resource receipt is unavailable")
	}
	environment := os.Environ()
	name, err := resolveDockerCommandFromEnvironment(environment)
	if err != nil || ContainerProviderBinding(name, environment) != receipt.Provider {
		return errors.New("the original Docker provider selection changed")
	}
	daemon, err := containerDaemonIdentity(ctx, name, environment)
	if err != nil || daemon != receipt.Daemon {
		return errors.New("the original Docker daemon identity is unavailable")
	}
	if receipt.Completed {
		return nil
	}
	if !retire {
		return errors.New("Docker completion is uncertain; explicitly retire the captured container to fence delayed daemon work")
	}
	if observer, exists := ctx.Value(containerLifecycleKey{}).(containerLifecycleObserver); exists {
		if err := observer(receipt); err != nil {
			return err
		}
	}
	command := exec.CommandContext(ctx, name, "container", "rm", "--force", "--", receipt.Container)
	command.Env = dockerCommandEnv(environment, name)
	command.Stdout, command.Stderr = io.Discard, io.Discard
	command.WaitDelay = time.Second
	// Removal failure is not proof of absence. A successful authoritative list
	// below may still confirm that a prior explicit removal finished.
	_ = process.RunWithScope(ctx, command)
	data, err := containerProviderQuery(ctx, name, environment, "container", "ls", "--all", "--no-trunc", "--filter", "id="+receipt.Container, "--format", "{{.ID}}")
	if err != nil || strings.TrimSpace(string(data)) != "" {
		return errors.New("the exact container is not yet confirmed removed; its unknown operation remains blocked")
	}
	verifiedDaemon, err := containerDaemonIdentity(ctx, name, environment)
	if err != nil || verifiedDaemon != receipt.Daemon {
		return errors.New("Docker daemon changed during resource retirement")
	}
	return observeContainerCompletion(ctx, &receipt)
}
