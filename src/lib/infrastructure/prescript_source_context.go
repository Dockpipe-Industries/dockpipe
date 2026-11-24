package infrastructure

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"sync"
)

// SourceHostScriptWithContext preserves the sourced environment contract while
// carrying cancellation through the actual platform process and output owner.
func SourceHostScriptWithContext(parent context.Context, path string, environment []string) (map[string]string, error) {
	ctx, stop := hostExecutionSignalContext(hostExecutionContext(parent))
	defer stop()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return AdmitSourceHostScript(NativeExecutionAdmission{Context: ctx}, path, environment,
		func(path string, environment []string) (map[string]string, error) {
			return sourceHostScriptUsing(path, environment, func(original *exec.Cmd) ([]byte, error) {
				command := hostCommandWithContext(ctx, original)
				output := &sourceHostOutput{}
				err := runContextSourceHostScriptProcess(ctx, path, command, original.Env, output)
				return output.bytes(), err
			})
		})
}

// Both output readers share the same synchronized sink, matching CombinedOutput
// without concurrent bytes.Buffer writes or publishing env0 data to the console.
// Capture retains the legacy unbounded contract. A separate source-output byte
// contract is still required; cancellation does not establish a memory bound.
type sourceHostOutput struct {
	mutex  sync.Mutex
	buffer bytes.Buffer
}

var _ io.Writer = (*sourceHostOutput)(nil)

func (output *sourceHostOutput) Write(data []byte) (int, error) {
	output.mutex.Lock()
	defer output.mutex.Unlock()

	return output.buffer.Write(data)
}

func (output *sourceHostOutput) bytes() []byte {
	output.mutex.Lock()
	defer output.mutex.Unlock()

	return output.buffer.Bytes()
}
