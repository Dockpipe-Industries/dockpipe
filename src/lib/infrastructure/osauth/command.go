package osauth

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

type commandRunner func(context.Context, string, ...string) ([]byte, error)

const systemCommandFailureMessage = "OS security service rejected the request or is unavailable"

type systemCommandExitError struct {
	code int
}

func (systemCommandExitError) Error() string {
	return systemCommandFailureMessage
}

type boundedOutput struct {
	buffer  bytes.Buffer
	maximum int
}

func (output *boundedOutput) Write(data []byte) (int, error) {
	if len(data) > output.maximum-output.buffer.Len() {
		return 0, errors.New("OS security command output exceeded its bound")
	}
	return output.buffer.Write(data)
}

func (output *boundedOutput) Bytes() []byte {
	return output.buffer.Bytes()
}

func (output *boundedOutput) String() string {
	return output.buffer.String()
}

func runSystemCommand(ctx context.Context, program string, arguments ...string) ([]byte, error) {
	return runSystemCommandBounded(ctx, program, 64<<10, arguments...)
}

func runSystemCommandBounded(ctx context.Context, program string, maximum int, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, program, arguments...)
	// OS services are selected explicitly. Project environment, dynamic loader
	// overrides, user PATH and caller-selected D-Bus addresses are not inherited.
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	command.WaitDelay = time.Second
	output := &boundedOutput{maximum: maximum}
	command.Stdout = output
	command.Stderr = &boundedOutput{maximum: 4096}
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil, systemCommandExitError{code: exit.ExitCode()}
		}

		return nil, errors.New(systemCommandFailureMessage)
	}
	return output.Bytes(), nil
}
