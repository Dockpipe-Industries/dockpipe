package infrastructure

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"time"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
	"dockpipe/src/lib/infrastructure/process"
)

// ContainerExecRequest selects an interactive shell in one exact container.
// Docker provider, endpoint, arguments and inherited descriptors are native
// composition decisions. This local session is not a remote feature action.
type ContainerExecRequest struct {
	Container string
	Shell     string
}

func ValidateContainerExecRequest(request ContainerExecRequest) error {
	if !isFullContainerID(request.Container) {
		return errors.New("container control requires an exact full container ID")
	}
	if request.Shell != "/bin/sh" && request.Shell != "/bin/bash" {
		return errors.New("unsupported container shell")
	}
	return nil
}

func ExecLocalContainer(ctx context.Context, request ContainerExecRequest) error {
	return ExecLocalContainerWithAdmission(NativeExecutionAdmission{Context: ctx}, request, os.Stdin, os.Stdout, os.Stderr)
}

// ExecLocalContainerWithAdmission admits the actual session, not an executable
// path that another adapter may later launch. Input bytes remain on inherited
// stdin; no reusable permit, command journal or shell-text parser is involved.
// The owning terminal/context controls session lifetime without a fixed timeout.
func ExecLocalContainerWithAdmission(admission NativeExecutionAdmission, request ContainerExecRequest, stdin, stdout, stderr *os.File) error {
	if err := ValidateContainerExecRequest(request); err != nil {
		return err
	}
	if stdin == nil || stdout == nil || stderr == nil {
		return errors.New("container shell requires inherited standard streams")
	}
	target, identity := nativeinvocation.Local()
	id := invocation.ID{Capability: "dockpipe.containers", Action: "exec", Version: "1"}
	definition := invocation.Definition{ID: id, Target: target, RequireAuthorization: admission.Authorize != nil}
	native, err := invocation.New(definition, func(_ context.Context, owned ContainerExecRequest) (invocation.Prepared[struct{}], error) {
		environment := os.Environ()
		prepared := invocation.Prepared[struct{}]{Execute: func(ctx context.Context) (struct{}, error) {
			name, err := resolveDockerCommandFromEnvironment(environment)
			if err != nil {
				return struct{}{}, err
			}
			arguments := []string{"container", "exec", "-i"}
			tty := useDockerInteractiveTTY(stdin, stdout, stderr)
			if tty {
				arguments = append(arguments, "-t")
			}
			arguments = append(arguments, owned.Container, owned.Shell)
			command := exec.CommandContext(ctx, name, arguments...)
			command.Env = dockerCommandEnv(environment, name)
			command.Stdin, command.Stdout, command.Stderr = stdin, stdout, stderr
			command.WaitDelay = time.Second
			restore, err := prepareDockerExecTerminal(command, stdin)
			if err != nil {
				return struct{}{}, err
			}
			err = process.Run(command)
			if ctx.Err() != nil {
				err = ctx.Err()
			}
			return struct{}{}, errors.Join(err, restore())
		}}
		if definition.RequireAuthorization {
			prepared.Digest = NativeExecutionDigest([]string{owned.Container, owned.Shell,
				invocationFileName(stdin), invocationFileName(stdout), invocationFileName(stderr)}, environment)
		}
		return prepared, nil
	}, admission.Authorize)
	if err != nil {
		return err
	}
	_, err = native.Invoke(admission.ExecutionContext(), invocation.Call{Contract: id, Target: target, Identity: identity}, request)
	return err
}
