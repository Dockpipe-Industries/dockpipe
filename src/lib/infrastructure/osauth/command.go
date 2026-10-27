package osauth

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

type commandRunner func(context.Context, string, ...string) ([]byte, error)

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
	command := exec.CommandContext(ctx, program, arguments...)
	// OS services are selected explicitly. Project environment, dynamic loader
	// overrides, user PATH and caller-selected D-Bus addresses are not inherited.
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	command.WaitDelay = time.Second
	output := &boundedOutput{maximum: 64 << 10}
	command.Stdout = output
	command.Stderr = &boundedOutput{maximum: 4096}
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("OS security service rejected the request or is unavailable")
	}
	return output.Bytes(), nil
}
